package kbload

import (
	"bytes"
	"flag"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/argmark"
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
	// parityConverted is the 2.0.1 fixture's conversion to the current format
	// as the converters' golden pairs pin it.
	parityConverted = "../formats/testdata/adjagent-sheet-4.0.0/" + kb.KBDir
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
	// paritySummaryAnchors is the anchor each hosted node's title takes on
	// its kb-summary.md, by title.
	paritySummaryAnchors = map[string]string{
		"A supporting lemma":                   "a-supporting-lemma",
		"Flow simulation":                      "flow-simulation",
		"Theorem 1":                            "theorem-1",
		"Equation (`eq:flow`) — Flow Dynamics": "equation-eqflow--flow-dynamics",
		"The gradient $`\\nabla f`$ is \"bounded\" when a < b & c":                                                                "the-gradient-nabla-f-is-bounded-when-a--b--c",
		"A deliberately long prose claim title that runs well past the three wrapped lines a sheet label allows before it is cut": "a-deliberately-long-prose-claim-title-that-runs-well-past-the-three-wrapped-lines-a-sheet-label-allows-before-it-is-cut",
		"The governance bifurcation":                "the-governance-bifurcation",
		"Smith, A. 2020. *Markets*. Fixture Press.": "smith-a-2020-markets-fixture-press",
	}
	// parityHosts is each hosted node's host leaf and the anchor of the
	// section its statement marker stands in, by id.
	parityHosts = map[string][2]string{
		"clm-aaa001": {"alpha/results.md", "results"},
		"clm-aaa002": {"alpha/results.md", "results"},
		"clm-aaa003": {"alpha/results.md", "results"},
		"clm-aaa004": {"alpha/results.md", "results"},
		"clm-bbb002": {"beta/governance.md", "governance"},
		"ana-aaa007": {"alpha/analysis.md", "a-supporting-lemma"},
		"exp-aaa008": {"alpha/experiments/flow.md", "flow-simulation"},
	}
	// parityVolumes is the conversion's volumes: the top-level directories
	// holding an index.md.
	parityVolumes = map[string]bool{"alpha": true, "beta": true, "gamma": true}
	// parityRationales is the rationale of each analysis and work record the
	// conversion leaves, which 2.0.1's show did not carry, by id.
	parityRationales = map[string]string{"ana-aaa007": "Fixture.", "wrk-omcjwo": "Fixture Press."}
	// parityModelScoredEdges is the conversion's edges carrying a
	// manually_scored flag, every one false: its scorable and demoted edges.
	parityModelScoredEdges = 9
)

// What the 2.0.1 fixture's graph holds that no golden shows, stated here.
var (
	// parityDependents is the nodes each claim's strength enters, by id, for
	// every claim with any.
	parityDependents = map[string]int{"clm-aaa001": 2, "clm-aaa002": 2}
	// parityPremisesMerged is the scoring edges into each claim that repeat
	// another's relation, other end and bearing, by id: the conversion merges
	// each into the edge it repeats.
	parityPremisesMerged = map[string]int{"clm-aaa001": 1}
	// parityEdgesMerged is every edge the conversion merges into one it
	// repeats.
	parityEdgesMerged = 1
)

// parityMintedIDs is the file stating each 2.0.1 work's and axiom's id and
// the id the conversion mints it, one tab-separated pair per line: the parity
// recipe runs each golden case's arguments through it, and the parity rules
// read the answers through it.
const parityMintedIDs = "testdata/parity-minted-ids.tsv"

// readParityMinted is parityMintedIDs' pairs, by 2.0.1 id.
func readParityMinted(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(parityMintedIDs)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		from, to, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s: %q is no tab-separated pair", parityMintedIDs, line)
		}
		out[from] = to
	}
	return out
}

