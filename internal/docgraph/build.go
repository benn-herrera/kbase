// Package docgraph builds kb_tools' document graph from its volume roots: per
// volume the pre-passes, pandoc's read, the JSON transforms, pandoc's gfm
// render and the whole-volume cut into a tree of its own; then the entry point
// listing every volume, and the records the claim graph reads.
package docgraph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/latex/pandoc"
	"kbase/internal/latex/prepass"
	"kbase/internal/log"
	"kbase/internal/records"
)

// Options is one build of every volume.
type Options struct {
	// VolumeRoots are the top .tex files of the papers, one volume each, in
	// the order the entry point lists them.
	VolumeRoots []string
	// Bibliographies are every file citations resolve against, in the order
	// that decides a key two files define; every volume is offered them all.
	Bibliographies []string
	// KBRoot is where the tree is written.
	KBRoot string
}

// Report is what one build produced.
type Report struct {
	Documents int
	Records   []records.Record
	Censuses  []VolumeCensus
	// UnreadBibliographies are the files offered that the reader could not
	// use; a volume that could not use them was converted as though offered
	// none.
	UnreadBibliographies []string
	// Findings are the build's checks over its own products, in the order
	// they ran.
	Findings []Finding
}

// VolumeCensus is one pre-pass's census of one volume root.
type VolumeCensus struct {
	Volume string `json:"volume"`
	prepass.Census
}

// VolumeError is a volume root that could not be converted.
type VolumeError struct {
	Root string
	Err  error
}

func (e VolumeError) Error() string { return e.Root + ": " + e.Err.Error() }

func (e VolumeError) Unwrap() error { return e.Err }

// Failed is whether any check found its two products disagreeing.
func (r Report) Failed() bool {
	return slices.ContainsFunc(r.Findings, func(f Finding) bool { return f.Status == StatusFail })
}

// Build converts each volume root into a tree of its own and writes them,
// with the entry point listing every volume, into KBRoot.
func Build(ctx context.Context, lg log.Logger, opts Options) (Report, error) {
	var report Report
	var trees []*volumeTree
	var placed [][]records.Record
	for _, root := range opts.VolumeRoots {
		v, err := convertVolume(ctx, lg, root, opts.Bibliographies)
		if err != nil {
			return Report{}, VolumeError{Root: root, Err: err}
		}
		trees = append(trees, v.tree)
		placed = append(placed, v.records)
		for _, c := range v.censuses {
			report.Censuses = append(report.Censuses, VolumeCensus{Volume: root, Census: c})
		}
		if v.unreadBibliographies {
			report.UnreadBibliographies = opts.Bibliographies
		}
		report.Findings = append(report.Findings, v.findings...)
		report.Documents += len(v.tree.index.walk())
	}
	if err := writeTrees(trees, opts.KBRoot); err != nil {
		return Report{}, err
	}
	validation, err := validateBuild(opts.KBRoot)
	if err != nil {
		return Report{}, err
	}
	links, err := deadLinks(opts.KBRoot)
	if err != nil {
		return Report{}, err
	}
	report.Findings = slices.Concat(report.Findings, validation, links)
	report.Records = joinRecords(placed)
	return report, nil
}

// volume is one volume root converted: its tree, its records placed in it,
// its pre-pass censuses, whether it was offered bibliographies it could not
// use, and its own checks' findings.
type volume struct {
	tree                 *volumeTree
	records              []records.Record
	censuses             []prepass.Census
	unreadBibliographies bool
	findings             []Finding
}

// convertVolume converts one volume root into its tree.
func convertVolume(ctx context.Context, lg log.Logger, root string, offered []string) (volume, error) {
	raw, err := os.ReadFile(root)
	if err != nil {
		return volume{}, fmt.Errorf("reading the volume root: %w", err)
	}
	volumeDir, err := filepath.Abs(filepath.Dir(root))
	if err != nil {
		return volume{}, err
	}
	stem := strings.TrimSuffix(filepath.Base(root), filepath.Ext(root))
	pre := prepass.Apply(string(raw))
	for _, c := range pre.Censuses {
		for _, u := range c.Unread {
			lg.Warn("pre-pass left a declaration it could not read where it was written", "volume", root, "pass", c.Pass, "line", u.Line, "text", u.Text)
		}
	}

	ast, bibliographies, err := read(ctx, lg, volumeDir, pre.Source, offered)
	if err != nil {
		return volume{}, err
	}
	doc, err := decodeAST(ast)
	if err != nil {
		return volume{}, err
	}
	keysOnly := len(bibliographies) == 0
	o, drafts, err := readVolume(doc, pre.TheoremNames, keysOnly)
	if err != nil {
		return volume{}, err
	}
	transform(doc, pre.TheoremNames, keysOnly)
	transformed, err := encodeAST(doc)
	if err != nil {
		return volume{}, err
	}
	markdown, err := pandoc.WriteGFM(ctx, lg, transformed)
	if err != nil {
		return volume{}, err
	}
	tree, err := buildTree(stem, markdown, o, volumeDir)
	if err != nil {
		return volume{}, err
	}
	return volume{
		tree:                 tree,
		records:              place(drafts, tree, len(o.headers)),
		censuses:             pre.Censuses,
		unreadBibliographies: len(bibliographies) < len(offered),
		findings: slices.Concat(
			checkASTAgainstMarkdown(stem, doc, markdown),
			checkMarkdownAgainstTree(stem, tree),
			checkMath(stem, doc, tree),
			checkAnchors(stem, tree),
			checkAssets(stem, tree),
		),
	}, nil
}

// deadLinks is the dead-link gate over the written tree.
func deadLinks(kbRoot string) ([]Finding, error) {
	dead, err := kb.DeadLinks(writtenTree(kbRoot))
	if err != nil {
		return nil, err
	}
	if len(dead) == 0 {
		return []Finding{{StatusPass, nameLinks, "no dead link under " + filepath.Base(kbRoot) + "/"}}, nil
	}
	findings := make([]Finding, len(dead))
	for i, d := range dead {
		findings[i] = Finding{StatusFail, nameLinks, d.String()}
	}
	return findings, nil
}

// read parses the source against the bibliographies, and again against none
// where they cannot be used: a bibliography the reader cannot read is the
// same input as no bibliography. The whole set goes, because the reader's
// exit code says a file failed and not which. It returns the bibliographies
// the AST was resolved against.
func read(ctx context.Context, lg log.Logger, dir, source string, bibliographies []string) ([]byte, []string, error) {
	usable := true
	for _, b := range bibliographies {
		if info, err := os.Stat(b); err != nil || !info.Mode().IsRegular() {
			lg.Warn("bibliography is not a readable file; converting as though offered none", "bibliography", b)
			usable = false
		}
	}
	if usable && len(bibliographies) > 0 {
		abs := make([]string, len(bibliographies))
		for i, b := range bibliographies {
			p, err := filepath.Abs(b)
			if err != nil {
				return nil, nil, err
			}
			abs[i] = p
		}
		ast, err := pandoc.ReadLaTeX(ctx, lg, dir, source, abs)
		var unread pandoc.BibliographyError
		if !errors.As(err, &unread) {
			return ast, bibliographies, err
		}
		lg.Warn("the reader could not read a bibliography; converting as though offered none", "complaint", unread.Complaint)
	}
	ast, err := pandoc.ReadLaTeX(ctx, lg, dir, source, nil)
	return ast, nil, err
}
