package write

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
)

// kb_tools' frozen 2026-09-02 registers: every marker authored above its
// heading, so each register's first entry parses to nothing and every later
// title is its neighbour's. The replay writes the same population through
// this package and requires all of it to read back.

const (
	regressionClaims = 23
	silentID         = "clm-07687j"
)

var markerLineRE = kb.PyRE(`^\s*<!--\s*id:\s*(` + kb.IDBody("clm", "sup") + `)\s*-->\s*$`)

type recorded struct {
	nodeID, register, kind, title, rationale string
	rigor                                    *float64
	dependsOn                                []string
}

// repaired puts every marker back below its own heading — the inverse of the
// defect, touching no title, score or bullet.
func repaired(text string) string {
	lines := kb.SplitLines(text)
	out := slices.Clone(lines)
	for i, line := range lines {
		if markerLineRE.MatchString(line) && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "## ") {
			out[i], out[i+1] = out[i+1], out[i]
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// loadRecorded reads the intended values out of the repaired registers with
// the production readers, in document order.
func loadRecorded(t *testing.T) []recorded {
	t.Helper()
	dir := requireKbTools(t, "fixtures/writeapi-regression")
	sources, err := filepath.Glob(filepath.Join(dir, "*-claim-quality.md"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(sources)
	var out []recorded
	for _, source := range sources {
		rel := strings.TrimSuffix(filepath.Base(source), "-claim-quality.md") + "/" + kb.RegisterFile
		text := repaired(readText(t, source))
		claims := map[string]kb.ClaimEntry{}
		for _, c := range kb.ParseClaimEntries(text, rel, nil, log.Discard()) {
			claims[c.ID] = c
		}
		_, supports := kb.ParseSupportEntries(text, rel, nil, log.Discard())
		for _, line := range kb.SplitLines(text) {
			m := markerLineRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			r := recorded{nodeID: m[1], register: rel}
			var edges []kb.Edge
			if c, ok := claims[m[1]]; ok {
				r.kind, r.title, r.rigor, r.rationale, edges = "clm", c.Title, c.Confidence, c.Rationale, c.DependsOn
			} else if s, ok := supports[m[1]]; ok {
				r.kind, r.title, r.rigor, r.rationale, edges = "sup", s.Title, s.Quality, s.Rationale, s.DependsOn
			} else {
				t.Fatalf("%s: %s yields no record even with its marker repaired", source, m[1])
			}
			for _, e := range edges {
				r.dependsOn = append(r.dependsOn, e.Target)
			}
			out = append(out, r)
		}
	}
	known := map[string]bool{}
	for _, r := range out {
		known[r.nodeID] = true
	}
	for _, r := range out {
		if r.rigor == nil {
			t.Fatalf("%s: rigor is pending; the replay renders numbers only", r.nodeID)
		}
		for _, d := range r.dependsOn {
			if !known[d] {
				t.Fatalf("%s depends on %s, which the frozen corpus does not declare", r.nodeID, d)
			}
		}
	}
	return out
}

func registersOf(recs []recorded) []string {
	var out []string
	for _, r := range recs {
		if !slices.Contains(out, r.register) {
			out = append(out, r.register)
		}
	}
	return out
}

// stampedEntryPoint is an entry point that declares the current format and
// nothing else.
const stampedEntryPoint = "---\nkb-format: \"1.0.0\"\n---\n# KB\n"

func readText(t *testing.T, p string) string {
	t.Helper()
	text, err := kb.OnDisk(filepath.Dir(p)).ReadText(p)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func seedRegisters(t *testing.T, root string, registers []string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, kb.EntryPointFile), []byte(stampedEntryPoint), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range registers {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# "+filepath.Base(filepath.Dir(p))+" Claim Quality Register\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// readBack is id -> (title, depends-on targets) over the written registers,
// through the production readers.
func readBack(t *testing.T, root string, registers []string) map[string][2]any {
	t.Helper()
	seen := map[string][2]any{}
	for _, rel := range registers {
		text := readText(t, filepath.Join(root, filepath.FromSlash(rel)))
		for _, c := range kb.ParseClaimEntries(text, rel, nil, log.Discard()) {
			var targets []string
			for _, e := range c.DependsOn {
				targets = append(targets, e.Target)
			}
			seen[c.ID] = [2]any{c.Title, targets}
		}
	}
	return seen
}

func censusTotals(t *testing.T, root string, registers []string) (markers, records int) {
	t.Helper()
	for _, rel := range registers {
		c := takeCensus(readText(t, filepath.Join(root, filepath.FromSlash(rel))))
		if !c.consistent() {
			t.Errorf("%s: %s", rel, c.describe())
		}
		markers += c.claimMarkers
		records += c.claimRecords
	}
	return markers, records
}

func edgeSet(back map[string][2]any, rename func(string) string) map[[2]string]bool {
	out := map[[2]string]bool{}
	for id, v := range back {
		for _, target := range v[1].([]string) {
			out[[2]string{rename(id), rename(target)}] = true
		}
	}
	return out
}

func recordedEdges(recs []recorded) map[[2]string]bool {
	out := map[[2]string]bool{}
	for _, r := range recs {
		for _, d := range r.dependsOn {
			out[[2]string{r.nodeID, d}] = true
		}
	}
	return out
}

// TestRegressionReplayStore writes the population through render and the
// store with its recorded ids, in one proven act.
func TestRegressionReplayStore(t *testing.T) {
	recs := loadRecorded(t)
	if len(recs) != regressionClaims {
		t.Fatalf("recovered %d markers, want %d", len(recs), regressionClaims)
	}
	root := filepath.Join(t.TempDir(), kb.KBDir)
	registers := registersOf(recs)
	seedRegisters(t, root, registers)
	titles := map[string]string{}
	for _, r := range recs {
		titles[r.nodeID] = r.title
	}
	var edits []edit
	for _, rel := range registers {
		var rendered []string
		expected := &[]expectedEntry{}
		e := edit{path: rel, expect: expected}
		for _, r := range recs {
			if r.register != rel {
				continue
			}
			entryValues := registerEntry{NodeID: r.nodeID, Title: r.title, Score: r.rigor, Rationale: r.rationale, ScoreField: rigorField(r.nodeID)}
			want := expectedEntry{NodeID: r.nodeID, Title: r.title, Rigor: r.rigor, Rationale: r.rationale}
			for _, d := range r.dependsOn {
				entryValues.DependsOn = append(entryValues.DependsOn, dependsOnTarget{Target: d, Title: titles[d]})
				want.DependsOn = append(want.DependsOn, expectedEdge{Target: d})
			}
			rendered = append(rendered, renderEntry(entryValues))
			*expected = append(*expected, want)
			e.claimDelta++
		}
		e.splice = func(document string) (string, error) {
			for _, r := range rendered {
				document = insertEntry(document, r)
			}
			return document, nil
		}
		edits = append(edits, e)
	}
	out, err := applyEdits(kb.OnDisk(root), edits)
	if err != nil || out.status != statusWritten {
		t.Fatalf("applyEdits: %+v, %v", out, err)
	}
	if markers, records := censusTotals(t, root, registers); markers != regressionClaims || records != regressionClaims {
		t.Errorf("markers %d, records %d; want %d of each", markers, records, regressionClaims)
	}
	back := readBack(t, root, registers)
	for _, r := range recs {
		if got, ok := back[r.nodeID]; !ok || got[0] != r.title {
			t.Errorf("%s reads back titled %v, want %q", r.nodeID, got[0], r.title)
		}
	}
	if _, ok := back[silentID]; !ok {
		t.Errorf("%s is absent", silentID)
	}
	if got, want := edgeSet(back, func(s string) string { return s }), recordedEdges(recs); !mapsEqual(got, want) {
		t.Errorf("depends-on graph %v, want %v", got, want)
	}
}

// TestRegressionReplayOps drives the same values through the insert op,
// minting fresh ids, in dependency order; the graph must come back
// isomorphic under the renaming.
func TestRegressionReplayOps(t *testing.T) {
	recs := loadRecorded(t)
	root := filepath.Join(t.TempDir(), kb.KBDir)
	registers := registersOf(recs)
	seedRegisters(t, root, registers)
	minted := map[string]string{}
	remaining := slices.Clone(recs)
	for len(remaining) > 0 {
		var ready, rest []recorded
		for _, r := range remaining {
			blocked := false
			for _, d := range r.dependsOn {
				blocked = blocked || slices.ContainsFunc(remaining, func(o recorded) bool { return o.nodeID == d })
			}
			if blocked {
				rest = append(rest, r)
			} else {
				ready = append(ready, r)
			}
		}
		if len(ready) == 0 {
			t.Fatal("the recorded depends-on graph has a cycle")
		}
		var values []map[string]any
		for _, r := range ready {
			v := map[string]any{"register": r.register, "title": r.title, "rigor": *r.rigor, "rationale": r.rationale}
			var deps []map[string]any
			for _, d := range r.dependsOn {
				deps = append(deps, map[string]any{"id": minted[d]})
			}
			if len(deps) > 0 {
				v["depends-on"] = deps
			}
			values = append(values, v)
		}
		data, err := yaml.Marshal(map[string]any{entryKey: values})
		if err != nil {
			t.Fatal(err)
		}
		res := Run("insert-claim-entry", Options{KBRoot: root, Values: data, NoRefresh: true})
		if res.Outcome != result.Done {
			t.Fatalf("insert-claim-entry: %+v", res)
		}
		for i, r := range ready {
			minted[r.nodeID] = res.IDs[i]
		}
		remaining = rest
	}
	if len(minted) != len(recs) {
		t.Fatalf("minted %d ids, want %d", len(minted), len(recs))
	}
	if markers, records := censusTotals(t, root, registers); markers != regressionClaims || records != regressionClaims {
		t.Errorf("markers %d, records %d; want %d of each", markers, records, regressionClaims)
	}
	back := map[string]string{}
	for old, fresh := range minted {
		back[fresh] = old
		if old == fresh {
			t.Errorf("%s was written under its recorded id", old)
		}
	}
	readback := readBack(t, root, registers)
	for _, r := range recs {
		if got := readback[minted[r.nodeID]]; got[0] != r.title {
			t.Errorf("%s (was %s) reads back titled %v, want %q", minted[r.nodeID], r.nodeID, got[0], r.title)
		}
	}
	for _, rel := range registers {
		landed := readBack(t, root, []string{rel})
		for _, r := range recs {
			if _, ok := landed[minted[r.nodeID]]; (r.register == rel) != ok {
				t.Errorf("%s landed in the wrong register (want %s)", r.nodeID, r.register)
			}
		}
	}
	if got, want := edgeSet(readback, func(s string) string { return back[s] }), recordedEdges(recs); !mapsEqual(got, want) {
		t.Errorf("depends-on graph under the renaming %v, want %v", got, want)
	}
}

// TestRegressionFixtureCarriesDefect: the frozen registers as stored still
// lose one entry each to the marker above its heading — 23 markers, 18
// records — or the replay's 23 of 23 proves nothing.
func TestRegressionFixtureCarriesDefect(t *testing.T) {
	dir := requireKbTools(t, "fixtures/writeapi-regression")
	sources, err := filepath.Glob(filepath.Join(dir, "*-claim-quality.md"))
	if err != nil {
		t.Fatal(err)
	}
	markers, records := 0, 0
	for _, source := range sources {
		c := takeCensus(readText(t, source))
		markers += c.claimMarkers
		records += c.claimRecords
	}
	if markers != regressionClaims || records != 18 {
		t.Errorf("frozen registers hold %d markers and %d records; want %d and 18", markers, records, regressionClaims)
	}
}

func mapsEqual(a, b map[[2]string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
