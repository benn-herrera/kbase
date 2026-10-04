package index

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

func TestRoundHalfUp2(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0.125, 0.13}, {0.196, 0.2}, {0.005, 0.01}, {0.004999, 0.0}, {2.675, 2.68}, {-0.125, -0.13}, {0.9, 0.9}, {0.18, 0.18},
	} {
		if got := RoundHalfUp2(tc.in); got != tc.want {
			t.Errorf("RoundHalfUp2(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func f(v float64) *float64 { return &v }

func claimEntry(id string, confidence *float64, deps ...kb.Edge) kb.ClaimEntry {
	for i := range deps {
		deps[i].Source = id
	}
	return kb.ClaimEntry{ID: id, Confidence: confidence, DependsOn: deps}
}

func dep(target string) kb.Edge {
	return kb.Edge{Target: target, Relation: "depends", TargetKind: "claim"}
}

func restsOn(work string, fraction kb.Fraction) kb.Edge {
	return kb.Edge{Target: work, Relation: "rests-on", TargetKind: "work", Fraction: fraction}
}

func TestComputeSolidity(t *testing.T) {
	st := kb.State{
		ClaimEntries: []kb.ClaimEntry{
			claimEntry("clm-base01", f(0.9)),
			claimEntry("clm-weak01", f(0.4), dep("clm-base01")),
			claimEntry("clm-chain1", f(0.8), dep("clm-weak01"), kb.Edge{Target: "INVARIANT-S1", Relation: "depends", TargetKind: "invariant"}),
			claimEntry("clm-pend01", nil),
			claimEntry("clm-poisn1", f(0.9), dep("clm-pend01")),
			claimEntry("clm-lift01", f(0.2)),
			claimEntry("clm-rescu1", nil),
			claimEntry("clm-work00", f(0.9), restsOn("work-a", kb.Fraction{Set: true, Value: 0})),
			claimEntry("clm-work01", f(0.9), restsOn("work-a", kb.Fraction{Set: true, Value: 0.3})),
			claimEntry("clm-workpd", f(0.9), restsOn("work-a", kb.Fraction{Set: true, Pending: true})),
			claimEntry("clm-workns", f(0.9), restsOn("work-b", kb.Fraction{Set: true, Value: 1})),
		},
		Supports: []kb.SupportNode{
			{ID: "sup-lift01", Quality: f(0.9), Supports: []kb.SupportPair{{ClaimID: "clm-lift01", Fraction: kb.Fraction{Set: true, Value: 0.98}}}},
		},
		Experiments: []kb.ExperimentNode{
			{ID: "exp-run001", Status: "run", Strengthens: []kb.StrengthensPair{{ClaimID: "clm-rescu1", Strength: 0.7}, {ClaimID: "clm-weak01", Strength: 0.6}}},
			{ID: "exp-pend01", Status: "pending", Strengthens: []kb.StrengthensPair{{ClaimID: "clm-pend01", Strength: 1}}},
		},
		Works: []kb.ExternalWork{{ID: "work-a", Strength: f(0.5)}, {ID: "work-b"}},
	}
	sol, err := ComputeSolidity(st)
	if err != nil {
		t.Fatal(err)
	}
	finals := sol.Finals()
	for _, tc := range []struct {
		id   string
		want *float64
	}{
		{"clm-base01", f(0.9)},
		{"clm-weak01", f(0.6)},
		{"clm-chain1", f(0.6)},
		{"clm-pend01", nil},
		{"clm-poisn1", nil},
		{"clm-lift01", f(0.88)},
		{"clm-rescu1", f(0.7)},
		{"clm-work00", f(0.9)},
		{"clm-work01", f(0.5)},
		{"clm-workpd", nil},
		{"clm-workns", nil},
	} {
		got, ok := finals[tc.id]
		if tc.want == nil && ok || tc.want != nil && (!ok || got != *tc.want) {
			t.Errorf("%s: final %v (set %v), want %v", tc.id, got, ok, tc.want)
		}
	}
	if got := RenderSolidityTrace(sol.Results["clm-weak01"]); got != " [= max(0.40, 0.60)]" {
		t.Errorf("experimentally rescued trace = %q", got)
	}
	if got := RenderSolidityTrace(sol.Results["clm-rescu1"]); got != " [= experimental 0.70]" {
		t.Errorf("rescue-only trace = %q", got)
	}
	if got := RenderSolidityTrace(sol.Results["clm-chain1"]); got != " [= min(0.80, 0.60)]" {
		t.Errorf("weakest-link trace = %q", got)
	}
	if got := RenderSolidityTrace(sol.Results["clm-lift01"]); got != "" {
		t.Errorf("dependency-free trace = %q", got)
	}
}

func TestComputeSolidityRefusesACycleAmongUnscoredClaims(t *testing.T) {
	st := kb.State{ClaimEntries: []kb.ClaimEntry{
		claimEntry("clm-aaaaaa", nil, dep("clm-bbbbbb")),
		claimEntry("clm-bbbbbb", nil, dep("clm-aaaaaa")),
		claimEntry("clm-cccccc", f(0.5)),
	}}
	_, err := ComputeSolidity(st)
	var cycle CycleError
	if !errors.As(err, &cycle) || !slices.Equal(cycle.Members, []string{"clm-aaaaaa", "clm-bbbbbb"}) {
		t.Errorf("err = %v", err)
	}
}

func TestSerialize(t *testing.T) {
	recs := []record{{{"s", "a\"b\\c\n\t\x01é"}, {"n", nil}, {"f", 0.5}, {"i", 3}, {"b", true}, {"l", []string{"x", "y"}}, {"e", []string{}}}}
	want := `{"s": "a\"b\\c\n\t\u0001é", "n": null, "f": 0.5, "i": 3, "b": true, "l": ["x", "y"], "e": []}` + "\n"
	if got := Serialize(recs); got != want {
		t.Errorf("Serialize =\n%s\nwant\n%s", got, want)
	}
	if got := Serialize(nil); got != "" {
		t.Errorf("Serialize(nil) = %q", got)
	}
}

func TestRenderLeafReferences(t *testing.T) {
	for _, tc := range []struct {
		register string
		leaves   []string
		want     string
	}{
		{"claim-quality.md", []string{"common/a.md", "b.md"}, "> **Leaf references:** [a](./common/a.md), [b](./b.md)."},
		{"vol/claim-quality.md", []string{"vol/x/a.md", "common/b.md"}, "> **Leaf references:** [a](./x/a.md), [b](../common/b.md)."},
		{"vol/claim-quality.md", nil, kb.LeafReferencesPendingFooter},
	} {
		if got := RenderLeafReferences(tc.register, tc.leaves); got != tc.want {
			t.Errorf("RenderLeafReferences(%q, %q) = %q, want %q", tc.register, tc.leaves, got, tc.want)
		}
	}
}

// writeKB writes files under a fresh kb-root and returns it.
func writeKB(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const fmKind = "<!-- kb-frontmatter\nkind: %s\n-->\n\n"

func withKind(kind, body string) string { return strings.Replace(fmKind, "%s", kind, 1) + body }

// The cases are kb_tools' own citation-gate tests (tests/test_verify_citations.py).
func TestCitationFindings(t *testing.T) {
	target := "# Target\n\n## The Section\n\nThe load-bearing clause lives here, stated plainly.\n"
	register := "## Entry\n<!-- id: clm-aa1111 -->\n\nRationale prose naming clm-bb2222 from the other domain.\n"
	nodes := `{"id": "clm-aa1111", "canonical_path": "alpha/one.md"}` + "\n" + `{"id": "clm-bb2222", "canonical_path": "beta/two.md"}` + "\n"
	long := strings.Repeat("x", excerptMaxChars+1)
	bound := strings.Repeat("y", excerptMaxChars)
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"plain invariant numbering", map[string]string{"index.md": withKind("index", "This follows per design-doc Invariant 3.\n")}, []string{checkChannel}},
		{"invariant id", map[string]string{"index.md": withKind("index", "See INVARIANT-S2 for the numbering.\n")}, []string{checkChannel}},
		{"bare claim id", map[string]string{"index.md": withKind("index", "The claim clm-aa1111 supports this.\n")}, []string{checkChannel}},
		{"clean prose", map[string]string{"index.md": withKind("index", "Nothing citation-shaped here at all.\n")}, nil},
		{"fenced example", map[string]string{"index.md": withKind("index", "How:\n\n```\nSee INVARIANT-S2 and clm-aa1111, per Invariant 11.\n```\n")}, nil},
		{"sanctioned channels", map[string]string{
			"claim-quality.md": "## A Claim\n<!-- id: clm-aa1111 -->\n\n- confidence: 0.75\n- depends-on:\n  - INVARIANT-S2 (axiom scaffold)\n  - clm-bb2222 (a real dependency)\n- solidity: 0.75 (ok to build on, see caveats)\n",
			"leaf.md":          withKind("leaf", "<!-- claim-quality: clm-aa1111 -->\n\nProse.\n"),
		}, nil},
		{"leaf body exempt", map[string]string{
			"leaf.md": withKind("leaf", "The source says INVARIANT-S2 and clm-aa1111, per Invariant 4.\n"),
		}, nil},
		{"declaration sections may name siblings", map[string]string{
			"invariants.md": "# Invariants\n\n### INVARIANT-S1: First rule\n\nHolds jointly with INVARIANT-S2, which numbers the axioms.\n\n### INVARIANT-S2: Axiom numbering\n\n- Axiom 1: **One** — a bullet.\n",
		}, nil},
		{"framework prose outside a declaration", map[string]string{
			"invariants.md": "# Invariants\n\n## Preamble\n\nThis corpus relies on INVARIANT-S2 throughout.\n\n### INVARIANT-S2: Axiom numbering\n\nBody.\n",
		}, []string{checkChannel}},
		{"well-formed authority citation", map[string]string{
			"index.md": withKind("index", `As established in ["The load-bearing clause lives here"](target.md#the-section).`+"\n"), "target.md": target,
		}, nil},
		{"unresolvable target", map[string]string{"index.md": withKind("index", `See ["the clause"](nowhere.md#x).`+"\n")}, []string{checkReferent}},
		{"missing anchor", map[string]string{
			"index.md": withKind("index", `See ["The load-bearing clause lives here"](target.md#no-such).`+"\n"), "target.md": target,
		}, []string{checkReferent}},
		{"absent excerpt", map[string]string{
			"index.md": withKind("index", `See ["a clause never written there"](target.md#the-section).`+"\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"whitespace-normalized excerpt", map[string]string{
			"index.md": withKind("index", `See ["The load-bearing   clause    lives here"](target.md#the-section).`+"\n"), "target.md": target,
		}, nil},
		{"excerpt without anchor", map[string]string{
			"index.md": withKind("index", `See ["The load-bearing clause lives here"](target.md).`+"\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"bloated excerpt", map[string]string{
			"index.md": withKind("index", `See ["`+long+`"](target.md#the-section).`+"\n"), "target.md": "# T\n\n## The Section\n\n" + long + "\n",
		}, []string{checkExcerpt}},
		{"excerpt at the bound", map[string]string{
			"index.md": withKind("index", `See ["`+bound+`"](target.md#the-section).`+"\n"), "target.md": "# T\n\n## The Section\n\n" + bound + "\n",
		}, nil},
		{"blank excerpt", map[string]string{
			"index.md": withKind("index", `See [" "](target.md#the-section) for the rule.`+"\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"excerpt only inside a fence at the target", map[string]string{
			"index.md":  withKind("index", `Authority: ["the moon is made of cheese"](target.md#the-section).`+"\n"),
			"target.md": "# Target\n\n## The Section\n\nThe real clause.\n\n```\nthe moon is made of cheese\n```\n",
		}, []string{checkExcerpt}},
		{"anchor only as a fenced heading", map[string]string{
			"index.md":  withKind("index", `Authority: ["invented authority text"](target.md#fake-section).`+"\n"),
			"target.md": "# Target\n\n```markdown\n## Fake Section\n\ninvented authority text\n```\n",
		}, []string{checkReferent}},
		{"typographic quotes", map[string]string{
			"index.md": withKind("index", "Authority: [“a clause that is not in the target”](target.md#the-section).\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"bracket inside the link text", map[string]string{
			"index.md": withKind("index", `Authority: ["the rule [see note] applies"](does-not-exist.md#nope).`+"\n"),
		}, []string{checkReferent}},
		{"non-durable target", map[string]string{
			"domain/sub/index.md": withKind("index", `See ["the clause"](../../../elsewhere/notes.md#x).`+"\n"),
		}, []string{checkDurable}},
		{"external link", map[string]string{"index.md": withKind("index", "See [the spec](https://example.invalid/spec).\n")}, nil},
		{"foreign-domain reference without an edge", map[string]string{
			"alpha/claim-quality.md": register, ".index/claims.jsonl": nodes, ".index/depends-on.jsonl": "",
		}, []string{checkChannel, checkEdge}},
		{"foreign-domain reference with an edge", map[string]string{
			"alpha/claim-quality.md": register, ".index/claims.jsonl": nodes,
			".index/depends-on.jsonl": `{"source": "clm-aa1111", "target": "clm-bb2222"}` + "\n",
		}, []string{checkChannel}},
		{"no-edge exemption", map[string]string{
			"alpha/claim-quality.md":  "## Entry\n<!-- id: clm-aa1111 -->\n\n- no-edge: cited as contrast\n\nRationale prose naming clm-bb2222 from the other domain.\n",
			".index/claims.jsonl":     nodes,
			".index/depends-on.jsonl": "",
		}, []string{checkChannel}},
		{".index out of scope", map[string]string{".index/SCHEMA.md": "# Schema\n\n1. Invariant 1: the index is derived.\n"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := CitationFindings(writeKB(t, tc.files))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, fd := range findings {
				got = append(got, fd.Check)
			}
			slices.Sort(got)
			got = slices.Compact(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("checks = %q, want %q; findings %v", got, tc.want, findings)
			}
		})
	}
}
