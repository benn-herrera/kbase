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

	"kbase/internal/buildrecords"
)

var (
	liveRepo   = flag.String("claimgraph.repo", "", "a repository a build without --no-inference wrote its records into")
	liveResult = flag.String("claimgraph.result", "", "that build's result document")
	liveOut    = flag.String("claimgraph.out", "", "where counts.yaml, nodepass.json, kb-build-node-pass.yaml and unmarked-plan.json are written")
)

// TestLiveBuildCounts reads what a build that asked wrote — its node-pass,
// unmarked and classification records, and the stage reports its result
// names — and writes the counts the live recipe reports: counts.yaml;
// nodepass.json, the record's verdicts and claims per leaf for kb_tools'
// reading of the same tree to be compared against; and, for kb_tools'
// shortlist over that tree, a copy of the node-pass record
// (kb-build-node-pass.yaml) and the planned pairs (unmarked-plan.json). It
// skips unless the live recipe names a repository.
func TestLiveBuildCounts(t *testing.T) {
	if *liveRepo == "" {
		t.Skip("no -claimgraph.repo: the live build recipe runs this")
	}
	np, ok, err := buildrecords.ReadNodePass(recordsAt(*liveRepo))
	if err != nil || !ok {
		t.Fatalf("node-pass record: %t, %v", ok, err)
	}
	cl, ok, err := buildrecords.ReadClassification(recordsAt(*liveRepo))
	if err != nil || !ok {
		t.Fatalf("classification record: %t, %v", ok, err)
	}
	um, ok, err := buildrecords.ReadUnmarked(recordsAt(*liveRepo))
	if err != nil || !ok || um.Planned == nil {
		t.Fatalf("unmarked record: %t, %v, planned %v", ok, err, um.Planned)
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
	classify := resultFinding(t, *liveResult, "stage-D-classify")
	demoted := []string{}
	list, _ := classify["demoted"].([]any)
	for _, d := range list {
		demoted = append(demoted, fmt.Sprint(d))
	}
	slices.Sort(demoted)
	candidates := resultFinding(t, *liveResult, "stage-D-candidates")
	unmarkedAsks := resultFinding(t, *liveResult, "stage-U-asks")

	unmarkedLetters := map[string]int{}
	unmarkedOutcomes := map[string]int{}
	sources := map[string]bool{}
	for _, p := range um.Pairs {
		l := "none"
		if p.Letter != nil {
			l = *p.Letter
		}
		unmarkedLetters[l]++
		unmarkedOutcomes[p.Outcome]++
		sources[p.Source] = true
	}
	plan := make([][2]string, len(*um.Planned))
	for i, p := range *um.Planned {
		plan[i] = p
	}

	zero := func(names []string, of map[string]int) map[string]int {
		out := map[string]int{}
		for _, n := range names {
			out[n] = of[n]
		}
		return out
	}
	counts := map[string]any{
		"unmarked": map[string]any{
			"planned-pairs":      len(plan),
			"pairs-asked":        len(um.Pairs),
			"unanswered":         len(unanswered(um)),
			"source-groups":      len(sources),
			"calls":              unmarkedAsks["calls"],
			"pairs-by-letter":    zero([]string{"A", "B", "none"}, unmarkedLetters),
			"pairs-by-outcome":   zero([]string{ClassifyAnswered, ClassifyReasked, ClassifyDefaulted}, unmarkedOutcomes),
			"yeses":              unmarkedLetters["A"],
			"defaults":           unmarkedOutcomes[ClassifyDefaulted],
			"report-asks-totals": unmarkedAsks,
		},
		"candidates-by-harvest": candidates["by-harvest"],
		"own-equations-dropped": candidates["own-equations-dropped"],
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
	writeJSON(t, filepath.Join(*liveOut, "unmarked-plan.json"), plan)
	b, err = os.ReadFile(filepath.Join(*liveRepo, buildrecords.NodePassFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*liveOut, buildrecords.NodePassFile), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if states[ReadLanded] != len(np.Leaves) {
		t.Errorf("leaves by state = %v, want every leaf landed", states)
	}
	if classified[ClassifyDrafted] != 0 {
		t.Errorf("%d candidates took their draft with no reader, in a build that asked", classified[ClassifyDrafted])
	}
	if missing := unanswered(um); len(missing) > 0 {
		t.Errorf("%d planned pairs carry no outcome: %q", len(missing), pairs(missing))
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// resultFinding is the claim-graph finding named check, out of the stage
// reports a build's result document names.
func resultFinding(t *testing.T, path, check string) map[string]any {
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
				if finding["check"] == check {
					return finding
				}
			}
		}
	}
	t.Fatalf("no stage report %s names holds a %s finding: %s", path, check, strings.SplitN(string(b), "\n", 2)[0])
	return nil
}
