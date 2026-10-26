package from200to300

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format200"
	"kbase/internal/formats/format300"
)

const (
	entryPoint = "kb-root/entry-point.md"
	register   = "kb-root/claim-rigor.md"
	leafPath   = "kb-root/leaf.md"
	axiomsPath = "kb-root/axioms.md"
)

const entryPointText = "---\nkind: entry-point\nkb-format: \"2.0.0\"\n---\n# KB\n\n- [A Leaf](leaf.md)\n"

// entry is one register entry under heading, its Rigor section holding fields.
func entry(id, heading string, fields ...string) string {
	return "## " + heading + "\n<!-- id: " + id + " -->\n\n> **Leaf references:** *(none)*\n\n### Rigor\n" +
		strings.Join(fields, "\n") + "\n\n### Strength\n- strength: 0.5\n\n---\n\n"
}

// kbFiles is a 2.0.0 KB: the entry point, axioms.md declaring Axiom S3, and
// the register holding entries; extra files are added or replace these.
func kbFiles(entries string, extra common.Files) common.Files {
	files := common.Files{
		entryPoint: []byte(entryPointText),
		axiomsPath: []byte("### Axiom S3: A labelling convention\n\nLabels are written as authored.\n"),
		register:   []byte("# Claim Rigor\n\n---\n\n" + entries),
	}
	for p, b := range extra {
		files[p] = b
	}
	return files
}

// convert is files converted, the records it wrote decoded, and its warnings.
func convert(t *testing.T, files common.Files) (format300.Graph, common.Files, []common.Warning) {
	t.Helper()
	out, _, warnings, err := Convert200To300(files)
	if err != nil {
		t.Fatalf("Convert200To300: %v", err)
	}
	var g format300.Graph
	faults := append(format300.DecodeNodes(out[nodesPath], nodesPath, &g), format300.DecodeEdges(out[edgesPath], edgesPath, &g)...)
	if len(faults) > 0 {
		t.Fatalf("the records do not decode: %+v\n%s\n%s", faults, out[nodesPath], out[edgesPath])
	}
	return g, out, warnings
}

// lineOf is the 1-based line of the first line of text holding s.
func lineOf(t *testing.T, text, s string) int {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, s) {
			return i + 1
		}
	}
	t.Fatalf("no line holds %q", s)
	return 0
}

// wantWarnings fails t unless warnings are exactly one per want, each at its
// path and line with a detail holding its text.
func wantWarnings(t *testing.T, warnings []common.Warning, want ...common.Warning) {
	t.Helper()
	if len(warnings) != len(want) {
		t.Fatalf("warnings %+v, want %d", warnings, len(want))
	}
	for i, w := range want {
		got := warnings[i]
		if got.Path != w.Path || got.Line != w.Line || !strings.Contains(got.Detail, w.Detail) {
			t.Errorf("warning %d = %+v, want %s:%d naming %q", i, got, w.Path, w.Line, w.Detail)
		}
	}
}

func edgeLines(g format300.Graph) []string {
	var out []string
	for _, e := range g.References {
		out = append(out, fmt.Sprintf("%s %s %s ctx %q", format300.RelationReferences, e.Source, e.Target, e.Context))
	}
	for _, e := range g.Demoted {
		out = append(out, fmt.Sprintf("%s %s %s ctx %q replaced %s bearing %s manual %t", format300.RelationDemoted,
			e.Source, e.Target, e.Context, e.ReplacedRelation, bearingText(e.Bearing), e.ManuallyScored))
	}
	for _, e := range g.Founds {
		out = append(out, fmt.Sprintf("%s %s %s ctx %q bearing %s", format300.RelationFounds, e.Source, e.Target, e.Context, bearingText(e.Bearing)))
	}
	for _, e := range g.Scorable {
		out = append(out, fmt.Sprintf("%s %s %s ctx %q bearing %s manual %t", e.Relation, e.Source, e.Target, e.Context,
			bearingText(e.Bearing), e.ManuallyScored))
	}
	return out
}

func value(v float64) *float64 { return &v }

