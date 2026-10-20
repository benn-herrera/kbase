package export

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
)

// fig is an asset's bytes, not valid UTF-8, so a copy that decodes text
// would not keep them.
var fig = []byte{0x89, 'P', 'N', 'G', 0xff, 0x00, 0x01}

// testVersion is the kbase version the tests assemble under.
const testVersion = "9.9.9-test"

// exportKB is smallKB with what the export reads beyond the walk: a theorem
// and a lemma block, a label line inside a code fence, citations, vol1's
// references.md entries, the root register's work entry, a document with no
// heading, and assets inside and outside the volumes.
func exportKB() map[string]string {
	files := smallKB()
	files["vol1/v1-leaf.md"] = "---\nkind: leaf\nno-claim: true\n---\n[↑ First Volume](index.md)\n\n# §1 The Leaf\n\n" +
		"Body text with a cross-reference to [the other leaf](../vol2/v2-leaf.md), citing " +
		"<span class=\"citation\" data-cites=\"k1\">(Author 2001)</span>.\n\n" +
		"> <span id=\"thm:a\">**Theorem**</span>\n>\n> **Theorem 1**. Every leaf is a leaf.\n\n" +
		"```\n> **Fenced**\n```\n"
	files["vol1/references.md"] = "---\nkind: leaf\n---\n[↑ First Volume](index.md)\n\n# References\n\n" +
		"<div id=\"refs\" class=\"references csl-bib-body hanging-indent\">\n\n" +
		"<div id=\"ref-k1\" class=\"csl-entry\">\n\nAuthor, A. 2001. *Title One*.\n\n</div>\n\n</div>\n"
	files["vol2/index.md"] = "---\nkind: index\n---\n[↑ A KB](../entry-point.md)\n\n# Second Volume\n\n" +
		"- [The Second Leaf](v2-leaf.md) — a leaf\n- [No Heading](v2-noheading.md) — a leaf with no heading\n"
	files["vol2/v2-leaf.md"] = "---\nkind: leaf\n---\n[↑ Second Volume](index.md)\n\n# §1 The Second Leaf\n\n" +
		"> **Lemma**\n>\n> **Lemma 1**. Body cites <span class=\"citation\" data-cites=\"smith2020\">(Smith)</span>.\n"
	files["vol2/v2-noheading.md"] = "---\nkind: leaf\n---\n[↑ Second Volume](index.md)\n\nJust prose.\n"
	files["claim-rigor.md"] = "# Register\n\n## Smith, *A Work*\n<!-- id: wrk-smith2020 -->\n\n### Rigor\n- rigor: 0.70\n- rationale: sound.\n"
	files["vol1/fig.png"] = string(fig)
	files["vol1/nested/data.csv"] = "a,b\n"
	files["logo.png"] = "outside every volume"
	// The claim-graph sheets refresh writes, and an author's file of a
	// sheet's name deeper in a volume, which is no sheet.
	files["claim-graph.svg"] = "<svg>full sheet</svg>"
	files["claim-graph-digest.svg"] = "<svg>digest</svg>"
	files["vol1/claim-graph.svg"] = "<svg>volume sheet</svg>"
	files["vol1/nested/claim-graph.svg"] = "<svg>the author's</svg>"
	return files
}

// assembleKB walks and assembles the KB files describes, failing on any walk
// finding: the fixtures are clean trees.
func assembleKB(t *testing.T, files map[string]string, volumes ...string) map[string][]byte {
	t.Helper()
	src := writeKB(t, files)
	rep := &Report{}
	tree, err := Walk(src, rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !rep.Empty() {
		t.Fatalf("Walk reported %v over a clean tree", rep.Findings())
	}
	if tree, err = SelectVolumes(tree, volumes); err != nil {
		t.Fatalf("SelectVolumes: %v", err)
	}
	out, err := Assemble(src, tree, testVersion)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	return out
}

func tablesOf(t *testing.T, files map[string][]byte) xrefTables {
	t.Helper()
	var x xrefTables
	if err := json.Unmarshal(files[xrefFile], &x); err != nil {
		t.Fatalf("%s: %v\n%s", xrefFile, err, files[xrefFile])
	}
	return x
}

// streamOf is the stream: the files the tables' streams name, in order,
// failing where one is missing.
func streamOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var out []byte
	for _, name := range tablesOf(t, files).Streams {
		data, ok := files[name]
		if !ok {
			t.Fatalf("the tables name stream file %s, which the set lacks", name)
		}
		out = append(out, data...)
	}
	return out
}

