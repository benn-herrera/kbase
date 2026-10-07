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

	"kbase/internal/kb"
	"kbase/internal/kbload"
)

// adjagentFixture is kb_tools' claim-sheet fixture
// (adjagent kb_tools/tests/fixtures/claim-graph-sheet), copied as it stands: a
// 1.0.0 KB, its unmarked build record, and the DOT kb_tools composes for it.
const adjagentFixture = "testdata/adjagent-sheet"

func openAdjagentFixture(t *testing.T) *kb.Source {
	t.Helper()
	src, err := kbload.Open(filepath.Join(adjagentFixture, "kb-root"))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// pinnedAdjagentFixture is the fixture adjagentFixture copies, in the pinned
// adjagent clone.
const pinnedAdjagentFixture = "../../.claude/adjagent/kb_tools/tests/fixtures/claim-graph-sheet"

// TestAdjagentFixtureIsThePinned: the copy holds the files the pinned
// clone's fixture holds, byte for byte; it skips where the clone is absent.
func TestAdjagentFixtureIsThePinned(t *testing.T) {
	if _, err := os.Stat(pinnedAdjagentFixture); err != nil {
		t.Skipf("%s is absent; the adjagent clone is not here", pinnedAdjagentFixture)
	}
	copied, pinned := fixtureFiles(t, adjagentFixture), fixtureFiles(t, pinnedAdjagentFixture)
	for rel, data := range copied {
		if want, ok := pinned[rel]; !ok {
			t.Errorf("%s is not in the pinned fixture", rel)
		} else if !bytes.Equal(data, want) {
			t.Errorf("%s differs from the pinned fixture's; re-copy it from %s", rel, pinnedAdjagentFixture)
		}
	}
	for rel := range pinned {
		if _, ok := copied[rel]; !ok {
			t.Errorf("the pinned fixture's %s is not copied", rel)
		}
	}
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

// TestAdjagentSheetGoldens: kbase draws kb_tools' fixture as the full sheet,
// the digest and the sheets of the two volumes holding nodes — none for
// gamma, which holds none — and its sheets are kb_tools' DOT, byte for byte.
func TestAdjagentSheetGoldens(t *testing.T) {
	g, err := load(openAdjagentFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	drawn := map[string]string{}
	var paths []string
	for _, s := range g.sheets() {
		drawn[s.path] = s.dot
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
	got := drawnEdges(rows, map[pair]bool{{"a", "b"}: true})
	if len(got) != 1 || got[0].source != "a" || got[0].target != "b" ||
		got[0].stroke.relation != kb.RelationDemoted || got[0].stroke.provenance != kb.OriginCited {
		t.Errorf("drawnEdges = %+v, want the a → b cut alone, cited", got)
	}
}

func TestReduce(t *testing.T) {
	stroked := func(s *stroke) func(string, string) edge {
		return func(source, target string) edge { return edge{source: source, target: target, stroke: s} }
	}
	dep, asked := stroked(strokeFor(kb.RelationDepends, false)), stroked(strokeFor(kb.RelationDepends, true))
	restsOn, demoted := stroked(strokeFor(kb.RelationRestsOn, false)), stroked(strokeFor(kb.RelationDemoted, false))
	for _, tc := range []struct {
		name  string
		edges []edge
		want  []string
	}{
		{"chain shortcut dropped", []edge{dep("a", "b"), dep("a", "c"), dep("b", "c")}, []string{"ab", "bc"}},
		{"long path drops the shortcut", []edge{dep("a", "b"), dep("a", "d"), dep("b", "c"), dep("c", "d")}, []string{"ab", "bc", "cd"}},
		{"diamond kept", []edge{dep("a", "b"), dep("a", "c"), dep("b", "d"), dep("c", "d")}, []string{"ab", "ac", "bd", "cd"}},
		{"a path through a non-depends edge does not count", []edge{dep("a", "b"), dep("a", "c"), restsOn("b", "c")}, []string{"ab", "ac", "bc"}},
		{"a demoted edge neither reduces nor is reduced", []edge{dep("a", "b"), demoted("a", "c"), dep("b", "c")}, []string{"ab", "ac", "bc"}},
		{"an inferred edge is a depends edge", []edge{dep("a", "b"), dep("a", "c"), asked("b", "c")}, []string{"ab", "bc"}},
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
		"> **Lemma**\n> **Lemma 2** (Small). *Holds.*\n> More. <!-- claim-quality: clm-blk001 -->\n\n" +
		"> **Note.** An aside. <!-- claim-quality: clm-not001 -->\n\n" +
		"> Plain quote. <!-- claim-quality: clm-qte001 -->\n\n" +
		">\n> **Theorem**\n> **Theorem 9.** Holds. <!-- claim-quality: clm-gap001 -->\n\n" +
		"> <span id=\"prop:p\">**Proposition 3**</span> (P).\n>\n> Holds. <!-- claim-quality: clm-spn001 -->\n\n" +
		"> A plain opening, *then emphasis*. <!-- claim-quality: clm-emp001 -->\n\n" +
		"``` math\na = b <!-- claim-quality: clm-eqn001 -->\n```\n\n" +
		"```\ncode <!-- claim-quality: clm-cod001 -->\n```\n\n" +
		"Prose. <!-- claim-quality: clm-prs001 -->\n\n" +
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
// drawing, references alone being undrawn.
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
	for _, r := range kb.Relations {
		if drawn := strokeFor(r, false) != nil; drawn == (r == kb.RelationReferences) {
			t.Errorf("relation %q: drawn %t", r, drawn)
		}
	}
	if strokeFor(kb.RelationDepends, true) == strokeFor(kb.RelationDepends, false) {
		t.Error("an inferred depends edge draws as a cited one")
	}
	if len(ladderFills) != len(kb.BuildBandLadder) {
		t.Errorf("%d fills for a ladder of %d bands", len(ladderFills), len(kb.BuildBandLadder))
	}
}

// TestDOTQuoting: a quote, a backslash and markup in the KB's title, a
// volume's title and a node title stay inside their DOT strings, and a
// tooltip's backslash survives Graphviz's escString pass.
func TestDOTQuoting(t *testing.T) {
	dep := strokeFor(kb.RelationDepends, false)
	g := &graph{kbTitle: `K "B" \x`,
		volumes: []volume{{"v", `A "quoted" \title <x>`, "v/" + kb.IndexFile}, {"w", "W", "w/" + kb.IndexFile}, {"", `K "B" \x`, kb.EntryPointFile}},
		nodes: []node{
			{id: "clm-aaaaaa", kind: kindProse, title: `Says "so" \lambda`, href: "v/claim-quality.md#says", band: kb.UnknownBandSlug, group: "v"},
			{id: "clm-bbbbbb", kind: kindProse, title: "B", href: "claim-quality.md#b", band: kb.UnknownBandSlug},
			{id: "clm-cccccc", kind: kindProse, title: "C", href: "w/claim-quality.md#c", band: kb.UnknownBandSlug, group: "w"},
		},
		edges: []edge{{source: "clm-aaaaaa", target: "clm-bbbbbb", stroke: dep}}}
	sheets := g.sheets()
	for _, want := range []string{`label=<<b>K &quot;B&quot; \x</b>>]`, `label=<<b>A &quot;quoted&quot; \title &lt;x&gt;</b>`,
		`label="clm-aaaaaa\nSays \"so\" \\lambda"`, `tooltip="clm-aaaaaa [prose, *pending*] Says \"so\" \\\\lambda"`} {
		if !strings.Contains(sheets[0].dot, want) {
			t.Errorf("full sheet lacks %s:\n%s", want, sheets[0].dot)
		}
	}
	dot, err := exec.LookPath("dot")
	if err != nil {
		return
	}
	for _, s := range sheets {
		if _, err := renderSVG(dot, s.dot); err != nil {
			t.Errorf("%s does not parse: %v", s.path, err)
		}
	}
}

// TestVolumeCountRule: one volume holding nodes, beside the root bucket,
// draws the full sheet alone, with no link to a digest.
func TestVolumeCountRule(t *testing.T) {
	g := &graph{kbTitle: "K", volumes: []volume{{"v", "V", "v/" + kb.IndexFile}, {"", "K", kb.EntryPointFile}},
		nodes: []node{
			{id: "clm-aaaaaa", kind: kindProse, href: "v/claim-quality.md", band: kb.UnknownBandSlug, group: "v"},
			{id: "clm-bbbbbb", kind: kindProse, href: "claim-quality.md", band: kb.UnknownBandSlug},
		},
		edges: []edge{{source: "clm-aaaaaa", target: "clm-bbbbbb", stroke: strokeFor(kb.RelationDepends, false)}}}
	sheets := g.sheets()
	if len(sheets) != 1 || sheets[0].path != kb.ClaimGraphFile || strings.Contains(sheets[0].dot, kb.ClaimGraphDigestFile) {
		t.Errorf("one volume: sheets %v, want the full sheet alone, unlinked", sheets)
	}
}

func TestRenderWithoutDot(t *testing.T) {
	src := openAdjagentFixture(t)
	t.Setenv("PATH", "")
	if _, err := Render(src); !errors.Is(err, ErrNoDot) {
		t.Errorf("Render with no PATH = %v, want ErrNoDot", err)
	}
}

func TestRenderDraws(t *testing.T) {
	sheets, err := Render(openAdjagentFixture(t))
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
	for name, text := range map[string]string{"cites.yaml": "c\n", "claims.yaml": "a\n", "depends-on.yaml": "",
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
