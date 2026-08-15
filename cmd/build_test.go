package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/assemble"
	"kbase/internal/distill"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/pipeline"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// The hermetic end-to-end: a corpus this test writes, built by the same
// function the verb calls, verified by the same ten gates.
//
// It is a synthetic corpus rather than the pinned one for a reason worth
// stating: Rojo's docs resolve ZERO internal links (their destinations are
// site-root-relative and do not match the file layout), so a build over them
// exercises the rebase map's inventory path and never its rewriting path. This
// corpus links document to document and section to section, so the delivered
// tree here has rebased targets in it — and the gates get to prove they
// resolve.
// These are UNIT tests of the verb's logic: they call runBuild in process,
// with no binary and no network. The integration tests that prove the same
// pipeline through the front door are the `just test-integration-build-*`
// recipes, which run ./bin/kbase and nothing else.
const (
	pinnedBuildDate = "2026-08-13"

	corpusIntro = `# Introduction

Welcome. Read [the sync page](sync.md) and the [details of syncing](sync.md#details).

## Getting Started

Install it, then read on. See also [the missing page](gone.md).
`

	corpusSync = `# Sync

How syncing works.

## Overview

The overview, with a [jump](#details) inside the page.

## Details

The details.
`
)

func writeCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{"intro.md": corpusIntro, "sync.md": corpusSync} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// build runs the verb over a corpus. keepTempWork is a parameter rather than a
// constant because the run record lives at the temp-work root: a test that
// reads the record has to keep the scratch tree, and a test that asserts the
// teardown must not.
func build(t *testing.T, root, out string, keepTempWork bool) (buildResult, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root:         root,
		Out:          out,
		BuildDate:    pinnedBuildDate,
		KeepTempWork: keepTempWork,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Logger:       log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstderr:\n%s", err, stderr.String())
	}
	return res, stdout.String()
}

// recordPath is the run record's home: the temp-work root, which mirrors the
// delivered tree's root (ruled 2026-08-15). The record is a development record
// and lives in the dev mirror, so it exists only for a run that kept it.
func recordPath(out string) string {
	return filepath.Join(out, pipeline.TempWorkDirName, buildRecordName)
}

func TestBuildDeliversAVerifiedTree(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, stdout := build(t, root, out, false)

	if !res.Report.Passed() {
		t.Fatalf("the gates did not pass:\n%s", stdout)
	}
	if len(res.Report.Checks) != 10 {
		t.Fatalf("the report holds %d checks, want ten", len(res.Report.Checks))
	}
	if !res.Job.DeliveryReady() {
		t.Fatalf("the job is not delivery-ready: %d units, %d produced, %d failed",
			res.Job.Units, res.Job.Produced, len(res.Job.Failures))
	}

	// Every delivered path is on disk, and the delivered set is exactly the
	// tree plan's nodes plus the fixture manifest (§8.1).
	if want := len(res.Plan.Nodes) + len(assemble.Manifest()); len(res.Delivered) != want {
		t.Errorf("delivered %d files, want %d nodes plus fixtures", len(res.Delivered), want)
	}
	for _, p := range res.Delivered {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s is in the delivered set and not on disk: %v", p, err)
		}
	}
	for _, name := range []string{"entry-point.md", assemble.AgentsFixture, assemble.ReadmeFixture,
		assemble.ClaudeFixture} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s was not delivered: %v", name, err)
		}
	}

	// A successful run tears the scratch tree down — and the run record goes
	// with it, because it lives inside it.
	if _, err := os.Stat(filepath.Join(out, pipeline.TempWorkDirName)); !os.IsNotExist(err) {
		t.Errorf("%s survived a successful run", pipeline.TempWorkDirName)
	}
	if _, err := os.Stat(filepath.Join(out, buildRecordName)); !os.IsNotExist(err) {
		t.Errorf("%s was delivered; the run record is a development record and belongs in %s",
			buildRecordName, pipeline.TempWorkDirName)
	}
}

