package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/kb"
	toolresult "kbase/internal/result"
)

// TestWaitRefusesAnAbsentOrNonPositivePid: --pid names the run to wait for;
// absent or not positive is a usage refusal, before any store is read.
func TestWaitRefusesAnAbsentOrNonPositivePid(t *testing.T) {
	for name, args := range map[string][]string{
		"absent":   nil,
		"zero":     {"--pid", "0"},
		"negative": {"--pid=-1"},
	} {
		t.Run(name, func(t *testing.T) {
			out, code := kbase(t, t.TempDir(), append([]string{"wait"}, args...)...)
			doc := checkDocument(t, "wait", out)
			items := doc.items(toolresult.RefusalsKey)
			if doc.Outcome != toolresult.Refused || code != toolresult.ExitCode(toolresult.Refused) || len(items) != 1 ||
				items[0]["check"] != toolresult.CheckUsage || items[0]["key"] != "--pid" {
				t.Errorf("wait %q = outcome %s, exit %d, %v\n%s", args, doc.Outcome, code, items, out)
			}
		})
	}
}

// TestWaitReportsTheStoreItLocated: given no --state-dir, wait reads the store
// the operation lock names, and its final document is that store's although
// the run removed the lock on its way out.
func TestWaitReportsTheStoreItLocated(t *testing.T) {
	base := t.TempDir()
	storetest.GitEnv(t, base)
	repo, other := filepath.Join(base, "repo"), t.TempDir()
	storetest.MakeRepo(t, repo)
	release, err := build.Hold(filepath.Join(repo, kb.KBDir), other, buildstore.KindBuild, "", time.Now())
	if err != nil {
		t.Fatalf("hold a non-default store: %v", err)
	}
	// The run ends once wait has located its store and is polling it: wait
	// polls every waitPoll, so half of one is past its first read.
	released := make(chan error, 1)
	go func() {
		time.Sleep(waitPoll / 2)
		released <- release()
	}()
	out, _ := kbase(t, repo, "wait", "--pid", strconv.Itoa(os.Getpid()))
	if err := <-released; err != nil {
		t.Fatalf("release the store: %v", err)
	}
	if doc := checkDocument(t, "wait", out); doc.Values["state-dir"] != other {
		t.Errorf("wait reports state-dir %v, want the store the operation lock named, %s\n%s", doc.Values["state-dir"], other, out)
	}
}
