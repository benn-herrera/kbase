package claimgraph

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
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

// askingStage is a stage that puts multichoice asks, the kind it asks, and a
// seed it asks over.
type askingStage struct {
	name  string
	kind  asks.Kind
	seed  func(t *testing.T) (string, string, Options)
	stage func(context.Context, Options) (Report, error)
}

// askingStages is every stage that puts multichoice asks.
var askingStages = []askingStage{
	{"claims-discovered", asks.Paragraph, func(t *testing.T) (string, string, Options) {
		return seedDiscoveryLeaves(t, []string{"a", "b"}, "a", "b")
	}, Discovered},
	{"references-found", asks.Unmarked, func(t *testing.T) (string, string, Options) {
		root, repo, opts := seedCullStage(t, nil)
		if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{}); err != nil {
			t.Fatal(err)
		}
		return root, repo, opts
	}, References},
	{"claims-culled", asks.Colinear, func(t *testing.T) (string, string, Options) { return seedCullStage(t, nil) }, Culled},
	{"depends-attributed", asks.Classify, func(t *testing.T) (string, string, Options) {
		root, repo, opts := seedCullStage(t, nil)
		if err := WriteCull(repo, buildrecords.CullRecord{Links: []buildrecords.CullLink{{A: cullA, B: cullD, Harvest: harvestUnmarked}}}); err != nil {
			t.Fatal(err)
		}
		return root, repo, opts
	}, Depends},
}

// TestAStoppedStageLeavesItsGroupRecords: each stage whose batch stops on a
// call that never completes reports the stop, and leaves its group records
// with that ask recorded failed; where they cannot be written, the stage
// reports the stop and fails on the write.
func TestAStoppedStageLeavesItsGroupRecords(t *testing.T) {
	never := func(_ context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
		return asks.Reply{}, asks.IncompleteError{Subject: q.Group + "->" + q.Item, Detail: "no reply arrived"}
	}
	inferenceStop := askStop(asks.IncompleteError{}).(*stop).check
	stopped := func(t *testing.T, report Report) {
		t.Helper()
		if !slices.ContainsFunc(report.Failures(), func(it result.Item) bool { return it.Check == inferenceStop }) {
			t.Errorf("the stage reported %+v, want the inference stop", report.Failures())
		}
	}
	groupRecords := func(t *testing.T, opts Options, kind asks.Kind) []asks.GroupRecord {
		t.Helper()
		f, err := os.Open(asks.GroupRecordsPath(opts.AskRecords, kind))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		var recs []asks.GroupRecord
		for dec := yaml.NewDecoder(f); ; {
			var r asks.GroupRecord
			if err := dec.Decode(&r); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			recs = append(recs, r)
		}
		return recs
	}
	for _, tc := range askingStages {
		t.Run(tc.name, func(t *testing.T) {
			_, _, opts := tc.seed(t)
			opts.Launch = readerLaunch(never)
			report, err := tc.stage(context.Background(), opts)
			if err != nil {
				t.Fatal(err)
			}
			stopped(t, report)
			recs := groupRecords(t, opts, tc.kind)
			if len(recs) == 0 || len(recs[0].Items) == 0 || recs[0].Items[0].Outcome != asks.Failed {
				t.Errorf("the stage left %+v, want its first ask recorded failed", recs)
			}
		})
		t.Run(tc.name+" unwritable", func(t *testing.T) {
			_, _, opts := tc.seed(t)
			opts.Launch = readerLaunch(never)
			blocker := filepath.Join(t.TempDir(), "a-file")
			if err := os.WriteFile(blocker, nil, 0o666); err != nil {
				t.Fatal(err)
			}
			opts.AskRecords = filepath.Join(blocker, "asks")
			report, err := tc.stage(context.Background(), opts)
			stopped(t, report)
			if err == nil {
				t.Error("the stage's records could not be written, and it returned no error")
			}
		})
	}
	// askCull names a group for its pass: p<pass>-<claim>.
	const pass1, pass2 = "p1-", "p2-"
	// The first pass answering that the first shown adds nothing culls A into
	// B and B into D, so A's root is not its target and the second pass asks.
	t.Run("claims-culled stopped in its second pass", func(t *testing.T) {
		_, _, opts := seedCullStage(t, nil)
		opts.Launch = readerLaunch(func(ctx context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
			if strings.HasPrefix(q.Group, pass2) {
				return never(ctx, q)
			}
			return asks.Reply{Text: asks.LetterFirstAddsNothing}, nil
		})
		report, err := Culled(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		stopped(t, report)
		var first, second, failed bool
		for _, rec := range groupRecords(t, opts, asks.Colinear) {
			first = first || strings.HasPrefix(rec.Group, pass1)
			if strings.HasPrefix(rec.Group, pass2) {
				second = true
				failed = failed || slices.ContainsFunc(rec.Items, func(it asks.ItemRecord) bool { return it.Outcome == asks.Failed })
			}
		}
		if !first || !second || !failed {
			t.Errorf("the records hold a first-pass group %t, a second-pass group %t, a second-pass item failed %t; want all three", first, second, failed)
		}
	})
}

// TestAStageWritesNoCommonFileWhileItAsks: at every ask of each inference
// stage, every build record and both kb-root/.index/ record files hold the
// bytes they held before the stage, and its group records are not yet
// written; after the stage, they are.
func TestAStageWritesNoCommonFileWhileItAsks(t *testing.T) {
	answer := func(_ context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
		return asks.Reply{Text: q.Offered[0]}, nil
	}
	for _, tc := range askingStages {
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
