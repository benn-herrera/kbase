package claimgraph_test

// The claim-graph comparison: kbase's kb-root/ and build records against
// kb_tools' for the same paper at its depends-attributed commit, and against a
// second kbase build of the same inputs, each modulo node ids — ids are minted
// at random on every build. The recipes test-integration-claimgraph-arxiv and
// test-integration-build-arxiv supply the flags over artifacts they guarantee,
// and the verifiers' exit codes beside them; without the flags the test skips.

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/claimgraph"
	"kbase/internal/kb"
)

var (
	cmpKbase   = flag.String("claimgraph.kbase", "", "the repository kbase built: kb-root/ and the YAML build records")
	cmpRerun   = flag.String("claimgraph.rerun", "", "a second repository kbase built from the same inputs, compared modulo node ids")
	cmpKbtools = flag.String("claimgraph.kbtools", "", "kb_tools' tree at its depends-attributed commit: kb-root/ and the JSON build records")
	cmpChecks  = flag.String("claimgraph.checks", "", "the directory holding each verifier's .exit beside its .out and .log")
	cmpOut     = flag.String("claimgraph.comparison", "", "where comparison.yaml is written")
)

// kbtoolsRecords are kb_tools' JSON spellings of the two build records.
const (
	kbtoolsNodePass       = "kb-build-node-pass.json"
	kbtoolsClassification = "kb-build-classification.json"
)

// runs are the runs the recipe records, each of which must exit 0; kb_tools'
// verify may instead be red on the sheet alone, recorded as an .exception
// beside it.
var runs = []string{"kbase-build", "kbase-build-2", "kbase-refresh", "kbase-verify", "kbtools-verify"}

type check struct {
	Check       string   `yaml:"check"`
	Passed      bool     `yaml:"passed"`
	Detail      string   `yaml:"detail,omitempty"`
	Differences []string `yaml:"differences,omitempty"`
}

type comparison struct {
	Kbase   string  `yaml:"kbase"`
	Kbtools string  `yaml:"kbtools,omitempty"`
	Rerun   string  `yaml:"rerun,omitempty"`
	Passed  bool    `yaml:"passed"`
	Checks  []check `yaml:"checks"`
}

func newCheck(name string, differences []string) check {
	return check{Check: name, Passed: len(differences) == 0, Differences: differences}
}

// moduloIDs is every check of a and b compared modulo node ids, each name
// prefixed by prefix.
func moduloIDs(t *testing.T, prefix string, a, b claimgraph.ComparedKB) []check {
	t.Helper()
	compared, err := claimgraph.CompareModuloIDs(a, b)
	if err != nil {
		t.Fatal(err)
	}
	var out []check
	for _, c := range compared {
		ch := newCheck(prefix+c.Name, c.Differences)
		ch.Detail = fmt.Sprintf("%s %d, %s %d", a.Name, c.Counts[0], b.Name, c.Counts[1])
		out = append(out, ch)
	}
	return out
}

// runChecks reads the recipe's run exits: each must be 0, but for kb_tools'
// verify, red on the sheet alone.
func runChecks(dir string) []check {
	var out []check
	for _, v := range runs {
		data, err := os.ReadFile(filepath.Join(dir, v+".exit"))
		if err != nil {
			out = append(out, newCheck(v, []string{err.Error()}))
			continue
		}
		code := strings.TrimSpace(string(data))
		c := newCheck(v, nil)
		c.Detail = "exit " + code
		if code != "0" {
			if _, err := os.Stat(filepath.Join(dir, v+".exception")); err == nil {
				c.Detail += "; red on the claim-graph sheet alone, kbase's placeholder (SPEC §3)"
			} else {
				c = newCheck(v, []string{"exit " + code + " — see " + filepath.Join(dir, v+".out") + " and .log"})
			}
		}
		out = append(out, c)
	}
	return out
}

func TestCompareKbTools(t *testing.T) {
	if *cmpKbase == "" || *cmpOut == "" || (*cmpKbtools == "" && *cmpRerun == "") {
		t.Skip("no claim-graph comparison named; test-integration-claimgraph-arxiv and test-integration-build-arxiv supply it")
	}
	c := comparison{Kbase: *cmpKbase, Kbtools: *cmpKbtools, Rerun: *cmpRerun}
	kbase := claimgraph.BuiltBy("kbase", *cmpKbase)
	if *cmpKbtools != "" {
		kbtools := claimgraph.ComparedKB{Name: "kb_tools", KBRoot: filepath.Join(*cmpKbtools, kb.KBDir),
			NodePass: filepath.Join(*cmpKbtools, kbtoolsNodePass), Classification: filepath.Join(*cmpKbtools, kbtoolsClassification)}
		c.Checks = append(c.Checks, moduloIDs(t, "", kbase, kbtools)...)
	}
	if *cmpRerun != "" {
		c.Checks = append(c.Checks, moduloIDs(t, "determinism modulo ids: ", kbase, claimgraph.BuiltBy("rerun", *cmpRerun))...)
	}
	if *cmpChecks != "" {
		c.Checks = append(c.Checks, runChecks(*cmpChecks)...)
	}
	c.Passed = !slices.ContainsFunc(c.Checks, func(ch check) bool { return !ch.Passed })

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(*cmpOut, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, ch := range c.Checks {
		if !ch.Passed {
			t.Errorf("%s: %d difference(s) — see %s", ch.Check, len(ch.Differences), *cmpOut)
		}
	}
}
