package claimgraph

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/log"
)

// discoveryParagraph is the one paragraph of each discovery leaf: every leaf
// asks it, and a yes mints it under the same title in every leaf.
const discoveryParagraph = "We show that every widget is a gadget for all inputs.\n"

// seedDiscoveryLeaves writes a KB whose volume v holds one leaf per name of
// leaves, each holding discoveryParagraph, an empty docgraph record, empty
// classification and unmarked records, and a node-pass record listing unread
// those of leaves named in unread. It returns the kb-root, the repository root
// and the stage's options with no reader.
func seedDiscoveryLeaves(t *testing.T, leaves []string, unread ...string) (string, string, Options) {
	t.Helper()
	return seedLeavesOf(t, discoveryParagraph, leaves, unread...)
}

// seedLeavesOf is seedDiscoveryLeaves with body in place of
// discoveryParagraph.
func seedLeavesOf(t *testing.T, body string, leaves []string, unread ...string) (string, string, Options) {
	t.Helper()
	index := "[↑ Entry](../entry-point.md)\n\n# V\n\n"
	docs := map[string]string{}
	for _, name := range leaves {
		index += "- [" + name + "](" + name + ".md)\n"
		docs["v/"+name+".md"] = "[↑ V](index.md)\n\n# " + name + "\n\n" + body
	}
	docs["v/index.md"] = index
	root := seedKB(t, "# Entry\n\n- [V](v/index.md)\n", docs, kb.Graph{})
	repo := filepath.Dir(root)
	state := t.TempDir()
	recordsPath := filepath.Join(state, "records", "docgraph.jsonl")
	if err := os.MkdirAll(filepath.Dir(recordsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordsPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pass := buildrecords.NodePassRecord{Leaves: map[string]buildrecords.LeafEntry{}}
	for _, name := range unread {
		pass.Leaves["v/"+name+".md"] = buildrecords.LeafEntry{State: ReadUnread}
	}
	if err := WriteNodePass(repo, pass); err != nil {
		t.Fatal(err)
	}
	if err := WriteClassification(repo, buildrecords.ClassificationRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{}); err != nil {
		t.Fatal(err)
	}
	return root, repo, Options{KBRoot: root, RepoRoot: repo, Records: recordsPath, Logger: log.Discard(), AskRecords: filepath.Join(state, "asks")}
}

// commonFiles is the bytes of every build record under repo and of both
// record files under root's .index/, by path; nil where one does not stand.
func commonFiles(t *testing.T, root, repo string) map[string][]byte {
	t.Helper()
	paths := []string{filepath.Join(root, kb.GraphNodesFile), filepath.Join(root, kb.GraphEdgesFile)}
	for _, f := range buildrecords.RecordFiles {
		paths = append(paths, filepath.Join(repo, f))
	}
	out := map[string][]byte{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		out[p] = b
	}
	return out
}

// TestAStageWritesNoCommonFileWhileItAsks: at every ask of each inference
// stage, every build record and both kb-root/.index/ record files hold the
// bytes they held before the stage, and its group records are not yet
// written; after the stage, they are.
func TestAStageWritesNoCommonFileWhileItAsks(t *testing.T) {
	answer := func(_ context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
		return asks.Reply{Text: q.Offered[0]}, nil
	}
	for _, tc := range []struct {
		name  string
		seed  func(t *testing.T) (string, string, Options)
		stage func(context.Context, Options) (Report, error)
	}{
		{"claims-discovered", func(t *testing.T) (string, string, Options) {
			return seedDiscoveryLeaves(t, []string{"a", "b"}, "a", "b")
		}, Discovered},
		{"references-found", func(t *testing.T) (string, string, Options) {
			root, repo, opts := seedCullStage(t, nil)
			if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{}); err != nil {
				t.Fatal(err)
			}
			return root, repo, opts
		}, References},
		{"claims-culled", func(t *testing.T) (string, string, Options) { return seedCullStage(t, nil) }, Culled},
		{"depends-attributed", func(t *testing.T) (string, string, Options) {
			root, repo, opts := seedCullStage(t, nil)
			if err := WriteCull(repo, buildrecords.CullRecord{Links: []buildrecords.CullLink{{A: cullA, B: cullD, Harvest: harvestUnmarked}}}); err != nil {
				t.Fatal(err)
			}
			return root, repo, opts
		}, Depends},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, repo, opts := tc.seed(t)
			before := commonFiles(t, root, repo)
			var mu sync.Mutex
			asked := 0
			opts.Launch = readerLaunch(func(ctx context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
				mu.Lock()
				asked++
				mu.Unlock()
				for p, b := range commonFiles(t, root, repo) {
					if !bytes.Equal(b, before[p]) {
						t.Errorf("%s changed while %s->%s was asked", p, q.Group, q.Item)
					}
				}
				if entries, err := os.ReadDir(opts.AskRecords); err == nil && len(entries) > 0 {
					t.Errorf("%s holds %d file(s) while %s->%s was asked", opts.AskRecords, len(entries), q.Group, q.Item)
				}
				return answer(ctx, q)
			})
			report, err := tc.stage(context.Background(), opts)
			requireStagePassed(t, report, err)
			if asked == 0 {
				t.Fatal("the stage asked nothing, so nothing was checked")
			}
			if entries, err := os.ReadDir(opts.AskRecords); err != nil || len(entries) != 1 {
				t.Errorf("after the stage %s holds %v, %v; want its one group-records file", opts.AskRecords, entries, err)
			}
		})
	}
}

// TestDiscoveredLandsAsTheLeavesOneByOne: two leaves whose claims take the
// same title, landed in one insert batch, leave the kb-root/ and the
// node-pass record that landing them one leaf at a time leaves, modulo ids.
func TestDiscoveredLandsAsTheLeavesOneByOne(t *testing.T) {
	claim := func(_ context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
		return asks.Reply{Text: asks.LetterClaim}, nil
	}
	leaves := []string{"a", "b"}

	_, batch, opts := seedDiscoveryLeaves(t, leaves, leaves...)
	opts.Launch = readerLaunch(claim)
	report, err := Discovered(context.Background(), opts)
	requireStagePassed(t, report, err)

	_, sequence, opts := seedDiscoveryLeaves(t, leaves, leaves[0])
	opts.Launch = readerLaunch(claim)
	report, err = Discovered(context.Background(), opts)
	requireStagePassed(t, report, err)
	rec, err := requireNodePass(sequence)
	if err != nil {
		t.Fatal(err)
	}
	rec.Leaves["v/"+leaves[1]+".md"] = buildrecords.LeafEntry{State: ReadUnread}
	if err := WriteNodePass(sequence, rec); err != nil {
		t.Fatal(err)
	}
	report, err = Discovered(context.Background(), opts)
	requireStagePassed(t, report, err)

	checks, err := CompareModuloIDs(BuiltBy("one batch", batch), BuiltBy("leaf by leaf", sequence))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range checks {
		for _, d := range c.Differences {
			t.Errorf("%s: %s", c.Name, d)
		}
	}
	g := loadRecords(t, filepath.Join(batch, kb.KBDir))
	if len(g.Claims) != 2 || g.Claims[0].Title != g.Claims[1].Title {
		t.Errorf("the batch minted %+v, want one claim per leaf under one title", g.Claims)
	}
}
