package claimgraph_test

// The claim-graph comparison: kbase's kb-root/ and build records against the
// 0.9.0 fixture's for the same paper at its depends-attributed commit, and
// against a second kbase build of the same inputs, each modulo node ids — ids
// are minted at random on every build — with the two ledgers to that commit,
// kbase status's stages, and depends-attributed's candidate counts. The
// recipes test-integration-claimgraph-arxiv and test-integration-build-arxiv
// supply the flags over artifacts they guarantee, and the verifiers' exit codes
// beside them; without the flags the test skips.

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/claimgraph"
)

var (
	cmpKbase     = flag.String("claimgraph.kbase", "", "the repository kbase built: kb-root/ and the build records")
	cmpRerun     = flag.String("claimgraph.rerun", "", "a second repository kbase built from the same inputs, compared modulo node ids")
	cmpReference = flag.String("claimgraph.reference", "", "the 0.9.0 fixture's tree at its depends-attributed commit: kb-root/, the JSON build records, "+
		"and beside them the ledger to that commit (ledger.txt), the stage vocabulary (stage-ids.txt) and the depends-attributed report (depends-report.txt)")
	cmpChecks = flag.String("claimgraph.checks", "", "the directory holding each verifier's .exit beside its .out and .log")
	cmpOut    = flag.String("claimgraph.comparison", "", "where comparison.yaml is written")
	cmpLedger = flag.String("claimgraph.ledger", "", "kbase's ledger, spelled as the fixture's ledger.txt is")
	cmpStatus = flag.String("claimgraph.status", "", "kbase status's result document over the kbase build")
	cmpState  = flag.String("claimgraph.state", "", "the kbase build's state store, holding its stage reports")
)

// The files the recipe writes beside the fixture's tree and build records, off
// the 0.9.0 build.
const (
	referenceLedger        = "ledger.txt"
	referenceStageIDs      = "stage-ids.txt"
	referenceDependsReport = "depends-report.txt"
)

// runs are the runs the recipe records, every one of which must exit 0.
var runs = []string{"kbase-build", "kbase-build-2", "kbase-refresh", "kbase-verify", "kbtools-verify"}

type check struct {
	Check       string   `yaml:"check"`
	Passed      bool     `yaml:"passed"`
	Detail      string   `yaml:"detail,omitempty"`
	Differences []string `yaml:"differences,omitempty"`
}

type comparison struct {
	Kbase     string  `yaml:"kbase"`
	Reference string  `yaml:"reference,omitempty"`
	Rerun     string  `yaml:"rerun,omitempty"`
	Passed    bool    `yaml:"passed"`
	Checks    []check `yaml:"checks"`
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

// runChecks reads the recipe's run exits: each must be 0.
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
			c = newCheck(v, []string{"exit " + code + " — see " + filepath.Join(dir, v+".out") + " and .log"})
		}
		out = append(out, c)
	}
	return out
}

// ledgerEntries is a ledger file's boundary commits, oldest first: each one's
// subject and body, NUL-separated as git log -z writes them.
func ledgerEntries(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range strings.Split(string(b), "\x00") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out, nil
}

// ledgerStages is each entry's stage id, off its subject.
func ledgerStages(entries []string) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		subject, _, _ := strings.Cut(e, "\n")
		out[i], _, _ = strings.Cut(strings.TrimPrefix(subject, "kb-build: "), " | ")
	}
	return out
}

// withoutInputs is a ledger entry less the build's input lines it ends with,
// which kbase's bodies carry and 0.9.0's do not; the lines are the entry's
// last, whether a paragraph of their own or the whole body.
func withoutInputs(entry string) string {
	lines := strings.Split(entry, "\n")
	end := len(lines)
	for end > 0 && (strings.HasPrefix(lines[end-1], "volume-root: ") || strings.HasPrefix(lines[end-1], "bibliography: ")) {
		end--
	}
	return strings.TrimRight(strings.Join(lines[:end], "\n"), "\n")
}

