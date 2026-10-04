package index

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/sheet"
)

// Finding is one verify failure: the check that found it, the kb-root-relative
// file and line it is about where it is about one, what it found, and whether
// a refresh clears it.
type Finding struct {
	Check, Path    string
	Line           int
	Detail         string
	RefreshFixable bool
}

// RefreshRemedy is the command that clears a refresh-fixable finding.
const RefreshRemedy = "kbase refresh"

// Item is the finding as a result item.
func (f Finding) Item() result.Item {
	it := result.Item{Check: f.Check, Path: f.Path, Line: f.Line, Detail: f.Detail}
	if f.RefreshFixable {
		it.Remedy = RefreshRemedy
	}
	return it
}

// Items is findings as result items.
func Items(findings []Finding) []result.Item {
	items := make([]result.Item, len(findings))
	for i, f := range findings {
		items[i] = f.Item()
	}
	return items
}

// CheckFrontmatterPresence names the finding for a document carrying no
// frontmatter block.
const CheckFrontmatterPresence = "frontmatter"

// Verify checks the KB at kbRoot as kb_tools' kb-verify does — the dead-link
// and unknown-id gate over kb-root/, the metadata gate, the citation gate —
// and returns every finding. The error is a failure to read the KB, never a
// finding.
func Verify(kbRoot string) ([]Finding, error) {
	links, err := LinkFindings(kbRoot)
	if err != nil {
		return nil, err
	}
	meta, err := MetadataFindings(kbRoot)
	if err != nil {
		return nil, err
	}
	cites, err := CitationFindings(kbRoot)
	if err != nil {
		return nil, err
	}
	return slices.Concat(links, meta, cites), nil
}

var (
	scoreLineRE     = kb.PyRE(`^\s*-\s+(confidence|quality|strength):\s*(\S.*?)\s*$`)
	leadingNumberRE = regexp.MustCompile(`^[-+]?[0-9]*\.?[0-9]+`)
	expIDFull       = regexp.MustCompile(`^` + kb.IDBody("exp") + `$`)
	supIDFull       = regexp.MustCompile(`^` + kb.IDBody("sup") + `$`)
	claimIDFull     = regexp.MustCompile(`^` + kb.IDBody("clm") + `$`)
	workIDFull      = regexp.MustCompile(`^` + kb.WorkIDPattern + `$`)
)

type document struct {
	rel string
	fm  kb.Frontmatter
}

func (d document) items(key string) ([]string, error) {
	v, ok := d.fm.Get(key)
	if !ok {
		return nil, nil
	}
	return v.Items()
}

// MetadataFindings is the metadata gate: coverage, uniqueness, referential
// integrity, acyclicity, the register census and fan-out, and the freshness
// of every derived field, of .index/ and of the placeholder sheet. A KB
// state its readers cannot take — malformed node bodies, a claim graph with
// a cycle, an edge into a node nothing declares — ends the gate with that
// one finding.
func MetadataFindings(kbRoot string) ([]Finding, error) {
	if err := precheck(kbRoot); err != nil {
		var r Refusal
		if errors.As(err, &r) {
			var out []Finding
			for _, it := range r.Items {
				out = append(out, Finding{Check: checkMetadata, Path: it.Path, Detail: it.Detail})
			}
			return out, nil
		}
		return nil, err
	}
	m := metaCheck{root: kbRoot}
	findings, err := m.run()
	var mal kb.MalformedError
	var cov CoverageError
	if errors.As(err, &mal) || errors.As(err, &cov) {
		return append(findings, Finding{Check: checkMetadata, Detail: err.Error()}), nil
	}
	return findings, err
}

type metaCheck struct {
	root     string
	findings []Finding
}

func (m *metaCheck) add(check string, fixable bool, format string, args ...any) {
	m.addAt(check, fixable, "", 0, format, args...)
}

// addAt is a finding about the file at rel, at line where line is not 0.
func (m *metaCheck) addAt(check string, fixable bool, rel string, line int, format string, args ...any) {
	m.findings = append(m.findings, Finding{Check: check, Path: rel, Line: line, Detail: fmt.Sprintf(format, args...), RefreshFixable: fixable})
}

