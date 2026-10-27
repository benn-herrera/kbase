package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/modelclient"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// closeLog counts the closes the clients a verb opened reported.
type closeLog struct{ n int }

// closer is a mock client that records its Close — the release the registry
// owes every client it opened.
type closer struct {
	model.Client
	log *closeLog
}

func (c closer) Close() error { c.log.n++; return nil }

// TestVerbsCloseEveryClientTheyOpen: every client a verb opens is closed
// before the verb returns, whatever the outcome — done, failed, refused and
// cancelled alike.
func TestVerbsCloseEveryClientTheyOpen(t *testing.T) {
	pool := func() config.Providers {
		return config.Providers{"solo": provider("solo", "http://solo.example/v1")}
	}
	var log closeLog
	newClient := func() modelclient.NewClientFunc {
		return func(model.Endpoint) model.Client {
			return closer{modeltest.NewScriptedMock(nil, catalogue("m1")).Client(), &log}
		}
	}

	t.Run("models, done", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		code, err := runModels(context.Background(), modelsOptions{
			providerOptions: providerOptions{Providers: pool(), Stderr: &stderr, NewClient: newClient()},
			Stdout:          &stdout,
		})
		if err != nil || code != 0 {
			t.Fatalf("models: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("models, failed", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		unreachable := modeltest.NewScriptedMock(nil, nil)
		unreachable.SetError(errors.New("http 503: service unavailable"))
		code, err := runModels(context.Background(), modelsOptions{
			providerOptions: providerOptions{Providers: pool(), Stderr: &stderr,
				NewClient: func(model.Endpoint) model.Client { return closer{unreachable.Client(), &log} }},
			Stdout: &stdout,
		})
		if err != nil || code != toolresult.ExitCode(toolresult.Failed) {
			t.Fatalf("models: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("configure, refused after the dial", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		code, err := runConfigure(context.Background(), configureOptions{
			providerOptions: providerOptions{Providers: pool(), Stderr: &stderr, NewClient: newClient(), WriteDir: t.TempDir()},
			Stdout:          &stdout,
		})
		if err != nil || code != toolresult.ExitCode(toolresult.Refused) {
			t.Fatalf("configure: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("score, cancelled", func(t *testing.T) {
		log.n = 0
		repo, state := scoreFixture(t), t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Ctx: ctx}, func() model.Client {
			return closer{(&cancelOnSecond{MockClient: scriptedScoring("0.11"), cancel: cancel}).Client(), &log}
		})
		if got.doc.Outcome != toolresult.Cancelled {
			t.Fatalf("score: outcome %s\n%s", got.doc.Outcome, got.stdout)
		}
		assertClosed(t, log.n, 1)
	})
}

// assertClosed fails t unless n clients were closed — want them all.
func assertClosed(t *testing.T, n, want int) {
	t.Helper()
	if n != want {
		t.Errorf("clients closed = %d, want %d: every client the verb opened is released before it returns", n, want)
	}
}

// TestRefusedOrFailed: an error is a verb's stop — a refusal is refused with its
// items, a live run's retry is retry, and anything else is failed.
func TestRefusedOrFailed(t *testing.T) {
	items := []toolresult.Item{{Check: config.CheckProvider, Key: "models.text", Detail: "d"}}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"a refusal", toolresult.Refusal(items), toolresult.Refused},
		{"a wrapped refusal", fmt.Errorf("resolve: %w", toolresult.Refusal(items)), toolresult.Refused},
		{"a live run", buildstore.Retry(items), toolresult.Retry},
		{"a failure", errors.New("http 503"), toolresult.Failed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if outcome, _ := refusedOrFailed(nil, tc.err); outcome != tc.want {
				t.Errorf("outcome = %q, want %q", outcome, tc.want)
			}
		})
	}
}
