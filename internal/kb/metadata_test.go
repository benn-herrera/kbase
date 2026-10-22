package kb

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log"
)

// TestMintID: every id is the grammar's and clear of the existing set,
// and a large sample holds no two equal draws.
func TestMintID(t *testing.T) {
	grammar := PyRE(`^clm-[a-z0-9]{6}$`)
	// Two of 500 uniform draws over 36^6 bodies coincide with probability
	// about 6e-5; a larger sample makes the test flaky.
	const draws = 500
	existing := map[string]bool{}
	for range draws {
		id := MintID("clm", func(id string) bool { return existing[id] })
		if !grammar.MatchString(id) {
			t.Fatalf("MintID = %q, outside clm-[a-z0-9]{6}", id)
		}
		if existing[id] {
			t.Fatalf("MintID = %q, already in the existing set", id)
		}
		existing[id] = true
	}

	var offered []string
	got := MintID("clm", func(id string) bool { offered = append(offered, id); return len(offered) <= 3 })
	if len(offered) != 4 || got != offered[3] || !grammar.MatchString(got) {
		t.Errorf("three collisions then a free id: minted %q after offering %q", got, offered)
	}

	seen := map[string]bool{}
	for range draws {
		id := MintID("ana", func(string) bool { return false })
		if seen[id] {
			t.Fatalf("two of %d draws were both %q", draws, id)
		}
		seen[id] = true
	}
}

