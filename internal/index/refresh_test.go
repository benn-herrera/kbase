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
		"entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md":       "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ Vol](index.md)\n\n# A\n",
		"vol/claim-quality.md": "# Register\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n### Quality\n" +
			"- confidence: 0.70\n- solidity: *pending*\n- rationale: r.\n",
	})
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
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
	written, err := Refresh(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"vol/index.md", "entry-point.md", "vol/claim-quality.md", ".index/claims.yaml", kb.ClaimGraphFile} {
		if !slices.Contains(written, want) {
			t.Errorf("first refresh wrote %q, missing %s", written, want)
		}
	}
	register := read(t, filepath.Join(root, "vol/claim-quality.md"))
	for _, want := range []string{"- solidity: 0.70 (ok to build on, see caveats)\n", "> **Leaf references:** [a](./a.md).\n"} {
		if !strings.Contains(register, want) {
			t.Errorf("register lacks %q:\n%s", want, register)
		}
	}
	if got, want := read(t, filepath.Join(root, kb.EntryPointFile)),
		"---\nkind: entry-point\nsubtree-claims: [clm-aaaaaa]\nsubtree-experiments: []\nkb-format: \"1.0.0\"\n---\n\n# KB\n\n- [vol](vol/index.md)\n"; got != want {
		t.Errorf("entry point after refresh =\n%s\nwant\n%s", got, want)
	}
	if findings, err := Verify(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Fatalf("verify after refresh: %v %v", findings, err)
	}
	if again, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil || len(again) > 0 {
		t.Errorf("second refresh wrote %q (%v), want nothing", again, err)
	}
}

// TestWriteSheetWithoutDot: with no dot on PATH nothing is drawn, a sheet
// standing is left as it is, and the placeholder is written only where
// kb-root has none.
func TestWriteSheetWithoutDot(t *testing.T) {
	root := smallKB(t)
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	sheetPath := filepath.Join(root, kb.ClaimGraphFile)
	if err := os.Remove(sheetPath); err != nil {
		t.Fatal(err)
	}
	written, removed, drawn, err := WriteSheet(kb.OnDisk(root))
	if err != nil || drawn || len(removed) > 0 || !slices.Equal(written, []string{kb.ClaimGraphFile}) {
		t.Fatalf("WriteSheet with none standing = %q, %q, %t, %v; want the placeholder written", written, removed, drawn, err)
	}
	if got, _ := os.ReadFile(sheetPath); !strings.Contains(string(got), ">no Graphviz dot on PATH<") {
		t.Errorf("the sheet written is not the placeholder: %s", got)
	}
	standing := []byte("<svg><text>a sheet drawn earlier</text></svg>\n")
	if err := os.WriteFile(sheetPath, standing, 0o644); err != nil {
		t.Fatal(err)
	}
	if written, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil || len(written) > 0 {
		t.Errorf("refresh with no dot wrote %q (%v), want nothing", written, err)
	}
	if got, _ := os.ReadFile(sheetPath); string(got) != string(standing) {
		t.Errorf("refresh with no dot replaced the sheet standing")
	}
}

