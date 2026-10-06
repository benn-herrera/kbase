package index

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
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

// Verify is the standard check kb_tools' kb-verify makes of the KB src reads —
// the dead-link and unknown-id gate over kb-root/, then the metadata gate —
// and returns every finding. The error is a failure to read the KB, never a
// finding.
func Verify(src *kb.Source) ([]Finding, error) {
	links, err := LinkFindings(src)
	if err != nil {
		return nil, err
	}
	meta, err := MetadataFindings(src)
	if err != nil {
		return nil, err
	}
	return slices.Concat(links, meta), nil
}

// CheckDemoted is the check a verify finding for a demoted edge names.
const CheckDemoted = kb.RelationDemoted

// DemotedFindings is every demoted edge the KB's registers record, each an
// item naming its register: structure the KB carries, not a fault.
func DemotedFindings(src *kb.Source) ([]result.Item, error) {
	st, err := kb.Discover(src, log.Discard())
	if err != nil {
		return nil, err
	}
	items := []result.Item{}
	for _, e := range st.ClaimEntries {
		for _, d := range e.Demoted {
			items = append(items, result.Item{Check: CheckDemoted, Path: e.CanonicalPath,
				Detail: fmt.Sprintf("%s → %s, origin %s: cut from depends by cycle breaking", d.Source, d.Target, cmp.Or(d.Origin, "none"))})
		}
	}
	return items, nil
}