func (m *metaCheck) read(rel string) (string, error) {
	return kb.ReadText(filepath.Join(m.root, filepath.FromSlash(rel)))
}

func (m *metaCheck) run() ([]Finding, error) {
	docs, err := kb.Documents(m.root)
	if err != nil {
		return nil, err
	}
	var files []document
	for _, rel := range docs {
		text, err := m.read(rel)
		if err != nil {
			return nil, err
		}
		files = append(files, document{rel, kb.ParseFrontmatter(text)})
	}
	regs, err := kb.Registers(m.root)
	if err != nil {
		return nil, err
	}
	registerText := map[string]string{}
	for _, rel := range regs {
		if registerText[rel], err = m.read(rel); err != nil {
			return nil, err
		}
	}
	type canonicalEntry struct{ id, path string }
	var canonical []canonicalEntry
	canonicalSet := map[string]bool{}
	for _, rel := range regs {
		for _, mm := range kb.CanonicalClaimMarkerRE.FindAllStringSubmatch(kb.StripCodeFences(registerText[rel]), -1) {
			canonical = append(canonical, canonicalEntry{mm[1], rel})
			canonicalSet[mm[1]] = true
		}
	}
	st, err := kb.Discover(m.root, log.Discard())
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.fm == nil {
			m.addAt(CheckFrontmatterPresence, false, f.rel, 0, "%s has no frontmatter block; stamp one with set-frontmatter", f.rel)
		}
	}
	for _, rel := range regs {
		m.qualityBlocks(rel, registerText[rel])
		m.scoreValues(rel, registerText[rel])
	}
	equations := map[string]bool{}
	for _, e := range st.ClaimEntries {
		if _, ok := kb.EquationLabel(e.Title); ok {
			equations[e.ID] = true
		}
	}
	cited := map[string]bool{}
	for _, f := range files {
		if f.fm == nil {
			continue
		}
		if f.fm.Kind() == kb.DocumentLeaf {
			has := func(k string) bool { v, ok := f.fm.Get(k); return ok && v.Truthy() }
			switch claims, noClaim, exp := has("claims"), has("no-claim"), has("exp-id"); {
			case !claims && !noClaim && !exp:
				m.addAt("tier-1 coverage", false, f.rel, 0, "%s declares none of claims / no-claim / exp-id", f.rel)
			case claims && noClaim:
				m.addAt("tier-1 coverage", false, f.rel, 0, "%s declares BOTH claims and no-claim", f.rel)
			}
		}
		claims, err := f.items("claims")
		if err != nil {
			return m.findings, kb.MalformedError{Msg: fmt.Sprintf("%s: claims: %v", f.rel, err)}
		}
		if err := m.tier2(f, claims, equations); err != nil {
			return nil, err
		}
		for _, c := range claims {
			if !canonicalSet[c] {
				m.addAt("orphan reference", false, f.rel, 0, "%s cites %s, which no canonical entry keys", f.rel, c)
			}
		}
		if len(f.fm) > 0 {
			for _, c := range claims {
				cited[c] = true
			}
		}
	}
	seen := map[string][]string{}
	var order []string
	for _, c := range canonical {
		if _, ok := seen[c.id]; !ok {
			order = append(order, c.id)
		}
		seen[c.id] = append(seen[c.id], c.path)
	}
	for _, id := range order {
		if len(seen[id]) > 1 {
			m.add("id uniqueness", false, "%s keyed %d times: %s", id, len(seen[id]), strings.Join(seen[id], ", "))
		}
	}
	agg := SubtreeAggregates(st)
	for _, idx := range st.Indexes {
		m.aggregateDrift("subtree-claims", idx.Path, agg[idx.Path].Claims, idx.DeclaredSubtreeClaims)
	}
	for _, c := range canonical {
		if !cited[c.id] {
			m.add("bidirectional coverage", false, "%s (in %s) has no leaf citation", c.id, c.path)
		}
	}

	sol, err := ComputeSolidity(st)
	var cycle CycleError
	if errors.As(err, &cycle) {
		return []Finding{{Check: "acyclicity", Detail: cycle.Error()}}, nil
	}
	if err != nil {
		return nil, err
	}

	indexDir := filepath.Join(m.root, kb.IndexDir)
	missing, malformed, err := m.indexWellFormed(indexDir)
	if err != nil {
		return nil, err
	}
	if err := m.indexFresh(indexDir, st); err != nil {
		return m.findings, err
	}
	if err := m.referentialIntegrity(indexDir); err != nil {
		return nil, err
	}
	if !missing && !malformed {
		if err := m.sheetFresh(indexDir); err != nil {
			return nil, err
		}
	}
	if err := m.solidityFresh(st, sol, indexDir); err != nil {
		return nil, err
	}
	refs := LeafReferences(st)
	for _, rel := range regs {
		m.footersFresh(rel, registerText[rel], refs)
	}
	walk, err := kb.WalkRegisters(st, m.root)
	if err != nil {
		return nil, err
	}
	m.registerWalk(walk)
	if err := m.experimentRefs(files, st); err != nil {
		return nil, err
	}
	for _, idx := range st.Indexes {
		m.aggregateDrift("subtree-experiments", idx.Path, agg[idx.Path].Experiments, idx.DeclaredSubtreeExperiments)
	}
	return m.findings, nil
}