// headingLines is every line of the stream opening with #.
func headingLines(stream []byte) []string {
	var out []string
	for line := range strings.SplitSeq(string(stream), "\n") {
		if strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

func TestAssembleDeterministic(t *testing.T) {
	first := assembleKB(t, exportKB())
	second := assembleKB(t, exportKB())
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two assemblies of one KB differ")
	}
}

// TestAssembleStream: the entry point's prose leads, its title and volume
// list gone; then each volume's index and spine in walk order, navigation
// stripped, each first heading carrying its label, a heading-less document
// given one from its path. The front matter and each volume are files of
// their own, which the defaults file reads in that order, file-scoped.
func TestAssembleStream(t *testing.T) {
	files := assembleKB(t, exportKB())
	streams := []string{"kbase-export-front.md", "kbase-export-vol1.md", "kbase-export-vol2.md"}
	if got := tablesOf(t, files).Streams; !slices.Equal(got, streams) {
		t.Errorf("streams %q, want %q", got, streams)
	}
	var defaults pandocDefaults
	if err := yaml.Unmarshal(files[DefaultsFile], &defaults); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(defaults.InputFiles, streams) || !defaults.FileScope {
		t.Errorf("defaults.yaml input-files %q, file-scope %v; want %q, true", defaults.InputFiles, defaults.FileScope, streams)
	}
	if !strings.HasPrefix(string(files["kbase-export-vol2.md"]), "# Second Volume {#vol2}") {
		t.Errorf("kbase-export-vol2.md does not open with its volume's index:\n%s", files["kbase-export-vol2.md"])
	}
	stream := streamOf(t, files)
	want := []string{
		"## What this KB contains {.unnumbered}",
		"## Key Results {.unnumbered}",
		"# First Volume {#vol1}",
		"## Key Results",
		"## Derivations",
		"# §1 The Leaf {#d1-v1-leaf}",
		"# Nested {#d1-nested-index}",
		"# Deep {#d1-nested-deep}",
		"# Overview {#d1-overview}",
		"# Second Volume {#vol2}",
		"# §1 The Second Leaf {#d2-v2-leaf}",
		"# v2-noheading {#d2-v2-noheading}",
	}
	if got := headingLines(stream); !slices.Equal(got, want) {
		t.Errorf("heading lines\n%q\nwant\n%q\nstream:\n%s", got, want, stream)
	}
	s := string(stream)
	for _, absent := range []string{"# A KB", "[↑", "kind:", "— the first volume", "— a leaf", "— one rank deeper", "Entry one"} {
		if strings.Contains(s, absent) {
			t.Errorf("the stream carries %q:\n%s", absent, s)
		}
	}
	order := []string{"A blurb about the KB.", "Prose carrying a link to the first leaf", "# §1 The Leaf", "Deeper body.", "Just prose."}
	at := -1
	for _, w := range order {
		i := strings.Index(s, w)
		if i <= at {
			t.Errorf("%q is missing or out of order:\n%s", w, s)
		}
		at = i
	}
	if !strings.Contains(string(files[DefaultsFile]), "title: A KB\n") {
		t.Errorf("defaults.yaml does not carry the entry point's title:\n%s", files[DefaultsFile])
	}
	if !strings.Contains(string(files[DefaultsFile]), "variables:\n  kbase-version: "+testVersion+"\n") {
		t.Errorf("defaults.yaml does not set kbase-version among the template variables:\n%s", files[DefaultsFile])
	}
}

// TestFrontMatter: the entry point's first heading is the title and leaves
// the prose; a heading whose section held only the volume list, blank lines
// and headings emptied the same way goes with the list; every other heading
// stays, marked unnumbered.
func TestFrontMatter(t *testing.T) {
	for _, tc := range []struct {
		name, text, prose, title string
	}{
		{
			"0.9.0-built: title over the list",
			"---\nkind: entry-point\n---\n# Knowledge Base\n\n- [A Paper](a-paper/index.md)\n",
			"", "Knowledge Base",
		},
		{
			"overview-drafted: a Volumes section",
			"---\nkind: entry-point\n---\n# A KB\n\nBlurb.\n\n## Key Results\n\nResult.\n\n## Volumes\n\n- [First](vol1/index.md) — the first volume\n- [Second](vol2/index.md) — the second volume\n",
			"\nBlurb.\n\n## Key Results {.unnumbered}\n\nResult.\n\n", "A KB",
		},
		{
			"nested navigation sections go together",
			"# T\n\n## Volumes\n\n### Part A\n\n- [A](a/index.md)\n\n### Part B\n\n- [B](b/index.md)\n\n## After\n\nText.\n",
			"\n\n\n\n\n\n## After {.unnumbered}\n\nText.", "T",
		},
		{
			"prose beside the list keeps its heading; an authored empty heading stays",
			"# T\n\n## Contents\n\nRead these:\n\n- [A](a/index.md)\n\n## Notes\n",
			"\n## Contents {.unnumbered}\n\nRead these:\n\n\n## Notes {.unnumbered}", "T",
		},
		{
			"a heading in a code fence is no heading",
			"# T\n\n```\n# not a heading\n```\n",
			"\n```\n# not a heading\n```", "T",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prose, title := frontMatter("entry-point.md", tc.text)
			if prose != tc.prose || title != tc.title {
				t.Errorf("frontMatter = %q, %q\nwant %q, %q", prose, title, tc.prose, tc.title)
			}
		})
	}
}

