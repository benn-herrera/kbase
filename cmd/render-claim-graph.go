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
	if info, err := os.Stat(filepath.Join(root, kb.IndexDir)); err != nil || !info.IsDir() {
		return refused(fields, toolresult.Item{Check: "index", Path: kb.IndexDir, Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("%s has no %s/ to render from; refresh derives it", root, kb.IndexDir)})
	}
	written, drawn, err := index.WriteSheet(root)
	if err != nil {
		return failed(fields, err)
	}
	sheet := sheetPlaceholder
	if drawn {
		sheet = sheetDrawn
	}
	if written {
		return toolresult.Done, append(fields, toolresult.Field{Key: "written", Value: []string{kb.ClaimGraphFile}}, toolresult.Field{Key: "sheet", Value: sheet})
	}
	return toolresult.Unchanged, append(fields, toolresult.Field{Key: "written", Value: []string{}}, toolresult.Field{Key: "sheet", Value: sheet})
}

// What render-claim-graph found standing: kbase's placeholder, or a drawn
// sheet, which it never writes over.
const (
	sheetPlaceholder = "placeholder"
	sheetDrawn       = "drawn"
)

func init() {
	rootCmd.AddCommand(maintenanceCommand("render-claim-graph",
		"render the claim-graph sheet from .index/",
		`render-claim-graph renders the placeholder claim-graph.svg — "NYI" over a
digest of kb-root/.index/ — and writes it only where no sheet exists or the
existing one is the placeholder; a drawn sheet is left as it is. It writes only
when the bytes change. Its result is one YAML document on stdout.`, runRenderClaimGraph))
}