func TestWithoutInputs(t *testing.T) {
	const subject = "kb-build: gate | Gate"
	for _, tc := range []struct{ name, entry, want string }{
		{"inputs alone", subject + "\n\nvolume-root: a\nbibliography: b.bib", subject},
		{"inputs alone, one newline", subject + "\nvolume-root: a\nbibliography: b.bib", subject},
		{"inputs after a note", subject + "\n\nDropped rows.\n\nvolume-root: a", subject + "\n\nDropped rows."},
		{"no body", subject, subject},
		{"note only", subject + "\n\nDropped rows.", subject + "\n\nDropped rows."},
		{"mixed paragraph kept", subject + "\n\nvolume-root: a\nother", subject + "\n\nvolume-root: a\nother"},
		{"inputs mid-body kept", subject + "\n\nvolume-root: a\n\nDropped rows.", subject + "\n\nvolume-root: a\n\nDropped rows."},
	} {
		if got := withoutInputs(tc.entry); got != tc.want {
			t.Errorf("%s: withoutInputs = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// kbaseOnlyStages are the stage ids kbase walks and 0.9.0 never had; the
// comparisons against the 0.9.0 build set them aside.
var kbaseOnlyStages = []string{"claims-culled"}

// ledgerCheck compares the two ledgers boundary by boundary: the stages
// recorded, in order, and each one's body, which names the rows the stage
// dropped; kbase's kbase-only boundaries are set aside, and both ledgers'
// trailing build inputs.
func ledgerCheck(kbasePath, referencePath string) check {
	entries, err := ledgerEntries(kbasePath)
	if err != nil {
		return newCheck("ledger", []string{err.Error()})
	}
	var a []string
	for i, stage := range ledgerStages(entries) {
		if !slices.Contains(kbaseOnlyStages, stage) {
			a = append(a, withoutInputs(entries[i]))
		}
	}
	b, err := ledgerEntries(referencePath)
	if err != nil {
		return newCheck("ledger", []string{err.Error()})
	}
	for i := range b {
		b[i] = withoutInputs(b[i])
	}
	var diffs []string
	for i := range max(len(a), len(b)) {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			diffs = append(diffs, fmt.Sprintf("boundary %d: kbase %q, 0.9.0 %q", i+1, x, y))
		}
	}
	c := newCheck("ledger: stages recorded and the rows each dropped", diffs)
	c.Detail = fmt.Sprintf("kbase %d boundaries (%d kbase-only set aside), 0.9.0 %d", len(a), len(entries)-len(a), len(b))
	return c
}

// statusCheck reads kbase status's stages against the 0.9.0 build's: the
// vocabulary in order, and the stages recorded as its ledger records them;
// kbase's kbase-only stages are set aside.
func statusCheck(statusPath, stageIDsPath, ledgerPath string) check {
	var doc struct {
		Stages []struct {
			Stage  string  `yaml:"stage"`
			Commit *string `yaml:"commit"`
		} `yaml:"stages"`
	}
	b, err := os.ReadFile(statusPath)
	if err == nil {
		err = yaml.Unmarshal(b, &doc)
	}
	if err != nil {
		return newCheck("status stages", []string{err.Error()})
	}
	ids, err := os.ReadFile(stageIDsPath)
	if err != nil {
		return newCheck("status stages", []string{err.Error()})
	}
	entries, err := ledgerEntries(ledgerPath)
	if err != nil {
		return newCheck("status stages", []string{err.Error()})
	}
	var listed, recorded []string
	setAside := 0
	for _, s := range doc.Stages {
		if slices.Contains(kbaseOnlyStages, s.Stage) {
			setAside++
			continue
		}
		listed = append(listed, s.Stage)
		if s.Commit != nil {
			recorded = append(recorded, s.Stage)
		}
	}
	var diffs []string
	if want := strings.Fields(string(ids)); !slices.Equal(listed, want) {
		diffs = append(diffs, fmt.Sprintf("status lists %q; 0.9.0's stage vocabulary is %q", listed, want))
	}
	if want := ledgerStages(entries); !slices.Equal(recorded, want) {
		diffs = append(diffs, fmt.Sprintf("status records %q; 0.9.0's ledger records %q", recorded, want))
	}
	c := newCheck("status stages against 0.9.0's vocabulary and ledger", diffs)
	c.Detail = fmt.Sprintf("%d stages listed, %d recorded (%d kbase-only set aside)", len(listed), len(recorded), setAside)
	return c
}

// referenceCandidatesRE is the 0.9.0 build's stage-D-candidates report line.
var referenceCandidatesRE = regexp.MustCompile(`(?m)^\[claimgraph\] FACT stage-D-candidates (\d+) candidates over (\d+) source claims; by harvest: (.*); by letters offered: .*; (\d+) pairs dropped as a claim's own equation$`)

// candidateCountsCheck compares depends-attributed's candidate counts — how
// many, over how many sources, by harvest, and the pairs dropped as a claim's
// own equation — in kbase's stage report and the 0.9.0 build's report.
func candidateCountsCheck(stateDir, referenceReport string) check {
	name := "depends-attributed's candidate counts"
	report, err := os.ReadFile(referenceReport)
	if err != nil {
		return newCheck(name, []string{err.Error()})
	}
	m := referenceCandidatesRE.FindStringSubmatch(string(report))
	if m == nil {
		return newCheck(name, []string{referenceReport + " holds no stage-D-candidates line"})
	}
	want := fmt.Sprintf("candidates %s, sources %s, by harvest %s, own equations dropped %s", m[1], m[2], m[3], m[4])

	b, err := os.ReadFile(filepath.Join(stateDir, "reports", "depends-attributed.yaml"))
	if err != nil {
		return newCheck(name, []string{err.Error()})
	}
	var doc struct {
		Rows []struct {
			Claimgraph []map[string]any `yaml:"claimgraph"`
		} `yaml:"rows"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return newCheck(name, []string{err.Error()})
	}
	got := "no stage-D-candidates finding"
	for _, r := range doc.Rows {
		for _, f := range r.Claimgraph {
			if f["check"] != "stage-D-candidates" {
				continue
			}
			byHarvest, _ := f["by-harvest"].(map[string]any)
			var names []string
			for n := range byHarvest {
				names = append(names, n)
			}
			slices.Sort(names)
			var parts []string
			for _, n := range names {
				parts = append(parts, fmt.Sprintf("%s %v", n, byHarvest[n]))
			}
			harvest := strings.Join(parts, ", ")
			if harvest == "" {
				harvest = "none"
			}
			got = fmt.Sprintf("candidates %v, sources %v, by harvest %s, own equations dropped %v", f["candidates"], f["sources"], harvest, f["own-equations-dropped"])
		}
	}
	var diffs []string
	if got != want {
		diffs = []string{fmt.Sprintf("kbase %s; 0.9.0 %s", got, want)}
	}
	c := newCheck(name, diffs)
	c.Detail = got
	return c
}

func TestCompareReference(t *testing.T) {
	if *cmpKbase == "" || *cmpOut == "" || (*cmpReference == "" && *cmpRerun == "") {
		t.Skip("no claim-graph comparison named; test-integration-claimgraph-arxiv and test-integration-build-arxiv supply it")
	}
	c := comparison{Kbase: *cmpKbase, Reference: *cmpReference, Rerun: *cmpRerun}
	kbase := claimgraph.BuiltBy("kbase", *cmpKbase)
	if *cmpReference != "" {
		c.Checks = append(c.Checks, moduloIDs(t, "", kbase, claimgraph.BuiltBy("0.9.0", *cmpReference))...)
		if *cmpLedger != "" {
			c.Checks = append(c.Checks, ledgerCheck(*cmpLedger, filepath.Join(*cmpReference, referenceLedger)))
		}
		if *cmpStatus != "" {
			c.Checks = append(c.Checks, statusCheck(*cmpStatus, filepath.Join(*cmpReference, referenceStageIDs), filepath.Join(*cmpReference, referenceLedger)))
		}
		if *cmpState != "" {
			c.Checks = append(c.Checks, candidateCountsCheck(*cmpState, filepath.Join(*cmpReference, referenceDependsReport)))
		}
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
