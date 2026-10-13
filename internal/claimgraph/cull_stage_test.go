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
	m := func(id string) string { return " <!-- claim-quality: " + id + " -->" }
	root, repo, opts := seedTheoremLeaf(t, []string{cullA, cullB, cullD},
		"Alpha holds here."+m(cullA)+"\n\nAlpha holds here, said again."+m(cullB)+"\n\n"+
			"> **Theorem**\n>\n> **Theorem 1.** Delta holds here."+m(cullD)+"\n",
		registerQuality(cullA, "Result A")+registerQuality(cullB, "Result B")+registerQuality(cullD, "Theorem 1"))
	yes := asks.LetterPoints
	if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{Pairs: []buildrecords.CandidateEntry{
		{Source: cullB, Target: cullD, Offered: []string{asks.LetterPoints, asks.LetterDoesNotPoint}, Letter: &yes, Outcome: ClassifyAnswered},
	}}); err != nil {
		t.Fatal(err)
	}
	opts.Reader = reader
	return root, repo, opts
}

// seedTheoremLeaf writes a KB whose one leaf, cullLeaf, hosts claims and
// holds body, its one block a Theorem, beside a register holding register, a
// docgraph record of that block and the node pass landed. It returns the
// kb-root, the repository root and the stage's options with no reader.
func seedTheoremLeaf(t *testing.T, claims []string, body, register string) (string, string, Options) {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	repo := filepath.Dir(root)
	for rel, text := range map[string]string{
		kb.EntryPointFile: "---\nkb-format: \"1.0.0\"\n---\n# Entry\n\n- [V](v/index.md)\n",
		"v/index.md":      "---\nkind: index\n---\n[↑ Entry](../entry-point.md)\n\n# V\n\n- [Leaf](leaf.md)\n",
		cullLeaf:          "---\nkind: leaf\nclaims: [" + strings.Join(claims, ", ") + "]\n---\n[↑ V](index.md)\n\n# Leaf\n\n" + body,
		kb.RegisterFile:   "# Register\n\n" + register,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, kb.IndexDir), 0o755); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	recordsPath := filepath.Join(state, "records", "docgraph.jsonl")
	if err := os.MkdirAll(filepath.Dir(recordsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	block, err := json.Marshal(records.Record{Kind: records.KindBlock, Order: 1, Document: cullLeaf, Name: "Theorem"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordsPath, append(block, '\n'), 0o644); err != nil {
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
// culled into A — its entry, marker and claims id gone — and its unmarked
// link to D is re-pointed onto A in the record, with no edge written.
func TestCulledCullsAndRepoints(t *testing.T) {
	root, repo, opts := seedCullStage(t, answeringItem(cullB, asks.LetterSecondAddsNothing))
	report, err := Culled(context.Background(), opts)
	requireStagePassed(t, report, err)

	for _, rel := range []string{kb.RegisterFile, cullLeaf} {
		text, err := kb.OnDisk(root).ReadText(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, cullB) {
			t.Errorf("%s still names B:\n%s", rel, text)
		}
	}
	edges, err := capturedEdges(root, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("the cull wrote depends edges %v, want none", edges)
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

	register, err := kb.OnDisk(root).ReadText(filepath.Join(root, kb.RegisterFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(register, "<!-- id: "+cullB+" -->") {
		t.Errorf("B's entry did not stand:\n%s", register)
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
