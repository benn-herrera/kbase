package main

import (
	"maps"
	"strings"
	"testing"

	"kbase/internal/kb"
	toolresult "kbase/internal/result"
)

// TestStatsKeysABandBySlugNotLabel: stats counts a claim under its band's
// slug, never its label, on the do-not-build rung, whose slug and label differ.
// The claim rests fully on a fully judged work, so its strength is its rigor:
// a claim nothing carries reads 0, the bottom rung, whatever its rigor.
func TestStatsKeysABandBySlugNotLabel(t *testing.T) {
	repo := t.TempDir()
	rigor, whole := 0.3, 1.0
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": maintenanceEntryPoint,
		maintenanceStampFile:     maintenanceFormatStamp,
		"kb-root/a.md":           "[↑ KB](entry-point.md)\n\n# A\n\nAn assertion a work carries.<!-- kbase-id: clm-aaaaaa -->\n",
	}
	maps.Copy(files, recordFiles(kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A Claim", Rigor: &rigor, Rationale: "Shown."}},
		Works:  []kb.WorkNode{{ID: "wrk-aaaaaa", Label: "Work2020", Title: "A Work", Rigor: &whole, Rationale: "Cited."}},
		Founds: []kb.BearingEdge{{Source: "wrk-aaaaaa", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: whole}}},
	}))
	writeRepo(t, repo, files)
	updateFormatKB(t, repo)

	stats := checkResult(t, "stats", mustRun(t, repo, "done", "stats"))
	results, _ := stats.Values["results"].(map[string]any)
	bands, _ := results["strength-bands"].(map[string]any)
	rung := kb.StrengthBandLadder[len(kb.StrengthBandLadder)-2]
	if _, byLabel := bands[rung.Label]; bands[rung.Slug] != 1 || byLabel {
		t.Errorf("strength-bands = %v, want the claim counted under %q and nothing under %q", bands, rung.Slug, rung.Label)
	}
}

// mustRun is one invocation wanted to end with an outcome: its stdout, the test
// failing where the outcome or its exit code differs.
func mustRun(t *testing.T, repo, outcome string, args ...string) string {
	t.Helper()
	out, code := kbase(t, repo, args...)
	if want := toolresult.ExitCode(outcome); code != want {
		t.Fatalf("%q: exit %d, want %d for %s\n%s", args, code, want, outcome, out)
	}
	if !strings.Contains(out, `outcome: "`+outcome+`"`) {
		t.Fatalf("%q: outcome %q, want %s\n%s", args, out, outcome, out)
	}
	return out
}
