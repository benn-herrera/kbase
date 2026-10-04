// Package docgraph builds kb_tools' document graph from one volume root: the
// pre-passes, pandoc's read, the JSON transforms, pandoc's gfm render, the
// whole-volume cut into a tree, and the records the claim graph reads.
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

// Options is one build of one volume.
type Options struct {
	// VolumeRoot is the top .tex file of the paper.
	VolumeRoot string
	// Bibliographies are every file citations resolve against, in the order
	// that decides a key two files define.
	Bibliographies []string
	// KBRoot is where the tree is written.
	KBRoot string
}

// Report is what one build produced.
type Report struct {
	Documents int
	Records   []records.Record
	Censuses  []prepass.Census
	// UnreadBibliographies are the files offered that the reader could not
	// use; the volume was converted as though offered none.
	UnreadBibliographies []string
	// Findings are the build's checks over its own products, in the order
	// they ran.
	Findings []Finding
}

// Failed is whether any check found its two products disagreeing.
func (r Report) Failed() bool {
	return slices.ContainsFunc(r.Findings, func(f Finding) bool { return f.Status == StatusFail })
}

// Build converts the volume root and writes its tree into KBRoot.
func Build(ctx context.Context, lg log.Logger, opts Options) (Report, error) {
	raw, err := os.ReadFile(opts.VolumeRoot)
	if err != nil {
		return Report{}, fmt.Errorf("reading the volume root: %w", err)
	}
	volumeDir, err := filepath.Abs(filepath.Dir(opts.VolumeRoot))
	if err != nil {
		return Report{}, err
	}
	stem := strings.TrimSuffix(filepath.Base(opts.VolumeRoot), filepath.Ext(opts.VolumeRoot))
	pre := prepass.Apply(string(raw))
	report := Report{Censuses: pre.Censuses}
	for _, c := range pre.Censuses {
		for _, u := range c.Unread {
			lg.Warn("pre-pass left a declaration it could not read where it was written", "pass", c.Pass, "line", u.Line, "text", u.Text)
		}
	}

	ast, bibliographies, err := read(ctx, lg, volumeDir, pre.Source, opts.Bibliographies)
	if err != nil {
		return Report{}, err
	}
	if len(bibliographies) < len(opts.Bibliographies) {
		report.UnreadBibliographies = opts.Bibliographies
	}
	doc, err := decodeAST(ast)
	if err != nil {
		return Report{}, err
	}
	keysOnly := len(bibliographies) == 0
	o, drafts, err := readVolume(doc, pre.TheoremNames, keysOnly)
	if err != nil {
		return Report{}, err
	}
	transform(doc, pre.TheoremNames, keysOnly)
	transformed, err := encodeAST(doc)
	if err != nil {
		return Report{}, err
	}
	markdown, err := pandoc.WriteGFM(ctx, lg, transformed)
	if err != nil {
		return Report{}, err
	}
	tree, err := buildTree(stem, markdown, o, volumeDir)
	if err != nil {
		return Report{}, err
	}
	placed := place(drafts, tree, len(o.headers))
	report.Findings = slices.Concat(
		checkASTAgainstMarkdown(stem, doc, markdown),
		checkMarkdownAgainstTree(stem, tree),
		checkMath(stem, doc, tree),
		checkAnchors(stem, tree),
		checkAssets(stem, tree),
	)
	if err := writeTree(tree, opts.KBRoot, volumeDir); err != nil {
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
	report.Documents = len(tree.index.walk())
	report.Records = placed
	return report, nil
}

// deadLinks is the dead-link gate over the written tree.
func deadLinks(kbRoot string) ([]Finding, error) {
	dead, err := kb.DeadLinks(kbRoot)
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
