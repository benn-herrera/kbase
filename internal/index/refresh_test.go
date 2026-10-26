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
			"vol/a.md":     "[↑ Vol](index.md)\n\n# A\n\nA claim. <!-- claim-rigor: clm-aaaaaa -->\n",
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
	if want := []string{kb.ClaimGraphFile, "vol/claim-rigor.md"}; !slices.Equal(slices.Sorted(slices.Values(written)), want) {
		t.Errorf("first refresh wrote %q, want %q", written, want)
	}
	register := read(t, filepath.Join(root, "vol/claim-rigor.md"))
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

// TestRefreshLeavesAnAboveMaximumRigorInTheFile: a rigor standing above the
// rigor tables' maximum — a hand-edited 1.20 — reads at the maximum, and that
// is a reading rule: refresh derives the rows from 1.00 and leaves the node
// records holding `rigor: 1.20` byte for byte, the number it found never
// rewritten.
// TestADegradedRepairKeepsAnAboveMaximumRigor: the repair of a degraded load —
// here an edge naming a node no record carries — rewrites the edge records and
// leaves a node's rigor of 1.2 in the node records as the file held it.
func TestADegradedRepairKeepsAnAboveMaximumRigor(t *testing.T) {
	g := kb.Graph{
		Claims:      []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(1.2)}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Exp", Status: "run", Rigor: f(1.2)}},
		Scorable:    []kb.ScorableEdge{{Relation: kb.RelationLifts, Source: "exp-aaaaaa", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}}},
		References:  []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-cccccc"}},
	}
	root := kbWith(t, g, []string{"vol"}, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n- [exp](exp.md)\n",
		"vol/a.md":     "[↑ Vol](index.md)\n\n# A\n\nA claim. <!-- claim-rigor: clm-aaaaaa -->\n",
		"vol/exp.md":   "[↑ Vol](index.md)\n\n# Exp\n<!-- claim-rigor: exp-aaaaaa -->\n",
	})
	r, err := RefreshWithWarnings(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 || !slices.Contains(r.Written, kb.GraphEdgesFile) {
		t.Fatalf("refresh warned %v and wrote %q; want the dangling edge dropped from %s", r.Warnings, r.Written, kb.GraphEdgesFile)
	}
	if nodes := read(t, filepath.Join(root, kb.GraphNodesFile)); strings.Count(nodes, "rigor: 1.2\n") != 2 {
		t.Errorf("the repair rewrote a rigor above the maximum:\n%s", nodes)
	}
}

