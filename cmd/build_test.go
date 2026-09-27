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
	"kbase/internal/dissect"
	"kbase/internal/distill"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/pipeline"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// The hermetic end-to-end: a corpus this test writes, built by the same
// function the verb calls, verified by the same ten gates.
//
// It is a synthetic corpus rather than a pinned one because a unit test may
// not depend on a fetched corpus being present: this one links document to
// document, section to section, within a page, and at one destination that
// names nothing, so every landing rule and the rule-3 exemption are exercised
// without leaving the temp directory. The pinned corpora prove the same
// pipeline through the front door, in the `just test-integration-build-*`
// recipes.
// These are UNIT tests of the verb's logic: they call runBuild in process,
// with no binary and no network. The integration tests that prove the same
// pipeline through the front door are the `just test-integration-build-*`
// recipes, which run ./bin/kbase and nothing else.
const (
	pinnedBuildDate = "2026-08-13"

	// Each document is written over the content floor a tree plan enforces
	// (dissect.MinTokens): a whole document under it has no adjacent material
	// of its own to merge into, and composition refuses rather than deliver a
	// page holding nothing.
	corpusIntro = `# Introduction

Welcome. Read [the sync page](sync.md) and the [details of syncing](sync.md#details).

This paragraph is here so the document carries more material than the minimum a
delivered page may hold, which is what keeps this fixture about link landings
rather than about the content floor.

## Getting Started

Install it, then read on. See also [the missing page](gone.md).

Installation is out of scope for a fixture, so this paragraph stands in for the
steps a real corpus would spell out here, and carries the section along with it.
`

	corpusSync = `# Sync

How syncing works.

Syncing is the subject of this document, and this paragraph is the material that
makes it long enough to be a page of its own rather than a fragment merged into
whatever sits beside it.

## Overview

The overview, with a [jump](#details) inside the page.

An overview would normally describe the moving parts and how they fit together;
here it exists to give the section a body worth reading.

## Details

The details.

The details would normally be the longest part of a page like this one, and this
paragraph stands in for them so the section is not a stub.
`
)

// corpusHelp is the corpus's non-document: a reStructuredText file, which is
// the class the walk ignores by policy — a format that arrives as Markdown
// through an external converter or not at all (ARCHITECTURE §4 row 1), and the
// class B-3 was filed over. It is here so every build in this file has
// something to exclude, and the run record has a denominator to state.
//
// It used to be an `.mdx` file. MDX is a DOCUMENT now (ruled 2026-08-18,
// markdown.Extensions), so a fixture using it to stand for the ignored class
// would have been testing the exclusion machinery against nothing.
const corpusHelp = "Help\n====\n\nA documentation format kbase does not read.\n"

func writeCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{
		"intro.md": corpusIntro, "sync.md": corpusSync, "help.rst": corpusHelp,
	} {
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
//
// Both streams come back: the report is on stdout and the verb's user-facing
// warnings are on stderr, so a test that could only read one of them could not
// assert what a successful-but-noteworthy run told the operator.
func build(t *testing.T, root, out string, keepTempWork bool) (buildResult, string, string) {
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
	return res, stdout.String(), stderr.String()
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
	res, stdout, _ := build(t, root, out, false)

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
	// tree plan's nodes plus the fixture manifest (§4.7).
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
	res, _, _ := build(t, root, out, false)

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
	res, _, _ := build(t, root, out, false)

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
		t.Error("the unresolvable link was not left verbatim; §4.5 rule 3 was never exercised")
	}
}

