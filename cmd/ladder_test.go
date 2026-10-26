package main

import (
	"maps"
	"strings"
	"testing"

	"kbase/internal/kb"
	toolresult "kbase/internal/result"
)

// TestStatsCountsTheBottomRungUnderItsLabel: the ladder's last rung reads
// `unsubstantiated`, and stats counts a bottom-rung claim under that label.
func TestStatsCountsTheBottomRungUnderItsLabel(t *testing.T) {
	repo := t.TempDir()
	rigor := 0.1
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": maintenanceEntryPoint,
		maintenanceStampFile:     maintenanceFormatStamp,
		"kb-root/a.md":           "[↑ KB](entry-point.md)\n\n# A\n\nAn assertion nothing carries. <!-- claim-rigor: clm-aaaaaa -->\n",
	}
	maps.Copy(files, recordFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A Claim", Rigor: &rigor, Rationale: "Shown."}}}))
	writeRepo(t, repo, files)
	if out, code := kbase(t, repo, "refresh"); code != 0 {
		t.Fatalf("refresh: exit %d\n%s", code, out)
	}

	stats := checkDocument(t, "stats", mustRun(t, repo, "done", "stats"))
	results, _ := stats.Values["results"].(map[string]any)
	bands, _ := results["strength_bands"].(map[string]any)
	if bands["unsubstantiated"] != 1 {
		t.Errorf("strength_bands = %v, want the bottom-rung claim counted under unsubstantiated", bands)
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