func (m *metaCheck) aggregateDrift(field, path string, expected, declared []string) {
	want, got := setOf(expected), setOf(declared)
	missing, extra := setMinus(want, got), setMinus(got, want)
	if len(missing) > 0 || len(extra) > 0 {
		m.add(field, true, "%s: missing from declared %v, extra in declared %v", path, missing, extra)
	}
}

func setOf(items []string) map[string]bool {
	out := map[string]bool{}
	for _, i := range items {
		out[i] = true
	}
	return out
}

func setMinus(a, b map[string]bool) []string {
	out := []string{}
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

func (m *metaCheck) qualityBlocks(rel, text string) {
	lines := kb.SplitLines(kb.StripCodeFences(text))
	type section struct {
		start int
		lines []string
	}
	var sections []section
	current := section{start: 1}
	for n, line := range lines {
		if kb.Strip(line) == "---" {
			sections = append(sections, current)
			current = section{start: n + 2}
			continue
		}
		current.lines = append(current.lines, line)
	}
	sections = append(sections, current)
	for _, sec := range sections {
		quality := slices.IndexFunc(sec.lines, func(l string) bool { return kb.Strip(l) == "### Quality" })
		if quality < 0 {
			continue
		}
		hasTitle := slices.ContainsFunc(sec.lines, func(l string) bool { return strings.HasPrefix(l, "## ") })
		hasID := slices.ContainsFunc(sec.lines, func(l string) bool { return kb.CanonicalEntryMarkerRE.MatchString(kb.Strip(l)) })
		if !hasTitle || !hasID {
			m.addAt("quality block", false, rel, sec.start+quality, "%s:%d: `### Quality` block without its `## <title>` heading and id marker", rel, sec.start+quality)
		}
	}
}

func (m *metaCheck) scoreValues(rel, text string) {
	for n, line := range strings.Split(kb.StripCodeFences(text), "\n") {
		mm := scoreLineRE.FindStringSubmatch(line)
		if mm == nil || mm[2] == kb.PendingLiteral {
			continue
		}
		num := leadingNumberRE.FindString(mm[2])
		if num == "" {
			m.addAt("score value", false, rel, n+1, "%s:%d: %s: %q is neither a number nor *pending*", rel, n+1, mm[1], mm[2])
			continue
		}
		if v, _ := strconv.ParseFloat(num, 64); !(0 <= v && v <= 1) {
			m.addAt("score value", false, rel, n+1, "%s:%d: %s: %v is outside [0, 1]", rel, n+1, mm[1], v)
		}
	}
}

var tier2RE = kb.PyRE(`(?s)<!--\s*claim-quality:\s*(.*?)\s*-->`)

func (m *metaCheck) tier2(f document, claims []string, equations map[string]bool) error {
	var ids []string
	for _, c := range claims {
		if !equations[c] {
			ids = append(ids, c)
		}
	}
	if len(ids) < 2 {
		return nil
	}
	text, err := m.read(f.rel)
	if err != nil {
		return err
	}
	markers := tier2RE.FindAllStringSubmatch(kb.StripFrontmatter(text), -1)
	var missing []string
	for _, id := range ids {
		if !slices.ContainsFunc(markers, func(mk []string) bool { return kb.ContainsWord(mk[1], id) }) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		m.addAt("tier-2 coverage", false, f.rel, 0, "%s: claims %v, missing inline markers for %v", f.rel, ids, missing)
	}
	return nil
}

func (m *metaCheck) indexWellFormed(dir string) (missing, malformed bool, err error) {
	for _, name := range IndexFiles {
		p := filepath.Join(dir, name+".jsonl")
		if _, statErr := os.Stat(p); statErr != nil {
			m.add("index", true, "%s.jsonl is missing", name)
			missing = true
			continue
		}
		raw, err := kb.ReadText(p)
		if err != nil {
			return false, false, err
		}
		if raw == "" {
			continue
		}
		if !strings.HasSuffix(raw, "\n") {
			m.add("index", true, "%s.jsonl: missing final newline", name)
		} else if strings.HasSuffix(raw, "\n\n") {
			m.add("index", true, "%s.jsonl: multiple trailing newlines", name)
		}
		for n, line := range strings.Split(raw, "\n") {
			if line == "" {
				continue
			}
			v, ok := parseJSON(line)
			if !ok {
				m.add("index", false, "%s.jsonl:%d: not well-formed JSON", name, n+1)
				malformed = true
			} else if _, isObj := v.(map[string]any); !isObj {
				m.add("index", false, "%s.jsonl:%d: line is not a JSON object", name, n+1)
				malformed = true
			}
		}
	}
	return missing, malformed, nil
}

func parseJSON(line string) (any, bool) {
	var v any
	if err := json.Unmarshal([]byte(line), &v); err != nil {
		return nil, false
	}
	return v, true
}

func (m *metaCheck) indexFresh(dir string, st kb.State) error {
	records, err := BuildRecords(st)
	if err != nil {
		return err
	}
	for _, name := range IndexFiles {
		p := filepath.Join(dir, name+".jsonl")
		if _, err := os.Stat(p); err != nil {
			continue
		}
		actual, err := kb.ReadText(p)
		if err != nil {
			return err
		}
		if expected := Serialize(records[name]); actual != expected {
			lines := 0
			for _, l := range strings.Split(actual, "\n") {
				if l != "" {
					lines++
				}
			}
			m.add("index freshness", true, "%s.jsonl: %d expected vs %d actual records", name, len(records[name]), lines)
		}
	}
	return nil
}

// jsonLines is every line of a JSONL file that parses, with its line number;
// absent where the file is.
func jsonLines(path string) ([]any, []int, bool, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil, false, nil
	}
	text, err := kb.ReadText(path)
	if err != nil {
		return nil, nil, false, err
	}
	var vals []any
	var nums []int
	for n, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		if v, ok := parseJSON(line); ok {
			vals = append(vals, v)
			nums = append(nums, n+1)
		}
	}
	return vals, nums, true, nil
}