func TestPyFloatRepr(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{0.9, "0.9"}, {1, "1.0"}, {0, "0.0"}, {0.196, "0.196"}, {0.30000000000000004, "0.30000000000000004"},
		{1e-4, "0.0001"}, {1e-5, "1e-05"}, {1.5e-7, "1.5e-07"}, {1e16, "1e+16"}, {1e15, "1000000000000000.0"},
		{123456789, "123456789.0"}, {-0.5, "-0.5"},
	} {
		if got := PyFloatRepr(tc.in); got != tc.want {
			t.Errorf("PyFloatRepr(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitLines(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"trailing newline adds no line", "a\nb\n", []string{"a", "b"}},
		{"crlf is one break", "a\r\nb", []string{"a", "b"}},
		{"python's other breaks", "a\vb\fc\x1cd\u0085e f", []string{"a", "b", "c", "d", "e", "f"}},
		{"blank lines kept", "a\n\nb", []string{"a", "", "b"}},
		{"empty", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SplitLines(tc.text); !slices.Equal(got, tc.want) {
				t.Errorf("SplitLines(%q) = %q, want %q", tc.text, got, tc.want)
			}
			if got := strings.Join(SplitLinesKeepEnds(tc.text), ""); got != tc.text {
				t.Errorf("keepends parts rejoin to %q, want %q", got, tc.text)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Solidity — the dep gate", "solidity-the-dep-gate"},
		{"Rule : applied", "rule-applied"},
		{"INVARIANT-S2: Axiom numbering", "invariant-s2-axiom-numbering"},
		{"  Ünïcode Wörds_ok ", "ünïcode-wörds_ok"},
		{"(Parenthesised) Title!", "parenthesised-title"},
	} {
		if got := Slugify(tc.in); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWordBoundedIDs(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"see clm-abc123 and (clm-def456).", []string{"clm-abc123", "clm-def456"}},
		{"clm-abc1234 is too long", nil},
		{"éclm-abc123 follows a letter", nil},
		{"x-clm-abc123 follows a hyphen", []string{"clm-abc123"}},
	} {
		if got := ClaimIDs(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("ClaimIDs(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWorkTokens(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"wrk-smith2020", []string{"wrk-smith2020"}},
		{"wrk-key.with:punct/ok.", []string{"wrk-key.with:punct/ok"}},
		{"xwrk-a/wrk-b", []string{"wrk-b"}},
		{"pre-wrk-a", nil},
		{"work-smith2020", nil},
	} {
		if got := workTokens(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("workTokens(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	for id, want := range map[string]string{"wrk-smith2020": "smith2020", "work-smith2020": "", "clm-aaaaaa": ""} {
		if got := WorkKey(id); got != want {
			t.Errorf("WorkKey(%q) = %q, want %q", id, got, want)
		}
	}
}

// TestIDKinds: the minted kinds are clm, exp and ana; an ana- id is a node and
// a sup- id is not.
func TestIDKinds(t *testing.T) {
	if !slices.Equal(IDKinds, []string{"clm", "exp", "ana"}) {
		t.Errorf("IDKinds = %q", IDKinds)
	}
	if !IDPlaceholders["ana-xxxxxx"] || IDPlaceholders["sup-xxxxxx"] {
		t.Errorf("IDPlaceholders = %v", IDPlaceholders)
	}
	body := PyRE(`^` + IDBody() + `$`)
	for id, want := range map[string]bool{"clm-aaaaaa": true, "exp-aaaaaa": true, "ana-aaaaaa": true,
		"sup-aaaaaa": false, "ana-AAAAAA": false, "wrk-smith2020": false} {
		if got := body.MatchString(id); got != want {
			t.Errorf("IDBody() matches %q = %t, want %t", id, got, want)
		}
	}
	if got := PyRE(`^` + IDBody("ana") + `$`); !got.MatchString("ana-aaaaaa") || got.MatchString("sup-aaaaaa") {
		t.Errorf("IDBody(\"ana\") = %s", got)
	}
	if !slices.Equal(NodeKinds, []string{"claim", "analysis", "experiment", "axiom", "work"}) {
		t.Errorf("NodeKinds = %q, want five kinds", NodeKinds)
	}
	for _, kind := range NodeKinds {
		if kind == "support" || kind == "invariant" || kind == "framework" {
			t.Errorf("NodeKinds carries the retired word %q", kind)
		}
	}
}

// TestBearingLegal: every scoring pairing may carry a bearing; a work and an
// axiom found, a claim may refute, a lift or a refutation may reach an
// analysis, and neither an axiom nor a work supports.
func TestBearingLegal(t *testing.T) {
	for _, tc := range []struct {
		relation, source, target string
		want                     bool
	}{
		{RelationSupports, NodeKindClaim, NodeKindClaim, true},
		{RelationSupports, NodeKindExperiment, NodeKindAnalysis, true},
		{RelationFounds, NodeKindWork, NodeKindClaim, true},
		{RelationFounds, NodeKindWork, NodeKindAnalysis, true},
		{RelationFounds, NodeKindAxiom, NodeKindClaim, true},
		{RelationFounds, NodeKindAxiom, NodeKindAnalysis, true},
		{RelationLifts, NodeKindAnalysis, NodeKindClaim, true},
		{RelationLifts, NodeKindExperiment, NodeKindClaim, true},
		{RelationLifts, NodeKindAnalysis, NodeKindAnalysis, true},
		{RelationLifts, NodeKindExperiment, NodeKindAnalysis, true},
		{RelationRefutes, NodeKindClaim, NodeKindClaim, true},
		{RelationRefutes, NodeKindAnalysis, NodeKindClaim, true},
		{RelationRefutes, NodeKindExperiment, NodeKindClaim, true},
		{RelationRefutes, NodeKindClaim, NodeKindAnalysis, true},
		{RelationRefutes, NodeKindAnalysis, NodeKindAnalysis, true},
		{RelationRefutes, NodeKindExperiment, NodeKindAnalysis, true},
		{RelationSupports, NodeKindWork, NodeKindClaim, false},
		{RelationSupports, NodeKindAxiom, NodeKindClaim, false},
		{RelationSupports, NodeKindAxiom, NodeKindAnalysis, false},
		{RelationFounds, NodeKindExperiment, NodeKindClaim, false},
		{RelationFounds, NodeKindAnalysis, NodeKindAnalysis, false},
		{RelationRefutes, NodeKindWork, NodeKindClaim, false},
		{RelationRefutes, NodeKindAxiom, NodeKindClaim, false},
		{RelationLifts, NodeKindClaim, NodeKindClaim, false},
		{RelationLifts, NodeKindClaim, NodeKindAnalysis, false},
		{RelationFounds, NodeKindClaim, NodeKindClaim, false},
		{RelationReferences, NodeKindClaim, NodeKindClaim, false},
		{RelationDemoted, NodeKindClaim, NodeKindClaim, false},
	} {
		if got := BearingLegal(tc.relation, tc.source, tc.target); got != tc.want {
			t.Errorf("BearingLegal(%s, %s, %s) = %t, want %t", tc.relation, tc.source, tc.target, got, tc.want)
		}
	}
}

// TestScoring: the four scoring relations carry a bearing, and the two
// outside the walk do not.
func TestScoring(t *testing.T) {
	for _, tc := range []struct {
		relation string
		want     bool
	}{
		{RelationSupports, true},
		{RelationFounds, true},
		{RelationLifts, true},
		{RelationRefutes, true},
		{RelationReferences, false},
		{RelationDemoted, false},
	} {
		if got := Scoring(tc.relation); got != tc.want {
			t.Errorf("Scoring(%s) = %t, want %t", tc.relation, got, tc.want)
		}
	}
}

// TestBearingWalk: the score run's bearing pass asks about three relations and
// no more, a founds edge's bearing being the user's rather than a model's.
func TestBearingWalk(t *testing.T) {
	for _, tc := range []struct {
		relation string
		want     bool
	}{
		{RelationSupports, true},
		{RelationFounds, false},
		{RelationLifts, true},
		{RelationRefutes, true},
		{RelationReferences, false},
		{RelationDemoted, false},
	} {
		if got := BearingWalk(tc.relation); got != tc.want {
			t.Errorf("BearingWalk(%s) = %t, want %t", tc.relation, got, tc.want)
		}
	}
}

func TestFrontmatterListShapes(t *testing.T) {
	for _, body := range []string{
		"kind: leaf\nclaims: [clm-aaaaaa, clm-bbbbbb]",
		"kind: leaf\nclaims: [clm-aaaaaa,\n         clm-bbbbbb]",
		"kind: leaf\nclaims:\n  - clm-aaaaaa\n  - clm-bbbbbb",
		"kind: leaf\nclaims:\n- clm-aaaaaa\n- not an id\n- clm-bbbbbb",
	} {
		fm, err := ParseFrontmatter("---\n" + body + "\n---\n# T\n")
		if err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		claims, err := fm.ListOrEmpty("claims")
		if err != nil || !slices.Equal(claims, []string{"clm-aaaaaa", "clm-bbbbbb"}) {
			t.Errorf("claims of %q = %q, %v", body, claims, err)
		}
	}
	// A fenced block whose first non-blank line opens no kebab-case key is a
	// thematic break opening prose, and the document has no frontmatter.
	for _, text := range []string{"no frontmatter here", "--- a rule, not a fence\n", "---\nunclosed: true\n",
		"---\nSome notes: with: colons\n---\n", "---\nPlain prose.\n---\n", "---\n- a list\n---\n", "---\n\nkey with space: x\n---\n"} {
		if fm, err := ParseFrontmatter(text); fm != nil || err != nil {
			t.Errorf("%q parsed to %v, %v; want none", text, fm, err)
		}
	}
	leaf := "---\nSome notes: with: colons\n---\n# T\n"
	if end := FrontmatterEnd(leaf); end != 0 {
		t.Errorf("a prose block's frontmatter ends at %d, want 0", end)
	}
	for name, read := range map[string]func(string, string) error{
		"ParseLeaf":           func(text, rel string) error { _, err := ParseLeaf(text, rel); return err },
		"ParseExperimentLeaf": func(text, rel string) error { _, err := ParseExperimentLeaf(text, rel); return err },
		"ParseAnalysisLeaf":   func(text, rel string) error { _, err := ParseAnalysisLeaf(text, rel); return err },
	} {
		if err := read(leaf, "v/l.md"); err != nil {
			t.Errorf("%s of a leaf opening with a prose block: %v", name, err)
		}
	}
	if fm, err := ParseFrontmatter("---\n---\n# T\n"); fm == nil || len(fm) != 0 || err != nil {
		t.Errorf("an empty frontmatter parsed to %#v, %v; want empty and present", fm, err)
	}
	// A block opening with a key is frontmatter, and one no reader can take
	// is malformed, never prose.
	for _, text := range []string{"---\nkey: [unclosed\n---\n", "---\nkind: leaf\n- stray\n---\n", "---\nk: 1\nk: 2\n---\n", "---\nk:\n  nested: 1\n---\n"} {
		var mal MalformedError
		if _, err := ParseFrontmatter(text); !errors.As(err, &mal) {
			t.Errorf("%q: err = %v, want a MalformedError", text, err)
		}
	}
}

// TestNodeDeclarations: a leaf's experiment and analysis lists read into
// nodes, their pair lists in list order, a pair whose value is no bearing
// malformed, and an entry without its id refused.
func TestNodeDeclarations(t *testing.T) {
	leaf := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n    rigor: 1.0\n" +
		"    supports:\n      - clm-aaaaaa: 0.8\n    lifts:\n      - clm-bbbbbb: 0.6\n      - clm-eeeeee: 1e-05\n" +
		"    refutes:\n      - clm-dddddd: 0.2\n  - exp-id: exp-eeeeee\n    status: pending\n" +
		"analysis-nodes:\n  - ana-id: ana-ffffff\n    lifts:\n      - clm-aaaaaa: \"*pending*\"\n      - clm-bbbbbb: 0.5\n" +
		"    refutes:\n      - clm-dddddd: \"*pending*\"\n" +
		"---\n[↑ V](index.md)\n\n# The leaf\n"
	pairs := func(edges []Edge) []string {
		var out []string
		for _, e := range edges {
			out = append(out, e.Relation+" "+e.Target+" "+BearingText(e.Bearing))
		}
		return out
	}
	exps, err := ParseExperimentLeaf(leaf, "v/l.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(exps) != 2 || exps[0].ID != "exp-cccccc" || exps[0].Title != "The leaf" || exps[0].CanonicalPath != "v/l.md" ||
		exps[0].CanonicalAnchor != "the-leaf" || exps[0].Status != "run" {
		t.Fatalf("experiments = %+v", exps)
	}
	if exps[1].ID != "exp-eeeeee" || exps[1].Title != "The leaf" || exps[1].Status != "pending" {
		t.Errorf("second experiment = %+v", exps[1])
	}
	if exps[0].Rigor == nil || *exps[0].Rigor != 1 || exps[1].Rigor != nil {
		t.Errorf("rigor = %v, %v; want 1 and none", exps[0].Rigor, exps[1].Rigor)
	}
	for name, got := range map[string][]string{
		"supports": pairs(exps[0].Supports), "lifts": pairs(exps[0].Lifts), "refutes": pairs(exps[0].Refutes),
	} {
		want := map[string][]string{
			"supports": {"supports clm-aaaaaa 0.8"},
			"lifts":    {"lifts clm-bbbbbb 0.6", "lifts clm-eeeeee 1e-05"},
			"refutes":  {"refutes clm-dddddd 0.2"},
		}[name]
		if !slices.Equal(got, want) {
			t.Errorf("experiment %s = %q, want %q", name, got, want)
		}
	}
	anas, err := ParseAnalysisLeaf(leaf, "v/l.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(anas) != 1 || anas[0].ID != "ana-ffffff" ||
		!slices.Equal(pairs(anas[0].Lifts), []string{"lifts clm-aaaaaa " + PendingLiteral, "lifts clm-bbbbbb 0.5"}) ||
		!slices.Equal(pairs(anas[0].Refutes), []string{"refutes clm-dddddd " + PendingLiteral}) {
		t.Errorf("analyses = %+v", anas)
	}
	fm, err := ParseFrontmatter(leaf)
	if err != nil {
		t.Fatal(err)
	}
	if got := DeclaredNodeIDs(fm); !slices.Equal(got, []string{"exp-cccccc", "exp-eeeeee", "ana-ffffff"}) {
		t.Errorf("DeclaredNodeIDs = %q", got)
	}
	var mal MalformedError
	if _, err := ParseExperimentLeaf("---\nkind: leaf\nexperiment-nodes:\n  - status: run\n---\n", "x.md"); !errors.As(err, &mal) {
		t.Errorf("an entry with no exp-id: err = %v, want a MalformedError", err)
	}
	if _, err := ParseExperimentLeaf("---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n---\n", "x.md"); !errors.As(err, &mal) {
		t.Errorf("an experiment with no status: err = %v, want a MalformedError", err)
	}
	for _, entry := range []string{"      - clm-aaaaaa:\n", "      - clm-aaaaaa: 1.5\n", "      - clm-aaaaaa: sometimes\n"} {
		text := "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-ffffff\n    lifts:\n" + entry + "---\n"
		var faulted *AnnotationError
		if _, err := ParseAnalysisLeaf(text, "x.md"); !errors.As(err, &faulted) || len(faulted.Faults) != 1 {
			t.Fatalf("a pair line %q: err = %v, want one AnnotationFault", entry, err)
		}
		if f := faulted.Faults[0]; f.Field != LiftsKey || f.Path != "x.md" || f.Line != 6 {
			t.Errorf("a pair line %q: fault = %+v, want %s at x.md:6", entry, f, LiftsKey)
		}
	}
	// A pair whose value is a list or a boolean is no bearing either: it is
	// refused, never skipped.
	for _, entry := range []string{"      - clm-aaaaaa: [0.5]\n", "      - clm-aaaaaa: true\n"} {
		text := "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-ffffff\n    lifts:\n" + entry + "---\n"
		var faulted *AnnotationError
		if _, err := ParseAnalysisLeaf(text, "x.md"); !errors.As(err, &faulted) {
			t.Errorf("a pair line %q: err = %v, want an AnnotationFault", entry, err)
		}
	}
	if _, err := ParseAnalysisLeaf("---\nkind: leaf\nanalysis-nodes:\n  - ana-id: sup-ffffff\n---\n", "x.md"); !errors.As(err, &mal) {
		t.Errorf("a sup- id under analysis-nodes: err = %v, want a MalformedError", err)
	}
}

// axiomSourceOf is a KB whose files are the given kb-root-relative ones.
func axiomSourceOf(files map[string]string) *Source {
	m := map[string][]byte{}
	for name, text := range files {
		m["kb-root/"+name] = []byte(text)
	}
	return MigratedSource("/kb/kb-root", FormatVersion, m, nil)
}

// TestParseAxioms: axioms.md's Axiom headings are the axioms, each anchored at
// its own heading; a legacy document's INVARIANT headings and Axiom bullets
// are read where axioms.md is absent, a bullet anchored at INVARIANT-S2's
// heading.
func TestParseAxioms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []AxiomNode
	}{
		{"axioms.md", map[string]string{AxiomsFile: "# Axioms\n\n### Axiom S1: The design rule\n\nBody.\n\n### Axiom 1: One\n"},
			[]AxiomNode{
				{NodeKindAxiom, "axm-S1", "The design rule", AxiomsFile, "axiom-s1-the-design-rule"},
				{NodeKindAxiom, "axm-1", "One", AxiomsFile, "axiom-1-one"},
			}},
		{"legacy invariants.md", map[string]string{InvariantsFile: "# Invariants\n\n### INVARIANT-S1: First rule\n\nBody.\n\n" +
			"### INVARIANT-S2: Axiom numbering\n\n- Axiom 1: **One** — a bullet.\n"},
			[]AxiomNode{
				{NodeKindAxiom, "axm-S1", "First rule", InvariantsFile, "invariant-s1-first-rule"},
				{NodeKindAxiom, "axm-S2", "Axiom numbering", InvariantsFile, "invariant-s2-axiom-numbering"},
				{NodeKindAxiom, "axm-1", "One", InvariantsFile, "invariant-s2-axiom-numbering"},
			}},
		{"legacy AGENTS.md", map[string]string{AgentsFile: "### INVARIANT-S1: First rule\n"},
			[]AxiomNode{{NodeKindAxiom, "axm-S1", "First rule", AgentsFile, "invariant-s1-first-rule"}}},
		{"axioms.md shadows the legacy documents", map[string]string{
			AxiomsFile:     "### Axiom S1: The rule\n",
			InvariantsFile: "### INVARIANT-S9: Stale\n",
			AgentsFile:     "### INVARIANT-S8: Stalest\n",
		}, []AxiomNode{{NodeKindAxiom, "axm-S1", "The rule", AxiomsFile, "axiom-s1-the-rule"}}},
		{"no declaration document", map[string]string{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAxioms(axiomSourceOf(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("axioms = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestLegacyAxiomDeclarations: the scan reports each declaration AGENTS.md
// still carries, by path, 1-based line and token as written, and nothing where
// the KB has no AGENTS.md.
func TestLegacyAxiomDeclarations(t *testing.T) {
	src := axiomSourceOf(map[string]string{AxiomsFile: "### Axiom S1: The rule\n",
		AgentsFile: "# Agents\n\n### INVARIANT-S1: First rule\n\n- Axiom 2: **Two**\n"})
	want := []LegacyDeclaration{
		{AgentsFile, 3, "INVARIANT-S1"},
		{AgentsFile, 5, "Axiom 2"},
	}
	if got := LegacyAxiomDeclarations(src); !slices.Equal(got, want) {
		t.Errorf("declarations = %+v, want %+v", got, want)
	}
	if got := LegacyAxiomDeclarations(axiomSourceOf(map[string]string{AxiomsFile: "### Axiom S1: The rule\n"})); got != nil {
		t.Errorf("declarations without an AGENTS.md = %+v, want none", got)
	}
}

func TestReplaceOrInsertFrontmatterField(t *testing.T) {
	for _, tc := range []struct {
		name, doc, field, anchor, want string
		ids                            []string
	}{
		{"replaces a wrapped list whole",
			"---\nkind: index\nsubtree-claims: [clm-aaaaaa,\n  clm-bbbbbb]\nx: 1\n---\nbody",
			"subtree-claims", "kind:",
			"---\nkind: index\nsubtree-claims: [clm-cccccc]\nx: 1\n---\nbody",
			[]string{"clm-cccccc"}},
		{"replaces a block list whole",
			"---\nkind: index\nsubtree-claims:\n  - clm-aaaaaa\n- clm-bbbbbb\nkb-format: \"1.0.0\"\n---\n",
			"subtree-claims", "kind:",
			"---\nkind: index\nsubtree-claims: []\nkb-format: \"1.0.0\"\n---\n",
			[]string{}},
		{"inserts after the anchor that ended the frontmatter",
			"---\nkind: index\nsubtree-claims: []\n---\n",
			"subtree-experiments", "subtree-claims:",
			"---\nkind: index\nsubtree-claims: []\nsubtree-experiments: []\n---\n",
			[]string{}},
		{"inserts at the top where the anchor is absent",
			"---\nx: 1\n---\n",
			"subtree-claims", "kind:",
			"---\nsubtree-claims: [clm-aaaaaa]\nx: 1\n---\n",
			[]string{"clm-aaaaaa"}},
		{"takes the body's own terminator",
			"---\r\nkind: index\r\n---\r\n",
			"subtree-claims", "kind:",
			"---\r\nkind: index\r\nsubtree-claims: []\r\n---\r\n",
			[]string{}},
		{"fills an empty frontmatter",
			"---\n---\nbody\n",
			"subtree-claims", "kind:",
			"---\nsubtree-claims: []\n---\nbody\n",
			[]string{}},
		{"leaves a document without frontmatter alone", "# no frontmatter", "subtree-claims", "kind:", "# no frontmatter", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReplaceOrInsertFrontmatterField(tc.doc, tc.field, tc.ids, tc.anchor); got != tc.want {
				t.Errorf("got\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

const register = `# Register

<!-- id: clm-orphan -->

## First Claim
<!-- id: clm-aaaaaa -->

> **Leaf references:** stale

### Rigor
- rigor: 0.80
- rationale: first line
  continues here.
- rests-on:
  - clm-bbbbbb — Second Claim (strength 0.60) (bearing 0.4) [ctx note]
  - clm-zzzzzz — unknown (strength 0.10) (bearing 0.2)
  - Axiom S2 — Axiom numbering (bearing 0.8) [frame]
  - wrk-smith2020 — Smith (bearing 0.5)
  - *(none entry-local)*
- refutes:
  - clm-cccccc: 0.7
  - clm-bbbbbb: *pending*
- strengthen-by:
  - add a lemma citing clm-bbbbbb
    over two lines

### Strength
- strength: 0.10 (unsubstantiated) [= min(0.80, 0.10)]

---

## Second Claim
<!-- id: clm-bbbbbb -->

### Rigor
- rigor: *pending*

### Strength
- strength: *pending*

---

## Work
<!-- id: wrk-smith2020 -->

### Rigor
- rigor: 0.70
- rationale: sound.
`

func TestParseClaimEntries(t *testing.T) {
	known := map[string]bool{"clm-aaaaaa": true, "clm-bbbbbb": true}
	entries := ParseClaimEntries(register, "claim-rigor.md", known, log.Discard())
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (the unbound marker yields none)", len(entries))
	}
	e := entries[0]
	if e.Title != "First Claim" || e.CanonicalAnchor != "first-claim" || *e.Rigor != 0.8 {
		t.Errorf("entry = %+v", e)
	}
	if *e.Strength != 0.1 || *e.BuildStatus != "unsubstantiated" || e.StrengthTrace != " [= min(0.80, 0.10)]" {
		t.Errorf("strength line read as %v %q %q", *e.Strength, *e.BuildStatus, e.StrengthTrace)
	}
	if e.Rationale != "first line continues here." {
		t.Errorf("rationale = %q", e.Rationale)
	}
	var got []string
	for _, d := range e.Premises {
		ctx := ""
		if d.Context != nil {
			ctx = *d.Context
		}
		got = append(got, d.Relation+" "+d.Source+" "+d.TargetKind+" "+d.Target+" ["+ctx+"]")
	}
	want := []string{
		"supports clm-bbbbbb claim clm-aaaaaa [ctx note]",
		"founds axm-S2 claim clm-aaaaaa [frame]",
		"founds wrk-smith2020 claim clm-aaaaaa []",
	}
	if !slices.Equal(got, want) {
		t.Errorf("premises =\n%q\nwant\n%q", got, want)
	}
	if p := e.Premises[0]; p.SourceStrengthRecorded == nil || *p.SourceStrengthRecorded != 0.6 || p.Bearing != (Bearing{Value: 0.4}) {
		t.Errorf("first premise = %+v", p)
	}
	if p := e.Premises[1]; p.Bearing != (Bearing{Value: 0.8}) || p.SourceStrengthRecorded != nil || p.Relation != RelationFounds {
		t.Errorf("axiom premise = %+v", p)
	}
	if p := e.Premises[2]; p.Bearing != (Bearing{Value: 0.5}) || p.SourceStrengthRecorded != nil {
		t.Errorf("work premise = %+v", p)
	}
	if len(e.Refutes) != 2 {
		t.Fatalf("refutes = %+v", e.Refutes)
	}
	if r := e.Refutes[0]; r.Source != "clm-aaaaaa" || r.Target != "clm-cccccc" || r.Relation != RelationRefutes ||
		r.TargetKind != NodeKindClaim || r.Bearing != (Bearing{Value: 0.7}) {
		t.Errorf("refute = %+v", r)
	}
	if r := e.Refutes[1]; r.Source != "clm-aaaaaa" || r.Target != "clm-bbbbbb" || !r.Bearing.Pending {
		t.Errorf("pending refute = %+v", r)
	}
	if len(e.StrengthenBy) != 1 || e.StrengthenBy[0].Text != "add a lemma citing clm-bbbbbb over two lines" ||
		!slices.Equal(e.StrengthenBy[0].MentionedIDs, []string{"clm-bbbbbb"}) {
		t.Errorf("strengthen-by = %+v", e.StrengthenBy)
	}
	if entries[1].Rigor != nil || entries[1].Strength != nil {
		t.Errorf("pending entry read as %+v", entries[1])
	}
	works := ParseWorkEntries(register, "claim-rigor.md")
	if len(works) != 1 || works[0].Key != "smith2020" || *works[0].Rigor != 0.7 || works[0].Rationale != "sound." {
		t.Errorf("works = %+v", works)
	}
}

// TestParseDemoted reads a demoted list beside a references list: each
// bullet its target, context and origin, a bullet with no annotation no
// origin, an unknown target dropped, and the fields around it unfolded.
func TestParseDemoted(t *testing.T) {
	text := "## A\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n- rigor: 0.5\n- references:\n  - clm-bbbbbb — B\n" +
		"- demoted:\n  - clm-bbbbbb — B (with a paren) (origin cited) [ctx]\n  - clm-cccccc — C\n  - clm-zzzzzz — Z (origin inferred)\n" +
		"- rationale: Shown.\n\n### Strength\n- strength: *pending*\n"
	known := map[string]bool{"clm-aaaaaa": true, "clm-bbbbbb": true, "clm-cccccc": true}
	entries := ParseClaimEntries(text, "claim-rigor.md", known, log.Discard())
	if len(entries) != 1 {
		t.Fatalf("got %d entries", len(entries))
	}
	e := entries[0]
	var got []string
	for _, d := range e.Demoted {
		ctx := ""
		if d.Context != nil {
			ctx = *d.Context
		}
		got = append(got, d.Relation+" "+d.Target+" "+d.Origin+" ["+ctx+"]")
	}
	want := []string{"demoted clm-bbbbbb cited [ctx]", "demoted clm-cccccc  []"}
	if !slices.Equal(got, want) {
		t.Errorf("demoted = %q, want %q", got, want)
	}
	if len(e.References) != 1 || e.References[0].Target != "clm-bbbbbb" || e.Rationale != "Shown." {
		t.Errorf("references %+v, rationale %q", e.References, e.Rationale)
	}
}

func TestRegisterBinding(t *testing.T) {
	entries := LocateRegisterEntries(register)
	if len(entries) != 4 || entries[0].Bound() || !entries[1].Bound() || !entries[1].Adjacent {
		t.Fatalf("entries = %+v", entries)
	}
	if mis := MisBoundEntries(entries); len(mis) != 0 {
		t.Errorf("well-formed register reports mis-bound %+v", mis)
	}
	shared := strings.Replace(register, "## Second Claim\n", "", 1)
	if mis := MisBoundEntries(LocateRegisterEntries(shared)); len(mis) != 2 {
		t.Errorf("two markers under one heading: mis-bound %+v, want both", mis)
	}
	bands := LocateLeafReferenceFooters(register)
	if len(bands) != 2 || bands[0].FooterLine != 7 || bands[1].FooterLine != -1 {
		t.Errorf("footer bands = %+v", bands)
	}
}
