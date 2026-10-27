package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/modelclient"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// TestAskConcurrencyTakesTheFlagThenTheKeyThenTwo: N is --ask-concurrency
// where the invocation gave it, else [asks] concurrency, else 2; a flag under
// 1 is a usage refusal naming the flag, whatever the key says.
func TestAskConcurrencyTakesTheFlagThenTheKeyThenTwo(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flag, key *int
		want      int
		refused   bool
	}{
		{"neither", nil, nil, 2, false},
		{"the key", nil, new(3), 3, false},
		{"the flag over the key", new(5), new(3), 5, false},
		{"the flag alone", new(1), nil, 1, false},
		{"a flag of 0", new(0), new(3), 0, true},
		{"a negative flag", new(-2), nil, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := askConcurrency(tc.flag, tc.key)
			if tc.refused {
				var refusal toolresult.Refusal
				if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Check != checkUsage || refusal[0].Key != "--ask-concurrency" {
					t.Errorf("askConcurrency = %d, %v; want one usage refusal naming --ask-concurrency", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("askConcurrency = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}

// TestBuildAndScoreRefuseAnAskConcurrencyUnderOne: each verb refuses the flag
// under 1 in its result document, naming the flag, before it reaches anything;
// build's refusal names no state-dir and leaves the state store uncreated.
func TestBuildAndScoreRefuseAnAskConcurrencyUnderOne(t *testing.T) {
	refusedKey := func(t *testing.T, doc resultDoc) {
		t.Helper()
		items := doc.items(toolresult.RefusalsKey)
		if doc.Outcome != toolresult.Refused || len(items) != 1 || items[0]["check"] != checkUsage || items[0]["key"] != "--ask-concurrency" {
			t.Errorf("outcome %s, refusals %v; want one usage refusal naming --ask-concurrency", doc.Outcome, items)
		}
	}
	t.Run("score", func(t *testing.T) {
		got := scoreIn(t, scoreRepo(t), t.TempDir(), scoreOptions{Pass: query.PassRigor, AskConcurrency: new(0)}, nil)
		refusedKey(t, got.doc)
	})
	t.Run("build", func(t *testing.T) {
		state := filepath.Join(t.TempDir(), "state")
		var stdout, stderr bytes.Buffer
		if _, err := runBuild(context.Background(), buildOptions{VolumeRoots: []string{"paper.tex"}, WorkDir: t.TempDir(),
			StateDir: state, AskConcurrency: new(0), Stdout: &stdout, Stderr: &stderr, Logger: log.Discard()}); err != nil {
			t.Fatal(err)
		}
		doc := checkDocument(t, "build", stdout.String())
		refusedKey(t, doc)
		if _, ok := doc.Values["state-dir"]; ok {
			t.Errorf("the refusal carries state-dir: %v", doc.Values["state-dir"])
		}
		if _, err := os.Stat(state); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the state store %s exists after the refusal (stat: %v)", state, err)
		}
	})
}

// TestScoreResumeLeavesOutTheAskConcurrency: a score run's resume command does
// not carry the --ask-concurrency it ran at.
func TestScoreResumeLeavesOutTheAskConcurrency(t *testing.T) {
	resume := scoreResume(scoreOptions{AskConcurrency: new(3)}, "run", "state", config.ModeFile, config.Overrides{})
	if strings.Contains(resume, "--ask-concurrency") {
		t.Errorf("resume = %q, want no --ask-concurrency", resume)
	}
}

// TestBuildAndScoreSayReaderConcurrencyIsSuperseded: a file-mode build or
// score whose configuration states [asks] readerConcurrency — 0 here, which
// the load no longer refuses — says once on stderr that the key is
// superseded, and one without it never names it; the score run runs as the
// run without it does.
func TestBuildAndScoreSayReaderConcurrencyIsSuperseded(t *testing.T) {
	const line = "config.toml states the superseded [asks] readerConcurrency key; kbase does not use it — set asks.concurrency instead"
	for _, verb := range []struct {
		name string
		// stderr is the verb's stderr with the key stated and without it.
		stderr func(t *testing.T) (with, without string)
	}{
		{"score", func(t *testing.T) (string, string) {
			run := func(readerConcurrency *int) scoreResult {
				opts := scoreOptions{Pass: query.PassRigor, providerOptions: scoreProvider()}
				opts.Config.Asks.ReaderConcurrency = readerConcurrency
				return scoreIn(t, scoreFixture(t), t.TempDir(), opts, func() model.Client { return scoring("none", "none", "none", "none") })
			}
			with, without := run(new(0)), run(nil)
			// What differs between two runs of one set is where each ran and
			// the id it minted.
			ownRun := func(k string, _ any) bool {
				return slices.Contains([]string{"kb-root", "state-dir", "run", "ledger"}, k)
			}
			a, b := maps.Clone(with.doc.Values), maps.Clone(without.doc.Values)
			maps.DeleteFunc(a, ownRun)
			maps.DeleteFunc(b, ownRun)
			if !slices.Equal(with.doc.Keys, without.doc.Keys) || !reflect.DeepEqual(a, b) {
				t.Errorf("the run stating the key:\n%s\nwant the run without it:\n%s", with.stdout, without.stdout)
			}
			return with.stderr, without.stderr
		}},
		{"build", func(t *testing.T) (string, string) {
			resolve := func(readerConcurrency *int) string {
				p := scoreProvider()
				p.Config.Asks.ReaderConcurrency = readerConcurrency
				var stderr bytes.Buffer
				reg := modelclient.New(func(model.Endpoint) model.Client { return &model.ClientFuncs{} }, nil, "", 0)
				if _, err := buildProvider(p, config.Overrides{}, &stderr)(reg); err != nil {
					t.Fatal(err)
				}
				return stderr.String()
			}
			return resolve(new(0)), resolve(nil)
		}},
	} {
		t.Run(verb.name, func(t *testing.T) {
			with, without := verb.stderr(t)
			if n := strings.Count(with, line); n != 1 {
				t.Errorf("stderr = %q, want the superseded-key line once", with)
			}
			if strings.Contains(without, "readerConcurrency") {
				t.Errorf("a run without the key names it: %q", without)
			}
		})
	}
}

// slowScoring is a scoring provider each of whose calls takes delay before it
// answers.
type slowScoring struct {
	scripted *modeltest.MockClient
	delay    time.Duration
}

func (s slowScoring) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	time.Sleep(s.delay)
	return s.scripted.ConsultStream(ctx, req)
}

// Client is the model.Client whose calls s answers.
func (s slowScoring) Client() model.Client { return &model.ClientFuncs{Stream: s.ConsultStream} }

// TestScoreTimesTheCallNotTheWait: with one call in flight and the items
// queued behind slow calls, --timeout bounds each call from its own start —
// longer than one call, shorter than the queue's whole wait — and no item
// fails for the time it waited for its token.
func TestScoreTimesTheCallNotTheWait(t *testing.T) {
	const delay = 100 * time.Millisecond
	opts := scoreOptions{Pass: query.PassRigor, AskConcurrency: new(1), providerOptions: scoreProvider()}
	opts.Timeout = 3 * delay
	got := scoreIn(t, scoreFixture(t), t.TempDir(), opts, func() model.Client {
		return slowScoring{scripted: scriptedScoring("none", "none", "none", "none"), delay: delay}.Client()
	})
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if got.doc.Outcome != toolresult.Done || counts["done"] != 4 {
		t.Errorf("outcome %s, counts %v; want all four items done though the last waited past --timeout\n%s",
			got.doc.Outcome, counts, got.stdout)
	}
}