// Same inputs, byte-identical tree (§5). The build date is an input, which
// is exactly why it is a flag and not a clock read inside the renderer.
func TestBuildIsDeterministic(t *testing.T) {
	root := writeCorpus(t)
	base := t.TempDir()
	first, _, _ := build(t, root, filepath.Join(base, "a"), false)
	second, _, _ := build(t, root, filepath.Join(base, "b"), false)

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
	res, _, _ := build(t, root, out, true)

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
	// The link census (§3.8): the measurement guarantee 1's exemption is taken
	// against. This corpus resolves two cross-references, cannot resolve
	// `gone.md`, and jumps within a page once — so a record that carried no
	// census would differ from a record that carried an honest one.
	if want := (survey.LinkTotals{Internal: 2, Unresolved: 1, Anchor: 1}); rec.Links != want {
		t.Errorf("link census = %+v, want %+v", rec.Links, want)
	}
	// The corpus denominator (§3.8): this corpus holds three files and kbase
	// ingested two, and the record is the only place that difference is
	// visible [MAD2: B-3].
	if want := (ingest.Exclusion{Path: "help.rst", Reason: ingest.ExcludedNotADocument}); rec.Excluded != 1 ||
		len(rec.Exclusions) != 1 || rec.Exclusions[0] != want {
		t.Errorf("the record excludes %d paths %+v, want 1: %+v", rec.Excluded, rec.Exclusions, want)
	}
	if rec.Files+rec.Excluded != 3 {
		t.Errorf("%d ingested + %d excluded = %d, and the corpus root holds 3 files",
			rec.Files, rec.Excluded, rec.Files+rec.Excluded)
	}
}

// The delivery precondition (§1.5, §3.1): a `--out` that already holds files is
// refused before anything is read, with every path named and nothing written.
// Rerunning a build over a delivered knowledge base is the standing case, and it
// is now a refusal rather than a silent overwrite of whatever the new plan does
// not happen to name [MAD2: B-7].
func TestBuildRefusesAPopulatedOut(t *testing.T) {
	root := writeCorpus(t)

	// Kept, so the refused rerun sees the shape the integration recipes leave:
	// a delivered tree beside the temp-work of the run that delivered it. That
	// temp-work is the one thing that could be mistaken for resume material, and
	// the run record in it is what tells the two apart.
	t.Run("a delivered knowledge base", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "kb")
		build(t, root, out, true)
		entry := filepath.Join(out, "entry-point.md")
		before, err := os.ReadFile(entry)
		if err != nil {
			t.Fatalf("read the first run's entry point: %v", err)
		}

		var stdout, stderr bytes.Buffer
		_, err = runBuild(context.Background(), buildOptions{
			Root: root, Out: out, BuildDate: pinnedBuildDate,
			Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
		})
		if err == nil {
			t.Fatal("the verb delivered a second time into a populated --out")
		}
		for _, want := range []string{"entry-point.md", "CONVENTIONS.md", "nothing was written",
			"rerun, not the resume"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal does not say %q: %v", want, err)
			}
		}
		after, err := os.ReadFile(entry)
		if err != nil || !bytes.Equal(before, after) {
			t.Errorf("the refused run touched the delivered tree (err %v)", err)
		}
	})

	// --fresh is about the store, not the delivery [MAD2: B-12]. It discards
	// what a prior run left in temp-work; it does not license deleting a
	// delivered knowledge base, so a finished --out refuses with the flag
	// exactly as it does without it — and refuses BEFORE the job directory is
	// opened, which is why the run record is still there afterwards.
	t.Run("--fresh does not override it", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "kb")
		build(t, root, out, true)

		var stdout, stderr bytes.Buffer
		_, err := runBuild(context.Background(), buildOptions{
			Root: root, Out: out, BuildDate: pinnedBuildDate, Fresh: true,
			Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
		})
		if err == nil {
			t.Fatal("--fresh delivered a second time into a populated --out")
		}
		for _, want := range []string{"nothing was written", "rerun, not the resume"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal does not say %q: %v", want, err)
			}
		}
		if _, err := os.Stat(recordPath(out)); err != nil {
			t.Errorf("the refused --fresh run discarded the finished run's temp-work: %v", err)
		}
	})

	// Not only a knowledge base: --out receives the delivered tree and nothing
	// else, so anything already living there refuses.
	t.Run("an unrelated file", func(t *testing.T) {
		out := t.TempDir()
		if err := os.WriteFile(filepath.Join(out, "notes.md"), []byte("mine\n"), 0o600); err != nil {
			t.Fatalf("write the unrelated file: %v", err)
		}
		var stdout, stderr bytes.Buffer
		_, err := runBuild(context.Background(), buildOptions{
			Root: root, Out: out, BuildDate: pinnedBuildDate,
			Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
		})
		if err == nil {
			t.Fatal("the verb delivered into a directory holding someone else's file")
		}
		for _, want := range []string{"notes.md", "no " + pipeline.TempWorkDirName + "/ here"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal does not say %q: %v", want, err)
			}
		}
	})
}

