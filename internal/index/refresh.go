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
	checkMetadata = "metadata"
	checkKBRoot   = "kb-root"
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
// through; src is what locates a coverage refusal at the bullet that wrote it.
func refusal(src *kb.Source, err error) error {
	var m kb.MalformedError
	var c CycleError
	var cov CoverageError
	var ann *kb.AnnotationError
	if r, ok := symlinkRefusal(err); ok {
		return r
	}
	switch {
	case errors.As(err, &ann):
		return result.Refusal(AnnotationItems(ann))
	case errors.As(err, &cov):
		cov = coverageAt(src, cov)
		return result.Refusal{{Check: checkMetadata, Key: cov.Key, Path: cov.Path, Line: cov.Line, Detail: cov.Msg}}
	case errors.As(err, &m) || errors.As(err, &c):
		return result.Refusal{{Check: checkMetadata, Detail: err.Error()}}
	}
	return err
}

// precheck refuses a kb-root that is not a directory or whose CLAUDE.md is
// not the redirect.
func precheck(src *kb.Source) error {
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

var (
	strengthLineRE = kb.PyRE(`^(\s*)-\s*strength:`)
	annotationRE   = kb.PyRE(`\(strength\s+(?:-?\d+(?:\.\d+)?|\*pending\*)\)`)
)

// Refresh derives every derived field of the KB src reads and writes it: the
// subtree aggregates of every index node, every register entry's strength
// line, premise annotations and leaf-references footer, the .index/*.yaml
// files and the claim-graph sheets. It then saves: every file the format
// covers that src holds in a form the disk does not — the whole KB, where src
// was migrated — then removes the paths the migration made obsolete, less
// those the save wrote, then writes the entry point, stamped, last. It writes
// only files whose bytes change and returns their paths, kb-root-relative
// inside kb-root and absolute beside it. A KB state it refuses is a result.Refusal;
// earlier phases' writes stand, and a migrated KB is left unstamped.
func Refresh(src *kb.Source, lg log.Logger) ([]string, error) {
	written, _, _, err := RefreshReporting(src, lg)
	return written, err
}

// RefreshReporting is Refresh, and also what it removed — the sheets its
// sheet phase no longer drew and the obsolete paths it drained,
// kb-root-relative inside kb-root and repository-relative beside it — and
// whether it ran with no Graphviz dot on PATH, drawing nothing.
func RefreshReporting(src *kb.Source, lg log.Logger) (written, removed []string, noDot bool, err error) {
	if err := precheck(src); err != nil {
		return nil, nil, false, err
	}
	r := refresher{src: src}
	for _, phase := range []func() error{r.aggregates, r.strength, r.leafReferences, func() error { return r.emit(lg) }, r.sheet, r.save} {
		if err := phase(); err != nil {
			return r.written, r.removed, r.noDot, refusal(src, err)
		}
	}
	return r.written, r.removed, r.noDot, nil
}

type refresher struct {
	src     *kb.Source
	written []string
	// removed is what the sheet phase and the drain removed; noDot is whether
	// the sheet phase found no Graphviz dot to draw with.
	removed []string
	noDot   bool
}

func (r *refresher) path(rel string) string { return r.src.KBPath(rel) }

// write replaces a document's text, keeping its mode. The entry point is
// held for the save, which writes it once, stamped, last.
func (r *refresher) write(rel, text string) error {
	p := r.path(rel)
	if rel == kb.EntryPointFile {
		r.src.Put(p, []byte(text))
		return nil
	}
	if err := WriteKBFile(r.src, p, []byte(text)); err != nil {
		return err
	}
	r.written = append(r.written, rel)
	return nil
}

// WriteKBFile replaces the KB file at p with data, atomically and keeping
// its mode, and records the bytes in src so later reads see them. A file
// beside kb-root — a build record — is written under the repository; every
// other is written under kb-root, and a symbolic link at p or a target
// outside either tree is refused.
func WriteKBFile(src *kb.Source, p string, data []byte) error {
	root := src.Root()
	if !kb.Within(root, p) {
		root = src.Repo()
	}
	if err := writeInside(root, p, data); err != nil {
		return err
	}
	src.Put(p, data)
	return nil
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

func (r *refresher) aggregates() error {
	st, err := kb.Discover(r.src, log.Discard())
	if err != nil {
		return err
	}
	agg := SubtreeAggregates(st)
	for _, idx := range st.Indexes {
		text, err := r.src.ReadText(r.path(idx.Path))
		if err != nil {
			return err
		}
		a := agg[idx.Path]
		next := kb.ReplaceOrInsertFrontmatterField(text, "subtree-claims", a.Claims, "kind:")
		anchor := "subtree-claims:"
		if !strings.Contains(next, anchor) {
			anchor = "kind:"
		}
		next = kb.ReplaceOrInsertFrontmatterField(next, "subtree-experiments", a.Experiments, anchor)
		if next != text {
			if err := r.write(idx.Path, next); err != nil {
				return err
			}
		}
	}
	return nil
}

// editableLines is a file's text split on \n, less the empty piece a final
// newline leaves, and whether it had one.
func editableLines(text string) ([]string, bool) {
	lines := strings.Split(text, "\n")
	final := strings.HasSuffix(text, "\n")
	if final {
		lines = lines[:len(lines)-1]
	}
	return lines, final
}

func joinLines(lines []string, final bool) string {
	text := strings.Join(lines, "\n")
	if final {
		text += "\n"
	}
	return text
}

func lineAt(lines []string, i int, rel string) (string, error) {
	if i < 0 || i >= len(lines) {
		return "", fmt.Errorf("%s: line %d is past the register's %d lines", rel, i+1, len(lines))
	}
	return lines[i], nil
}

func (r *refresher) strength() error {
	st, err := kb.Discover(r.src, log.Discard())
	if err != nil {
		return err
	}
	strength, err := ComputeStrength(st)
	if err != nil {
		return err
	}
	byFile := map[string][]kb.ClaimEntry{}
	for _, e := range st.ClaimEntries {
		byFile[e.CanonicalPath] = append(byFile[e.CanonicalPath], e)
	}
	regs, err := kb.Registers(r.src)
	if err != nil {
		return err
	}
	for _, rel := range regs {
		if _, ok := byFile[rel]; !ok {
			byFile[rel] = nil
		}
	}
	files := make([]string, 0, len(byFile))
	for rel := range byFile {
		files = append(files, rel)
	}
	slices.Sort(files)
	for _, rel := range files {
		if err := r.rewriteStrength(rel, byFile[rel], st.Analyses, strength); err != nil {
			return err
		}
	}
	return nil
}

func (r *refresher) rewriteStrength(rel string, entries []kb.ClaimEntry, analyses []kb.AnalysisNode, strength Strength) error {
	text, err := r.src.ReadText(r.path(rel))
	if err != nil {
		return err
	}
	lines, final := editableLines(text)
	joined := strings.Join(lines, "\n")
	finals := strength.Finals()
	analysisResults := strength.AnalysisResults
	byID := map[string]kb.ClaimEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	analysisByID := map[string]kb.AnalysisNode{}
	for _, s := range analyses {
		analysisByID[s.ID] = s
	}
	var order []string
	ranges := map[string][2]int{}
	for _, loc := range kb.LocateEntries(joined) {
		if loc.RigorStart < 0 {
			continue
		}
		if _, ok := ranges[loc.NodeID]; !ok {
			order = append(order, loc.NodeID)
		}
		ranges[loc.NodeID] = [2]int{loc.RigorStart, loc.RigorEnd}
	}
	scrubbed := kb.SplitLines(kb.StripCodeFences(joined))
	for _, id := range order {
		entry, isClaim := byID[id]
		_, isAnalysis := analysisByID[id]
		if !isClaim && !isAnalysis {
			continue
		}
		var res Result
		if isClaim {
			res = strength.Results[entry.ID]
		} else {
			res = analysisResults[id]
		}
		span := ranges[id]
		for idx := span[0]; idx < span[1]; idx++ {
			line, err := lineAt(lines, idx, rel)
			if err != nil {
				return err
			}
			probe := scrubbed[idx]
			if strengthLineRE.MatchString(probe) {
				lines[idx] = strengthLine(res.Final, RenderStrengthTrace(res))
				continue
			}
			if !strings.Contains(probe, "(strength") {
				continue
			}
			targets := kb.ClaimIDs(kb.BulletHead(kb.TrimBulletLead(kb.Strip(probe))))
			if len(targets) == 0 {
				continue
			}
			var source *float64
			if v, ok := finals[targets[0]]; ok {
				source = ptr(v)
			}
			if loc := annotationRE.FindStringIndex(line); loc != nil {
				lines[idx] = line[:loc[0]] + kb.RenderStrengthAnnotation(FormatStrengthRecord(source)) + line[loc[1]:]
			}
		}
	}
	if next := joinLines(lines, final); next != text {
		return r.write(rel, next)
	}
	return nil
}

func strengthLine(v *float64, trace string) string {
	if v == nil {
		return kb.RenderStrengthLine(nil, "", "")
	}
	value := FormatStrengthRecord(v)
	return kb.RenderStrengthLine(&value, *BuildStatusPhrase(v), trace)
}

func (r *refresher) leafReferences() error {
	st, err := kb.Discover(r.src, log.Discard())
	if err != nil {
		return err
	}
	refs := LeafReferences(st)
	regs, err := kb.Registers(r.src)
	if err != nil {
		return err
	}
	for _, rel := range regs {
		text, err := r.src.ReadText(r.path(rel))
		if err != nil {
			return err
		}
		lines, final := editableLines(text)
		bands := kb.LocateLeafReferenceFooters(text)
		slices.SortFunc(bands, func(a, b kb.FooterBand) int { return b.BodyStart - a.BodyStart })
		for _, band := range bands {
			footer := RenderLeafReferences(rel, refs[band.NodeID])
			if band.FooterLine >= 0 {
				if _, err := lineAt(lines, band.FooterLine, rel); err != nil {
					return err
				}
				lines[band.FooterLine] = footer
				continue
			}
			at := band.RigorStart
			block := []string{footer, ""}
			if at > band.BodyStart {
				prev, err := lineAt(lines, at-1, rel)
				if err != nil {
					return err
				}
				if kb.Strip(prev) != "" {
					block = []string{"", footer, ""}
				}
			}
			lines = slices.Insert(lines, min(at, len(lines)), block...)
		}
		if next := joinLines(lines, final); next != text {
			if err := r.write(rel, next); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *refresher) emit(lg log.Logger) error {
	dir := r.path(kb.IndexDir)
	if err := os.Mkdir(dir, 0o777); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	st, err := kb.Discover(r.src, lg)
	if err != nil {
		return err
	}
	records, err := BuildRecords(st)
	if err != nil {
		return err
	}
	for _, name := range kb.IndexFiles {
		body := Serialize(records[name])
		rel := kb.IndexDir + "/" + kb.IndexFileName(name)
		target := r.path(rel)
		if current, err := r.src.DiskFile(target); err == nil && string(current) == body {
			r.src.Put(target, current)
			continue
		}
		if err := WriteKBFile(r.src, target, []byte(body)); err != nil {
			return err
		}
		r.written = append(r.written, rel)
	}
	return nil
}

func (r *refresher) sheet() error {
	written, removed, drawn, err := WriteSheet(r.src)
	r.written = append(r.written, written...)
	r.removed = append(r.removed, removed...)
	r.noDot = err == nil && !drawn
	return err
}

// save writes every migrated file the disk does not hold as the source does,
// but the entry point; removes the obsolete paths, less those it or an
// earlier phase wrote; then stamps the entry point and writes it, last.
func (r *refresher) save() error {
	entry := r.path(kb.EntryPointFile)
	saved := map[string]bool{}
	for _, w := range r.written {
		if filepath.IsAbs(w) {
			saved[w] = true
		} else {
			saved[r.path(w)] = true
		}
	}
	for _, rel := range r.src.MigratedFiles() {
		p := r.src.Path(rel)
		if p == entry {
			continue
		}
		saved[p] = true
		if err := r.saveFile(p); err != nil {
			return err
		}
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
		r.removed = append(r.removed, r.reported(p, true))
	}
	text, err := r.src.ReadText(entry)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	stamped, err := kb.StampFormat(text, kb.FormatVersion)
	if err != nil {
		return err
	}
	r.src.Put(entry, []byte(stamped))
	if err := r.saveFile(entry); err != nil {
		return err
	}
	r.src.Saved()
	return nil
}

// saveFile writes p as the source holds it, where the disk holds other
// bytes.
func (r *refresher) saveFile(p string) error {
	data, err := r.src.ReadFile(p)
	if err != nil {
		return err
	}
	if current, err := r.src.DiskFile(p); err == nil && bytes.Equal(current, data) {
		return nil
	}
	if err := WriteKBFile(r.src, p, data); err != nil {
		return err
	}
	r.written = append(r.written, r.reported(p, false))
	return nil
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

// WriteSheet draws the KB's claim-graph sheets and writes each whose bytes
// change, then removes the digest and every volume's sheet the drawing no
// longer calls for. It returns the kb-root-relative paths it wrote and
// removed and whether the sheets were drawn. Without Graphviz dot on PATH
// nothing is drawn: every sheet standing is left as it is, the placeholder is
// written only where kb-root has no claim-graph.svg, and nothing is removed.
func WriteSheet(src *kb.Source) (written, removed []string, drawn bool, err error) {
	kbRoot := src.Root()
	sheets, err := sheet.Render(src)
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
