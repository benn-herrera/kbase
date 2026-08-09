package log

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      string
		want    Level
		wantErr bool
	}{
		{"empty selects the default", "", DefaultLevel, false},
		{"debug", "debug", LevelDebug, false},
		{"info", "info", LevelInfo, false},
		{"warn", "warn", LevelWarn, false},
		{"error", "error", LevelError, false},
		{"case is not significant", "WARN", LevelWarn, false},
		{"surrounding space is tolerated", "  info\n", LevelInfo, false},
		{"misspelling is an error, not a fallback", "warning", "", true},
		{"slog spelling is not accepted", "WARNING", "", true},
		{"numeric level is an error", "3", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLevel(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseLevel(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("ParseLevel(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseLevelErrorNamesTheSet keeps the error useful: a user who
// mistyped needs the accepted vocabulary, not just a rejection.
func TestParseLevelErrorNamesTheSet(t *testing.T) {
	_, err := ParseLevel("verbose")
	if err == nil {
		t.Fatal("ParseLevel(\"verbose\") = nil error, want an error")
	}
	for _, want := range []string{"verbose", "debug", "info", "warn", "error"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestLevelFiltering pins the whole point of the level knob: each
// configured level admits itself and everything more severe, and nothing
// less severe.
func TestLevelFiltering(t *testing.T) {
	for _, tc := range []struct {
		name  string
		level Level
		want  []string // messages expected to survive, in order
	}{
		{"debug admits everything", LevelDebug, []string{"d", "i", "w", "e"}},
		{"info drops debug", LevelInfo, []string{"i", "w", "e"}},
		{"warn is the default posture", LevelWarn, []string{"w", "e"}},
		{"error admits only error", LevelError, []string{"e"}},
		{"zero value matches the default level", "", []string{"w", "e"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var console bytes.Buffer
			lg, closer, err := New(Options{Level: tc.level, Console: &console})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer closeOrFail(t, closer)

			lg.Debug("d")
			lg.Info("i")
			lg.Warn("w")
			lg.Error("e")

			if got := messages(console.String()); !equal(got, tc.want) {
				t.Errorf("level %q emitted %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}

// TestUnknownLevelRejected: New validates rather than falling back, so a bad
// --log-level fails the invocation instead of silently logging at warn.
func TestUnknownLevelRejected(t *testing.T) {
	var console bytes.Buffer
	if _, _, err := New(Options{Level: "chatty", Console: &console}); err == nil {
		t.Fatal("New with an unknown level = nil error, want an error")
	}
	if console.Len() != 0 {
		t.Errorf("a failed New wrote to the console: %q", console.String())
	}
}

// TestStructuredOutputShape pins the record shape consumers will grep: a
// level, the message, and the key/value pairs as key=value.
func TestStructuredOutputShape(t *testing.T) {
	var console bytes.Buffer
	lg, closer, err := New(Options{Level: LevelInfo, Console: &console})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer closeOrFail(t, closer)

	lg.Info("stage complete", "stage", 3, "leaves", 12)

	got := console.String()
	for _, want := range []string{"level=INFO", `msg="stage complete"`, "stage=3", "leaves=12"} {
		if !strings.Contains(got, want) {
			t.Errorf("record %q is missing %q", strings.TrimSpace(got), want)
		}
	}
}

// TestFileTee: with a FilePath the file is a transcript of the console —
// same records, same format, and level filtering applies to both.
func TestFileTee(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kbase.log")
	var console bytes.Buffer

	lg, closer, err := New(Options{Level: LevelInfo, Console: &console, FilePath: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Debug("dropped")
	lg.Warn("kept", "k", "v")
	closeOrFail(t, closer)

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if string(onDisk) != console.String() {
		t.Errorf("file and console diverged:\nfile:    %q\nconsole: %q", onDisk, console.String())
	}
	if got := messages(string(onDisk)); !equal(got, []string{"kept"}) {
		t.Errorf("file holds %v, want [kept] (level filtering applies to the tee)", got)
	}
}

// TestFileTeeAppends: two runs pointed at one --log-file accumulate rather
// than the second truncating the first.
func TestFileTeeAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kbase.log")
	for _, msg := range []string{"first", "second"} {
		lg, closer, err := New(Options{Level: LevelWarn, FilePath: path, Console: io.Discard})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		lg.Warn(msg)
		closeOrFail(t, closer)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if got := messages(string(onDisk)); !equal(got, []string{"first", "second"}) {
		t.Errorf("log file holds %v, want [first second]", got)
	}
}

// TestFileTeeMode pins the owner-only mode: a log carries corpus and
// provider detail, so it is not world-readable.
func TestFileTeeMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kbase.log")
	_, closer, err := New(Options{FilePath: path, Console: io.Discard})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	closeOrFail(t, closer)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	if got := info.Mode().Perm(); got != logFileMode {
		t.Errorf("log file mode = %o, want %o", got, logFileMode)
	}
}

// TestUnopenableFile: a --log-file that cannot be opened fails the
// invocation, rather than degrading to console-only without saying so.
func TestUnopenableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "kbase.log")
	if _, _, err := New(Options{FilePath: path, Console: io.Discard}); err == nil {
		t.Fatal("New with an unopenable log file = nil error, want an error")
	}
}

// TestNoFileClosesClean: the closer is non-nil even with no file, so the
// caller's deferred Close needs no guard.
func TestNoFileClosesClean(t *testing.T) {
	_, closer, err := New(Options{Console: io.Discard})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if closer == nil {
		t.Fatal("New returned a nil closer")
	}
	closeOrFail(t, closer)
}

// TestDiscard is a smoke test: Discard's whole contract is that every level
// is callable and writes nowhere, so the only failure it can catch is a
// panic from a Logger nobody gave a destination.
func TestDiscard(t *testing.T) {
	lg := Discard()
	lg.Debug("d")
	lg.Info("i", "k", "v")
	lg.Warn("w")
	lg.Error("e")
}

// messages extracts the msg= value of every record in a text-handler
// transcript, in order. Asserting on messages rather than whole lines keeps
// these tests off the timestamp and off slog's exact spacing.
func messages(transcript string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(transcript), "\n") {
		_, rest, ok := strings.Cut(line, "msg=")
		if !ok {
			continue
		}
		// The text handler quotes a value only when it has to, so the
		// field ends at the closing quote or, unquoted, at the next space.
		delim := " "
		if quoted, ok := strings.CutPrefix(rest, `"`); ok {
			rest, delim = quoted, `"`
		}
		field, _, _ := strings.Cut(rest, delim)
		out = append(out, field)
	}
	return out
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func closeOrFail(t *testing.T, c io.Closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
