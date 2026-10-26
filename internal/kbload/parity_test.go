package kbload

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/index"
	"kbase/internal/kb"
)

// The 2.0.1 parity comparison: test-integration-parity-adjagent-sheet runs
// every case of the frozen 2.0.1 goldens through ./bin/kbase over the 2.0.1
// fixture as converted at load, and TestConvertedAnswersMatchTheGoldens
// compares each answer with its golden. Without its flags it skips.
var (
	parityAnswers    = flag.String("parity.answers", "", "the directory holding the converted KB's answers, NNN.out and NNN.exit per golden case")
	parityComparison = flag.String("parity.comparison", "", "where the comparison is written")
)

const (
	parityGoldenDir = "testdata/parity-2.0.1"
	parityFrom      = "2.0.1"
	// parityConverted is the 2.0.1 fixture's conversion as the converter's
	// golden pair pins it.
	parityConverted = "../formats/testdata/adjagent-sheet-3.0.0/" + kb.KBDir
)

// markerHosted is whether a node kind is hosted by a statement marker, total
// over kb.NodeKinds.
var markerHosted = map[string]bool{
	kb.NodeKindClaim: true, kb.NodeKindAnalysis: true, kb.NodeKindExperiment: true,
	kb.NodeKindAxiom: false, kb.NodeKindWork: false,
}

// What the conversion of the 2.0.1 fixture leaves, stated here and checked
// against the converter's golden by TestParityFactsAreTheConvertersGolden, so
// that no rule takes its expectation from the answers it judges.
var (
	// parityUnhosted is every claim, analysis and experiment the conversion
	// leaves with no statement marker: claims listed and marked nowhere.
	parityUnhosted = []string{"clm-aaa005", "clm-aaa006", "clm-bbb001", "clm-bbb003"}
	// parityLeafPages is every document but the entry point the conversion
	// leaves with no child list: the 2.0.1 fixture's index pages list no child.
	parityLeafPages = []string{"alpha/analysis.md", "alpha/experiments/flow.md", "alpha/index.md", "alpha/results.md",
		"beta/governance.md", "beta/index.md", "gamma/index.md"}
)

// TestParityFactsAreTheConvertersGolden: the parity rules' statement of what
// the conversion leaves unhosted and which documents it leaves leaves is what
// the converter's golden output holds.
func TestParityFactsAreTheConvertersGolden(t *testing.T) {
	src := kb.OnDisk(parityConverted)
	g, err := kb.LoadGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := kb.Documents(src)
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	leafPages := []string{}
	for _, rel := range docs {
		body, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			t.Fatal(err)
		}
		for id := range kb.MarkerLines(body) {
			marked[id] = true
		}
		if rel != kb.EntryPointFile && len(kb.ChildList(rel, body)) == 0 {
			leafPages = append(leafPages, rel)
		}
	}
	unhosted := []string{}
	for id, kind := range index.GraphKinds(g) {
		if markerHosted[kind] && !marked[id] {
			unhosted = append(unhosted, id)
		}
	}
	slices.Sort(unhosted)
	slices.Sort(leafPages)
	if !slices.Equal(unhosted, parityUnhosted) {
		t.Errorf("the converter's golden leaves %q unhosted, the parity rules state %q", unhosted, parityUnhosted)
	}
	if !slices.Equal(leafPages, parityLeafPages) {
		t.Errorf("the converter's golden leaves %q with no child list, the parity rules state %q", leafPages, parityLeafPages)
	}
}

// A golden-side rule turns a 2.0.1 answer into the answer the converted KB
// gives, each a designed outcome of the conversion; it reports whether it
// moved the answer. The rules run in this order.
var goldenRules = []struct {
	name  string
	apply func(f *conversionFacts, args []string, doc *yaml.Node) bool
}{
	{"an edge identity authored twice with equal bearings is one record, its contexts joined", (*conversionFacts).mergeEdges},
	{"a demoted edge is oriented as the edge it replaced", (*conversionFacts).orientDemoted},
	{"a claim listed in a leaf and marked in none is marked in its first listing leaf", (*conversionFacts).markListed},
	{"a cited-by row is a marker in a leaf, so it names no leaf_kind and no tier2_marked", (*conversionFacts).citedByMarkerRow},
	{"a document's kind comes from its child list: an index page with none is a leaf", (*conversionFacts).kindFromChildList},
	{"a node with no marker has a null canonical path and anchor", (*conversionFacts).unhostedCanonical},
}

