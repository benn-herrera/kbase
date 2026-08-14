package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/assemble"
	"kbase/internal/distill"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// The build over the PINNED corpus, and the artifact it leaves behind.
//
// It skips when the corpus is absent — a unit-test run must not reach the
// network — and the integration recipe is where it is guaranteed to run. The
// evidence rule applies in full here: the knowledge base itself is the result,
// so the KB, its run record and this test's own measurements all land under
// test_data/transient/ where a human can walk them. A build whose only trace
// was a green tick would be a result nobody can examine.
const (
	rojoBuildCorpus = "test_data/transient/rojo.space/docs"

	// rojoBuildEvidence is this test's output root, named for the recipe that
	// runs it so the log the recipe tees and the artifacts the test writes sit
	// together.
	rojoBuildEvidence = "test_data/transient/test-integration-rojo-build"

	// rojoBuildDate is pinned so two runs of this test produce a
	// byte-identical tree — the determinism §8.1 asks for, which a clock would
	// otherwise break once a day.
	rojoBuildDate = "2026-01-01"
)

func TestDevBuildOverRealCorpus(t *testing.T) {
	root := moduleRoot(t)
	corpusDir := filepath.Join(root, filepath.FromSlash(rojoBuildCorpus))
	if _, err := os.Stat(corpusDir); err != nil {
		t.Skipf("the pinned corpus is not present (%s); run `just prep-test-integration-rojo`", rojoBuildCorpus)
	}
	evidence := filepath.Join(root, filepath.FromSlash(rojoBuildEvidence))
	out := filepath.Join(evidence, "kb")
	// A fresh tree every run: a KB left over from a previous build would make
	// a delivered-file count meaningless and a walk misleading.
	if err := os.RemoveAll(out); err != nil {
		t.Fatalf("clear the previous build: %v", err)
	}

	var stdout, stderr bytes.Buffer
	res, err := runDevBuild(context.Background(), devBuildOptions{
		Root:      corpusDir,
		Out:       out,
		Budget:    treeplan.DefaultBudgets().LeafTokens,
		BuildDate: rojoBuildDate,
		// Always on for an integration test (AGENTS.md): the intermediates —
		// the survey, the tree plan, the leaves, the stamps and the verify
		// report — are as much of the evidence as the tree is.
		KeepTempWork: true,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Logger:       log.Discard(),
	})
	if err != nil {
		t.Fatalf("dev-build over the pinned corpus: %v\nstderr:\n%s", err, stderr.String())
	}
	t.Logf("dev-build over %s:\n%s", rojoBuildCorpus, stdout.String())

	// 1. The gates are green.
	for _, c := range res.Report.Checks {
		if !c.OK {
			t.Errorf("gate %d (%s) refused: %s", c.Number, c.Name, c.Detail)
		}
	}
	if len(res.Report.Checks) != 9 {
		t.Errorf("the report holds %d checks, want nine", len(res.Report.Checks))
	}

	// 2. The node counts match the tree plan, and the delivered set is the
	//    nodes plus the fixture manifest and nothing else.
	if want := len(res.Plan.Nodes) + len(assemble.Manifest()); len(res.Delivered) != want {
		t.Errorf("delivered %d files, want %d nodes plus %d fixtures",
			len(res.Delivered), len(res.Plan.Nodes), len(assemble.Manifest()))
	}
	if res.Report.Leaves+res.Report.Indexes != len(res.Plan.Nodes) {
		t.Errorf("%d pages plus %d sections is not the plan's %d nodes",
			res.Report.Leaves, res.Report.Indexes, len(res.Plan.Nodes))
	}

	// 3. The entry-point is on disk and reaches every domain.
	entry, err := os.ReadFile(filepath.Join(out, "entry-point.md"))
	if err != nil {
		t.Fatalf("the entry-point was not delivered: %v", err)
	}
	domains := 0
	for _, n := range res.Plan.Nodes {
		if n.Parent == "entry-point.md" {
			domains++
			if !bytes.Contains(entry, []byte("("+n.Path+")")) {
				t.Errorf("the entry-point does not link to the domain %s", n.Path)
			}
		}
	}
	if domains == 0 {
		t.Error("the tree has no domains")
	}

	// 4. A page, byte-for-byte against a derivation made outside the job.
	spotCheckPage(t, corpusDir, out, res.Plan)

	writeBuildEvidence(t, evidence, res, stdout.String())
}

