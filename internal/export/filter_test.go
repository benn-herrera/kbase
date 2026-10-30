package export_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kbase/internal/export"
	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
)

// xrefTables mirrors the table schema at the head of kbase-export.lua.
type xrefTables struct {
	Documents    map[string]xrefDocument `json:"documents"`
	Paths        map[string]string       `json:"paths"`
	Volumes      []xrefVolume            `json:"volumes"`
	Theorems     []string                `json:"theorems"`
	Bibliography map[string]string       `json:"bibliography"`
	Report       string                  `json:"report"`
	Assets       map[string]string       `json:"assets"`
	EntryPoint   string                  `json:"entry_point"`
	Streams      []string                `json:"streams"`
}

type xrefDocument struct {
	Path   string `json:"path"`
	Volume int    `json:"volume"`
	Rank   int    `json:"rank"`
}

type xrefVolume struct {
	Label string `json:"label"`
	Title string `json:"title"`
}

type filterFinding struct {
	Form   string `json:"form"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// add puts the document at path, of volume n and rank, in the tables under
// the id Go mints for it, and returns the id.
func (x *xrefTables) add(n int, volumeDir, path string, rank int) string {
	id := export.DocumentLabel(n, volumeDir, path)
	if rank == 0 {
		id = export.VolumeLabel(n)
	}
	x.Documents[id] = xrefDocument{Path: path, Volume: n, Rank: rank}
	x.Paths[path] = id
	return id
}

// fixtureTables is two volumes, alpha and beta:
//
//	vol1     alpha/index.md   rank 0
//	d1-a     alpha/a.md       rank 1
//	d1-sub-b alpha/sub/b.md   rank 2
//	vol2     beta/index.md    rank 0
//	d2-c     beta/c.md        rank 1
func fixtureTables() xrefTables {
	x := xrefTables{
		Documents:    map[string]xrefDocument{},
		Paths:        map[string]string{},
		Volumes:      []xrefVolume{{Label: export.VolumeLabel(1), Title: "Alpha"}, {Label: export.VolumeLabel(2), Title: "Beta"}},
		Theorems:     []string{"Theorem", "Main Result"},
		Bibliography: map[string]string{"k1": "Author, *Title One*, 2001.", "k2": "Other, *Title Two*, 2002."},
		Report:       "filter-findings.json",
		Assets:       map[string]string{},
		EntryPoint:   "entry-point.md",
		Streams:      []string{"kbase-export.md"},
	}
	x.add(1, "alpha", "alpha/index.md", 0)
	x.add(1, "alpha", "alpha/a.md", 1)
	x.add(1, "alpha", "alpha/sub/b.md", 2)
	x.add(2, "beta", "beta/index.md", 0)
	x.add(2, "beta", "beta/c.md", 1)
	return x
}

const filterDefaults = `from: gfm+attributes-gfm_auto_identifiers
to: latex
standalone: %t
top-level-division: chapter
wrap: none
filters: ["${.}/kbase-export.lua"]
metadata:
  kbase-xref: kbase-xref.json
input-files: ["${.}/kbase-export.md"]
output-file: "${.}/main.tex"
`

// runFilter writes stream, tables, the filter and a defaults file naming
// them into a fresh directory, runs pandoc over the defaults, and returns
// the LaTeX and the filter's findings. It skips where pandoc is unusable.
func runFilter(t *testing.T, stream string, tables xrefTables, standalone bool) (string, []filterFinding) {
	t.Helper()
	ctx := context.Background()
	if _, err := pandoc.Preflight(ctx); err != nil {
		t.Skipf("pandoc is unusable here: %v", err)
	}
	encoded, err := json.Marshal(tables)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string][]byte{
		"kbase-export.md":  []byte(stream),
		"kbase-xref.json":  encoded,
		"kbase-export.lua": export.LuaFilter,
		"defaults.yaml":    fmt.Appendf(nil, filterDefaults, standalone),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := pandoc.RunDefaults(ctx, log.Discard(), filepath.Join(dir, "defaults.yaml")); err != nil {
		t.Fatal(err)
	}
	latex, err := os.ReadFile(filepath.Join(dir, "main.tex"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(dir, tables.Report))
	if err != nil {
		t.Fatal(err)
	}
	var findings []filterFinding
	if err := json.Unmarshal(report, &findings); err != nil {
		t.Fatalf("the findings are not a JSON list of findings: %v\n%s", err, report)
	}
	return string(latex), findings
}

func wantContains(t *testing.T, latex string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(latex, w) {
			t.Errorf("the LaTeX does not carry %q:\n%s", w, latex)
		}
	}
}

func wantAbsent(t *testing.T, latex string, absent ...string) {
	t.Helper()
	for _, a := range absent {
		if strings.Contains(latex, a) {
			t.Errorf("the LaTeX carries %q:\n%s", a, latex)
		}
	}
}

// wantFindings holds the findings to exactly forms, in order, each at path.
func wantFindings(t *testing.T, findings []filterFinding, path string, forms ...string) {
	t.Helper()
	got := make([]string, len(findings))
	for i, f := range findings {
		got[i] = f.Form
		if f.Path != path {
			t.Errorf("finding %+v is not at %s", f, path)
		}
	}
	if strings.Join(got, " ") != strings.Join(forms, " ") {
		t.Errorf("finding forms %q, want %q: %+v", got, forms, findings)
	}
}

func TestFilterLabelSpan(t *testing.T) {
	latex, findings := runFilter(t, "# A {#d1-a}\n\nSee <span id=\"fig:x\">the figure</span> here.\n", fixtureTables(), false)
	wantContains(t, latex, `\protect\phantomsection\label{v1-fig:x}{the figure}`)
	wantFindings(t, findings, "")
}

// anchorStream defines lem:a and lem:b in alpha/a.md and eq:c in beta/c.md,
// and holds one anchor in alpha/sub/b.md.
const anchorStream = `# A {#d1-a}

<span id="lem:a">A</span> and <span id="lem:b">B</span>.

# B {#d1-sub-b}

Before %s after.

# C {#d2-c}

<span id="eq:c">C</span>
`

func TestFilterAnchors(t *testing.T) {
	for _, tc := range []struct {
		name, anchor, want string
		forms              []string
	}{
		{"element in own volume", `<a data-reference-type="ref" data-reference="lem:a">TEXT</a>`, `Before \ref{v1-lem:a} after`, nil},
		{"element via href volume", `<a href="../../beta/c.md#eq:c" data-reference-type="ref" data-reference="eq:c">TEXT</a>`, `\ref{v2-eq:c}`, nil},
		{"fragmentless href resolved by data-reference", `<a href="../../beta/c.md" data-reference-type="eqref" data-reference="eq:c">TEXT</a>`, `\eqref{v2-eq:c}`, nil},
		{"document fallback", `<a href="../a.md#lem:z" data-reference-type="ref" data-reference="lem:z">TEXT</a>`, `\ref{d1-a}`, nil},
		{"cref over a list", "<a data-reference-type=\"ref+label\"\ndata-reference=\"lem:a, lem:b\">TEXT</a>", `\cref{v1-lem:a,v1-lem:b}`, nil},
		{"Cref over a list", `<a data-reference-type="ref+Label" data-reference="lem:a,lem:b">TEXT</a>`, `\Cref{v1-lem:a,v1-lem:b}`, nil},
		{"degrade: href names no exported document", `<a href="../gone.md#x" data-reference-type="ref" data-reference="x">TEXT</a>`, `Before TEXT after`, []string{"anchor"}},
		{"degrade: label undefined, no href", `<a data-reference-type="ref" data-reference="lem:z">TEXT</a>`, `Before TEXT after`, []string{"anchor"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			latex, findings := runFilter(t, fmt.Sprintf(anchorStream, tc.anchor), fixtureTables(), false)
			wantContains(t, latex, tc.want)
			if tc.forms == nil {
				wantAbsent(t, latex, "TEXT")
			}
			wantFindings(t, findings, "alpha/sub/b.md", tc.forms...)
		})
	}
}

// TestFilterCitations: a citation span all of whose keys have bibliography
// entries is one \cite of them; a span naming any key without one is its own
// text and a finding, whatever its other keys, so no \cite names an undefined
// key and no work the span's text names is dropped.
func TestFilterCitations(t *testing.T) {
	stream := "# A {#d1-a}\n\nFirst <span class=\"citation\" data-cites=\"k2 k1\">(K2; K1)</span>, then <span class=\"citation\"\ndata-cites=\"k1 k3\">(K1; K3)</span>, and <span class=\"citation\" data-cites=\"k4\">(K4)</span>.\n"
	latex, findings := runFilter(t, stream, fixtureTables(), false)
	wantContains(t, latex,
		`First \cite{k2,k1}, then (K1; K3), and (K4).`,
		"\\begin{thebibliography}{2}\n\\bibitem{k2} Other, \\emph{Title Two}, 2002.\n\\bibitem{k1} Author, \\emph{Title One}, 2001.\n\\end{thebibliography}")
	wantAbsent(t, latex, `k3`, `k4`, "(K2; K1)")
	wantFindings(t, findings, "alpha/a.md", "citation", "citation")
}

func TestFilterMathLabels(t *testing.T) {
	stream := "# A {#d1-a}\n\n``` math\n\\begin{align}\nx \\label{eq:a} \\\\\ny \\label{eq:b}\n\\end{align}\n```\n\n" +
		"See $`\\ref{eq:a}`$, $`\\eqref{eq:b}`$, $`\\cref{eq:a,eq:b}`$, $`\\Cref{eq:b}`$ and $`\\ref{eq:none}`$.\n"
	latex, findings := runFilter(t, stream, fixtureTables(), false)
	wantContains(t, latex, `\label{v1-eq:a}`, `\label{v1-eq:b}`, `\ref{v1-eq:a}`, `\eqref{v1-eq:b}`,
		`\cref{v1-eq:a,v1-eq:b}`, `\Cref{v1-eq:b}`, `\ref{eq:none}`)
	wantFindings(t, findings, "alpha/a.md", "math-reference")
}

// TestFilterMathNestedReference: a reference inside another macro's
// argument is namespaced like any other.
func TestFilterMathNestedReference(t *testing.T) {
	stream := "# A {#d1-a}\n\n``` math\nx \\label{eq:a}\n```\n\n$`\\frac{C_{\\eqref{eq:a}}}{2}`$\n"
	latex, findings := runFilter(t, stream, fixtureTables(), false)
	wantContains(t, latex, `\frac{C_{\eqref{v1-eq:a}}}{2}`)
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, "")
}

func TestFilterLabelVolumes(t *testing.T) {
	t.Run("defined twice in one volume", func(t *testing.T) {
		stream := "# A {#d1-a}\n\n<span id=\"x\">one</span>\n\n# B {#d1-sub-b}\n\n``` math\nx \\label{x}\n```\n"
		_, findings := runFilter(t, stream, fixtureTables(), false)
		wantFindings(t, findings, "alpha/sub/b.md", "label")
	})
	t.Run("one label in two volumes", func(t *testing.T) {
		stream := "# A {#d1-a}\n\n<span id=\"x\">one</span>\n\n# C {#d2-c}\n\n<span id=\"x\">two</span> <a data-reference-type=\"ref\" data-reference=\"x\">r</a>\n"
		latex, findings := runFilter(t, stream, fixtureTables(), false)
		wantContains(t, latex, `\label{v1-x}{one}`, `\label{v2-x}{two}`, `\ref{v2-x}`)
		wantFindings(t, findings, "")
	})
}

func TestFilterLinks(t *testing.T) {
	stream := "# B {#d1-sub-b}\n\n[to A](../a.md#frag), [self](#top), [web](https://example.org/x) and [dead](../gone.md).\n"
	latex, findings := runFilter(t, stream, fixtureTables(), false)
	wantContains(t, latex, `\hyperref[d1-a]{to A}`, `\hyperref[d1-sub-b]{self}`, `\href{https://example.org/x}{web}`, `and dead.`)
	wantAbsent(t, latex, "gone")
	wantFindings(t, findings, "alpha/sub/b.md", "link")
}

// TestFilterDegradedTextLabels: a dead link or a missing image becomes its
// text, and a label span or a maths \label in that text, nested or not, is
// namespaced once and defined once.
func TestFilterDegradedTextLabels(t *testing.T) {
	const text = "<span id=\"L\">l</span> $`x \\label{M}`$ *<span id=\"N\">n</span> $`y \\label{O}`$*"
	for _, tc := range []struct{ name, element, form string }{
		{"dead link", "[" + text + "](gone.md)", "link"},
		{"missing image", "![" + text + "](missing.png)", "image"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := "# A {#d1-a}\n\n" + tc.element + "\n\nSee " +
				`<a data-reference-type="ref" data-reference="L">1</a> <a data-reference-type="ref" data-reference="N">2</a>` +
				" and $`\\ref{M} \\ref{O}`$.\n"
			latex, findings := runFilter(t, stream, fixtureTables(), false)
			for _, label := range []string{"L", "M", "N", "O"} {
				if n := strings.Count(latex, `\label{v1-`+label+`}`); n != 1 {
					t.Errorf("label v1-%s is defined %d times, want once:\n%s", label, n, latex)
				}
			}
			wantAbsent(t, latex, "v1-v1-")
			wantReferencesLabelled(t, latex)
			wantFindings(t, findings, "alpha/a.md", tc.form)
		})
	}
}