// TestEntriesBecomeNodeRecords: a claim and a work entry of either register
// become node records — title the heading, rigor, rationale, no-edge,
// strengthen-by, manually-scored false — and the register holding each is
// dropped silently.
func TestEntriesBecomeNodeRecords(t *testing.T) {
	files := kbFiles(entry("clm-aaaaaa", "Regime Conservation Laws",
		"- rigor: 0.85",
		"- no-edge: a definition nothing rests on",
		"- rationale: CES asymptotics.",
		"- strengthen-by:",
		"  - prove the bound for clm-bbbbbb")+
		entry("wrk-smith2020", "Smith, A. 2020", "- rigor: 0.9", "- rationale: Fixture Press."),
		common.Files{"kb-root/vol/claim-rigor.md": []byte("# Volume\n\n" + entry("clm-bbbbbb", "Foundation B", "- rationale: Moved."))})
	g, _, warnings := convert(t, files)
	wantWarnings(t, warnings)
	wantClaims := []format300.ClaimNode{
		{ID: "clm-aaaaaa", Title: "Regime Conservation Laws", Rigor: value(0.85), Rationale: "CES asymptotics.",
			NoEdge: "a definition nothing rests on", StrengthenBy: []string{"prove the bound for clm-bbbbbb"}},
		{ID: "clm-bbbbbb", Title: "Foundation B", Rationale: "Moved."},
	}
	if !reflect.DeepEqual(g.Claims, wantClaims) {
		t.Errorf("claims %+v, want %+v", g.Claims, wantClaims)
	}
	wantWorks := []format300.WorkNode{{ID: "wrk-smith2020", Title: "Smith, A. 2020", Rigor: value(0.9), Rationale: "Fixture Press."}}
	if !reflect.DeepEqual(g.Works, wantWorks) {
		t.Errorf("works %+v, want %+v", g.Works, wantWorks)
	}
}

const analysisLeaf = "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n    lifts:\n      - clm-aaaaaa: 0.5\n---\n\n# A Lemma\n"

// TestAnalysisTakesItsRegisterHeading: a hosted analysis's title is the heading
// its register entry is written under — its rigor, rationale and no-edge its
// entry's too — and a host heading that differs is no loss, since the register
// heading is the author's title. An analysis the register carries no entry for
// is titled with its host's first heading.
func TestAnalysisTakesItsRegisterHeading(t *testing.T) {
	for _, tc := range []struct{ name, heading string }{
		{"the same heading", "A Lemma"},
		{"a different heading", "An Old Name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := entry("clm-aaaaaa", "Claim A", "- rigor: 0.5") +
				entry("ana-aaaaaa", tc.heading, "- rigor: 0.8", "- no-edge: self-contained", "- rationale: Derives it.")
			files := kbFiles(entries, common.Files{leafPath: []byte(analysisLeaf)})
			g, _, warnings := convert(t, files)
			want := []format300.AnalysisNode{{ID: "ana-aaaaaa", Title: tc.heading, Rigor: value(0.8), Rationale: "Derives it.", NoEdge: "self-contained"}}
			if !reflect.DeepEqual(g.Analyses, want) {
				t.Errorf("analyses %+v, want %+v", g.Analyses, want)
			}
			wantWarnings(t, warnings)
		})
	}
	t.Run("no register entry", func(t *testing.T) {
		leaf := "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-bbbbbb\n---\n\n# A Lemma\n"
		g, _, warnings := convert(t, kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"), common.Files{leafPath: []byte(leaf)}))
		want := []format300.AnalysisNode{{ID: "ana-bbbbbb", Title: "A Lemma"}}
		if !reflect.DeepEqual(g.Analyses, want) {
			t.Errorf("analyses %+v, want %+v", g.Analyses, want)
		}
		wantWarnings(t, warnings)
	})
}

// TestUnhostedAnalysisIsDropped: an analysis entry no leaf's analysis-nodes
// hosts is no node, its pairs no edges, and it is warned of.
func TestUnhostedAnalysisIsDropped(t *testing.T) {
	entries := entry("clm-aaaaaa", "Claim A", "- rigor: 0.5") +
		entry("ana-bbbbbb", "A Stray Lemma", "- rigor: 0.8", "- lifts:", "  - clm-aaaaaa: 0.5")
	g, out, warnings := convert(t, kbFiles(entries, nil))
	if len(g.Analyses) != 0 || len(edgeLines(g)) != 0 {
		t.Errorf("analyses %+v and edges %q, want none", g.Analyses, edgeLines(g))
	}
	wantWarnings(t, warnings, common.Warning{Path: register, Line: lineOf(t, string(out[register]), "## A Stray Lemma"), Detail: "ana-bbbbbb"})
}