func TestRefreshLeavesAnAboveMaximumRigorInTheFile(t *testing.T) {
	g := kb.Graph{
		Claims:      []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(0.7)}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Exp", Status: "run", Rigor: f(1.2)}},
		Scorable:    []kb.ScorableEdge{{Relation: kb.RelationLifts, Source: "exp-aaaaaa", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}}},
	}
	root := kbWith(t, g, []string{"vol"}, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n- [exp](exp.md)\n",
		"vol/a.md":     "[↑ Vol](index.md)\n\n# A\n\nA claim. <!-- claim-rigor: clm-aaaaaa -->\n",
		"vol/exp.md":   "[↑ Vol](index.md)\n\n# Exp\n<!-- claim-rigor: exp-aaaaaa -->\n",
	})
	nodes := read(t, filepath.Join(root, kb.GraphNodesFile))
	if !strings.Contains(nodes, "rigor: 1.2\n") {
		t.Fatalf("the node records do not hold the rigor above the maximum:\n%s", nodes)
	}
	written, err := Refresh(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, kb.GraphNodesFile)); got != nodes {
		t.Errorf("refresh rewrote the node records:\n%s\nwant them as it found them:\n%s", got, nodes)
	}
	if slices.Contains(written, kb.GraphNodesFile) {
		t.Errorf("refresh reports the node records written; it wrote %q", written)
	}
	rows, err := Derive(kb.OnDisk(root))
	if err != nil {
		t.Fatalf("deriving the rows: %v", err)
	}
	for _, r := range rows.Nodes {
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
// needs more than four decimals as that value — and every page refresh writes
// stands fresh, while a hand-edited one is a register freshness finding.
func TestRefreshRecordsTheExactStrength(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(0.7), Rationale: "r."},
			{ID: "clm-bbbbbb", Title: "Claim B", Rigor: f(0.9), Rationale: "r."},
			{ID: "clm-cccccc", Title: "Claim C", Rigor: f(0.9), Rationale: "r."},
			{ID: "clm-dddddd", Title: "Claim D", Rigor: f(1), Rationale: "r."}},
		Axioms: []kb.AxiomNode{{ID: "axm-S1", Title: "The rule"}},
		Founds: []kb.BearingEdge{founds("axm-S1", "clm-aaaaaa", kb.Bearing{Value: 0.7})},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-aaaaaa", "clm-bbbbbb", 0.72),
			scorable(kb.RelationSupports, "clm-bbbbbb", "clm-cccccc", 0.72)},
	}
	root := kbWith(t, g, []string{"vol"}, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md": "[↑ Vol](index.md)\n\n# A\n\n" +
			"<!-- claim-rigor: clm-aaaaaa -->\n\nBody A.\n\n<!-- claim-rigor: clm-bbbbbb -->\n\nBody B.\n\n" +
			"<!-- claim-rigor: clm-cccccc -->\n\nBody C.\n\n<!-- claim-rigor: clm-dddddd -->\n\nBody D.\n",
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
	if len(findings) != 1 || findings[0].Check != CheckRegisterFreshness || findings[0].Path != "vol/claim-rigor.md" || !findings[0].RefreshFixable {
		t.Errorf("a hand-truncated strength drew %+v, want one refresh-fixable %s finding on vol/claim-rigor.md", findings, CheckRegisterFreshness)
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
// next refresh removes its register page and all three sheets and reports them
// removed.
func TestRefreshRemovesSheetsNoLongerCalledFor(t *testing.T) {
	leaf := func(up, body string) string { return "[↑ " + up + "](index.md)\n\n# A\n" + body }
	claimA := kb.ClaimNode{ID: "clm-aaaaaa", Title: "Claim A", Rigor: f(0.7), Rationale: "r."}
	root := kbWith(t, kb.Graph{Claims: []kb.ClaimNode{claimA, {ID: "clm-bbbbbb", Title: "Claim B", Rigor: f(0.7), Rationale: "r."}}},
		[]string{"vol", "wol"}, map[string]string{
			"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
			"vol/a.md":     leaf("Vol", "\nA. <!-- claim-rigor: clm-aaaaaa -->\n"),
			"wol/index.md": "[↑ KB](../entry-point.md)\n\n# Wol\n\n- [a](a.md)\n",
			"wol/a.md":     leaf("Wol", "\nB. <!-- claim-rigor: clm-bbbbbb -->\n"),
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

// TestRefreshForgetsAPageItRemoves: a 2.0.0 KB's claim sits in a leaf at
// kb-root with its register entry in the volume's page. The converted KB
// renders the claim on kb-root's page, so refresh removes the volume's page,
// and the source the refresh ran over no longer reads it as standing.
func TestRefreshForgetsAPageItRemoves(t *testing.T) {
	const page = "vol/" + kb.RegisterFile
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
	r, err := RefreshWithWarnings(src, log.Discard())
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !slices.Contains(r.Removed, page) {
		t.Errorf("refresh removed %q, want %s among them", r.Removed, page)
	}
	findings, err := RegisterFreshness(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Path == page {
			t.Errorf("register freshness after the refresh: %+v", f)
		}
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
	appendTo("\nThis follows from clm-aaaaaa.\n")
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
	src := kb.MigratedSource(root, "1.0.0", map[string][]byte{"kb-root/vol/a.md": []byte(leaf)},
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
	converted := []byte("[↑ Vol](index.md)\n\n# A changed\n\nA claim. <!-- claim-rigor: clm-aaaaaa -->\n")
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

// TestMetadataFindings: a frontmatter block on a leaf, a claim with no
// statement marker and one with two, an axiom record with no heading and a
// heading with no record, an edge naming no node and a register page called
// for and missing are each one finding, in that order; the frontmatter block
// carries its remedy, the degraded load's fault and the page refresh's, and
// the markers and the axioms none. Refreshed, and with the block, the markers
// and the axioms put right, verify finds nothing.
func TestMetadataFindings(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(0.7)}, {ID: "clm-bbbbbb", Title: "B"}, {ID: "clm-cccccc", Title: "C"}},
		Axioms: []kb.AxiomNode{{ID: "axm-S1", Title: "The rule"}, {ID: "axm-S2", Title: "Second"}},
		Founds: []kb.BearingEdge{founds("wrk-gone", "clm-aaaaaa", kb.Bearing{Value: 1})},
	}
	leaf := "[↑ V](index.md)\n\n# Leaf\n\nA. <!-- claim-rigor: clm-aaaaaa -->\n\nC. <!-- claim-rigor: clm-cccccc -->\n"
	root := kbWith(t, g, []string{"v"}, map[string]string{
		"v/index.md":  "[↑ KB](../entry-point.md)\n\n# V\n\n- [leaf](leaf.md)\n- [other](other.md)\n",
		"v/leaf.md":   "---\nkind: leaf\n---\n" + leaf,
		"v/other.md":  "[↑ V](index.md)\n\n# Other\n\nC again. <!-- claim-rigor: clm-cccccc -->\n",
		kb.AxiomsFile: "# Axioms\n\n### Axiom S1: The rule\n\nBody.\n\n### Axiom S3: Stray\n",
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
		CheckFrontmatterPresence + " v/leaf.md:1 " + frontmatterRemedy,
		CheckStatementMarker + " :0 ",
		CheckStatementMarker + " v/leaf.md:10 ",
		CheckAxiomHeading + " axioms.md:0 ",
		CheckAxiomHeading + " axioms.md:7 ",
		kb.CheckDegraded + " " + kb.GraphEdgesFile + ":0 " + RefreshRemedy,
		CheckRegisterFreshness + " v/claim-rigor.md:0 " + RefreshRemedy,
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings\n%q\nwant\n%q\n(%+v)", got, want, findings)
	}

	if _, err := Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatal(err)
	}
	for rel, text := range map[string]string{
		"v/leaf.md":   leaf + "\nB. <!-- claim-rigor: clm-bbbbbb -->\n",
		"v/other.md":  "[↑ V](index.md)\n\n# Other\n",
		kb.AxiomsFile: "# Axioms\n\n### Axiom S1: The rule\n\nBody.\n\n### Axiom S2: Second\n",
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
		"v/leaf.md":  "[↑ V](index.md)\n\n# Leaf\n\nA. <!-- claim-rigor: clm-aaaaaa -->\n",
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
// sheet fixture with its pages removed and a register standing in gamma,
// which hosts no node, refresh writes each page the KB calls for, fresh to
// verify, and the sheets, removes gamma's page and reports it, and writes no
// file under .index/; a second refresh writes and removes nothing.
func TestRefreshRendersPagesAndSheetsOverTheSheetFixture(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	if err := os.CopyFS(root, os.DirFS(sheetFixtureKB)); err != nil {
		t.Fatal(err)
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

	r, err := RefreshWithWarnings(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	sheets := []string{kb.ClaimGraphFile}
	if !r.NoDot {
		sheets = append(sheets, kb.ClaimGraphDigestFile, "alpha/"+kb.ClaimGraphFile, "beta/"+kb.ClaimGraphFile)
	}
	if want := slices.Concat(pages, sheets); !slices.Equal(slices.Sorted(slices.Values(r.Written)), slices.Sorted(slices.Values(want))) {
		t.Errorf("refresh wrote %q, want the pages and the sheets %q", r.Written, want)
	}
	if !slices.Equal(r.Removed, []string{stale}) || kb.IsFile(filepath.Join(root, filepath.FromSlash(stale))) {
		t.Errorf("refresh removed %q, want %s gone and reported", r.Removed, stale)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("the fixture's load is not degraded, and refresh warned %+v", r.Warnings)
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

	again, err := RefreshWithWarnings(kb.OnDisk(root), log.Discard())
	if err != nil || len(again.Written)+len(again.Removed) > 0 {
		t.Errorf("second refresh wrote %q and removed %q (%v), want nothing", again.Written, again.Removed, err)
	}
}

// TestRefreshRepairsADegradedLoad: a KB whose records hold an edge naming no
// node, an edge identity authored twice and a supports cycle is written back
// as the walk resolves it — the dangling edge and the second record gone, the
// cycle cut into a demoted record — with a warning per loss or cut; the
// records then load clean, and a second refresh writes nothing and warns of
// nothing.
func TestRefreshRepairsADegradedLoad(t *testing.T) {
	one := kb.Bearing{Value: 1}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(0.9)}, {ID: "clm-bbbbbb", Title: "B", Rigor: f(0.8)}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: one},
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}}},
		Founds: []kb.BearingEdge{{Source: "wrk-gone", Target: "clm-aaaaaa", Bearing: one}},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "first"},
			{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "second"}},
	}
	files := graphFiles(g)
	for rel, text := range map[string]string{
		kb.EntryPointFile: "# KB\n\n- [v](v/index.md)\n",
		"v/index.md":      "[↑ KB](../entry-point.md)\n\n# V\n\n- [leaf](leaf.md)\n",
		"v/leaf.md":       "[↑ V](index.md)\n\n# Leaf\n\nA. <!-- claim-rigor: clm-aaaaaa -->\n\nB. <!-- claim-rigor: clm-bbbbbb -->\n",
	} {
		files[rel] = text
	}
	root := writeKB(t, files)
	degraded, err := Derive(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(degraded.Degraded) != 3 {
		t.Fatalf("the fixture loads with warnings %+v, want the dangling edge, the second record and the cut", degraded.Degraded)
	}

	r, err := RefreshWithWarnings(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.Written, kb.GraphEdgesFile) || !slices.Contains(r.Written, "v/"+kb.RegisterFile) {
		t.Errorf("refresh wrote %q, want the edge records and v's page", r.Written)
	}
	var warned, want []string
	for _, w := range r.Warnings {
		warned = append(warned, w.Path+": "+w.Detail)
	}
	for _, w := range degraded.Degraded {
		want = append(want, w.Path+": "+w.Detail)
	}
	if !slices.Equal(warned, want) {
		t.Errorf("refresh warned %q, want a warning per loss or cut %q", warned, want)
	}
	repaired, err := kb.LoadGraph(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	cut := kb.DemotedEdge{Source: "clm-bbbbbb", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Value: 0.5}}
	if len(repaired.Founds) != 0 || !slices.Equal(repaired.References, g.References[:1]) ||
		!slices.Equal(repaired.Scorable, g.Scorable[:1]) || !slices.Equal(repaired.Demoted, []kb.DemotedEdge{cut}) {
		t.Errorf("the repaired records are %+v, want the dangling founds gone, the first references kept and the cycle cut", repaired)
	}
	if rows, err := Derive(kb.OnDisk(root)); err != nil || len(rows.Degraded) != 0 {
		t.Errorf("the repaired KB loads with warnings %+v (%v), want none", rows.Degraded, err)
	}

	again, err := RefreshWithWarnings(kb.OnDisk(root), log.Discard())
	if err != nil || len(again.Written)+len(again.Removed)+len(again.Warnings) > 0 {
		t.Errorf("second refresh wrote %q, removed %q and warned %+v (%v), want nothing", again.Written, again.Removed, again.Warnings, err)
	}
}

// TestARepairBesideAStandingDemotedRecordLoadsClean: over the cycle
// clm-aaaaaa ⇄ clm-bbbbbb, each edge already standing beside its demoted
// record, one repair cuts an edge and writes no second demoted record of its
// identity, so the repaired KB loads clean with both demoted records as they
// stood.
func TestARepairBesideAStandingDemotedRecordLoadsClean(t *testing.T) {
	half, third := kb.Bearing{Value: 0.5}, kb.Bearing{Value: 0.3}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: half},
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: half}},
		Demoted: []kb.DemotedEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Bearing: third},
			{Source: "clm-bbbbbb", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationSupports, Bearing: third}},
	}
	root := kbWith(t, g, nil, nil)
	r, err := RefreshWithWarnings(kb.OnDisk(root), log.Discard())
	if err != nil || len(r.Warnings) != 1 {
		t.Fatalf("refresh warned %+v (%v), want the one cut", r.Warnings, err)
	}
	if rows, err := Derive(kb.OnDisk(root)); err != nil || len(rows.Degraded) != 0 {
		t.Errorf("the repaired KB loads with warnings %+v (%v), want none", rows.Degraded, err)
	}
	repaired, err := kb.LoadGraph(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(repaired.Scorable) != 1 || !slices.Equal(repaired.Demoted, g.Demoted) {
		t.Errorf("the repaired records are %+v and %+v, want one edge left and the demoted records as they stood", repaired.Scorable, repaired.Demoted)
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
