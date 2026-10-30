package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// TestAskConcurrencyTakesTheFlagThenTheKeyThenTheDefault: N is --ask-concurrency
// where the invocation gave it, else [asks] concurrency, else the default; a flag under
// 1 is a usage refusal naming the flag, whatever the key says.
func TestAskConcurrencyTakesTheFlagThenTheKeyThenTheDefault(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flag, key *int
		want      int
		refused   bool
	}{
		{"neither", nil, nil, config.DefaultAskConcurrency, false},
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
				if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Check != toolresult.CheckUsage || refusal[0].Key != "--ask-concurrency" {
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
		if doc.Outcome != toolresult.Refused || len(items) != 1 || items[0]["check"] != toolresult.CheckUsage || items[0]["key"] != "--ask-concurrency" {
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
	resume := scoreResume(scoreOptions{AskConcurrency: new(3)}, "run", "state")
	if strings.Contains(resume, "--ask-concurrency") {
		t.Errorf("resume = %q, want no --ask-concurrency", resume)
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
