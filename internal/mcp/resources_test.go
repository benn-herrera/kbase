package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/buildstore"
	"kbase/internal/kb"
	"kbase/internal/mcp/protocol"
	"kbase/internal/query"
	"kbase/internal/result"
	"kbase/internal/score"
)

// stubResources answers List with what a test supplies and reads nothing.
type stubResources struct {
	listed []protocol.Resource
	err    error
}

func (s stubResources) List() ([]protocol.Resource, error) { return s.listed, s.err }

func (s stubResources) Read(protocol.Resource) (string, error) { return "", nil }

// TestResourceFamiliesListSkipsOnlyRefusals: a family refusing what it lists
// from contributes no entries and leaves the rest listing, while any other
// error is the list's and stops it.
func TestResourceFamiliesListSkipsOnlyRefusals(t *testing.T) {
	hints := []protocol.Resource{{URI: ModelHintsURI}}
	refusing := stubResources{err: result.Refusal{{Check: kb.CheckKBRoot, Path: "kb-root"}}}
	families := resourceFamilies{{Resources: refusing}, {Resources: stubResources{listed: hints}}}
	if listed, err := families.List(); err != nil || !slices.Equal(listed, hints) {
		t.Errorf("a refusing family: listed %v, %v; want %v", listed, err, hints)
	}
	failing := stubResources{err: errors.New("the KB's index is unreadable")}
	families = resourceFamilies{{Resources: stubResources{listed: hints}}, {Resources: failing}}
	if listed, err := families.List(); !errors.Is(err, failing.err) || listed != nil {
		t.Errorf("a failing family: listed %v, %v; want its error and no list", listed, err)
	}
}

// TestBuildResourceReadsTheNewestScoreRun: the build resource's document is the
// status tool's, so over a store whose newest run is a score run — one that
// started after the store's last build invocation — it names that run.
func TestBuildResourceReadsTheNewestScoreRun(t *testing.T) {
	const buildStart, scoreStart = "2026-10-01T10:00:00Z", "2026-10-02T10:00:00Z"
	repo, state := t.TempDir(), t.TempDir()
	for _, d := range []string{".git/objects", ".git/refs/heads", kb.KBDir} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var progress []byte
	for _, e := range []buildstore.Event{
		{Time: buildStart, Event: buildstore.EventRun, PID: 1},
		{Time: buildStart, Event: buildstore.EventEnd, Outcome: result.Done},
	} {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		progress = append(append(progress, line...), '\n')
	}
	if err := os.WriteFile(filepath.Join(state, buildstore.ProgressFile), progress, 0o644); err != nil {
		t.Fatal(err)
	}
	const run = "run-newer"
	l, err := score.Create(state, score.RunHeader{Run: run, KBRoot: filepath.Join(repo, kb.KBDir), Pass: query.PassRigor, Started: scoreStart})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := score.WriteResultDocument(l.Dir(), result.Done, nil); err != nil {
		t.Fatal(err)
	}

	text, err := statusDocument(buildstore.MonitorOptions{StateDir: state, WorkDir: repo})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatalf("decoding the resource's document: %v\n%s", err, text)
	}
	if doc["run"] != run {
		t.Errorf("the resource's document names run %v, want the newest run %s\n%s", doc["run"], run, text)
	}
}
