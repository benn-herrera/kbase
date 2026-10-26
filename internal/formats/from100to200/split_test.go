package from100to200_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/formats"
	"kbase/internal/formats/format100"
	"kbase/internal/formats/format200"
	"kbase/internal/formats/format300"
)

const splitRegister = "kb-root/claim-rigor.md"

// splitKB is a 1.0.0 KB whose claims stand in every leaf shape a 1.0.0 marker
// takes — a plain paragraph, a blockquote, a heading, a display-maths line —
// one of them stating no premise of its own, and one listed with no marker at
// all.
func splitKB() formats.Files {
	return formats.Files{
		"kb-root/entry-point.md": []byte("---\nkind: entry-point\nsubtree-claims: [clm-blk001, clm-quo001, clm-hea001, clm-eqn001, clm-qui001]\nkb-format: \"1.0.0\"\n---\n\n# KB\n\n- [A Leaf](leaf.md)\n- [An Unmarked Leaf](quiet.md)\n"),
		"kb-root/leaf.md": []byte("---\nkind: leaf\nclaims: [clm-blk001, clm-quo001, clm-hea001, clm-eqn001]\n---\n" +
			"[↑ KB](entry-point.md)\n\n\n# A Leaf\n\nA plain paragraph states the first result. <!-- claim-quality: clm-blk001 -->\n\n" +
			"> **Theorem 2.** A quoted statement. <!-- claim-quality: clm-quo001 -->\n\n" +
			"## A Section Claim <!-- claim-quality: clm-hea001 -->\n\n" +
			"``` math\nx = 1 \\label{eq:one} <!-- claim-quality: clm-eqn001 -->\n```\n"),
		"kb-root/quiet.md": []byte("---\nkind: leaf\nclaims: [clm-qui001]\n---\n[↑ KB](entry-point.md)\n\n\n# An Unmarked Leaf\n\nProse carrying no marker.\n"),
		"kb-root/claim-quality.md": []byte("# Claim Quality Register\n\n---\n\n" +
			"## A Plain Result\n<!-- id: clm-blk001 -->\n\n### Quality\n- confidence: 0.80\n- depends-on:\n" +
			"  - clm-quo001 — A Quoted Result (solidity 0.70) [the quoted argument]\n" +
			"- rationale: The argument of the plain claim.\n\n### Solidity\n- solidity: 0.70\n\n---\n\n" +
			"## A Quoted Result\n<!-- id: clm-quo001 -->\n\n### Quality\n- confidence: 0.70\n- depends-on:\n" +
			"  - clm-hea001 — A Section Result (solidity 0.60)\n" +
			"- rationale: The argument of the quoted claim.\n\n### Solidity\n- solidity: 0.60\n\n---\n\n" +
			"## A Section Result\n<!-- id: clm-hea001 -->\n\n### Quality\n- confidence: 0.60\n" +
			"- rationale: The argument of the section claim.\n\n### Solidity\n- solidity: 0.60\n\n---\n\n" +
			"## An Equation Result\n<!-- id: clm-eqn001 -->\n\n### Quality\n- confidence: 0.90\n- depends-on:\n" +
			"  - clm-aaaaaa — A Claim No Entry Carries (solidity 0.50)\n" +
			"- rationale: The argument of the equation claim.\n\n### Solidity\n- solidity: 0.50\n\n---\n\n" +
			"## An Unmarked Result\n<!-- id: clm-qui001 -->\n\n### Quality\n- confidence: 0.40\n" +
			"- rationale: The argument of the unmarked claim.\n\n### Solidity\n- solidity: 0.40\n\n"),
	}
}

// drop is a load's log sink: a token no entry carries is not what these tests
// are about.
func drop(string, int, string) {}

func convertSplitKB(t *testing.T) formats.Files {
	t.Helper()
	return chain200(t, splitKB())
}

// chain200 is files converted through the 1.0.0 → 2.0.0 step.
func chain200(t *testing.T, files formats.Files) formats.Files {
	t.Helper()
	got, _, _, err := formats.Chain(format100.Version100, format200.Version200, files)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	return got
}