// TestExperimentNodes: an experiment-nodes entry is an experiment record —
// title its host's first heading, status, rigor pending where absent — and its
// pairs are edges.
func TestExperimentNodes(t *testing.T) {
	leaf := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 0.7\n    supports:\n      - clm-aaaaaa: 0.4\n" +
		"  - exp-id: exp-bbbbbb\n    status: pending\n---\n\n# Flow Simulation\n"
	g, _, warnings := convert(t, kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"), common.Files{leafPath: []byte(leaf)}))
	wantWarnings(t, warnings)
	want := []format300.ExperimentNode{
		{ID: "exp-aaaaaa", Title: "Flow Simulation", Status: "run", Rigor: value(0.7)},
		{ID: "exp-bbbbbb", Title: "Flow Simulation", Status: "pending"},
	}
	if !reflect.DeepEqual(g.Experiments, want) {
		t.Errorf("experiments %+v, want %+v", g.Experiments, want)
	}
	if got, want := edgeLines(g), []string{`supports exp-aaaaaa clm-aaaaaa ctx "" bearing 0.4 manual false`}; !slices.Equal(got, want) {
		t.Errorf("edges %q, want %q", got, want)
	}
}

// TestAHostWithNoHeadingTitlesTheExperimentByItsFile: an experiment whose host
// leaf has no heading is titled with the leaf's file name less its extension,
// warned of at its declaration, and gets no marker; an analysis in the same
// leaf keeps its register heading, gets no marker, and is warned of for that
// alone.
func TestAHostWithNoHeadingTitlesTheExperimentByItsFile(t *testing.T) {
	leaf := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n---\nNo heading here.\n"
	host := "kb-root/notes/flow-sim.md"
	g, out, warnings := convert(t, kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5")+entry("ana-aaaaaa", "A Lemma", "- rigor: 0.8"),
		common.Files{host: []byte(leaf)}))
	if len(g.Experiments) != 1 || g.Experiments[0].Title != "flow-sim" || len(g.Analyses) != 1 || g.Analyses[0].Title != "A Lemma" {
		t.Errorf("experiments %+v, analyses %+v; want the experiment titled flow-sim and the analysis A Lemma", g.Experiments, g.Analyses)
	}
	if strings.Contains(string(out[host]), "claim-rigor:") {
		t.Errorf("the headingless host was marked:\n%s", out[host])
	}
	wantWarnings(t, warnings,
		common.Warning{Path: host, Line: lineOf(t, leaf, "exp-id:"), Detail: `exp-aaaaaa's host has no heading, so it is titled "flow-sim"`},
		common.Warning{Path: host, Line: lineOf(t, leaf, "exp-id:"), Detail: "exp-aaaaaa gets no marker"},
		common.Warning{Path: host, Line: lineOf(t, leaf, "ana-id:"), Detail: "ana-aaaaaa gets no marker"})
}

// TestAxiomNodes: an axioms.md heading is an axiom record, and without
// axioms.md a legacy declaration is.
func TestAxiomNodes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files common.Files
		want  []format300.AxiomNode
	}{
		{"axioms.md", nil, []format300.AxiomNode{{ID: "axm-S3", Title: "A labelling convention"}}},
		{"a legacy invariants.md", common.Files{"kb-root/invariants.md": []byte("### INVARIANT-S2: Numbering\n\n- Axiom 1: **Substrate Topology** the lattice.\n")},
			[]format300.AxiomNode{{ID: "axm-1", Title: "Substrate Topology"}, {ID: "axm-S2", Title: "Numbering"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := kbFiles("", tc.files)
			if tc.files != nil {
				delete(files, axiomsPath)
			}
			g, _, _ := convert(t, files)
			if !reflect.DeepEqual(g.Axioms, tc.want) {
				t.Errorf("axioms %+v, want %+v", g.Axioms, tc.want)
			}
		})
	}
}

