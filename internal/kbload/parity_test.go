package kbload

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"kbase/internal/kb"
	"kbase/internal/log"
)

// The 0.9.0 typed readers kb carried before the 1.0.0 readers replaced them,
// frozen here as the oracle the replacement is held to: a KB read through
// them from its 0.9.0 bytes must equal the same KB read through the
// migration and the 1.0.0 readers. Nothing in production reaches them.

var (
	old090FrontmatterRE = kb.PyRE(`(?s)<!--\s*kb-frontmatter\s*\n(.*?)\n[ \t]*-->`)
	old090BulletRE      = kb.PyRE(`^\s*-\s+(.*)$`)
	old090ClaimOrExpRE  = kb.PyRE(kb.IDBody("clm", "exp"))
	old090Tier2RE       = kb.PyRE(`(?s)<!--\s*claim-quality:\s*(.*?)\s*-->`)
	old090Strengthens   = kb.PyRE(`^\s*(?:-\s*)?(` + kb.IDBody("clm") + `)\s*:\s*(-?\d+(?:\.\d+)?)\s*$`)
	old090Heading       = kb.PyRE(`^(#{1,6})\s+(.*)$`)
	old090ExpIDFull     = kb.PyRE(`^` + kb.IDBody("exp") + `$`)
	old090SupIDFull     = kb.PyRE(`^` + kb.IDBody("sup") + `$`)
	old090Declarations  = []*regexp.Regexp{
		kb.PyRE(`(?m)^\s*-?\s*exp-id:\s*(` + kb.IDBody("exp") + `)\s*$`),
		kb.PyRE(`(?m)^\s*-?\s*sup-id:\s*(` + kb.IDBody("sup") + `)\s*$`),
	}
)

func old090Bounded(re *regexp.Regexp, s string) []string {
	boundary := func(i int) bool {
		last, _ := utf8.DecodeLastRuneInString(s[:i])
		next, _ := utf8.DecodeRuneInString(s[i:])
		return (i > 0 && kb.IsWord(last)) != (i < len(s) && kb.IsWord(next))
	}
	var out []string
	for _, m := range re.FindAllStringIndex(s, -1) {
		if boundary(m[0]) && boundary(m[1]) {
			out = append(out, s[m[0]:m[1]])
		}
	}
	return out
}

func old090Find(text string) []int { return old090FrontmatterRE.FindStringSubmatchIndex(text) }

func old090Strip(text string) string {
	var b strings.Builder
	pos := 0
	for {
		m := old090FrontmatterRE.FindStringSubmatchIndex(text[pos:])
		if m == nil {
			break
		}
		b.WriteString(text[pos : pos+m[0]])
		pos += m[1]
	}
	b.WriteString(text[pos:])
	return b.String()
}

func old090FieldEnd(lines []string, start int) int {
	_, value, _ := strings.Cut(kb.RStrip(lines[start]), ":")
	value = kb.Strip(value)
	i := start + 1
	if strings.HasPrefix(value, "[") && !strings.HasSuffix(value, "]") {
		for i < len(lines) && !strings.HasSuffix(kb.Strip(lines[i]), "]") {
			i++
		}
		return min(i+1, len(lines))
	}
	if value == "" {
		for i < len(lines) && old090BulletRE.MatchString(lines[i]) {
			i++
		}
	}
	return i
}

