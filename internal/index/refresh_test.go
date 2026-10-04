package index

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
)

func smallKB(t *testing.T) string {
	return writeKB(t, map[string]string{
		"entry-point.md": "<!-- kb-frontmatter\nkind: entry-point\n-->\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":   "[↑ KB](../entry-point.md)\n\n<!-- kb-frontmatter\nkind: index\n-->\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md":       "[↑ Vol](index.md)\n\n<!-- kb-frontmatter\nkind: leaf\nclaims: [clm-aaaaaa]\n-->\n\n# A\n",
		"vol/claim-quality.md": "# Register\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n### Quality\n" +
			"- confidence: 0.70\n- solidity: *pending*\n- rationale: r.\n",
	})
}

func checks(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Check)
	}
	return out
}

func TestRefreshThenVerify(t *testing.T) {
	root := smallKB(t)
	written, err := Refresh(root, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"vol/index.md", "entry-point.md", "vol/claim-quality.md", ".index/claims.jsonl", kb.ClaimGraphFile} {
		if !slices.Contains(written, want) {
			t.Errorf("first refresh wrote %q, missing %s", written, want)
		}
	}
	register, _ := kb.ReadText(filepath.Join(root, "vol/claim-quality.md"))
	for _, want := range []string{"- solidity: 0.70 (ok to build on, see caveats)\n", "> **Leaf references:** [a](./a.md).\n"} {
		if !strings.Contains(register, want) {
			t.Errorf("register lacks %q:\n%s", want, register)
		}
	}
	if findings, err := Verify(root); err != nil || len(findings) > 0 {
		t.Fatalf("verify after refresh: %v %v", findings, err)
	}
	if again, err := Refresh(root, log.Discard()); err != nil || len(again) > 0 {
		t.Errorf("second refresh wrote %q (%v), want nothing", again, err)
	}

	sheetPath := filepath.Join(root, kb.ClaimGraphFile)
	placeholder, _ := os.ReadFile(sheetPath)
	if err := os.WriteFile(sheetPath, append(placeholder, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if findings, _ := MetadataFindings(root); !slices.Equal(checks(findings), []string{"claim-graph sheet"}) {
		t.Errorf("an edited placeholder: findings %v", findings)
	}
	if err := os.Remove(sheetPath); err != nil {
		t.Fatal(err)
	}
	if findings, _ := MetadataFindings(root); !slices.Equal(checks(findings), []string{"claim-graph sheet"}) {
		t.Errorf("an absent sheet: findings %v", findings)
	}

	drawn := []byte("<svg><text>a drawn sheet</text></svg>\n")
	if err := os.WriteFile(sheetPath, drawn, 0o644); err != nil {
		t.Fatal(err)
	}
	if findings, _ := MetadataFindings(root); len(findings) > 0 {
		t.Errorf("a drawn sheet is checked: %v", findings)
	}
	if written, err := Refresh(root, log.Discard()); err != nil || len(written) > 0 {
		t.Errorf("refresh over a drawn sheet wrote %q (%v)", written, err)
	}
	if got, _ := os.ReadFile(sheetPath); string(got) != string(drawn) {
		t.Errorf("refresh replaced a drawn sheet")
	}
}

func TestRefreshRefuses(t *testing.T) {
	root := smallKB(t)
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("project notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Refresh(root, log.Discard()); err == nil {
		t.Error("refresh over a CLAUDE.md that is not the redirect did not refuse")
	}
	if findings, _ := MetadataFindings(root); len(findings) != 1 {
		t.Errorf("verify over it: %v", findings)
	}
}