// TestFilterFrontMatter: content before the first document heading is the
// entry point's, so its links resolve from the entry point and its findings
// name it.
func TestFilterFrontMatter(t *testing.T) {
	tables := fixtureTables()
	latex, findings := runFilter(t, "[to A](alpha/a.md) and [dead](gone.md).\n\n# A {#d1-a}\n", tables, false)
	wantContains(t, latex, `\hyperref[d1-a]{to A} and dead.`)
	wantFindings(t, findings, tables.EntryPoint, "link")
}

// TestFilterNoteLinks: inside a footnote, a destination naming no exported
// document from the current document names the one exported document it
// names from a document of the current volume; none or several are a
// finding. Outside a footnote it resolves from the current document alone.
// alpha/p/x.md's p/x.md names alpha/p/x.md from alpha/ and alpha/sub/p/x.md
// from alpha/sub/.
func TestFilterNoteLinks(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		forms            []string
	}{
		{"one resolution", "Text[^1].\n\n[^1]: See [B](sub/b.md).\n", `\hyperref[d1-sub-b]{B}`, nil},
		{"several resolutions", "Text[^1].\n\n[^1]: See [X](p/x.md).\n", `See X.`, []string{"link"}},
		{"outside a footnote", "See [B](sub/b.md).\n", `See B.`, []string{"link"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tables := fixtureTables()
			id := tables.add(1, "alpha", "alpha/p/x.md", 2)
			tables.add(1, "alpha", "alpha/sub/p/x.md", 3)
			latex, findings := runFilter(t, "# X {#"+id+"}\n\n"+tc.body, tables, false)
			wantContains(t, latex, tc.want)
			wantFindings(t, findings, "alpha/p/x.md", tc.forms...)
		})
	}
}