// TestParityFactsAreTheConvertersGolden: the parity rules' statement of what
// the conversion leaves unhosted, where each hosted node is stated, which
// directories are volumes, the analyses' and works' rationales, how many edges
// are model-scored and the minted ids is what the converter's golden output
// holds.
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
	hosts := map[string][2]string{}
	for _, rel := range docs {
		body, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			t.Fatal(err)
		}
		for id, line := range kb.MarkerLines(body) {
			marked[id] = true
			hosts[id] = [2]string{rel, index.SectionSlug(body, line)}
		}
	}
	unhosted := []string{}
	for id, kind := range index.GraphKinds(g) {
		if markerHosted[kind] && !marked[id] {
			unhosted = append(unhosted, id)
		}
	}
	slices.Sort(unhosted)
	if !slices.Equal(unhosted, parityUnhosted) {
		t.Errorf("the converter's golden leaves %q unhosted, the parity rules state %q", unhosted, parityUnhosted)
	}
	if !maps.Equal(hosts, parityHosts) {
		t.Errorf("the converter's golden hosts %v, the parity rules state %v", hosts, parityHosts)
	}
	if volumes := kb.Volumes(docs); !maps.Equal(volumes, parityVolumes) {
		t.Errorf("the converter's golden has volumes %v, the parity rules state %v", volumes, parityVolumes)
	}
	rationales := map[string]string{}
	for _, n := range g.Analyses {
		rationales[n.ID] = n.Rationale
	}
	for _, n := range g.Works {
		rationales[n.ID] = n.Rationale
	}
	if !maps.Equal(rationales, parityRationales) {
		t.Errorf("the converter's golden gives the analyses and works the rationales %v, the parity rules state %v", rationales, parityRationales)
	}
	if flagged := len(g.Scorable) + len(g.Demoted); flagged != parityModelScoredEdges {
		t.Errorf("the converter's golden has %d scorable and demoted edges, the parity rules state %d", flagged, parityModelScoredEdges)
	}
	minted := map[string]string{}
	for _, n := range g.Works {
		minted[kb.WorkPrefix+"-"+n.Label] = n.ID
	}
	for _, n := range g.Axioms {
		minted[kb.AxiomPrefix+"-"+n.Label] = n.ID
	}
	if stated := readParityMinted(t); !maps.Equal(minted, stated) {
		t.Errorf("the converter's golden mints the 2.0.1 works and axioms %v, %s states %v", minted, parityMintedIDs, stated)
	}
}

// A golden-side rule turns a 2.0.1 answer into the answer the converted KB
// gives, each a designed outcome of the conversion; it reports whether it
// moved the answer. The rules run in this order.
var goldenRules = []struct {
	name  string
	apply func(f *conversionFacts, args []string, doc *yaml.Node) bool
}{
	{"an edge identity authored twice with equal bearings is one record: a claim's premise_count and stats' edges count it once", (*conversionFacts).mergeEdges},
	{"a node with no marker has a null canonical path and anchor", (*conversionFacts).unhostedCanonical},
	{"4.0.0 carries no strengthen-by: a claim record has no strengthen_by_count or strengthen_by, stats no strengthen_by_items", (*conversionFacts).dropStrengthenBy},
	{"no query counts citations: a claim record has no citation_count, stats no citation_edges", (*conversionFacts).dropCitationCounts},
	{"stats' subtree_aggregates is per volume: each volume's claims and experiments hosted under it", (*conversionFacts).volumeAggregates},
	{"4.0.0's register page is kb-summary.md, an entry anchored at the anchor the parity rules state for its title", (*conversionFacts).summaryPageAnchors},
	{"an unknown id is refused with the one no-node refusal, its remedy included", (*conversionFacts).noNodeRefusal},
	{"show names a node's host, host_anchor and summary entry in place of its canonical path and anchor, a work's and an axiom's label, its 2.0.1 key or label, and an analysis's and a work's rationale", (*conversionFacts).showIdentity},
	{"show reads a pending rigor or strength *pending*", (*conversionFacts).showPending},
	{"find does not page, a row its id, node_type, title, label (null on a claim), host, strength (*pending* where pending) and dependents; a case asks --kind claim --title, the claims by title 2.0.1's find listed", (*conversionFacts).findRows},
	{"a node's band is keyed strength_band", (*conversionFacts).strengthBandKey},
	{"every work's and axiom's id is minted: a 2.0.1 work or axiom id, its key or label now its label, reads as the id parity-minted-ids.tsv states the conversion mints it", (*conversionFacts).mintedIDs},
}

// strengthBandKey renames every build_band key of an answer strength_band.
func (f *conversionFacts) strengthBandKey(_ []string, doc *yaml.Node) bool {
	moved := false
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if i := keyAt(n, "build_band"); i >= 0 {
			n.Content[i].Value, moved = "strength_band", true
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc)
	return moved
}

// mintedIDs gives every scalar of an answer naming a 2.0.1 work or axiom id
// the id the conversion mints it.
func (f *conversionFacts) mintedIDs(_ []string, doc *yaml.Node) bool {
	moved := false
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if to, ok := f.minted[n.Value]; ok && n.Kind == yaml.ScalarNode {
			n.Value, moved = to, true
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc)
	return moved
}

