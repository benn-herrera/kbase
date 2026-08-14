package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kbase/internal/pipeline"
	"kbase/internal/survey"
)

// surveyCorpus is the fixture every survey-verb test runs against: two files,
// three sections, one resolvable link and one that is not — small enough that
// every number in the summary line is checkable by hand.
var surveyCorpus = map[string]string{
	"index.md": `# Index

Welcome to the corpus.

## Topics

- [guide](guide.md)
- [missing](nope.md)
`,
	"guide.md": `# Guide

Guide body.
`,
}

// surveyFixtureDir materializes a corpus in a temporary directory.
func surveyFixtureDir(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

// runSurveyVerb invokes the verb with buffered streams and returns both.
func runSurveyVerb(t *testing.T, opts surveyOptions) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	opts.Stdout, opts.Stderr = &out, &errb
	err = runSurvey(opts)
	return out.String(), errb.String(), err
}

// TestRunSurveySummary is the happy path: the counts a human reads land on
// stdout, and nothing else does.
func TestRunSurveySummary(t *testing.T) {
	root := surveyFixtureDir(t, surveyCorpus)
	stdout, stderr, err := runSurveyVerb(t, surveyOptions{Root: root})
	if err != nil {
		t.Fatalf("runSurvey: %v", err)
	}
	for _, want := range []string{"files=2", "sections=3", "top-level=2", "internal=1", "unresolved=1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout %q missing %q", stdout, want)
		}
	}
	if stderr != "" {
		t.Errorf("stderr: got %q, want empty on success", stderr)
	}
}

// TestRunSurveyJSONFile: --json writes the artifact where it was told to, and
// the summary still goes to stdout.
func TestRunSurveyJSONFile(t *testing.T) {
	mask := setTestUmask(t)
	root := surveyFixtureDir(t, surveyCorpus)
	out := filepath.Join(t.TempDir(), "survey.json")

	stdout, _, err := runSurveyVerb(t, surveyOptions{Root: root, JSONPath: out})
	if err != nil {
		t.Fatalf("runSurvey: %v", err)
	}
	if !strings.Contains(stdout, "survey ok:") {
		t.Errorf("stdout %q should still carry the summary", stdout)
	}

	// The artifact is created the way every kbase output is: permissive mode
	// asked for, the user's umask deciding (ruled 2026-08-14).
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	want := os.FileMode(pipeline.CreateFileMode) &^ mask
	if runtime.GOOS != "windows" && info.Mode().Perm() != want {
		t.Errorf("artifact mode = %v, want %v", info.Mode().Perm(), want)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var artifact survey.Artifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}
	if artifact.Schema != survey.SchemaVersion || len(artifact.Files) != 2 {
		t.Errorf("artifact = %+v, want the schema version and both files", artifact.Corpus)
	}
	if strings.Contains(string(raw), root) {
		t.Error("the artifact must not carry the machine's corpus path")
	}
}

// TestRunSurveyJSONStdout: with "-" the artifact owns stdout, so the summary
// moves aside and a caller can pipe the output.
func TestRunSurveyJSONStdout(t *testing.T) {
	root := surveyFixtureDir(t, surveyCorpus)

	stdout, stderr, err := runSurveyVerb(t, surveyOptions{Root: root, JSONPath: jsonToStdout})
	if err != nil {
		t.Fatalf("runSurvey: %v", err)
	}
	var artifact survey.Artifact
	if err := json.Unmarshal([]byte(stdout), &artifact); err != nil {
		t.Fatalf("stdout is not the bare artifact: %v", err)
	}
	if !strings.Contains(stderr, "survey ok:") {
		t.Errorf("stderr %q should carry the summary", stderr)
	}
}

// TestRunSurveyFailures: every failure is loud, and none of them prints a
// summary that suggests the run succeeded.
func TestRunSurveyFailures(t *testing.T) {
	root := surveyFixtureDir(t, surveyCorpus)

	for _, tc := range []struct {
		name string
		opts surveyOptions
		want string
	}{
		{"no corpus given", surveyOptions{}, "corpus directory is required"},
		{"corpus does not exist", surveyOptions{Root: filepath.Join(root, "nope")}, "corpus root"},
		{"corpus holds no markdown", surveyOptions{Root: t.TempDir()}, "no .md files"},
		{"artifact path unwritable", surveyOptions{Root: root, JSONPath: filepath.Join(root, "no", "such", "dir", "a.json")}, "create"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, err := runSurveyVerb(t, tc.opts)
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
			if strings.Contains(stdout, "survey ok:") {
				t.Errorf("a failed run must not report success; got %q", stdout)
			}
		})
	}
}