// TestFilterFootnoteAnchor: a built KB's footnote definition, relinked for
// the document holding it, is referenced from a document in another
// directory; its anchor resolves against the volume's documents.
// Excerpts, of 2609.09855v1/kb-root/a-unifying-perspective-on-probabilities-as/:
//
//	footnote-reference.md   behind-every-probability-is-a-prediction-method/predictors-and-prediction-methods.md, lines 46-51
//	footnote-definition.md  conditional-probabilities-generalised.md, lines 87-92
func TestFilterFootnoteAnchor(t *testing.T) {
	const volume = "a-unifying-perspective-on-probabilities-as"
	tables := fixtureTables()
	referencing := tables.add(1, volume, volume+"/behind-every-probability-is-a-prediction-method/predictors-and-prediction-methods.md", 2)
	defining := tables.add(1, volume, volume+"/conditional-probabilities-generalised.md", 1)
	target := tables.add(1, volume, volume+"/how-is-calibration-possible/extrapolating-calibration.md", 2)
	stream := "# Predictors {#" + referencing + "}\n\n" + excerpt(t, "footnote-reference.md") +
		"\n# Conditional {#" + defining + "}\n\n" + excerpt(t, "footnote-definition.md") +
		"\n# Extrapolating {#" + target + "}\n"
	latex, findings := runFilter(t, stream, tables, false)
	wantContains(t, latex, `(Section~\ref{`+target+`})`)
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, "")
}

func TestFilterHeadings(t *testing.T) {
	tables := fixtureTables()
	tables.add(1, "alpha", "alpha/deep.md", 5)
	tables.add(1, "alpha", "alpha/deeper.md", 6)
	stream := "# Alpha {#vol1}\n\n## Alpha intro\n\n# A {#d1-a}\n\n## Inner {#custom}\n\n# B {#d1-sub-b}\n\n" +
		"# Deep {#d1-deep}\n\n## Too deep\n\n# Deeper {#d1-deeper}\n"
	latex, findings := runFilter(t, stream, tables, false)
	wantContains(t, latex, `\chapter{Alpha}\label{vol1}`, `\section{Alpha intro}`, `\section{A}\label{d1-a}`,
		`\subsection{Inner}`, `\subsection{B}\label{d1-sub-b}`, `\subparagraph{Deep}\label{d1-deep}`,
		`\textbf{Too deep}`, `\protect\phantomsection\label{d1-deeper}{\textbf{Deeper}}`)
	wantAbsent(t, latex, "custom", `\subparagraph{Too deep}`)
	if len(findings) != 2 || findings[0].Path != "alpha/deep.md" || findings[1].Path != "alpha/deeper.md" {
		t.Errorf("want one heading finding at each deep document, got %+v", findings)
	}
	for _, f := range findings {
		if f.Form != "heading" {
			t.Errorf("finding %+v is not a heading finding", f)
		}
	}
}

func TestFilterResidue(t *testing.T) {
	stream := "# A {#d1-a}\n\nAn <span id=\"x\">unclosed span and <a data-reference=\"r\">an unclosed anchor.\n"
	_, findings := runFilter(t, stream, fixtureTables(), false)
	wantFindings(t, findings, "alpha/a.md", "residue", "residue")
}

func TestFilterDropsMarkers(t *testing.T) {
	stream := "# A {#d1-a}\n\n<!-- id: clm-abc123 -->\n\nA claim. <!-- claim-rigor: solid -->\n"
	latex, findings := runFilter(t, stream, fixtureTables(), false)
	wantContains(t, latex, "A claim.")
	wantAbsent(t, latex, "clm-abc123", "claim-rigor")
	wantFindings(t, findings, "")
}

