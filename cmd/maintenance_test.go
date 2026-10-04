package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/log"
)

func maintenanceRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": "<!-- kb-frontmatter\nkind: entry-point\n-->\n\n# KB\n\n- [a](a.md)\n",
		"kb-root/a.md":           "[↑ KB](entry-point.md)\n\n<!-- kb-frontmatter\nkind: leaf\nno-claim: \"prose\"\n-->\n\n# A\n",
	}
	for rel, text := range files {
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func runMaintenance(t *testing.T, run func(maintenanceOptions) (int, error), dir string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code, err := run(maintenanceOptions{WorkDir: dir, Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	if err != nil {
		t.Fatal(err)
	}
	return code, out.String()
}

func TestRefreshAndVerifyVerbs(t *testing.T) {
	repo := maintenanceRepo(t)
	sub := filepath.Join(repo, "kb-root")

	if code, out := runMaintenance(t, runVerify, repo); code != 1 || !strings.Contains(out, `outcome: "refused"`) || !strings.Contains(out, "claims.jsonl is missing") {
		t.Errorf("verify before refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runRefresh, sub); code != 0 || !strings.Contains(out, `outcome: "done"`) || !strings.Contains(out, `- ".index/claims.jsonl"`) {
		t.Errorf("first refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runRefresh, repo); code != 0 || !strings.Contains(out, `outcome: "unchanged"`) {
		t.Errorf("second refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runVerify, repo); code != 0 || !strings.Contains(out, `outcome: "done"`) {
		t.Errorf("verify after refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runVerify, t.TempDir()); code != 1 || !strings.Contains(out, "not inside a git worktree") {
		t.Errorf("verify outside a worktree: exit %d\n%s", code, out)
	}
}
