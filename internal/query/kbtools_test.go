package query

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// The query comparison against kb_tools' kb_cmd. test-integration-query-arxiv
// stages each fixture's cases with TestStageQueryCases (and, on a clone, the
// scores of TestStageQueryScores), runs every case through ./bin/kbase and
// kb_cmd --json, and compares them with TestCompareQueries. Without their
// flags all three skip.
var (
	queryKBRoot     = flag.String("query.kbroot", "", "the kb-root cases or scores are chosen from")
	queryCases      = flag.String("query.cases", "", "where the staged cases are written, one NNN.args file each")
	queryValues     = flag.String("query.values", "", "where the scoring values documents are written")
	queryDirs       = flag.String("query.dirs", "", "space-separated case directories, each holding run outputs")
	queryComparison = flag.String("query.comparison", "", "where the comparison is written")
)

// comparedQueries is every query the comparison must see answered non-empty.
var comparedQueries = []string{"deps", "gated-on", "cited-by", "find", "referenced-by", "solidity-below", "subtree", "show", "weak-points", "stats"}

// stageCases is every case over ix: each query over every node id it takes,
// every index node path in both spellings, a find over each distinct first
// title word, and fixed thresholds.
func stageCases(ix *Index) [][]string {
	var cases [][]string
	var nodeIDs []string
	for _, n := range ix.nodes {
		if !slices.Contains(nodeIDs, n.ID) {
			nodeIDs = append(nodeIDs, n.ID)
		}
	}
	for _, id := range nodeIDs {
		cases = append(cases, []string{"deps", id}, []string{"deps", id, "-i"}, []string{"show", id},
			[]string{"gated-on", id}, []string{"cited-by", id}, []string{"referenced-by", id})
	}
	cases = append(cases, []string{"show", "clm-zzzzzz"}, []string{"subtree", ""}, []string{"subtree", "."},
		[]string{"subtree", "nowhere"})
	for _, a := range ix.aggregates {
		cases = append(cases, []string{"subtree", a.NodePath}, []string{"subtree", path.Dir(a.NodePath) + "/"})
	}
	var words []string
	for _, c := range ix.claims() {
		if w := strings.Fields(c.Title); len(w) > 0 && !strings.HasPrefix(w[0], "-") && !slices.Contains(words, w[0]) {
			words = append(words, w[0])
		}
	}
	for i, w := range words {
		cases = append(cases, []string{"find", w})
		if i == 0 {
			cases = append(cases, []string{"find", strings.ToUpper(w)})
		}
	}
	cases = append(cases, []string{"find", ""}, []string{"find", "no-such-claim"},
		[]string{"solidity-below", "0.5"}, []string{"solidity-below", "0.65"}, []string{"solidity-below", "1.1"},
		[]string{"weak-points"}, []string{"weak-points", "--max-solidity", "1.1", "--min-dependents", "0"},
		[]string{"stats"})
	return cases
}

