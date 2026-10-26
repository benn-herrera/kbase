package kb

import (
	"errors"
	"slices"
	"strings"
	"testing"
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

func TestWorkKey(t *testing.T) {
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

func TestFrontmatterRecognition(t *testing.T) {
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

// TestAxiomHeadings: each "### Axiom <label>: <title>" heading outside code is
// read with its label, its title trimmed and its 1-based line; a heading of
// another depth or form, or one inside a fence, is none.
func TestAxiomHeadings(t *testing.T) {
	text := "# Axioms\n\n### Axiom S1: The rule  \n\nBody.\n\n```\n### Axiom S2: Fenced\n```\n\n## Axiom S3: Too shallow\n\n### Axiom 4.b: Another\n"
	want := []AxiomHeading{{"S1", "The rule", 3}, {"4.b", "Another", 13}}
	if got := AxiomHeadings(text); !slices.Equal(got, want) {
		t.Errorf("headings = %+v, want %+v", got, want)
	}
}
