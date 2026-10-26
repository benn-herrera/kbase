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

// The refusal classes refresh and verify raise.
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

// Precheck is what refresh refuses before it writes: a kb-root that is not a
// directory or whose CLAUDE.md is not the redirect.
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

// Refresh renders the KB src reads: where its load is degraded it first
// writes the records as the walk resolves them. Where src was migrated it then
// saves every file the conversion produced, and every file put in src since,
// that the disk does not hold as src does: the edge records, the node records,
// every other file but the stamped documents and the entry point, then the
// stamped documents. It then writes every register page the KB calls for and
// the claim-graph sheets, removing a page no longer called for; writes the
// format stamp file at the version the KB is still at; removes the paths the
// migration made obsolete, less those it wrote; writes the entry point; and
// last rewrites the stamp file at kb.FormatVersion. It writes only files whose
// bytes change and returns their paths, kb-root-relative inside kb-root and
// absolute beside it. A KB state it refuses is a result.Refusal; earlier
// phases' writes stand, and a save that stops part-way leaves the KB stamped
// at the version it converts from, which the next refresh completes.
func Refresh(src *kb.Source, lg log.Logger) ([]string, error) {
	written, _, _, err := RefreshReporting(src, lg)
	return written, err
}

// RefreshReporting is Refresh, and also what it removed — the register pages
// and sheets no longer called for and the obsolete paths it drained,
// kb-root-relative inside kb-root and repository-relative beside it — and
// whether it ran with no Graphviz dot on PATH, drawing nothing.
func RefreshReporting(src *kb.Source, lg log.Logger) (written, removed []string, noDot bool, err error) {
	r, err := RefreshWithWarnings(src, lg)
	return r.Written, r.Removed, r.NoDot, err
}

// Refreshed is what one refresh did: the paths it wrote and removed, as
// RefreshReporting names them, whether it drew nothing for want of Graphviz
// dot, and a warning per record its repair of a degraded load dropped or edge
// it cut.
type Refreshed struct {
	Written, Removed []string
	NoDot            bool
	Warnings         []result.Item
}

// RefreshWithWarnings is RefreshReporting, and also the repair's warnings.
func RefreshWithWarnings(src *kb.Source, lg log.Logger) (Refreshed, error) {
	if err := Precheck(src); err != nil {
		return Refreshed{}, err
	}
	r := refresher{src: src}
	for _, phase := range []func() error{r.repair, r.saveConverted, r.pages, r.sheet, r.save} {
		if err := phase(); err != nil {
			return r.Refreshed, refusal(err)
		}
	}
	return r.Refreshed, nil
}

type refresher struct {
	src *kb.Source
	Refreshed
}

func (r *refresher) path(rel string) string { return r.src.KBPath(rel) }

// repair writes the records as the walk resolves them where the load is
// degraded, keeping its warnings; a load that is not writes nothing. The
// records written are the authored ones under the same resolution, so a rigor
// the walk reads clamped keeps its value in the file. The repair runs first so
// the pages and the sheets are drawn from the records as they then stand.
func (r *refresher) repair() error {
	rows, err := Derive(r.src)
	if err != nil {
		return err
	}
	if len(rows.Degraded) == 0 {
		return nil
	}
	authored, err := kb.LoadAuthoredGraph(r.src)
	if err != nil {
		return err
	}
	hosted, err := HostedOrder(r.src, authored)
	if err != nil {
		return err
	}
	resolved, _ := resolveGraph(authored, GraphKinds(authored), hosted)
	written, err := WriteGraph(r.src, resolved)
	r.Written = append(r.Written, written...)
	if err != nil {
		return err
	}
	r.Warnings = rows.Degraded
	return nil
}

// pages writes every register page the KB calls for whose bytes on disk
// differ, and removes each page standing at a page position that is no longer
// called for.
func (r *refresher) pages() error {
	x, err := loadDerivation(r.src)
	if err != nil {
		return err
	}
	pages := x.registerPages()
	for _, rel := range x.pagePositions() {
		p := r.path(rel)
		data, called := pages[rel]
		if !called {
			if !r.src.IsFile(p) {
				continue
			}
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			r.src.Removed(p)
			r.Removed = append(r.Removed, rel)
			continue
		}
		if current, err := r.src.DiskFile(p); err == nil && bytes.Equal(current, data) {
			r.src.Put(p, current)
			continue
		}
		if err := WriteKBFile(r.src, p, data); err != nil {
			return err
		}
		r.Written = append(r.Written, rel)
	}
	return nil
}