func TestStageQueryCases(t *testing.T) {
	if *queryKBRoot == "" || *queryCases == "" {
		t.Skip("staged by test-integration-query-arxiv")
	}
	ix, err := Load(*queryKBRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(*queryCases, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, c := range stageCases(ix) {
		name := filepath.Join(*queryCases, fmt.Sprintf("%03d.args", i+1))
		if err := os.WriteFile(name, []byte(strings.Join(c, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestStageQueryScores prepares a clone of a kb_tools-built KB, whose claims
// are all pending and whose leaves cross-reference only through HTML anchors,
// for the queries those leave empty. It appends to one leaf a body link to
// another claim's originating leaf, as a maintainer would write one, and
// writes the values that put scores and a strengthen-by mention into the
// index: rigor on the first claim that something depends on and that gates
// on nothing, and a new low-rigor claim in its register whose strengthen-by
// item names it. Where no claim qualifies it writes no values.
func TestStageQueryScores(t *testing.T) {
	if *queryKBRoot == "" || *queryValues == "" {
		t.Skip("staged by test-integration-query-arxiv")
	}
	ix, err := Load(*queryKBRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.cites) > 0 {
		origin := ix.CitedBy(ix.cites[0].ClaimID)[0].LeafPath
		for _, c := range ix.cites {
			if c.LeafPath == origin {
				continue
			}
			link, err := filepath.Rel(path.Dir(c.LeafPath), origin)
			if err != nil {
				t.Fatal(err)
			}
			citing := filepath.Join(*queryKBRoot, filepath.FromSlash(c.LeafPath))
			text, err := os.ReadFile(citing)
			if err != nil {
				t.Fatal(err)
			}
			text = append(text, fmt.Sprintf("\nSee [the origin](%s#probe) of the probe.\n", filepath.ToSlash(link))...)
			if err := os.WriteFile(citing, text, 0o644); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	var target *Node
	for _, c := range ix.claims() {
		gates := slices.ContainsFunc(ix.edges, func(e Edge) bool {
			return e.Source == c.ID && (e.Relation == "depends" || e.Relation == restsOn)
		})
		held := slices.ContainsFunc(ix.edges, func(e Edge) bool { return e.Target == c.ID && e.Source != c.ID })
		if held && !gates {
			target = &c
			break
		}
	}
	if target == nil {
		return
	}
	if err := os.MkdirAll(*queryValues, 0o755); err != nil {
		t.Fatal(err)
	}
	docs := map[string]any{
		"set-rigor": map[string]any{"entry": []any{map[string]any{"id": target.ID, "rigor": 0.5}}},
		"insert-claim-entry": map[string]any{"entry": []any{map[string]any{
			"register":      target.CanonicalPath,
			"title":         "Query Probe Gated On " + target.ID,
			"rigor":         0.3,
			"rationale":     "Written by the query comparison so the index carries a low score and a strengthen-by mention.",
			"strengthen-by": []any{"Tighten the bound " + target.ID + " gives."},
		}}},
	}
	for op, doc := range docs {
		data, err := yaml.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*queryValues, op+".yaml"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// normalize is a parsed value with every number a float64, so YAML's ints and
// JSON's floats compare as the numbers they are.
func normalize(v any) any {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case uint64:
		return float64(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalize(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = normalize(e)
		}
		return out
	}
	return v
}

// full is whether an answer exercises its query: a non-empty list, or an
// object whose every list is non-empty.
func full(v any) bool {
	switch x := v.(type) {
	case []any:
		return len(x) > 0
	case map[string]any:
		for _, e := range x {
			if l, ok := e.([]any); ok && len(l) == 0 {
				return false
			}
		}
		return true
	}
	return false
}

type runSide struct {
	Exit  int `yaml:"exit"`
	Value any `yaml:"value"`
}

type caseDifference struct {
	Case   string   `yaml:"case"`
	Query  string   `yaml:"query"`
	Args   []string `yaml:"args"`
	Kbase  runSide  `yaml:"kbase"`
	KbCmd  runSide  `yaml:"kb_cmd"`
	Reason string   `yaml:"reason"`
}

type queryTally struct {
	Query    string `yaml:"query"`
	Cases    int    `yaml:"cases"`
	Equal    int    `yaml:"equal"`
	NonEmpty int    `yaml:"non-empty"`
}

type comparison struct {
	Cases       int              `yaml:"cases"`
	Equal       int              `yaml:"equal"`
	Queries     []*queryTally    `yaml:"queries"`
	NeverFull   []string         `yaml:"never-non-empty"`
	Differences []caseDifference `yaml:"differences"`
}

func readRun(t *testing.T, stem string) (int, string) {
	t.Helper()
	code, err := os.ReadFile(stem + ".exit")
	if err != nil {
		t.Fatal(err)
	}
	exit, err := strconv.Atoi(strings.TrimSpace(string(code)))
	if err != nil {
		t.Fatalf("%s.exit: %v", stem, err)
	}
	out, err := os.ReadFile(stem + ".out")
	if err != nil {
		t.Fatal(err)
	}
	return exit, string(out)
}

// compareCase is "" where kbase's answer equals kb_cmd's as data, else why
// not, with the two sides as parsed.
func compareCase(t *testing.T, stem string) (reason string, kbase, kbcmd runSide) {
	t.Helper()
	var kbaseOut, kbcmdOut string
	kbase.Exit, kbaseOut = readRun(t, stem+".kbase")
	kbcmd.Exit, kbcmdOut = readRun(t, stem+".kbcmd")
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(kbaseOut), &doc); err != nil {
		kbase.Value = kbaseOut
		return "kbase's result is not YAML: " + err.Error(), kbase, kbcmd
	}
	kbase.Value = doc
	if kbcmd.Exit != 0 {
		kbcmd.Value = kbcmdOut
		if kbase.Exit != 1 || doc["outcome"] != "refused" {
			return "kb_cmd failed and kbase did not refuse", kbase, kbcmd
		}
		return "", kbase, kbcmd
	}
	var want any
	if err := json.Unmarshal([]byte(kbcmdOut), &want); err != nil {
		kbcmd.Value = kbcmdOut
		return "kb_cmd's output is not JSON: " + err.Error(), kbase, kbcmd
	}
	kbcmd.Value = want
	if kbase.Exit != 0 || doc["outcome"] != "done" {
		return "kb_cmd answered and kbase's outcome is not done", kbase, kbcmd
	}
	for k := range doc {
		if !slices.Contains([]string{"outcome", "kb-root", "count", "offset", "truncated", ResultsKey}, k) {
			return "kbase's result carries a key beside its documented ones: " + k, kbase, kbcmd
		}
	}
	if doc["truncated"] == true {
		return "kbase's result is truncated; the comparison passes --limit 0", kbase, kbcmd
	}
	payload := doc[ResultsKey]
	kbase.Value = payload
	if !reflect.DeepEqual(normalize(payload), normalize(want)) {
		return "the answers differ as data", kbase, kbcmd
	}
	return "", kbase, kbcmd
}

func TestCompareQueries(t *testing.T) {
	if *queryDirs == "" || *queryComparison == "" {
		t.Skip("run by test-integration-query-arxiv")
	}
	var cmp comparison
	tallies := map[string]*queryTally{}
	for _, q := range comparedQueries {
		tallies[q] = &queryTally{Query: q}
		cmp.Queries = append(cmp.Queries, tallies[q])
	}
	for _, dir := range strings.Fields(*queryDirs) {
		argFiles, err := filepath.Glob(filepath.Join(dir, "*.args"))
		if err != nil {
			t.Fatal(err)
		}
		for _, argFile := range argFiles {
			raw, err := os.ReadFile(argFile)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
			stem := strings.TrimSuffix(argFile, ".args")
			tally := tallies[args[0]]
			if tally == nil {
				t.Fatalf("%s: %q is no compared query", argFile, args[0])
			}
			cmp.Cases++
			tally.Cases++
			reason, kbase, kbcmd := compareCase(t, stem)
			if reason != "" {
				cmp.Differences = append(cmp.Differences, caseDifference{Case: stem, Query: args[0], Args: args[1:],
					Kbase: kbase, KbCmd: kbcmd, Reason: reason})
				continue
			}
			cmp.Equal++
			tally.Equal++
			if kbcmd.Exit == 0 && full(kbcmd.Value) {
				tally.NonEmpty++
			}
		}
	}
	for _, q := range cmp.Queries {
		if q.NonEmpty == 0 {
			cmp.NeverFull = append(cmp.NeverFull, q.Query)
		}
	}
	data, err := yaml.Marshal(cmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(*queryComparison, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if len(cmp.Differences) > 0 || len(cmp.NeverFull) > 0 {
		t.Errorf("%d of %d cases differ; never answered non-empty: %v — see %s",
			len(cmp.Differences), cmp.Cases, cmp.NeverFull, *queryComparison)
	}
}
