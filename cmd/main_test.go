package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kbase/internal/atomicfile"
)

// executeRoot runs the REAL root command — flag parsing, PersistentPreRunE,
// and all — with output captured. Nothing else exercises the composition
// root's logging path: the logger is built inside Execute, from flags cobra
// has just parsed, so a test that skips Execute tests something else.
//
// The package-level state the run mutates (the flag vars and the process
// logger) is restored afterwards, so test order cannot leak a log file or a
// level into the next test.
func executeRoot(t *testing.T, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()

	savedDir, savedLevel, savedFile, savedLog := flagConfigDir, flagLogLevel, flagLogFile, processLog
	t.Cleanup(func() {
		flagConfigDir, flagLogLevel, flagLogFile, processLog = savedDir, savedLevel, savedFile, savedLog
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)
	return stdout, stderr, rootCmd.Execute()
}

// TestRootLoggingWiring walks the whole --log-level/--log-file path on a bare
// invocation: the flags reach log.New, the file is created owner-only, a
// record at the requested level lands in BOTH destinations (the flag tees, it
// does not redirect), and the closer main defers over comes back clean.
//
// The record is emitted here rather than waited for, because no verb logs on
// a bare invocation — and the level and the tee are only observable through
// one. Emitting at debug is also what proves --log-level arrived: at the warn
// default the record would be dropped and the file would stay empty.
func TestRootLoggingWiring(t *testing.T) {
	// The umask is set before Execute, which is what creates the log file.
	mask := setTestUmask(t)
	path := filepath.Join(t.TempDir(), "run.log")

	stdout, stderr, err := executeRoot(t, "--log-level", "debug", "--log-file", path)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("a bare invocation should print help on stdout; got %q", stdout)
	}
	if processLog.closer == nil {
		t.Fatal("PersistentPreRunE left no closer; main would never release the log file")
	}

	const msg = "logging wiring probe"
	processLog.logger.Debug(msg, "flag", "--log-file")

	if err := processLog.closer.Close(); err != nil {
		t.Fatalf("closing the log file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("log file: %v", err)
	}
	// The log file is created like every other kbase output: permissive
	// mode asked for, the user's umask deciding. log.logFileMode is
	// unexported and is the same number as atomicfile's.
	want := os.FileMode(atomicfile.CreateMode) &^ mask
	if runtime.GOOS != "windows" && info.Mode().Perm() != want {
		t.Errorf("log file mode: got %v, want %v", info.Mode().Perm(), want)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(body), msg) {
		t.Errorf("debug record did not reach the log file; file holds %q", body)
	}
	if !strings.Contains(stderr.String(), msg) {
		t.Errorf("--log-file must tee, not redirect; console got %q", stderr)
	}
}

// TestRootLoggingFlagFailures: a logger that cannot be built the way it was
// asked for fails the invocation. Downgrading itself silently would suppress
// exactly the diagnostics the flags were reaching for, and leave main with a
// half-open file to close.
func TestRootLoggingFlagFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    func(dir string) []string
		wantErr string
	}{{
		name:    "unknown level",
		args:    func(string) []string { return []string{"--log-level", "verbose"} },
		wantErr: "unknown log level",
	}, {
		name:    "unopenable file",
		args:    func(dir string) []string { return []string{"--log-file", filepath.Join(dir, "absent", "run.log")} },
		wantErr: "open log file",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeRoot(t, tc.args(t.TempDir())...)
			if err == nil {
				t.Fatalf("Execute: got nil, want an error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error: got %v, want substring %q", err, tc.wantErr)
			}
			if processLog.closer != nil {
				t.Error("a failed logger build must leave no closer behind")
			}
		})
	}
}

// TestExitStatus: an error no verb wrote a result for — a usage error among
// them — is refused with its own result document; a verb's own exit code
// passes through with nothing more written.
func TestExitStatus(t *testing.T) {
	_, _, usage := executeRoot(t, "build")
	for _, tc := range []struct {
		name     string
		err      error
		code     int
		document bool
	}{
		{"success", nil, 0, false},
		{"a verb's own exit", exitCode(3), 3, false},
		{"a usage error", usage, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := exitStatus(tc.err, &stdout, &stderr); got != tc.code {
				t.Errorf("exit = %d, want %d", got, tc.code)
			}
			if !tc.document {
				if stdout.Len() != 0 {
					t.Errorf("stdout = %q, want nothing", stdout.String())
				}
				return
			}
			doc := checkDocument(t, "build", stdout.String())
			items := doc.items("refusals")
			if doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != "usage" || items[0]["detail"] != tc.err.Error() {
				t.Errorf("document = %+v, want one usage refusal naming %q", doc.Values, tc.err)
			}
		})
	}
}