func TestAssembleTables(t *testing.T) {
	files := assembleKB(t, exportKB())
	x := tablesOf(t, files)
	want := xrefTables{
		Documents: map[string]xrefDocument{
			"vol1":            {Path: "vol1/index.md", Volume: 1, Rank: 0},
			"d1-v1-leaf":      {Path: "vol1/v1-leaf.md", Volume: 1, Rank: 1},
			"d1-nested-index": {Path: "vol1/nested/index.md", Volume: 1, Rank: 1},
			"d1-nested-deep":  {Path: "vol1/nested/deep.md", Volume: 1, Rank: 2},
			"d1-overview":     {Path: "vol1/overview.md", Volume: 1, Rank: 1},
			"vol2":            {Path: "vol2/index.md", Volume: 2, Rank: 0},
			"d2-v2-leaf":      {Path: "vol2/v2-leaf.md", Volume: 2, Rank: 1},
			"d2-v2-noheading": {Path: "vol2/v2-noheading.md", Volume: 2, Rank: 1},
		},
		Volumes:  []xrefVolume{{Label: "vol1", Title: "First Volume"}, {Label: "vol2", Title: "Second Volume"}},
		Theorems: []string{"Lemma", "Theorem"},
		Bibliography: map[string]string{
			"k1":        "Author, A. 2001. *Title One*.",
			"smith2020": "Smith, *A Work*",
		},
		Report: FindingsFile,
		Assets: map[string]string{
			"vol1/fig.png":                "assets/vol1/fig.png",
			"vol1/nested/data.csv":        "assets/vol1/nested/data.csv",
			"vol1/nested/claim-graph.svg": "assets/vol1/nested/claim-graph.svg",
		},
		EntryPoint: "entry-point.md",
		Streams:    []string{"kbase-export-front.md", "kbase-export-vol1.md", "kbase-export-vol2.md"},
	}
	want.Paths = map[string]string{}
	for id, d := range want.Documents {
		want.Paths[d.Path] = id
	}
	if !reflect.DeepEqual(x, want) {
		t.Errorf("tables\n%+v\nwant\n%+v", x, want)
	}
	var keys map[string]any
	if err := json.Unmarshal(files[xrefFile], &keys); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"documents", "paths", "volumes", "theorems", "bibliography", "report", "assets", "entry_point", "streams"} {
		if v, ok := keys[key]; !ok || v == nil {
			t.Errorf("the tables carry no %s table: %s", key, files[xrefFile])
		}
	}
}