// jsonKey is a JSON value as a lookup key: strings as themselves, anything
// else in a spelling no string takes.
func jsonKey(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return "\x00" + string(b)
}

func jsonTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

// jsonItems is a JSON value iterated as Python iterates it.
func jsonItems(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case string:
		var out []any
		for _, r := range x {
			out = append(out, string(r))
		}
		return out
	case map[string]any:
		var out []any
		for k := range x {
			out = append(out, k)
		}
		return out
	}
	return nil
}

func inUnit(v any) bool {
	f, ok := v.(float64)
	return ok && 0 <= f && f <= 1
}

func (m *metaCheck) referentialIntegrity(dir string) error {
	claimsPath := filepath.Join(dir, "claims.jsonl")
	if _, err := os.Stat(claimsPath); err != nil {
		return nil
	}
	claimsText, err := kb.ReadText(claimsPath)
	if err != nil {
		return err
	}
	nodeType := map[string]string{}
	var nodeOrder []string
	for _, line := range strings.Split(claimsText, "\n") {
		if line == "" {
			continue
		}
		c, ok := parseJSON(line)
		if !ok {
			return nil
		}
		rec, ok := c.(map[string]any)
		if !ok {
			continue
		}
		id, ok := rec["id"]
		if !ok {
			return nil
		}
		t := "claim"
		if v, ok := rec["node_type"]; ok {
			t = jsonKey(v)
		}
		k := jsonKey(id)
		if _, seen := nodeType[k]; !seen {
			nodeOrder = append(nodeOrder, k)
		}
		nodeType[k] = t
	}
	violation := func(file string, id any, format string, args ...any) {
		m.add("referential integrity", false, "%s.jsonl: %v: %s", file, id, fmt.Sprintf(format, args...))
	}
	orphans := func(file string, keys ...string) error {
		recs, nums, ok, err := jsonLines(filepath.Join(dir, file+".jsonl"))
		if err != nil || !ok {
			return err
		}
		for i, r := range recs {
			rec, isObj := r.(map[string]any)
			if !isObj {
				continue
			}
			for _, key := range keys {
				var ids []any
				switch key {
				case "subtree_claims", "subtree_experiments":
					if jsonTruthy(rec[key]) {
						ids = jsonItems(rec[key])
					}
				default:
					if jsonTruthy(rec[key]) {
						ids = []any{rec[key]}
					}
				}
				for _, id := range ids {
					if _, ok := nodeType[jsonKey(id)]; !ok {
						violation(file, id, "line %d, %s resolves to no node", nums[i], key)
					}
				}
			}
		}
		return nil
	}
	for _, c := range []struct {
		file string
		keys []string
	}{
		{"depends-on", []string{"source", "target"}}, {"strengthen-by", []string{"claim_id"}}, {"cites", []string{"claim_id"}},
		{"supported-by", []string{"claim_id", "sup_id"}}, {"subtree-aggregates", []string{"subtree_claims", "subtree_experiments"}},
	} {
		if err := orphans(c.file, c.keys...); err != nil {
			return err
		}
	}
	typeOf := func(v any) (string, bool) { t, ok := nodeType[jsonKey(v)]; return t, ok }
	deps, nums, _, err := jsonLines(filepath.Join(dir, "depends-on.jsonl"))
	if err != nil {
		return err
	}
	for i, d := range deps {
		rec, ok := d.(map[string]any)
		if !ok {
			continue
		}
		n := nums[i]
		source, target, kind := rec["source"], rec["target"], rec["target_kind"]
		strength, fraction := rec["strength"], rec["fraction"]
		src, srcOK := typeOf(source)
		tgt, tgtOK := typeOf(target)
		relation, _ := rec["relation"].(string)
		if _, isStr := rec["relation"].(string); !isStr {
			relation = "\x00"
		}
		expect := func(want, end string, ok bool, got string, id any) {
			if ok && got != want {
				violation("depends-on", id, "line %d, %s edge %s resolves to %q, expected %s", n, relation, end, got, want)
			}
		}
		kindIs := func(want string) {
			if k, isStr := kind.(string); !isStr || k != want {
				violation("depends-on", target, "line %d, %s edge target_kind %v != %q", n, relation, kind, want)
			}
		}
		nullStrength := func() {
			if strength != nil {
				violation("depends-on", source, "line %d, %s edge has non-null strength %v", n, relation, strength)
			}
		}
		fractionOrPending := func() {
			if fraction != kb.PendingLiteral && !inUnit(fraction) {
				violation("depends-on", source, "line %d, %s edge fraction %v not in [0, 1] or *pending*", n, relation, fraction)
			}
		}
		switch relation {
		case "supports":
			expect("support", "source", srcOK, src, source)
			expect("claim", "target", tgtOK, tgt, target)
			kindIs("claim")
			nullStrength()
			fractionOrPending()
		case "strengthens":
			expect("experiment", "source", srcOK, src, source)
			expect("claim", "target", tgtOK, tgt, target)
			kindIs("claim")
			if strength == nil {
				violation("depends-on", source, "line %d, strengthens edge has null strength", n)
			} else if !inUnit(strength) {
				violation("depends-on", source, "line %d, strengthens edge strength %v not in [0, 1]", n, strength)
			}
		case "rests-on":
			expect("claim", "source", srcOK, src, source)
			expect("work", "target", tgtOK, tgt, target)
			kindIs("work")
			nullStrength()
			fractionOrPending()
		case "references":
			expect("claim", "source", srcOK, src, source)
			expect("claim", "target", tgtOK, tgt, target)
			kindIs("claim")
			nullStrength()
			if fraction != nil {
				violation("depends-on", source, "line %d, references edge has non-null fraction %v", n, fraction)
			}
		case "depends":
			if srcOK && src != "claim" && src != "support" {
				violation("depends-on", source, "line %d, depends edge source resolves to %q, expected claim or support", n, src)
			}
			if tgtOK && jsonKey(kind) != tgt {
				violation("depends-on", target, "line %d, target_kind %v != node_type %q", n, kind, tgt)
			}
			if tgt == "experiment" {
				violation("depends-on", target, "line %d, depends edge targets an experiment node", n)
			}
			nullStrength()
		default:
			violation("depends-on", source, "line %d, unknown relation %v", n, rec["relation"])
		}
	}
	experiments := map[string]bool{}
	for _, id := range nodeOrder {
		switch t := nodeType[id]; {
		case t == "experiment":
			experiments[id] = true
			if !expIDFull.MatchString(id) {
				violation("claims", id, "experiment node id is not %s", kb.IDBody("exp"))
			}
		case t == "support" && !supIDFull.MatchString(id):
			violation("claims", id, "support node id is not %s", kb.IDBody("sup"))
		case t == "work" && !workIDFull.MatchString(id):
			violation("claims", id, "external-work node id is not %s", kb.WorkIDPattern)
		}
	}
	for _, file := range []string{"cites", "subtree-aggregates"} {
		recs, nums, _, err := jsonLines(filepath.Join(dir, file+".jsonl"))
		if err != nil {
			return err
		}
		for i, r := range recs {
			rec, ok := r.(map[string]any)
			if !ok {
				continue
			}
			ids := []any{rec["claim_id"]}
			if file == "subtree-aggregates" {
				ids = nil
				if jsonTruthy(rec["subtree_claims"]) {
					ids = jsonItems(rec["subtree_claims"])
				}
			}
			for _, id := range ids {
				if experiments[jsonKey(id)] {
					violation(file, id, "line %d, experiment node referenced where a claim id is expected", nums[i])
				}
			}
			if file == "subtree-aggregates" && jsonTruthy(rec["subtree_experiments"]) {
				for _, id := range jsonItems(rec["subtree_experiments"]) {
					if t, ok := typeOf(id); ok && !experiments[jsonKey(id)] {
						violation(file, id, "line %d, subtree_experiments id resolves to %q, expected experiment", nums[i], t)
					}
				}
			}
		}
	}
	sb, sbNums, _, err := jsonLines(filepath.Join(dir, "supported-by.jsonl"))
	if err != nil {
		return err
	}
	for i, r := range sb {
		rec, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := typeOf(rec["claim_id"]); ok && t != "claim" {
			violation("supported-by", rec["claim_id"], "line %d, claim_id resolves to %q, expected claim", sbNums[i], t)
		}
		if t, ok := typeOf(rec["sup_id"]); ok && t != "support" {
			violation("supported-by", rec["sup_id"], "line %d, sup_id resolves to %q, expected support", sbNums[i], t)
		}
	}
	return nil
}

