package write

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// kbtoolsTests is kb_tools' test tree in the adjagent clone; corpus-driven
// tests skip where it is absent.
const kbtoolsTests = "../../.claude/adjagent/kb_tools/tests"

func requireKbTools(t *testing.T, rel string) string {
	t.Helper()
	dir := filepath.Join(kbtoolsTests, rel)
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("%s is absent; the adjagent clone is not here", dir)
	}
	return dir
}

func f64(v float64) *float64 { return &v }

func str(s string) *string { return &s }

// goldenShapes are kb_tools' render specimens, the same values its render
// test hands its renderers; the name is the golden's file stem.
var goldenShapes = map[string]func() string{
	"claim-entry-full": func() string {
		return renderEntry(registerEntry{
			NodeID: "clm-aa1111", Title: "Regime Conservation Laws", ScoreField: "confidence", Score: f64(0.7),
			Rationale: "CES asymptotics applied with an explicit exponent formula and a cited " +
				"methodology; scope marked as leading-order with error-term bounds.",
			DependsOn: []dependsOnTarget{
				{Target: "clm-bb2222", Title: "Foundation Claim B", Context: "builds directly on the anchor claim"},
				{Target: "clm-cc3333", Title: "Single-Dependency Claim C"},
				{Target: "INVARIANT-S2", Context: "labelling convention for the d-axis treatment"},
				{Target: "Axiom 4"},
			},
			StrengthenBy: []string{
				"Run an independent derivation to confirm the anchor value.",
				"Discharge the open step so the dependency on clm-bb2222 can be closed.",
			},
			NoEdge: "the foreign-domain reference is illustrative, not load-bearing",
		})
	},
	"claim-entry-minimal": func() string {
		return renderEntry(registerEntry{NodeID: "clm-dd4444", Title: "Unassessed Foundation Result", ScoreField: "confidence", Rationale: "*pending*"})
	},
	"support-entry": func() string {
		return renderEntry(registerEntry{NodeID: "sup-ee5555", Title: "Free-Standing Analytical Support", ScoreField: "quality", Score: f64(0.9),
			Rationale: "Non-physical analytical support; lifts a claim without gating it.",
			DependsOn: []dependsOnTarget{{Target: "clm-aa1111", Title: "Regime Conservation Laws"}}})
	},
	"support-entry-staged": func() string {
		return renderEntry(registerEntry{NodeID: "sup-ee5555", Title: "Staged Analytical Support", ScoreField: "quality", Score: f64(0.9),
			Rationale: "Derives the beneficiary result in full; staged before its hosting leaf exists.",
			DependsOn: []dependsOnTarget{{Target: "clm-aa1111", Title: "Regime Conservation Laws"}},
			Supports:  []pair{{ID: "clm-bb2222", Score: f64(0.5)}, {ID: "clm-cc3333"}},
			NoEdge:    "the foreign-domain reference is illustrative, not load-bearing"})
	},
	"frontmatter-path-stable": func() string {
		return rendered(renderFrontmatterBlock(frontmatterValues{Kind: "leaf", PathStable: str("regime conservation — stable reference label"), Claims: []string{"clm-aa1111"}}))
	},
	"frontmatter-claims": func() string {
		return rendered(renderFrontmatterBlock(frontmatterValues{Kind: "leaf", Claims: []string{"clm-aa1111", "clm-bb2222", "clm-cc3333"}, Experiments: []string{"exp-ff6666"}}))
	},
	"frontmatter-no-claim": func() string {
		return rendered(renderFrontmatterBlock(frontmatterValues{Kind: "leaf", NoClaim: str("navigation-only leaf — carries no claim-quality entries")}))
	},
	"frontmatter-hosts": func() string {
		return rendered(renderFrontmatterBlock(frontmatterValues{
			Kind: "leaf", NoClaim: str("hosts an experiment and two support nodes only"),
			ExperimentNodes: []experimentDecl{{ExpID: "exp-gg7777", Status: "run", Strengthens: []pair{{ID: "clm-aa1111", Score: f64(0.8)}}}},
			SupportNodes: []supportDecl{
				{SupID: "sup-hh8888", Supports: []pair{{ID: "clm-bb2222", Score: f64(1.0)}}},
				{SupID: "sup-ii9999", Supports: []pair{{ID: "clm-bb2222"}, {ID: "clm-cc3333", Score: f64(0.5)}}},
			},
		}))
	},
	"markers": func() string {
		return strings.Join([]string{renderIDMarker("clm-aa1111"), renderIDMarker("sup-ee5555"), renderTier2Marker("clm-aa1111")}, "\n")
	},
	"depends-on-bullets": func() string {
		var lines []string
		for _, t := range []dependsOnTarget{
			{Target: "clm-bb2222", Title: "Foundation Claim B", Context: "the anchor dependency"},
			{Target: "clm-bb2222", Title: "Foundation Claim B"},
			{Target: "clm-bb2222"},
			{Target: "INVARIANT-S2", Context: "labelling convention"},
			{Target: "Axiom 4"},
		} {
			lines = append(lines, renderDependsOnBullet(t))
		}
		return strings.Join(lines, "\n")
	},
	"no-edge": func() string {
		return renderNoEdgeLine("cited for context only; the derivation takes nothing from that domain")
	},
	"citation": func() string {
		return renderCitation("the weakest link in the dependency cone", "part3/claim-quality.md", "regime-conservation-laws")
	},
}