// chain300 is files converted through the whole chain to 3.0.0.
func chain300(t *testing.T, files formats.Files) formats.Files {
	t.Helper()
	got, _, _, err := formats.Chain(format100.Version100, format300.Version300, files)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	return got
}

// TestSplitMovesEachClaimsArgumentToItsAnalysis: every claim keeps its title
// and its rigor and gives up its premise list and its rationale, and the
// analysis beside it carries both under the claim's own title.
func TestSplitMovesEachClaimsArgumentToItsAnalysis(t *testing.T) {
	got := convertSplitKB(t)
	claims := format200.ParseClaimEntries(string(got[splitRegister]), format200.RegisterFile, nil, drop)
	order, analyses, err := format200.ParseAnalysisEntries(string(got[splitRegister]), format200.RegisterFile, nil, drop)
	if err != nil {
		t.Fatalf("ParseAnalysisEntries: %v", err)
	}
	if len(order) != len(claims) {
		t.Fatalf("the register holds %d analyses for %d claims, want one each", len(order), len(claims))
	}
	for _, c := range claims {
		body := strings.TrimPrefix(c.ID, "clm-")
		a, ok := analyses["ana-"+body]
		if !ok {
			t.Fatalf("no analysis ana-%s for %s; the register holds %q", body, c.ID, order)
		}
		if a.Title != c.Title {
			t.Errorf("%s's analysis is titled %q, want the claim's %q", c.ID, a.Title, c.Title)
		}
		if a.Rigor == nil || c.Rigor == nil || *a.Rigor != *c.Rigor {
			t.Errorf("%s's analysis rigor %v, want the claim's %v", c.ID, a.Rigor, c.Rigor)
		}
		if c.Rationale != "" {
			t.Errorf("%s keeps the rationale %q, want the analysis's", c.ID, c.Rationale)
		}
		if len(c.Premises) != 0 {
			t.Errorf("%s rests on %d premises, want them moved to its analysis", c.ID, len(c.Premises))
		}
		if a.Rationale == "" {
			t.Errorf("%s's analysis states no rationale, want the claim's", c.ID)
		}
		if !strings.Contains(string(got[splitRegister]), "<!-- id: ana-"+body+" -->") {
			t.Errorf("%s's analysis carries no register entry", c.ID)
		}
	}
	// The premises move whole: the source, its recorded strength and its
	// context come across unchanged, and the analysis is the node they feed.
	if r := analyses["ana-blk001"].Rigor; r == nil || *r != 0.80 {
		t.Errorf("ana-blk001's rigor %v, want the claim's authored 0.80", r)
	}
	moved := analyses["ana-blk001"].Premises
	if len(moved) != 1 || moved[0].Source != "clm-quo001" || moved[0].Target != "ana-blk001" ||
		moved[0].Context == nil || *moved[0].Context != "the quoted argument" {
		t.Errorf("ana-blk001's premises %+v, want clm-quo001's bullet in full", moved)
	}
	if p := analyses["ana-hea001"].Premises; len(p) != 0 {
		t.Errorf("ana-hea001 rests on %+v, want the premise-less argument as it stands", p)
	}
	if s := string(got[splitRegister]); strings.Contains(sectionFor(s, "## A Section Result"), "- rests-on:") {
		t.Errorf("the premise-less analysis carries a premise list:\n%s", sectionFor(s, "## A Section Result"))
	}
	// An argument with no premise stands as it is: the conversion mints no
	// axiom to found it.
	if _, ok := got[format200.AxiomsPath]; ok {
		t.Errorf("the conversion wrote %s for a premise-less analysis", format200.AxiomsPath)
	}
}

// sectionFor is the register text from the entry headed heading to the next
// entry, so one entry's fields are read apart from the analysis beside it.
func sectionFor(register, heading string) string {
	_, rest, ok := strings.Cut(register, heading)
	if !ok {
		return ""
	}
	rest = strings.TrimPrefix(rest, "\n")
	for _, end := range []string{"\n## ", "\n---"} {
		if i := strings.Index(rest, end); i >= 0 {
			rest = rest[:i]
		}
	}
	return rest
}

