package main

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/export"
	"kbase/internal/kb"
	"kbase/internal/latex/pandoc"
)

// exportDir is a repository holding a stamped KB of two volumes, the first
// one's spine naming a missing child (a walk finding) and its leaf citing a
// key with no entry and linking a missing document (two filter findings).
func exportDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"entry-point.md": "---\nkind: entry-point\nkb-format: \"" + kb.FormatVersion + "\"\n---\n\n# Sample KB\n\nProse.\n\n## Volumes\n\n" +
			"- [First](vol1/index.md) — the first volume\n- [Second](vol2/index.md) — the second volume\n",
		"vol1/index.md": "---\nkind: index\n---\n[↑ Sample KB](../entry-point.md)\n\n# First Volume\n\n" +
			"- [The Leaf](leaf.md) — a leaf\n- [Missing](missing.md) — names no file\n",
		"vol1/leaf.md": "---\nkind: leaf\nno-claim: true\n---\n[↑ First Volume](index.md)\n\n# The Leaf\n\n" +
			"Citing <span class=\"citation\" data-cites=\"nokey\">(Nobody 1999)</span> and [a dead link](gone.md).\n",
		"vol2/index.md": "---\nkind: index\n---\n[↑ Sample KB](../entry-point.md)\n\n# Second Volume\n\nProse alone.\n",
	}
	for rel, text := range files {
		writeFileAt(t, filepath.Join(dir, kb.KBDir), rel, text)
	}
	writeFileAt(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	return dir
}