// An answer-side rule removes from the converted KB's answer a key 3.0.0 adds,
// after checking where it stands and what it holds; it reports why not where
// the key is not as the rule states.
var answerRules = []struct {
	name  string
	apply func(a *answerFacts, args []string, doc *yaml.Node) string
}{
	{"a converted, unsaved KB's answer carries its migration record after kb-root", (*answerFacts).migrationRecord},
	{"stats' relations after bearing_model, as 3.0.0 adds it", (*answerFacts).statsRelations},
	{"manually_scored after a show record's rigor; stats' four flag counts", (*answerFacts).manuallyScored},
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

func setNull(n *yaml.Node) { *n = *null() }

func setInt(n *yaml.Node, v int) { *n = *integer(v) }

// quoted is s as a result document emits a string; key is s as a mapping key.
func quoted(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: s}
}

func key(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }

func null() *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"} }

func integer(v int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(v)}
}

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

// conversionFacts is what the golden-side rules read beside the stated facts
// they read directly.
type conversionFacts struct {
	unhosted map[string]bool   // parityUnhosted
	minted   map[string]string // 2.0.1 work or axiom id → its minted id, parityMintedIDs
}

func readConversionFacts(t *testing.T) *conversionFacts {
	t.Helper()
	f := &conversionFacts{unhosted: map[string]bool{}, minted: readParityMinted(t)}
	for _, id := range parityUnhosted {
		f.unhosted[id] = true
	}
	return f
}

// mergeEdges takes the edges parityPremisesMerged and parityEdgesMerged state
// merged from a claim record's premise_count and from stats' edges.
func (f *conversionFacts) mergeEdges(args []string, doc *yaml.Node) bool {
	moved := false
	for _, m := range mappings(doc) {
		if n := value(m, "premise_count"); n != nil && parityPremisesMerged[text(value(m, "id"))] > 0 {
			v, _ := strconv.Atoi(n.Value)
			setInt(n, v-parityPremisesMerged[text(value(m, "id"))])
			moved = true
		}
	}
	if args[0] == "stats" && parityEdgesMerged > 0 {
		n := value(value(doc, "results"), "edges")
		v, _ := strconv.Atoi(n.Value)
		setInt(n, v-parityEdgesMerged)
		moved = true
	}
	return moved
}

// volumeAggregates gives stats' subtree_aggregates, in place of 2.0.1's
// count, one record per volume of parityVolumes in path order: its claims and
// its experiments parityHosts states hosted under it.
func (f *conversionFacts) volumeAggregates(args []string, doc *yaml.Node) bool {
	n := value(value(doc, "results"), "subtree_aggregates")
	if args[0] != "stats" || n == nil {
		return false
	}
	out := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, v := range slices.Sorted(maps.Keys(parityVolumes)) {
		claims, experiments := 0, 0
		for id, h := range parityHosts {
			if !strings.HasPrefix(h[0], v+"/") {
				continue
			}
			switch kind, _ := kb.NodeIDKind(id); kind {
			case kb.NodeKindClaim:
				claims++
			case kb.NodeKindExperiment:
				experiments++
			}
		}
		out.Content = append(out.Content, key(v), &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
			Content: []*yaml.Node{key("claims"), integer(claims), key("experiments"), integer(experiments)}})
	}
	*n = *out
	return true
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

// dropStrengthenBy removes a claim record's strengthen_by_count and the
// strengthen_by list beside it, each where it stands directly after
// premise_count, and stats' strengthen_by_items after demoted_edges.
func (f *conversionFacts) dropStrengthenBy(args []string, doc *yaml.Node) bool {
	moved := false
	for _, m := range mappings(doc) {
		for _, key := range []string{"strengthen_by_count", "strengthen_by"} {
			if removeAfter(m, "premise_count", key) != nil {
				moved = true
			}
		}
	}
	if args[0] == "stats" && removeAfter(value(doc, "results"), "demoted_edges", "strengthen_by_items") != nil {
		moved = true
	}
	return moved
}

