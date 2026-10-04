package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

func claim(id, title string, solidity *float64, band string) map[string]any {
	return map[string]any{"node_type": "claim", "id": id, "title": title, "canonical_path": "main/claim-quality.md",
		"canonical_anchor": strings.ToLower(strings.ReplaceAll(title, " ", "-")), "confidence": solidity, "solidity": solidity,
		"build_status": nil, "build_band": band, "rationale": "", "depends_on_count": 0, "strengthen_by_count": 0, "citation_count": 1}
}

func edge(source, target, relation string, fraction any) map[string]any {
	return map[string]any{"source": source, "target": target, "relation": relation, "target_kind": "claim",
		"target_solidity_recorded": nil, "strength": nil, "context": nil, "fraction": fraction}
}

func cite(id, leaf string) map[string]any {
	return map[string]any{"claim_id": id, "leaf_path": leaf, "leaf_kind": "leaf", "tier2_marked": false}
}

func f(v float64) *float64 { return &v }

// writeIndex writes kbRoot/.index with the given records per file; files not
// named are written empty.
func writeIndex(t *testing.T, kbRoot string, files map[string][]map[string]any) {
	t.Helper()
	dir := filepath.Join(kbRoot, kb.IndexDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range requiredFiles {
		var b strings.Builder
		for _, rec := range files[name] {
			line, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			b.Write(line)
			b.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, name+".jsonl"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func load(t *testing.T, files map[string][]map[string]any) *Index {
	t.Helper()
	root := t.TempDir()
	writeIndex(t, root, files)
	ix, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func ids(nodes []Node) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

func TestFind(t *testing.T) {
	ix := load(t, map[string][]map[string]any{"claims": {
		claim("clm-cccccc", "Proposition 4.4 — KPI-Gating as Discrete Pontryagin", f(0.55), "input-only"),
		claim("clm-aaaaaa", "Proposition 4.3 — Institutional Misalignment", f(0.55), "input-only"),
		claim("clm-bbbbbb", "Theorem 4.5 — Positive Invariance of M", f(0.9), "ok-to-build"),
		claim("clm-dddddd", "Lemma 1 — Straße", nil, "unknown"),
	}})
	for _, tc := range []struct {
		name, query string
		want        []string
	}{
		{"by number", "4.3", []string{"clm-aaaaaa"}},
		{"case-insensitive", "pontryagin", []string{"clm-cccccc"}},
		{"shared substring, id order", "Proposition", []string{"clm-aaaaaa", "clm-cccccc"}},
		{"full case folding", "STRASSE", []string{"clm-dddddd"}},
		{"empty matches all", "", []string{"clm-aaaaaa", "clm-bbbbbb", "clm-cccccc", "clm-dddddd"}},
		{"no match", "no-such-claim", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(ix.Find(tc.query)); !slices.Equal(got, tc.want) {
				t.Errorf("Find(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func leaf(front, body string) string {
	return "[↑ Up](../index.md)\n\n<!-- kb-frontmatter\nkind: leaf\n" + front + "\n-->\n\n# Leaf\n\n" + body + "\n"
}

func TestReferencedBy(t *testing.T) {
	root := t.TempDir()
	writeIndex(t, root, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "Origin Claim", f(0.5), "input-only")},
		"cites":  {cite("clm-aaaaaa", "main/origin.md")},
	})
	writeFile(t, filepath.Join(root, "main/origin.md"), leaf("claims: [clm-aaaaaa]", "Self link [here](origin.md)."))
	writeFile(t, filepath.Join(root, "main/citing.md"), leaf(`no-claim: "links"`, "See [the origin](origin.md#sec) for it."))
	writeFile(t, filepath.Join(root, "other/deep.md"), leaf(`no-claim: "links"`, "Back [up](../main/origin.md)."))
	writeFile(t, filepath.Join(root, "main/coded.md"), leaf(`no-claim: "code"`, "Only `[x](origin.md)` in code."))
	writeFile(t, filepath.Join(root, "main/index.md"), "<!-- kb-frontmatter\nkind: index\n-->\n\n# Index\n\n[origin](origin.md)\n")
	ix, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ix.ReferencedBy("clm-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"main/citing.md", "other/deep.md"}; !slices.Equal(got, want) {
		t.Errorf("ReferencedBy = %v, want %v", got, want)
	}
	if got, _ := ix.ReferencedBy("clm-zzzzzz"); len(got) != 0 {
		t.Errorf("unknown claim: %v, want none", got)
	}
}

func TestSolidityBelowAndWeakPoints(t *testing.T) {
	ix := load(t, map[string][]map[string]any{
		"claims": {
			claim("clm-aaaaaa", "A", f(0.5), "input-only"),
			claim("clm-bbbbbb", "B", f(0.3), "do-not-build"),
			claim("clm-cccccc", "C", f(0.5), "input-only"),
			claim("clm-dddddd", "D", nil, "unknown"),
			claim("clm-eeeeee", "E", f(0.9), "ok-to-build"),
		},
		"depends-on": {
			edge("clm-dddddd", "clm-aaaaaa", "depends", nil),
			edge("clm-eeeeee", "clm-aaaaaa", "references", nil),
			edge("clm-eeeeee", "clm-aaaaaa", "depends", nil),
			edge("clm-dddddd", "clm-bbbbbb", "depends", nil),
			edge("clm-dddddd", "clm-cccccc", "depends", nil),
		},
	})
	if got, want := ids(ix.SolidityBelow(0.65)), []string{"clm-bbbbbb", "clm-aaaaaa", "clm-cccccc"}; !slices.Equal(got, want) {
		t.Errorf("SolidityBelow(0.65) = %v, want %v", got, want)
	}
	var got []string
	for _, wp := range ix.WeakPoints(DefaultMaxSolidity, DefaultMinDependents) {
		got = append(got, wp.Claim.ID)
	}
	if want := []string{"clm-aaaaaa", "clm-bbbbbb", "clm-cccccc"}; !slices.Equal(got, want) {
		t.Errorf("WeakPoints = %v, want %v (most distinct dependents, then lowest solidity, then id)", got, want)
	}
	if wp := ix.WeakPoints(DefaultMaxSolidity, 2); len(wp) != 1 || wp[0].Dependents != 2 {
		t.Errorf("WeakPoints(min 2) = %+v, want clm-aaaaaa with 2", wp)
	}
}

func TestDepsApplicability(t *testing.T) {
	ix := load(t, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", nil, "unknown")},
		"depends-on": {
			edge("clm-aaaaaa", "work-Z", "rests-on", kb.PendingLiteral),
			edge("clm-aaaaaa", "sup-aaaaaa", "supports", 0.4),
			edge("clm-aaaaaa", "work-B", "rests-on", 0.5),
		},
	})
	var got []string
	for _, rec := range DepsPayload(ix, "clm-aaaaaa", false).([]result.Record) {
		got = append(got, fmt.Sprint(rec[1].Value, "=", rec[2].Value))
	}
	if want := []string{"sup-aaaaaa=<nil>", "work-B=0.5", "work-Z=*pending*"}; !slices.Equal(got, want) {
		t.Errorf("deps = %v, want %v", got, want)
	}
}

func TestSubtreeClaims(t *testing.T) {
	ix := load(t, map[string][]map[string]any{"subtree-aggregates": {
		{"node_path": "entry-point.md", "node_kind": "entry-point", "subtree_claims": []string{"clm-aaaaaa", "clm-bbbbbb"}},
		{"node_path": "vol/index.md", "node_kind": "index", "subtree_claims": []string{"clm-bbbbbb"}},
	}})
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"", []string{"clm-aaaaaa", "clm-bbbbbb"}},
		{".", []string{"clm-aaaaaa", "clm-bbbbbb"}},
		{"vol/index.md", []string{"clm-bbbbbb"}},
		{"vol", []string{"clm-bbbbbb"}},
		{"vol//", []string{"clm-bbbbbb"}},
		{"nowhere", []string{}},
	} {
		if got := ix.SubtreeClaims(tc.path); !slices.Equal(got, tc.want) {
			t.Errorf("SubtreeClaims(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestNodeFieldsTotalOverKinds(t *testing.T) {
	for _, kind := range kb.NodeKinds {
		if nodeFields[kind] == nil {
			t.Errorf("no field list for node kind %q", kind)
		}
	}
	if len(nodeFields) != len(kb.NodeKinds) {
		t.Errorf("nodeFields has %d kinds, kb.NodeKinds %d", len(nodeFields), len(kb.NodeKinds))
	}
}

func TestSupportBandIsDerived(t *testing.T) {
	ix := load(t, map[string][]map[string]any{"claims": {
		{"node_type": "support", "id": "sup-aaaaaa", "title": "S", "canonical_path": "p", "canonical_anchor": "s", "quality": 0.7, "solidity": 0.7},
	}})
	n, ok := ix.Node("sup-aaaaaa")
	if !ok || n.BuildBand != "ok-with-caveats" {
		t.Errorf("support node = %+v, %v; want build_band ok-with-caveats", n, ok)
	}
}

func TestStatsCensusPartitionsClaims(t *testing.T) {
	recs := []map[string]any{claim("clm-aaaaaa", "A", nil, "unknown")}
	for _, kind := range kb.NodeKinds[1:] {
		recs = append(recs, map[string]any{"node_type": kind, "id": kind + "-x", "title": "T", "canonical_path": "p",
			"canonical_anchor": "a", "status": "run"})
	}
	ix := load(t, map[string][]map[string]any{"claims": recs})
	total := 0
	for _, field := range StatsPayload(ix)[:len(kb.NodeKinds)] {
		total += field.Value.(int)
	}
	if total != len(recs) {
		t.Errorf("census sums to %d, want %d", total, len(recs))
	}
}

func TestLoadRefusals(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root); !isRefusal(err, len(requiredFiles)) {
		t.Errorf("empty kb-root: %v, want a refusal naming all %d files", err, len(requiredFiles))
	}
	writeIndex(t, root, map[string][]map[string]any{"claims": {{"node_type": "gadget", "id": "x"}}})
	writeFile(t, filepath.Join(root, kb.IndexDir, "cites.jsonl"), "[1]\n\nnot json\n")
	if _, err := Load(root); !isRefusal(err, 3) {
		t.Errorf("bad lines: %v, want a refusal naming 3", err)
	}
}

func isRefusal(err error, items int) bool {
	var r index.Refusal
	return errors.As(err, &r) && len(r.Items) == items
}