// The spot check the design's check 7 makes over the whole tree, made here
// from outside the job: a delivered page is byte-identical to a fresh
// derivation from the corpus bytes, the tree plan and the rebase map.
func TestBuildPagesReDeriveFromSource(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, _ := build(t, root, out, false)

	corpus, err := ingest.Walk(root, markdown.Extensions(), log.Discard())
	if err != nil {
		t.Fatalf("ingest.Walk: %v", err)
	}
	art, err := markdown.Survey(corpus, tokens.Estimator{}, log.Discard())
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	d, err := distill.New(res.Plan, art, corpus, nil,
		distill.Provenance{CorpusHash: art.Corpus.ContentHash, BuildDate: pinnedBuildDate})
	if err != nil {
		t.Fatalf("distill.New: %v", err)
	}
	pages := 0
	for _, n := range d.Leaves() {
		want, err := d.Render(n)
		if err != nil {
			t.Fatalf("Render %s: %v", n.Path, err)
		}
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(n.Path)))
		if err != nil {
			t.Fatalf("read %s: %v", n.Path, err)
		}
		if !bytes.Equal(want.Data, got) {
			t.Errorf("%s is not its re-derivation from source", n.Path)
		}
		pages++
	}
	if pages == 0 {
		t.Fatal("the build delivered no pages")
	}
}

// The rebase, end to end: a source link that resolved becomes a link to a
// delivered file, and one that did not stays exactly as the author wrote it.
func TestBuildRebasesResolvedLinksOnly(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, _ := build(t, root, out, false)

	delivered := map[string]bool{}
	for _, p := range res.Delivered {
		delivered[p] = true
	}
	rebased, verbatim := 0, 0
	for _, p := range res.Delivered {
		if !strings.HasSuffix(p, ".md") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		for _, dst := range distill.Destinations(body) {
			text := dst.Text(body)
			if distill.IsExternal(text) {
				continue
			}
			if text == "gone.md" {
				verbatim++
				continue
			}
			target := filepath.ToSlash(filepath.Clean(filepath.Join(
				filepath.Dir(p), strings.SplitN(text, "#", 2)[0])))
			if !delivered[target] {
				t.Errorf("%s links to %q, which resolves to %q and is not delivered", p, text, target)
			}
			if strings.HasSuffix(target, ".md") && !strings.Contains(p, "index.md") {
				rebased++
			}
		}
	}
	if rebased == 0 {
		t.Error("no page carries a rebased link; the rewriting path was never exercised")
	}
	if verbatim == 0 {
		t.Error("the unresolvable link was not left verbatim; §4.4 rule 3 was never exercised")
	}
}

