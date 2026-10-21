package index

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
)

func smallKB(t *testing.T) string {
	return writeKB(t, map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md":       "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ Vol](index.md)\n\n# A\n",
		"vol/claim-rigor.md": "# Register\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n" +
			"- rigor: 0.70\n- strength: *pending*\n- rationale: r.\n",
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
	for _, want := range []string{"vol/index.md", "entry-point.md", "vol/claim-rigor.md", ".index/claims.yaml", kb.ClaimGraphFile} {
		if !slices.Contains(written, want) {
			t.Errorf("first refresh wrote %q, missing %s", written, want)
		}
	}
	register := read(t, filepath.Join(root, "vol/claim-rigor.md"))
	for _, want := range []string{"- strength: 0 (unsubstantiated)\n", "> **Leaf references:** [a](./a.md).\n"} {
		if !strings.Contains(register, want) {
			t.Errorf("register lacks %q:\n%s", want, register)
		}
	}
	if got, want := read(t, filepath.Join(root, kb.EntryPointFile)),
		"---\nkind: entry-point\nsubtree-claims: [clm-aaaaaa]\nsubtree-experiments: []\nkb-format: \""+kb.FormatVersion+"\"\n---\n\n# KB\n\n- [vol](vol/index.md)\n"; got != want {
		t.Errorf("entry point after refresh =\n%s\nwant\n%s", got, want)
	}
	if findings, err := Verify(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Fatalf("verify after refresh: %v %v", findings, err)
	}
	if again, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil || len(again) > 0 {
		t.Errorf("second refresh wrote %q (%v), want nothing", again, err)
	}
}

// TestRefreshLeavesAnAuthoredOneInTheFile: a rigor standing above the rigor
// tables' maximum reads at the maximum, and that is a reading rule — refresh
// derives the index from 0.95 and leaves the leaf holding `rigor: 1.0` byte for
// byte, the number it found never rewritten.
func TestRefreshLeavesAnAuthoredOneInTheFile(t *testing.T) {
	leaf := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 1.0\n" +
		"    lifts:\n      - clm-aaaaaa: 0.5\n---\n[↑ Vol](index.md)\n\n# Exp\n"
	root := writeKB(t, map[string]string{
		"entry-point.md":     "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":       "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n- [exp](exp.md)\n",
		"vol/a.md":           "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ Vol](index.md)\n\n# A\n",
		"vol/exp.md":         leaf,
		"vol/claim-rigor.md": "# Register\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n- rigor: 0.70\n- strength: *pending*\n- rationale: r.\n",
	})
	written, err := Refresh(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, "vol/exp.md")); got != leaf {
		t.Errorf("refresh rewrote the leaf:\n%s\nwant it as it found it:\n%s", got, leaf)
	}
	if slices.Contains(written, "vol/exp.md") {
		t.Errorf("refresh reports the leaf written; it wrote %q", written)
	}
	_, rows, problems, err := kb.ReadIndex[kb.NodeRow](kb.OnDisk(root), nil)
	if err != nil || len(problems) > 0 {
		t.Fatalf("reading the index rows: %v, %v", problems, err)
	}
	for _, r := range rows {
		if r.ID != "exp-aaaaaa" {
			continue
		}
		if r.Rigor == nil || *r.Rigor != kb.RigorMaximum {
			t.Errorf("the index row reads rigor %v, want the maximum %v", r.Rigor, kb.RigorMaximum)
		}
		return
	}
	t.Errorf("the index carries no row for exp-aaaaaa: %+v", rows)
}