// sheetFresh checks the placeholder sheet against the index on disk. A
// drawn sheet is not this toolchain's to check; an absent one is drift.
func (m *metaCheck) sheetFresh(indexDir string) error {
	path := filepath.Join(m.root, kb.ClaimGraphFile)
	if !kb.IsFile(path) {
		m.addAt("claim-graph sheet", true, kb.ClaimGraphFile, 0, "no %s on disk", kb.ClaimGraphFile)
		return nil
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !sheet.IsOwn(current) {
		return nil
	}
	want, err := sheet.Render(indexDir)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, want) {
		m.addAt("claim-graph sheet", true, kb.ClaimGraphFile, 0, "%s is not what %s/ renders: %d bytes on disk, %d rendered", kb.ClaimGraphFile, kb.IndexDir, len(current), len(want))
	}
	return nil
}

func approx(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(*a-*b) < 1e-9
}

func optFinal(m map[string]float64, id string) *float64 {
	if v, ok := m[id]; ok {
		return ptr(v)
	}
	return nil
}

func (m *metaCheck) annotationDrift(owner string, edges []kb.Edge, finals map[string]float64) {
	for _, e := range edges {
		if e.TargetKind != "claim" {
			continue
		}
		target := optFinal(finals, e.Target)
		switch {
		case target == nil && e.TargetSolidityRecorded != nil:
			m.add("solidity freshness", true, "%s: depends-on annotation for %s — recorded %s, expected *pending*", owner, e.Target, FormatSolidity(e.TargetSolidityRecorded))
		case target != nil && e.TargetSolidityRecorded != nil && !approx(e.TargetSolidityRecorded, target):
			m.add("solidity freshness", true, "%s: depends-on annotation for %s — recorded %s, expected %s", owner, e.Target, FormatSolidity(e.TargetSolidityRecorded), FormatSolidity(target))
		}
	}
}

