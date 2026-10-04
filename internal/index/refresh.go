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

// Refusal is a KB state refresh, verify or a query will not proceed over,
// every offending item named.
type Refusal struct{ Items []result.Item }

func (r Refusal) Error() string { return result.Items(r.Items).Error() }

// The refusal classes refresh and verify raise.
const (
	checkMetadata = "metadata"
	checkKBRoot   = "kb-root"
)

// refusal turns an input-caused error into a Refusal and passes any other
// through.
func refusal(err error) error {
	var m kb.MalformedError
	var c CycleError
	var cov CoverageError
	if errors.As(err, &m) || errors.As(err, &c) || errors.As(err, &cov) {
		return Refusal{[]result.Item{{Check: checkMetadata, Detail: err.Error()}}}
	}
	return err
}

// precheck refuses a kb-root that is not a directory or whose CLAUDE.md is
// not the redirect.
func precheck(kbRoot string) error {
	if info, err := os.Stat(kbRoot); err != nil || !info.IsDir() {
		return Refusal{[]result.Item{{Check: checkKBRoot, Path: kbRoot, Detail: fmt.Sprintf("KB directory %s not found", kbRoot)}}}
	}
	msg, err := kb.UnmigratedAgentsFile(kbRoot)
	if err != nil {
		return err
	}
	if msg != "" {
		return Refusal{[]result.Item{{Check: kb.UnmigratedAgentsCheck, Path: kb.AgentsRedirectFile, Detail: msg}}}
	}
	return nil
}

var (
	solidityLineRE = kb.PyRE(`^(\s*)-\s*solidity:`)
	annotationRE   = kb.PyRE(`\(solidity\s+(?:-?\d+(?:\.\d+)?|\*pending\*)\)`)
)

// Refresh derives every derived field of the KB at kbRoot and writes it: the
// subtree aggregates of every index node, every register entry's solidity
// line, depends-on annotations and leaf-references footer, the .index/*.jsonl
// files and the placeholder sheet. It writes only files whose bytes change
// and returns their kb-root-relative paths. A KB state it refuses is a
// Refusal; earlier phases' writes stand.
func Refresh(kbRoot string, lg log.Logger) ([]string, error) {
	if err := precheck(kbRoot); err != nil {
		return nil, err
	}
	r := refresher{root: kbRoot}
	for _, phase := range []func() error{r.aggregates, r.solidity, r.leafReferences, func() error { return r.emit(lg) }, r.sheet} {
		if err := phase(); err != nil {
			return r.written, refusal(err)
		}
	}
	return r.written, nil
}

type refresher struct {
	root    string
	written []string
}

func (r *refresher) path(rel string) string { return filepath.Join(r.root, filepath.FromSlash(rel)) }

// write replaces a file's text, keeping its mode.
func (r *refresher) write(rel, text string) error {
	if err := atomicfile.Write(r.path(rel), []byte(text), nil); err != nil {
		return err
	}
	r.written = append(r.written, rel)
	return nil
}