// BuildVerify is the build-time check: Verify, then the citation gate, every
// gate run whatever the earlier found, findings in that order.
func BuildVerify(src *kb.Source) ([]Finding, error) {
	standard, err := Verify(src)
	if err != nil {
		return nil, err
	}
	cites, err := CitationFindings(src)
	if err != nil {
		return nil, err
	}
	return slices.Concat(standard, cites), nil
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
// of every derived field and of .index/. A KB
// state its readers cannot take — malformed node bodies, a claim graph with
// a cycle, an edge into a node nothing declares — ends the gate with that
// one finding.
func MetadataFindings(src *kb.Source) ([]Finding, error) {
	if err := precheck(src); err != nil {
		var r result.Refusal
		if errors.As(err, &r) {
			var out []Finding
			for _, it := range r {
				out = append(out, Finding{Check: checkMetadata, Path: it.Path, Detail: it.Detail})
			}
			return out, nil
		}
		return nil, err
	}
	m := metaCheck{src: src}
	findings, err := m.run()
	var mal kb.MalformedError
	var cov CoverageError
	if errors.As(err, &mal) || errors.As(err, &cov) {
		return append(findings, Finding{Check: checkMetadata, Detail: err.Error()}), nil
	}
	return findings, err
}

// CheckIndexFreshness names the finding for an index a refresh would write
// otherwise.
const CheckIndexFreshness = "index freshness"

type metaCheck struct {
	src      *kb.Source
	findings []Finding
}

func (m *metaCheck) add(check string, fixable bool, format string, args ...any) {
	m.addAt(check, fixable, "", 0, format, args...)
}

// addAt is a finding about the file at rel, at line where line is not 0.
func (m *metaCheck) addAt(check string, fixable bool, rel string, line int, format string, args ...any) {
	m.findings = append(m.findings, Finding{Check: check, Path: rel, Line: line, Detail: fmt.Sprintf(format, args...), RefreshFixable: fixable})
}

func (m *metaCheck) read(rel string) (string, error) { return m.src.ReadText(m.src.KBPath(rel)) }

func (m *metaCheck) run() ([]Finding, error) {
	docs, err := kb.Documents(m.src)
	if err != nil {
		return nil, err
	}
	var files []document
	for _, rel := range docs {
		text, err := m.read(rel)
		if err != nil {
			return nil, err
		}
		fm, err := kb.ParseFrontmatter(text)
		if err != nil {
			return m.findings, kb.MalformedError{Msg: rel + ": " + err.Error()}
		}
		files = append(files, document{rel, fm})
	}
	regs, err := kb.Registers(m.src)
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
	st, err := kb.Discover(m.src, log.Discard())
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
			switch claims, noClaim, exp := has("claims"), has("no-claim"), has(kb.ExperimentNodesKey); {
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

	if err := m.indexWellFormed(); err != nil {
		return nil, err
	}
	if err := m.indexFresh(st); err != nil {
		return m.findings, err
	}
	if err := m.referentialIntegrity(); err != nil {
		return nil, err
	}
	if err := m.solidityFresh(st, sol); err != nil {
		return nil, err
	}
	refs := LeafReferences(st)
	for _, rel := range regs {
		m.footersFresh(rel, registerText[rel], refs)
	}
	walk, err := kb.WalkRegisters(st, m.src)
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

// indexText is the index file name as src reads it, ok false where it does
// not stand.
func indexText(src *kb.Source, name string) (string, bool, error) {
	p := src.KBPath(kb.IndexDir + "/" + kb.IndexFileName(name))
	if !src.IsFile(p) {
		return "", false, nil
	}
	text, err := src.ReadText(p)
	return text, err == nil, err
}

// streamLine is one non-empty line of an index file: its 1-based number, the
// record after the document marker, and whether the marker opened it.
type streamLine struct {
	n      int
	record string
	marked bool
}

func streamLines(text string) []streamLine {
	var out []streamLine
	for n, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		record, marked := kb.IndexRecordJSON(line)
		out = append(out, streamLine{n + 1, record, marked})
	}
	return out
}

func (m *metaCheck) indexWellFormed() error {
	for _, name := range kb.IndexFiles {
		file := kb.IndexFileName(name)
		raw, ok, err := indexText(m.src, name)
		if err != nil {
			return err
		}
		if !ok {
			m.add("index", true, "%s is missing", file)
			continue
		}
		if raw == "" {
			continue
		}
		if !strings.HasSuffix(raw, "\n") {
			m.add("index", true, "%s: missing final newline", file)
		} else if strings.HasSuffix(raw, "\n\n") {
			m.add("index", true, "%s: multiple trailing newlines", file)
		}
		for _, line := range streamLines(raw) {
			if !line.marked {
				m.add("index", false, "%s:%d: line does not open with the document marker %q", file, line.n, kb.IndexRecordMarker)
				continue
			}
			v, ok := parseJSON(line.record)
			if !ok {
				m.add("index", false, "%s:%d: not well-formed JSON", file, line.n)
			} else if _, isObj := v.(map[string]any); !isObj {
				m.add("index", false, "%s:%d: line is not a JSON object", file, line.n)
			}
		}
	}
	return nil
}

func parseJSON(line string) (any, bool) {
	var v any
	if err := json.Unmarshal([]byte(line), &v); err != nil {
		return nil, false
	}
	return v, true
}

// indexFresh is the index's freshness: on a KB loaded at an older format, the
// index is stale whatever its bytes — a refresh is what writes the KB in the
// current form; otherwise each file is stale where its bytes differ from
// what a refresh writes.
func (m *metaCheck) indexFresh(st kb.State) error {
	if m.src.Migrated() {
		m.addAt(CheckIndexFreshness, true, kb.IndexDir, 0, "the KB's metadata format is %s and kbase writes %s; a refresh rewrites the KB in %s",
			m.src.Version(), kb.FormatVersion, kb.FormatVersion)
		return nil
	}
	records, err := BuildRecords(st)
	if err != nil {
		return err
	}
	for _, name := range kb.IndexFiles {
		file := kb.IndexFileName(name)
		actual, ok, err := indexText(m.src, name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if expected := Serialize(records[name]); actual != expected {
			lines := len(streamLines(actual))
			if lines == len(records[name]) {
				m.add(CheckIndexFreshness, true, "%s: %d records, bytes differ from what refresh writes", file, lines)
			} else {
				m.add(CheckIndexFreshness, true, "%s: %d expected vs %d actual records", file, len(records[name]), lines)
			}
		}
	}
	return nil
}

// recordLines is every record of an index file that parses, with its line
// number; ok false where the file does not stand.
func recordLines(src *kb.Source, name string) ([]any, []int, bool, error) {
	text, ok, err := indexText(src, name)
	if err != nil || !ok {
		return nil, nil, false, err
	}
	var vals []any
	var nums []int
	for _, line := range streamLines(text) {
		if !line.marked {
			continue
		}
		if v, ok := parseJSON(line.record); ok {
			vals = append(vals, v)
			nums = append(nums, line.n)
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

func (m *metaCheck) referentialIntegrity() error {
	claimsText, ok, err := indexText(m.src, "claims")
	if err != nil || !ok {
		return err
	}
	nodeType := map[string]string{}
	var nodeOrder []string
	for _, line := range streamLines(claimsText) {
		c, ok := parseJSON(line.record)
		if !ok || !line.marked {
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
		m.add("referential integrity", false, "%s: %v: %s", kb.IndexFileName(file), id, fmt.Sprintf(format, args...))
	}
	orphans := func(file string, keys ...string) error {
		recs, nums, ok, err := recordLines(m.src, file)
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
	deps, nums, _, err := recordLines(m.src, "depends-on")
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
		case kb.RelationSupports:
			expect("support", "source", srcOK, src, source)
			expect("claim", "target", tgtOK, tgt, target)
			kindIs("claim")
			nullStrength()
			fractionOrPending()
		case kb.RelationStrengthens:
			expect("experiment", "source", srcOK, src, source)
			expect("claim", "target", tgtOK, tgt, target)
			kindIs("claim")
			if strength == nil {
				violation("depends-on", source, "line %d, strengthens edge has null strength", n)
			} else if !inUnit(strength) {
				violation("depends-on", source, "line %d, strengthens edge strength %v not in [0, 1]", n, strength)
			}
		case kb.RelationRestsOn:
			expect("claim", "source", srcOK, src, source)
			expect("work", "target", tgtOK, tgt, target)
			kindIs("work")
			nullStrength()
			fractionOrPending()
		case kb.RelationReferences, kb.RelationDemoted:
			expect("claim", "source", srcOK, src, source)
			expect("claim", "target", tgtOK, tgt, target)
			kindIs("claim")
			nullStrength()
			if fraction != nil {
				violation("depends-on", source, "line %d, %s edge has non-null fraction %v", n, relation, fraction)
			}
			if origin, _ := rec["origin"].(string); relation == kb.RelationDemoted && !slices.Contains(kb.Origins, origin) {
				violation("depends-on", source, "line %d, demoted edge to %v carries origin %v, not one of %v; only the build writes a demoted edge, "+
					"and its origin says whether the text marked the dependency", n, target, rec["origin"], kb.Origins)
			}
		case kb.RelationDepends:
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
		recs, nums, _, err := recordLines(m.src, file)
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
	sb, sbNums, _, err := recordLines(m.src, "supported-by")
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

func (m *metaCheck) solidityFresh(st kb.State, sol Solidity) error {
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
	text, ok, err := indexText(m.src, "claims")
	if err != nil || !ok {
		return err
	}
	claimsFile := kb.IndexFileName("claims")
	for _, line := range streamLines(text) {
		v, ok := parseJSON(line.record)
		if !ok || !line.marked {
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
			drift(jsonKey(rec["id"]), fmt.Sprintf("%s solidity %v", claimsFile, onDisk), "expected null")
		case computed != nil && (!isNum || !approx(&f, computed)):
			drift(jsonKey(rec["id"]), fmt.Sprintf("%s solidity %v", claimsFile, onDisk), "expected "+FormatSolidity(computed))
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