// TestEdgeRecords: rests-on, refutes and references bullets, an analysis's
// register and leaf pairs and an experiment's pairs are edge records oriented
// as the 2.0.x reader orients them, context "" where none; a demoted bullet on
// X naming Y is a demoted record Y → X replacing supports, bearing pending, its
// origin dropped silently.
func TestEdgeRecords(t *testing.T) {
	entries := entry("clm-aaaaaa", "Claim A", "- rigor: 0.5",
		"- rests-on:",
		"  - clm-bbbbbb — Claim B (strength 0.90) (bearing 0.9) [builds on it]",
		"  - wrk-smith2020 — Smith (bearing 0.8)",
		"  - Axiom S3 (bearing 1.0) [a convention]",
		"- refutes:",
		"  - clm-cccccc: 0.4",
		"- references:",
		"  - clm-cccccc — Claim C [see also]",
		"- demoted:",
		"  - clm-bbbbbb — Claim B (origin inferred)") +
		entry("clm-bbbbbb", "Claim B", "- rigor: 0.9") +
		entry("clm-cccccc", "Claim C", "- rigor: 0.6") +
		entry("wrk-smith2020", "Smith", "- rigor: 0.9") +
		entry("ana-aaaaaa", "A Lemma", "- rigor: 0.8", "- refutes:", "  - clm-bbbbbb: 0.2")
	g, _, warnings := convert(t, kbFiles(entries, common.Files{leafPath: []byte(analysisLeaf)}))
	wantWarnings(t, warnings)
	want := []string{
		`references clm-aaaaaa clm-cccccc ctx "see also"`,
		`demoted clm-bbbbbb clm-aaaaaa ctx "" replaced supports bearing *pending* manual false`,
		`founds axm-S3 clm-aaaaaa ctx "a convention" bearing 1.0`,
		`founds wrk-smith2020 clm-aaaaaa ctx "" bearing 0.8`,
		`supports clm-bbbbbb clm-aaaaaa ctx "builds on it" bearing 0.9 manual false`,
		`lifts ana-aaaaaa clm-aaaaaa ctx "" bearing 0.5 manual false`,
		`refutes ana-aaaaaa clm-bbbbbb ctx "" bearing 0.2 manual false`,
		`refutes clm-aaaaaa clm-cccccc ctx "" bearing 0.4 manual false`,
	}
	if got := edgeLines(g); !slices.Equal(got, want) {
		t.Errorf("edges\n %s\nwant\n %s", strings.Join(got, "\n "), strings.Join(want, "\n "))
	}
}

// TestKnownFilterDropWarns: a bullet whose token names no node is dropped, as
// the 2.0.x load drops it, with a warning quoting the bullet at its line.
func TestKnownFilterDropWarns(t *testing.T) {
	entries := entry("clm-aaaaaa", "Claim A", "- rigor: 0.5", "- rests-on:", "  - clm-zzzzzz — Nowhere (bearing 1.0)")
	g, out, warnings := convert(t, kbFiles(entries, nil))
	if len(edgeLines(g)) != 0 {
		t.Errorf("edges %q, want none", edgeLines(g))
	}
	wantWarnings(t, warnings, common.Warning{Path: register, Line: lineOf(t, string(out[register]), "clm-zzzzzz"),
		Detail: "clm-zzzzzz — Nowhere (bearing 1.0)"})
}

// TestFrontmatterIsRemoved: every document's frontmatter is removed, the
// entry point's kb-format with it; kind, claims, no-claim and subtree-* go
// silently, while experiments, path-stable, a key no 2.0.x reader knows and a
// value of the wrong type are each warned of by file, line and key.
func TestFrontmatterIsRemoved(t *testing.T) {
	leaf := "---\nkind: leaf\nclaims: [clm-aaaaaa]\nno-claim: \"a reason\"\nexperiments: [exp-aaaaaa]\npath-stable: true\ndepends-on: [clm-aaaaaa]\n---\n[↑ KB](entry-point.md)\n\n# A Leaf <!-- claim-rigor: clm-aaaaaa -->\n"
	index := "---\nkind: index\nsubtree-claims: [clm-aaaaaa]\nsubtree-experiments: []\nno-claim: true\n---\n# Volume\n"
	g, out, warnings := convert(t, kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"),
		common.Files{leafPath: []byte(leaf), "kb-root/vol/index.md": []byte(index)}))
	if len(g.Claims) != 1 {
		t.Errorf("claims %+v, want clm-aaaaaa alone", g.Claims)
	}
	for p, want := range map[string]string{
		leafPath:               "[↑ KB](entry-point.md)\n\n# A Leaf <!-- claim-rigor: clm-aaaaaa -->\n",
		"kb-root/vol/index.md": "# Volume\n",
		entryPoint:             "# KB\n\n- [A Leaf](leaf.md)\n",
	} {
		if string(out[p]) != want {
			t.Errorf("%s = %q, want %q", p, out[p], want)
		}
	}
	wantWarnings(t, warnings,
		common.Warning{Path: leafPath, Line: lineOf(t, leaf, "experiments:"), Detail: `"experiments"`},
		common.Warning{Path: leafPath, Line: lineOf(t, leaf, "path-stable:"), Detail: `"path-stable"`},
		common.Warning{Path: leafPath, Line: lineOf(t, leaf, "depends-on:"), Detail: `"depends-on"`},
		common.Warning{Path: "kb-root/vol/index.md", Line: lineOf(t, index, "no-claim:"), Detail: `"no-claim" holds a value of the wrong type`})
}