// excerpt is the file name under testdata/excerpts.
func excerpt(t *testing.T, name string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "excerpts", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

// The files under testdata/excerpts are byte-exact line ranges of documents in
// the reference's arXiv KBs; that clone is transient and is not in this
// repository, so these files are the fixtures. Each line names the source
// document and range:
//
//	theorem-rremark.md    2609.00268v1/kb-root/obstructions-for-morin-and-fold-maps-stiefel/theory/thom-polynomials-and-the-role-of-the/the-factorization-formula-for-schur-polynomials.md, lines 103-108
//	proof.md              2609.00183v1/kb-root/rectifiability-of-the-singular-strata-for/appendix.md, lines 63-66
//	remark-emphasised.md  2609.10488v1/kb-root/difference-equations-for-local-gromov-witten/arithmetic-reconstruction-and-cyclotomic/m-bius-reconstruction.md, lines 53-60
//	minipage.md           2609.09693v1/kb-root/main/limited-attention-exchange/a-school-choice-query-comparison.md, lines 34-40
//	eqnarray.md           2609.00183v1/kb-root/rectifiability-of-the-singular-strata-for/preliminaries/harmonic-maps-into-npc-spaces.md, lines 44-48
//	math-blank-lines.md   2609.00268v1/kb-root/obstructions-for-morin-and-fold-maps-stiefel/theory/thom-series-in-negative-relative-codimension/the-quadratic-unfolding.md, lines 43-93
//	native-span-anchor.md 2609.10038v1/kb-root/<jutePath>, lines 10-11
//	native-span-id.md     2609.10038v1/kb-root/<jutePath>, line 57
//	theorem-bare-number.md  2609.00269v1/kb-root/a-holomorphic-theorem-for-abelian-varieties-of/linear-algebra.md, lines 130-136
//	table-caption-id.md     2609.10517v1/kb-root/<enclosuresPath>, lines 14-34
//	link-around-anchor.md   2609.10499v1/kb-root/<proofOfOnePath>, lines 412-414

// TestFilterTheorems: a labelled blockquote whose first body paragraph has
// the printed-number opening is a numbered environment kb-<label name>,
// declared unconditionally under the opening's name (rremark prints as
// Remark), or the label line's where the opening prints a bare number (PAR).
// One without that opening — a proof, which prints its own head and
// end mark; an emphasised head; a LaTeX environment's name — is its body,
// unquoted, its label span's id kept as a label. Any other blockquote stays a
// quote.
func TestFilterTheorems(t *testing.T) {
	stream := `# A {#d1-a}

> <span id="thm:big">**Theorem**</span>
>
> **Theorem 1** (Big One). *Body text* cites <span class="citation" data-cites="k1">(K1)</span>.

> **Main Result**
>
> **Main Result 2**. Plain body.

> **Theorem**
>
> **Theorem 2** (Range ]a, b]). Half-open.

> **Note**
>
> Not a theorem name.

> An ordinary quote.

> **Lemma**
> <!-- claim-rigor: ana-aaaaaa -->
> **Lemma 3** (Marked). Split body. <!-- claim-rigor: clm-aaaaaa -->

See <a data-reference-type="ref" data-reference="thm:big">1</a>, <a data-reference-type="ref" data-reference="rmk:HomAB">7</a> and <a data-reference-type="ref" data-reference="rem:g-normalization">14</a>.

` + excerpt(t, "theorem-rremark.md") + "\n" + excerpt(t, "proof.md") + "\n" + excerpt(t, "remark-emphasised.md") + "\n" + excerpt(t, "minipage.md") + "\n" + excerpt(t, "theorem-bare-number.md")
	tables := fixtureTables()
	tables.Theorems = append(tables.Theorems, "rremark", "proof", "Remark", "minipage", "PAR", "Lemma")
	latex, findings := runFilter(t, stream, tables, true)
	wantContains(t, latex,
		`\begin{kb-lemma}[{Marked}] Split body.`,
		"\\end{kb-lemma}",
		`\begin{kb-theorem}[{Big One}]\label{v1-thm:big} \emph{Body text} cites \cite{k1}.`,
		`\begin{kb-theorem}[{Range {]}a, b{]}}] Half-open.`,
		"\\end{kb-theorem}",
		`\begin{kb-main-result} Plain body.`,
		"\\end{kb-main-result}",
		`\begin{kb-rremark}\label{v1-rmk:HomAB} \emph{If \(a_i\) and \(b_i\) denote`,
		"\\end{kb-rremark}",
		"\n\\emph{Proof.} The proof is analogous to the proof of (Breiner and Dees 2025, Proposition A.2).~◻\n",
		"\n\\protect\\phantomsection\\label{v1-rem:g-normalization}{}\\emph{Remark 14}. The normalization \\(g\\)",
		"\nNotes: Mean distinct binary comparisons",
		"\\begin{quote}\n\\textbf{Note}",
		"\\begin{quote}\nAn ordinary quote.",
		`\begin{kb-par} \emph{We will be interested in the following cosimplicial dg-modules over \(K\):}`,
		"\\end{kb-par}")
	wantAbsent(t, latex, "Theorem 1", "Theorem 2", "Main Result 2", "Remark 7", "\\begin{quote}\n\\emph{Proof.}",
		"\\begin{quote}\nNotes:", "\\begin{quote}\n\\protect", "kb-proof", "kb-remark", "kb-minipage", `\begin{proof}`, "ifcsname", "ana-aaaaaa", "claim-rigor")
	wantReferencesLabelled(t, latex)
	preamble, _, ok := strings.Cut(latex, `\begin{document}`)
	if !ok {
		t.Fatalf("a standalone run has no \\begin{document}:\n%s", latex)
	}
	wantContains(t, preamble, `\kbasetheorem{kb-theorem}{Theorem}{Theorems}`,
		`\kbasetheorem{kb-main-result}{Main Result}{Main Results}`, `\kbasetheorem{kb-rremark}{Remark}{Remarks}`,
		`\kbasetheorem{kb-par}{PAR}{PARs}`)
	wantFindings(t, findings, "alpha/a.md", "citation")
}

// TestFilterTheoremPlurals: a theorem kind is declared with its display
// name's English plural, by the last word's ending.
func TestFilterTheoremPlurals(t *testing.T) {
	cases := []struct{ name, plural string }{
		{"Hypothesis", "Hypotheses"},
		{"Corollary", "Corollaries"},
		{"Survey", "Surveys"},
		{"Process", "Processes"},
		{"Approach", "Approaches"},
		{"Box", "Boxes"},
		{"Examples", "Examples"},
		{"Key Lemma", "Key Lemmas"},
		{"Open Problem", "Open Problems"},
	}
	tables := fixtureTables()
	var stream strings.Builder
	stream.WriteString("# A {#d1-a}\n\n")
	for _, c := range cases {
		tables.Theorems = append(tables.Theorems, c.name)
		fmt.Fprintf(&stream, "> **%s**\n>\n> **%s 1**. Body.\n\n", c.name, c.name)
	}
	latex, _ := runFilter(t, stream.String(), tables, true)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := "kb-" + strings.ReplaceAll(strings.ToLower(c.name), " ", "-")
			wantContains(t, latex, fmt.Sprintf(`\kbasetheorem{%s}{%s}{%s}`, env, c.name, c.plural))
		})
	}
}