func (m *metaCheck) solidityFresh(st kb.State, sol Solidity, indexDir string) error {
	finals, supFinals := sol.Finals(), sol.SupFinals()
	drift := func(id, got, want string) { m.add("solidity freshness", true, "%s: %s, %s", id, got, want) }
	for _, s := range st.Supports {
		computed := optFinal(supFinals, s.ID)
		switch {
		case computed == nil:
			if s.Solidity != nil {
				drift(s.ID, "solidity "+FormatSolidity(s.Solidity), "expected *pending*")
			}
		case !approx(s.Solidity, computed):
			drift(s.ID, "solidity "+FormatSolidity(s.Solidity), "expected "+FormatSolidity(computed))
		default:
			if want := RenderMinTrace(s.Quality, MinDependencySolidity(s.DependsOn, finals)); s.SolidityTrace != want {
				drift(s.ID, "trace "+s.SolidityTrace, "expected "+want)
			}
		}
		m.annotationDrift(s.ID, s.DependsOn, finals)
	}
	for _, e := range st.ClaimEntries {
		computed := optFinal(finals, e.ID)
		if computed == nil {
			if e.Solidity != nil {
				drift(e.ID, "solidity "+FormatSolidity(e.Solidity), "expected *pending*")
			}
			continue
		}
		phrase := BuildStatusPhrase(computed)
		switch {
		case !approx(e.Solidity, computed):
			drift(e.ID, "solidity "+FormatSolidity(e.Solidity), "expected "+FormatSolidity(computed))
		case e.BuildStatus == nil || *e.BuildStatus != *phrase:
			drift(e.ID, "build-status mismatch", "expected "+*phrase)
		default:
			if want := RenderSolidityTrace(sol.Results[e.ID]); e.SolidityTrace != want {
				drift(e.ID, "trace "+e.SolidityTrace, "expected "+want)
			}
		}
		m.annotationDrift(e.ID, e.DependsOn, finals)
	}
	p := filepath.Join(indexDir, "claims.jsonl")
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	text, err := kb.ReadText(p)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		v, ok := parseJSON(line)
		if !ok {
			break
		}
		rec, ok := v.(map[string]any)
		if !ok {
			continue
		}
		nodeType := "claim"
		if t, ok := rec["node_type"]; ok {
			nodeType = jsonKey(t)
		}
		var computed *float64
		switch nodeType {
		case "support":
			computed = optFinal(supFinals, jsonKey(rec["id"]))
		case "claim":
			computed = optFinal(finals, jsonKey(rec["id"]))
		default:
			continue
		}
		onDisk := rec["solidity"]
		f, isNum := onDisk.(float64)
		switch {
		case computed == nil && onDisk != nil:
			drift(jsonKey(rec["id"]), fmt.Sprintf("claims.jsonl solidity %v", onDisk), "expected null")
		case computed != nil && (!isNum || !approx(&f, computed)):
			drift(jsonKey(rec["id"]), fmt.Sprintf("claims.jsonl solidity %v", onDisk), "expected "+FormatSolidity(computed))
		}
	}
	return nil
}

