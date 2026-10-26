package claimgraph

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/records"
	"kbase/internal/result"
)

const (
	cullA    = "clm-aaaaaa"
	cullB    = "clm-bbbbbb"
	cullD    = "clm-dddddd"
	cullLeaf = "v/leaf.md"
)

// seedCullStage writes a KB whose one leaf holds prose claims A then B and
// block claim D, in that reading order, beside a docgraph record of D's block,
// the node pass landed and an unmarked record answering B → D yes. It returns
// the kb-root, the repository root and the stage's options with reader.
func seedCullStage(t *testing.T, reader asks.LetterReader) (string, string, Options) {
	t.Helper()
	m := func(id string) string { return " <!-- claim-rigor: " + id + " -->" }
	root, repo, opts := seedTheoremLeaf(t, map[string]string{cullA: "Result A", cullB: "Result B", cullD: "Theorem 1"},
		"Alpha holds here."+m(cullA)+"\n\nAlpha holds here, said again."+m(cullB)+"\n\n"+
			"> **Theorem**\n>\n> **Theorem 1.** Delta holds here."+m(cullD)+"\n")
	yes := asks.LetterPoints
	if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{Pairs: []buildrecords.CandidateEntry{
		{Source: cullB, Target: cullD, Offered: []string{asks.LetterPoints, asks.LetterDoesNotPoint}, Letter: &yes, Outcome: string(asks.Answered)},
	}}); err != nil {
		t.Fatal(err)
	}
	opts.Reader = reader
	return root, repo, opts
}

// seedTheoremLeaf writes a KB whose one leaf, cullLeaf, holds body, its one
// block a Theorem, beside a claim record per id of titles, a docgraph record
// of that block — and of each further record extra names — and the node pass
// landed. It returns the kb-root, the repository root and the stage's options
// with no reader.
func seedTheoremLeaf(t *testing.T, titles map[string]string, body string, extra ...records.Record) (string, string, Options) {
	t.Helper()
	root := seedKB(t, "# Entry\n\n- [V](v/index.md)\n", map[string]string{
		"v/index.md": "[↑ Entry](../entry-point.md)\n\n# V\n\n- [Leaf](leaf.md)\n",
		cullLeaf:     "[↑ V](index.md)\n\n# Leaf\n\n" + body,
	}, kb.Graph{Claims: claimRecords(titles)})
	repo := filepath.Dir(root)
	state := t.TempDir()
	recordsPath := filepath.Join(state, "records", "docgraph.jsonl")
	if err := os.MkdirAll(filepath.Dir(recordsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []byte
	for _, r := range append([]records.Record{{Kind: records.KindBlock, Order: 1, Document: cullLeaf, Name: "Theorem"}}, extra...) {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(append(lines, line...), '\n')
	}
	if err := os.WriteFile(recordsPath, lines, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteNodePass(repo, buildrecords.NodePassRecord{Leaves: map[string]buildrecords.LeafEntry{cullLeaf: {State: ReadLanded}}}); err != nil {
		t.Fatal(err)
	}
	return root, repo, Options{KBRoot: root, RepoRoot: repo, Records: recordsPath, Logger: log.Discard(), AskRecords: filepath.Join(state, "asks")}
}

// answeringItem answers the ask named item with letter and every other ask
// distinct.
func answeringItem(item, letter string) asks.LetterReader {
	return func(_ context.Context, q asks.LetterQuestion) (asks.Reply, error) {
		if q.Item == item {
			return asks.Reply{Text: letter}, nil
		}
		return asks.Reply{Text: asks.LetterDistinct}, nil
	}
}

// requireStagePassed fails the test on a stage error or a failed finding,
// the gate's included.
func requireStagePassed(t *testing.T, report Report, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if report.Failed() {
		t.Fatalf("the stage reported failures: %+v", report.Failures())
	}
}

func readCullRecord(t *testing.T, repo string) buildrecords.CullRecord {
	t.Helper()
	rec, ok, err := buildrecords.ReadCull(recordsAt(repo))
	if err != nil || !ok {
		t.Fatalf("the cull record read back as present %t, %v", ok, err)
	}
	return rec
}

// TestCulledCullsAndRepoints: B, shown second, adds nothing to A, so B is
// culled into A — its record and its marker gone — and its unmarked link to D
// is re-pointed onto A in the record, with no edge written.
func TestCulledCullsAndRepoints(t *testing.T) {
	root, repo, opts := seedCullStage(t, answeringItem(cullB, asks.LetterSecondAddsNothing))
	report, err := Culled(context.Background(), opts)
	requireStagePassed(t, report, err)

	for _, rel := range []string{kb.GraphNodesFile, cullLeaf} {
		text, err := kb.OnDisk(root).ReadText(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, cullB) {
			t.Errorf("%s still names B:\n%s", rel, text)
		}
	}
	if g := loadRecords(t, root); len(g.Scorable)+len(g.References) != 0 {
		t.Errorf("the cull wrote edges %+v %+v, want none", g.Scorable, g.References)
	}

	rec := readCullRecord(t, repo)
	wantEntries := []buildrecords.CullEntry{
		{A: cullA, B: cullB, Pass: 1, Verdict: buildrecords.VerdictBAddsNothing, Outcome: string(asks.Answered)},
		{A: cullB, B: cullD, Pass: 1, Verdict: buildrecords.VerdictDistinct, Outcome: string(asks.Answered)},
	}
	if !slices.Equal(rec.Entries, wantEntries) {
		t.Errorf("entries = %+v, want %+v", rec.Entries, wantEntries)
	}
	if want := []buildrecords.Cull{{ID: cullB, Into: cullA}}; !slices.Equal(rec.Culls, want) {
		t.Errorf("culls = %+v, want %+v", rec.Culls, want)
	}
	if want := []buildrecords.CullLink{{A: cullA, B: cullD, Harvest: harvestCulled}}; !slices.Equal(rec.Links, want) {
		t.Errorf("links = %+v, want B's link to D re-pointed onto A: %+v", rec.Links, want)
	}
}

// TestCulledDefaultedAskKeeps: an ask answered unreadably on every attempt is
// recorded distinct and defaulted, keeps both claims, and is one fallback.
func TestCulledDefaultedAskKeeps(t *testing.T) {
	root, repo, opts := seedCullStage(t, answeringItem(cullB, "?"))
	var fallbacks []result.Item
	opts.Progress.Fallback = func(it result.Item) error { fallbacks = append(fallbacks, it); return nil }
	report, err := Culled(context.Background(), opts)
	requireStagePassed(t, report, err)

	if g := loadRecords(t, root); !slices.ContainsFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == cullB }) {
		t.Errorf("B's record did not stand: %+v", g.Claims)
	}
	rec := readCullRecord(t, repo)
	want := buildrecords.CullEntry{A: cullA, B: cullB, Pass: 1, Verdict: buildrecords.VerdictDistinct, Outcome: string(asks.Defaulted)}
	if !slices.Contains(rec.Entries, want) || len(rec.Culls) != 0 {
		t.Errorf("record = %+v, want %+v among its entries and no cull", rec, want)
	}
	if len(fallbacks) != 1 {
		t.Errorf("fallbacks = %+v, want exactly one", fallbacks)
	}
}
