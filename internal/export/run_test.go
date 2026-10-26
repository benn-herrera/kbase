package export

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

// stubPandoc stands in for the two pandoc calls Run takes. preflight returns
// preflightErr; the defaults run returns runErr where it is set, and otherwise
// writes latex as OutputFile and findings as FindingsFile beside the defaults
// file, as pandoc and the filter do. calls is each call's name, in order.
type stubPandoc struct {
	preflightErr, runErr error
	latex, findings      string
	calls                []string
}

func (s *stubPandoc) preflight(context.Context) error {
	s.calls = append(s.calls, "preflight")
	return s.preflightErr
}

func (s *stubPandoc) runDefaults(_ context.Context, defaultsPath string) error {
	s.calls = append(s.calls, "defaults")
	if s.runErr != nil {
		return s.runErr
	}
	if _, err := os.Stat(defaultsPath); err != nil {
		return err
	}
	gen := filepath.Dir(defaultsPath)
	if err := os.WriteFile(filepath.Join(gen, OutputFile), []byte(s.latex), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(gen, FindingsFile), []byte(s.findings), 0o644)
}

func (s *stubPandoc) run(t *testing.T, files map[string]string, out string) (Result, error) {
	t.Helper()
	s.calls = nil
	return Run(context.Background(), writeKB(t, files), out, nil, testVersion, s.preflight, s.runDefaults)
}

// runKB is exportKB with one walk finding: vol1's index names a child that is
// not there.
func runKB() map[string]string {
	files := exportKB()
	files["vol1/index.md"] += "- [Missing](no-such-leaf.md) — a child naming no file\n"
	return files
}

// TestRunExportsAndReissues: an export preflights, writes, runs the defaults
// file once and writes the report — the walk's finding, then the filter's,
// then the preamble's line — reporting every file it wrote, sorted. A
// re-issue whose conversion reproduces pandoc's outputs writes nothing and is
// unchanged; one whose conversion changes main.tex reports main.tex alone.
func TestRunExportsAndReissues(t *testing.T) {
	out := t.TempDir()
	gen := filepath.Join(out, GenTexDir)
	s := &stubPandoc{latex: "\\documentclass{book}\n",
		findings: `[{"form":"link","path":"vol1/v1-leaf.md","reason":"stubbed"}]`}

	res, err := s.run(t, runKB(), out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !slices.Equal(s.calls, []string{"preflight", "defaults"}) {
		t.Errorf("pandoc calls %q, want the preflight then one defaults run", s.calls)
	}
	if !res.Changed || !slices.Equal(res.Volumes, []string{"vol1", "vol2"}) || res.Report != filepath.Join(gen, ReportFile) {
		t.Errorf("Run = %+v, want changed, both volumes and the report's path", res)
	}
	if !slices.IsSorted(res.Written) {
		t.Errorf("written %q is not sorted", res.Written)
	}
	for _, p := range []string{filepath.Join(gen, DefaultsFile), filepath.Join(gen, OutputFile),
		filepath.Join(gen, FindingsFile), filepath.Join(gen, ReportFile), filepath.Join(out, PreambleFile)} {
		if !slices.Contains(res.Written, p) {
			t.Errorf("written lacks %s: %q", p, res.Written)
		}
	}
	data, err := os.ReadFile(filepath.Join(gen, ReportFile))
	if err != nil {
		t.Fatal(err)
	}
	var report []Finding
	if err := yaml.Unmarshal(data, &report); err != nil {
		t.Fatalf("%s: %v\n%s", ReportFile, err, data)
	}
	var lines []string
	for _, f := range report {
		lines = append(lines, f.Form+" "+f.Path)
	}
	if want := []string{walkForm + " vol1/index.md", "link vol1/v1-leaf.md", preambleForm + " " + filepath.Join(out, PreambleFile)}; !slices.Equal(lines, want) {
		t.Errorf("report lines %q, want %q", lines, want)
	}

	res, err = s.run(t, runKB(), out)
	if err != nil {
		t.Fatalf("re-issue: %v", err)
	}
	if res.Changed || len(res.Written) != 0 {
		t.Errorf("re-issue = %+v, want unchanged with nothing written", res)
	}

	s.latex += "% changed\n"
	res, err = s.run(t, runKB(), out)
	if err != nil {
		t.Fatalf("re-issue with a changed conversion: %v", err)
	}
	if !res.Changed || !slices.Equal(res.Written, []string{filepath.Join(gen, OutputFile)}) {
		t.Errorf("re-issue with a changed conversion = %+v, want main.tex alone written", res)
	}
}

// TestRunPandocErrors: a preflight error comes back as it was returned, before
// anything is written or converted; a defaults run's error comes back as it
// was returned, and no report is written.
func TestRunPandocErrors(t *testing.T) {
	refusal := errors.New("pandoc refused")
	tests := []struct {
		name      string
		stub      stubPandoc
		wantCalls []string
	}{
		{"preflight", stubPandoc{preflightErr: refusal}, []string{"preflight"}},
		{"defaults run", stubPandoc{runErr: refusal}, []string{"preflight", "defaults"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := t.TempDir()
			_, err := tc.stub.run(t, runKB(), out)
			if err != refusal {
				t.Errorf("Run = %v, want %v unwrapped", err, refusal)
			}
			if !slices.Equal(tc.stub.calls, tc.wantCalls) {
				t.Errorf("pandoc calls %q, want %q", tc.stub.calls, tc.wantCalls)
			}
			entries, err := os.ReadDir(out)
			if err != nil {
				t.Fatal(err)
			}
			if tc.stub.preflightErr != nil && len(entries) != 0 {
				t.Errorf("a refused preflight left %v in the out directory", entries)
			}
			if _, err := os.Stat(filepath.Join(out, GenTexDir, ReportFile)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the report stands after a failed conversion (%v)", err)
			}
		})
	}
}