func (m *metaCheck) footersFresh(rel, text string, refs map[string][]string) {
	lines := strings.Split(text, "\n")
	for _, band := range kb.LocateLeafReferenceFooters(text) {
		want := RenderLeafReferences(rel, refs[band.NodeID])
		switch {
		case band.FooterLine < 0:
			m.addAt("leaf-references footer", true, rel, 0, "%s:%s: footer missing", rel, band.NodeID)
		case band.FooterLine >= len(lines):
			m.addAt("leaf-references footer", false, rel, 0, "%s:%s: footer line %d is past the register's end", rel, band.NodeID, band.FooterLine+1)
		case lines[band.FooterLine] != want:
			m.addAt("leaf-references footer", true, rel, 0, "%s:%s: footer is stale", rel, band.NodeID)
		}
	}
}

func (m *metaCheck) registerWalk(w kb.RegisterWalk) {
	for _, c := range w.Census {
		if !c.Consistent() {
			m.addAt("register census", false, c.RegisterPath, 0, "%s: %s markers %d vs records %d; no record for %v", c.RegisterPath, c.Kind, len(c.Markers), len(c.Records), c.Lost())
		}
	}
	for _, e := range w.MisBound() {
		m.add("register binding", false, "%s:%d: %s binds to `## %s` at line %d, a heading not its own",
			e.RegisterPath, e.Entry.MarkerLine+1, e.Entry.NodeID, e.Entry.HeadingTitle, e.Entry.HeadingLine+1)
	}
	for _, r := range w.Unreconciled() {
		m.add("support fan-out", false, "%s -> %s: register %s and hosting leaf %s disagree (recorded at %s)",
			r.SupID, r.ClaimID, r.RegisterPath, r.LeafPath, strings.Join(r.Ends(), ", "))
	}
}

func (m *metaCheck) experimentRefs(files []document, st kb.State) error {
	experiments := map[string]bool{}
	for _, x := range st.Experiments {
		experiments[x.ID] = true
	}
	claims := map[string]bool{}
	for _, e := range st.ClaimEntries {
		claims[e.ID] = true
	}
	for _, f := range files {
		if f.fm == nil {
			continue
		}
		v, ok := f.fm.Get("experiments")
		if !ok || !v.Truthy() {
			continue
		}
		refs, err := v.Items()
		if err != nil {
			return kb.MalformedError{Msg: fmt.Sprintf("%s: experiments: %v", f.rel, err)}
		}
		for _, id := range refs {
			switch {
			case !expIDFull.MatchString(id) && (claims[id] || claimIDFull.MatchString(id)):
				m.addAt("experiment reference", false, f.rel, 0, "%s: %s resolves to a claim, not an experiment", f.rel, id)
			case !expIDFull.MatchString(id):
				m.addAt("experiment reference", false, f.rel, 0, "%s: %s is a malformed exp-id", f.rel, id)
			case !experiments[id]:
				m.addAt("experiment reference", false, f.rel, 0, "%s: %s is no experiment node", f.rel, id)
			}
		}
	}
	return nil
}