func (r *refresher) aggregates() error {
	st, err := kb.Discover(r.root, log.Discard())
	if err != nil {
		return err
	}
	agg := SubtreeAggregates(st)
	for _, idx := range st.Indexes {
		text, err := kb.ReadText(r.path(idx.Path))
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

func (r *refresher) solidity() error {
	st, err := kb.Discover(r.root, log.Discard())
	if err != nil {
		return err
	}
	sol, err := ComputeSolidity(st)
	if err != nil {
		return err
	}
	byFile := map[string][]kb.ClaimEntry{}
	for _, e := range st.ClaimEntries {
		byFile[e.CanonicalPath] = append(byFile[e.CanonicalPath], e)
	}
	regs, err := kb.Registers(r.root)
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
		if err := r.rewriteSolidity(rel, byFile[rel], st.Supports, sol); err != nil {
			return err
		}
	}
	return nil
}

func (r *refresher) rewriteSolidity(rel string, entries []kb.ClaimEntry, supports []kb.SupportNode, sol Solidity) error {
	text, err := kb.ReadText(r.path(rel))
	if err != nil {
		return err
	}
	lines, final := editableLines(text)
	joined := strings.Join(lines, "\n")
	finals, supFinals := sol.Finals(), sol.SupFinals()
	byID := map[string]kb.ClaimEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	supByID := map[string]kb.SupportNode{}
	for _, s := range supports {
		supByID[s.ID] = s
	}
	var order []string
	ranges := map[string][2]int{}
	for _, loc := range kb.LocateEntries(joined) {
		if loc.QualityStart < 0 {
			continue
		}
		if _, ok := ranges[loc.NodeID]; !ok {
			order = append(order, loc.NodeID)
		}
		ranges[loc.NodeID] = [2]int{loc.QualityStart, loc.QualityEnd}
	}
	scrubbed := kb.SplitLines(kb.StripCodeFences(joined))
	for _, id := range order {
		entry, isClaim := byID[id]
		sup, isSup := supByID[id]
		if !isClaim && !isSup {
			continue
		}
		var computed *float64
		trace := ""
		if isClaim {
			if v, ok := finals[id]; ok {
				computed = ptr(v)
			}
			if res, ok := sol.Results[entry.ID]; ok {
				trace = RenderSolidityTrace(res)
			}
		} else {
			if v, ok := supFinals[id]; ok {
				computed = ptr(v)
			}
			trace = RenderMinTrace(sup.Quality, MinDependencySolidity(sup.DependsOn, finals))
		}
		span := ranges[id]
		for idx := span[0]; idx < span[1]; idx++ {
			line, err := lineAt(lines, idx, rel)
			if err != nil {
				return err
			}
			probe := scrubbed[idx]
			if solidityLineRE.MatchString(probe) {
				lines[idx] = solidityLine(computed, trace)
				continue
			}
			if !strings.Contains(probe, "(solidity") {
				continue
			}
			targets := kb.ClaimIDs(kb.BulletHead(kb.TrimBulletLead(kb.Strip(probe))))
			if len(targets) == 0 {
				continue
			}
			var target *float64
			if v, ok := finals[targets[0]]; ok {
				target = ptr(v)
			}
			if loc := annotationRE.FindStringIndex(line); loc != nil {
				lines[idx] = line[:loc[0]] + kb.RenderSolidityAnnotation(FormatSolidity(target)) + line[loc[1]:]
			}
		}
	}
	if next := joinLines(lines, final); next != text {
		return r.write(rel, next)
	}
	return nil
}

func solidityLine(v *float64, trace string) string {
	if v == nil {
		return kb.RenderSolidityLine(nil, "", "")
	}
	value := FormatSolidity(v)
	return kb.RenderSolidityLine(&value, *BuildStatusPhrase(v), trace)
}

func (r *refresher) leafReferences() error {
	st, err := kb.Discover(r.root, log.Discard())
	if err != nil {
		return err
	}
	refs := LeafReferences(st)
	regs, err := kb.Registers(r.root)
	if err != nil {
		return err
	}
	for _, rel := range regs {
		text, err := kb.ReadText(r.path(rel))
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
			at := band.QualityStart
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
	dir := filepath.Join(r.root, kb.IndexDir)
	if err := os.Mkdir(dir, 0o777); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	st, err := kb.Discover(r.root, lg)
	if err != nil {
		return err
	}
	records, err := BuildRecords(st)
	if err != nil {
		return err
	}
	for _, name := range IndexFiles {
		body := Serialize(records[name])
		target := filepath.Join(dir, name+".jsonl")
		if kb.IsFile(target) {
			if current, err := kb.ReadText(target); err == nil && current == body {
				continue
			}
		}
		if err := atomicfile.Write(target, []byte(body), nil); err != nil {
			return err
		}
		r.written = append(r.written, kb.IndexDir+"/"+name+".jsonl")
	}
	return nil
}

func (r *refresher) sheet() error {
	written, _, err := WriteSheet(r.root)
	if written {
		r.written = append(r.written, kb.ClaimGraphFile)
	}
	return err
}

// WriteSheet renders the placeholder sheet from the KB's .index/ and writes it
// where no sheet exists or the one there is the placeholder, only where its
// bytes change. A drawn sheet is left as it is and reported drawn.
func WriteSheet(kbRoot string) (written, drawn bool, err error) {
	target := filepath.Join(kbRoot, kb.ClaimGraphFile)
	current, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, false, err
	}
	if err == nil && !sheet.IsOwn(current) {
		return false, true, nil
	}
	svg, err := sheet.Render(filepath.Join(kbRoot, kb.IndexDir))
	if err != nil {
		return false, false, err
	}
	if bytes.Equal(svg, current) {
		return false, false, nil
	}
	if err := atomicfile.Write(target, svg, nil); err != nil {
		return false, false, err
	}
	return true, false, nil
}
