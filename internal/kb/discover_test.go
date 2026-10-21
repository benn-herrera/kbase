package kb

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/log/logtest"
)

// TestBearing: a bearing is a number in [0, 1] or the pending literal, and
// each value reads and writes back as itself; zero is the judgement 0.0, never
// the absence of one.
func TestBearing(t *testing.T) {
	for _, tc := range []struct {
		in, text string
		ok       bool
	}{
		{"0", "0.0", true}, {"1", "1.0", true}, {"0.4", "0.4", true}, {PendingLiteral, PendingLiteral, true},
		{"1.5", "", false}, {"-0.1", "", false}, {"sometimes", "", false}, {"", "", false},
	} {
		b, ok := ParseBearing("(bearing " + tc.in + ")")
		if ok != tc.ok {
			t.Errorf("ParseBearing(%q) ok = %t, want %t", tc.in, ok, tc.ok)
			continue
		}
		if !ok {
			continue
		}
		if got := BearingText(b); got != tc.text {
			t.Errorf("BearingText(%q) = %q, want %q", tc.in, got, tc.text)
		}
		again, ok := ParseBearing("(bearing " + BearingText(b) + ")")
		if !ok || again != b {
			t.Errorf("(bearing %q) read back as %+v, %t; want %+v", BearingText(b), again, ok, b)
		}
	}
}

// registerWith is a register holding three claims; the first carries the
// body given under its authored section.
func registerWith(body string) string {
	return "## A Claim\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n- rigor: 0.9\n" + body +
		"\n\n### Strength\n- strength: *pending*\n\n---\n\n## B Claim\n<!-- id: clm-bbbbbb -->\n\n---\n\n## C Claim\n<!-- id: clm-cccccc -->\n"
}

// discoverFiles runs Discover over a KB whose kb-root holds files.
func discoverFiles(t *testing.T, files map[string]string, lg log.Logger) (State, error) {
	t.Helper()
	repo := t.TempDir()
	root := filepath.Join(repo, KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	held := map[string][]byte{}
	for name, text := range files {
		held["kb-root/"+name] = []byte(text)
	}
	return Discover(MigratedSource(root, FormatVersion, held, nil), lg)
}

// discoverText runs Discover over a KB whose register is register, declaring
// the axiom its bullets cite.
func discoverText(t *testing.T, register string) (State, error) {
	t.Helper()
	return discoverFiles(t, map[string]string{
		RegisterFile: register,
		AxiomsFile:   "# Axioms\n\n### Axiom S2: Numbering\n",
	}, log.Discard())
}

// TestDiscoverRefusesASymlinkedDocument: a document that is a symbolic link is
// refused, naming the path, rather than read through it; a tree of regular
// documents still loads.
func TestDiscoverRefusesASymlinkedDocument(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	register := registerWith("")
	outside := filepath.Join(t.TempDir(), RegisterFile)
	if err := os.WriteFile(outside, []byte(register), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, RegisterFile)
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	_, err := Discover(OnDisk(root), log.Discard())
	var sym *SymlinkError
	if !errors.As(err, &sym) || sym.Path != link {
		t.Fatalf("Discover over a symlinked register = %v, want a SymlinkError naming %s", err, link)
	}

	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte(register), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(OnDisk(root), log.Discard()); err != nil {
		t.Fatalf("Discover over a register that stands where it is: %v", err)
	}
}

// lineOf is the 1-based physical line of text's first line naming needle.
func lineOf(t *testing.T, text, needle string) int {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("%q names no line of the register", needle)
	return 0
}

// TestDiscoverAnnotationFaults: each malformed rests-on or references bullet is
// refused by Discover as one fault naming its field, file and physical line.
func TestDiscoverAnnotationFaults(t *testing.T) {
	for _, tc := range []struct{ name, body, field, needle, allowed string }{
		{"R1 two scoring targets", "- rests-on:\n  - clm-aaaaaa, clm-bbbbbb — Two (bearing 0.5)",
			ListRestsOn, "clm-aaaaaa, clm-bbbbbb", ""},
		{"R1 a claim and an axiom in one bullet", "- rests-on:\n  - clm-bbbbbb — B (bearing 0.4), Axiom S2 — N (bearing 0.5)",
			ListRestsOn, "Axiom S2", ""},
		{"R2 scoring target with no bearing", "- rests-on:\n  - clm-bbbbbb — B (strength 0.9)",
			ListRestsOn, "clm-bbbbbb — B (strength 0.9)", ""},
		{"R2 an axiom bullet with no bearing", "- rests-on:\n  - Axiom S2 — Numbering",
			ListRestsOn, "Axiom S2 — Numbering", ""},
		{"R3 bearing out of domain", "- rests-on:\n  - clm-bbbbbb — B (bearing 1.5)",
			ListRestsOn, "(bearing 1.5)", "[0, 1] or " + PendingLiteral},
		{"R4 bearing on a bullet naming no scoring target", "- rests-on:\n  - INVARIANT-S2 (bearing 0.5)",
			ListRestsOn, "INVARIANT-S2 (bearing 0.5)", ""},
		{"R4 bearing on a references bullet", "- references:\n  - clm-bbbbbb — B (bearing 0.5)",
			"references", "clm-bbbbbb — B (bearing 0.5)", ""},
		{"R5 two bearings", "- rests-on:\n  - clm-bbbbbb — B (bearing 0.5) (bearing 0.6)",
			ListRestsOn, "(bearing 0.5) (bearing 0.6)", ""},
		{"a refutes pair with no bearing", "- refutes:\n  - clm-bbbbbb: sometimes",
			RefutesKey, "clm-bbbbbb: sometimes", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := registerWith(tc.body)
			_, err := discoverText(t, reg)
			var faulted *AnnotationError
			if !errors.As(err, &faulted) {
				t.Fatalf("err = %v, want an AnnotationError", err)
			}
			if len(faulted.Faults) != 1 {
				t.Fatalf("faults = %+v, want one", faulted.Faults)
			}
			f := faulted.Faults[0]
			if f.Field != tc.field || f.Path != RegisterFile || f.Line != lineOf(t, reg, tc.needle) {
				t.Errorf("fault = %+v, want field %q at %s:%d", f, tc.field, RegisterFile, lineOf(t, reg, tc.needle))
			}
			if f.Detail == "" || f.Remedy == "" {
				t.Errorf("fault carries no detail or remedy: %+v", f)
			}
			if f.Allowed != tc.allowed {
				t.Errorf("allowed = %q, want %q", f.Allowed, tc.allowed)
			}
		})
	}
}

