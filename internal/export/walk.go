// Package export assembles the inputs of a standing KB's LaTeX export: the
// walk of its volume and document spine with the minted label namespace, the
// navigation strip, the one Markdown stream pandoc reads, the tables the
// embedded filter reads, the defaults file, the embedded filter and template,
// the copied assets, gen-tex/ written under its manifest, and the author's
// starter preamble where none stands.
//
// It reads the KB through the kb.Source the verb hands it and never loads one
// itself, and it never runs pandoc: the verb does.
package export

import (
	"fmt"
	"path"
	"strings"

	"kbase/internal/kb"
)

// ReferencesFile is the leaf a volume's bibliography carries. It is read as
// data by the bibliography step, never walked as a document, so no
// \section{References} is emitted from it.
const ReferencesFile = "references.md"

// The report forms the walk raises: walkForm for a target it did not walk,
// and why; labelForm for a minted label another path already owns.
const (
	walkForm  = "walk"
	labelForm = "label"
)

// Tree is the whole walk: the entry point, and every volume in written order,
// volume number N being its position.
type Tree struct {
	EntryPoint string
	Volumes    []Volume
}

// Volume is one walked volume: its directory under kb-root, the label minted
// for it, the title its index carries, and its ordered document spine.
type Volume struct {
	Dir       string
	Label     string
	Title     string
	Documents []Document
}

// Document is one walked document: its kb-root-relative slash path, its rank
// in the volume's tree (1 below its parent), and the label minted for it, ""
// where another document of its volume already owns the label its path mints.
type Document struct {
	Path  string
	Rank  int
	Label string
}

// Target is what a walked path is to a reference naming it: the label minted
// for it and the number of the volume it walked in.
type Target struct {
	Label  string
	Volume int
}

// Targets is every walked path that owns a label — each volume's index under
// the volume's label, and each document under its own — mapped to its target.
// A path walks once across the whole export, so each key is set once.
func (t Tree) Targets() map[string]Target {
	targets := map[string]Target{}
	for i, v := range t.Volumes {
		n := i + 1
		targets[path.Join(v.Dir, kb.IndexFile)] = Target{Label: v.Label, Volume: n}
		for _, d := range v.Documents {
			if d.Label != "" {
				targets[d.Path] = Target{Label: d.Label, Volume: n}
			}
		}
	}
	return targets
}

// walk reads the KB tree src describes. The entry point's child list gives the
// volumes in written order; each volume index's child-list spine descends, its
// nested indexes one rank deeper, in the source's own order. Every named
// document walks — no-claim: leaves included, overview.md as ordinary leaf
// content — except a references.md leaf, which is not walked as a document.
//
// A document walks once across the whole export, whichever volume reaches it
// first: a path named again — by a second index, by an index naming itself or
// an ancestor, or by a later volume — is reported and not walked again, so a
// cycle is a finding rather than an unbounded descent; a volume whose index
// was already walked is reported and not walked. A child-list target naming no
// file, a target resolving outside its volume's directory, and a volume the
// entry point names but does not hold are each reported and not walked. A
// minted label that collides with another is reported, and the first path
// keeps it.
func walk(src *kb.Source, rep *Report) (Tree, error) {
	entry := kb.EntryPointFile
	text, err := src.ReadText(src.KBPath(entry))
	if err != nil {
		return Tree{}, fmt.Errorf("reading the entry point: %w", err)
	}
	tree := Tree{EntryPoint: entry}
	byLabel := map[string]string{}
	walked := map[string]bool{}
	for _, indexPath := range kb.ChildList(entry, text) {
		// A volume is an index: the entry point may also carry links to
		// documents that are not volumes, which no walk follows.
		if path.Base(indexPath) != kb.IndexFile {
			continue
		}
		if walked[indexPath] {
			rep.Add(walkForm, entry, "the volume index %s was already walked; the first walk stands", indexPath)
			continue
		}
		if !src.IsFile(src.KBPath(indexPath)) {
			rep.Add(walkForm, entry, "the volume index %s names no file in the KB", indexPath)
			continue
		}
		walked[indexPath] = true
		w := &walker{
			src:       src,
			rep:       rep,
			byLabel:   byLabel,
			volumeDir: path.Dir(indexPath),
			volume:    len(tree.Volumes) + 1,
			walked:    walked,
		}
		v, err := w.walkVolume(indexPath)
		if err != nil {
			return Tree{}, err
		}
		tree.Volumes = append(tree.Volumes, v)
	}
	return tree, nil
}

// walker is one volume's descent: the source it reads, the volume's directory
// and number, the labels the whole walk has minted, the paths the whole walk
// has already walked, and the report it raises findings in.
type walker struct {
	src       *kb.Source
	rep       *Report
	byLabel   map[string]string
	volumeDir string
	volume    int
	walked    map[string]bool
}

