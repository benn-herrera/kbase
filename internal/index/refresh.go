package index

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/atomicfile"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/sheet"
)

// The refusal classes a refresh, update-format and verify raise.
const (
	checkMetadata = kb.CheckMetadata
	checkKBRoot   = kb.CheckKBRoot
)

// symlinkRefusal is err as the KB's symlink refusal, and false where err is
// not one.
func symlinkRefusal(err error) (result.Refusal, bool) {
	var sym *kb.SymlinkError
	if !errors.As(err, &sym) {
		return nil, false
	}
	return result.Refusal{{Check: kb.CheckSymlink, Path: sym.Path, Detail: err.Error()}}, true
}

// refusal turns an input-caused error into a result.Refusal and passes any other
// through.
func refusal(err error) error {
	var m kb.MalformedError
	var graph *kb.GraphLoadError
	if r, ok := symlinkRefusal(err); ok {
		return r
	}
	switch {
	case errors.As(err, &graph):
		return result.Refusal(graphItems(graph))
	case errors.As(err, &m):
		return result.Refusal{{Check: checkMetadata, Detail: err.Error()}}
	}
	return err
}

// Precheck is what a refresh and update-format refuse before they write: a
// kb-root that is not a directory or whose CLAUDE.md is not the redirect.
func Precheck(src *kb.Source) error {
	kbRoot := src.Root()
	if info, err := os.Stat(kbRoot); err != nil || !info.IsDir() {
		return result.Refusal{{Check: checkKBRoot, Path: kbRoot, Detail: fmt.Sprintf("KB directory %s not found", kbRoot)}}
	}
	msg, err := kb.UnmigratedAgentsFile(src)
	if err != nil {
		return err
	}
	if msg != "" {
		return result.Refusal{{Check: kb.UnmigratedAgentsCheck, Path: kb.AgentsRedirectFile, Detail: msg}}
	}
	return nil
}

// Refresh renders the KB src reads, which its caller has read at the current
// format: a write over a KB whose conversion is pending is refused before it
// (PendingConversion). A KB whose load fails is refused before anything is
// written. It writes every register page the KB calls for and the claim-graph
// sheets, removing a page no longer called for. It writes only files whose
// bytes change and returns their kb-root-relative paths. A KB state it refuses
// is a result.Refusal; earlier phases' writes stand.
func Refresh(src *kb.Source, lg log.Logger) ([]string, error) {
	written, _, _, err := RefreshReporting(src, lg)
	return written, err
}

// RefreshReporting is Refresh, and also what it removed — the register pages
// and sheets no longer called for, kb-root-relative — and whether it ran with
// no Graphviz dot on PATH, drawing nothing.
func RefreshReporting(src *kb.Source, lg log.Logger) (written, removed []string, noDot bool, err error) {
	r := refresher{src: src}
	return r.run(r.load, r.pages, r.sheet)
}

// Derivation is what the derivation would change over the KB a source reads:
// each file whose bytes differ from the disk's, kb-root-relative, the register
// pages then the sheets; each file it would remove, kb-root-relative; and
// whether it draws the sheets, which it does not without Graphviz dot.
type Derivation struct {
	Files   []DerivedFile
	Removed []string
	Drawn   bool
}

// DerivedFile is one file the derivation writes: its kb-root-relative path
// and its bytes.
type DerivedFile struct {
	Rel  string
	Data []byte
}

// PlanDerivation is the derivation over the KB src reads, its records and
// leaves as src holds them, computed and nothing written. Its caller has
// prechecked src. A KB state the derivation refuses is a result.Refusal.
func PlanDerivation(src *kb.Source) (Derivation, error) {
	if _, err := Derive(src); err != nil {
		return Derivation{}, refusal(err)
	}
	files, removed, err := planPages(src)
	if err != nil {
		return Derivation{}, refusal(err)
	}
	sheets, gone, drawn, err := planSheet(src)
	if err != nil {
		return Derivation{}, refusal(err)
	}
	return Derivation{Files: append(files, sheets...), Removed: append(removed, gone...), Drawn: drawn}, nil
}

