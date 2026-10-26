package sheet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// TestDrawnEdges covers what the fixture lacks: demoted rows of one pair
// draw as one cut, and a relation with no stroke is not drawn.
func TestDrawnEdges(t *testing.T) {
	first, second := "first", "second"
	rows := []kb.EdgeRow{{Source: "a", Target: "b", Relation: kb.RelationDemoted, Context: &first},
		{Source: "a", Target: "b", Relation: kb.RelationDemoted, Context: &second}, {Source: "c", Target: "b", Relation: "unknown"}}
	got := drawnEdges(rows)
	if len(got) != 1 || got[0].source != "a" || got[0].target != "b" || got[0].stroke != strokeFor(kb.RelationDemoted) {
		t.Errorf("drawnEdges = %+v, want the a → b cut alone", got)
	}
}

func TestReduce(t *testing.T) {
	stroked := func(s *stroke) func(string, string) edge {
		return func(source, target string) edge { return edge{source: source, target: target, stroke: s} }
	}
	dep := stroked(strokeFor(kb.RelationSupports))
	founds, demoted := stroked(strokeFor(kb.RelationFounds)), stroked(strokeFor(kb.RelationDemoted))
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

// TestStyleTablesAreTotal: every node kind and band has its drawing, every
// relation one stroke, references alone being undrawn, and the legend lists the node kinds
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
		if drawn := strokeFor(r) != nil; drawn == (r == kb.RelationReferences) {
			t.Errorf("relation %q: drawn %t", r, drawn)
		}
	}
	if len(strokes) != len(kb.Relations)-1 {
		t.Errorf("%d strokes for the %d drawn relations, want one each", len(strokes), len(kb.Relations)-1)
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
	dep := strokeFor(kb.RelationSupports)
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
		edges: []edge{{source: "clm-zzz999", target: "clm-aaaaaa", stroke: strokeFor(kb.RelationSupports)}}}
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
		edges: []edge{{source: "clm-aaaaaa", target: "clm-bbbbbb", stroke: strokeFor(kb.RelationSupports)}}}
	sheets := g.sheets()
	if len(sheets) != 1 || sheets[0].path != kb.ClaimGraphFile || strings.Contains(sheets[0].dot(false), kb.ClaimGraphDigestFile) {
		t.Errorf("one volume: sheets %v, want the full sheet alone, unlinked", sheets)
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