// TestSplitLiftsEachClaimFromItsAnalysis: each claim's own edge to its analysis
// is the analysis's conclusion at full bearing, declared on the leaf that hosts
// it, and the 2.0.x load reads one lift per claim off that declaration.
func TestSplitLiftsEachClaimFromItsAnalysis(t *testing.T) {
	st, err := discover(t, convertSplitKB(t))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	lifted := map[string]string{}
	for _, a := range st.Analyses {
		if len(a.Lifts) != 1 {
			t.Errorf("%s lifts %d targets, want the one claim it argues for", a.ID, len(a.Lifts))
			continue
		}
		e := a.Lifts[0]
		if e.TargetKind != format200.NodeKindClaim || e.Bearing.Pending || e.Bearing.Value != 1 {
			t.Errorf("%s lifts %s as %+v, want a claim at bearing 1", a.ID, e.Target, e)
		}
		lifted[e.Target] = a.ID
	}
	for _, id := range []string{"clm-blk001", "clm-quo001", "clm-hea001", "clm-eqn001", "clm-qui001"} {
		if want := "ana-" + strings.TrimPrefix(id, "clm-"); lifted[id] != want {
			t.Errorf("%s is lifted by %q, want %s", id, lifted[id], want)
		}
	}
}

// TestSplitPlacesTheAnalysisMarkerBesideTheClaim: the analysis's statement
// marker stands on the line carrying its claim's marker, after it — the claim's
// block reading where it read before, the analysis's section opening over it —
// for every leaf shape a 1.0.0 marker takes; a claim its leaf lists and does
// not mark takes the leaf's first heading.
func TestSplitPlacesTheAnalysisMarkerBesideTheClaim(t *testing.T) {
	got := convertSplitKB(t)
	for _, tc := range []struct{ path, line string }{
		{format200.KBDir + "/leaf.md",
			"A plain paragraph states the first result. <!-- claim-rigor: clm-blk001 --> <!-- claim-rigor: ana-blk001 -->"},
		{format200.KBDir + "/leaf.md",
			"> **Theorem 2.** A quoted statement. <!-- claim-rigor: clm-quo001 --> <!-- claim-rigor: ana-quo001 -->"},
		{format200.KBDir + "/leaf.md",
			"## A Section Claim <!-- claim-rigor: clm-hea001 --> <!-- claim-rigor: ana-hea001 -->"},
		{format200.KBDir + "/leaf.md",
			`x = 1 \label{eq:one} <!-- claim-rigor: clm-eqn001 --> <!-- claim-rigor: ana-eqn001 -->`},
		{format200.KBDir + "/quiet.md",
			"# An Unmarked Leaf\n<!-- claim-rigor: ana-qui001 -->"},
	} {
		if text := string(got[tc.path]); !strings.Contains(text, tc.line+"\n") {
			t.Errorf("%s does not carry\n%s\nbut\n%s", tc.path, tc.line, text)
		}
	}
}

// shapeRegister is one claim's 1.0.0 register entry, as the shapes below spell
// it: the fields are the entry's Quality section.
func shapeRegister(title, id string, fields ...string) []byte {
	var b strings.Builder
	b.WriteString("# Claim Quality Register\n\n---\n\n## " + title + "\n<!-- id: " + id + " -->\n\n### Quality\n")
	for _, f := range fields {
		b.WriteString(f + "\n")
	}
	return []byte(b.String())
}

// markerOf is a statement marker as the conversion writes it.
func markerOf(id string) string { return "<!-- claim-rigor: " + id + " -->" }

// hostReports is the documents declaring id in an analysis-nodes list, and the
// documents carrying its statement marker.
func hostReports(t *testing.T, files formats.Files, id string) (declared, marked []string) {
	t.Helper()
	for _, p := range slices.Sorted(maps.Keys(files)) {
		text := string(files[p])
		if strings.Contains(text, "- "+format200.AnaIDKey+": "+id) {
			declared = append(declared, p)
		}
		if strings.Contains(text, markerOf(id)) {
			marked = append(marked, p)
		}
	}
	return declared, marked
}