// RemoveKBFile removes the KB file at p, where it stands, and records in src
// that it is gone.
func RemoveKBFile(src *kb.Source, p string) error {
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	src.Removed(p)
	return nil
}

// RecordMove is one build record the conversion moves from the repository
// root into kb-root/.build/, by repository-relative path.
type RecordMove struct{ From, To string }

// UpdateFormat is update-format's save and re-derivation over the KB src
// reads. A KB whose load fails is refused before anything is written or
// moved. Once the load passes, it moves each of moves with move — the
// caller's git mv — reporting the record's root path among the removed,
// repository-relative, and its new path among the written; the save itself
// moves no record. Where src was migrated it then saves every file the
// conversion produced that the disk does not hold as src does: the edge
// records, the node records, then every other file but the entry point. It
// then re-derives as Refresh does; writes the format stamp file at the version
// the KB is still at; removes the paths the migration made obsolete, less
// those it wrote; writes the entry point; and last rewrites the stamp file at
// kb.FormatVersion. It writes only files whose bytes change and reports as
// RefreshReporting does. A save that stops part-way leaves the KB stamped at
// the version it converts from.
func UpdateFormat(src *kb.Source, moves []RecordMove, move func(from, to string) error, lg log.Logger) (written, removed []string, noDot bool, err error) {
	r := refresher{src: src}
	return r.run(r.load, func() error { return r.moveRecords(moves, move) }, r.saveConverted, r.pages, r.sheet, r.save)
}

// moveRecords moves each of moves with move, reporting each as UpdateFormat
// does. The moved bytes are the ones src holds at the new path, so the save
// finds each record in place and its root path gone.
func (r *refresher) moveRecords(moves []RecordMove, move func(from, to string) error) error {
	for _, m := range moves {
		if err := move(m.From, m.To); err != nil {
			return err
		}
		r.Removed = append(r.Removed, m.From)
		r.Written = append(r.Written, r.reported(r.src.Path(m.To)))
	}
	return nil
}

// run is the phases in order after the precheck, stopping at the first that
// refuses or fails.
func (r *refresher) run(phases ...func() error) (written, removed []string, noDot bool, err error) {
	if err := Precheck(r.src); err != nil {
		return nil, nil, false, err
	}
	for _, phase := range phases {
		if err := phase(); err != nil {
			return r.Written, r.Removed, r.NoDot, refusal(err)
		}
	}
	return r.Written, r.Removed, r.NoDot, nil
}

// refresher is one refresh or update-format over src: the paths it wrote and removed, as
// RefreshReporting names them, and whether it drew nothing for want of
// Graphviz dot.
type refresher struct {
	src              *kb.Source
	Written, Removed []string
	NoDot            bool
}

func (r *refresher) path(rel string) string { return r.src.KBPath(rel) }

// load refuses a KB whose load fails before any phase writes or moves.
func (r *refresher) load() error {
	_, err := Derive(r.src)
	return err
}

// pages writes every register page the KB calls for whose bytes on disk
// differ, and removes each page standing at a page position that is no longer
// called for.
func (r *refresher) pages() error {
	files, removed, err := planPages(r.src)
	if err != nil {
		return err
	}
	return r.apply(files, removed)
}

// apply writes each of files and removes each of removed, reporting each.
func (r *refresher) apply(files []DerivedFile, removed []string) error {
	for _, f := range files {
		if err := WriteKBFile(r.src, r.path(f.Rel), f.Data); err != nil {
			return err
		}
		r.Written = append(r.Written, f.Rel)
	}
	for _, rel := range removed {
		if err := RemoveKBFile(r.src, r.path(rel)); err != nil {
			return err
		}
		r.Removed = append(r.Removed, rel)
	}
	return nil
}