// The exception the refusal above is written around: an interrupted build owns
// the residue it left in --out, so the next run of the same --out finishes it.
//
// interrupt reproduces the state a kill leaves — a temp-work tree with no run
// record, the record being written only once a delivery completes — which is
// exactly the state the resume predicate reads.
func TestBuildResumesAnInterruptedJob(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	build(t, root, out, true)
	interrupt(t, out)

	res, _, _ := build(t, root, out, true)
	if !res.Report.Passed() {
		t.Fatal("the resumed run did not pass its gates")
	}
	if _, err := os.Stat(recordPath(out)); err != nil {
		t.Errorf("the resumed run wrote no run record: %v", err)
	}
}

// What --fresh overrides: the store's prior state [MAD2: B-12]. Both runs below
// are the same interrupted-job shape — the one shape a second run over one
// --out legitimately takes (§3.1) — so the flag is the only difference between
// them, and the unit counts say what it bought: the resume proves every
// artifact the first run left and spends nothing, the fresh run discards them
// and rebuilds the pipeline from the corpus.
func TestBuildFreshRebuildsInsteadOfResuming(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	rerun := func(fresh bool) buildResult {
		t.Helper()
		var stdout, stderr bytes.Buffer
		res, err := runBuild(context.Background(), buildOptions{
			Root: root, Out: out, BuildDate: pinnedBuildDate, KeepTempWork: true, Fresh: fresh,
			Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
		})
		if err != nil {
			t.Fatalf("runBuild(fresh=%t): %v\nstderr:\n%s", fresh, err, stderr.String())
		}
		return res
	}
	build(t, root, out, true)

	interrupt(t, out)
	resumed := rerun(false)
	if resumed.Job.Mode != pipeline.ModeResume {
		t.Errorf("the default run ran in %s mode", resumed.Job.Mode)
	}
	if resumed.Job.Reused != resumed.Job.Units || resumed.Job.Produced != 0 {
		t.Errorf("the resumed run reused %d of %d units and produced %d, want all and none",
			resumed.Job.Reused, resumed.Job.Units, resumed.Job.Produced)
	}

	interrupt(t, out)
	fresh := rerun(true)
	if fresh.Job.Mode != pipeline.ModeFresh {
		t.Errorf("--fresh ran in %s mode", fresh.Job.Mode)
	}
	if fresh.Job.Reused != 0 || fresh.Job.Produced != fresh.Job.Units {
		t.Errorf("--fresh reused %d units and produced %d of %d, want none reused and all produced",
			fresh.Job.Reused, fresh.Job.Produced, fresh.Job.Units)
	}
	if !fresh.Report.Passed() {
		t.Error("the fresh run did not pass its gates")
	}
	if len(fresh.Delivered) != len(resumed.Delivered) {
		t.Errorf("--fresh delivered %d files, the resume %d", len(fresh.Delivered), len(resumed.Delivered))
	}
}

// interrupt makes <out> look like the residue of a build that never finished:
// the run record goes, the temp-work tree and the delivered files stay.
func interrupt(t *testing.T, out string) {
	t.Helper()
	if err := os.Remove(recordPath(out)); err != nil {
		t.Fatalf("remove the run record: %v", err)
	}
}

// tryBuild runs the verb over a corpus for the cases where the error IS the
// result. Everything but the corpus, the output directory and the title is the
// same as build's.
func tryBuild(t *testing.T, root, out, title string) error {
	t.Helper()
	var stdout, stderr bytes.Buffer
	_, err := runBuild(context.Background(), buildOptions{
		Root: root, Out: out, Title: title, BuildDate: pinnedBuildDate, KeepTempWork: true,
		Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
	})
	return err
}

// priorManifest is the delivery manifest the last run of this --out left at the
// temp-work root — read through the same function the next run reads it with.
func priorManifest(t *testing.T, out string) *deliveryManifest {
	t.Helper()
	m, err := readDeliveryManifest(filepath.Join(out, pipeline.TempWorkDirName))
	if err != nil {
		t.Fatalf("read %s: %v", deliveryManifestName, err)
	}
	if m == nil {
		t.Fatalf("a delivered run left no %s at the temp-work root", deliveryManifestName)
	}
	return m
}

