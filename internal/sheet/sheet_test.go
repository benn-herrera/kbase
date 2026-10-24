package sheet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/kbload"
)

// adjagentFixture is the claim-sheet fixture: a 1.0.0 KB, its unmarked build
// record, and the DOT the sheets compose to.
const adjagentFixture = "testdata/adjagent-sheet"

func openAdjagentFixture(t *testing.T) *kb.Source {
	t.Helper()
	src, err := kbload.Open(filepath.Join(adjagentFixture, "kb-root"))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// committedRows is every row of T's index file as src's committed .index/
// holds it.
func committedRows[T kb.IndexRowType](t *testing.T, src *kb.Source) []T {
	t.Helper()
	path, rows, problems, err := kb.ReadIndex[T](src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("%s:%d: %v", path, problems[0].Line, problems[0].Err)
	}
	return rows
}

// renderCommitted is Render over src's committed .index/.
func renderCommitted(t *testing.T, src *kb.Source) ([]Sheet, error) {
	t.Helper()
	return Render(src, committedRows[kb.NodeRow](t, src), committedRows[kb.EdgeRow](t, src), committedRows[kb.CiteRow](t, src))
}

// loadCommitted is load over src's committed .index/.
func loadCommitted(t *testing.T, src *kb.Source) *graph {
	t.Helper()
	g, err := load(src, committedRows[kb.NodeRow](t, src), committedRows[kb.EdgeRow](t, src), committedRows[kb.CiteRow](t, src))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// fixtureFiles is every file under dir, by slash path relative to it.
func fixtureFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestAdjagentSheetGoldens: kbase draws the fixture as the full sheet,
// the digest and the sheets of the two volumes holding nodes — none for
// gamma, which holds none — and its sheets are the goldens' DOT, byte for byte.
func TestAdjagentSheetGoldens(t *testing.T) {
	g := loadCommitted(t, openAdjagentFixture(t))
	drawn := map[string]string{}
	var paths []string
	for _, s := range g.sheets() {
		drawn[s.path] = s.dot(false)
		paths = append(paths, s.path)
	}
	if want := []string{kb.ClaimGraphFile, kb.ClaimGraphDigestFile, "alpha/" + kb.ClaimGraphFile, "beta/" + kb.ClaimGraphFile}; !slices.Equal(paths, want) {
		t.Errorf("sheets %q, want %q", paths, want)
	}
	for golden, path := range map[string]string{"claim-graph.dot": kb.ClaimGraphFile, "claim-graph-digest.dot": kb.ClaimGraphDigestFile,
		"alpha-claim-graph.dot": "alpha/" + kb.ClaimGraphFile} {
		data, err := os.ReadFile(filepath.Join(adjagentFixture, golden))
		if err != nil {
			t.Fatal(err)
		}
		want := string(data)
		got, ok := drawn[path]
		if !ok {
			t.Errorf("no sheet %s", path)
			continue
		}
		if got != want {
			gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
			for i := range max(len(gl), len(wl)) {
				if i >= len(gl) || i >= len(wl) || gl[i] != wl[i] {
					t.Errorf("%s differs from %s at line %d:\ngot:  %s\nwant: %s", path, golden, i+1, at(gl, i), at(wl, i))
					break
				}
			}
		}
	}
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(none)"
}

// TestDrawnEdges covers what the fixture lacks: a demoted row with no origin
// draws as the cited cut, whatever the unmarked ask answered for its pair,
// and a relation with no stroke is not drawn.
func TestDrawnEdges(t *testing.T) {
	rows := []kb.EdgeRow{{Source: "a", Target: "b", Relation: kb.RelationDemoted}, {Source: "c", Target: "b", Relation: "unknown"}}
	got := drawnEdges(rows, func(source, target string) bool { return source == "a" && target == "b" })
	if len(got) != 1 || got[0].source != "a" || got[0].target != "b" ||
		got[0].stroke.relation != kb.RelationDemoted || got[0].stroke.provenance != kb.OriginCited {
		t.Errorf("drawnEdges = %+v, want the a → b cut alone, cited", got)
	}
}

func TestReduce(t *testing.T) {
	stroked := func(s *stroke) func(string, string) edge {
		return func(source, target string) edge { return edge{source: source, target: target, stroke: s} }
	}
	dep, asked := stroked(strokeFor(kb.RelationSupports, false)), stroked(strokeFor(kb.RelationSupports, true))
	founds, demoted := stroked(strokeFor(kb.RelationFounds, false)), stroked(strokeFor(kb.RelationDemoted, false))
	for _, tc := range []struct {
		name  string
		edges []edge
		want  []string
	}{
		{"chain shortcut dropped", []edge{dep("a", "b"), dep("a", "c"), dep("b", "c")}, []string{"ab", "bc"}},
		{"long path drops the shortcut", []edge{dep("a", "b"), dep("a", "d"), dep("b", "c"), dep("c", "d")}, []string{"ab", "bc", "cd"}},
		{"diamond kept", []edge{dep("a", "b"), dep("a", "c"), dep("b", "d"), dep("c", "d")}, []string{"ab", "ac", "bd", "cd"}},
		{"a path through a non-supports edge does not count", []edge{dep("a", "b"), dep("a", "c"), founds("b", "c")}, []string{"ab", "ac", "bc"}},
		{"a demoted edge neither reduces nor is reduced", []edge{dep("a", "b"), demoted("a", "c"), dep("b", "c")}, []string{"ab", "ac", "bc"}},
		{"an inferred edge is a supports edge", []edge{dep("a", "b"), dep("a", "c"), asked("b", "c")}, []string{"ab", "bc"}},
		{"a cycle keeps every reachability", []edge{dep("a", "b"), dep("a", "c"), dep("b", "a"), dep("b", "c")}, []string{"ab", "ba", "bc"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, e := range reduce(tc.edges) {
				got = append(got, e.source+e.target)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("reduce = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClaimKind(t *testing.T) {
	leaf := "# L\n\n" +
		"> **Lemma**\n> **Lemma 2** (Small). *Holds.*\n> More. <!-- claim-rigor: clm-blk001 -->\n\n" +
		"> **Note.** An aside. <!-- claim-rigor: clm-not001 -->\n\n" +
		"> Plain quote. <!-- claim-rigor: clm-qte001 -->\n\n" +
		">\n> **Theorem**\n> **Theorem 9.** Holds. <!-- claim-rigor: clm-gap001 -->\n\n" +
		"> <span id=\"prop:p\">**Proposition 3**</span> (P).\n>\n> Holds. <!-- claim-rigor: clm-spn001 -->\n\n" +
		"> A plain opening, *then emphasis*. <!-- claim-rigor: clm-emp001 -->\n\n" +
		"``` math\na = b <!-- claim-rigor: clm-eqn001 -->\n```\n\n" +
		"```\ncode <!-- claim-rigor: clm-cod001 -->\n```\n\n" +
		"Prose. <!-- claim-rigor: clm-prs001 -->\n\n" +
		"``` math\nc\n\\label{eq:c}\n```\n"
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "v"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "v", "l.md"), []byte(leaf), 0o644); err != nil {
		t.Fatal(err)
	}
	r := kindReader{src: kb.OnDisk(root), texts: map[string]string{}}
	for _, tc := range []struct{ id, title, want string }{
		{"clm-blk001", "Lemma 2", kindBlock},
		{"clm-qte001", "Plain", kindProse},
		{"clm-not001", "Note", kindBlock},
		{"clm-spn001", "The governance bifurcation", kindBlock},
		{"clm-emp001", "Theorem 4", kindProse},
		{"clm-eqn001", "An equation", kindEquation},
		{"clm-cod001", "Code", kindProse},
		{"clm-prs001", "Prose", kindProse},
		{"clm-gap001", "Theorem 9", kindBlock},
		{"clm-unm001", "Equation (`eq:c`) — L", kindEquation},
		{"clm-unm002", "Theorem 5.11 — Formal Stability", kindBlock},
		{"clm-unm003", "Lemma: Forward Invariance", kindBlock},
		{"clm-unm004", "Theorem of the mean", kindBlock},
		{"clm-unm005", "Test example 2", kindBlock},
		{"clm-unm006", "Three word name 4", kindProse},
		{"clm-unm007", "So it goes, and so on", kindProse},
		{"clm-unm008", "Note that it holds", kindProse},
	} {
		if got := r.claimKind(tc.id, tc.title, []string{"v/l.md", "v/missing.md"}); got != tc.want {
			t.Errorf("claimKind(%s, %q) = %q; want %q", tc.id, tc.title, got, tc.want)
		}
	}
}

func TestWrapAsPython(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"Proposition 4.4 — KPI-Gating as Discrete Pontryagin", []string{"Proposition 4.4 — KPI-", "Gating as Discrete", "Pontryagin"}},
		{"Supercalifragilisticexpialidocious-words here", []string{"Supercalifragilisticex", "pialidocious-words", "here"}},
		{"a  b", []string{"a  b"}},
		{"Equation (eq:linkattack) — Overview of the whole thing", []string{"Equation", "(eq:linkattack) —", "Overview of the whole", "thing"}},
	} {
		if got := wrap(tc.text, 22); !slices.Equal(got, tc.want) {
			t.Errorf("wrap(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// TestStyleTablesAreTotal: every node kind, relation and band has its
// drawing, references alone being undrawn, and the legend lists the node kinds
// in kb.NodeKinds order after a claim's three.
func TestStyleTablesAreTotal(t *testing.T) {
	for _, k := range kb.NodeKinds {
		claim := k == kb.NodeKindClaim
		if styled := slices.ContainsFunc(nodeStyles, func(s nodeStyle) bool { return s.kind == k }); styled == claim {
			t.Errorf("node kind %q: styled %t", k, styled)
		}
	}
	for _, k := range []string{kindBlock, kindEquation, kindProse} {
		if styleOf(k).kind != k {
			t.Errorf("claim kind %q has no style", k)
		}
	}
	var nodeKinds []string
	for _, s := range nodeStyles {
		if !slices.Contains([]string{kindBlock, kindEquation, kindProse}, s.kind) {
			nodeKinds = append(nodeKinds, s.kind)
		}
	}
	if want := kb.NodeKinds[1:]; !slices.Equal(nodeKinds, want) {
		t.Errorf("legend node kinds %q, want %q", nodeKinds, want)
	}
	for _, r := range kb.Relations {
		if drawn := strokeFor(r, false) != nil; drawn == (r == kb.RelationReferences) {
			t.Errorf("relation %q: drawn %t", r, drawn)
		}
	}
	if strokeFor(kb.RelationSupports, true) == strokeFor(kb.RelationSupports, false) {
		t.Error("an inferred supports edge draws as a cited one")
	}
	if len(ladderFills) != len(kb.BuildBandLadder) {
		t.Errorf("%d fills for a ladder of %d bands", len(ladderFills), len(kb.BuildBandLadder))
	}
}

// TestPlural: the digest counts the nodes of a kind by its plural, analyses
// for analysis, and a claim's prose kind by its own name.
func TestPlural(t *testing.T) {
	for _, tc := range []struct{ kind, want string }{
		{kb.NodeKindAnalysis, "analyses"},
		{kb.NodeKindExperiment, "experiments"},
		{kb.NodeKindWork, "works"},
		{kindProse, kindProse},
	} {
		if got := plural(tc.kind); got != tc.want {
			t.Errorf("plural(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// TestDOTQuoting: a quote, a backslash and markup in the KB's title, a
// volume's title and a node title stay inside their DOT strings, and a
// tooltip's backslash survives Graphviz's escString pass.
func TestDOTQuoting(t *testing.T) {
	dep := strokeFor(kb.RelationSupports, false)
	g := &graph{kbTitle: `K "B" \x`,
		volumes: []volume{{"v", `A "quoted" \title <x>`, "v/" + kb.IndexFile}, {"w", "W", "w/" + kb.IndexFile}, {"", `K "B" \x`, kb.EntryPointFile}},
		nodes: []node{
			{id: "clm-aaaaaa", kind: kindProse, title: `Says "so" \lambda`, href: "v/claim-rigor.md#says", band: kb.UnknownBandSlug, group: "v"},
			{id: "clm-bbbbbb", kind: kindProse, title: "B", href: "claim-rigor.md#b", band: kb.UnknownBandSlug},
			{id: "clm-cccccc", kind: kindProse, title: "C", href: "w/claim-rigor.md#c", band: kb.UnknownBandSlug, group: "w"},
		},
		edges: []edge{{source: "clm-aaaaaa", target: "clm-bbbbbb", stroke: dep}}}
	sheets := g.sheets()
	full := sheets[0].dot(false)
	for _, want := range []string{`label=<<b>K &quot;B&quot; \x</b>>]`, `label=<<b>A &quot;quoted&quot; \title &lt;x&gt;</b>`,
		`label="clm-aaaaaa\nSays \"so\" \\lambda"`, `tooltip="clm-aaaaaa [prose, *pending*] Says \"so\" \\\\lambda"`} {
		if !strings.Contains(full, want) {
			t.Errorf("full sheet lacks %s:\n%s", want, full)
		}
	}
	if _, err := exec.LookPath("dot"); err != nil {
		return
	}
	for _, s := range sheets {
		if _, err := renderSVG(s.dot); err != nil {
			t.Errorf("%s does not parse: %v", s.path, err)
		}
	}
}

// TestGhostDraws: an edge end no node row carries draws as a ghost — a red
// dashed box with no href, a tooltip saying no record carries it, and a
// legend row. A refreshed KB never carries one, the index refusing an edge
// whose source it cannot name.
func TestGhostDraws(t *testing.T) {
	g := &graph{kbTitle: "K", volumes: []volume{{"", "K", kb.EntryPointFile}},
		nodes: []node{
			{id: "clm-aaaaaa", kind: kindProse, title: "A", href: "claim-rigor.md#a", band: kb.UnknownBandSlug},
			{id: "clm-zzz999", kind: kindGhost},
		},
		edges: []edge{{source: "clm-zzz999", target: "clm-aaaaaa", stroke: strokeFor(kb.RelationSupports, false)}}}
	full := g.sheets()[0].dot(false)
	for _, want := range []string{
		`"clm-zzz999" [shape=box style="dashed" penwidth=1 color="#c0392b" label="clm-zzz999" tooltip="clm-zzz999 — no record carries this id"]`,
		`<td border="1" color="#c0392b" style="dashed">ghost</td><td align="left">an id no record carries</td>`,
	} {
		if !strings.Contains(full, want) {
			t.Errorf("the sheet lacks %s:\n%s", want, full)
		}
	}
}

// TestVolumeCountRule: one volume holding nodes, beside the root bucket,
// draws the full sheet alone, with no link to a digest.
func TestVolumeCountRule(t *testing.T) {
	g := &graph{kbTitle: "K", volumes: []volume{{"v", "V", "v/" + kb.IndexFile}, {"", "K", kb.EntryPointFile}},
		nodes: []node{
			{id: "clm-aaaaaa", kind: kindProse, href: "v/claim-rigor.md", band: kb.UnknownBandSlug, group: "v"},
			{id: "clm-bbbbbb", kind: kindProse, href: "claim-rigor.md", band: kb.UnknownBandSlug},
		},
		edges: []edge{{source: "clm-aaaaaa", target: "clm-bbbbbb", stroke: strokeFor(kb.RelationSupports, false)}}}
	sheets := g.sheets()
	if len(sheets) != 1 || sheets[0].path != kb.ClaimGraphFile || strings.Contains(sheets[0].dot(false), kb.ClaimGraphDigestFile) {
		t.Errorf("one volume: sheets %v, want the full sheet alone, unlinked", sheets)
	}
}

// TestLadderThroughFakeDot drives renderSVG over every sheet the emitter
// composes for the adjagent fixture, with a dot stand-in that refuses any DOT
// lacking splines=line: the first rung fails and the emitter's retry DOT
// renders; where the retry fails too, the first error comes back.
func TestLadderThroughFakeDot(t *testing.T) {
	g := loadCommitted(t, openAdjagentFixture(t))
	dir := t.TempDir()
	fake := filepath.Join(dir, "dot")
	script := "#!/bin/sh\ninput=$(cat)\nif printf '%s' \"$input\" | grep -q 'splines=line'; then echo '<svg/>'; exit 0; fi\necho 'routesplines failed' >&2\nexit 1\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, s := range g.sheets() {
		if svg, err := renderSVG(s.dot); err != nil || string(svg) != "<svg/>\n" {
			t.Errorf("%s: renderSVG with fallback = %q, %v; want <svg/>", s.path, svg, err)
		}
		unlined := func(bool) string { return s.dot(false) }
		if _, err := renderSVG(unlined); err == nil || !strings.Contains(err.Error(), "routesplines failed") {
			t.Errorf("%s: renderSVG where both rungs fail = %v, want the first error", s.path, err)
		}
	}
}

// copiedAdjagentFixture is a writable copy of the adjagent fixture's
// repository.
func copiedAdjagentFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, data := range fixtureFiles(t, adjagentFixture) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// fullSheetEdge is the full sheet's DOT line for the edge source → target in
// the KB at root.
func fullSheetEdge(t *testing.T, root, source, target string) string {
	t.Helper()
	src, err := kbload.Open(filepath.Join(root, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	g := loadCommitted(t, src)
	edgeLine := `"` + source + `" -> "` + target + `" [`
	for line := range strings.SplitSeq(g.full(false), "\n") {
		if strings.Contains(line, edgeLine) {
			return line
		}
	}
	t.Fatalf("the full sheet draws no %s edge", edgeLine)
	return ""
}

// TestCullLinkDrawsInferredEitherWay: a depends row whose direction runs
// against the cull record's link — target first in id order — draws
// inferred; the same KB without the record draws it cited.
func TestCullLinkDrawsInferredEitherWay(t *testing.T) {
	root := copiedAdjagentFixture(t)
	if line := fullSheetEdge(t, root, "clm-aaa001", "clm-bbb001"); strings.Contains(line, "style=dashed") {
		t.Errorf("without a cull record: %s, want the cited stroke", line)
	}
	record := "links:\n  - a: clm-aaa001\n    b: clm-bbb001\n    harvest: adjacent\n"
	if err := os.WriteFile(filepath.Join(root, buildrecords.CullFile), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	if line := fullSheetEdge(t, root, "clm-aaa001", "clm-bbb001"); !strings.Contains(line, "style=dashed") {
		t.Errorf("with the link recorded: %s, want the inferred stroke", line)
	}
}

func TestRenderWithoutDot(t *testing.T) {
	src := openAdjagentFixture(t)
	t.Setenv("PATH", "")
	if _, err := renderCommitted(t, src); !errors.Is(err, ErrNoDot) {
		t.Errorf("Render with no PATH = %v, want ErrNoDot", err)
	}
}

func TestRenderDraws(t *testing.T) {
	sheets, err := renderCommitted(t, openAdjagentFixture(t))
	if errors.Is(err, ErrNoDot) {
		t.Skip("no Graphviz dot on PATH")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) == 0 {
		t.Fatal("Render drew no sheet")
	}
	for _, s := range sheets {
		if !bytes.Contains(s.SVG, []byte(`<svg width="100%"`)) {
			t.Errorf("%s: the root is not fitted to the viewer", s.Path)
		}
	}
}

func TestPlaceholderDigestsTheIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".index"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"cites.yaml": "c\n", "claims.yaml": "a\n", "edges.yaml": "",
		"notes.txt": "ignored", "claims.jsonl": "ignored"} {
		if err := os.WriteFile(filepath.Join(root, ".index", name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svg, err := Placeholder(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("c\na\n"))
	for _, want := range []string{">no Graphviz dot on PATH<", "index sha256:" + hex.EncodeToString(sum[:])[:12] + "<"} {
		if !bytes.Contains(svg, []byte(want)) {
			t.Errorf("Placeholder = %s, want it to show %q", svg, want)
		}
	}
}