// TestRefreshRecordsTheExactStrength: a premise's own value reaches the record
// unrounded — 0.70 × 0.72 = 0.504 as itself, and the dependent of a value that
// needs more than four decimals as that value — and every record refresh writes
// compares fresh.
func TestRefreshRecordsTheExactStrength(t *testing.T) {
	entry := func(title, id, rigor, rests string) string {
		return "## " + title + "\n<!-- id: " + id + " -->\n\n### Rigor\n- rigor: " + rigor + "\n" + rests +
			"- strength: *pending*\n- rationale: r.\n\n"
	}
	root := writeKB(t, map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md": "---\nkind: leaf\nclaims: [clm-aaaaaa, clm-bbbbbb, clm-cccccc, clm-dddddd]\n---\n[↑ Vol](index.md)\n\n# A\n\n" +
			"<!-- claim-rigor: clm-aaaaaa -->\n\nBody A.\n\n<!-- claim-rigor: clm-bbbbbb -->\n\nBody B.\n\n" +
			"<!-- claim-rigor: clm-cccccc -->\n\nBody C.\n\n<!-- claim-rigor: clm-dddddd -->\n\nBody D.\n",
		"vol/claim-rigor.md": "# Register\n\n" +
			entry("Claim A", "clm-aaaaaa", "0.70", "- rests-on:\n  - Axiom S1 (bearing 0.70)\n") +
			entry("Claim B", "clm-bbbbbb", "0.90", "- rests-on:\n  - clm-aaaaaa (strength *pending*) (bearing 0.72)\n") +
			entry("Claim C", "clm-cccccc", "0.90", "- rests-on:\n  - clm-bbbbbb (strength *pending*) (bearing 0.72)\n") +
			entry("Claim D", "clm-dddddd", "1.00", ""),
		kb.AxiomsFile: "# Axioms\n\n### Axiom S1: The rule\n\nBody.\n",
	})
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	register := read(t, filepath.Join(root, "vol/claim-rigor.md"))
	for _, want := range []string{
		"- strength: 0.7 (ok to build on, see caveats)",
		"- strength: 0.504 (use as input only",
		"- strength: 0.36288 (do not build on, rework needed)",
		"- strength: 0 (unsubstantiated)",
		"(strength 0.7) (bearing 0.72)",
		"(strength 0.504) (bearing 0.72)",
	} {
		if !strings.Contains(register, want) {
			t.Errorf("register lacks %q:\n%s", want, register)
		}
	}
	for _, unwanted := range []string{"- strength: 0.5 ", "(strength 0.5)"} {
		if strings.Contains(register, unwanted) {
			t.Errorf("register carries the two-decimal spelling %q:\n%s", unwanted, register)
		}
	}
	if findings, err := Verify(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Fatalf("verify after refresh: %v %v", findings, err)
	}
	if again, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil || len(again) > 0 {
		t.Errorf("second refresh wrote %q (%v), want nothing", again, err)
	}
	if findings, err := Verify(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Errorf("verify after the second refresh: %v %v", findings, err)
	}

	truncated := strings.Replace(register, "- strength: 0.504 ", "- strength: 0.5 ", 1)
	if err := os.WriteFile(filepath.Join(root, "vol/claim-rigor.md"), []byte(truncated), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := Verify(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(findings, func(f Finding) bool {
		return f.Check == "strength freshness" && strings.Contains(f.Detail, "clm-bbbbbb")
	}) {
		t.Errorf("a hand-truncated record drew no strength freshness finding: %v", findings)
	}
	if !slices.ContainsFunc(findings, func(f Finding) bool {
		return f.Check == "strength freshness" && strings.Contains(f.Detail, "strength 0.5, expected 0.504")
	}) {
		t.Errorf("the freshness finding does not show the truncation it reports: %v", findings)
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
		return "# Register\n\n## " + title + "\n<!-- id: " + id + " -->\n\n### Rigor\n- rigor: 0.70\n- strength: *pending*\n- rationale: r.\n"
	}
	root := writeKB(t, map[string]string{
		"entry-point.md":     "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n- [wol](wol/index.md)\n",
		"vol/index.md":       "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md":           leaf("Vol", "claims: [clm-aaaaaa]"),
		"vol/claim-rigor.md": register("Claim A", "clm-aaaaaa"),
		"wol/index.md":       "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Wol\n\n- [a](a.md)\n",
		"wol/a.md":           leaf("Wol", "claims: [clm-bbbbbb]"),
		"wol/claim-rigor.md": register("Claim B", "clm-bbbbbb"),
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
	if err := os.Remove(filepath.Join(root, "wol/claim-rigor.md")); err != nil {
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

// TestRefreshRefusesASymlinkedIndexFile: a derived file that is a relative
// symbolic link out of the tree is refused, and the file the link names keeps
// its bytes.
func TestRefreshRefusesASymlinkedIndexFile(t *testing.T) {
	root := smallKB(t)
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(filepath.Dir(root)), "outside.yaml")
	if err := os.WriteFile(outside, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, kb.IndexDir, kb.IndexFileName(kb.IndexClaims))
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../../outside.yaml", link); err != nil {
		t.Fatal(err)
	}
	_, err := Refresh(kb.OnDisk(root), log.Discard())
	var r result.Refusal
	if !errors.As(err, &r) || len(r) != 1 || r[0].Check != kb.CheckSymlink || r[0].Path != link {
		t.Fatalf("refresh over a symlinked index file = %v, want one %s refusal naming %s", err, kb.CheckSymlink, link)
	}
	if got := read(t, outside); got != "kept\n" {
		t.Errorf("%s = %q, want it untouched", outside, got)
	}
}

// TestSaveRefusesASymlinkedDocument: a migrated KB whose document is a
// symbolic link is refused at the save, the entry point stays unstamped and
// the file the link names keeps its bytes.
func TestSaveRefusesASymlinkedDocument(t *testing.T) {
	root := smallKB(t)
	entry := read(t, filepath.Join(root, kb.EntryPointFile))
	outside := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "vol/a.md")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	converted := []byte("---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ Vol](index.md)\n\n# A changed\n")
	src := kb.MigratedSource(root, "1.0.0", map[string][]byte{"kb-root/vol/a.md": converted}, nil)
	_, _, _, err := RefreshReporting(src, log.Discard())
	var r result.Refusal
	if !errors.As(err, &r) || len(r) != 1 || r[0].Check != kb.CheckSymlink || r[0].Path != link {
		t.Fatalf("refresh over a symlinked document = %v, want one %s refusal naming %s", err, kb.CheckSymlink, link)
	}
	if got := read(t, filepath.Join(root, kb.EntryPointFile)); got != entry {
		t.Errorf("the entry point after a refused save =\n%s\nwant it unstamped", got)
	}
	if got := read(t, outside); got != "outside\n" {
		t.Errorf("%s = %q, want it untouched", outside, got)
	}
}

// TestLegacyAxiomFindings: a declaration the converter leaves in AGENTS.md
// beside the KB's axioms.md is reported once each, informational and never
// refresh-fixable, and the metadata gate stays clean; a KB with no axioms.md
// reports none.
func TestLegacyAxiomFindings(t *testing.T) {
	agents := "# Agents\n\n### INVARIANT-S1: First rule\n\nBody.\n\n- Axiom 1: **One** — a bullet.\n\nInstructions stay.\n"
	root := smallKB(t)
	for rel, text := range map[string]string{"axioms.md": "# Axioms\n\n### Axiom S1: First rule\n\nBody.\n", "AGENTS.md": agents} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	findings, err := LegacyAxiomFindings(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range findings {
		if f.Check != CheckAxiomSource || f.Path != kb.AgentsFile {
			t.Errorf("finding %+v, want check %s in %s", f, CheckAxiomSource, kb.AgentsFile)
		}
		if f.RefreshFixable {
			t.Errorf("%s is refresh-fixable, want informational", f.Detail)
		}
		if item := f.Item(); item.Remedy != "" {
			t.Errorf("%s carries remedy %q, want none", f.Detail, item.Remedy)
		}
		got = append(got, fmt.Sprintf("%d:%s", f.Line, f.Detail))
	}
	if want := []string{"3:INVARIANT-S1", "7:Axiom 1"}; !slices.Equal(got, want) {
		t.Errorf("findings %q, want %q", got, want)
	}
	if findings, err := Verify(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Errorf("verify over a KB carrying both: %v %v, want none", findings, err)
	}

	root = smallKB(t)
	if err := os.WriteFile(filepath.Join(root, kb.AgentsFile), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	if findings, err := LegacyAxiomFindings(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Errorf("a KB with no axioms.md: %v %v, want none", findings, err)
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
