package main

import (
	"fmt"
	"os"
	"path/filepath"

	"kbase/internal/index"
	"kbase/internal/kb"
	toolresult "kbase/internal/result"
)

func runRenderClaimGraph(opts maintenanceOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "render-claim-graph"); err != nil {
		return 0, err
	}
	outcome, fields := renderClaimGraph(opts)
	return emitResult("render-claim-graph", opts.Stdout, outcome, fields)
}

func renderClaimGraph(opts maintenanceOptions) (string, []toolresult.Field) {
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	fields := []toolresult.Field{{Key: "kb-root", Value: root}}
	release, outcome, items, err := holdKB(root, "re-run kbase render-claim-graph")
	if err != nil {
		return failed(fields, err)
	}
	if release == nil {
		return outcome, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: items})
	}
	defer release()
	src, refusals, err := openKB(root)
	if err != nil {
		return failed(fields, err)
	}
	if refusals != nil {
		return refused(fields, refusals...)
	}
	if info, err := os.Stat(filepath.Join(root, kb.IndexDir)); err != nil || !info.IsDir() {
		return refused(fields, toolresult.Item{Check: "index", Path: kb.IndexDir, Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("%s has no %s/ to render from; refresh derives it", root, kb.IndexDir)})
	}
	written, removed, drawn, err := index.WriteSheet(src)
	if err != nil {
		return failed(fields, err)
	}
	fields = append(fields, toolresult.Field{Key: "written", Value: append([]string{}, written...)},
		toolresult.Field{Key: "removed", Value: append([]string{}, removed...)})
	if drawn {
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetDrawn})
	} else {
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetPlaceholder}, toolresult.Field{Key: "dot", Value: dotNotFound})
	}
	if len(written)+len(removed) > 0 {
		return toolresult.Done, fields
	}
	return toolresult.Unchanged, fields
}

// Whether render-claim-graph drew the sheets through Graphviz dot or, with no
// dot on PATH, stood the placeholder in for the root sheet.
const (
	sheetPlaceholder = "placeholder"
	sheetDrawn       = "drawn"
	dotNotFound      = "not found on PATH"
)

func init() {
	rootCmd.AddCommand(maintenanceCommand("render-claim-graph",
		"render the claim-graph sheets from .index/",
		`render-claim-graph draws the KB's claim-graph sheets from kb-root/.index/ and
the build records beside kb-root/, through Graphviz dot: kb-root/claim-graph.svg,
every claim with each volume a cluster, always; and, where two or more volumes
hold nodes, kb-root/claim-graph-digest.svg, one box per volume linking its
index.md and its sheet, the two root sheets linking each other, and
<volume>/claim-graph.svg for every volume holding a node, that volume and every
node one edge from it. It writes only the sheets whose bytes change, and
removes a digest or volume sheet the KB no longer calls for. With no dot on
PATH it draws nothing, leaves every sheet standing as it is, and writes a
placeholder kb-root/claim-graph.svg only where none exists. Its result is one
YAML document on stdout.`, runRenderClaimGraph))
}
