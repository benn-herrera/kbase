package index

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/result"
)

// kbWith is files under a fresh kb-root, beside g's records, the format stamp
// and an entry point listing volumes, and returns the kb-root.
func kbWith(t *testing.T, g kb.Graph, volumes []string, files map[string]string) string {
	t.Helper()
	ep := "# KB\n\n"
	for _, v := range volumes {
		ep += "- [" + v + "](" + v + "/index.md)\n"
	}
	all := graphFiles(g)
	all[kb.EntryPointFile] = ep
	all[kb.FormatFile] = string(kb.EncodeFormatStamp(kb.FormatVersion))
	maps.Copy(all, files)
	return writeKB(t, all)
}

func smallKB(t *testing.T) string {
	return kbWith(t, kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(0.7), Rationale: "r."}}}, []string{"vol"},
		map[string]string{
			"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
			"vol/a.md":     "[↑ Vol](index.md)\n\n# A\n\nA claim. <!-- kbase-id: clm-aaaaaa -->\n",
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

// TestRefreshThenVerify: a first refresh writes the register page and the
// sheet and leaves the entry point, the documents and the records as they
// stand; verify then finds nothing, and a second refresh writes nothing.
func TestRefreshThenVerify(t *testing.T) {
	root := smallKB(t)
	entry := read(t, filepath.Join(root, kb.EntryPointFile))
	written, err := Refresh(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{kb.ClaimGraphFile, "vol/" + kb.RegisterFile}; !slices.Equal(slices.Sorted(slices.Values(written)), want) {
		t.Errorf("first refresh wrote %q, want %q", written, want)
	}
	register := read(t, filepath.Join(root, "vol", kb.RegisterFile))
	for _, want := range []string{"- strength: 0 (unsubstantiated)\n", "> **Leaf references:** [a](./a.md).\n"} {
		if !strings.Contains(register, want) {
			t.Errorf("register lacks %q:\n%s", want, register)
		}
	}
	if got := read(t, filepath.Join(root, kb.EntryPointFile)); got != entry {
		t.Errorf("entry point after refresh =\n%s\nwant it as it stood:\n%s", got, entry)
	}
	if findings, err := Verify(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Fatalf("verify after refresh: %v %v", findings, err)
	}
	if again, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil || len(again) > 0 {
		t.Errorf("second refresh wrote %q (%v), want nothing", again, err)
	}
}

// TestRefreshRefusesAnAboveMaximumRigor: a node record whose rigor stands
// above the maximum refuses the load, and refresh writes nothing.
func TestRefreshRefusesAnAboveMaximumRigor(t *testing.T) {
	g := kb.Graph{
		Claims:      []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(0.7)}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Exp", Status: "run", Rigor: f(1.2)}},
		Scorable:    []kb.ScorableEdge{{Relation: kb.RelationLifts, Source: "exp-aaaaaa", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}}},
	}
	root := kbWith(t, g, []string{"vol"}, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n- [exp](exp.md)\n",
		"vol/a.md":     "[↑ Vol](index.md)\n\n# A\n\nA claim. <!-- kbase-id: clm-aaaaaa -->\n",
		"vol/exp.md":   "[↑ Vol](index.md)\n\n# Exp\n<!-- kbase-id: exp-aaaaaa -->\n",
	})
	nodes := read(t, filepath.Join(root, kb.GraphNodesFile))
	if !strings.Contains(nodes, "rigor: 1.2\n") {
		t.Fatalf("the node records do not hold the rigor above the maximum:\n%s", nodes)
	}
	_, err := Refresh(kb.OnDisk(root), log.Discard())
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 {
		t.Fatalf("refresh error = %v, want one refusal item", err)
	}
	if it := refusal[0]; it.Check != kb.CheckMetadata || it.Path != kb.GraphNodesFile || it.Key != kb.RigorKey {
		t.Errorf("refusal item = %+v, want a %s refusal on %s's %s", it, kb.CheckMetadata, kb.GraphNodesFile, kb.RigorKey)
	}
	if got := read(t, filepath.Join(root, kb.GraphNodesFile)); got != nodes {
		t.Errorf("refresh rewrote the node records:\n%s\nwant them as they were:\n%s", got, nodes)
	}
}