func old090Value(value string) kb.Value {
	switch {
	case strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"):
		return kb.Value{IsList: true, List: old090Bounded(old090ClaimOrExpRE, value)}
	case strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`):
		if len(value) < 2 {
			return kb.Value{}
		}
		return kb.Value{Str: value[1 : len(value)-1]}
	case value == "true" || value == "false":
		return kb.Value{IsBool: true, Bool: value == "true"}
	}
	return kb.Value{Str: value}
}

func old090Fields(body string) kb.Frontmatter {
	fields := kb.Frontmatter{}
	lines := kb.SplitLines(body)
	for i := 0; i < len(lines); {
		line := kb.RStrip(lines[i])
		if line == "" || !strings.Contains(line, ":") {
			i++
			continue
		}
		end := old090FieldEnd(lines, i)
		key, value, _ := strings.Cut(line, ":")
		value = kb.Strip(value)
		if tail := lines[i+1 : end]; len(tail) > 0 {
			if strings.HasPrefix(value, "[") {
				parts := []string{value}
				for _, l := range tail {
					parts = append(parts, kb.Strip(l))
				}
				value = strings.Join(parts, " ")
			} else {
				items := make([]string, len(tail))
				for j, l := range tail {
					items[j] = kb.Strip(old090BulletRE.FindStringSubmatch(l)[1])
				}
				value = "[" + strings.Join(items, ", ") + "]"
			}
		}
		fields[kb.Strip(key)] = old090Value(value)
		i = end
	}
	return fields
}

func old090Parse(text string) kb.Frontmatter {
	m := old090Find(text)
	if m == nil {
		return nil
	}
	return old090Fields(text[m[2]:m[3]])
}

func old090FirstHeading(text string) string {
	for _, line := range kb.SplitLines(text) {
		if m := old090Heading.FindStringSubmatch(line); m != nil {
			return kb.Strip(m[2])
		}
	}
	return ""
}

func old090Key(stripped string) (key, value string, ok bool) {
	k, v, ok := strings.Cut(stripped, ":")
	if !ok {
		return "", "", false
	}
	return kb.Strip(strings.TrimLeft(kb.Strip(k), "- ")), kb.Strip(v), true
}

func old090Malformed(format string, args ...any) error {
	return kb.MalformedError{Msg: fmt.Sprintf(format, args...)}
}

func old090Leaf(text, rel string) (*kb.LeafRecord, error) {
	fm := old090Parse(text)
	if len(fm) == 0 || fm.Kind() != kb.DocumentLeaf {
		return nil, nil
	}
	claims, err := fm.ListOrEmpty("claims")
	if err != nil {
		return nil, old090Malformed("%s: claims: %v", rel, err)
	}
	leaf := &kb.LeafRecord{Path: rel, Kind: kb.DocumentLeaf, Claims: claims, Tier2Marked: map[string]bool{}}
	if v, ok := fm.Get("no-claim"); ok && !v.IsList && !v.IsBool {
		leaf.NoClaimReason = v.Str
	}
	refs, err := fm.ListOrEmpty("experiments")
	if err != nil {
		return nil, old090Malformed("%s: experiments: %v", rel, err)
	}
	for _, r := range refs {
		if strings.HasPrefix(r, "exp-") {
			leaf.ExperimentRefs = append(leaf.ExperimentRefs, r)
		}
	}
	for _, m := range old090Tier2RE.FindAllStringSubmatch(old090Strip(text), -1) {
		for _, cid := range kb.ClaimIDs(m[1]) {
			if slices.Contains(claims, cid) {
				leaf.Tier2Marked[cid] = true
			}
		}
	}
	return leaf, nil
}

func old090Experiments(text, rel string) ([]kb.ExperimentNode, error) {
	m := old090Find(text)
	if m == nil {
		return nil, nil
	}
	kind, hasRefs, inStrengthens := "", false, false
	var ids []string
	var statuses []*string
	var pairs [][]kb.StrengthensPair
	for _, line := range kb.SplitLines(text[m[2]:m[3]]) {
		s := kb.Strip(line)
		if s == "" {
			continue
		}
		if p := old090Strengthens.FindStringSubmatch(line); inStrengthens && p != nil && len(pairs) > 0 {
			v, _ := strconv.ParseFloat(p[2], 64)
			pairs[len(pairs)-1] = append(pairs[len(pairs)-1], kb.StrengthensPair{ClaimID: p[1], Strength: v})
			continue
		}
		key, value, ok := old090Key(s)
		if !ok {
			continue
		}
		if key == "strengthens" {
			inStrengthens = true
			continue
		}
		inStrengthens = false
		switch key {
		case "kind":
			kind = value
		case "exp-id":
			ids = append(ids, value)
			statuses = append(statuses, nil)
			pairs = append(pairs, nil)
		case "status":
			if len(statuses) > 0 {
				statuses[len(statuses)-1] = &value
			}
		case "experiments":
			if value != "" {
				hasRefs = true
			}
		}
	}
	if kind != kb.DocumentLeaf || len(ids) == 0 {
		return nil, nil
	}
	if hasRefs {
		return nil, old090Malformed("%s: experiment-hosting leaf carries experiments:", rel)
	}
	heading := old090FirstHeading(text)
	var nodes []kb.ExperimentNode
	for i, id := range ids {
		if !old090ExpIDFull.MatchString(id) {
			return nil, old090Malformed("%s: malformed exp-id %q", rel, id)
		}
		if statuses[i] == nil || (*statuses[i] != "run" && *statuses[i] != "pending") {
			return nil, old090Malformed("%s: experiment %s has invalid status", rel, id)
		}
		for _, p := range pairs[i] {
			if !(0 <= p.Strength && p.Strength <= 1) {
				return nil, old090Malformed("%s: experiment %s strength out of range", rel, id)
			}
		}
		nodes = append(nodes, kb.ExperimentNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: kb.Slugify(heading),
			Status: *statuses[i], Strengthens: pairs[i]})
	}
	return nodes, nil
}

func old090Supports(text, rel string) ([]kb.SupportNode, error) {
	m := old090Find(text)
	if m == nil {
		return nil, nil
	}
	kind, inSupports := "", false
	var ids []string
	var pairs [][]kb.SupportPair
	for _, line := range kb.SplitLines(text[m[2]:m[3]]) {
		s := kb.Strip(line)
		if s == "" {
			continue
		}
		if p, ok := kb.ParseSupportPair(line); inSupports && ok && len(pairs) > 0 {
			pairs[len(pairs)-1] = append(pairs[len(pairs)-1], p)
			continue
		}
		key, value, ok := old090Key(s)
		if !ok {
			continue
		}
		if key == "supports" {
			inSupports = true
			continue
		}
		inSupports = false
		switch key {
		case "kind":
			kind = value
		case "sup-id":
			ids = append(ids, value)
			pairs = append(pairs, nil)
		}
	}
	if kind != kb.DocumentLeaf || len(ids) == 0 {
		return nil, nil
	}
	heading := old090FirstHeading(text)
	var nodes []kb.SupportNode
	for i, id := range ids {
		if !old090SupIDFull.MatchString(id) {
			return nil, old090Malformed("%s: malformed sup-id %q", rel, id)
		}
		for _, p := range pairs[i] {
			if !p.Fraction.Pending && !(0 <= p.Fraction.Value && p.Fraction.Value <= 1) {
				return nil, old090Malformed("%s: support %s fraction out of range", rel, id)
			}
		}
		nodes = append(nodes, kb.SupportNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: kb.Slugify(heading), Supports: pairs[i]})
	}
	return nodes, nil
}

func old090Index(text, rel string) (*kb.IndexRecord, error) {
	fm := old090Parse(text)
	if len(fm) == 0 {
		return nil, nil
	}
	kind := fm.Kind()
	if kind != kb.DocumentIndex && kind != kb.DocumentEntryPoint {
		return nil, nil
	}
	claims, err := fm.ListOrEmpty("subtree-claims")
	if err != nil {
		return nil, err
	}
	exps, err := fm.ListOrEmpty("subtree-experiments")
	if err != nil {
		return nil, err
	}
	rec := &kb.IndexRecord{Path: rel, Kind: kind, DeclaredSubtreeClaims: claims}
	for _, e := range exps {
		if strings.HasPrefix(e, "exp-") {
			rec.DeclaredSubtreeExperiments = append(rec.DeclaredSubtreeExperiments, e)
		}
	}
	return rec, nil
}

// old090Discover is kb.Discover as it read a 0.9.0 KB: the registers through
// kb's register readers, which the format change does not touch, and the
// documents through the frozen 0.9.0 readers above.
func old090Discover(src *kb.Source) (kb.State, error) {
	known, err := kb.KnownClaimIDs(src)
	if err != nil {
		return kb.State{}, err
	}
	regs, err := kb.Registers(src)
	if err != nil {
		return kb.State{}, err
	}
	var st kb.State
	supportQ := map[string]kb.SupportQuality{}
	for _, rel := range regs {
		text, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			return kb.State{}, err
		}
		st.Works = append(st.Works, kb.ParseWorkEntries(text, rel)...)
		st.ClaimEntries = append(st.ClaimEntries, kb.ParseClaimEntries(text, rel, known, log.Discard())...)
		_, sq := kb.ParseSupportEntries(text, rel, known, log.Discard())
		for id, q := range sq {
			supportQ[id] = q
		}
	}
	docs, err := kb.Documents(src)
	if err != nil {
		return kb.State{}, err
	}
	var hosted []kb.SupportNode
	for _, rel := range docs {
		text, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			return kb.State{}, err
		}
		leaf, err := old090Leaf(text, rel)
		if err != nil {
			return kb.State{}, err
		}
		if leaf != nil {
			st.Leaves = append(st.Leaves, *leaf)
		}
		exps, err := old090Experiments(text, rel)
		if err != nil {
			return kb.State{}, err
		}
		st.Experiments = append(st.Experiments, exps...)
		sups, err := old090Supports(text, rel)
		if err != nil {
			return kb.State{}, err
		}
		hosted = append(hosted, sups...)
		if leaf == nil && len(exps) == 0 && len(sups) == 0 {
			idx, err := old090Index(text, rel)
			if err != nil {
				return kb.State{}, err
			}
			if idx != nil {
				st.Indexes = append(st.Indexes, *idx)
			}
		}
	}
	for _, s := range hosted {
		q := supportQ[s.ID]
		s.Quality, s.DependsOn, s.Rationale, s.Solidity, s.SolidityTrace = q.Quality, q.DependsOn, q.Rationale, q.Solidity, q.SolidityTrace
		st.Supports = append(st.Supports, s)
	}
	if st.FrameworkNodes, err = kb.ParseFrameworkNodes(src); err != nil {
		return kb.State{}, err
	}
	slices.SortStableFunc(st.Works, func(a, b kb.ExternalWork) int { return strings.Compare(a.ID, b.ID) })
	return st, nil
}

// old090Declared is every exp- and sup- id each document's 0.9.0 block
// declared, as AuthoredIDs inventoried them.
func old090Declared(src *kb.Source) (map[string]string, error) {
	docs, err := kb.Documents(src)
	if err != nil {
		return nil, err
	}
	declared := map[string]string{}
	for _, rel := range docs {
		text, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			return nil, err
		}
		m := old090Find(text)
		if m == nil {
			continue
		}
		for _, re := range old090Declarations {
			for _, d := range re.FindAllStringSubmatch(text[m[2]:m[3]], -1) {
				if _, ok := declared[d[1]]; !ok {
					declared[d[1]] = rel
				}
			}
		}
	}
	return declared, nil
}

// declaredNow is the same inventory through the 1.0.0 readers.
func declaredNow(t *testing.T, src *kb.Source) map[string]string {
	t.Helper()
	docs, err := kb.Documents(src)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, rel := range docs {
		text, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			t.Fatal(err)
		}
		fm, err := kb.ParseFrontmatter(text)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		for _, id := range kb.DeclaredNodeIDs(fm) {
			if _, ok := out[id]; !ok {
				out[id] = rel
			}
		}
	}
	return out
}

// parityCase is one 0.9.0 KB, by its kb-root.
type parityCase struct{ name, kbRoot string }

// parityCases is the committed 0.9.0 KBs — the migration's golden input, the
// loose node shapes, and a KB kb_tools built in 0.9.0 (its claim-graph sheet
// fixture at the adjagent commit adjagent-commit.txt names, the last before
// kb_tools adopted 1.0.0) — and each kbtools-ref KB under test_data/transient
// that is in the 0.9.0 form. kb_tools builds 1.0.0 since that adoption, so a
// kbtools-ref KB staged after it is not 0.9.0 and is no case.
func parityCases(t *testing.T) []parityCase {
	t.Helper()
	cases := []parityCase{
		{"migrate goldens", "../migrate/testdata/0.9.0/kb-root"},
		{"loose node shapes", "testdata/loose-0.9.0/kb-root"},
		{"kb_tools-built", "testdata/kbtools-0.9.0/kb-root"},
	}
	refs, _ := filepath.Glob("../../test_data/transient/kbtools-ref/*/" + kb.KBDir)
	for _, r := range refs {
		if is090(r) {
			cases = append(cases, parityCase{"kbtools-ref " + filepath.Base(filepath.Dir(r)), r})
		}
	}
	return cases
}

// is090 is whether the KB at kbRoot is in the unstamped 0.9.0 form.
func is090(kbRoot string) bool {
	stamp, err := formatStamp(kbRoot)
	return err == nil && stamp == kb.UnstampedFormatVersion
}

// TestMigratedReadEqualsThe090Read: over every 0.9.0 KB parityCases names,
// the authored state and the declared node ids read through the migration
// and the 1.0.0 readers equal those the 0.9.0 readers read off the original
// bytes.
func TestMigratedReadEqualsThe090Read(t *testing.T) {
	for _, c := range parityCases(t) {
		t.Run(c.name, func(t *testing.T) {
			before := kb.OnDisk(c.kbRoot)
			want, err := old090Discover(before)
			if err != nil {
				t.Fatalf("the 0.9.0 readers: %v", err)
			}
			wantDeclared, err := old090Declared(before)
			if err != nil {
				t.Fatal(err)
			}
			src, err := Open(c.kbRoot)
			if err != nil {
				t.Fatalf("loading: %v", err)
			}
			if !src.Migrated() {
				t.Fatalf("%s loaded as current; the case is a 0.9.0 KB", c.kbRoot)
			}
			got, err := kb.Discover(src, log.Discard())
			if err != nil {
				t.Fatalf("the 1.0.0 readers: %v", err)
			}
			if !reflect.DeepEqual(normalized(got), normalized(want)) {
				t.Errorf("the migrated read differs from the 0.9.0 read:\n got %+v\nwant %+v", got, want)
			}
			if gotDeclared := declaredNow(t, src); !reflect.DeepEqual(gotDeclared, wantDeclared) {
				t.Errorf("declared node ids: got %v, want %v", gotDeclared, wantDeclared)
			}
		})
	}
}

// normalized is st with empty slices and maps made nil, so a reader that
// returns an empty list and one that returns none compare equal.
func normalized(st kb.State) kb.State {
	for i := range st.Leaves {
		if len(st.Leaves[i].Tier2Marked) == 0 {
			st.Leaves[i].Tier2Marked = nil
		}
		if len(st.Leaves[i].Claims) == 0 {
			st.Leaves[i].Claims = nil
		}
	}
	for i := range st.Experiments {
		if len(st.Experiments[i].Strengthens) == 0 {
			st.Experiments[i].Strengthens = nil
		}
	}
	for i := range st.Supports {
		if len(st.Supports[i].Supports) == 0 {
			st.Supports[i].Supports = nil
		}
	}
	for i := range st.Indexes {
		if len(st.Indexes[i].DeclaredSubtreeClaims) == 0 {
			st.Indexes[i].DeclaredSubtreeClaims = nil
		}
	}
	return st
}