// planPages is every register page the KB src reads calls for whose bytes
// differ from the disk's, and each page standing at a page position no longer
// called for. A page position the disk cannot be read at is an error.
func planPages(src *kb.Source) (files []DerivedFile, removed []string, err error) {
	x, err := loadDerivation(src)
	if err != nil {
		return nil, nil, err
	}
	pages := x.registerPages()
	for _, rel := range x.pagePositions() {
		p := src.KBPath(rel)
		data, called := pages[rel]
		if !called {
			if src.IsFile(p) {
				removed = append(removed, rel)
			}
			continue
		}
		current, err := src.DiskFile(p)
		if err == nil && bytes.Equal(current, data) {
			src.Put(p, current)
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
		files = append(files, DerivedFile{rel, data})
	}
	return files, removed, nil
}

// WriteKBFile replaces the KB file at p with data, atomically and keeping
// its mode, making its directory where none stands, and records the bytes in
// src so later reads see them. A symbolic link at p or a target outside
// kb-root is refused.
func WriteKBFile(src *kb.Source, p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := writeInside(src.Root(), p, data); err != nil {
		return err
	}
	src.Put(p, data)
	return nil
}

// WriteGraph writes g's record files through WriteKBFile — the edge file, then
// the node file — each only where its bytes change, and is the kb-root-relative
// paths it wrote.
func WriteGraph(src *kb.Source, g kb.Graph) ([]string, error) {
	var written []string
	for _, f := range []struct {
		rel  string
		data []byte
	}{{kb.GraphEdgesFile, kb.EncodeEdges(g)}, {kb.GraphNodesFile, kb.EncodeNodes(g)}} {
		p := src.KBPath(f.rel)
		old, err := src.ReadFile(p)
		if err != nil && !kb.ErrNotExist(err) {
			return written, err
		}
		if err == nil && bytes.Equal(old, f.data) {
			continue
		}
		if err := WriteKBFile(src, p, f.data); err != nil {
			return written, err
		}
		written = append(written, f.rel)
	}
	return written, nil
}

// writeInside replaces p with data atomically, refusing a path that is a
// symbolic link or resolves outside root as the KB's symlink refusal.
func writeInside(root, p string, data []byte) error {
	err := atomicfile.WriteIn(root, p, data)
	var link *atomicfile.LinkError
	if errors.As(err, &link) {
		return result.Refusal{{Check: kb.CheckSymlink, Path: p, Detail: link.Detail}}
	}
	return err
}

func strengthLine(v *float64, trace string) string {
	if v == nil {
		return kb.RenderStrengthLine(nil, "", "")
	}
	value := FormatStrengthRecord(v)
	return kb.RenderStrengthLine(&value, *BuildStatusPhrase(v), trace)
}

func (r *refresher) sheet() error {
	files, removed, drawn, err := planSheet(r.src)
	if err != nil {
		return err
	}
	r.NoDot = !drawn
	return r.apply(files, removed)
}

// saveConverted writes every file a migrated source holds that the disk does
// not hold as it does, but the entry point, in the order a stopped save is
// resumed from: the edge records, the node records, then every other file.
// The entry point waits for save, where it is written after the stamp file
// stands at the version the tree is still at. A source read at the current
// format has none.
func (r *refresher) saveConverted() error {
	entry := r.path(kb.EntryPointFile)
	rank := func(p string) int {
		switch p {
		case r.path(kb.GraphEdgesFile):
			return 0
		case r.path(kb.GraphNodesFile):
			return 1
		}
		return 2
	}
	var paths []string
	for _, rel := range r.src.MigratedFiles() {
		if p := r.src.Path(rel); p != entry {
			paths = append(paths, p)
		}
	}
	slices.SortStableFunc(paths, func(a, b string) int { return rank(a) - rank(b) })
	for _, p := range paths {
		if err := r.saveFile(p); err != nil {
			return err
		}
	}
	return nil
}

// save stamps a migrated KB at the version it is still at, removes the
// obsolete paths, less every migrated file and every path an earlier phase
// wrote, writes the entry point, and last rewrites the stamp file at
// kb.FormatVersion. The stamp file stands throughout and always names the
// version the tree is at, and a save that stops part-way leaves a KB the
// loader reads at its old version — the stamp file, or the entry point's
// frontmatter until the stamp file is written, saying so — which the converter
// resumes.
func (r *refresher) save() error {
	entry := r.path(kb.EntryPointFile)
	format := r.path(kb.FormatFile)
	if m, ok := r.src.Migration(); ok {
		r.src.Put(format, kb.EncodeFormatStamp(m.From))
		if _, err := r.writeChanged(format); err != nil {
			return err
		}
	}
	saved := map[string]bool{}
	for _, w := range r.Written {
		saved[r.path(w)] = true
	}
	for _, rel := range r.src.MigratedFiles() {
		saved[r.src.Path(rel)] = true
	}
	for _, rel := range r.src.Obsolete() {
		p := r.src.Path(rel)
		if saved[p] {
			continue
		}
		if err := os.Remove(p); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		r.Removed = append(r.Removed, r.reported(p))
	}
	if _, err := r.src.ReadText(entry); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := r.saveFile(entry); err != nil {
		return err
	}
	r.src.Put(format, kb.EncodeFormatStamp(kb.FormatVersion))
	if err := r.saveFile(format); err != nil {
		return err
	}
	r.src.Saved()
	return nil
}

// saveFile writes p as the source holds it, where the disk holds other bytes,
// and reports it among what this refresh wrote.
func (r *refresher) saveFile(p string) error {
	wrote, err := r.writeChanged(p)
	if err != nil || !wrote {
		return err
	}
	r.Written = append(r.Written, r.reported(p))
	return nil
}

// writeChanged writes p as the source holds it, where the disk holds other
// bytes, and is whether it wrote.
func (r *refresher) writeChanged(p string) (bool, error) {
	data, err := r.src.ReadFile(p)
	if err != nil {
		return false, err
	}
	if current, err := r.src.DiskFile(p); err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err := WriteKBFile(r.src, p, data); err != nil {
		return false, err
	}
	return true, nil
}

// reported is p, a path inside kb-root, as a result names it: kb-root-relative.
func (r *refresher) reported(p string) string {
	rel, err := filepath.Rel(r.src.Root(), p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

// planSheet is the KB's claim-graph sheets drawn from the rows Derive yields,
// nothing written: each sheet whose bytes differ from the disk's, the digest
// and every volume's sheet the drawing no longer calls for, kb-root-relative,
// and whether the sheets were drawn. A KB state Derive refuses is its refusal,
// dot or no dot. Without Graphviz dot on PATH nothing is drawn: every sheet
// standing is left as it is, the placeholder is written only where kb-root
// has no claim-graph.svg, and nothing is removed.
func planSheet(src *kb.Source) (files []DerivedFile, removed []string, drawn bool, err error) {
	rows, err := Derive(src)
	if err != nil {
		return nil, nil, false, err
	}
	sheets, err := sheet.Render(src, rows.Nodes, rows.Edges, rows.Cites)
	if errors.Is(err, sheet.ErrNoDot) {
		if _, err := os.Stat(src.KBPath(kb.ClaimGraphFile)); !errors.Is(err, os.ErrNotExist) {
			return nil, nil, false, err
		}
		svg, err := sheet.Placeholder(src)
		if err != nil {
			return nil, nil, false, err
		}
		return []DerivedFile{{kb.ClaimGraphFile, svg}}, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	drawnPaths := map[string]bool{}
	for _, s := range sheets {
		drawnPaths[s.Path] = true
		current, err := src.DiskFile(src.KBPath(s.Path))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, true, err
		}
		if !bytes.Equal(s.SVG, current) {
			files = append(files, DerivedFile{s.Path, s.SVG})
		}
	}
	_, present, err := SheetPaths(src)
	if err != nil {
		return nil, nil, true, err
	}
	for _, rel := range present {
		if !drawnPaths[rel] {
			removed = append(removed, rel)
		}
	}
	return files, removed, true, nil
}

// SheetPaths is every kb-root-relative slash path a claim-graph sheet of the
// KB at src can stand at — kb-root's sheet and digest, and each top-level
// directory's sheet — and those of them present, both in path order.
func SheetPaths(src *kb.Source) (candidates, present []string, err error) {
	candidates = []string{kb.ClaimGraphFile, kb.ClaimGraphDigestFile}
	entries, err := os.ReadDir(src.Root())
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			candidates = append(candidates, e.Name()+"/"+kb.ClaimGraphFile)
		}
	}
	slices.Sort(candidates)
	for _, rel := range candidates {
		if src.IsFile(src.KBPath(rel)) {
			present = append(present, rel)
		}
	}
	return candidates, present, nil
}
