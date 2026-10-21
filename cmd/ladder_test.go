package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRefreshRewritesTheBottomRungsWords: the ladder's last rung reads
// `unsubstantiated`, and a KB whose register and index still carry the words it
// replaced is reported by verify as build-status drift. The refresh that
// rewrites the derived layer clears the finding, and stats counts the bottom-rung
// claim under the rung's own label — the cut itself does not move, so the claim
// stays where it stood.
func TestRefreshRewritesTheBottomRungsWords(t *testing.T) {
	repo := t.TempDir()
	writeRepo(t, repo, map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [a](a.md)\n",
		"kb-root/a.md": "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ KB](entry-point.md)\n\n# A\n\n" +
			"An assertion nothing carries. <!-- claim-rigor: clm-aaaaaa -->\n",
		"kb-root/claim-rigor.md": "# Register\n\n## A Claim\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n" +
			"- rigor: 0.10\n- strength: *pending*\n- rationale: Shown.\n",
	})
	if out, code := kbase(t, repo, "refresh"); code != 0 {
		t.Fatalf("refresh: exit %d\n%s", code, out)
	}

	// The KB as an older kbase wrote it: the derived line and the index carry
	// the rung's superseded words.
	register := filepath.Join(repo, "kb-root", "claim-rigor.md")
	text, err := os.ReadFile(register)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "- strength: 0 (unsubstantiated)") {
		t.Fatalf("the refresh did not write the rung's words:\n%s", text)
	}
	edit := strings.Replace(string(text), "(unsubstantiated)", "(refuted, do not use)", 1)
	if err := os.WriteFile(register, []byte(edit), 0o644); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(repo, "kb-root", ".index", "claims.yaml")
	text, err = os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	old := strings.ReplaceAll(string(text), `"build_status": "unsubstantiated", "build_band": "unsubstantiated"`,
		`"build_status": "refuted, do not use", "build_band": "refuted"`)
	if old == string(text) {
		t.Fatalf("the index carries no build status to supersede:\n%s", text)
	}
	if err := os.WriteFile(index, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	// verify: the register's phrase and the index's words are both stale, and the
	// finding names the phrase the ladder now gives.
	verified := checkDocument(t, "verify", mustRun(t, repo, "refused", "verify"))
	details := itemDetails(verified.items("refusals"))
	if !strings.Contains(details, "build-status mismatch") || !strings.Contains(details, "expected unsubstantiated") {
		t.Errorf("verify's findings %q, want the bottom rung's words reported stale", details)
	}

	// The refresh rewrites the derived layer, and verify is green after it.
	mustRun(t, repo, "done", "refresh")
	green := checkDocument(t, "verify", mustRun(t, repo, "done", "verify"))
	if findings := green.items("findings"); len(findings) != 0 {
		t.Errorf("verify after the refresh reports %v", findings)
	}

	// stats counts the claim under the rung's own label, and nothing under the
	// words it replaced.
	stats := checkDocument(t, "stats", mustRun(t, repo, "done", "stats"))
	results, _ := stats.Values["results"].(map[string]any)
	bands, _ := results["strength_bands"].(map[string]any)
	if bands["unsubstantiated"] != 1 {
		t.Errorf("strength_bands = %v, want the bottom-rung claim counted under unsubstantiated", bands)
	}
	if _, old := bands["refuted, do not use"]; old {
		t.Errorf("strength_bands = %v, carries the superseded label", bands)
	}
}

// mustRun is one invocation wanted to end with an outcome: its stdout, the test
// failing where the outcome or its exit code differs.
func mustRun(t *testing.T, repo, outcome string, args ...string) string {
	t.Helper()
	out, code := kbase(t, repo, args...)
	if want := exitCodes[outcome]; code != want {
		t.Fatalf("%q: exit %d, want %d for %s\n%s", args, code, want, outcome, out)
	}
	if !strings.Contains(out, `outcome: "`+outcome+`"`) {
		t.Fatalf("%q: outcome %q, want %s\n%s", args, out, outcome, out)
	}
	return out
}

// itemDetails is a document's items as one string, so a test can look for the
// facts they name.
func itemDetails(items []map[string]any) string {
	var b strings.Builder
	for _, item := range items {
		if detail, ok := item["detail"].(string); ok {
			b.WriteString(detail)
			b.WriteString("\n")
		}
	}
	return b.String()
}
