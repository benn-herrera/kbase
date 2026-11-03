package write

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/result"
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

// scriptStep is one write batch of the script: the name its minted id goes
// by, the outcome its re-issue must report, the prefix a refused re-issue's
// first refusal detail must carry, and the batch.
type scriptStep struct {
	Name          string    `yaml:"name"`
	Reissue       string    `yaml:"reissue"`
	ReissueDetail string    `yaml:"reissue-detail"`
	Values        yaml.Node `yaml:"values"`
}

// fixtureChoice is what the placeholders resolve to in one KB: a leaf, the
// 1-based line an add places its marker on, and that line's words.
type fixtureChoice struct {
	Leaf    string
	Line    int
	Snippet string
}

// chooseFixture picks the first leaf, in path order, whose body holds a line
// the excerpt search finds exactly once — and, only where no leaf holds such a
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
			if line, snippet := chooseLine(text, runs); snippet != "" {
				return fixtureChoice{Leaf: leaf, Line: line, Snippet: snippet}
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

// chooseLine is the 1-based number and the words of the first plain-prose
// body line of five or more words the excerpt search finds exactly once; with
// runs, the first such run of plain prose within a line.
func chooseLine(document string, runs bool) (int, string) {
	b := bodyOf(document)
	for i, line := range kb.SplitLines(document)[b.first:] {
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
				return b.first + i + 1, candidate
			}
		}
	}
	return 0, ""
}

// resolvePlaceholders rewrites n in place: fixture placeholders become their
// values, {line} an integer, and {minted:<name>} its run-time token.
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
	case "{line}":
		n.Value, n.Tag, n.Style = strconv.Itoa(c.Line), "!!int", 0
	case "{snippet}":
		n.Value = c.Snippet
	default:
		if m := mintedPlaceholderRE.FindStringSubmatch(n.Value); m != nil {
			n.Value = mintToken(m[1])
		}
	}
}

// batchOperations is each operation the entries of a step's batch name.
func batchOperations(values *yaml.Node) []string {
	var ops []string
	var batch struct {
		Entry []struct {
			Operation string `yaml:"operation"`
		} `yaml:"entry"`
	}
	if err := values.Decode(&batch); err != nil {
		return nil
	}
	for _, e := range batch.Entry {
		ops = append(ops, e.Operation)
	}
	return ops
}

// TestStageOpScript renders the op script against one KB: per step NN.yaml,
// a write batch; steps.tsv naming each step's number, minted name, the
// outcome its re-issue must report and, for a refused re-issue, the prefix of
// its first refusal's detail; and choice.yaml recording what the
// placeholders became.
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
		ops := batchOperations(&step.Values)
		if len(ops) == 0 {
			t.Fatalf("step %d: its values are no batch of entries", i+1)
		}
		for _, op := range ops {
			if !slices.Contains(Ops(), op) {
				t.Fatalf("step %d: %q is not a write op", i+1, op)
			}
		}
		if step.Reissue != result.Refused && step.Reissue != result.Unchanged {
			t.Fatalf("step %d: reissue %q, want %s or %s", i+1, step.Reissue, result.Refused, result.Unchanged)
		}
		if (step.Reissue == result.Refused) != (step.ReissueDetail != "") {
			t.Fatalf("step %d: reissue %s with reissue-detail %q; a refused re-issue states its detail's prefix and an unchanged one none",
				i+1, step.Reissue, step.ReissueDetail)
		}
		resolvePlaceholders(&step.Values, choice)
		values, err := yaml.Marshal(&step.Values)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(*opscriptOut, fmt.Sprintf("%02d.yaml", i+1)), string(values))
		name := step.Name
		if name == "" {
			name = "-"
		}
		detail := step.ReissueDetail
		if detail == "" {
			detail = "-"
		}
		tsv = append(tsv, fmt.Sprintf("%02d\t%s\t%s\t%s", i+1, name, step.Reissue, detail))
	}
	writeFile(t, filepath.Join(*opscriptOut, "steps.tsv"), strings.Join(tsv, "\n")+"\n")
	recorded, err := yaml.Marshal(map[string]any{"leaf": choice.Leaf, "line": choice.Line, "snippet": choice.Snippet})
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
