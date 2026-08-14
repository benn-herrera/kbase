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
// function the verb calls, verified by the same nine gates.
//
// It is a synthetic corpus rather than the pinned one for a reason worth
// stating: Rojo's docs resolve ZERO internal links (their destinations are
// site-root-relative and do not match the file layout), so a build over them
// exercises the rebase map's inventory path and never its rewriting path. This
// corpus links document to document and section to section, so the delivered
// tree here has rebased targets in it — and the gates get to prove they
// resolve.
const (
	devBuildDate = "2026-08-13"

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

func build(t *testing.T, root, out string) (devBuildResult, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	res, err := runDevBuild(context.Background(), devBuildOptions{
		Root:      root,
		Out:       out,
		Budget:    treeplan.DefaultBudgets().LeafTokens,
		BuildDate: devBuildDate,
		Stdout:    &stdout,
		Stderr:    &stderr,
		Logger:    log.Discard(),
	})
	if err != nil {
		t.Fatalf("runDevBuild: %v\nstderr:\n%s", err, stderr.String())
	}
	return res, stdout.String()
}

func TestDevBuildDeliversAVerifiedTree(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, stdout := build(t, root, out)

	if !res.Report.Passed() {
		t.Fatalf("the gates did not pass:\n%s", stdout)
	}
	if len(res.Report.Checks) != 9 {
		t.Fatalf("the report holds %d checks, want nine", len(res.Report.Checks))
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
	for _, name := range []string{"entry-point.md", assemble.AgentsFixture, assemble.ReadmeFixture, devBuildRecordName} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s was not delivered: %v", name, err)
		}
	}

	// A successful run tears the scratch tree down.
	if _, err := os.Stat(filepath.Join(out, pipeline.TempWorkDirName)); !os.IsNotExist(err) {
		t.Errorf("%s survived a successful run", pipeline.TempWorkDirName)
	}
}

// The spot check the design's check 7 makes over the whole tree, made here
// from outside the job: a delivered page is byte-identical to a fresh
// derivation from the corpus bytes, the tree plan and the rebase map.
func TestDevBuildPagesReDeriveFromSource(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, _ := build(t, root, out)

	corpus, err := ingest.Walk(root, markdown.Extensions(), log.Discard())
	if err != nil {
		t.Fatalf("ingest.Walk: %v", err)
	}
	art, err := markdown.Survey(corpus, tokens.Estimator{}, log.Discard())
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	d, err := distill.New(res.Plan, art, corpus, nil,
		distill.Provenance{CorpusHash: art.Corpus.ContentHash, BuildDate: devBuildDate})
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
func TestDevBuildRebasesResolvedLinksOnly(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, _ := build(t, root, out)

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
func TestDevBuildIsDeterministic(t *testing.T) {
	root := writeCorpus(t)
	base := t.TempDir()
	first, _ := build(t, root, filepath.Join(base, "a"))
	second, _ := build(t, root, filepath.Join(base, "b"))

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

func TestDevBuildRefusals(t *testing.T) {
	root := writeCorpus(t)
	tests := []struct {
		name string
		opts devBuildOptions
	}{
		{"no corpus", devBuildOptions{Out: t.TempDir(), Budget: 100}},
		{"no out", devBuildOptions{Root: root, Budget: 100}},
		{"a budget of zero", devBuildOptions{Root: root, Out: t.TempDir(), Budget: 0}},
		{"a negative budget", devBuildOptions{Root: root, Out: t.TempDir(), Budget: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.Stdout, opts.Stderr, opts.Logger = &bytes.Buffer{}, &bytes.Buffer{}, log.Discard()
			if _, err := runDevBuild(context.Background(), opts); err == nil {
				t.Fatal("the verb accepted an invocation it cannot honour")
			}
		})
	}
}

// The keep switch, and the one test that runs without it: the build above
// proves the scratch tree is torn down on success, and this one proves the
// switch overrides that. Together they guard against logic that silently
// depends on either.
func TestDevBuildKeepsTempWorkOnRequest(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	var stdout, stderr bytes.Buffer
	if _, err := runDevBuild(context.Background(), devBuildOptions{
		Root: root, Out: out, Budget: treeplan.DefaultBudgets().LeafTokens,
		BuildDate: devBuildDate, KeepTempWork: true,
		Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
	}); err != nil {
		t.Fatalf("runDevBuild: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, pipeline.TempWorkDirName)); err != nil {
		t.Fatalf("--keep-temp-work did not keep the scratch tree: %v", err)
	}
}

func TestDevBuildRunRecord(t *testing.T) {
	root := writeCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, _ := build(t, root, out)

	data, err := os.ReadFile(filepath.Join(out, devBuildRecordName))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec devBuildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.CorpusHash != res.Plan.CorpusHash {
		t.Errorf("the record names corpus %s, the tree plan %s", rec.CorpusHash, res.Plan.CorpusHash)
	}
	if rec.BuildDate != devBuildDate {
		t.Errorf("the record stamps %s, the build stamped %s", rec.BuildDate, devBuildDate)
	}
	if len(rec.Verify) != 9 {
		t.Errorf("the record carries %d verify results, want nine", len(rec.Verify))
	}
	if rec.Nodes != len(res.Plan.Nodes) {
		t.Errorf("the record counts %d nodes, the tree plan holds %d", rec.Nodes, len(res.Plan.Nodes))
	}
}