// TestADeclarationOutsideALeafIsWarnedOf: an experiment-nodes or
// analysis-nodes list on a document that is not kind: leaf declares no node to
// the 2.0.x load, so the conversion makes no record of it; the list goes with
// the frontmatter, warned of at its key's line.
func TestADeclarationOutsideALeafIsWarnedOf(t *testing.T) {
	index := "---\nkind: index\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n---\n# Volume\n"
	host := "kb-root/vol/index.md"
	g, out, warnings := convert(t, kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"), common.Files{host: []byte(index)}))
	if len(g.Experiments)+len(g.Analyses) != 0 {
		t.Errorf("experiments %+v and analyses %+v, want none: the 2.0.x load reads none from an index", g.Experiments, g.Analyses)
	}
	if string(out[host]) != "# Volume\n" {
		t.Errorf("%s = %q, want its frontmatter removed", host, out[host])
	}
	wantWarnings(t, warnings,
		common.Warning{Path: host, Line: lineOf(t, index, "experiment-nodes:"), Detail: `"experiment-nodes" on a document that is not kind: leaf`},
		common.Warning{Path: host, Line: lineOf(t, index, "analysis-nodes:"), Detail: `"analysis-nodes" on a document that is not kind: leaf`})
}

// TestIndexFilesAreObsolete: the 2.0.x .index/ files are obsolete but
// edges.yaml, which the record file replaces; a register is left as bytes.
func TestIndexFilesAreObsolete(t *testing.T) {
	files := kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"), nil)
	for _, name := range []string{"claims", "cites", "strengthen-by", "subtree-aggregates", "edges"} {
		files["kb-root/.index/"+name+".yaml"] = []byte("--- {}\n")
	}
	out, obsolete, _, err := Convert200To300(files)
	if err != nil {
		t.Fatalf("Convert200To300: %v", err)
	}
	want := []string{"kb-root/.index/cites.yaml", "kb-root/.index/claims.yaml", "kb-root/.index/strengthen-by.yaml", "kb-root/.index/subtree-aggregates.yaml"}
	if !slices.Equal(obsolete, want) {
		t.Errorf("obsolete %q, want %q", obsolete, want)
	}
	for _, p := range want {
		if _, held := out[p]; held {
			t.Errorf("the converted files still hold %s", p)
		}
	}
	var g format300.Graph
	if faults := format300.DecodeEdges(out[edgesPath], edgesPath, &g); len(faults) > 0 {
		t.Errorf("edges.yaml is not the record file: %+v", faults)
	}
	if string(out[register]) != string(files[register]) {
		t.Errorf("the register was rewritten:\n%s", out[register])
	}
}

// TestTheStampFileIsCarriedThrough: a KB whose interrupted save wrote the
// stamp file at the version the tree is still at carries that file into the
// converted form, nothing making it obsolete, so the save's last write
// replaces it.
func TestTheStampFileIsCarriedThrough(t *testing.T) {
	files := kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"), nil)
	stamp := []byte("kb-format: \"2.0.1\"\n")
	files["kb-root/.index/format.yaml"] = stamp
	out, obsolete, _, err := Convert200To300(files)
	if err != nil {
		t.Fatalf("Convert200To300: %v", err)
	}
	if !bytes.Equal(out["kb-root/.index/format.yaml"], stamp) {
		t.Errorf("the stamp file converted to %q, want it as it stood", out["kb-root/.index/format.yaml"])
	}
	if slices.Contains(obsolete, "kb-root/.index/format.yaml") {
		t.Errorf("obsolete %q names the stamp file", obsolete)
	}
}

// TestEdgeAuthoredTwice: equal bearings make one record, contexts joined in
// authored order, silently; different bearings keep the higher, a number above
// pending, with a warning naming both sites.
func TestEdgeAuthoredTwice(t *testing.T) {
	claimB := entry("clm-bbbbbb", "Claim B", "- rigor: 0.9")
	restsOn := func(bullets ...string) string {
		return entry("clm-aaaaaa", "Claim A", append([]string{"- rigor: 0.5", "- rests-on:"}, bullets...)...) + claimB
	}
	pairLeaf := func(bearing string) []byte {
		return []byte("---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n    lifts:\n      - clm-aaaaaa: " + bearing + "\n---\n\n# A Lemma\n")
	}
	expLeaf := []byte("---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    supports:\n      - clm-aaaaaa: 0.3\n---\n\n# Flow\n")
	for _, tc := range []struct {
		name    string
		entries string
		leaf    []byte
		edge    string
		warning string // the path the warning names, "" for none
		other   string // the other site's path, named in its detail
	}{
		{"a rests-on target repeated with equal bearings",
			restsOn("  - clm-bbbbbb — B (bearing 1.0) [first use]", "  - clm-bbbbbb — B (bearing 1.0) [second use]"), nil,
			`supports clm-bbbbbb clm-aaaaaa ctx "first use; second use" bearing 1.0 manual false`, "", ""},
		{"a rests-on target repeated with different bearings",
			restsOn("  - clm-bbbbbb — B (bearing 0.5) [low]", "  - clm-bbbbbb — B (bearing 1.0) [high]"), nil,
			`supports clm-bbbbbb clm-aaaaaa ctx "high" bearing 1.0 manual false`, register, register},
		{"a number above pending",
			restsOn("  - clm-bbbbbb — B (bearing 0.2)", "  - clm-bbbbbb — B (bearing *pending*)"), nil,
			`supports clm-bbbbbb clm-aaaaaa ctx "" bearing 0.2 manual false`, register, register},
		{"an analysis pair in the register and the leaf, equal",
			entry("clm-aaaaaa", "Claim A", "- rigor: 0.5") + entry("ana-aaaaaa", "A Lemma", "- lifts:", "  - clm-aaaaaa: 0.5"), pairLeaf("0.5"),
			`lifts ana-aaaaaa clm-aaaaaa ctx "" bearing 0.5 manual false`, "", ""},
		{"an analysis pair in the register and the leaf, different",
			entry("clm-aaaaaa", "Claim A", "- rigor: 0.5") + entry("ana-aaaaaa", "A Lemma", "- lifts:", "  - clm-aaaaaa: 0.7"), pairLeaf("0.5"),
			`lifts ana-aaaaaa clm-aaaaaa ctx "" bearing 0.7 manual false`, leafPath, register},
		{"an experiment's supports pair and a rests-on bullet naming it",
			entry("clm-aaaaaa", "Claim A", "- rigor: 0.5", "- rests-on:", "  - exp-aaaaaa — Flow (bearing 0.6)"), expLeaf,
			`supports exp-aaaaaa clm-aaaaaa ctx "" bearing 0.6 manual false`, register, leafPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extra := common.Files{}
			if tc.leaf != nil {
				extra[leafPath] = tc.leaf
			}
			g, _, warnings := convert(t, kbFiles(tc.entries, extra))
			var got []string
			for _, e := range edgeLines(g) {
				if strings.Fields(e)[2] == "clm-aaaaaa" {
					got = append(got, e)
				}
			}
			if !slices.Equal(got, []string{tc.edge}) {
				t.Errorf("edges into clm-aaaaaa %q, want %q", got, tc.edge)
			}
			if tc.warning == "" {
				wantWarnings(t, warnings)
				return
			}
			if len(warnings) != 1 || warnings[0].Path != tc.warning || warnings[0].Line == 0 ||
				!strings.Contains(warnings[0].Detail, tc.warning+":") || !strings.Contains(warnings[0].Detail, tc.other+":") {
				t.Errorf("warnings %+v, want one at %s naming both sites", warnings, tc.warning)
			}
		})
	}
}