// TestFilterTableCaption: the paragraph pandoc's gfm writer writes after a
// captioned table — the caption, then the table's id as an attribute span —
// is the table's caption again, its id the table's, so a reference to it
// names the table's counter. Excerpt: excerpts/table-caption-id.md.
func TestFilterTableCaption(t *testing.T) {
	tables := fixtureTables()
	id := tables.add(1, enclosuresVolume, enclosuresPath, 1)
	latex, findings := runFilter(t, "# Document {#"+id+"}\n\n"+excerpt(t, "table-caption-id.md"), tables, false)
	wantContains(t, latex, `collected in \Cref{v1-tab:enclosures}, with`,
		`\caption{Certified enclosures \(\kappa(P_{2n})\in\left[l_n,\,l_n+10^{-12}\right]\). For comparison, \(j_{1,1}=3.831705970207\ldots\)}\label{v1-tab:enclosures}`)
	wantAbsent(t, latex, `\phantomsection`)
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, enclosuresPath)
}

// TestFilterLinkAroundAnchor: a link around a reference anchor — a LaTeX
// source's \hyperref[L]{Appendix \ref{L}}, its bare fragment naming no
// exported label — is its content, so the anchor's href names the target,
// not the current document. Excerpt: excerpts/link-around-anchor.md.
func TestFilterLinkAroundAnchor(t *testing.T) {
	tables := fixtureTables()
	id := tables.add(1, proofOfOneVolume, proofOfOnePath, 2)
	appendix := tables.add(1, proofOfOneVolume, proofOfOneVolume+"/appendix-omitted-details-in-appendix-6-1.md", 1)
	latex, findings := runFilter(t, "# Document {#"+id+"}\n\n"+excerpt(t, "link-around-anchor.md"), tables, false)
	wantContains(t, latex, `Appendix \ref{`+appendix+`} proves this inequality based on direct calculation.`)
	wantAbsent(t, latex, `\hyperref`)
	wantFindings(t, findings, proofOfOnePath)
}

// TestFilterFigureInTheorem: blocks a theorem rewrite returns get the block
// rules too — a figure in a theorem body has its id namespaced, so the anchor
// to it finds its label. Excerpt: html-forms/figure-in-theorem.md.
func TestFilterFigureInTheorem(t *testing.T) {
	tables := fixtureTables()
	tables.Theorems = append(tables.Theorems, "Remark")
	latex, findings := runFilter(t, htmlFormsStream(t, &tables, figureInTheoremPath, "", "figure-in-theorem.md"), tables, false)
	wantContains(t, latex, `\begin{kb-remark}\label{v1-rmk:numevid}`, `Figure \ref{v1-fig:rectlevelcurv} below`,
		`\label{v1-fig:rectlevelcurv}`)
	wantAbsent(t, latex, `\label{fig:rectlevelcurv}`)
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, figureInTheoremPath)
}

// TestFilterFigureManyBlocks: the writer sets a figure's blocks side by side
// at 0.9/n of the line each and prints a width under 0.1 in a notation LaTeX
// cannot read, so a figure of ten blocks has them one under another.
// Excerpt: html-forms/figure-ten-blocks.md.
func TestFilterFigureManyBlocks(t *testing.T) {
	tables := fixtureTables()
	for _, name := range []string{"jute_1hd", "jute_1qq", "jute_1pp", "jute_2hd", "jute_2qq", "jute_2pp", "jute_3hd", "jute_3qq", "jute_3pp", "juteboxplot"} {
		asset := juteDir + "/assets/" + name + ".pdf"
		tables.Assets[asset] = "assets/" + asset
	}
	latex, findings := runFilter(t, htmlFormsStream(t, &tables, jutePath, "", "figure-ten-blocks.md"), tables, false)
	if n := strings.Count(latex, `\includegraphics`); n != 10 {
		t.Errorf("the figure holds %d images, want 10:\n%s", n, latex)
	}
	wantContains(t, latex, `\caption{Graphical representations of the three jute fiber datasets (Jute 1--3) and the corresponding boxplot.}`)
	wantAbsent(t, latex, `e-2\linewidth`, `\begin{minipage}`)
	wantFindings(t, findings, jutePath)
}

// TestFilterRawDisplayEnvironments: a display environment the writer would
// wrap in \[…\] is written bare. Excerpt: excerpts/eqnarray.md.
func TestFilterRawDisplayEnvironments(t *testing.T) {
	latex, findings := runFilter(t, "# A {#d1-a}\n\n"+excerpt(t, "eqnarray.md"), fixtureTables(), false)
	wantContains(t, latex, "\\begin{eqnarray*}\nI^u(x_0,r) := ", "\\end{eqnarray*}")
	wantAbsent(t, latex, `\[`)
	wantFindings(t, findings, "")
}

// TestFilterMathComment: a maths body whose last line holds an unescaped %
// ends with a newline, so the comment stops before the writer's closing
// delimiter. Synthetic: the writer already breaks the line after a lone %
// (fixture 2609.00268v1's case), but reads \\% as an escaped %.
func TestFilterMathComment(t *testing.T) {
	stream := "# A {#d1-a}\n\nInline $`x \\\\% c`$ and $`y \\% d`$.\n\n``` math\nz \\\\% e\n```\n"
	latex, findings := runFilter(t, stream, fixtureTables(), false)
	wantContains(t, latex, "\\(x \\\\% c\n\\)", "\\(y \\% d\\)", "\\[z \\\\% e\n\\]")
	wantFindings(t, findings, "")
}