// spotCheckPage re-derives ONE delivered page from the corpus bytes, through a
// distiller this test builds itself, and compares.
//
// It is the same statement check 7 makes inside the job, made from outside it:
// the value here is that nothing in the build's own machinery is between the
// source file and the comparison.
func spotCheckPage(t *testing.T, corpusDir, out string, plan treeplan.TreePlan) {
	t.Helper()
	corpus, err := ingest.Walk(corpusDir, markdown.Extensions(), log.Discard())
	if err != nil {
		t.Fatalf("ingest.Walk: %v", err)
	}
	art, err := markdown.Survey(corpus, tokens.Estimator{}, log.Discard())
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	d, err := distill.New(plan, art, corpus, nil,
		distill.Provenance{CorpusHash: art.Corpus.ContentHash, BuildDate: rojoBuildDate})
	if err != nil {
		t.Fatalf("distill.New: %v", err)
	}
	pages := d.Leaves()
	if len(pages) == 0 {
		t.Fatal("the tree plan holds no pages")
	}
	// The largest page, since it is the one whose slice a derivation is most
	// likely to get wrong.
	pick := pages[0]
	best := 0
	for _, n := range pages {
		g, _ := plan.SplitGroup(n.SplitGroup)
		if size := g.Source.End - g.Source.Start; size > best {
			best, pick = size, n
		}
	}
	want, err := d.Render(pick)
	if err != nil {
		t.Fatalf("Render %s: %v", pick.Path, err)
	}
	got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(pick.Path)))
	if err != nil {
		t.Fatalf("read %s: %v", pick.Path, err)
	}
	if !bytes.Equal(want.Data, got) {
		t.Fatalf("%s is not its re-derivation from source (%d bytes delivered, %d derived)",
			pick.Path, len(got), len(want.Data))
	}
	t.Logf("spot check: %s (%d bytes) is byte-identical to its derivation from %s",
		pick.Path, len(got), pick.SplitGroup)
}

// buildEvidence is what this run measured, in a shape a diff can be taken
// over. Nothing on this path is a map: an artifact that reorders itself
// between runs cannot be diffed, and a diff is the whole reason to keep it.
type buildEvidence struct {
	Corpus     string                 `json:"corpus"`
	CorpusHash string                 `json:"corpusHash"`
	BuildDate  string                 `json:"buildDate"`
	Budgets    treeplan.Budgets       `json:"budgets"`
	Nodes      int                    `json:"nodes"`
	Pages      int                    `json:"pages"`
	Sections   int                    `json:"sections"`
	Groups     int                    `json:"groups"`
	Split      int                    `json:"splitGroups"`
	Delivered  []string               `json:"delivered"`
	Verify     []assemble.CheckResult `json:"verify"`
}

func writeBuildEvidence(t *testing.T, dir string, res devBuildResult, summary string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("make the evidence directory: %v", err)
	}
	ev := buildEvidence{
		Corpus:     rojoBuildCorpus,
		CorpusHash: res.Plan.CorpusHash,
		BuildDate:  rojoBuildDate,
		Budgets:    res.Plan.Budgets,
		Nodes:      len(res.Plan.Nodes),
		Pages:      res.Report.Leaves,
		Sections:   res.Report.Indexes,
		Groups:     len(res.Plan.Groups),
		Split:      splitGroups(res.Plan),
		Delivered:  res.Delivered,
		Verify:     res.Report.Checks,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ev); err != nil {
		t.Fatalf("encode the evidence: %v", err)
	}
	for name, data := range map[string][]byte{
		"stats.json":  buf.Bytes(),
		"summary.txt": []byte(summary),
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("evidence: %s (%d bytes)", path, len(data))
	}
}

// moduleRoot walks up to the directory holding go.mod, so a corpus path is
// resolved against the module rather than against whatever directory the test
// binary was started in.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package under test")
		}
		dir = parent
	}
}
