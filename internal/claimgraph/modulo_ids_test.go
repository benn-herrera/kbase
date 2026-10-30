package claimgraph

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
)

// TestCompareModuloIDsComparesTheCullRecord: two KBs whose cull records
// differ only in their ids — the culled claim's, which neither KB holds any
// more, and the ids each minted for one paragraph's claim — agree; two whose
// culls fold the culled claim into different claims disagree on the cull
// record.
func TestCompareModuloIDsComparesTheCullRecord(t *testing.T) {
	const (
		widget   = "We show that every widget is a gadget for all inputs."
		gizmo    = "We prove that every gizmo is a doohickey for all inputs."
		sprocket = "We show that every sprocket is a cog for all inputs."
	)
	// built is a KB of the three paragraphs' claims, its cull record culling
	// the claim culled — an id no claim of the KB holds — into the claim titled
	// into, and keeping the gizmo–sprocket link.
	built := func(culled, into string) string {
		t.Helper()
		root, repo, opts := seedLeavesOf(t, widget+"\n\n"+gizmo+"\n\n"+sprocket+"\n", []string{"a"}, "a")
		opts.Launch = readerLaunch(func(context.Context, asks.MultichoiceQuestion) (asks.Reply, error) {
			return asks.Reply{Text: asks.LetterClaim}, nil
		})
		report, err := Discovered(context.Background(), opts)
		requireStagePassed(t, report, err)
		ids := map[string]string{}
		for _, c := range loadRecords(t, root).Claims {
			ids[c.Title] = c.ID
		}
		if len(ids) != 3 {
			t.Fatalf("minted %v, want one claim per paragraph", ids)
		}
		entry := buildrecords.CullEntry{A: culled, B: ids[into], Pass: 1, Verdict: buildrecords.VerdictAAddsNothing, Outcome: string(asks.Answered)}
		if entry.A > entry.B {
			entry.A, entry.B, entry.Verdict = entry.B, entry.A, buildrecords.VerdictBAddsNothing
		}
		link := buildrecords.CullLink{A: min(ids[gizmo], ids[sprocket]), B: max(ids[gizmo], ids[sprocket]), Harvest: harvestUnmarked}
		if err := WriteCull(repo, buildrecords.CullRecord{Entries: []buildrecords.CullEntry{entry},
			Culls: []buildrecords.Cull{{ID: culled, Into: ids[into]}}, Links: []buildrecords.CullLink{link}}); err != nil {
			t.Fatal(err)
		}
		return repo
	}
	// The culled ids sort below and above every minted one, so the renamed
	// pair is turned round.
	low := kb.ClaimPrefix + "-" + strings.Repeat("0", kb.HashLen)
	high := kb.ClaimPrefix + "-" + strings.Repeat("z", kb.HashLen)
	differences := func(a, b string) map[string][]string {
		t.Helper()
		checks, err := CompareModuloIDs(BuiltBy("one", a), BuiltBy("other", b))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string][]string{}
		for _, c := range checks {
			if len(c.Differences) > 0 {
				out[c.Name] = c.Differences
			}
		}
		return out
	}

	one := built(low, widget)
	if diffs := differences(one, built(high, widget)); len(diffs) != 0 {
		t.Errorf("two KBs differing only in ids differ: %v", diffs)
	}
	diffs := differences(one, built(high, gizmo))
	for _, check := range []string{"cull record, less the ids", "cull record bytes, ids renamed"} {
		if len(diffs[check]) == 0 {
			t.Errorf("two KBs culling into different claims agree on the %s: %v", check, diffs)
		}
	}
}

// TestCompareModuloIDsNamesAnalyses: an analysis is named by its kind, title
// and host, so two KBs whose analysis lifting one claim differs only in its id
// agree, and two whose analyses differ in title or in host do not.
func TestCompareModuloIDsNamesAnalyses(t *testing.T) {
	const claim = "clm-cccccc"
	// built is a KB of two leaves, a.md hosting the claim, and the analysis
	// ana titled title, hosted in host and lifting the claim, beside empty
	// build records.
	built := func(ana, title, host string) string {
		t.Helper()
		leaf := func(name, body string) string { return "[↑ V](index.md)\n\n# " + name + "\n\n" + body + "\n" }
		docs := map[string]string{
			"v/index.md": "[↑ Entry](../entry-point.md)\n\n# V\n\n- [A](a.md)\n- [B](b.md)\n",
			"v/a.md":     leaf("A", "The claim holds."+marked(claim)),
			"v/b.md":     leaf("B", "Nothing is claimed here."),
		}
		docs[host] = strings.TrimSuffix(docs[host], "\n") + "\n\nIts argument runs so." + marked(ana) + "\n"
		root := seedKB(t, "# Entry\n\n- [V](v/index.md)\n", docs, kb.Graph{
			Claims:   claimRecords(map[string]string{claim: "The claim holds"}),
			Analyses: []kb.AnalysisNode{{ID: ana, Title: title, Rationale: "Split."}},
			Scorable: []kb.ScorableEdge{{Relation: kb.RelationLifts, Source: ana, Target: claim, Bearing: kb.Bearing{Pending: true}}},
		})
		repo := filepath.Dir(root)
		if err := WriteNodePass(repo, buildrecords.NodePassRecord{}); err != nil {
			t.Fatal(err)
		}
		if err := WriteClassification(repo, buildrecords.ClassificationRecord{}); err != nil {
			t.Fatal(err)
		}
		if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{}); err != nil {
			t.Fatal(err)
		}
		return repo
	}
	differing := func(a, b string) []string {
		t.Helper()
		checks, err := CompareModuloIDs(BuiltBy("one", a), BuiltBy("other", b))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, c := range checks {
			if len(c.Differences) > 0 {
				out = append(out, c.Name)
			}
		}
		return out
	}
	one, other := kb.AnalysisPrefix+"-"+strings.Repeat("a", kb.HashLen), kb.AnalysisPrefix+"-"+strings.Repeat("b", kb.HashLen)
	base := built(one, "The claim holds", "v/a.md")
	if got := differing(base, built(other, "The claim holds", "v/a.md")); len(got) != 0 {
		t.Errorf("two KBs differing only in the analysis's id differ on %q", got)
	}
	both := []string{"claim and analysis records by (kind, title, host)", "edge records by endpoint title and host"}
	for name, kbOf := range map[string]string{
		"title": built(other, "Another title", "v/a.md"),
		"host":  built(other, "The claim holds", "v/b.md"),
	} {
		if got := differing(base, kbOf); !slices.Equal(got, both) {
			t.Errorf("two KBs whose analyses differ in %s differ on %q, want %q", name, got, both)
		}
	}
}