// TestRegisterText: text outside every entry is dropped silently; text inside
// an entry that is no field, footer, derived line or continuation is dropped
// with a warning quoting it.
func TestRegisterText(t *testing.T) {
	outside := "# Part 1 Claim Quality Register\n\nAn introduction.\n\n| Band | Meaning |\n|---|---|\n| 0.9 | sound |\n\n---\n\n## Legend\n\nNo entry here.\n\n"
	inside := "## Claim A\n<!-- id: clm-aaaaaa -->\n\n> **Leaf references:** *(none)*\n\n### Rigor\n  - rigor: 0.60\n- quality: 0.4\n" +
		"- rationale: A first line\ncontinued here.\n\nA stray note.\n\n### Strength\n- strength: 0.6\n\n---\n"
	files := kbFiles("", nil)
	files[register] = []byte(outside + inside)
	g, _, warnings := convert(t, files)
	if len(g.Claims) != 1 || g.Claims[0].Rationale != "A first line continued here." {
		t.Errorf("claims %+v, want clm-aaaaaa with its folded rationale", g.Claims)
	}
	text := outside + inside
	wantWarnings(t, warnings,
		common.Warning{Path: register, Line: lineOf(t, text, "- quality: 0.4"), Detail: `"- quality: 0.4"`},
		common.Warning{Path: register, Line: lineOf(t, text, "A stray note."), Detail: `"A stray note."`})
}