// Same inputs, byte-identical tree (§8.1). The build date is an input, which
// is exactly why it is a flag and not a clock read inside the renderer.
func TestBuildIsDeterministic(t *testing.T) {
	root := writeCorpus(t)
	base := t.TempDir()
	first, _ := build(t, root, filepath.Join(base, "a"), false)
	second, _ := build(t, root, filepath.Join(base, "b"), false)

	if len(first.Delivered) != len(second.Delivered) {
		t.Fatalf("the two runs delivered %d and %d files", len(first.Delivered), len(second.Delivered))
	}
	for i, p := range first.Delivered {
		if second.Delivered[i] != p {
			t.Fatalf("the two runs delivered different sets: %q vs %q", p, second.Delivered[i])
		}
		a, err := os.ReadFile(filepath.Join(base, "a", filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		b, err := os.ReadFile(filepath.Join(base, "b", filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between two runs over the same corpus", p)
		}
	}
}

func TestBuildRefusals(t *testing.T) {
	root := writeCorpus(t)
	tests := []struct {
		name string
		opts buildOptions
	}{
		{"no corpus", buildOptions{Out: t.TempDir()}},
		{"no out", buildOptions{Root: root}},
		{"a build date that is not one", buildOptions{
			Root: root, Out: t.TempDir(), BuildDate: "August 2026"}},
		// §2.8, acceptance criterion 14: an annex naming nothing is a flag
		// that did not do what it said, so it refuses rather than excluding
		// nothing quietly.
		{"an annex naming no document", buildOptions{
			Root: root, Out: t.TempDir(), Annexes: []string{"reference"}}},
		{"nested annexes", buildOptions{
			Root: root, Out: t.TempDir(), Annexes: []string{"sync.md", "sync.md"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.Stdout, opts.Stderr, opts.Logger = &bytes.Buffer{}, &bytes.Buffer{}, log.Discard()
			if _, err := runBuild(context.Background(), opts); err == nil {
				t.Fatal("the verb accepted an invocation it cannot honour")
			}
		})
	}
}

// The keep switch, and the one test that runs without it: the build above
// proves the scratch tree is torn down on success, and this one proves the
// switch overrides that. Together they guard against logic that silently
// depends on either.
func TestBuildKeepsTempWorkOnRequest(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	var stdout, stderr bytes.Buffer
	if _, err := runBuild(context.Background(), buildOptions{
		Root: root, Out: out,
		BuildDate: pinnedBuildDate, KeepTempWork: true,
		Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
	}); err != nil {
		t.Fatalf("runBuild: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, pipeline.TempWorkDirName)); err != nil {
		t.Fatalf("--keep-temp-work did not keep the scratch tree: %v", err)
	}
}

func TestBuildRunRecord(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	// Kept, because the record lives in temp-work: reading it is exactly the
	// case the keep switch exists for.
	res, _ := build(t, root, out, true)

	data, err := os.ReadFile(recordPath(out))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.CorpusHash != res.Plan.CorpusHash {
		t.Errorf("the record names corpus %s, the tree plan %s", rec.CorpusHash, res.Plan.CorpusHash)
	}
	if rec.BuildDate != pinnedBuildDate {
		t.Errorf("the record stamps %s, the build stamped %s", rec.BuildDate, pinnedBuildDate)
	}
	if len(rec.Verify) != 10 {
		t.Errorf("the record carries %d verify results, want ten", len(rec.Verify))
	}
	if rec.Nodes != len(res.Plan.Nodes) {
		t.Errorf("the record counts %d nodes, the tree plan holds %d", rec.Nodes, len(res.Plan.Nodes))
	}
	// A tree nobody designed says so. The live half asserts the other value
	// (TestBuildLiveWritesTaxonomyAndSummaries).
	if rec.TreePlan != treePlanMechanical {
		t.Errorf("the record says the tree plan came from %q, want %q", rec.TreePlan, treePlanMechanical)
	}
	if rec.Budgets.LeafTokens != treeplan.DefaultBudgets().LeafTokens {
		t.Errorf("the record was built at a leaf budget of %d, want the shipped %d",
			rec.Budgets.LeafTokens, treeplan.DefaultBudgets().LeafTokens)
	}
}

// The knowledge base is named for the documentation it was built from: --title
// as the user stated it, else the corpus directory's base name. The name is
// asserted where a reader meets it — the entry-point's heading — and in the run
// record, which is where a later reader finds what this run was told.
func TestBuildNamesTheKnowledgeBase(t *testing.T) {
	root := writeCorpus(t)
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"the stated title", "Rojo v7 Documentation", kbTitlePrefix + "Rojo v7 Documentation"},
		{"blank falls back to the corpus directory", "   ", kbTitlePrefix + filepath.Base(root)},
		{"absent falls back to the corpus directory", "", kbTitlePrefix + filepath.Base(root)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "kb")
			var stdout, stderr bytes.Buffer
			if _, err := runBuild(context.Background(), buildOptions{
				Root: root, Title: tt.title, Out: out, BuildDate: pinnedBuildDate,
				// Kept, so the record this test reads survives the run.
				KeepTempWork: true,
				Stdout:       &stdout, Stderr: &stderr, Logger: log.Discard(),
			}); err != nil {
				t.Fatalf("runBuild: %v\nstderr:\n%s", err, stderr.String())
			}
			if head := entryHeading(t, out); head != "# "+tt.want {
				t.Errorf("the entry point opens %q, want %q", head, "# "+tt.want)
			}
			data, err := os.ReadFile(recordPath(out))
			if err != nil {
				t.Fatalf("read the run record: %v", err)
			}
			var rec buildRun
			if err := json.Unmarshal(data, &rec); err != nil {
				t.Fatalf("decode the run record: %v", err)
			}
			if got := kbTitlePrefix + rec.Title; got != tt.want {
				t.Errorf("the record names the doc set %q, the tree is titled %q", rec.Title, tt.want)
			}
		})
	}
}

// A retitled run over the same --out delivers the new name. The title lands in
// the tree plan, and a tree plan is reused when its inputs are unchanged — so
// the title has to be one of those inputs or a resume would prove a stale
// artifact fresh and deliver the old name out of it.
func TestBuildRetitlesAResumedRun(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	run := func(title string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if _, err := runBuild(context.Background(), buildOptions{
			Root: root, Title: title, Out: out, BuildDate: pinnedBuildDate,
			// Kept, so the second run has the first one's artifacts to resume
			// from — which is the case under test.
			KeepTempWork: true,
			Stdout:       &stdout, Stderr: &stderr, Logger: log.Discard(),
		}); err != nil {
			t.Fatalf("runBuild(%q): %v\nstderr:\n%s", title, err, stderr.String())
		}
	}
	run("The First Manual")
	run("The Second Manual")

	want := "# " + kbTitlePrefix + "The Second Manual"
	if head := entryHeading(t, out); head != want {
		t.Errorf("the resumed run delivered %q, want %q", head, want)
	}
}

// entryHeading is the delivered entry-point's H1, read under the frontmatter
// block that owns file-start on every class-A page (SPEC §4.9).
func entryHeading(t *testing.T, out string) string {
	t.Helper()
	entry, err := os.ReadFile(filepath.Join(out, "entry-point.md"))
	if err != nil {
		t.Fatalf("read the entry point: %v", err)
	}
	loc, body, ok := distill.ParseFrontmatter(entry)
	if !ok || loc != "entry-point.md" {
		t.Fatalf("the entry point declares Location %q (block found: %t)", loc, ok)
	}
	return distill.FirstLine(body)
}

// The annex, end to end through the verb: the declared prefix is exempt from
// coverage, contributes no page, and reaches the delivered entry point as a
// lookup entry whose convention text the verifier wrote from the survey
// (§2.8). A build with no --annex distils the whole corpus, which every other
// test here already proves.
func TestBuildDeliversDeclaredAnnexes(t *testing.T) {
	root := writeCorpus(t)
	if err := os.MkdirAll(filepath.Join(root, "reference"), 0o700); err != nil {
		t.Fatalf("create the annex directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "reference", "api.md"),
		[]byte("# API\n\nEvery entry point, listed.\n"), 0o600); err != nil {
		t.Fatalf("write the annexed document: %v", err)
	}

	out := filepath.Join(t.TempDir(), "kb")
	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root: root, Out: out, Annexes: []string{"reference"},
		BuildDate: pinnedBuildDate,
		Stdout:    &stdout, Stderr: &stderr, Logger: log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstderr:\n%s", err, stderr.String())
	}
	if !res.Report.Passed() {
		t.Fatalf("the gates did not pass with an annex declared:\n%s", stdout.String())
	}
	for _, p := range res.Delivered {
		if strings.HasPrefix(p, "reference") {
			t.Errorf("%s was delivered; the annex was supposed to exclude it", p)
		}
	}
	entry, err := os.ReadFile(filepath.Join(out, "entry-point.md"))
	if err != nil {
		t.Fatalf("read the entry point: %v", err)
	}
	for _, want := range []string{"## Annex lookup", "reference/<name>.md, e.g. reference/api.md"} {
		if !strings.Contains(string(entry), want) {
			t.Errorf("the entry point is missing %q:\n%s", want, entry)
		}
	}
}