// TestAssembleTablesEmpty: a KB with nothing to cite, label or copy still
// carries every table, each empty rather than null.
func TestAssembleTablesEmpty(t *testing.T) {
	files := assembleKB(t, map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n# Empty\n\n- [V](v/index.md) — the volume\n",
		"v/index.md":     "---\nkind: index\n---\n[↑ Empty](../entry-point.md)\n\n# V\n",
	})
	s := string(files[xrefFile])
	for _, want := range []string{`"theorems": []`, `"bibliography": {}`, `"assets": {}`} {
		if !strings.Contains(s, want) {
			t.Errorf("the tables do not carry %s:\n%s", want, s)
		}
	}
}

// TestAssembleBibliographySources: with every exported volume carrying
// references.md entries, the register's works do not join the bibliography;
// with one that carries none, they do.
func TestAssembleBibliographySources(t *testing.T) {
	if got := tablesOf(t, assembleKB(t, exportKB(), "vol1")).Bibliography; !reflect.DeepEqual(got, map[string]string{"k1": "Author, A. 2001. *Title One*."}) {
		t.Errorf("vol1 alone: bibliography %v, want its references.md entry only", got)
	}
	if got := tablesOf(t, assembleKB(t, exportKB(), "vol2")).Bibliography; !reflect.DeepEqual(got, map[string]string{"smith2020": "Smith, *A Work*"}) {
		t.Errorf("vol2 alone: bibliography %v, want the register's work only", got)
	}
}

