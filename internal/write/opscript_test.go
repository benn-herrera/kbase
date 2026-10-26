package write

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/kbload"
)

// The compatibility op script, rendered against one KB.
// test-integration-opscript-results runs TestStageOpScript over a scratch copy
// of the results fixture and applies the steps through ./bin/kbase. Without
// its flags it skips.
var (
	opscriptScript = flag.String("opscript.script", "", "the op script, test-data/fixtures/compat/ops.yaml")
	opscriptKBRoot = flag.String("opscript.kbroot", "", "the kb-root the placeholders are resolved against")
	opscriptOut    = flag.String("opscript.out", "", "where the rendered steps are written")
)

// mintToken is how a {minted:<name>} placeholder reaches the rendered steps;
// the runner substitutes the id the named step minted for it.
func mintToken(name string) string { return "@MINT-" + name + "@" }

var mintedPlaceholderRE = regexp.MustCompile(`^\{minted:([A-Za-z0-9_-]+)\}$`)

type scriptStep struct {
	Name   string    `yaml:"name"`
	Op     string    `yaml:"op"`
	Values yaml.Node `yaml:"values"`
}

// fixtureChoice is what the placeholders resolve to in one KB.
type fixtureChoice struct {
	Leaf, Locator string
}

// chooseFixture picks the first leaf, in path order, whose body holds a line
// the locator search finds exactly once — and, only where no leaf holds such a
// line, a plain-prose run within a line, as an unwrapped page offers.
func chooseFixture(t *testing.T, root string) fixtureChoice {
	t.Helper()
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := kb.Documents(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, runs := range []bool{false, true} {
		for _, leaf := range documents {
			text, err := src.ReadText(src.KBPath(leaf))
			if err != nil {
				t.Fatal(err)
			}
			_, _, children := kb.TreeLinks(leaf, text)
			if kb.DocumentKind(leaf, len(children) > 0) != kb.DocumentLeaf {
				continue
			}
			if locator := chooseLocator(text, runs); locator != "" {
				return fixtureChoice{Leaf: leaf, Locator: locator}
			}
		}
	}
	t.Fatalf("%s holds no leaf the op script can be resolved against", root)
	return fixtureChoice{}
}

var (
	plainProseRE    = regexp.MustCompile(`^[A-Za-z][A-Za-z ,.;'-]+$`)
	notPlainProseRE = regexp.MustCompile(`[^A-Za-z ,.;'-]+`)
)

// chooseLocator is the first plain-prose body line of five or more words the
// locator search finds exactly once; with runs, the first such run of plain
// prose within a line.
func chooseLocator(document string, runs bool) string {
	b := bodyOf(document)
	for _, line := range kb.SplitLines(document)[b.first:] {
		candidates := []string{collapse(line)}
		if runs {
			candidates = nil
			for _, run := range notPlainProseRE.Split(line, -1) {
				candidates = append(candidates, strings.Trim(collapse(run), " ,.;'-"))
			}
		}
		for _, candidate := range candidates {
			if !plainProseRE.MatchString(candidate) || len(strings.Fields(candidate)) < 5 || len(candidate) > 200 {
				continue
			}
			if hits := excerptLines(document, candidate); len(hits) == 1 {
				return candidate
			}
		}
	}
	return ""
}

// resolvePlaceholders rewrites n in place: fixture placeholders become their
// values, and {minted:<name>} becomes its run-time token.
func resolvePlaceholders(n *yaml.Node, c fixtureChoice) {
	if n.Kind != yaml.ScalarNode {
		for _, item := range n.Content {
			resolvePlaceholders(item, c)
		}
		return
	}
	switch n.Value {
	case "{leaf}":
		n.Value = c.Leaf
	case "{locator}":
		n.Value = c.Locator
	default:
		if m := mintedPlaceholderRE.FindStringSubmatch(n.Value); m != nil {
			n.Value = mintToken(m[1])
		}
	}
}

// TestStageOpScript renders the op script against one KB: per step
// NN-<op>.yaml, steps.tsv naming each step's number, op and minted name, and
// choice.yaml recording what the placeholders became.
func TestStageOpScript(t *testing.T) {
	if *opscriptScript == "" {
		t.Skip("no op script named; test-integration-opscript-results names one")
	}
	data, err := os.ReadFile(*opscriptScript)
	if err != nil {
		t.Fatal(err)
	}
	var script struct {
		Steps []scriptStep `yaml:"steps"`
	}
	if err := yaml.Unmarshal(data, &script); err != nil {
		t.Fatal(err)
	}
	choice := chooseFixture(t, *opscriptKBRoot)
	if err := os.MkdirAll(*opscriptOut, 0o755); err != nil {
		t.Fatal(err)
	}
	var tsv []string
	for i, step := range script.Steps {
		if _, ok := opFields[step.Op]; !ok {
			t.Fatalf("step %d: %s is not an op", i+1, step.Op)
		}
		resolvePlaceholders(&step.Values, choice)
		values, err := yaml.Marshal(&step.Values)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(*opscriptOut, fmt.Sprintf("%02d-%s.yaml", i+1, step.Op)), string(values))
		name := step.Name
		if name == "" {
			name = "-"
		}
		tsv = append(tsv, fmt.Sprintf("%02d\t%s\t%s", i+1, step.Op, name))
	}
	writeFile(t, filepath.Join(*opscriptOut, "steps.tsv"), strings.Join(tsv, "\n")+"\n")
	recorded, err := yaml.Marshal(map[string]any{"leaf": choice.Leaf, "locator": choice.Locator})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(*opscriptOut, "choice.yaml"), string(recorded))
}

func writeFile(t *testing.T, p, text string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