// TestRefreshRemovesSheetsNoLongerCalledFor: a KB of two volumes draws the
// digest and both volumes' sheets; once the second volume holds no node, the
// next refresh removes all three and reports them removed.
func TestRefreshRemovesSheetsNoLongerCalledFor(t *testing.T) {
	leaf := func(up, fm string) string {
		return "---\nkind: leaf\n" + fm + "\n---\n[↑ " + up + "](index.md)\n\n# A\n"
	}
	register := func(title, id string) string {
		return "# Register\n\n## " + title + "\n<!-- id: " + id + " -->\n\n### Quality\n- confidence: 0.70\n- solidity: *pending*\n- rationale: r.\n"
	}
	root := writeKB(t, map[string]string{
		"entry-point.md":       "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n- [wol](wol/index.md)\n",
		"vol/index.md":         "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md":             leaf("Vol", "claims: [clm-aaaaaa]"),
		"vol/claim-quality.md": register("Claim A", "clm-aaaaaa"),
		"wol/index.md":         "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Wol\n\n- [a](a.md)\n",
		"wol/a.md":             leaf("Wol", "claims: [clm-bbbbbb]"),
		"wol/claim-quality.md": register("Claim B", "clm-bbbbbb"),
	})
	sheets := []string{kb.ClaimGraphDigestFile, "vol/" + kb.ClaimGraphFile, "wol/" + kb.ClaimGraphFile}
	written, removed, noDot, err := RefreshReporting(kb.OnDisk(root), log.Discard())
	if noDot {
		t.Skip("no Graphviz dot on PATH")
	}
	if err != nil || len(removed) > 0 {
		t.Fatalf("first refresh: removed %q, %v", removed, err)
	}
	for _, s := range sheets {
		if !slices.Contains(written, s) {
			t.Fatalf("first refresh wrote %q, missing %s", written, s)
		}
	}
	if err := os.Remove(filepath.Join(root, "wol/claim-quality.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wol/a.md"), []byte(leaf("Wol", `no-claim: "moved"`)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, removed, _, err = RefreshReporting(kb.OnDisk(root), log.Discard()); err != nil || !slices.Equal(removed, sheets) {
		t.Errorf("second refresh removed %q (%v), want %q", removed, err, sheets)
	}
	for _, s := range sheets {
		if kb.IsFile(filepath.Join(root, filepath.FromSlash(s))) {
			t.Errorf("%s still stands", s)
		}
	}
	if !kb.IsFile(filepath.Join(root, kb.ClaimGraphFile)) {
		t.Error("the full sheet is gone")
	}
}

func TestVerifyLeavesTheSheetAndTheCitationGateAlone(t *testing.T) {
	root := smallKB(t)
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	verify := func(state string) (standard, build []Finding) {
		t.Helper()
		standard, err := Verify(kb.OnDisk(root))
		if err != nil {
			t.Fatalf("%s: Verify: %v", state, err)
		}
		build, err = BuildVerify(kb.OnDisk(root))
		if err != nil {
			t.Fatalf("%s: BuildVerify: %v", state, err)
		}
		return standard, build
	}

	sheetPath := filepath.Join(root, kb.ClaimGraphFile)
	placeholder, _ := os.ReadFile(sheetPath)
	if err := os.WriteFile(sheetPath, append(placeholder, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if standard, build := verify("hand-edited sheet"); len(standard)+len(build) > 0 {
		t.Errorf("a hand-edited sheet: Verify found %v, BuildVerify found %v", standard, build)
	}
	if err := os.Remove(sheetPath); err != nil {
		t.Fatal(err)
	}
	if standard, build := verify("deleted sheet"); len(standard)+len(build) > 0 {
		t.Errorf("a deleted sheet: Verify found %v, BuildVerify found %v", standard, build)
	}

	indexPath := filepath.Join(root, "vol/index.md")
	appendTo := func(text string) {
		t.Helper()
		body, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(indexPath, append(body, text...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	appendTo("\nThis follows per design-doc Invariant 3.\n")
	standard, build := verify("citation-shaped sentence")
	if len(standard) > 0 {
		t.Errorf("a citation-shaped sentence: Verify found %v", standard)
	}
	if got := checks(build); !slices.Equal(got, []string{checkChannel}) {
		t.Errorf("a citation-shaped sentence: BuildVerify checks %q, want %q", got, []string{checkChannel})
	}

	appendTo("\n[gone](missing.md)\n")
	standard, build = verify("citation and dead link")
	cites, err := CitationFindings(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(standard) == 0 || len(cites) == 0 || !slices.Equal(build, slices.Concat(standard, cites)) {
		t.Errorf("BuildVerify found %v, want the standard check's %v then the citation gate's %v", build, standard, cites)
	}
}

// TestSaveDrainsTheObsoleteButWhatItSaves: the save of a migrated KB writes
// every file the migration produced, stamps the entry point, and removes the
// obsolete paths — but one the save itself covers, which stands.
func TestSaveDrainsTheObsoleteButWhatItSaves(t *testing.T) {
	root := smallKB(t)
	leaf := read(t, filepath.Join(root, "vol/a.md"))
	for rel, text := range map[string]string{".index/claims.jsonl": "{}\n", "vol/old.md": "# Old\n"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := kb.MigratedSource(root, kb.UnstampedFormatVersion, map[string][]byte{"kb-root/vol/a.md": []byte(leaf)},
		[]string{"kb-root/.index/claims.jsonl", "kb-root/vol/a.md", "kb-root/vol/old.md"})
	_, removed, _, err := RefreshReporting(src, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	removed = slices.DeleteFunc(removed, func(p string) bool { return strings.HasSuffix(p, ".svg") })
	if want := []string{".index/claims.jsonl", "vol/old.md"}; !slices.Equal(removed, want) {
		t.Errorf("removed %q, want %q", removed, want)
	}
	if got := read(t, filepath.Join(root, "vol/a.md")); got != leaf {
		t.Errorf("the obsolete path the save covers was not kept as saved: %q", got)
	}
	for _, rel := range []string{".index/claims.jsonl", "vol/old.md"} {
		if kb.IsFile(filepath.Join(root, filepath.FromSlash(rel))) {
			t.Errorf("%s still stands", rel)
		}
	}
	if stamp, _, _ := kb.FormatStamp(read(t, filepath.Join(root, kb.EntryPointFile))); stamp != kb.FormatVersion {
		t.Errorf("the save did not stamp the entry point: %q", stamp)
	}
	if src.Migrated() {
		t.Error("the source still reads as migrated after the save")
	}
}

func TestRefreshRefuses(t *testing.T) {
	root := smallKB(t)
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("project notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err == nil {
		t.Error("refresh over a CLAUDE.md that is not the redirect did not refuse")
	}
	if findings, _ := MetadataFindings(kb.OnDisk(root)); len(findings) != 1 {
		t.Errorf("verify over it: %v", findings)
	}
}
