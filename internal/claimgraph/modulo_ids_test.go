package claimgraph

import (
	"context"
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