// WriteKBFile replaces the KB file at p with data, atomically and keeping
// its mode, making its directory where none stands, and records the bytes in
// src so later reads see them. A file beside kb-root — a build record — is
// written under the repository; every other is written under kb-root, and a
// symbolic link at p or a target outside either tree is refused.
func WriteKBFile(src *kb.Source, p string, data []byte) error {
	root := src.Root()
	if !kb.Within(root, p) {
		root = src.Repo()
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := writeInside(root, p, data); err != nil {
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
	written, removed, drawn, err := WriteSheet(r.src)
	r.Written = append(r.Written, written...)
	r.Removed = append(r.Removed, removed...)
	r.NoDot = err == nil && !drawn
	return err
}

// saveConverted writes every file a migrated source holds that the disk does
// not hold as it does, but the entry point, in the order a stopped save is
// resumed from: the edge records, the node records, every other file, then
// the readiness-stamped documents. The entry point waits for save, where it is
// written after the stamp file stands at the version the tree is still at. A
// source read at the current format has none.
func (r *refresher) saveConverted() error {
	entry := r.path(kb.EntryPointFile)
	rank := func(p string) int {
		switch {
		case p == r.path(kb.GraphEdgesFile):
			return 0
		case p == r.path(kb.GraphNodesFile):
			return 1
		case filepath.Dir(p) == r.src.Root() && slices.Contains(kb.StampedFiles, filepath.Base(p)):
			return 3
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
// resumes and the next refresh completes.
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
		if filepath.IsAbs(w) {
			saved[w] = true
		} else {
			saved[r.path(w)] = true
		}
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
		r.Removed = append(r.Removed, r.reported(p, true))
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
	r.Written = append(r.Written, r.reported(p, false))
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

// reported is p as a result names it: kb-root-relative inside kb-root;
// beside it, repository-relative for a removal and absolute for a write.
func (r *refresher) reported(p string, removal bool) string {
	if rel, err := filepath.Rel(r.src.Root(), p); err == nil && kb.Within(r.src.Root(), p) {
		return filepath.ToSlash(rel)
	}
	if rel, ok := r.src.RepoRel(p); ok && removal {
		return rel
	}
	return p
}

// WriteSheet draws the KB's claim-graph sheets from the rows Derive yields and
// writes each whose bytes change, then removes the digest and every volume's
// sheet the drawing no longer calls for. A KB state Derive refuses is its
// refusal, dot or no dot. It returns the kb-root-relative paths it wrote and
// removed and whether the sheets were drawn. Without Graphviz dot on PATH
// nothing is drawn: every sheet standing is left as it is, the placeholder is
// written only where kb-root has no claim-graph.svg, and nothing is removed.
func WriteSheet(src *kb.Source) (written, removed []string, drawn bool, err error) {
	kbRoot := src.Root()
	rows, err := Derive(src)
	if err != nil {
		return nil, nil, false, err
	}
	sheets, err := sheet.Render(src, rows.Nodes, rows.Edges, rows.Cites)
	if errors.Is(err, sheet.ErrNoDot) {
		target := filepath.Join(kbRoot, kb.ClaimGraphFile)
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			return nil, nil, false, err
		}
		svg, err := sheet.Placeholder(src)
		if err != nil {
			return nil, nil, false, err
		}
		if err := writeInside(kbRoot, target, svg); err != nil {
			return nil, nil, false, err
		}
		return []string{kb.ClaimGraphFile}, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	drawnPaths := map[string]bool{}
	for _, s := range sheets {
		drawnPaths[s.Path] = true
		target := filepath.Join(kbRoot, filepath.FromSlash(s.Path))
		current, err := src.DiskFile(target)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return written, nil, true, err
		}
		if bytes.Equal(s.SVG, current) {
			continue
		}
		if err := writeInside(kbRoot, target, s.SVG); err != nil {
			return written, nil, true, err
		}
		written = append(written, s.Path)
	}
	removed, err = removeUndrawnSheets(src, drawnPaths)
	return written, removed, true, err
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

// removeUndrawnSheets removes every sheet present that drawn does not hold,
// returning their kb-root-relative paths in path order.
func removeUndrawnSheets(src *kb.Source, drawn map[string]bool) ([]string, error) {
	_, present, err := SheetPaths(src)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, rel := range present {
		if drawn[rel] {
			continue
		}
		if err := os.Remove(src.KBPath(rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, rel)
	}
	return removed, nil
}