// TestRefreshRecordsTheExactStrength: a premise's own value reaches the record
// unrounded — 0.70 × 0.72 = 0.504 as itself, and the dependent of a value that
// needs more than four decimals as that value — and every page refresh writes
// stands fresh, while a hand-edited one is a register freshness finding.
func TestRefreshRecordsTheExactStrength(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(0.7), Rationale: "r."},
			{ID: "clm-bbbbbb", Title: "Claim B", Rigor: f(0.9), Rationale: "r."},
			{ID: "clm-cccccc", Title: "Claim C", Rigor: f(0.9), Rationale: "r."},
			{ID: "clm-dddddd", Title: "Claim D", Rigor: f(1), Rationale: "r."}},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "The rule"}},
		Founds: []kb.BearingEdge{founds("axm-s10000", "clm-aaaaaa", kb.Bearing{Value: 0.7})},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-aaaaaa", "clm-bbbbbb", 0.72),
			scorable(kb.RelationSupports, "clm-bbbbbb", "clm-cccccc", 0.72)},
	}
	root := kbWith(t, g, []string{"vol"}, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md": "[↑ Vol](index.md)\n\n# A\n\n" +
			"<!-- kbase-id: clm-aaaaaa -->\n\nBody A.\n\n<!-- kbase-id: clm-bbbbbb -->\n\nBody B.\n\n" +
			"<!-- kbase-id: clm-cccccc -->\n\nBody C.\n\n<!-- kbase-id: clm-dddddd -->\n\nBody D.\n",
		kb.AxiomsFile: "# Axioms\n\n" + kb.AxiomHeadingLine("S1", "The rule", "axm-s10000") + "\n\nBody.\n",
	})
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	register := read(t, filepath.Join(root, "vol", kb.RegisterFile))
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
	if err := os.WriteFile(filepath.Join(root, "vol/"+kb.RegisterFile), []byte(truncated), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := Verify(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Check != CheckRegisterFreshness || findings[0].Path != "vol/"+kb.RegisterFile || findings[0].Remedy != EmptyBatchRemedy {
		t.Errorf("a hand-truncated strength drew %+v, want one %s finding on vol/%s, remedy the empty batch", findings, CheckRegisterFreshness, kb.RegisterFile)
	}
}

// TestSheetWithoutDot: with no dot on PATH nothing is drawn, a sheet
// standing is left as it is, and the placeholder is written only where
// kb-root has none.
func TestSheetWithoutDot(t *testing.T) {
	root := smallKB(t)
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	sheetPath := filepath.Join(root, kb.ClaimGraphFile)
	if err := os.Remove(sheetPath); err != nil {
		t.Fatal(err)
	}
	written, removed, noDot, err := RefreshReporting(kb.OnDisk(root), log.Discard())
	if err != nil || !noDot || len(removed) > 0 || !slices.Equal(written, []string{kb.ClaimGraphFile}) {
		t.Fatalf("refresh with none standing = %q, %q, no dot %t, %v; want the placeholder written", written, removed, noDot, err)
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
// next refresh removes its register page and all three sheets and reports them
// removed.
func TestRefreshRemovesSheetsNoLongerCalledFor(t *testing.T) {
	leaf := func(up, body string) string { return "[↑ " + up + "](index.md)\n\n# A\n" + body }
	claimA := kb.ClaimNode{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(0.7), Rationale: "r."}
	root := kbWith(t, kb.Graph{Claims: []kb.ClaimNode{claimA, {ID: "clm-bbbbbb", Title: "Claim B", Rigor: f(0.7), Rationale: "r."}}},
		[]string{"vol", "wol"}, map[string]string{
			"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
			"vol/a.md":     leaf("Vol", "\nA. <!-- kbase-id: clm-aaaaaa -->\n"),
			"wol/index.md": "[↑ KB](../entry-point.md)\n\n# Wol\n\n- [a](a.md)\n",
			"wol/a.md":     leaf("Wol", "\nB. <!-- kbase-id: clm-bbbbbb -->\n"),
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
	for rel, text := range graphFiles(kb.Graph{Claims: []kb.ClaimNode{claimA}}) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "wol/a.md"), []byte(leaf("Wol", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	want := slices.Concat([]string{"wol/" + kb.RegisterFile}, sheets)
	if _, removed, _, err = RefreshReporting(kb.OnDisk(root), log.Discard()); err != nil || !slices.Equal(removed, want) {
		t.Errorf("second refresh removed %q (%v), want %q", removed, err, want)
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

// TestUpdateFormatForgetsAPageItRemoves: a 2.0.0 KB's claim sits in a leaf at
// kb-root with its register entry in the volume's 2.0.0 register. The
// converted KB renders the claim on kb-root's page, so update-format removes
// the volume's register, and the source it ran over no longer reads it as
// standing.
func TestUpdateFormatForgetsAPageItRemoves(t *testing.T) {
	const page = "vol/claim-rigor.md"
	root := writeKB(t, map[string]string{
		"entry-point.md": "---\nkind: entry-point\nkb-format: \"2.0.0\"\n---\n\n# KB\n\n- [vol](vol/index.md)\n- [leaf](leaf.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n",
		"leaf.md": "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ KB](entry-point.md)\n\n# Leaf\n\n" +
			"The result holds. <!-- claim-rigor: clm-aaaaaa -->\n",
		page: "# Claim Rigor\n\n---\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n> **Leaf references:** *(none)*\n\n" +
			"### Rigor\n- rigor: 0.5\n\n### Strength\n- strength: 0.5\n\n---\n\n",
		".index/claims.yaml": "--- {}\n",
	})
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatalf("opening the 2.0.0 KB: %v", err)
	}
	_, removed, _, err := UpdateFormat(src, nil, nil, log.Discard())
	if err != nil {
		t.Fatalf("update-format: %v", err)
	}
	if !slices.Contains(removed, page) {
		t.Errorf("update-format removed %q, want %s among them", removed, page)
	}
	if src.IsFile(src.KBPath(page)) {
		t.Errorf("the source still reads %s as standing after update-format removed it", page)
	}
}

// TestVerifyLeavesTheSheetAlone: a hand-edited or deleted sheet is no verify
// finding, and the build's check is the standard one: a bare id in prose is
// its fault, beside a dead link's.
func TestVerifyLeavesTheSheetAlone(t *testing.T) {
	root := smallKB(t)
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	verify := func(state string) []Finding {
		t.Helper()
		standard, err := Verify(kb.OnDisk(root))
		if err != nil {
			t.Fatalf("%s: Verify: %v", state, err)
		}
		return standard
	}

	sheetPath := filepath.Join(root, kb.ClaimGraphFile)
	placeholder, _ := os.ReadFile(sheetPath)
	if err := os.WriteFile(sheetPath, append(placeholder, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if found := verify("hand-edited sheet"); len(found) > 0 {
		t.Errorf("a hand-edited sheet: Verify found %v", found)
	}
	if err := os.Remove(sheetPath); err != nil {
		t.Fatal(err)
	}
	if found := verify("deleted sheet"); len(found) > 0 {
		t.Errorf("a deleted sheet: Verify found %v", found)
	}

	indexPath := filepath.Join(root, "vol/index.md")
	body, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, append(body, "\nThis follows from clm-aaaaaa.\n\n[gone](missing.md)\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := checks(verify("bare id and dead link")), []string{"dead-link", "bare-id"}; !slices.Equal(got, want) {
		t.Errorf("a bare id and a dead link: Verify checks %q, want %q", got, want)
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
	src := kb.MigratedSource(root, "1.0.0", map[string][]byte{"kb-root/vol/a.md": []byte(leaf)},
		[]string{"kb-root/.index/claims.jsonl", "kb-root/vol/a.md", "kb-root/vol/old.md"})
	_, removed, _, err := UpdateFormat(src, nil, nil, log.Discard())
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
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(kb.FormatFile)))
	if err != nil {
		t.Fatal(err)
	}
	if stamp, err := kb.DecodeFormatStamp(data); err != nil || stamp != kb.FormatVersion {
		t.Errorf("the save did not stamp the KB in %s: %q (%v)", kb.FormatFile, stamp, err)
	}
	if src.Migrated() {
		t.Error("the source still reads as migrated after the save")
	}
}

// TestRefreshRefusesASymlinkedRegisterPage: a derived file that is a relative
// symbolic link out of the tree is refused, and the file the link names keeps
// its bytes.
func TestRefreshRefusesASymlinkedRegisterPage(t *testing.T) {
	root := smallKB(t)
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(filepath.Dir(root)), "outside.md")
	if err := os.WriteFile(outside, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "vol", kb.RegisterFile)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../../outside.md", link); err != nil {
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
	converted := []byte("[↑ Vol](index.md)\n\n# A changed\n\nA claim. <!-- kbase-id: clm-aaaaaa -->\n")
	src := kb.MigratedSource(root, "1.0.0", map[string][]byte{"kb-root/vol/a.md": converted}, nil)
	_, _, _, err := UpdateFormat(src, nil, nil, log.Discard())
	var r result.Refusal
	if !errors.As(err, &r) || len(r) != 1 || r[0].Check != kb.CheckSymlink || r[0].Path != link {
		t.Fatalf("update-format over a symlinked document = %v, want one %s refusal naming %s", err, kb.CheckSymlink, link)
	}
	if got := read(t, filepath.Join(root, kb.EntryPointFile)); got != entry {
		t.Errorf("the entry point after a refused save =\n%s\nwant it unstamped", got)
	}
	if got := read(t, outside); got != "outside\n" {
		t.Errorf("%s = %q, want it untouched", outside, got)
	}
}

// TestMetadataFindings: a frontmatter block on a leaf, a claim with no
// statement marker and one with two; an axiom record with no heading carrying
// its marker, one whose marker ends two headings, a marked heading whose text
// is not its record's label and title, and a marked heading naming no axiom
// record; and a register page called for and missing are each one finding, in
// that order, after the link gate's unknown-id for the orphan marker, and an
// unmarked axiom heading none, each check named as the result table names it;
// the frontmatter block
// carries its remedy, the page the empty batch, and the markers and the axioms
// none. Refreshed, and with the block, the markers and the axioms put right,
// verify finds nothing.
func TestMetadataFindings(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(0.7)}, {ID: "clm-bbbbbb", Title: "B"}, {ID: "clm-cccccc", Title: "C"}},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "The rule"}, {ID: "axm-s20000", Label: "S2", Title: "Second"},
			{ID: "axm-s30000", Label: "S3", Title: "Third"}, {ID: "axm-s40000", Label: "S4", Title: "Fourth"}},
	}
	axiom := kb.AxiomHeadingLine
	leaf := "[↑ V](index.md)\n\n# Leaf\n\nA. <!-- kbase-id: clm-aaaaaa -->\n\nC. <!-- kbase-id: clm-cccccc -->\n"
	root := kbWith(t, g, []string{"v"}, map[string]string{
		"v/index.md": "[↑ KB](../entry-point.md)\n\n# V\n\n- [leaf](leaf.md)\n- [other](other.md)\n",
		"v/leaf.md":  "---\nkind: leaf\n---\n" + leaf,
		"v/other.md": "[↑ V](index.md)\n\n# Other\n\nC again. <!-- kbase-id: clm-cccccc -->\n",
		kb.AxiomsFile: "# Axioms\n\n" + axiom("S1", "The rule", "axm-s10000") + "\n\nBody.\n\n### Axiom S5: Stray\n\n" +
			axiom("S3", "Third", "axm-s30000") + "\n\n" + axiom("S3", "Third", "axm-s30000") + "\n\n" +
			axiom("S4", "Wrong", "axm-s40000") + "\n\n" + axiom("S9", "Orphan", "axm-s90000") + "\n",
	})
	findings, err := Verify(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range findings {
		it := f.Item()
		got = append(got, fmt.Sprintf("%s %s:%d %s", it.Check, it.Path, it.Line, it.Remedy))
	}
	want := []string{
		"unknown-id axioms.md:15 ",
		"frontmatter v/leaf.md:1 " + frontmatterRemedy,
		"statement-marker :0 ",
		"statement-marker v/leaf.md:10 ",
		"axiom-heading axioms.md:0 ",
		"axiom-heading axioms.md:9 ",
		"axiom-heading axioms.md:13 ",
		"axiom-heading axioms.md:15 ",
		"register-freshness v/" + kb.RegisterFile + ":0 " + EmptyBatchRemedy,
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings\n%q\nwant\n%q\n(%+v)", got, want, findings)
	}

	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	for rel, text := range map[string]string{
		"v/leaf.md":  leaf + "\nB. <!-- kbase-id: clm-bbbbbb -->\n",
		"v/other.md": "[↑ V](index.md)\n\n# Other\n",
		kb.AxiomsFile: "# Axioms\n\n" + axiom("S1", "The rule", "axm-s10000") + "\n\nBody.\n\n### Axiom S5: Stray\n\n" +
			axiom("S2", "Second", "axm-s20000") + "\n\n" + axiom("S3", "Third", "axm-s30000") + "\n\n" + axiom("S4", "Fourth", "axm-s40000") + "\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	if findings, err := Verify(kb.OnDisk(root)); err != nil || len(findings) > 0 {
		t.Errorf("verify over the KB put right: %+v %v, want none", findings, err)
	}
}

// TestDemotedFindings: every demoted record is one finding naming the edge and
// the relation it replaced, about its target's register page, or about no
// file where the target has no host.
func TestDemotedFindings(t *testing.T) {
	one := kb.Bearing{Value: 1}
	root := kbWith(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}},
		Demoted: []kb.DemotedEdge{{Source: "clm-bbbbbb", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationSupports, Bearing: one},
			{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Bearing: one}},
	}, []string{"v"}, map[string]string{
		"v/index.md": "[↑ KB](../entry-point.md)\n\n# V\n\n- [leaf](leaf.md)\n",
		"v/leaf.md":  "[↑ V](index.md)\n\n# Leaf\n\nA. <!-- kbase-id: clm-aaaaaa -->\n",
	})
	items, err := DemotedFindings(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range items {
		if it.Check != CheckDemoted || !strings.Contains(it.Detail, "demoted from "+kb.RelationSupports) {
			t.Errorf("item %+v, want a %s finding naming the replaced relation", it, CheckDemoted)
		}
		got[it.Detail[:strings.Index(it.Detail, ",")]] = it.Path
	}
	want := map[string]string{"clm-bbbbbb → clm-aaaaaa": "v/" + kb.RegisterFile, "clm-aaaaaa → clm-bbbbbb": ""}
	if !maps.Equal(got, want) {
		t.Errorf("demoted findings by edge and path %q, want %q", got, want)
	}
}

// TestRefreshRendersPagesAndSheetsOverTheSheetFixture: over a copy of the
// sheet fixture, converted to the current format by update-format, with its
// pages and sheets removed and a register standing in gamma, which hosts no node,
// refresh writes each page the KB calls for, fresh to verify, and the sheets,
// removes gamma's page and reports it, and writes no file under .index/; a
// second refresh writes and removes nothing.
func TestRefreshRendersPagesAndSheetsOverTheSheetFixture(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	if err := os.CopyFS(root, os.DirFS(sheetFixtureKB)); err != nil {
		t.Fatal(err)
	}
	converted, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := UpdateFormat(converted, nil, nil, log.Discard()); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{kb.ClaimGraphFile, kb.ClaimGraphDigestFile, "alpha/" + kb.ClaimGraphFile, "beta/" + kb.ClaimGraphFile} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	pages := []string{"alpha/" + kb.RegisterFile, "beta/" + kb.RegisterFile, kb.RegisterFile}
	for _, rel := range pages {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	stale := "gamma/" + kb.RegisterFile
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(stale)), []byte("# Claim register\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	indexDir := filepath.Join(root, kb.IndexDir)
	records := map[string]string{}
	for _, rel := range []string{kb.GraphNodesFile, kb.GraphEdgesFile, kb.FormatFile} {
		records[rel] = read(t, filepath.Join(root, filepath.FromSlash(rel)))
	}

	written, removed, noDot, err := RefreshReporting(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	sheets := []string{kb.ClaimGraphFile}
	if !noDot {
		sheets = append(sheets, kb.ClaimGraphDigestFile, "alpha/"+kb.ClaimGraphFile, "beta/"+kb.ClaimGraphFile)
	}
	if want := slices.Concat(pages, sheets); !slices.Equal(slices.Sorted(slices.Values(written)), slices.Sorted(slices.Values(want))) {
		t.Errorf("refresh wrote %q, want the pages and the sheets %q", written, want)
	}
	if !slices.Equal(removed, []string{stale}) || kb.IsFile(filepath.Join(root, filepath.FromSlash(stale))) {
		t.Errorf("refresh removed %q, want %s gone and reported", removed, stale)
	}
	if fresh, err := RegisterFreshness(kb.OnDisk(root)); err != nil || len(fresh) != 0 {
		t.Errorf("the pages refresh wrote are not fresh: %+v (%v)", fresh, err)
	}
	entries, err := os.ReadDir(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(records) {
		t.Errorf(".index/ holds %d files after refresh, want the %d records alone", len(entries), len(records))
	}
	for rel, want := range records {
		if got := read(t, filepath.Join(root, filepath.FromSlash(rel))); got != want {
			t.Errorf("refresh rewrote %s", rel)
		}
	}

	again, removedAgain, _, err := RefreshReporting(kb.OnDisk(root), log.Discard())
	if err != nil || len(again)+len(removedAgain) > 0 {
		t.Errorf("second refresh wrote %q and removed %q (%v), want nothing", again, removedAgain, err)
	}
}

// TestRefreshRefusesALoadError: a KB whose records hold a supports cycle is
// refused, a load error naming the edge file, and refresh writes nothing.
func TestRefreshRefusesALoadError(t *testing.T) {
	one := kb.Bearing{Value: 1}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(0.9)}, {ID: "clm-bbbbbb", Title: "B", Rigor: f(0.8)}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: one},
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}}},
	}
	files := graphFiles(g)
	for rel, text := range map[string]string{
		kb.EntryPointFile: "# KB\n\n- [v](v/index.md)\n",
		"v/index.md":      "[↑ KB](../entry-point.md)\n\n# V\n\n- [leaf](leaf.md)\n",
		"v/leaf.md":       "[↑ V](index.md)\n\n# Leaf\n\nA. <!-- kbase-id: clm-aaaaaa -->\n\nB. <!-- kbase-id: clm-bbbbbb -->\n",
	} {
		files[rel] = text
	}
	root := writeKB(t, files)
	written, removed, _, err := RefreshReporting(kb.OnDisk(root), log.Discard())
	var refused result.Refusal
	if !errors.As(err, &refused) || len(refused) != 1 || refused[0].Check != kb.CheckMetadata || refused[0].Path != kb.GraphEdgesFile {
		t.Errorf("refresh over a cycle = %v, want the load error naming %s", err, kb.GraphEdgesFile)
	}
	if len(written)+len(removed) > 0 {
		t.Errorf("refresh over a cycle wrote %q and removed %q, want nothing", written, removed)
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
