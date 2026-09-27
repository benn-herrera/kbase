package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/assemble"
	"kbase/internal/pipeline"
	"kbase/internal/version"
)

// These are UNIT tests of the verb's logic: they call runWriteAgents in
// process, with no binary and no network. The test that drives the same verb
// through the front door is the `just test-integration-write-agents` recipe,
// which runs ./bin/kbase and nothing else.

// guardBytes stands in for a file the user wrote and does not want back. Its
// survival is what "writes nothing" means on a refused run.
const guardBytes = "MINE — do not overwrite\n"

func sampleNames(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, s := range assemble.GenericAgents() {
		names = append(names, s.Path)
	}
	if len(names) == 0 {
		t.Fatal("there are no generic agent definitions to write")
	}
	return names
}

func TestWriteAgentsWritesEveryDefinition(t *testing.T) {
	// A path two levels below anything that exists: --out is created if
	// missing, parents included.
	out := filepath.Join(t.TempDir(), "tooling", "agents")
	var stderr bytes.Buffer
	if err := runWriteAgents(writeAgentsOptions{Out: out, Stderr: &stderr}); err != nil {
		t.Fatalf("writing into a fresh directory: %v", err)
	}
	names := sampleNames(t)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Errorf("%s was not written: %v", name, err)
			continue
		}
		text := string(data)
		if !strings.Contains(text, version.Current) {
			t.Errorf("%s does not name the kbase version that wrote it:\n%s", name, text)
		}
		// The two claims a sample must not make: that it came out of a corpus
		// build, and that kbase will overwrite it. Neither is true — the verb
		// refuses an existing file rather than rewriting one.
		if strings.Contains(text, "corpus sha256:") {
			t.Errorf("%s carries a corpus provenance receipt; no corpus was read:\n%s", name, text)
		}
		if strings.Contains(text, "overwritten") {
			t.Errorf("%s claims kbase overwrites it, which it never does:\n%s", name, text)
		}
	}
	if got := stderr.String(); !strings.Contains(got, out) {
		t.Errorf("the summary does not name where the definitions went: %q", got)
	}
}

// The refusal is all-or-nothing: one existing target refuses the whole
// command, every conflict is named rather than only the first, and no target
// that was absent gets written.
func TestWriteAgentsRefusesExistingFiles(t *testing.T) {
	names := sampleNames(t)
	tests := []struct {
		name     string
		existing []string
	}{
		{"one target already there", names[:1]},
		{"some targets already there", names[:len(names)-1]},
		{"every target already there", names},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := t.TempDir()
			for _, name := range tc.existing {
				if err := os.WriteFile(filepath.Join(out, name), []byte(guardBytes), pipeline.CreateFileMode); err != nil {
					t.Fatalf("seeding %s: %v", name, err)
				}
			}
			var stderr bytes.Buffer
			err := runWriteAgents(writeAgentsOptions{Out: out, Stderr: &stderr})
			if err == nil {
				t.Fatal("the verb wrote over files that were already there")
			}
			t.Logf("refusal with %d of %d targets present:\n%v", len(tc.existing), len(names), err)

			for _, name := range tc.existing {
				path := filepath.Join(out, name)
				if !strings.Contains(err.Error(), path) {
					t.Errorf("the refusal does not name the conflict %s:\n%v", path, err)
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil || string(data) != guardBytes {
					t.Errorf("%s was modified by a refused run (%v)", path, readErr)
				}
			}
			for _, name := range names {
				if slices.Contains(tc.existing, name) {
					continue
				}
				if _, statErr := os.Stat(filepath.Join(out, name)); !errors.Is(statErr, fs.ErrNotExist) {
					t.Errorf("%s was written by a run that refused; the refusal writes nothing", name)
				}
			}
			if stderr.Len() != 0 {
				t.Errorf("a refused run reported to stderr: %q", stderr.String())
			}
		})
	}
}

func TestWriteAgentsRefusesNonDirectoryOut(t *testing.T) {
	out := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(out, []byte(guardBytes), pipeline.CreateFileMode); err != nil {
		t.Fatalf("seeding %s: %v", out, err)
	}
	err := runWriteAgents(writeAgentsOptions{Out: out, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("--out naming a regular file was accepted")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("the refusal does not say what is wrong with --out: %v", err)
	}
	data, readErr := os.ReadFile(out)
	if readErr != nil || string(data) != guardBytes {
		t.Errorf("the file at --out was modified (%v)", readErr)
	}
}

func TestWriteAgentsRequiresOut(t *testing.T) {
	err := runWriteAgents(writeAgentsOptions{Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("the verb ran with no --out; it has no default target")
	}
	if !strings.Contains(err.Error(), "--out is required") {
		t.Errorf("the refusal does not name the missing flag: %v", err)
	}
}
