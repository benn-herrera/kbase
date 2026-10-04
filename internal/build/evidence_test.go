package build

import (
	"flag"
	"os"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/result"
)

var (
	statusDocument = flag.String("build.status", "", "a status result document kbase wrote")
	statusWant     = flag.String("build.state", "", "the state that document must report")
)

// TestStatusDocument parses a status document ./bin/kbase wrote and checks it
// against the state the build was left in; test-integration-build-arxiv
// supplies both.
func TestStatusDocument(t *testing.T) {
	if *statusDocument == "" {
		t.Skip("no status document named; test-integration-build-arxiv supplies one")
	}
	b, err := os.ReadFile(*statusDocument)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Outcome     string  `yaml:"outcome"`
		State       string  `yaml:"state"`
		PID         *int    `yaml:"pid"`
		Started     *string `yaml:"started"`
		NoInference bool    `yaml:"no-inference"`
		Stages      []struct {
			Stage  string  `yaml:"stage"`
			Commit *string `yaml:"commit"`
		} `yaml:"stages"`
		Current *struct {
			Stage      string `yaml:"stage"`
			UnitsDone  int    `yaml:"units-done"`
			UnitsTotal int    `yaml:"units-total"`
		} `yaml:"current"`
		Resume *string `yaml:"resume"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("%s does not parse: %v", *statusDocument, err)
	}
	if doc.Outcome != result.Done || doc.State != *statusWant {
		t.Errorf("outcome %q, state %q; want done, %s", doc.Outcome, doc.State, *statusWant)
	}
	if doc.PID == nil || doc.Started == nil || !doc.NoInference {
		t.Errorf("pid %v, started %v, no-inference %t: want the build's pid and start, and the flag it ran under", doc.PID, doc.Started, doc.NoInference)
	}
	if len(doc.Stages) != len(Stages) {
		t.Fatalf("%d stages listed, want %d", len(doc.Stages), len(Stages))
	}
	unrecorded := ""
	for i, s := range doc.Stages {
		if s.Stage != Stages[i].ID {
			t.Errorf("stage %d = %+v, want %s", i, s, Stages[i].ID)
		}
		if s.Commit == nil && unrecorded == "" {
			unrecorded = s.Stage
		}
	}
	switch {
	case unrecorded == "" && (doc.Current != nil || doc.Resume != nil):
		t.Errorf("a finished build reports current %+v and resume %v", doc.Current, doc.Resume)
	case unrecorded != "" && (doc.Current == nil || doc.Current.Stage != unrecorded || doc.Resume == nil):
		t.Errorf("current %+v and resume %v, want %s and a resume command", doc.Current, doc.Resume, unrecorded)
	}
}