// plantPriorPage fabricates a page a PREVIOUS plan delivered: the file under
// <out>, and the manifest entry claiming it — which is what a run whose plan
// has since changed leaves behind, and the only thing that licenses deleting a
// file kbase did not just write.
func plantPriorPage(t *testing.T, out, rel string) {
	t.Helper()
	dst := filepath.Join(out, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, []byte("# A page of the plan before this one\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	m := priorManifest(t, out)
	if err := writeDeliveryManifest(filepath.Join(out, pipeline.TempWorkDirName),
		append(m.Paths, rel)); err != nil {
		t.Fatalf("rewrite %s: %v", deliveryManifestName, err)
	}
}

// plantUserFile writes a file kbase never delivered and never claimed.
func plantUserFile(t *testing.T, out, rel string) string {
	t.Helper()
	const body = "notes of my own\n"
	dst := filepath.Join(out, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return body
}

// The kill window between the last delivered byte and the run record [GO H-2].
//
// The record is what tells a later run that this --out belongs to a build that
// finished (§3.1), and it is written after the delivery — so a kill in between,
// or any error from the reads that sit there, leaves a COMPLETE knowledge base
// that reads as an interrupted job forever after. Every subsequent build would
// then silently overwrite it, the populated---out refusal never firing again.
//
// What closes it is the delivery manifest: this run finds the previous one's
// list, verifies every path on it byte for byte against the artifact it just
// proved, and concludes the delivery happened. It writes the record and stops
// — no copy, and afterwards the refusal is armed again.
func TestBuildCompletesADeliveryWhoseRecordWasLost(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	first, _, _ := build(t, root, out, true)

	// The manifest is temp-work, like the record beside it: named in §3.9's
	// inventory, never delivered.
	if len(priorManifest(t, out).Paths) != len(first.Delivered) {
		t.Errorf("the manifest names %d paths, the run delivered %d",
			len(priorManifest(t, out).Paths), len(first.Delivered))
	}
	if _, err := os.Stat(filepath.Join(out, deliveryManifestName)); !os.IsNotExist(err) {
		t.Errorf("%s was delivered; it is temp-work, not a knowledge base file", deliveryManifestName)
	}

	entry := filepath.Join(out, "entry-point.md")
	before, err := os.Stat(entry)
	if err != nil {
		t.Fatalf("stat the delivered entry point: %v", err)
	}
	interrupt(t, out)

	res, _, _ := build(t, root, out, true)
	if len(res.Delivered) != len(first.Delivered) {
		t.Errorf("the completing run reports %d delivered files, the first run delivered %d",
			len(res.Delivered), len(first.Delivered))
	}
	after, err := os.Stat(entry)
	if err != nil {
		t.Fatalf("stat the entry point after the completing run: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("%s was rewritten; the delivery was already complete and there was nothing to copy", entry)
	}
	if _, err := os.Stat(recordPath(out)); err != nil {
		t.Errorf("the completing run wrote no run record, so the window is still open: %v", err)
	}

	// The consequence that made this a defect rather than an inefficiency: the
	// refusal has to be armed again afterwards.
	err = tryBuild(t, root, out, "")
	if err == nil {
		t.Fatal("a third run delivered into the finished directory; the B-7 refusal never came back")
	}
	if !strings.Contains(err.Error(), "rerun, not the resume") {
		t.Errorf("the refusal does not diagnose a finished build: %v", err)
	}
}

// The surgical sweep [ARCH F-1]: a resumed run whose plan changed delivers only
// what the new plan names, so the old plan's pages would otherwise stand in the
// delivered tree beside the new ones — all ten gates green over a tree nobody
// delivered.
//
// What may be deleted is decided by the previous run's own manifest and by
// nothing else. That is the whole safety argument, and the second case is its
// proof: a file kbase never wrote is not on any list kbase ever made.
func TestBuildSweepsOnlyThePriorDeliverysOwnPaths(t *testing.T) {
	const orphan = "gone-domain/old-page.md"

	t.Run("the prior plan's page goes", func(t *testing.T) {
		root := writeCorpus(t)
		out := filepath.Join(t.TempDir(), "kb")
		build(t, root, out, true)
		plantPriorPage(t, out, orphan)
		interrupt(t, out)

		// A changed title is a changed plan: it is stamped into the parameter
		// digest, so the tree plan and everything under it are re-derived.
		if err := tryBuild(t, root, out, "The Second Manual"); err != nil {
			t.Fatalf("the resumed run refused: %v", err)
		}
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(orphan))); !os.IsNotExist(err) {
			t.Errorf("%s survived a resume under a plan that does not name it (err %v)", orphan, err)
		}
		// The directory it was alone in went with it, and the delivered tree
		// stands — the postcondition inside the run already proved the second
		// half, and this states it where a reader can see it.
		if _, err := os.Stat(filepath.Join(out, "gone-domain")); !os.IsNotExist(err) {
			t.Errorf("the swept page left its directory behind (err %v)", err)
		}
		if head := entryHeading(t, out); head != "# "+kbTitlePrefix+"The Second Manual" {
			t.Errorf("the resumed run delivered %q", head)
		}
	})

	t.Run("a file kbase never delivered stays", func(t *testing.T) {
		root := writeCorpus(t)
		out := filepath.Join(t.TempDir(), "kb")
		build(t, root, out, true)
		plantPriorPage(t, out, orphan)
		body := plantUserFile(t, out, "notes.md")
		interrupt(t, out)

		// The run refuses: a delivered tree holding a file no plan names is
		// exactly what the postcondition exists to catch. What it must NOT do
		// is make the tree match by deleting someone else's file.
		if err := tryBuild(t, root, out, "The Second Manual"); err == nil {
			t.Fatal("the run delivered a tree holding a file no plan names")
		}
		got, err := os.ReadFile(filepath.Join(out, "notes.md"))
		if err != nil || string(got) != body {
			t.Errorf("the sweep took a file kbase never delivered (%q, err %v)", got, err)
		}
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(orphan))); !os.IsNotExist(err) {
			t.Errorf("the sweep spared the prior plan's own page (err %v)", err)
		}
	})
}