// walkVolume walks the volume whose index is the kb-root-relative indexPath.
func (w *walker) walkVolume(indexPath string) (Volume, error) {
	text, err := w.src.ReadText(w.src.KBPath(indexPath))
	if err != nil {
		return Volume{}, fmt.Errorf("reading the volume index %s: %w", indexPath, err)
	}
	title := kb.FirstHeading(kb.StripCodeFences(kb.StripFrontmatter(text)))
	v := Volume{Dir: w.volumeDir, Label: VolumeLabel(w.volume), Title: title}
	docs, err := w.spine(indexPath, text, 1)
	if err != nil {
		return Volume{}, err
	}
	v.Documents = docs
	return v, nil
}

// spine descends the child-list spine of the index at indexPath, rank being one
// below the index's own, and returns the documents it walked in the source's
// order. A child that is itself an index descends one rank deeper; every other
// named document joins the spine. A child outside the volume's directory is
// another volume's content or the entry point's; one naming no file, or one
// the export has already walked, is reported and does not join the spine.
func (w *walker) spine(indexPath, text string, rank int) ([]Document, error) {
	var out []Document
	for _, rel := range kb.ChildList(indexPath, text) {
		if path.Base(rel) == ReferencesFile {
			continue
		}
		if !kb.Within(w.src.KBPath(w.volumeDir), w.src.KBPath(rel)) {
			w.rep.Add(walkForm, indexPath, "the child %s resolves outside the volume directory %s, so it is not walked here", rel, w.volumeDir)
			continue
		}
		if !w.src.IsFile(w.src.KBPath(rel)) {
			w.rep.Add(walkForm, indexPath, "the child %s names no file in the KB, so it is not walked", rel)
			continue
		}
		if w.walked[rel] {
			w.rep.Add(walkForm, indexPath, "the child %s was already walked; the first walk stands", rel)
			continue
		}
		w.walked[rel] = true
		label := DocumentLabel(w.volume, w.volumeDir, rel)
		if !mint(w.rep, w.byLabel, label, rel) {
			label = ""
		}
		out = append(out, Document{Path: rel, Rank: rank, Label: label})
		if path.Base(rel) != kb.IndexFile {
			continue
		}
		childText, err := w.src.ReadText(w.src.KBPath(rel))
		if err != nil {
			return nil, fmt.Errorf("reading the index %s: %w", rel, err)
		}
		deep, err := w.spine(rel, childText, rank+1)
		if err != nil {
			return nil, err
		}
		out = append(out, deep...)
	}
	return out, nil
}

// DocumentLabel is the label minted for the document at the kb-root-relative
// slash path docPath in the volume whose directory is volumeDir, numbered n:
// d{N}-{path-slug}, the slug of docPath below volumeDir. The path below the
// volume is what the label names, so it is unique within the volume for every
// distinct path and holds a segment for every directory the document sits under
// — an index slugs as its own directory's name followed by "index". The d
// prefix keeps document labels apart from volume labels and author labels.
//
// A name whose every character slugs away contributes nothing, so two such
// paths in one volume mint one label; the walk reports the collision and the
// first path keeps it.
func DocumentLabel(n int, volumeDir, docPath string) string {
	return fmt.Sprintf("d%d-%s", n, fileSlug(below(volumeDir, docPath)))
}

// VolumeLabel is the label minted for the volume numbered n: vol{N}, unique by
// its number and apart from document and author labels by its prefix.
func VolumeLabel(n int) string {
	return fmt.Sprintf("vol%d", n)
}

// below is kbPath with the volume directory dir removed, kbPath itself where it
// is not under dir.
func below(dir, kbPath string) string {
	rel, ok := strings.CutPrefix(kbPath, dir+"/")
	if !ok {
		return kbPath
	}
	return rel
}

// fileSlug is a kb-root-relative slash path naming a file as a label slug: the
// final segment's extension dropped, then every segment slugged and joined with
// hyphens, so a dotted directory keeps every character it holds.
func fileSlug(p string) string {
	dir, name := path.Split(p)
	segments := append(strings.Split(strings.Trim(dir, "/"), "/"), strings.TrimSuffix(name, path.Ext(name)))
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		if slug := segmentSlug(segment); slug != "" {
			out = append(out, slug)
		}
	}
	return strings.Join(out, "-")
}

// segmentSlug is one path segment as a label slug: lowercased, every character
// outside [a-z0-9] one hyphen, runs collapsed, leading and trailing hyphens
// trimmed.
func segmentSlug(segment string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(segment) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			hyphen = false
		case !hyphen && b.Len() > 0:
			b.WriteByte('-')
			hyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// mint records label against p and reports whether p owns it: a label already
// naming another path is reported as a collision, and the first path keeps it.
func mint(rep *Report, byLabel map[string]string, label, p string) bool {
	if prev, ok := byLabel[label]; ok {
		rep.Add(labelForm, p, "label %s already names %s", label, prev)
		return false
	}
	byLabel[label] = p
	return true
}
