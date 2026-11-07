package build

import (
	"flag"
	"os"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/buildstore"
	"kbase/internal/result"
)

var (
	statusResult = flag.String("build.status", "", "a status result kbase wrote")
	statusWant   = flag.String("build.state", "", "the state that result must report")
)

// TestStatusResult parses a status result ./bin/kbase wrote and checks it
// against the state the build was left in; test-integration-build-arxiv
// supplies both.
func TestStatusResult(t *testing.T) {
	if *statusResult == "" {
		t.Skip("no status result named; test-integration-build-arxiv supplies one")
	}
	b, err := os.ReadFile(*statusResult)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Outcome     string  `yaml:"outcome"`
		State       string  `yaml:"state"`
		PID         *int    `yaml:"pid"`
		Started     *string `yaml:"started"`
		NoInference bool    `yaml:"no-inference"`
		Stages      struct {
			Done    []string `yaml:"done"`
			NotDone []string `yaml:"not-done"`
			Rewind  string   `yaml:"rewind"`
		} `yaml:"stages"`
		Current *struct {
			Stage string `yaml:"stage"`
			Done  int    `yaml:"done"`
			Total int    `yaml:"total"`
		} `yaml:"current"`
		Resume *string `yaml:"resume"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("%s does not parse: %v", *statusResult, err)
	}
	if doc.Outcome != result.Done || doc.State != *statusWant {
		t.Errorf("outcome %q, state %q; want done, %s", doc.Outcome, doc.State, *statusWant)
	}
	if doc.PID == nil || doc.Started == nil || !doc.NoInference {
		t.Errorf("pid %v, started %v, no-inference %t: want the build's pid and start, and the flag it ran under", doc.PID, doc.Started, doc.NoInference)
	}
	listed := append(append([]string{}, doc.Stages.Done...), doc.Stages.NotDone...)
	if len(listed) != len(buildstore.Stages) || doc.Stages.Rewind == "" {
		t.Fatalf("stages %+v, want each of the %d stages done or not done, and the line naming where their commits are", doc.Stages, len(buildstore.Stages))
	}
	for i, id := range listed {
		if id != buildstore.Stages[i].ID {
			t.Errorf("stage %d = %s, want %s: the stages done, then those not done, in walk order", i, id, buildstore.Stages[i].ID)
		}
	}
	unrecorded := ""
	if len(doc.Stages.NotDone) > 0 {
		unrecorded = doc.Stages.NotDone[0]
	}
	switch {
	case unrecorded == "" && (doc.Current != nil || doc.Resume != nil):
		t.Errorf("a finished build reports current %+v and resume %v", doc.Current, doc.Resume)
	case unrecorded != "" && (doc.Current == nil || doc.Current.Stage != unrecorded || doc.Resume == nil):
		t.Errorf("current %+v and resume %v, want %s and a resume command", doc.Current, doc.Resume, unrecorded)
	}
	if doc.Current != nil && (doc.Current.Total <= 0 || doc.Current.Done < 0 || doc.Current.Done > doc.Current.Total) {
		t.Errorf("current %+v, want a positive total and done between 0 and it", doc.Current)
	}
}