// An answer-side rule removes from the converted KB's answer a key 3.0.0 adds,
// after checking where it stands and what it holds; it reports why not where
// the key is not as the rule states.
var answerRules = []struct {
	name  string
	apply func(a *answerFacts, args []string, doc *yaml.Node) string
}{
	{"a converted, unsaved KB's answer carries its migration record after kb-root", (*answerFacts).migrationRecord},
	{"manually_scored after a deps record's bearing and a show record's rigor; stats' four flag counts", (*answerFacts).manuallyScored},
}

type parityCase struct {
	name string
	args []string
	exit int
	doc  *yaml.Node
}

// readParityCase is the case at stem: its arguments from argsStem.args, and
// the exit code and the answer, parsed, from stem.exit and stem.out. The
// parsed answer re-encodes to the file's bytes, so a comparison of encodings
// is a comparison of bytes.
func readParityCase(t *testing.T, argsStem, stem string) parityCase {
	t.Helper()
	read := func(p string) string {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	c := parityCase{name: filepath.Base(stem), args: strings.Split(strings.TrimSuffix(read(argsStem+".args"), "\n"), "\n")}
	exit, err := strconv.Atoi(strings.TrimSpace(read(stem + ".exit")))
	if err != nil {
		t.Fatalf("%s.exit: %v", stem, err)
	}
	c.exit = exit
	raw := read(stem + ".out")
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("%s.out: %v", stem, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		t.Fatalf("%s.out is not one mapping", stem)
	}
	c.doc = doc.Content[0]
	if got := encodeNode(t, c.doc); got != raw {
		t.Fatalf("%s.out does not re-encode to its bytes:\n%s", stem, got)
	}
	return c
}

func encodeNode(t *testing.T, n *yaml.Node) string {
	t.Helper()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(n); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// keyAt is the index of key's key node in mapping m, -1 where m has none.
func keyAt(m *yaml.Node, key string) int {
	if m == nil || m.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// value is key's value in mapping m, nil where m has none.
func value(m *yaml.Node, key string) *yaml.Node {
	if i := keyAt(m, key); i >= 0 {
		return m.Content[i+1]
	}
	return nil
}

// text is a scalar's value, "" for nil, null or a collection.
func text(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

func isNull(n *yaml.Node) bool { return n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!null" }

func setNull(n *yaml.Node) { *n = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"} }

func setInt(n *yaml.Node, v int) {
	*n = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(v)}
}

func setTrue(n *yaml.Node) { *n = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"} }

// removeAfter removes key from mapping m where it stands directly after
// prev, returning its value; nil where it does not stand there.
func removeAfter(m *yaml.Node, prev, key string) *yaml.Node {
	i := keyAt(m, prev)
	if i < 0 || i+2 >= len(m.Content) || m.Content[i+2].Value != key {
		return nil
	}
	v := m.Content[i+3]
	m.Content = slices.Delete(m.Content, i+2, i+4)
	return v
}

// mappings is every mapping at or below n, depth first.
func mappings(n *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	if n.Kind == yaml.MappingNode {
		out = append(out, n)
	}
	for _, c := range n.Content {
		out = append(out, mappings(c)...)
	}
	return out
}

// listAnswer is a list query's results, nil for any other answer.
func listAnswer(doc *yaml.Node) *yaml.Node {
	if r := value(doc, "results"); r != nil && r.Kind == yaml.SequenceNode {
		return r
	}
	return nil
}

// depsOf is the id a deps case lists the edges of, "" for any other case.
func depsOf(args []string) string {
	if len(args) == 2 && args[0] == "deps" {
		return args[1]
	}
	return ""
}

// demotedEdge is a 2.0.1 demoted row as deps showed it: from the entry it was
// listed on to the premise it named.
type demotedEdge struct {
	entry, premise   string
	bearing, context *yaml.Node
}

// conversionFacts is what the golden-side rules read: each gathered from the
// 2.0.1 goldens, or stated by parityUnhosted and parityLeafPages.
type conversionFacts struct {
	unhosted       map[string]bool   // parityUnhosted
	leafPages      map[string]bool   // parityLeafPages
	firstListing   map[string]string // claim id → the leaf §8.2 marks it in, for a claim listed and marked nowhere
	premisesMerged map[string]int    // claim id → scoring edges into it merged away
	edgesMerged    int
	demoted        []demotedEdge
	aggregatesLost int
}

// duplicates is each deps record of results that repeats an earlier one's
// relation, direction, other end and bearing, mapped to that earlier one's
// index. A demoted record is not one: its identity carries the relation it
// replaced, which deps does not show.
func duplicates(results *yaml.Node) map[int]int {
	first := map[string]int{}
	out := map[int]int{}
	for i, r := range results.Content {
		if text(value(r, "relation")) == kb.RelationDemoted {
			continue
		}
		b := value(r, "bearing")
		key := strings.Join([]string{text(value(r, "relation")), text(value(r, "direction")), text(value(r, "target")), b.Tag, b.Value}, "\x00")
		if j, seen := first[key]; seen {
			out[i] = j
		} else {
			first[key] = i
		}
	}
	return out
}

func gatherFacts(t *testing.T, goldens []parityCase) *conversionFacts {
	t.Helper()
	f := &conversionFacts{unhosted: map[string]bool{}, leafPages: map[string]bool{}, firstListing: map[string]string{}, premisesMerged: map[string]int{}}
	for _, id := range parityUnhosted {
		f.unhosted[id] = true
	}
	for _, rel := range parityLeafPages {
		f.leafPages[rel] = true
	}
	listings := map[string][]string{}
	markedSomewhere := map[string]bool{}
	aggregates := map[string]bool{}
	for _, c := range goldens {
		switch {
		case depsOf(c.args) != "":
			results := listAnswer(c.doc)
			for i := range duplicates(results) {
				r := results.Content[i]
				relation := text(value(r, "relation"))
				switch {
				case text(value(r, "direction")) == "in":
					f.premisesMerged[c.args[1]]++
					f.edgesMerged++
				case !kb.Scoring(relation):
					f.edgesMerged++
				}
			}
			for _, r := range results.Content {
				if text(value(r, "relation")) == kb.RelationDemoted {
					f.demoted = append(f.demoted, demotedEdge{entry: c.args[1], premise: text(value(r, "target")),
						bearing: value(r, "bearing"), context: value(r, "context")})
				}
			}
		case c.args[0] == "cited-by" && listAnswer(c.doc) != nil:
			for _, r := range listAnswer(c.doc).Content {
				id := text(value(r, "claim_id"))
				if text(value(r, "tier2_marked")) == "true" {
					markedSomewhere[id] = true
				}
				if text(value(r, "leaf_kind")) == "leaf" {
					listings[id] = append(listings[id], text(value(r, "leaf_path")))
				}
			}
		case c.args[0] == "subtree" && strings.HasSuffix(c.args[1], ".md"):
			aggregates[c.args[1]] = true
		}
	}
	for id, leaves := range listings {
		if !markedSomewhere[id] {
			f.firstListing[id] = slices.Min(leaves)
		}
	}
	for page := range aggregates {
		if f.leafPages[page] {
			f.aggregatesLost++
		}
	}
	return f
}

func (f *conversionFacts) mergeEdges(args []string, doc *yaml.Node) bool {
	moved := false
	if id := depsOf(args); id != "" {
		results := listAnswer(doc)
		dups := duplicates(results)
		var kept []*yaml.Node
		for i, r := range results.Content {
			j, dup := dups[i]
			if !dup {
				kept = append(kept, r)
				continue
			}
			into := results.Content[j]
			var contexts []string
			for _, c := range []*yaml.Node{value(into, "context"), value(r, "context")} {
				if !isNull(c) {
					contexts = append(contexts, c.Value)
				}
			}
			if len(contexts) > 0 {
				*value(into, "context") = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: strings.Join(contexts, "; ")}
			}
			moved = true
		}
		results.Content = kept
	}
	for _, m := range mappings(doc) {
		if n := value(m, "premise_count"); n != nil && f.premisesMerged[text(value(m, "id"))] > 0 {
			v, _ := strconv.Atoi(n.Value)
			setInt(n, v-f.premisesMerged[text(value(m, "id"))])
			moved = true
		}
	}
	if args[0] == "stats" && f.edgesMerged > 0 {
		n := value(value(doc, "results"), "edges")
		v, _ := strconv.Atoi(n.Value)
		setInt(n, v-f.edgesMerged)
		moved = true
	}
	return moved
}

// orientDemoted moves each demoted row from the entry's deps to its premise's,
// naming the entry: deps lists an id's outgoing demoted edges last, by the
// other end and then the context.
func (f *conversionFacts) orientDemoted(args []string, doc *yaml.Node) bool {
	id := depsOf(args)
	if id == "" || len(f.demoted) == 0 {
		return false
	}
	results := listAnswer(doc)
	before := len(results.Content)
	results.Content = slices.DeleteFunc(results.Content, func(r *yaml.Node) bool {
		return text(value(r, "relation")) == kb.RelationDemoted
	})
	moved := len(results.Content) != before
	var reoriented []demotedEdge
	for _, d := range f.demoted {
		if d.premise == id {
			reoriented = append(reoriented, d)
		}
	}
	slices.SortStableFunc(reoriented, func(a, b demotedEdge) int {
		if c := strings.Compare(a.entry, b.entry); c != 0 {
			return c
		}
		return strings.Compare(text(a.context), text(b.context))
	})
	quoted := func(s string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: s}
	}
	key := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	for _, d := range reoriented {
		results.Content = append(results.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			key("relation"), quoted(kb.RelationDemoted), key("direction"), quoted("out"), key("target"), quoted(d.entry),
			key("bearing"), d.bearing, key("context"), d.context}})
		moved = true
	}
	return moved
}

func (f *conversionFacts) markListed(args []string, doc *yaml.Node) bool {
	if args[0] != "cited-by" || listAnswer(doc) == nil {
		return false
	}
	moved := false
	for _, r := range listAnswer(doc).Content {
		if leaf, ok := f.firstListing[text(value(r, "claim_id"))]; ok && leaf == text(value(r, "leaf_path")) {
			setTrue(value(r, "tier2_marked"))
			moved = true
		}
	}
	return moved
}

// citedByMarkerRow drops the two keys from a cited-by row that names a leaf
// and is marked there; any other row keeps them and so differs.
func (f *conversionFacts) citedByMarkerRow(args []string, doc *yaml.Node) bool {
	if args[0] != "cited-by" || listAnswer(doc) == nil {
		return false
	}
	moved := false
	for _, r := range listAnswer(doc).Content {
		if text(value(r, "leaf_kind")) != "leaf" || text(value(r, "tier2_marked")) != "true" {
			continue
		}
		removeAfter(r, "leaf_path", "leaf_kind")
		removeAfter(r, "leaf_path", "tier2_marked")
		moved = true
	}
	return moved
}

// kindFromChildList empties the subtree answer of a page that is now a leaf —
// named by its path, or by its directory as dir/ — and drops each such
// page's aggregate from stats' count.
func (f *conversionFacts) kindFromChildList(args []string, doc *yaml.Node) bool {
	switch args[0] {
	case "subtree":
		page := args[1]
		if strings.HasSuffix(page, "/") {
			page += kb.IndexFile
		}
		results := listAnswer(doc)
		if !f.leafPages[page] || results == nil || len(results.Content) == 0 {
			return false
		}
		results.Content, results.Style = nil, yaml.FlowStyle
		return true
	case "stats":
		if f.aggregatesLost == 0 {
			return false
		}
		n := value(value(doc, "results"), "subtree_aggregates")
		v, _ := strconv.Atoi(n.Value)
		setInt(n, v-f.aggregatesLost)
		return true
	}
	return false
}

func (f *conversionFacts) unhostedCanonical(_ []string, doc *yaml.Node) bool {
	moved := false
	for _, m := range mappings(doc) {
		if !markerHosted[text(value(m, "node_type"))] || !f.unhosted[text(value(m, "id"))] {
			continue
		}
		for _, key := range []string{"canonical_path", "canonical_anchor"} {
			if n := value(m, key); n != nil && !isNull(n) {
				setNull(n)
				moved = true
			}
		}
	}
	return moved
}

// recount sets a list answer's count to its results' length: every case runs
// with --limit 0, so the count is the whole list.
func recount(doc *yaml.Node) {
	if r, n := listAnswer(doc), value(doc, "count"); r != nil && n != nil {
		setInt(n, len(r.Content))
	}
}

// answerFacts is what the answer-side rules read, gathered from the converted
// KB's answers before any rule removes a key.
type answerFacts struct {
	migration    string          // the first answer's migration record, encoded
	flaggedNodes map[string]bool // ids whose show record carries manually_scored
	flaggedEdges map[string]bool // edges a deps record shows with a true or false manually_scored
	encode       func(*yaml.Node) string
}

func gatherAnswerFacts(t *testing.T, answers []parityCase) *answerFacts {
	t.Helper()
	a := &answerFacts{flaggedNodes: map[string]bool{}, flaggedEdges: map[string]bool{},
		encode: func(n *yaml.Node) string { return encodeNode(t, n) }}
	for _, c := range answers {
		if a.migration == "" {
			if m := value(c.doc, "migration"); m != nil {
				a.migration = a.encode(m)
			}
		}
		results := value(c.doc, "results")
		switch {
		case c.args[0] == "show" && results != nil && results.Kind == yaml.MappingNode:
			if value(results, "manually_scored") != nil {
				a.flaggedNodes[c.args[1]] = true
			}
		case depsOf(c.args) != "":
			for _, r := range results.Content {
				if v := value(r, "manually_scored"); v == nil || isNull(v) {
					continue
				}
				source, target := c.args[1], text(value(r, "target"))
				if text(value(r, "direction")) == "in" {
					source, target = target, source
				}
				a.flaggedEdges[strings.Join([]string{source, text(value(r, "relation")), target, text(value(r, "context"))}, "\x00")] = true
			}
		}
	}
	return a
}

func (a *answerFacts) migrationRecord(_ []string, doc *yaml.Node) string {
	m := removeAfter(doc, "kb-root", "migration")
	switch {
	case m == nil:
		return "no migration record after kb-root"
	case text(value(m, "from")) != parityFrom || text(value(m, "to")) != kb.FormatVersion:
		return fmt.Sprintf("the migration record is from %q to %q, not from %s to %s", text(value(m, "from")), text(value(m, "to")), parityFrom, kb.FormatVersion)
	case a.encode(m) != a.migration:
		return "the migration record differs from the first answer's"
	}
	return ""
}

func (a *answerFacts) manuallyScored(args []string, doc *yaml.Node) string {
	results := value(doc, "results")
	if results == nil {
		return ""
	}
	switch {
	case depsOf(args) != "":
		for _, r := range results.Content {
			relation := text(value(r, "relation"))
			v := removeAfter(r, "bearing", "manually_scored")
			carried := kb.BearingWalk(relation) || relation == kb.RelationDemoted
			if v == nil || (carried && text(v) != "false") || (!carried && !isNull(v)) {
				return fmt.Sprintf("a %s record's manually_scored after bearing is not %s", relation, map[bool]string{true: "false", false: "null"}[carried])
			}
		}
	case args[0] == "show" && results.Kind == yaml.MappingNode && a.flaggedNodes[args[1]]:
		if v := removeAfter(results, "rigor", "manually_scored"); v == nil || text(v) != "false" {
			return "show's manually_scored is not false directly after rigor"
		}
	case args[0] == "stats":
		want := []struct {
			key   string
			count int
		}{{"rigor_manual", 0}, {"rigor_model", len(a.flaggedNodes)}, {"bearing_manual", 0}, {"bearing_model", len(a.flaggedEdges)}}
		// Each key removed, the next stands directly after subtree_aggregates.
		for _, w := range want {
			v := removeAfter(results, "subtree_aggregates", w.key)
			if v == nil || text(v) != strconv.Itoa(w.count) {
				return fmt.Sprintf("stats' %s is not %d in its place after subtree_aggregates", w.key, w.count)
			}
		}
	}
	return ""
}

type parityDifference struct {
	Case     string   `yaml:"case"`
	Args     []string `yaml:"args"`
	Reason   string   `yaml:"reason"`
	Expected string   `yaml:"expected"`
	Actual   string   `yaml:"actual"`
}

type parityRuleTally struct {
	Rule  string   `yaml:"rule"`
	Cases []string `yaml:"cases,flow"`
}

type parityComparisonDoc struct {
	Cases       int                `yaml:"cases"`
	Equal       int                `yaml:"equal"`
	Rules       []parityRuleTally  `yaml:"rules"`
	Differences []parityDifference `yaml:"differences"`
}

// TestConvertedAnswersMatchTheGoldens: every 2.0.1 golden, moved by the
// golden-side rules, equals byte for byte the converted KB's answer with the
// keys the answer-side rules state removed, and the exit codes agree.
func TestConvertedAnswersMatchTheGoldens(t *testing.T) {
	if *parityAnswers == "" || *parityComparison == "" {
		t.Skip("run by test-integration-parity-adjagent-sheet")
	}
	for _, kind := range kb.NodeKinds {
		if _, ok := markerHosted[kind]; !ok {
			t.Fatalf("markerHosted names no %s", kind)
		}
	}
	argFiles, err := filepath.Glob(filepath.Join(parityGoldenDir, "*.args"))
	if err != nil || len(argFiles) == 0 {
		t.Fatalf("no golden cases under %s: %v", parityGoldenDir, err)
	}
	var goldens, answers []parityCase
	for _, f := range argFiles {
		stem := strings.TrimSuffix(f, ".args")
		goldens = append(goldens, readParityCase(t, stem, stem))
		answers = append(answers, readParityCase(t, stem, filepath.Join(*parityAnswers, filepath.Base(stem))))
	}
	facts := gatherFacts(t, goldens)
	answerSide := gatherAnswerFacts(t, answers)

	cmp := parityComparisonDoc{Cases: len(goldens)}
	tallies := make([]parityRuleTally, len(goldenRules)+len(answerRules))
	for i, r := range answerRules {
		tallies[i].Rule = r.name
	}
	for i, r := range goldenRules {
		tallies[len(answerRules)+i].Rule = r.name
	}
	for i, g := range goldens {
		a := answers[i]
		diff := func(reason string) {
			cmp.Differences = append(cmp.Differences, parityDifference{Case: g.name, Args: g.args, Reason: reason,
				Expected: encodeNode(t, g.doc), Actual: encodeNode(t, a.doc)})
		}
		reason := ""
		for j, r := range answerRules {
			before := encodeNode(t, a.doc)
			if reason = r.apply(answerSide, a.args, a.doc); reason != "" {
				break
			}
			if encodeNode(t, a.doc) != before {
				tallies[j].Cases = append(tallies[j].Cases, g.name)
			}
		}
		if reason != "" {
			diff(reason)
			continue
		}
		for j, r := range goldenRules {
			if r.apply(facts, g.args, g.doc) {
				tallies[len(answerRules)+j].Cases = append(tallies[len(answerRules)+j].Cases, g.name)
			}
		}
		recount(g.doc)
		switch {
		case g.exit != a.exit:
			diff(fmt.Sprintf("exit %d, the golden's %d", a.exit, g.exit))
		case encodeNode(t, g.doc) != encodeNode(t, a.doc):
			diff("the answer differs from its golden under the rules")
		default:
			cmp.Equal++
		}
	}
	cmp.Rules = tallies
	data, err := yaml.Marshal(cmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(*parityComparison, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if len(cmp.Differences) > 0 {
		t.Errorf("%d of %d cases differ from their goldens — see %s", len(cmp.Differences), cmp.Cases, *parityComparison)
	}
}