// TestFilterMathBlankLines: a whitespace-only line in a maths body ends a
// paragraph, which maths mode cannot hold, so it is removed and every other
// line kept as written. Excerpt: excerpts/math-blank-lines.md.
func TestFilterMathBlankLines(t *testing.T) {
	latex, findings := runFilter(t, "# A {#d1-a}\n\n"+excerpt(t, "math-blank-lines.md"), fixtureTables(), false)
	open, end := strings.Index(latex, `\[`), strings.LastIndex(latex, `\]`)
	if open < 0 || end < open {
		t.Fatalf("no display maths:\n%s", latex)
	}
	if blank := regexp.MustCompile(`\n[ \t]*\n`).FindStringIndex(latex[open:end]); blank != nil {
		t.Errorf("the maths holds a blank line at byte %d:\n%s", open+blank[0], latex)
	}
	wantContains(t, latex,
		"s_2+    2s_{1,1}\n            \\\\[1.2em]\n            \\ell=-1\n",
		"2s_3+2s_{2,1}\n            \\\\[1.2em]\n            \\ell=-2\n",
		"3s_{4}+     2s_{3,1}\n        \\end{array}\n    \\end{array}")
	wantFindings(t, findings, "")
}

// TestFilterNativeIDs: an author id on a native element the writer prints as
// a label — a span, a link, a div, a code block — is defined and namespaced as
// a label span's id is, so an anchor to it resolves. Excerpts:
// excerpts/native-span-anchor.md and excerpts/native-span-id.md.
func TestFilterNativeIDs(t *testing.T) {
	tables := fixtureTables()
	id := tables.add(1, juteDir, jutePath, 1)
	stream := "# Document {#" + id + "}\n\n" + excerpt(t, "native-span-anchor.md") + "\n" + excerpt(t, "native-span-id.md") +
		"\n[web](https://example.org){#lnk:w}\n\n{#div:d}\nA block.\n\n``` {#code:c}\nx\n```\n\n" +
		`See <a data-reference-type="ref" data-reference="lnk:w">1</a>, <a data-reference-type="ref" data-reference="div:d">2</a>` +
		` and <a data-reference-type="ref" data-reference="code:c">3</a>.` + "\n"
	latex, findings := runFilter(t, stream, tables, false)
	wantContains(t, latex, `given in table \ref{v1-T:6}. The`, `\label{v1-T:6}`, `\label{v1-lnk:w}`,
		`\href{https://example.org}{web}`, `\label{v1-div:d}`, `\label{v1-code:c}`,
		`See \ref{v1-lnk:w}, \ref{v1-div:d} and \ref{v1-code:c}.`)
	wantAbsent(t, latex, `\label{T:6}`, `\ref{`+id+`}`)
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, jutePath)
}

var (
	referenceTargetRE = regexp.MustCompile(`\\(?:ref|eqref|cref|Cref)\{([^}]*)\}|\\hyperref\[([^\]]*)\]`)
	labelDefinitionRE = regexp.MustCompile(`\\label\{([^}]*)\}`)
)

// wantReferencesLabelled holds every reference the LaTeX makes to a label the
// LaTeX defines, and the LaTeX to at least one reference.
func wantReferencesLabelled(t *testing.T, latex string) {
	t.Helper()
	labels := map[string]bool{}
	for _, m := range labelDefinitionRE.FindAllStringSubmatch(latex, -1) {
		labels[m[1]] = true
	}
	references := 0
	for _, m := range referenceTargetRE.FindAllStringSubmatch(latex, -1) {
		for _, target := range strings.Split(m[1]+m[2], ",") {
			references++
			if !labels[target] {
				t.Errorf("%s names %s, which the output does not label", m[0], target)
			}
		}
	}
	if references == 0 {
		t.Errorf("the output makes no reference:\n%s", latex)
	}
}

// TestFilterGoldenConsumption runs the filter over the two consumption
// leaves as the export assembles them: navigation stripped, each first
// heading carrying its document id. Every reference the output makes names
// a label the output defines, and every finding is one the leaves earn.
func TestFilterGoldenConsumption(t *testing.T) {
	tables := fixtureTables()
	tables.Documents, tables.Paths = map[string]xrefDocument{}, map[string]string{}
	tables.Volumes = []xrefVolume{{Label: export.VolumeLabel(1), Title: "Consumption"}}
	tables.Bibliography = map[string]string{"Holm2018consumption": "Holm, M. B. *Consumption with liquidity constraints*. 2018."}
	firstHeading := regexp.MustCompile(`(?m)^#{1,6} .*$`)
	var stream strings.Builder
	for _, leaf := range []struct {
		path string
		rank int
	}{
		{"consumption/proof-of-proposition-2.md", 1},
		{"consumption/an-explicit-consumption-function-and-its/the-optimal-consumption-function.md", 2},
	} {
		text, err := os.ReadFile(filepath.Join("testdata", "golden", filepath.FromSlash(leaf.path)))
		if err != nil {
			t.Fatal(err)
		}
		id := tables.add(1, "consumption", leaf.path, leaf.rank)
		body := export.Strip(leaf.path, string(text))
		at := firstHeading.FindStringIndex(body)
		if at == nil {
			t.Fatalf("%s has no heading", leaf.path)
		}
		stream.WriteString(body[:at[1]] + " {#" + id + "}" + body[at[1]:] + "\n\n")
	}
	latex, findings := runFilter(t, stream.String(), tables, false)

	wantReferencesLabelled(t, latex)
	wantContains(t, latex, `\ref{v1-thm:cons_func}`, `\eqref{v1-eq:ODE}`, `\label{v1-eq:cstar_W0}`,
		`\begin{kb-theorem}\label{v1-thm:cons_func}`, `\bibitem{Holm2018consumption}`)

	expected := []filterFinding{
		{"anchor", "consumption/proof-of-proposition-2.md", "consumption/consumption-and-interest-rates/consumption-and-interest-rates-with-a-borrowing.md"},
		{"anchor", "consumption/an-explicit-consumption-function-and-its/the-optimal-consumption-function.md", "consumption/an-explicit-consumption-function-and-its/preferences-and-technology.md"},
	}
	if len(findings) != len(expected) {
		t.Fatalf("findings %+v, want one per entry of %+v", findings, expected)
	}
	for i, want := range expected {
		got := findings[i]
		if got.Form != want.Form || got.Path != want.Path || !strings.Contains(got.Reason, want.Reason) {
			t.Errorf("finding %d is %+v, want form %s at %s naming %s", i, got, want.Form, want.Path, want.Reason)
		}
	}
}

