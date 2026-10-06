package kb

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log"
)

// TestMintID: every id is kb_tools' grammar and clear of the existing set,
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
		id := MintID("sup", func(string) bool { return false })
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
		{"work-smith2020", []string{"work-smith2020"}},
		{"work-key.with:punct/ok.", []string{"work-key.with:punct/ok"}},
		{"xwork-a/work-b", []string{"work-b"}},
		{"pre-work-a", nil},
	} {
		if got := workTokens(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("workTokens(%q) = %q, want %q", tc.in, got, tc.want)
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
		"ParseSupportLeaf":    func(text, rel string) error { _, err := ParseSupportLeaf(text, rel); return err },
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

// TestNodeDeclarations: a leaf's experiment and support lists read into
// nodes, their pairs in list order, a pair whose score reads as no number
// skipped, and an entry without its id refused.
func TestNodeDeclarations(t *testing.T) {
	leaf := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n    strengthens:\n      - clm-aaaaaa: 0.8\n      - clm-bbbbbb: 1e-05\n" +
		"      - clm-dddddd: \"*pending*\"\n  - exp-id: exp-eeeeee\n    status: pending\n" +
		"support-nodes:\n  - sup-id: sup-ffffff\n    supports:\n      - clm-aaaaaa: \"*pending*\"\n      - clm-bbbbbb: 0.5\n---\n[↑ V](index.md)\n\n# The leaf\n"
	exps, err := ParseExperimentLeaf(leaf, "v/l.md")
	if err != nil {
		t.Fatal(err)
	}
	wantExps := []ExperimentNode{
		{ID: "exp-cccccc", Title: "The leaf", CanonicalPath: "v/l.md", CanonicalAnchor: "the-leaf", Status: "run",
			Strengthens: []StrengthensPair{{"clm-aaaaaa", 0.8}, {"clm-bbbbbb", 1e-05}}},
		{ID: "exp-eeeeee", Title: "The leaf", CanonicalPath: "v/l.md", CanonicalAnchor: "the-leaf", Status: "pending"},
	}
	if !reflect.DeepEqual(exps, wantExps) {
		t.Errorf("experiments = %+v, want %+v", exps, wantExps)
	}
	sups, err := ParseSupportLeaf(leaf, "v/l.md")
	if err != nil {
		t.Fatal(err)
	}
	wantSups := []SupportPair{{"clm-aaaaaa", Fraction{Set: true, Pending: true}}, {"clm-bbbbbb", Fraction{Set: true, Value: 0.5}}}
	if len(sups) != 1 || sups[0].ID != "sup-ffffff" || !reflect.DeepEqual(sups[0].Supports, wantSups) {
		t.Errorf("supports = %+v, want sup-ffffff with %+v", sups, wantSups)
	}
	fm, err := ParseFrontmatter(leaf)
	if err != nil {
		t.Fatal(err)
	}
	if got := DeclaredNodeIDs(fm); !slices.Equal(got, []string{"exp-cccccc", "exp-eeeeee", "sup-ffffff"}) {
		t.Errorf("DeclaredNodeIDs = %q", got)
	}
	var mal MalformedError
	if _, err := ParseExperimentLeaf("---\nkind: leaf\nexperiment-nodes:\n  - status: run\n---\n", "x.md"); !errors.As(err, &mal) {
		t.Errorf("an entry with no exp-id: err = %v, want a MalformedError", err)
	}
	if _, err := ParseExperimentLeaf("---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n---\n", "x.md"); !errors.As(err, &mal) {
		t.Errorf("an experiment with no status: err = %v, want a MalformedError", err)
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

### Quality
- confidence: 0.80
- solidity: 0.10 (refuted, do not use) [= min(0.80, 0.10)]
- rationale: first line
  continues here.
- depends-on:
  - clm-bbbbbb — Second Claim (solidity 0.60) [ctx note]
  - clm-zzzzzz — unknown (solidity 0.10)
  - INVARIANT-S2 (frame)
  - Axiom 3 and Axiom 12
  - work-smith2020 — Smith (applicability 0.5)
  - *(none entry-local)*
- strengthen-by:
  - add a lemma citing clm-bbbbbb
    over two lines

---

## Second Claim
<!-- id: clm-bbbbbb -->

### Quality
- confidence: *pending*
- solidity: *pending*

---

## Work
<!-- id: work-smith2020 -->

### Quality
- strength: 0.70
- rationale: sound.
`

func TestParseClaimEntries(t *testing.T) {
	known := map[string]bool{"clm-aaaaaa": true, "clm-bbbbbb": true}
	entries := ParseClaimEntries(register, "claim-quality.md", known, log.Discard())
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (the unbound marker yields none)", len(entries))
	}
	e := entries[0]
	if e.Title != "First Claim" || e.CanonicalAnchor != "first-claim" || *e.Confidence != 0.8 {
		t.Errorf("entry = %+v", e)
	}
	if *e.Solidity != 0.1 || *e.BuildStatus != "refuted, do not use" || e.SolidityTrace != " [= min(0.80, 0.10)]" {
		t.Errorf("solidity line read as %v %q %q", *e.Solidity, *e.BuildStatus, e.SolidityTrace)
	}
	if e.Rationale != "first line continues here." {
		t.Errorf("rationale = %q", e.Rationale)
	}
	var got []string
	for _, d := range e.DependsOn {
		ctx := ""
		if d.Context != nil {
			ctx = *d.Context
		}
		got = append(got, d.Relation+" "+d.TargetKind+" "+d.Target+" ["+ctx+"]")
	}
	want := []string{
		"depends claim clm-bbbbbb [ctx note]",
		"depends invariant INVARIANT-S2 [frame]",
		"depends axiom axiom-3 []",
		"depends axiom axiom-12 []",
		"rests-on work work-smith2020 []",
	}
	if !slices.Equal(got, want) {
		t.Errorf("depends-on =\n%q\nwant\n%q", got, want)
	}
	if f := e.DependsOn[4].Fraction; !f.Set || f.Pending || f.Value != 0.5 {
		t.Errorf("applicability = %+v", f)
	}
	if len(e.StrengthenBy) != 1 || e.StrengthenBy[0].Text != "add a lemma citing clm-bbbbbb over two lines" ||
		!slices.Equal(e.StrengthenBy[0].MentionedIDs, []string{"clm-bbbbbb"}) {
		t.Errorf("strengthen-by = %+v", e.StrengthenBy)
	}
	if entries[1].Confidence != nil || entries[1].Solidity != nil {
		t.Errorf("pending entry read as %+v", entries[1])
	}
	works := ParseWorkEntries(register, "claim-quality.md")
	if len(works) != 1 || works[0].Key != "smith2020" || *works[0].Strength != 0.7 || works[0].Rationale != "sound." {
		t.Errorf("works = %+v", works)
	}
}

// TestParseDemoted reads a demoted list beside a references list: each
// bullet its target, context and origin, a bullet with no annotation no
// origin, an unknown target dropped, and the fields around it unfolded.
func TestParseDemoted(t *testing.T) {
	text := "## A\n<!-- id: clm-aaaaaa -->\n\n### Quality\n- confidence: 0.5\n- references:\n  - clm-bbbbbb — B\n" +
		"- demoted:\n  - clm-bbbbbb — B (with a paren) (origin cited) [ctx]\n  - clm-cccccc — C\n  - clm-zzzzzz — Z (origin inferred)\n" +
		"- solidity: *pending*\n- rationale: Shown.\n"
	known := map[string]bool{"clm-aaaaaa": true, "clm-bbbbbb": true, "clm-cccccc": true}
	entries := ParseClaimEntries(text, "claim-quality.md", known, log.Discard())
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
