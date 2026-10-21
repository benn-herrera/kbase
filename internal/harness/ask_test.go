//go:build unix

package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ask is one item's call over a shell script: its reply, and its error.
func ask(t *testing.T, dir, script, prompt string) (string, error) {
	t.Helper()
	c, err := New([]string{"sh", "-c", script, "sh"}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	item, err := c.Item(prompt)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	return item.Ask(t.Context(), dir)
}

// TestAskReturnsTheWholeReply: stdout to EOF, trailing whitespace trimmed, and
// the process runs in the directory it was handed.
func TestAskReturnsTheWholeReply(t *testing.T) {
	dir := t.TempDir()
	got, err := ask(t, dir, `printf '  0.5\n\n  '; pwd`, "")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	// The reply is the whole of stdout: the answer and what follows it, since
	// nothing but the command's own bytes says where the answer ends.
	if !strings.HasPrefix(got, "  0.5") || !strings.HasSuffix(got, dir) {
		t.Errorf("reply = %q, want the whole of stdout with trailing space trimmed", got)
	}
	if strings.HasSuffix(got, "\n") || strings.HasSuffix(got, " ") {
		t.Errorf("reply = %q, want its trailing whitespace trimmed", got)
	}
}

// TestAskStdinIsThePrompt: a command naming no {prompt} reads the item's text
// from stdin; one naming it reads nothing there, so a command that also reads
// stdin sees the end of its input and not the prompt a second time.
func TestAskStdinIsThePrompt(t *testing.T) {
	dir := t.TempDir()
	got, err := ask(t, dir, `cat`, "the item's text")
	if err != nil || got != "the item's text" {
		t.Fatalf("stdin = %q, %v; want the prompt", got, err)
	}
	c, err := New([]string{"sh", "-c", `printf '%s|' "$1"; cat`, "sh", "{prompt}"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	item, err := c.Item("the item's text")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := item.Ask(t.Context(), dir); err != nil || got != "the item's text|" {
		t.Errorf("reply = %q, %v; want the argv-borne prompt and empty stdin", got, err)
	}
}

// TestAskFailsOnNonZeroExit: a command that exits non-zero fails the call with
// its exit code and its stderr in the reason, and nothing of that stderr
// reaches kbase's own output.
func TestAskFailsOnNonZeroExit(t *testing.T) {
	_, err := ask(t, t.TempDir(), `printf 'the stub refuses\n' >&2; exit 3`, "")
	if err == nil {
		t.Fatal("a non-zero exit did not fail the call")
	}
	for _, want := range []string{"exit status 3", "the stub refuses"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the reason = %q, want it to carry %q", err, want)
		}
	}
	if _, err := ask(t, t.TempDir(), `exit 7`, ""); err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Errorf("the reason = %v, want the exit code alone where nothing was written to stderr", err)
	}
}

// TestAskEnforcesBothBounds: a reply past ReplyLimit and stderr past StderrLimit
// each fail the call with the bound's own reason, and neither is read past it.
func TestAskEnforcesBothBounds(t *testing.T) {
	_, err := ask(t, t.TempDir(), fmt.Sprintf(`head -c %d /dev/zero | tr '\0' x`, ReplyLimit+1), "")
	if err == nil || !strings.Contains(err.Error(), "over the") || !strings.Contains(err.Error(), fmt.Sprint(ReplyLimit)) {
		t.Errorf("an over-limit reply = %v, want the reply's bound named", err)
	}
	_, err = ask(t, t.TempDir(), fmt.Sprintf(`head -c %d /dev/zero | tr '\0' x >&2`, StderrLimit+1), "")
	if err == nil || !strings.Contains(err.Error(), "stderr bound") {
		t.Errorf("over-limit stderr = %v, want the stderr bound named", err)
	}
	// The bounds hold at their own size: a reply of exactly ReplyLimit is a
	// reply, and it is what the call returns.
	reply, err := ask(t, t.TempDir(), fmt.Sprintf(`head -c %d /dev/zero | tr '\0' x`, ReplyLimit), "")
	if err != nil || len(reply) != ReplyLimit {
		t.Errorf("a reply at the bound = %d bytes, %v; want it accepted whole", len(reply), err)
	}
}

// TestAskStartsTheProgramTheProbeCertified: a command named relative to kbase's
// own directory runs — from the item's directory — as the program the probe
// found, and not as a file of that name standing where the item runs.
func TestAskStartsTheProgramTheProbeCertified(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	writeProgram(t, filepath.Join(base, "agent"), "the probed program")
	c, err := New([]string{"./agent", "--ask"}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	run := t.TempDir()
	writeProgram(t, filepath.Join(run, "agent"), "the impostor")
	item, err := c.Item("")
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	got, err := item.Ask(t.Context(), run)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "the probed program" {
		t.Errorf("the item started %q, want the program the probe certified", got)
	}
}

// writeProgram writes a shell script that answers with says.
func writeProgram(t *testing.T, path, says string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '"+says+"'\n"), 0o700); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestAskKillsTheProcessGroupOnDeadline: an item's deadline ends the command and
// the children it started, not the one process kbase holds a pid for — and the
// call returns rather than waiting the group out.
func TestAskKillsTheProcessGroupOnDeadline(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	// The touch of `finished` is a subshell's: killing the command alone would
	// leave it running long enough to write the file.
	c, err := New([]string{"sh", "-c", `touch started; (sleep 1; touch finished) & wait`, "sh"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	item, err := c.Item("")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := item.Ask(ctx, dir); err == nil {
		t.Fatal("the deadline did not end the call")
	}
	if waited := time.Since(started); waited > 5*time.Second {
		t.Errorf("the call waited %s: the group outlived its deadline", waited)
	}
	if _, err := os.Stat(filepath.Join(dir, "started")); err != nil {
		t.Fatalf("the command never ran: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "finished")); err == nil {
		t.Error("a child the command started outlived the deadline that killed the group")
	}
}
