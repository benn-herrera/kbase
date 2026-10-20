package claimgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/write"
)

// seedRingKB writes a minimal KB hosting three claims in one register whose
// depends edges close the cycle a → b → c → a, every claim hosted by one leaf.
func seedRingKB(t *testing.T) (string, map[string]string) {
	const (
		aID = "clm-raaaaa"
		bID = "clm-rbbbbb"
		cID = "clm-rccccc"
	)
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	// The spine check gates on kb-root/.index/ existing; the build's spine
	// seed creates it, so the fixture seeds it too.
	if err := os.MkdirAll(filepath.Join(root, kb.IndexDir), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, text := range map[string]string{
		kb.EntryPointFile: "---\nkb-format: \"2.0.0\"\n---\n# Entry\n\n- [V](v/index.md)\n",
		"v/index.md":      "---\nkind: index\n---\n[↑ Entry](../entry-point.md)\n\n# V\n\n- [Leaf](leaf.md)\n",
		"v/leaf.md":       "---\nkind: leaf\nclaims: [" + aID + ", " + bID + ", " + cID + "]\n---\n[↑ V](index.md)\n\n# Leaf\n\nA holds here. <!-- claim-rigor: " + aID + " -->\n\nB holds here. <!-- claim-rigor: " + bID + " -->\n\nC holds here. <!-- claim-rigor: " + cID + " -->\n",
		kb.RegisterFile:   "# Register\n\n" + registerQuality(aID, "Result A") + registerQuality(bID, "Result B") + registerQuality(cID, "Result C"),
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := writer{kbRoot: root, lg: log.Discard()}
	wires := values{
		{"id": aID, kb.ListRestsOn: []map[string]any{{"id": bID, "bearing": kb.PendingLiteral}}},
		{"id": bID, kb.ListRestsOn: []map[string]any{{"id": cID, "bearing": kb.PendingLiteral}}},
		{"id": cID, kb.ListRestsOn: []map[string]any{{"id": aID, "bearing": kb.PendingLiteral}}},
	}
	if _, err := w.run("seed", write.AddBuildEdges, wires, false); err != nil {
		t.Fatal(err)
	}
	return root, map[string]string{"a": aID, "b": bID, "c": cID}
}

// TestRingRowDemotesSurvivingCycles: the ring row, with its origins read from
// the build records as Depends reads them, re-reads the captured depends edges
// and issues demote-claim-edge for every edge lying on a cycle. Over the
// planted 3-cycle a → b → c → a every premise edge leaves the rests-on list and
// lands as a demoted row in its own entry, and the gate over the tree as the
// row left it holds. The unmarked record answers a → b yes, and c → b yes, which is b → c
// only in reverse. Without a cull record only a → b is inferred; with one
// holding the links a–c and b–c, each id-ordered, b → c is inferred as
// recorded and c → a reversed.
func TestRingRowDemotesSurvivingCycles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		links  func(ids map[string]string) []buildrecords.CullLink
		bc, ca string
	}{
		{"no cull record: a yes only in reverse is cited", nil, kb.OriginCited, kb.OriginCited},
		{"the cull record's links are inferred either way round", func(ids map[string]string) []buildrecords.CullLink {
			return []buildrecords.CullLink{{A: ids["a"], B: ids["c"], Harvest: harvestAdjacent}, {A: ids["b"], B: ids["c"], Harvest: harvestAdjacent}}
		}, kb.OriginInferred, kb.OriginInferred},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, ids := seedRingKB(t)
			repo := filepath.Dir(root)
			yes := asks.LetterPoints
			if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{Pairs: []buildrecords.CandidateEntry{
				{Source: ids["a"], Target: ids["b"], Offered: []string{asks.LetterPoints, asks.LetterDoesNotPoint}, Letter: &yes, Outcome: ClassifyAnswered},
				{Source: ids["c"], Target: ids["b"], Offered: []string{asks.LetterPoints, asks.LetterDoesNotPoint}, Letter: &yes, Outcome: ClassifyAnswered},
			}}); err != nil {
				t.Fatal(err)
			}
			if tc.links != nil {
				if err := WriteCull(repo, buildrecords.CullRecord{Links: tc.links(ids)}); err != nil {
					t.Fatal(err)
				}
			}

			inferred, err := inferredAt(repo)
			if err != nil {
				t.Fatal(err)
			}
			cut, err := ringRow(writer{kbRoot: root, repoRoot: repo, lg: log.Discard()}, inferred)
			if err != nil {
				t.Fatalf("ringRow: %v", err)
			}
			if len(cut) != 3 {
				t.Errorf("cut = %v, want the cycle's 3 edges", cut)
			}
			for _, f := range gate(root, log.Discard()) {
				if f.Status == StatusFail {
					t.Errorf("the gate failed %s: %+v", f.Check, f.Fields)
				}
			}

			got, err := capturedEdges(root, log.Discard())
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 {
				t.Errorf("depends edges after the ring row = %v, want the cycle gone", got)
			}
			text, err := kb.OnDisk(root).ReadText(filepath.Join(root, kb.RegisterFile))
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []struct{ source, target, title, origin string }{
				{ids["a"], ids["b"], "Result B", kb.OriginInferred},
				{ids["b"], ids["c"], "Result C", tc.bc},
				{ids["c"], ids["a"], "Result A", tc.ca},
			} {
				demoted := "- " + p.target + " — " + p.title + " (origin " + p.origin + ")"
				if !strings.Contains(text, demoted) {
					t.Errorf("no demoted row %q stands:\n%s", demoted, text)
				}
				if strings.Contains(text, "- "+p.target+" — "+p.title+" (strength") {
					t.Errorf("%s's rests-on bullet naming %s survived:\n%s", p.source, p.target, text)
				}
			}
		})
	}
}