// TestAnIDKeyedByTwoRegisters: an id two register entries key is one record,
// from the first keying register in the inventory's order; the other entry is
// dropped with a warning at its heading naming the id and the kept entry's
// register, and its edges stand.
func TestAnIDKeyedByTwoRegisters(t *testing.T) {
	vol := "# Volume\n\n" + entry("clm-aaaaaa", "Claim A Again", "- rigor: 0.2", "- rests-on:", "  - clm-bbbbbb — B (bearing 0.7)")
	files := kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5")+entry("clm-bbbbbb", "Claim B", "- rigor: 0.9"),
		common.Files{"kb-root/vol/claim-rigor.md": []byte(vol)})
	g, _, warnings := convert(t, files)
	want := []format300.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rigor: value(0.5)}, {ID: "clm-bbbbbb", Title: "Claim B", Rigor: value(0.9)}}
	if !reflect.DeepEqual(g.Claims, want) {
		t.Errorf("claims %+v, want %+v", g.Claims, want)
	}
	if got, want := edgeLines(g), []string{`supports clm-bbbbbb clm-aaaaaa ctx "" bearing 0.7 manual false`}; !slices.Equal(got, want) {
		t.Errorf("edges %q, want %q", got, want)
	}
	wantWarnings(t, warnings, common.Warning{Path: "kb-root/vol/claim-rigor.md", Line: lineOf(t, vol, "## Claim A Again"),
		Detail: "a second register entry for clm-aaaaaa is dropped; the entry in " + register + " is kept"})
}

// TestAnIDDeclaredByTwoHostLeaves: an experiment two leaves declare is one
// record, from the first declaring leaf in path order; the other declaration
// is dropped with a warning at its line naming the id and the kept leaf, and
// its pairs stand.
func TestAnIDDeclaredByTwoHostLeaves(t *testing.T) {
	first := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n---\n\n# Flow\n"
	second := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: pending\n    supports:\n      - clm-aaaaaa: 0.4\n---\n\n# Flow Again\n"
	g, _, warnings := convert(t, kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"),
		common.Files{"kb-root/a.md": []byte(first), "kb-root/b.md": []byte(second)}))
	want := []format300.ExperimentNode{{ID: "exp-aaaaaa", Title: "Flow", Status: "run"}}
	if !reflect.DeepEqual(g.Experiments, want) {
		t.Errorf("experiments %+v, want %+v", g.Experiments, want)
	}
	if got, want := edgeLines(g), []string{`supports exp-aaaaaa clm-aaaaaa ctx "" bearing 0.4 manual false`}; !slices.Equal(got, want) {
		t.Errorf("edges %q, want %q", got, want)
	}
	wantWarnings(t, warnings, common.Warning{Path: "kb-root/b.md", Line: lineOf(t, second, "exp-id:"),
		Detail: "a second declaration of exp-aaaaaa is dropped; the declaration in kb-root/a.md is kept"})
}

// TestRefusedByThe200Load: a KB the 2.0.x load refuses is refused with that
// load's own error.
func TestRefusedByThe200Load(t *testing.T) {
	files := kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5", "- rests-on:", "  - clm-bbbbbb — B (bearing sometimes)")+
		entry("clm-bbbbbb", "Claim B", "- rigor: 0.9"), nil)
	_, _, _, err := Convert200To300(files)
	var ann *format200.AnnotationError
	if !errors.As(err, &ann) || len(ann.Faults) != 1 || ann.Faults[0].Path != format200.RegisterFile {
		t.Errorf("err = %v, want the 2.0.x load's annotation fault on the register", err)
	}
}