// The excerpts under testdata/html-forms are byte-exact line ranges of
// documents in the reference's arXiv KBs; that clone is transient and is not in
// this repository, so these files are the fixtures. Each line names the source
// document and range:
//
//	figure.md           2609.09888v1/kb-root/<figurePath>, lines 63-87
//	figure-unclosed.md  2609.09888v1/kb-root/<figurePath>, lines 71-74
//	table.md            2609.10366v1/kb-root/<tablePath>, lines 135-221
//	figure-table.md     2609.10394v1/kb-root/<figureTablePath>, lines 18-114
//	raw-image-duplicate.md  2609.10074v1/kb-root/<sliderPath>, lines 21-27
//	raw-image-quoted.md     2609.09834v1/kb-root/<teaserPath>, lines 28-32
//	figure-in-theorem.md    2609.00209v1/kb-root/<figureInTheoremPath>, lines 329-345
//	figure-ten-blocks.md    2609.10038v1/kb-root/<jutePath>, lines 59-122
const (
	figurePath      = "the-response-of-consumption-to-interest-rates/an-explicit-consumption-function-and-its/a-characterization.md"
	figureAsset     = "the-response-of-consumption-to-interest-rates/assets/Consumption_Function.pdf"
	tablePath       = "avsrbench-a-multi-condition-avsr-benchmark/experiment-and-results/camera-angle-and-speaker-articulation-tcd-timit/overview.md"
	figureTablePath = "candor-lr-a-dyadic-conversational-dataset-for/candor-cleaning-for-avsr/overview.md"
	sliderPath      = "strategic-communication-of-narratives-an/experiment-instructions/slider-aufgabe.md"
	sliderAsset     = "strategic-communication-of-narratives-an/assets/slider.png"
	teaserPath      = "semi-implicit-pairwise-descent-for-nonlocal/overview.md"
	teaserAsset     = "semi-implicit-pairwise-descent-for-nonlocal/assets/Fall4.png"

	figureInTheoremPath = "exit-times-for-brownian-motion-and-location/rectangular-domains.md"
	juteDir             = "parametric-bootstrap-simultaneous-confidence"
	jutePath            = juteDir + "/real-data-analysis.md"

	enclosuresVolume = "an-isoperimetric-problem-for-fourier-zeros-of"
	enclosuresPath   = enclosuresVolume + "/regular-polygons-in-the-plane/certified-enclosures-for.md"
	proofOfOneVolume = "monitoring-hierarchies-in-knowledge-production"
	proofOfOnePath   = proofOfOneVolume + "/appendix-omitted-proofs-and-technical-details/proof-of-1.md"
)

// htmlFormsStream is a stream of one document at path, of volume 1: its
// heading, then prose, then the excerpt.
func htmlFormsStream(t *testing.T, tables *xrefTables, path, prose, excerpt string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "html-forms", excerpt))
	if err != nil {
		t.Fatal(err)
	}
	volumeDir, _, _ := strings.Cut(path, "/")
	id := tables.add(1, volumeDir, path, 1)
	return "# Document {#" + id + "}\n\n" + prose + "\n\n" + string(text)
}

// TestFilterFigure: a built figure — an embed of the LaTeX source's path, the
// Markdown image of its copy, MathML in its notes, a caption, an id — is one
// LaTeX figure holding the copy, the notes' maths as TeX, and the caption
// labelled where the anchor before it points. The thm:cons_func anchor names
// a document the stream does not export.
func TestFilterFigure(t *testing.T) {
	tables := fixtureTables()
	tables.Assets[figureAsset] = "assets/" + figureAsset
	latex, findings := runFilter(t, htmlFormsStream(t, &tables, figurePath, "", "figure.md"), tables, false)
	wantContains(t, latex,
		`clearly seen in Figure \ref{v1-fig:Consumption_Function}, which plots`,
		`\begin{figure}[ht]`,
		`\includegraphics[keepaspectratio]{assets/`+figureAsset+`}`,
		`\(r=0.04, \gamma = 2, \rho = 0.08\), \(\underline{a}=-5\) and \(y=2\).`,
		`\caption{Consumption functions with and without borrowing constraint}\label{v1-fig:Consumption_Function}`)
	if n := strings.Count(latex, `\includegraphics`); n != 1 {
		t.Errorf("the figure holds %d images, want its copy alone:\n%s", n, latex)
	}
	wantAbsent(t, latex, "<math", "annotation")
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, figurePath, "anchor")
}

// TestFilterFigureMissingAsset: an image naming no copied file is its alt
// text and a finding, the copy's Markdown image and the embed alike.
func TestFilterFigureMissingAsset(t *testing.T) {
	tables := fixtureTables()
	latex, findings := runFilter(t, htmlFormsStream(t, &tables, figurePath, "", "figure.md"), tables, false)
	wantContains(t, latex, `\begin{figure}[ht]`, `\label{v1-fig:Consumption_Function}`)
	wantAbsent(t, latex, `\includegraphics`)
	wantFindings(t, findings, figurePath, "anchor", "image", "link", "image")
}

// TestFilterHTMLTable: a built html table — a caption, an id, a rowspan cell,
// MathML in its header cells — is a LaTeX table with its caption labelled,
// the cell spanning its rows, and the maths as TeX.
func TestFilterHTMLTable(t *testing.T) {
	tables := fixtureTables()
	prose := `See Table <a href="overview.md#tab:matched_volunteers" data-reference-type="ref" data-reference="tab:matched_volunteers">1</a>.`
	latex, findings := runFilter(t, htmlFormsStream(t, &tables, tablePath, prose, "table.md"), tables, false)
	wantContains(t, latex,
		`See Table \ref{v1-tab:matched_volunteers}.`,
		`\caption{WER (\%) in TCD-TIMIT across varying modality for Lipspeakers, Volunteers, and Matched Volunteers.}\label{v1-tab:matched_volunteers}`,
		`\multirow{3}`,
		`\textbf{LS \(\downarrow\)}`,
		`\textbf{\(\Delta\) (LS\(-\)MV)}`)
	wantAbsent(t, latex, "<math", "annotation")
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, tablePath)
}