// TestDiscoverLegalBullets: an axiom-only bullet with a bearing, a two-id
// references bullet and a bullet naming one stale id all load; the stale id is
// dropped and requires no bearing.
func TestDiscoverLegalBullets(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{"an axiom bullet with a bearing",
			"- rests-on:\n  - Axiom S2 — Axiom numbering (bearing 0.8) [frame]\n- strengthen-by:",
			[]string{"founds axm-S2->clm-aaaaaa 0.8 [frame]"}},
		{"a claim bullet beside it",
			"- rests-on:\n  - clm-bbbbbb — B (strength 0.9) (bearing 0.4) [ctx]\n  - Axiom S2 — N (bearing 0.3)",
			[]string{"supports clm-bbbbbb->clm-aaaaaa 0.4 [ctx]", "founds axm-S2->clm-aaaaaa 0.3 []"}},
		{"two-id references bullet",
			"- references:\n  - clm-bbbbbb, clm-cccccc — Two",
			[]string{"references clm-aaaaaa->clm-bbbbbb none []", "references clm-aaaaaa->clm-cccccc none []"}},
		{"a stale id needs no bearing and yields no edge",
			"- rests-on:\n  - clm-zzzzzz — Gone (bearing 0.5)\n  - clm-zzzzzz — Gone",
			nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, err := discoverText(t, registerWith(tc.body))
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if len(st.ClaimEntries) == 0 {
				t.Fatal("no claim entries")
			}
			var got []string
			entry := st.ClaimEntries[0]
			for _, e := range slices.Concat(entry.Premises, entry.References) {
				bearing := "none"
				if e.Bearing != (Bearing{}) {
					bearing = BearingText(e.Bearing)
				}
				ctx := ""
				if e.Context != nil {
					ctx = *e.Context
				}
				got = append(got, e.Relation+" "+e.Source+"->"+e.Target+" "+bearing+" ["+ctx+"]")
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("edges = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDiscoverDropsAnUndeclaredAxiomCitation: an "Axiom <label>" token the
// axiom source does not declare is dropped and logged, as a stale clm- token
// is — a typo costs an edge, not the KB; the same token against a declaration
// yields the edge.
func TestDiscoverDropsAnUndeclaredAxiomCitation(t *testing.T) {
	register := registerWith("- rests-on:\n  - Axiom S2 — Numbering (bearing 0.8)")
	var lg logtest.Capture
	st, err := discoverFiles(t, map[string]string{RegisterFile: register}, &lg)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got := st.ClaimEntries[0].Premises; len(got) != 0 {
		t.Errorf("premises = %+v, want none", got)
	}
	if !lg.Has(t, "info", "target", "axm-S2") {
		t.Errorf("the drop was not logged: %v", lg.Snapshot())
	}

	declared, err := discoverFiles(t, map[string]string{
		RegisterFile: register,
		AxiomsFile:   "# Axioms\n\n### Axiom S2: Numbering\n",
	}, log.Discard())
	if err != nil {
		t.Fatalf("Discover over a KB declaring the axiom: %v", err)
	}
	if got := declared.ClaimEntries[0].Premises; len(got) != 1 || got[0].Source != "axm-S2" {
		t.Errorf("premises = %+v, want the declared axiom's edge", got)
	}
}

// pairFaultsOf is the one AnnotationError Discover returned for files, or a
// fatal where it returned none.
func pairFaultsOf(t *testing.T, files map[string]string) []AnnotationFault {
	t.Helper()
	_, err := discoverFiles(t, files, log.Discard())
	var faulted *AnnotationError
	if !errors.As(err, &faulted) {
		t.Fatalf("Discover = %v, want an AnnotationError", err)
	}
	return faulted.Faults
}

// registerWithAnalysis is a register holding three claims and an analysis whose
// authored section carries body; a second analysis is keyed for a bullet naming
// one.
func registerWithAnalysis(body string) string {
	return registerWith("") + "\n---\n\n## Another Analysis\n<!-- id: ana-bbbbbb -->\n\n---\n\n" +
		"## An Analysis\n<!-- id: ana-aaaaaa -->\n\n### Rigor\n- rigor: 0.8\n" + body +
		"\n\n### Strength\n- strength: *pending*\n"
}

// premiseText is one premise edge as "relation source->target bearing".
func premiseText(e Edge) string {
	return e.Relation + " " + e.Source + "->" + e.Target + " " + BearingText(e.Bearing)
}

// TestDiscoverPremisePairings: a rests-on token is read as the edge its pairing
// allows, refused where the table rejects it, and dropped where it names no
// authored node — a claim and an analysis premise alike.
func TestDiscoverPremisePairings(t *testing.T) {
	const experimentLeaf = "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 1.0\n---\n# The leaf\n"
	const analysisLeaf = "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n---\n# The leaf\n"
	for _, tc := range []struct {
		name, register, leaf, want, drop string
		faulted                          bool
	}{
		{name: "an experiment premise is read",
			register: registerWith("- rests-on:\n  - exp-aaaaaa — Run (bearing 0.7)"), leaf: experimentLeaf,
			want: "supports exp-aaaaaa->clm-aaaaaa 0.7"},
		{name: "an experiment token naming no experiment is dropped",
			register: registerWith("- rests-on:\n  - exp-aaaaaa — Run (bearing 0.7)"),
			drop:     "exp-aaaaaa"},
		{name: "a claim's premise of an analysis is one edge",
			register: registerWithAnalysis("- rests-on:\n  - clm-aaaaaa — A (bearing 0.6)"), leaf: analysisLeaf,
			want: "supports clm-aaaaaa->ana-aaaaaa 0.6"},
		{name: "an analysis premise is refused",
			register: registerWithAnalysis("- rests-on:\n  - ana-bbbbbb — Other (bearing 0.5)"), faulted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{RegisterFile: tc.register}
			if tc.leaf != "" {
				files["vol/a.md"] = tc.leaf
			}
			var lg logtest.Capture
			st, err := discoverFiles(t, files, &lg)
			if tc.faulted {
				var faulted *AnnotationError
				if !errors.As(err, &faulted) || len(faulted.Faults) != 1 {
					t.Fatalf("Discover = %v, want one AnnotationError", err)
				}
				if f := faulted.Faults[0]; f.Field != ListRestsOn || f.Path != RegisterFile ||
					f.Line != lineOf(t, tc.register, "ana-bbbbbb — Other") || f.Detail == "" || f.Remedy == "" {
					t.Errorf("fault = %+v, want %s at %s:%d", f, ListRestsOn, RegisterFile, lineOf(t, tc.register, "ana-bbbbbb — Other"))
				}
				return
			}
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			var got []string
			for _, e := range st.ClaimEntries[0].Premises {
				got = append(got, premiseText(e))
			}
			for _, s := range st.Analyses {
				for _, e := range s.Premises {
					got = append(got, premiseText(e))
				}
			}
			var want []string
			if tc.want != "" {
				want = []string{tc.want}
			}
			if !slices.Equal(got, want) {
				t.Errorf("premises = %q, want %q", got, want)
			}
			if tc.drop != "" && !lg.Has(t, "info", "target", tc.drop) {
				t.Errorf("the drop was not logged: %v", lg.Snapshot())
			}
		})
	}
}

// TestAnAuthoredRigorAboveTheMaximumReadsAtIt: 1.00 belongs to an axiom by
// rule, so an authored value standing above the rigor tables' maximum reads as
// that maximum whatever kind carries it — the claim, analysis and work
// registers and the experiment a leaf hosts — while the document keeps the
// value it wrote. Pending stays pending, and a value at the maximum reads as
// itself.
func TestAnAuthoredRigorAboveTheMaximumReadsAtIt(t *testing.T) {
	register := "# Register\n\n" +
		"## A Claim\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n- rigor: 1.0\n\n---\n\n" +
		"## An Analysis\n<!-- id: ana-aaaaaa -->\n\n### Rigor\n- rigor: 1.0\n\n---\n\n" +
		"## A Work\n<!-- id: wrk-Smith2020 -->\n\n### Rigor\n- rigor: 1.0\n\n---\n\n" +
		"## A Claim At The Maximum\n<!-- id: clm-bbbbbb -->\n\n### Rigor\n- rigor: 0.95\n\n---\n\n" +
		"## A Pending Claim\n<!-- id: clm-cccccc -->\n\n### Rigor\n- rigor: *pending*\n"
	const leaf = "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 1.0\n" +
		"analysis-nodes:\n  - ana-id: ana-aaaaaa\n---\n# The leaf\n"
	st, err := discoverFiles(t, map[string]string{RegisterFile: register, "vol/a.md": leaf}, log.Discard())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	atMaximum := RigorMaximum
	rigors := map[string]*float64{}
	for _, c := range st.ClaimEntries {
		rigors[c.ID] = c.Rigor
	}
	for _, a := range st.Analyses {
		rigors[a.ID] = a.Rigor
	}
	for _, w := range st.Works {
		rigors[w.ID] = w.Rigor
	}
	for _, x := range st.Experiments {
		rigors[x.ID] = x.Rigor
	}
	for _, tc := range []struct {
		id   string
		want *float64
	}{
		{"clm-aaaaaa", &atMaximum}, {"ana-aaaaaa", &atMaximum}, {"wrk-Smith2020", &atMaximum},
		{"exp-aaaaaa", &atMaximum}, {"clm-bbbbbb", &atMaximum}, {"clm-cccccc", nil},
	} {
		got, ok := rigors[tc.id]
		if !ok {
			t.Errorf("%s is in no discovered node", tc.id)
			continue
		}
		if (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
			t.Errorf("%s reads rigor %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestDiscoverRefusesALeafSupportsPair: an analysis's supports list names
// pairings the table rejects, refused at each pair's line rather than dropped.
func TestDiscoverRefusesALeafSupportsPair(t *testing.T) {
	leaf := "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n    supports:\n      - clm-aaaaaa: 0.7\n---\n# L\n"
	faults := pairFaultsOf(t, map[string]string{RegisterFile: registerWith(""), "vol/a.md": leaf})
	if len(faults) != 1 {
		t.Fatalf("faults = %+v, want one", faults)
	}
	if f := faults[0]; f.Field != SupportsKey || f.Path != "vol/a.md" || f.Line != lineOf(t, leaf, "- clm-aaaaaa: 0.7") {
		t.Errorf("fault = %+v, want %s at vol/a.md:%d", f, SupportsKey, lineOf(t, leaf, "- clm-aaaaaa: 0.7"))
	}
}

// TestDiscoverRefusesALeafPairLine: a pair line in a hosting leaf that carries
// no readable bearing is refused at its own line, by field and path — as a
// rests-on bullet is.
func TestDiscoverRefusesALeafPairLine(t *testing.T) {
	for _, tc := range []struct{ name, leaf, needle string }{
		{"no value", "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n    lifts:\n      - clm-aaaaaa:\n---\n# L\n", "- clm-aaaaaa:"},
		{"out of domain", "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 1.0\n    refutes:\n      - clm-aaaaaa: 1.5\n---\n# L\n", "- clm-aaaaaa: 1.5"},
		{"a list value", "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n    lifts:\n      - clm-aaaaaa: [0.5]\n---\n# L\n", "- clm-aaaaaa: [0.5]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			faults := pairFaultsOf(t, map[string]string{RegisterFile: registerWith(""), "vol/a.md": tc.leaf})
			if len(faults) != 1 {
				t.Fatalf("faults = %+v, want one", faults)
			}
			f := faults[0]
			if f.Path != "vol/a.md" || f.Line != lineOf(t, tc.leaf, tc.needle) || f.Field == "" || f.Remedy == "" {
				t.Errorf("fault = %+v, want a field at vol/a.md:%d with a remedy", f, lineOf(t, tc.leaf, tc.needle))
			}
		})
	}
}

// TestDiscoverRefusesARegisterStagedPairLine: an analysis entry's staged pair
// lists carry the same faults at the same lines as a leaf's.
func TestDiscoverRefusesARegisterStagedPairLine(t *testing.T) {
	register := "# Register\n\n## A Claim\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n- rigor: 0.9\n\n---\n\n" +
		"## An Analysis\n<!-- id: ana-aaaaaa -->\n\n### Rigor\n- rigor: 0.9\n- lifts:\n  - clm-aaaaaa: sometimes\n"
	faults := pairFaultsOf(t, map[string]string{RegisterFile: register})
	if len(faults) != 1 {
		t.Fatalf("faults = %+v, want one", faults)
	}
	if f := faults[0]; f.Field != LiftsKey || f.Path != RegisterFile || f.Line != lineOf(t, register, "sometimes") {
		t.Errorf("fault = %+v, want %s at %s:%d", f, LiftsKey, RegisterFile, lineOf(t, register, "sometimes"))
	}
}

// registerWithPairs is a register holding three claims and one analysis whose
// staged pair lists are those given.
func registerWithPairs(lists string) string {
	return registerWith("") + "\n---\n\n## An Analysis\n<!-- id: ana-aaaaaa -->\n\n### Rigor\n- rigor: 0.9\n" + lists
}

// TestDiscoverUnionsAStagedPairIntoTheLeafs: a pair authored only in the
// register reaches the walk, and where both homes carry the same target the
// leaf's declared bearing is the one edge — never two.
func TestDiscoverUnionsAStagedPairIntoTheLeafs(t *testing.T) {
	register := registerWithPairs("- lifts:\n  - clm-aaaaaa: 0.5\n  - clm-bbbbbb: 0.4\n- refutes:\n  - clm-cccccc: 0.3\n")
	leaf := "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n    lifts:\n      - clm-aaaaaa: 0.25\n---\n# L\n"
	st, err := discoverFiles(t, map[string]string{RegisterFile: register, "vol/a.md": leaf}, log.Discard())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(st.Analyses) != 1 {
		t.Fatalf("analyses = %+v, want one", st.Analyses)
	}
	got := map[string]string{}
	for _, list := range []struct {
		relation string
		edges    []Edge
	}{{RelationLifts, st.Analyses[0].Lifts}, {RelationRefutes, st.Analyses[0].Refutes}} {
		for _, e := range list.edges {
			key := list.relation + " " + e.Target
			if _, dup := got[key]; dup {
				t.Errorf("%s carries two edges; one target is one edge", key)
			}
			got[key] = BearingText(e.Bearing)
		}
	}
	want := map[string]string{"lifts clm-aaaaaa": "0.25", "lifts clm-bbbbbb": "0.4", "refutes clm-cccccc": "0.3"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("edges = %v, want %v", got, want)
	}
}

// TestWalkRegistersFanOutCoversRefutations: a refutation both ends record at
// different bearings is the fan-out's finding, as a lift is.
func TestWalkRegistersFanOutCoversRefutations(t *testing.T) {
	register := registerWithPairs("- refutes:\n  - clm-aaaaaa: 0.5\n")
	leaf := "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n    refutes:\n      - clm-aaaaaa: 0.2\n---\n# L\n"
	repo := t.TempDir()
	root := filepath.Join(repo, KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	src := MigratedSource(root, FormatVersion, map[string][]byte{
		"kb-root/" + RegisterFile: []byte(register), "kb-root/vol/a.md": []byte(leaf)}, nil)
	st, err := Discover(src, log.Discard())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	w, err := WalkRegisters(st, src)
	if err != nil {
		t.Fatalf("WalkRegisters: %v", err)
	}
	got := w.Unreconciled()
	if len(got) != 1 || got[0].Relation != RelationRefutes || got[0].ClaimID != "clm-aaaaaa" {
		t.Fatalf("unreconciled = %+v, want the refutation both ends record differently", got)
	}
	if r := got[0]; r.Staged == nil || *r.Staged != (Bearing{Value: 0.5}) || r.Declared == nil || *r.Declared != (Bearing{Value: 0.2}) {
		t.Errorf("record = %+v, want the staged and declared bearings it read", r)
	}
}
