package claimgraph

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

var (
	liveRepo   = flag.String("claimgraph.repo", "", "a repository a build without --no-inference wrote its records into")
	liveResult = flag.String("claimgraph.result", "", "that build's result document")
	liveOut    = flag.String("claimgraph.out", "", "where counts.yaml and nodepass.json are written")
)

// TestLiveBuildCounts reads what a build that asked wrote — its node-pass and
// classification records, and the result naming the ring classification
// demoted — and writes the counts the live recipe reports: counts.yaml, and
// nodepass.json, the record's verdicts and claims per leaf for kb_tools'
// reading of the same tree to be compared against. It skips unless the live
// recipe names a repository.
func TestLiveBuildCounts(t *testing.T) {
	if *liveRepo == "" {
		t.Skip("no -claimgraph.repo: the live build recipe runs this")
	}
	np, ok, err := ReadNodePass(*liveRepo)
	if err != nil || !ok {
		t.Fatalf("node-pass record: %t, %v", ok, err)
	}
	cl, ok, err := ReadClassification(*liveRepo)
	if err != nil || !ok {
		t.Fatalf("classification record: %t, %v", ok, err)
	}
	verdicts := map[string]int{}
	causes := map[string]int{}
	outcomes := map[string]int{}
	states := map[string]int{}
	asked, minted := 0, 0
	type leaf struct {
		Verdicts []map[string]any `json:"verdicts"`
		Claims   []string         `json:"claims"`
	}
	leaves := map[string]leaf{}
	for path, e := range np.Leaves {
		states[e.State]++
		if e.Outcome != nil {
			outcomes[*e.Outcome]++
		}
		var l leaf
		for _, v := range e.Verdicts {
			asked++
			verdicts[v.Verdict]++
			entry := map[string]any{"line": v.Line, "verdict": v.Verdict}
			if v.Cause != nil {
				causes[*v.Cause]++
				entry["cause"] = *v.Cause
			}
			l.Verdicts = append(l.Verdicts, entry)
		}
		for _, c := range e.Claims {
			minted++
			l.Claims = append(l.Claims, c.Title)
		}
		leaves[path] = l
	}
	letters := map[string]int{}
	classified := map[string]int{}
	for _, c := range cl.Candidates {
		l := "none"
		if c.Letter != nil {
			l = *c.Letter
		}
		letters[l]++
		classified[c.Outcome]++
	}
	demoted := resultDemoted(t, *liveResult)

	zero := func(names []string, of map[string]int) map[string]int {
		out := map[string]int{}
		for _, n := range names {
			out[n] = of[n]
		}
		return out
	}
	counts := map[string]any{
		"leaves-by-state":       zero([]string{ReadUnread, ReadPlanned, ReadLanded}, states),
		"leaves-by-outcome":     zero([]string{OutcomeMinted, OutcomeNoClaim, OutcomeNothingToRead}, outcomes),
		"paragraphs-asked":      asked,
		"verdicts":              zero([]string{JudgementClaim, JudgementNotAClaim, JudgementDefaulted}, verdicts),
		"defaults-by-cause":     zero(DefaultCauses, causes),
		"claims-minted":         minted,
		"candidates-classified": len(cl.Candidates),
		"candidates-by-letter":  zero([]string{"A", "B", "C", "none"}, letters),
		"candidates-by-outcome": zero([]string{ClassifyAnswered, ClassifyReasked, ClassifyDefaulted, ClassifyDrafted}, classified),
		"rings-demoted-edges":   len(demoted),
		"demoted":               demoted,
	}
	b, err := yaml.Marshal(map[string]any{"counts": counts})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*liveOut, "counts.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	j, err := json.MarshalIndent(leaves, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*liveOut, "nodepass.json"), j, 0o644); err != nil {
		t.Fatal(err)
	}
	if states[ReadLanded] != len(np.Leaves) {
		t.Errorf("leaves by state = %v, want every leaf landed", states)
	}
	if classified[ClassifyDrafted] != 0 {
		t.Errorf("%d candidates took their draft with no reader, in a build that asked", classified[ClassifyDrafted])
	}
}

// resultDemoted is the edges stage-D-classify reports a cycle demoted, out of
// the stage reports a build's result document names.
func resultDemoted(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Stages []struct {
			Report string `yaml:"report"`
		} `yaml:"stages"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, s := range doc.Stages {
		rb, err := os.ReadFile(s.Report)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Rows []struct {
				Claimgraph []map[string]any `yaml:"claimgraph"`
			} `yaml:"rows"`
		}
		if err := yaml.Unmarshal(rb, &report); err != nil {
			t.Fatal(err)
		}
		for _, row := range report.Rows {
			for _, finding := range row.Claimgraph {
				if finding["check"] != "stage-D-classify" {
					continue
				}
				out := []string{}
				list, _ := finding["demoted"].([]any)
				for _, d := range list {
					out = append(out, fmt.Sprint(d))
				}
				slices.Sort(out)
				return out
			}
		}
	}
	t.Fatalf("no stage report %s names holds a stage-D-classify finding: %s", path, strings.SplitN(string(b), "\n", 2)[0])
	return nil
}