// summaryPageAnchors moves a record whose canonical path is a 2.0.1 register
// page onto the summary page in its place, anchored at the anchor the parity
// rules state for its title (paritySummaryAnchors); a title the map lacks keeps
// its anchor as it stands.
func (f *conversionFacts) summaryPageAnchors(_ []string, doc *yaml.Node) bool {
	moved := false
	for _, m := range mappings(doc) {
		p := value(m, "canonical_path")
		dir, page := path.Split(text(p))
		if page != "claim-rigor.md" {
			continue
		}
		p.Value = dir + kb.RegisterFile
		if a, anchor := value(m, "canonical_anchor"), paritySummaryAnchors[text(value(m, "title"))]; a != nil && anchor != "" {
			a.Value = anchor
		}
		moved = true
	}
	return moved
}

// cliNoArgumentCalls spells marked text as the answers, CLI documents, carry
// it, for a remedy whose calls take no argument: such a call is `kbase
// <command>` whatever the command tree. A call with arguments stays marked, so
// the answer differs from the golden and says so.
var cliNoArgumentCalls = argmark.Spelling{Call: func(inv argmark.Invocation) string {
	if len(inv.Args) > 0 || len(inv.Argv) > 0 {
		return argmark.Call(inv.Command, inv.Args)
	}
	return "kbase " + inv.Command
}}

// noNodeRefusal gives a show case's unknown-id refusal at <id> kb.NoNode's
// detail, and its remedy before the detail, where result.Item serializes it,
// both spelled as the command line spells them.
func (f *conversionFacts) noNodeRefusal(args []string, doc *yaml.Node) bool {
	refusals := value(doc, "refusals")
	if args[0] != "show" || refusals == nil {
		return false
	}
	moved := false
	for _, item := range refusals.Content {
		at := keyAt(item, "detail")
		if text(value(item, "check")) != kb.CheckUnknownID || text(value(item, "key")) != "<id>" || at < 0 || keyAt(item, "remedy") >= 0 {
			continue
		}
		detail, remedy := kb.NoNode(argmark.Positional("id"), args[1])
		remedy = cliNoArgumentCalls.Spell(remedy)
		item.Content[at+1].Value = detail
		item.Content = slices.Insert(item.Content, at,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "remedy"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: remedy})
		moved = true
	}
	return moved
}

// dropCitationCounts removes a claim record's citation_count where it stands
// directly after premise_count, and stats' citation_edges after
// demoted_edges.
func (f *conversionFacts) dropCitationCounts(args []string, doc *yaml.Node) bool {
	moved := false
	for _, m := range mappings(doc) {
		if removeAfter(m, "premise_count", "citation_count") != nil {
			moved = true
		}
	}
	if args[0] == "stats" && removeAfter(value(doc, "results"), "demoted_edges", "citation_edges") != nil {
		moved = true
	}
	return moved
}

// showIdentity gives a show record, in place of its canonical path and
// anchor, the host leaf and section anchor parityHosts states (null where
// it states none) and its summary entry: a claim's or a work's canonical path
// and anchor where it has them, an analysis's or an experiment's volume page
// at the anchor paritySummaryAnchors states for its title; a work or an axiom
// takes, after the summary entry, its label, its 2.0.1 id less the prefix; it
// appends the rationale parityRationales states.
func (f *conversionFacts) showIdentity(args []string, doc *yaml.Node) bool {
	m := value(doc, "results")
	at := keyAt(m, "canonical_path")
	if args[0] != "show" || at < 0 || keyAt(m, "canonical_anchor") != at+2 {
		return false
	}
	id, kind, path, anchor := args[1], text(value(m, "node_type")), m.Content[at+1], m.Content[at+3]
	host, hostAnchor, summary := null(), null(), null()
	h, hosted := parityHosts[id]
	if hosted {
		host, hostAnchor = quoted(h[0]), quoted(h[1])
	}
	switch {
	case (kind == kb.NodeKindClaim || kind == kb.NodeKindWork) && !isNull(path):
		summary = quoted(text(path) + "#" + text(anchor))
	case (kind == kb.NodeKindAnalysis || kind == kb.NodeKindExperiment) && hosted:
		summary = quoted(kb.SummaryPage(h[0], parityVolumes) + "#" + paritySummaryAnchors[text(value(m, "title"))])
	}
	m.Content[at].Value, m.Content[at+1] = "host", host
	m.Content[at+2].Value, m.Content[at+3] = "host_anchor", hostAnchor
	m.Content = slices.Insert(m.Content, at+4, key("summary"), summary)
	if kind == kb.NodeKindWork || kind == kb.NodeKindAxiom {
		_, label, _ := strings.Cut(id, "-")
		m.Content = slices.Insert(m.Content, at+6, key("label"), quoted(label))
	}
	if minted, ok := f.minted[id]; ok {
		id = minted
	}
	if r, ok := parityRationales[id]; ok {
		m.Content = append(m.Content, key("rationale"), quoted(r))
	}
	return true
}