// Delivery's postcondition: <out> minus temp-work IS the delivered set (§3.1).
//
// The ten guarantees run store-side, before a byte moves, and cannot see <out>
// at all — so the delivery step proves its own work. This plants the stray
// AFTER everything else was delivered correctly, which is also the path where
// the check is easiest to skip: the delivery is already complete, nothing is
// copied, and the run's only remaining job is the record.
func TestBuildDeliveryProvesWhatItDelivered(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	build(t, root, out, true)

	entry := filepath.Join(out, "entry-point.md")
	before, err := os.Stat(entry)
	if err != nil {
		t.Fatalf("stat the delivered entry point: %v", err)
	}
	plantUserFile(t, out, "stray.md")
	interrupt(t, out)

	err = tryBuild(t, root, out, "")
	if err == nil {
		t.Fatal("the run reported a delivered tree that holds a file it did not deliver")
	}
	for _, want := range []string{"stray.md", "no delivered path names"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	after, err := os.Stat(entry)
	if err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("the run rewrote the delivered tree before refusing (err %v)", err)
	}
}

// `mkdir <out>/temp-work` was a one-command disarming of the populated---out
// refusal [GO H-3]: the exception is for an interrupted build, and an empty
// directory anyone can create was enough to claim to be one.
//
// The predicate now asks whether the residue is a job frame this binary would
// have written — a lock, a delivery manifest, or a stamped artifact. The
// resumes that must keep working are pinned next door
// (TestBuildResumesAnInterruptedJob, TestBuildCompletesADeliveryWhoseRecordWasLost).
func TestBuildRefusesTempWorkThatIsNoJobFrame(t *testing.T) {
	root := writeCorpus(t)
	tests := []struct {
		name    string
		forge   func(t *testing.T, work string)
		wantErr bool
	}{
		{name: "an empty temp-work", forge: func(*testing.T, string) {}, wantErr: true},
		{
			name: "temp-work holding something kbase did not write",
			forge: func(t *testing.T, work string) {
				if err := os.WriteFile(filepath.Join(work, "scratch.txt"), []byte("mine\n"), 0o600); err != nil {
					t.Fatalf("write the file: %v", err)
				}
			},
			wantErr: true,
		},
		{
			name: "temp-work holding a stamped artifact",
			forge: func(t *testing.T, work string) {
				if err := os.WriteFile(filepath.Join(work, "survey.json"+pipeline.StampSuffix),
					[]byte("{}\n"), 0o600); err != nil {
					t.Fatalf("write the stamp: %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := t.TempDir()
			body := plantUserFile(t, out, "notes.md")
			work := filepath.Join(out, pipeline.TempWorkDirName)
			if err := os.Mkdir(work, 0o700); err != nil {
				t.Fatalf("forge the temp-work directory: %v", err)
			}
			tt.forge(t, work)

			err := tryBuild(t, root, out, "")
			if err == nil {
				t.Fatal("the verb delivered over a directory holding someone else's file")
			}
			if got := strings.Contains(err.Error(), "no job frame"); got != tt.wantErr {
				t.Errorf("refusal names a missing job frame = %t, want %t: %v", got, tt.wantErr, err)
			}
			if got, rerr := os.ReadFile(filepath.Join(out, "notes.md")); rerr != nil || string(got) != body {
				t.Errorf("the refused run touched the operator's file (%q, err %v)", got, rerr)
			}
		})
	}
}

// The exemption is measured, never silent.
//
// §4.5 rule 3 tells the build to deliver an unresolvable destination exactly
// as the source wrote it, and guarantee 1 to let it through — both correct,
// since the defect is in someone else's corpus. What neither may do is leave
// the operator unable to tell a KB whose cross-references work from one whose
// cross-references are dead: hence one line, on the warning channel, carrying
// the whole census rather than only the bad half.
func TestBuildReportsTheExemptedDestinations(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	_, stdout, stderr := build(t, root, out, false)

	for _, want := range []string{"internal=2 unresolved=1", "exempt from guarantee 1"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not say %q:\n%s", want, stderr)
		}
	}
	if strings.Count(stderr, "\nlinks:")+strings.Count(stderr, "links:") != 1 {
		t.Errorf("the exemption gets ONE line; stderr:\n%s", stderr)
	}
	if strings.Contains(stdout, "unresolved=") {
		t.Errorf("the warning belongs on the warning channel, not in the report:\n%s", stdout)
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
//
// The two runs are an interrupted build and its resume, because that is the only
// shape a second run over one --out legitimately takes (§3.1) [MAD2: B-7] — and
// it is the shape that carries the risk: the artifacts of the run under the old
// title are all there to be wrongly proven fresh.
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
	interrupt(t, out)
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

// The content floor applies to SPLITS, not to documents (ruled 2026-08-17): a
// corpus whose whole document holds less than one page of material delivers that
// document as a page, rather than refusing for want of a legal merge. The
// floor's target is a page manufactured out of part of a larger document; an
// author's whole tiny document behind its own title is an honest page [MAD2:
// B-6 follow-on].
func TestBuildDeliversAWholeTinyDocument(t *testing.T) {
	// ~120 bytes: about 30 tokens at the shipped estimator, under half the
	// 64-token floor.
	const tiny = "# Notes\n\nA short note, and the whole of what this document has to " +
		"say about anything at all.\n"
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte(tiny), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}
	if got := (tokens.Estimator{}).Estimate(tiny); got >= dissect.MinTokens {
		t.Fatalf("the fixture is %d tokens, at or over the %d-token floor: it no longer tests the exemption",
			got, dissect.MinTokens)
	}

	out := filepath.Join(t.TempDir(), "kb")
	res, _, _ := build(t, root, out, false)
	if !res.Report.Passed() {
		t.Fatal("the gates did not pass over a corpus of one tiny document")
	}
	pages := 0
	for _, n := range res.Plan.Nodes {
		if n.Kind != treeplan.KindLeaf {
			continue
		}
		pages++
		body, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(n.Path)))
		if err != nil {
			t.Fatalf("read %s: %v", n.Path, err)
		}
		if !strings.Contains(string(body), "A short note") {
			t.Errorf("%s does not carry the document's material:\n%s", n.Path, body)
		}
	}
	if pages != 1 {
		t.Errorf("the tree holds %d pages, want the one the document is", pages)
	}
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