// TestSplitFollowsTheClaimsMarker: an analysis is hosted where its claim's
// statement is, so a listing outranks a marker — a listing leaf is where the
// 3.0.0 conversion reads the claim's statement, and the analysis must land
// there too. A claim listed in one leaf and marked in another is analysed in
// the listing leaf; one marked in a leaf that lists it nowhere is analysed in
// the marking leaf, no listing leaf existing.
func TestSplitFollowsTheClaimsMarker(t *testing.T) {
	for _, tc := range []struct {
		name string
		docs map[string]string
		host string
	}{
		{
			name: "listed in one leaf, marked in another",
			docs: map[string]string{
				"kb-root/listed.md": "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n\n# A Listed Leaf\n\nProse.\n",
				"kb-root/marked.md": "---\nkind: leaf\n---\n\n# A Marking Leaf\n\nResult. <!-- claim-quality: clm-aaaaaa -->\n",
			},
			host: "kb-root/listed.md",
		},
		{
			name: "marked where no leaf lists it",
			docs: map[string]string{
				"kb-root/quiet.md":  "---\nkind: leaf\n---\n\n# A Quiet Leaf\n\nNothing listed.\n",
				"kb-root/marked.md": "---\nkind: leaf\n---\n\n# A Marking Leaf\n\nResult. <!-- claim-quality: clm-aaaaaa -->\n",
			},
			host: "kb-root/marked.md",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := formats.Files{"kb-root/claim-quality.md": shapeRegister("Claim A", "clm-aaaaaa", "- confidence: 0.8", "- rationale: Its argument.")}
			for p, text := range tc.docs {
				files[p] = []byte(text)
			}
			got := chain200(t, files)
			declared, marked := hostReports(t, got, "ana-aaaaaa")
			if !slices.Equal(declared, []string{tc.host}) || !slices.Equal(marked, []string{tc.host}) {
				t.Errorf("ana-aaaaaa declared in %q, marked in %q; want both %s", declared, marked, tc.host)
			}
			if _, err := discover(t, got); err != nil {
				t.Errorf("the converted KB does not load: %v", err)
			}
			// The 3.0.0 conversion reads the claim's statement from the leaf
			// that lists it, so the analysis and the claim land together.
			text := string(chain300(t, files)[tc.host])
			for _, id := range []string{"clm-aaaaaa", "ana-aaaaaa"} {
				if !strings.Contains(text, markerOf(id)) {
					t.Errorf("%s is not marked in %s in the 3.0.0 KB:\n%s", id, tc.host, text)
				}
			}
		})
	}
}

// TestSplitOpensOneAnalysisList: a 1.0.0 leaf spelling its analysis list as the
// empty flow list keeps one analysis-nodes key, with the entries under it, and
// the KB converts on to 3.0.0 — a second key of that name is what the next step
// refuses.
func TestSplitOpensOneAnalysisList(t *testing.T) {
	files := formats.Files{
		"kb-root/claim-quality.md": shapeRegister("Claim A", "clm-aaaaaa", "- confidence: 0.8", "- rationale: Its argument."),
		"kb-root/leaf.md": []byte("---\nkind: leaf\nclaims: [clm-aaaaaa]\nsupport-nodes: []\n---\n\n# A Leaf\n\n" +
			"Result. <!-- claim-quality: clm-aaaaaa -->\n"),
	}
	text := string(chain200(t, files)["kb-root/leaf.md"])
	if n := strings.Count(text, "analysis-nodes:"); n != 1 {
		t.Errorf("the leaf spells analysis-nodes %d times, want once:\n%s", n, text)
	}
	if strings.Contains(text, "analysis-nodes: []") {
		t.Errorf("the empty flow list stands beside the entries:\n%s", text)
	}
	if !strings.Contains(text, "- "+format200.AnaIDKey+": ana-aaaaaa") {
		t.Errorf("the leaf declares no analysis:\n%s", text)
	}
	if _, _, _, err := formats.Chain(format100.Version100, format300.Version300, files); err != nil {
		t.Errorf("the KB the split wrote does not convert to 3.0.0: %v", err)
	}
}