// showPending gives a show answer's null rigor and strength *pending*: 2.0.1
// printed a pending value null.
func (f *conversionFacts) showPending(args []string, doc *yaml.Node) bool {
	if args[0] != "show" {
		return false
	}
	moved := false
	m := value(doc, "results")
	for _, k := range []string{"rigor", "strength"} {
		if v := value(m, k); isNull(v) {
			*v, moved = *quoted(kb.PendingLiteral), true
		}
	}
	return moved
}

// findRows gives each row of a find answer — 2.0.1's find listed claims, so
// each a claim — its node_type, a null label, its host leaf from parityHosts (null where it
// states none) and the dependents parityDependents states, in place of its
// build_band, and a null strength *pending*; find does not page, so the answer
// carries no offset or truncated.
func (f *conversionFacts) findRows(args []string, doc *yaml.Node) bool {
	results := listAnswer(doc)
	if args[0] != "find" || results == nil {
		return false
	}
	removeAfter(doc, "count", "offset")
	removeAfter(doc, "count", "truncated")
	for _, r := range results.Content {
		id := text(value(r, "id"))
		host := null()
		if h, ok := parityHosts[id]; ok {
			host = quoted(h[0])
		}
		strength := value(r, "strength")
		if isNull(strength) {
			strength = quoted(kb.PendingLiteral)
		}
		r.Content = []*yaml.Node{key("id"), value(r, "id"), key("node_type"), quoted(kb.NodeKindClaim), key("title"), value(r, "title"),
			key("label"), null(), key("host"), host, key("strength"), strength, key("dependents"), integer(parityDependents[id])}
	}
	return true
}

// answerFacts is what the answer-side rules read, gathered from the converted
// KB's answers before any rule removes a key.
type answerFacts struct {
	migration    string          // the first answer's migration record, encoded
	flaggedNodes map[string]bool // ids whose show record carries manually_scored
	encode       func(*yaml.Node) string
}

func gatherAnswerFacts(t *testing.T, answers []parityCase) *answerFacts {
	t.Helper()
	a := &answerFacts{flaggedNodes: map[string]bool{},
		encode: func(n *yaml.Node) string { return encodeNode(t, n) }}
	for _, c := range answers {
		if a.migration == "" {
			if m := value(c.doc, "migration"); m != nil {
				a.migration = a.encode(m)
			}
		}
		results := value(c.doc, "results")
		if c.args[0] == "show" && results != nil && results.Kind == yaml.MappingNode && value(results, "manually_scored") != nil {
			a.flaggedNodes[c.args[1]] = true
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

// statsRelations removes stats' relations, which stands directly after
// bearing_model and holds one record per relation in kb.Relations order.
func (a *answerFacts) statsRelations(args []string, doc *yaml.Node) string {
	results := value(doc, "results")
	if args[0] != "stats" || results == nil {
		return ""
	}
	relations := removeAfter(results, "bearing_model", "relations")
	if relations == nil || relations.Kind != yaml.MappingNode {
		return "stats' relations is not a mapping directly after bearing_model"
	}
	var names []string
	for i := 0; i < len(relations.Content); i += 2 {
		if relations.Content[i+1].Kind != yaml.MappingNode {
			return fmt.Sprintf("stats' relations %s is not a record", relations.Content[i].Value)
		}
		names = append(names, relations.Content[i].Value)
	}
	if !slices.Equal(names, kb.Relations) {
		return fmt.Sprintf("stats' relations holds %v, not one record per relation in %v", names, kb.Relations)
	}
	return ""
}

func (a *answerFacts) manuallyScored(args []string, doc *yaml.Node) string {
	results := value(doc, "results")
	if results == nil {
		return ""
	}
	switch {
	case args[0] == "show" && results.Kind == yaml.MappingNode && a.flaggedNodes[args[1]]:
		if v := removeAfter(results, "rigor", "manually_scored"); v == nil || text(v) != "false" {
			return "show's manually_scored is not false directly after rigor"
		}
	case args[0] == "stats":
		want := []struct {
			key   string
			count int
		}{{"rigor_manual", 0}, {"rigor_model", len(a.flaggedNodes)}, {"bearing_manual", 0}, {"bearing_model", parityModelScoredEdges}}
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
	facts := readConversionFacts(t)
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