func writeFileAt(t *testing.T, dir, rel, text string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// dirBytes is every file under dir, by slash path, with its bytes; none
// where dir does not exist.
func dirBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if os.IsNotExist(err) && p == dir {
			return filepath.SkipDir
		}
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func requirePandoc(t *testing.T) {
	t.Helper()
	if _, err := pandoc.Preflight(context.Background()); err != nil {
		t.Skipf("pandoc is unusable here: %v", err)
	}
}

// TestExportLatexRefusesUnknownVolumes: every --volume naming no volume of
// the entry point's list is one usage item, and nothing is written.
func TestExportLatexRefusesUnknownVolumes(t *testing.T) {
	dir := exportDir(t)
	out, code := kbase(t, dir, "export-latex", "--volume", "vol2", "--volume", "nope", "--volume", "vol9")
	d := checkDocument(t, "export-latex", out)
	items := d.items("refusals")
	if code != 1 || d.Outcome != "refused" || len(items) != 2 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for i, name := range []string{"nope", "vol9"} {
		if items[i]["check"] != checkUsage || items[i]["key"] != "--volume" || !strings.Contains(items[i]["detail"].(string), name) {
			t.Errorf("item %d %v, want a usage refusal of --volume naming %s", i, items[i], name)
		}
	}
	if got := dirBytes(t, filepath.Join(dir, exportOutDefault)); len(got) != 0 {
		t.Errorf("a refused export wrote %v", slices.Sorted(maps.Keys(got)))
	}
}

// TestExportLatexRefusesWithoutPandoc: with no pandoc on PATH the export is
// refused with the build's pandoc item, and nothing is written.
func TestExportLatexRefusesWithoutPandoc(t *testing.T) {
	dir := exportDir(t)
	t.Setenv("PATH", t.TempDir())
	out, code := kbase(t, dir, "export-latex")
	d := checkDocument(t, "export-latex", out)
	items := d.items("refusals")
	if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != "pandoc" ||
		!strings.Contains(items[0]["detail"].(string), pandoc.InstallPage) {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := dirBytes(t, filepath.Join(dir, exportOutDefault)); len(got) != 0 {
		t.Errorf("a refused export wrote %v", slices.Sorted(maps.Keys(got)))
	}
}

// TestExportLatexRefusesForeignGenTex: a gen-tex/ holding files and no
// manifest is refused naming it, with the remedy, and nothing is written.
func TestExportLatexRefusesForeignGenTex(t *testing.T) {
	requirePandoc(t)
	dir := exportDir(t)
	outDir := filepath.Join(dir, "paper")
	writeFileAt(t, outDir, export.GenTexDir+"/notes.txt", "mine")
	out, code := kbase(t, dir, "export-latex", "--out", "paper")
	d := checkDocument(t, "export-latex", out)
	items := d.items("refusals")
	gen := filepath.Join(outDir, export.GenTexDir)
	if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkExportDir ||
		items[0]["path"] != gen || items[0]["remedy"] != "empty the directory or name another --out" {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := dirBytes(t, outDir); len(got) != 1 || got[export.GenTexDir+"/notes.txt"] != "mine" {
		t.Errorf("the refused out directory holds %v, want gen-tex/notes.txt alone", got)
	}
}

// TestExportLatexReissue: a first export is done, its keys in order, writing
// pandoc's outputs, the report and the starter preamble, and reporting the
// volumes exported; the report holds the walk's finding, the filter's and
// the preamble's line. A re-issue writes nothing, leaves every byte as it was
// and reports unchanged.
func TestExportLatexReissue(t *testing.T) {
	requirePandoc(t)
	dir := exportDir(t)
	outDir := filepath.Join(dir, exportOutDefault)
	gen := filepath.Join(outDir, export.GenTexDir)
	out, code := kbase(t, filepath.Join(dir, kb.KBDir, "vol1"), "export-latex")
	d := checkDocument(t, "export-latex", out)
	if code != 0 || d.Outcome != "done" {
		t.Fatalf("first export: exit %d\n%s", code, out)
	}
	if !slices.Equal(d.Keys, []string{"outcome", "kb-root", "written", "volumes", "report"}) {
		t.Errorf("keys %q", d.Keys)
	}
	if d.Values["kb-root"] != filepath.Join(dir, kb.KBDir) {
		t.Errorf("kb-root %v", d.Values["kb-root"])
	}
	written := anyStrings(d.Values["written"])
	for _, p := range []string{filepath.Join(gen, export.OutputFile), filepath.Join(gen, export.FindingsFile),
		filepath.Join(gen, export.ReportFile), filepath.Join(gen, export.DefaultsFile), filepath.Join(outDir, export.PreambleFile)} {
		if !slices.Contains(written, p) {
			t.Errorf("written lacks %s: %q", p, written)
		}
	}
	if !slices.Equal(anyStrings(d.Values["volumes"]), []string{"vol1", "vol2"}) {
		t.Errorf("volumes %v", d.Values["volumes"])
	}
	if d.Values["report"] != filepath.Join(gen, export.ReportFile) {
		t.Errorf("report %v", d.Values["report"])
	}
	report := readExportReport(t, gen)
	var forms []string
	for _, f := range report {
		forms = append(forms, f.Form+" "+f.Path)
	}
	if want := []string{"walk vol1/index.md", "citation vol1/leaf.md", "link vol1/leaf.md", "preamble " + filepath.Join(outDir, export.PreambleFile)}; !slices.Equal(forms, want) {
		t.Errorf("report lines %q, want %q", forms, want)
	}

	before := dirBytes(t, outDir)
	out, code = kbase(t, dir, "export-latex")
	d = checkDocument(t, "export-latex", out)
	if code != 0 || d.Outcome != "unchanged" || len(anyStrings(d.Values["written"])) != 0 {
		t.Errorf("re-issue: exit %d\n%s", code, out)
	}
	if after := dirBytes(t, outDir); !mapsEqualStrings(before, after) {
		t.Errorf("the re-issue changed the export")
	}
}

// TestExportLatexKeepsTheAuthorsPreamble: a standing preamble.tex is not
// written, and the report names it as the author's; --volume narrows the
// export to the volume named.
func TestExportLatexKeepsTheAuthorsPreamble(t *testing.T) {
	requirePandoc(t)
	dir := exportDir(t)
	outDir := filepath.Join(dir, "paper")
	const mine = "\\newcommand{\\R}{\\mathbb{R}}\n"
	writeFileAt(t, outDir, export.PreambleFile, mine)
	out, code := kbase(t, dir, "export-latex", "--out", outDir, "--volume", "vol2")
	d := checkDocument(t, "export-latex", out)
	if code != 0 || d.Outcome != "done" || !slices.Equal(anyStrings(d.Values["volumes"]), []string{"vol2"}) {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if slices.Contains(anyStrings(d.Values["written"]), filepath.Join(outDir, export.PreambleFile)) {
		t.Errorf("the author's preamble is reported written")
	}
	if got := dirBytes(t, outDir)[export.PreambleFile]; got != mine {
		t.Errorf("the author's preamble changed: %q", got)
	}
	report := readExportReport(t, filepath.Join(outDir, export.GenTexDir))
	if len(report) == 0 || report[len(report)-1].Form != "preamble" || report[len(report)-1].Path != filepath.Join(outDir, export.PreambleFile) {
		t.Errorf("report %+v, want it to close on the preamble's line", report)
	}
	for _, f := range report {
		if f.Path == "vol1/leaf.md" {
			t.Errorf("report line %+v: vol1's leaf was not exported", f)
		}
	}
}

// TestExportLatexRefusesOutsideAWorktree: the export finds its KB as the
// queries do, so a kb-root/ with no .git beside it is the queries' worktree
// refusal.
func TestExportLatexRefusesOutsideAWorktree(t *testing.T) {
	dir := exportDir(t)
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	out, code := kbase(t, dir, "export-latex")
	d := checkDocument(t, "export-latex", out)
	items := d.items("refusals")
	if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkWorktree {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func readExportReport(t *testing.T, gen string) []export.Finding {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(gen, export.ReportFile))
	if err != nil {
		t.Fatal(err)
	}
	var report []export.Finding
	if err := yaml.Unmarshal(data, &report); err != nil {
		t.Fatalf("%s: %v\n%s", export.ReportFile, err, data)
	}
	return report
}

// TestMCPExportLatexTool: export-latex is a writing tool whose arguments are
// its flags.
func TestMCPExportLatexTool(t *testing.T) {
	for _, tool := range listTools(t) {
		if tool.Name != "export-latex" {
			continue
		}
		if tool.Annotations["readOnlyHint"] {
			t.Errorf("export-latex is marked read-only")
		}
		if tool.InputSchema.Properties["out"]["type"] != "string" || tool.InputSchema.Properties["volume"]["type"] != "array" ||
			len(tool.InputSchema.Properties) != 2 {
			t.Errorf("export-latex schema %+v, want out and volume", tool.InputSchema)
		}
		return
	}
	t.Error("no export-latex tool")
}