// TestFilterTableWidths: a table of default column widths too wide for the
// line gets relative widths — a column whose longest cell fits an equal share
// of what is left keeps that width, the rest share the remainder — while a
// table that fits, or one that carries widths, is written as it stands, and
// every table keeps its rows where they were.
func TestFilterTableWidths(t *testing.T) {
	long := strings.Repeat("word ", 40)
	cases := []struct {
		name, table string
		want        []string
	}{
		{"too wide", "| Key | Value | Note |\n|---|---|---|\n| a | tenletters | " + long + "|\n| b | short | c |\n",
			// Line 96 less two gaps of 3 is 90: Key 3 and tenletters 10 fit
			// their share, widened to their words with two fifths' slack,
			// 4.2 and 14; Note takes the other 71.8.
			[]string{`\real{0.0467}`, `\real{0.1556}`, `\real{0.7978}`}},
		{"fits", "| Key | Value |\n|---|---|\n| a | tenletters |\n| b | short |\n", []string{`\begin{longtable}[]{@{}ll@{}}`}},
		{"carries widths", `<table><colgroup><col style="width: 30%"><col style="width: 70%"></colgroup>` +
			"<tr><td>a</td><td>tenletters " + long + "</td></tr><tr><td>b</td><td>short</td></tr></table>\n",
			[]string{`\real{0.3000}`, `\real{0.7000}`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			latex, findings := runFilter(t, "# A {#d1-a}\n\n"+c.table, fixtureTables(), false)
			wantContains(t, latex, c.want...)
			for _, cell := range []string{"tenletters", "short"} {
				if n := strings.Count(latex, cell); n != 1 {
					t.Errorf("%q is written %d times, want once:\n%s", cell, n, latex)
				}
			}
			wantFindings(t, findings, "alpha/a.md")
		})
	}
}

// TestFilterFigureHoldingTable: the LaTeX writer writes a figure holding a
// table without its float, caption or label, so the figure is its body and
// caption, unfloated, and a finding; its label is not defined, so the anchor
// to it falls back to its document.
func TestFilterFigureHoldingTable(t *testing.T) {
	tables := fixtureTables()
	latex, findings := runFilter(t, htmlFormsStream(t, &tables, figureTablePath, "", "figure-table.md"), tables, false)
	wantContains(t, latex,
		`shown in Figure \ref{d1-candor-cleaning-for-avsr-overview}.`,
		`\(\llcorner\) \textbf{raw/}`,
		`Directory structure of CANDOR dataset.`)
	wantAbsent(t, latex, `\begin{figure}`, "fig:candor-structure")
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, figureTablePath, "figure")
}

// TestFilterRawImages: a raw <img> or <embed> outside a figure is dropped
// where its block list holds a Markdown image of its file name (the copy a
// built KB writes beside it); otherwise it is an image through the assets
// table, or a finding.
func TestFilterRawImages(t *testing.T) {
	t.Run("duplicate of a Markdown image in its div", func(t *testing.T) {
		tables := fixtureTables()
		tables.Assets[sliderAsset] = "assets/" + sliderAsset
		latex, findings := runFilter(t, htmlFormsStream(t, &tables, sliderPath, "", "raw-image-duplicate.md"), tables, false)
		if n := strings.Count(latex, `\includegraphics`); n != 1 {
			t.Errorf("%d images, want the copy alone:\n%s", n, latex)
		}
		wantContains(t, latex, `\includegraphics[keepaspectratio]{assets/`+sliderAsset+`}`)
		wantFindings(t, findings, sliderPath)
	})
	t.Run("Markdown image outside its blockquote", func(t *testing.T) {
		tables := fixtureTables()
		tables.Assets[teaserAsset] = "assets/" + teaserAsset
		latex, findings := runFilter(t, htmlFormsStream(t, &tables, teaserPath, "", "raw-image-quoted.md"), tables, false)
		wantContains(t, latex, `\includegraphics[keepaspectratio]{assets/`+teaserAsset+`}`)
		wantFindings(t, findings, teaserPath, "image")
	})
	t.Run("copied file", func(t *testing.T) {
		tables := fixtureTables()
		tables.Assets["alpha/f.png"] = "assets/alpha/f.png"
		latex, findings := runFilter(t, "# A {#d1-a}\n\n<img src=\"f.png\" />\n\nText <embed src=\"f.png\" /> here.\n", tables, false)
		if n := strings.Count(latex, `\includegraphics[keepaspectratio]{assets/alpha/f.png}`); n != 2 {
			t.Errorf("%d images of the copy, want 2:\n%s", n, latex)
		}
		wantFindings(t, findings, "alpha/a.md")
	})
}

// TestFilterLabelSpelling: an author label outside the characters the LaTeX
// writer prints unchanged is written one way wherever it appears — a span, a
// figure id and a maths \label, an anchor and a maths \eqref — so every
// reference finds its label; two labels written alike are a finding.
func TestFilterLabelSpelling(t *testing.T) {
	stream := "# A {#d1-a}\n\n<span id=\"lem: one\">L</span>\n\n``` math\nx \\label{eq: two}\n```\n\n" +
		"<figure id=\"fig: three\">\n<img src=\"f.png\" />\n<figcaption>Cap</figcaption>\n</figure>\n\n" +
		"See <a data-reference-type=\"ref\" data-reference=\"lem: one\">1</a>, " +
		"<a href=\"a.md#fig: three\" data-reference-type=\"ref\" data-reference=\"fig: three\">2</a> and $`\\eqref{eq: two}`$.\n"
	tables := fixtureTables()
	tables.Assets["alpha/f.png"] = "assets/alpha/f.png"
	latex, findings := runFilter(t, stream, tables, false)
	wantContains(t, latex, `\label{v1-lem:ux20one}`, `\label{v1-eq:ux20two}`, `\label{v1-fig:ux20three}`,
		`\ref{v1-lem:ux20one}`, `\ref{v1-fig:ux20three}`, `\eqref{v1-eq:ux20two}`)
	wantReferencesLabelled(t, latex)
	wantFindings(t, findings, "alpha/a.md")

	_, findings = runFilter(t, "# A {#d1-a}\n\n<span id=\"a b\">x</span> <span id=\"aux20b\">y</span>\n", fixtureTables(), false)
	wantFindings(t, findings, "alpha/a.md", "label")
}

// TestFilterUnrecoveredFigure: a figure whose close never comes stays raw
// html, which the writer drops, so it is a finding.
func TestFilterUnrecoveredFigure(t *testing.T) {
	tables := fixtureTables()
	tables.Assets[figureAsset] = "assets/" + figureAsset
	latex, findings := runFilter(t, htmlFormsStream(t, &tables, figurePath, "", "figure-unclosed.md"), tables, false)
	wantAbsent(t, latex, `\begin{figure}`)
	wantFindings(t, findings, figurePath, "figure")
}