// rendered is a renderer's text, or its error in place of it.
func rendered(s string, err error) string {
	if err != nil {
		return "render error: " + err.Error()
	}
	return s
}

// TestFrontmatterRenderedAsTheGoldens: each 1.0.0 golden document, its
// frontmatter read by the production reader and written back by
// set-frontmatter's splice, is byte-identical to the golden — the writer and
// the migration write one frontmatter.
func TestFrontmatterRenderedAsTheGoldens(t *testing.T) {
	const dir = "../migrate/testdata/1.0.0/kb-root"
	for _, rel := range []string{"a.md", "b/c.md", "entry-point.md"} {
		t.Run(rel, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			text := string(golden)
			intended, err := observedFrontmatter(text, rel)
			if err != nil || intended == nil {
				t.Fatalf("observedFrontmatter = %v, %v", intended, err)
			}
			fm, err := kb.ParseFrontmatter(text)
			if err != nil {
				t.Fatal(err)
			}
			got, err := replaceBlock(*intended, rel, kb.DeclaredNodeIDs(fm))(text)
			if err != nil {
				t.Fatal(err)
			}
			if got != text {
				t.Errorf("written back as\n%s\nthe golden is\n%s", got, text)
			}
		})
	}
}

// TestRenderGoldens byte-compares every rendered shape against kb_tools'
// committed golden for it; each golden carries the one trailing newline a
// text file ends with.
func TestRenderGoldens(t *testing.T) {
	dir := requireKbTools(t, "fixtures/writeapi-render-golden")
	onDisk, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var stems []string
	for _, p := range onDisk {
		if stem := strings.TrimSuffix(filepath.Base(p), ".md"); stem != "README" {
			stems = append(stems, stem)
		}
	}
	var shapes []string
	for name := range goldenShapes {
		shapes = append(shapes, name)
	}
	slices.Sort(stems)
	slices.Sort(shapes)
	if !slices.Equal(stems, shapes) {
		t.Fatalf("golden set %v and specimen table %v disagree", stems, shapes)
	}
	for _, name := range shapes {
		t.Run(name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join(dir, name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			if want, got := string(golden), goldenShapes[name]()+"\n"; got != want {
				t.Errorf("render differs from kb_tools' golden\n--- golden\n%s--- kbase\n%s", want, got)
			}
		})
	}
}
