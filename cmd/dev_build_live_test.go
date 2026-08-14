package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/treeplan"
)

// The verb's LIVE shape, driven by a mock model through the same composition
// the binary runs: the taxonomy descent designs the tree, the level stages
// write the summaries, and stage 8 renders their conclusions into the section
// pages.
//
// It is hermetic — the model is a scripted mock behind providerOptions'
// NewClient seam — and it is the only place the two new stages meet the
// mechanical spine before a real provider does.

// liveCorpus is shaped so every container call presents exactly TWO entries:
// two documents at the root, two top-level sections in each. That is what lets
// one scripted answer be a legal answer to every container without the test
// knowing the descent's internals.
const (
	liveDocOne = `# Alpha

The alpha section of document one.

# Beta

The beta section of document one.
`
	liveDocTwo = `# Gamma

The gamma section of document two.

# Delta

The delta section of document two.
`
)

func writeLiveCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{"one.md": liveDocOne, "two.md": liveDocTwo} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// liveScript is the scripted conversation: three container answers, then a
// summary for every section node. The taxonomy calls are one serial lane and
// come first, in order; the summaries follow, and are identical because their
// lanes fan out across workers and the order between two domains is not one.
func liveScript(t *testing.T) []model.Response {
	t.Helper()
	group := func(title string) string {
		data, err := json.Marshal(map[string]any{"groups": []map[string]any{{
			"title": title, "scope": "what a reader finds in " + title,
			"kind": "section", "members": []int{1, 2},
		}}})
		if err != nil {
			t.Fatalf("encode the grouping answer: %v", err)
		}
		return string(data)
	}
	summary, err := json.Marshal(map[string]string{
		"framing":            "This section holds the material below it.",
		"conclusionsHeading": "What it settles",
		"conclusions":        "Alpha and Beta are documented here.",
	})
	if err != nil {
		t.Fatalf("encode the summary answer: %v", err)
	}

	out := []model.Response{
		{Content: group("The Corpus"), FinishReason: "stop"},
		{Content: group("Document One"), FinishReason: "stop"},
		{Content: group("Document Two"), FinishReason: "stop"},
	}
	// Four section nodes get a summary: the entry-point, the domain the root's
	// group created, and the two the documents' groups created.
	for range 4 {
		out = append(out, model.Response{Content: string(summary), FinishReason: "stop"})
	}
	return out
}

func TestDevBuildLiveWritesTaxonomyAndSummaries(t *testing.T) {
	root := writeLiveCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	client := model.NewScriptedMockPerConsult(liveScript(t))

	var stdout, stderr bytes.Buffer
	res, err := runDevBuild(context.Background(), devBuildOptions{
		Root: root,
		Out:  out,
		Live: &providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config: config.Config{
				Provider: "solo",
				Models:   config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"},
			},
			NewClient: func(model.Endpoint) model.Client { return client },
			Stderr:    &stderr,
		},
		Budget:    treeplan.DefaultBudgets().LeafTokens,
		BuildDate: devBuildDate,
		Stdout:    &stdout,
		Stderr:    &stderr,
		Logger:    log.Discard(),
	})
	if err != nil {
		t.Fatalf("runDevBuild: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the live build did not deliver:\n%s", stdout.String())
	}

	// The tree is the model's, not the source's: the descent's own titles are
	// on the nodes, and no mechanical proposal produces them.
	for _, title := range []string{"The Corpus", "Document One", "Document Two"} {
		found := false
		for _, n := range res.Plan.Nodes {
			if n.Title == title {
				found = true
			}
		}
		if !found {
			t.Errorf("the tree plan holds no node the descent titled %q", title)
		}
	}

	// The conclusions block reached the delivered page, under the model's own
	// heading and above the down-link list.
	var section string
	for _, n := range res.Plan.Nodes {
		if n.Kind == treeplan.KindIndex {
			section = n.Path
			break
		}
	}
	page, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(section)))
	if err != nil {
		t.Fatalf("read the delivered section %s: %v", section, err)
	}
	body := string(page)
	for _, want := range []string{
		"This section holds the material below it.",
		"## What it settles",
		"Alpha and Beta are documented here.",
		"## Derivations and Detail",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the delivered section is missing %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "## What it settles") > strings.Index(body, "## Derivations and Detail") {
		t.Errorf("the conclusions block renders below the down-link list:\n%s", body)
	}

	// The run record carries what a live run dialed, asked and spent — and no
	// credential.
	data, err := os.ReadFile(filepath.Join(out, devBuildRecordName))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec devBuildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live == nil {
		t.Fatal("the run record carries no live block")
	}
	if rec.Live.Model != "gemma-4-31b" || rec.Live.Tier != config.TierHeavy {
		t.Errorf("live = %+v, want the heavy tier's configured model", rec.Live)
	}
	if rec.Live.TaxonomyCalls != 3 {
		t.Errorf("%d taxonomy calls recorded, want one per container", rec.Live.TaxonomyCalls)
	}
	if rec.Summaries != 4 {
		t.Errorf("%d summaries, want one per section node", rec.Summaries)
	}
	if strings.Contains(string(data), testAPIKey) {
		t.Error("the run record carries the API key")
	}
}

// TestDevBuildWithoutConfigDirMakesNoCall is the other half of the switch: the
// mechanical shape must stay mechanical. A client that fails on every call
// proves it — the build succeeds because nothing ever asks it anything.
func TestDevBuildWithoutConfigDirMakesNoCall(t *testing.T) {
	root := writeLiveCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, stdout := build(t, root, out)
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the mechanical build did not deliver:\n%s", stdout)
	}
	data, err := os.ReadFile(filepath.Join(out, devBuildRecordName))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec devBuildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live != nil {
		t.Errorf("a run with no model in the loop reported %+v", rec.Live)
	}
	if rec.Summaries != 0 {
		t.Errorf("%d summaries were written with no model in the loop", rec.Summaries)
	}
}