// TestSplitInALeafWithNoHeading: a marked claim's analysis stands beside its
// marker, which needs no heading; one the leaf lists and does not mark has
// nowhere to put the analysis's marker — it is declared and left unmarked, the
// state the claim itself is in.
func TestSplitInALeafWithNoHeading(t *testing.T) {
	marked := formats.Files{
		"kb-root/claim-quality.md": shapeRegister("Claim A", "clm-aaaaaa", "- confidence: 0.8"),
		"kb-root/leaf.md":          []byte("---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\nResult. <!-- claim-quality: clm-aaaaaa -->\n"),
	}
	text := string(chain200(t, marked)["kb-root/leaf.md"])
	if !strings.Contains(text, "<!-- claim-rigor: clm-aaaaaa --> "+markerOf("ana-aaaaaa")+"\n") {
		t.Errorf("the analysis's marker is not beside its claim's:\n%s", text)
	}

	unmarked := formats.Files{
		"kb-root/claim-quality.md": shapeRegister("Claim A", "clm-aaaaaa", "- confidence: 0.8"),
		"kb-root/leaf.md":          []byte("---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\nResult, unmarked.\n"),
	}
	got := chain200(t, unmarked)
	declared, markedAt := hostReports(t, got, "ana-aaaaaa")
	if !slices.Equal(declared, []string{"kb-root/leaf.md"}) || len(markedAt) != 0 {
		t.Errorf("ana-aaaaaa declared in %q, marked in %q; want it declared in the leaf and marked nowhere", declared, markedAt)
	}
}

// TestARegisterOnlyClaimKeepsItsArgument: a claim no leaf carries is left whole
// — its premise list and its rationale stay on it and no analysis is minted —
// since the analysis would hold them on behalf of no host, and the 2.0.x load
// would read it as no node.
func TestARegisterOnlyClaimKeepsItsArgument(t *testing.T) {
	got := chain200(t, formats.Files{
		"kb-root/claim-quality.md": shapeRegister("Claim A", "clm-aaaaaa",
			"- confidence: 0.8", "- depends-on:", "  - work-smith2020 — Smith, A. 2020 (applicability 0.5)", "- rationale: Its argument."),
	})
	text := string(got[splitRegister])
	if strings.Contains(text, "ana-") {
		t.Errorf("the register-only claim was split:\n%s", text)
	}
	if !strings.Contains(text, "- rests-on:") || !strings.Contains(text, "wrk-smith2020") || !strings.Contains(text, "- rationale: Its argument.") {
		t.Errorf("the claim did not keep its argument:\n%s", text)
	}
	st, err := discover(t, got)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(st.ClaimEntries) != 1 || len(st.ClaimEntries[0].Premises) != 1 {
		t.Errorf("the claim's entry %+v, want its premise kept", st.ClaimEntries)
	}
}

// TestADuplicatedEntryIsSplitOnce: a claim keyed by two entries in one register
// is one node, so the first entry is split and the second stands as it is — a
// second analysis entry under one id is a repeat the load reads as one.
func TestADuplicatedEntryIsSplitOnce(t *testing.T) {
	register := string(shapeRegister("Claim A", "clm-aaaaaa", "- confidence: 0.8", "- rationale: First.")) +
		"\n---\n\n## Claim A Again\n<!-- id: clm-aaaaaa -->\n\n### Quality\n- confidence: 0.8\n" +
		"- depends-on:\n  - clm-bbbbbb — Claim B (solidity 0.7)\n- rationale: Second.\n"
	got := chain200(t, formats.Files{
		"kb-root/claim-quality.md": []byte(register),
		"kb-root/leaf.md":          []byte("---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n\n# A Leaf\n\nResult. <!-- claim-quality: clm-aaaaaa -->\n"),
	})
	text := string(got[splitRegister])
	if n := strings.Count(text, "<!-- id: ana-aaaaaa -->"); n != 1 {
		t.Errorf("the register holds %d entries for ana-aaaaaa, want one:\n%s", n, text)
	}
	if second := sectionFor(text, "## Claim A Again"); !strings.Contains(second, "- rationale: Second.") || !strings.Contains(second, "- rests-on:") {
		t.Errorf("the second entry was split as well:\n%s", second)
	}
}