// TestAssembleVolumeSelection: a selection exports its volumes alone,
// numbered by their place among them; a name that is no volume refuses,
// naming every such name.
func TestAssembleVolumeSelection(t *testing.T) {
	files := assembleKB(t, exportKB(), "vol2/")
	x := tablesOf(t, files)
	if !reflect.DeepEqual(x.Volumes, []xrefVolume{{Label: "vol1", Title: "Second Volume"}}) {
		t.Errorf("volumes %+v, want the second volume as vol1", x.Volumes)
	}
	if got := x.Paths["vol2/v2-leaf.md"]; got != "d1-v2-leaf" {
		t.Errorf("vol2/v2-leaf.md is %q, want d1-v2-leaf", got)
	}
	if _, ok := x.Paths["vol1/v1-leaf.md"]; ok {
		t.Errorf("an unselected volume's document is in the tables: %v", x.Paths)
	}
	if stream := streamOf(t, files); strings.Contains(string(stream), "First Volume") {
		t.Errorf("the stream carries the unselected volume:\n%s", stream)
	}
	if !slices.Equal(x.Streams, []string{"kbase-export-front.md", "kbase-export-vol1.md"}) {
		t.Errorf("streams %q, want the front matter and the one selected volume, as vol1", x.Streams)
	}
	if _, ok := files["assets/vol1/fig.png"]; ok {
		t.Errorf("an unselected volume's asset was copied")
	}

	src := writeKB(t, exportKB())
	tree, err := Walk(src, &Report{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = SelectVolumes(tree, []string{"vol2", "nope", "vol9"})
	var unknown UnknownVolumesError
	if !errors.As(err, &unknown) || !slices.Equal(unknown.Names, []string{"nope", "vol9"}) {
		t.Errorf("SelectVolumes with unknown volumes = %v, want UnknownVolumesError naming nope and vol9", err)
	}
}

// TestAssembleAssets: every non-Markdown file under an exported volume is
// copied byte for byte under assets/ at its kb-root-relative path, and named
// in the tables' assets; a file outside every volume is not, nor is a
// volume's claim-graph sheet, while a file of that name deeper in a volume is.
func TestAssembleAssets(t *testing.T) {
	files := assembleKB(t, exportKB())
	if got := files["assets/vol1/fig.png"]; !slices.Equal(got, fig) {
		t.Errorf("assets/vol1/fig.png = %v, want %v", got, fig)
	}
	if got := string(files["assets/vol1/nested/data.csv"]); got != "a,b\n" {
		t.Errorf("assets/vol1/nested/data.csv = %q", got)
	}
	var copied []string
	for p := range files {
		if strings.HasPrefix(p, assetsDir+"/") {
			copied = append(copied, p)
		}
	}
	slices.Sort(copied)
	want := []string{"assets/vol1/fig.png", "assets/vol1/nested/claim-graph.svg", "assets/vol1/nested/data.csv"}
	if !slices.Equal(copied, want) {
		t.Errorf("assets %q, want %q", copied, want)
	}
	if named := slices.Sorted(maps.Values(tablesOf(t, files).Assets)); !slices.Equal(named, copied) {
		t.Errorf("the tables' assets name %q, the copies are %q", named, copied)
	}
}

// snapshot is every file under dir, by out-relative slash path, with its
// bytes and modification time.
type snapshotFile struct {
	data []byte
	mod  time.Time
}

func snapshot(t *testing.T, dir string) map[string]snapshotFile {
	t.Helper()
	out := map[string]snapshotFile{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = snapshotFile{data: data, mod: info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// writeFile writes data at the slash path rel under dir, making its parents.
func writeFile(t *testing.T, dir, rel, data string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWriteOutRefusesForeignGenTex: a gen-tex/ holding files and no manifest
// refuses, and nothing is written, the preamble included.
func TestWriteOutRefusesForeignGenTex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, GenTexDir+"/notes.txt", "mine")
	_, _, _, err := WriteOut(dir, map[string][]byte{"a.txt": []byte("a")})
	var refusal ExportDirError
	if !errors.As(err, &refusal) || refusal.Dir != filepath.Join(dir, GenTexDir) {
		t.Fatalf("WriteOut over a foreign gen-tex/ = %v, want ExportDirError naming it", err)
	}
	if got := slices.Sorted(maps.Keys(snapshot(t, dir))); !slices.Equal(got, []string{GenTexDir + "/notes.txt"}) {
		t.Errorf("the refused directory holds %q, want gen-tex/notes.txt alone", got)
	}
}

// TestWriteOutBesideAuthorFiles: files in the out directory outside gen-tex/
// — the author's own LaTeX project — neither refuse the export nor change;
// everything the export writes but the preamble is under gen-tex/.
func TestWriteOutBesideAuthorFiles(t *testing.T) {
	dir := t.TempDir()
	author := map[string]string{
		"main.tex":            "the author's",
		ManifestFile:          "files:\n  - main.tex\n",
		"chapters/one.tex":    "one",
		"assets/vol1/fig.png": "the author's figure",
	}
	for rel, data := range author {
		writeFile(t, dir, rel, data)
	}
	before := snapshot(t, dir)
	written, preamble, _, err := WriteOut(dir, assembleKB(t, exportKB()))
	if err != nil {
		t.Fatalf("WriteOut beside the author's files: %v", err)
	}
	gen := filepath.Join(dir, GenTexDir) + string(filepath.Separator)
	for _, p := range written {
		if p != preamble && !strings.HasPrefix(p, gen) {
			t.Errorf("wrote %s, outside gen-tex/", p)
		}
	}
	after := snapshot(t, dir)
	for rel := range author {
		if !reflect.DeepEqual(after[rel], before[rel]) {
			t.Errorf("the author's %s changed", rel)
		}
	}
	if _, _, _, err := WriteOut(dir, map[string][]byte{"a.txt": []byte("a")}); err != nil {
		t.Fatalf("reduced re-issue: %v", err)
	}
	after = snapshot(t, dir)
	for rel := range author {
		if !reflect.DeepEqual(after[rel], before[rel]) {
			t.Errorf("the author's %s changed on a reduced re-issue", rel)
		}
	}
	if _, ok := after[GenTexDir+"/assets/vol1/fig.png"]; ok {
		t.Errorf("gen-tex/assets/vol1/fig.png survived the reduced re-issue")
	}
}

// TestWriteOutReissue: a first write creates the files, the manifest and
// the starter preamble; an identical re-issue writes nothing, the author's
// edited preamble included, and leaves every byte and time as it was; a
// re-issue lacking a file the manifest listed removes it, and keeps the
// outputs the export owns and the files it does not.
func TestWriteOutReissue(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	gen := filepath.Join(dir, GenTexDir)
	files := assembleKB(t, exportKB())
	written, preamble, changed, err := WriteOut(dir, files)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, PreambleFile); preamble != want {
		t.Errorf("first write: preamble %q, want %q", preamble, want)
	}
	if !changed || len(written) != len(files)+2 || !slices.Contains(written, preamble) {
		t.Fatalf("first write: changed %v, %d written, want every file, the manifest and the preamble", changed, len(written))
	}
	for _, p := range written {
		if !filepath.IsAbs(p) {
			t.Errorf("written path %s is not absolute", p)
		}
	}
	if got, err := os.ReadFile(preamble); err != nil || !slices.Equal(got, preambleStarter) {
		t.Errorf("the preamble written is not the starter (%v):\n%s", err, got)
	}
	var m manifest
	if err := unmarshalYAML(filepath.Join(gen, ManifestFile), &m); err != nil {
		t.Fatal(err)
	}
	wantListed := append(slices.Collect(maps.Keys(files)), OutputFile, FindingsFile, ReportFile)
	slices.Sort(wantListed)
	if !slices.Equal(m.Files, wantListed) {
		t.Errorf("manifest %q, want %q", m.Files, wantListed)
	}

	// The outputs pandoc and the verb write, a file of the author's, and the
	// author's edit to the preamble.
	for _, name := range []string{OutputFile, "notes.txt"} {
		writeFile(t, gen, name, name)
	}
	writeFile(t, dir, PreambleFile, "\\newcommand{\\R}{\\mathbb{R}}\n")
	before := snapshot(t, dir)
	written, preamble, changed, err = WriteOut(dir, assembleKB(t, exportKB()))
	if err != nil {
		t.Fatal(err)
	}
	if changed || len(written) != 0 || preamble != "" {
		t.Errorf("identical re-issue: changed %v, wrote %q, preamble %q", changed, written, preamble)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("identical re-issue changed the directory")
	}

	reduced := maps.Clone(files)
	delete(reduced, "assets/vol1/fig.png")
	written, _, changed, err = WriteOut(dir, reduced)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || len(written) != 1 || filepath.Base(written[0]) != ManifestFile {
		t.Errorf("reduced re-issue: changed %v, wrote %q, want the manifest alone", changed, written)
	}
	after := snapshot(t, dir)
	if _, ok := after[GenTexDir+"/assets/vol1/fig.png"]; ok {
		t.Errorf("the file the previous manifest listed survived")
	}
	for _, kept := range []string{OutputFile, "notes.txt", "assets/vol1/nested/data.csv"} {
		if _, ok := after[GenTexDir+"/"+kept]; !ok {
			t.Errorf("gen-tex/%s was removed", kept)
		}
	}
	if !reflect.DeepEqual(after[PreambleFile], before[PreambleFile]) {
		t.Errorf("the author's preamble changed")
	}
}

// TestWriteOutRemovesEmptiedDirs: a re-issue that removes files removes the
// directories under gen-tex/ the removal emptied, innermost first, and keeps
// a directory still holding a file of the author's.
func TestWriteOutRemovesEmptiedDirs(t *testing.T) {
	dir := t.TempDir()
	gen := filepath.Join(dir, GenTexDir)
	first := map[string][]byte{
		"keep.txt":           []byte("k"),
		"assets/a/x/one.bin": []byte("1"),
		"assets/a/two.bin":   []byte("2"),
		"assets/b/three.bin": []byte("3"),
	}
	if _, _, _, err := WriteOut(dir, first); err != nil {
		t.Fatal(err)
	}
	writeFile(t, gen, "assets/b/mine.txt", "mine")
	if _, _, _, err := WriteOut(dir, map[string][]byte{"keep.txt": []byte("k")}); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"assets/a/x", "assets/a"} {
		if _, err := os.Stat(filepath.Join(gen, filepath.FromSlash(gone))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived the removal that emptied it (%v)", gone, err)
		}
	}
	if _, err := os.Stat(filepath.Join(gen, "assets", "b", "mine.txt")); err != nil {
		t.Errorf("the author's file was lost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gen, "assets", "b", "three.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("assets/b/three.bin survived (%v)", err)
	}
}

// TestWriteOutRemovesTheSingleStream: a re-issue over an export whose
// manifest lists the single stream file kbase-export.md, which the stream's
// files have replaced, removes it and writes the files that replace it.
func TestWriteOutRemovesTheSingleStream(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := WriteOut(dir, map[string][]byte{"kbase-export.md": []byte("# Old\n")}); err != nil {
		t.Fatal(err)
	}
	files := assembleKB(t, exportKB())
	if _, _, _, err := WriteOut(dir, files); err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, dir)
	if _, ok := after[GenTexDir+"/kbase-export.md"]; ok {
		t.Errorf("gen-tex/kbase-export.md, which the previous manifest listed, survived")
	}
	for _, name := range tablesOf(t, files).Streams {
		if !slices.Equal(after[GenTexDir+"/"+name].data, files[name]) {
			t.Errorf("gen-tex/%s does not hold the stream file", name)
		}
	}
}

func unmarshalYAML(p string, v any) error {
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

func TestWriteOutRefusesEscapingManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, GenTexDir+"/"+ManifestFile, "files:\n  - ../outside.txt\n")
	if _, _, _, err := WriteOut(dir, map[string][]byte{"a.txt": []byte("a")}); err == nil || !strings.Contains(err.Error(), "../outside.txt") {
		t.Errorf("WriteOut over a manifest naming ../outside.txt = %v, want an error naming it", err)
	}
}

// numberedSectioningRE is a numbered LaTeX sectioning command.
var numberedSectioningRE = regexp.MustCompile(`\\(?:part|chapter|section|subsection|subsubsection|paragraph|subparagraph)\{`)

// TestAssembledExportConverts: the assembled set, written and run through
// pandoc in gen-tex/ as the verb runs it, beside an author's preamble.tex
// defining a macro the maths uses, converts with the filter raising nothing:
// the tables match the filter's schema and the stream's ids its documents,
// and main.tex loads the preamble, the macro left to it.
func TestAssembledExportConverts(t *testing.T) {
	dir := t.TempDir()
	const preamble = "\\newcommand{\\kbR}{\\mathbb{R}}\n"
	writeFile(t, dir, PreambleFile, preamble)
	kb := exportKB()
	kb["vol2/v2-leaf.md"] += "\n``` math\nx \\in \\kbR\n```\n"
	latex, findings := convertKB(t, dir, kb)
	if got, err := os.ReadFile(filepath.Join(dir, PreambleFile)); err != nil || string(got) != preamble {
		t.Errorf("the author's preamble changed (%v): %q", err, got)
	}
	for _, want := range []string{
		"% Generated by kbase export-latex " + testVersion + ".",
		`\InputIfFileExists{../preamble.tex}{}{}`,
		`x \in \kbR`,
		`\title{A KB}`,
		`\chapter{First Volume}\label{vol1}`,
		`\chapter{Second Volume}\label{vol2}`,
		`\label{d1-v1-leaf}`, `\label{d1-nested-index}`, `\label{d1-nested-deep}`, `\label{d1-overview}`,
		`\label{d2-v2-leaf}`, `\label{d2-v2-noheading}`,
		`\section{`, `\subsection{Deep}\label{d1-nested-deep}`,
		`\hyperref[d2-v2-leaf]{the other leaf}`,
		`\begin{kb-theorem}\label{v1-thm:a}`, `\begin{kb-lemma}`,
		`\cite{k1}`, `\cite{smith2020}`,
		`\bibitem{k1} Author, A. 2001. \emph{Title One}.`, `\bibitem{smith2020} Smith, \emph{A Work}`,
	} {
		if !strings.Contains(string(latex), want) {
			t.Errorf("main.tex does not carry %q:\n%s", want, latex)
		}
	}
	_, body, _ := strings.Cut(string(latex), `\begin{document}`)
	front, _, ok := strings.Cut(body, `\chapter{`)
	if !ok {
		t.Fatalf("main.tex has no numbered chapter:\n%s", latex)
	}
	if !strings.Contains(front, `\section*{What this KB contains}`) {
		t.Errorf("the entry point's prose heading is not an unnumbered section before the first chapter:\n%s", front)
	}
	if m := numberedSectioningRE.FindString(front); m != "" {
		t.Errorf("a numbered %s precedes the first chapter:\n%s", m, front)
	}
	if len(findings) != 0 {
		t.Errorf("filter findings %v, want none", findings)
	}
}

// TestAssembledExportPerVolumeScope: two volumes each defining footnote [^1]
// with a note of their own keep each reference's own note; under the
// file-scope that buys it, a link to a document heading by its id and a
// native span's id still resolve, no file prefix left in main.tex.
func TestAssembledExportPerVolumeScope(t *testing.T) {
	kb := exportKB()
	kb["vol1/v1-leaf.md"] += "\nAlpha[^1] and [the top](#d1-v1-leaf).\n\n[^1]: Note of the first volume.\n\n" +
		"Strengths by length. {#T:6}\n\nSee table <a href=\"v1-leaf.md\" data-reference-type=\"ref\" data-reference=\"T:6\">9</a>.\n"
	kb["vol2/v2-leaf.md"] += "\nBeta[^1].\n\n[^1]: Note of the second volume.\n"
	latex, findings := convertKB(t, t.TempDir(), kb)
	for _, want := range []string{
		`Alpha\footnote{Note of the first volume.}`,
		`Beta\footnote{Note of the second volume.}`,
		`\hyperref[d1-v1-leaf]{the top}`,
		`\label{v1-T:6}`, `See table \ref{v1-T:6}.`,
	} {
		if !strings.Contains(latex, want) {
			t.Errorf("main.tex does not carry %q:\n%s", want, latex)
		}
	}
	if strings.Contains(latex, ".md__") {
		t.Errorf("main.tex carries a file-scope prefix:\n%s", latex)
	}
	if len(findings) != 0 {
		t.Errorf("filter findings %v, want none", findings)
	}
}

// convertKB assembles the KB files describes, writes it into dir and runs
// pandoc over it in gen-tex/ as the verb does, returning main.tex and the
// filter's findings. It skips where pandoc is unusable.
func convertKB(t *testing.T, dir string, files map[string]string) (string, []map[string]string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pandoc.Preflight(ctx); err != nil {
		t.Skipf("pandoc is unusable here: %v", err)
	}
	if _, _, _, err := WriteOut(dir, assembleKB(t, files)); err != nil {
		t.Fatal(err)
	}
	gen := filepath.Join(dir, GenTexDir)
	if err := pandoc.RunDefaults(ctx, log.Discard(), filepath.Join(gen, DefaultsFile)); err != nil {
		t.Fatalf("pandoc over the assembled export: %v", err)
	}
	latex, err := os.ReadFile(filepath.Join(gen, OutputFile))
	if err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(gen, FindingsFile))
	if err != nil {
		t.Fatal(err)
	}
	var findings []map[string]string
	if err := json.Unmarshal(report, &findings); err != nil {
		t.Fatalf("filter findings %s: %v", report, err)
	}
	return string(latex), findings
}
