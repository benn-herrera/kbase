package write

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
)

// The compatibility op script, rendered for both toolchains and their
// results compared. test-integration-write-arxiv runs TestStageOpScript per
// fixture, applies the steps through each toolchain, then runs
// TestCompareOpScript. Without their flags both skip.
var (
	opscriptScript = flag.String("opscript.script", "", "the op script, test_data/fixtures/compat/ops.yaml")
	opscriptKBRoot = flag.String("opscript.kbroot", "", "the kb-root the placeholders are resolved against")
	opscriptOut    = flag.String("opscript.out", "", "where the rendered steps are written")
	opscriptKbase  = flag.String("opscript.kbase", "", "the kb-root kbase's run left")
	opscriptTools  = flag.String("opscript.kbtools", "", "the kb-root kb_tools' run left")
	opscriptIDs    = flag.String("opscript.ids", "", "a file of 'name kbase-id kbtools-id' lines, one per minted id")
	opscriptResult = flag.String("opscript.comparison", "", "where the comparison is written")
)

// mintToken is how a {minted:<name>} placeholder reaches the rendered steps;
// the runner substitutes each toolchain's own id for it.
func mintToken(name string) string { return "@MINT-" + name + "@" }

var mintedPlaceholderRE = regexp.MustCompile(`^\{minted:([A-Za-z0-9_-]+)\}$`)

type scriptStep struct {
	Name   string    `yaml:"name"`
	Op     string    `yaml:"op"`
	Create bool      `yaml:"create"`
	Values yaml.Node `yaml:"values"`
}

// fixtureChoice is what the placeholders resolve to in one KB.
type fixtureChoice struct {
	Register, Leaf, Locator string
	LeafClaims              []string
}

// chooseFixture picks the first leaf, in path order, whose frontmatter
// carries only kind and two or more claims, else only kind and a no-claim,
// whose body holds a line the locator search finds exactly once — and, only
// where no leaf holds such a line, a plain-prose run within a line, as an
// unwrapped page offers. A leaf left citing two claims owes each a marker,
// which a single-claim leaf's first claim may lack.
func chooseFixture(t *testing.T, root string) fixtureChoice {
	t.Helper()
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := kb.Discover(src, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	for _, runs := range []bool{false, true} {
		for _, wantClaims := range []bool{true, false} {
			for _, leaf := range st.Leaves {
				text, err := src.ReadText(src.KBPath(leaf.Path))
				if err != nil {
					t.Fatal(err)
				}
				fm, err := kb.ParseFrontmatter(text)
				if err != nil {
					t.Fatal(err)
				}
				_, noClaim := fm.Get("no-claim")
				if len(fm) != 2 || (wantClaims && len(leaf.Claims) < 2) || (!wantClaims && !noClaim) {
					continue
				}
				locator := chooseLocator(text, runs)
				if locator == "" {
					continue
				}
				volume, _, _ := strings.Cut(leaf.Path, "/")
				register := kb.RegisterFile
				if volume != leaf.Path {
					register = volume + "/" + kb.RegisterFile
				}
				return fixtureChoice{Register: register, Leaf: leaf.Path, Locator: locator, LeafClaims: leaf.Claims}
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
// values, {leaf-claims} as a list item becomes the leaf's claims, and
// {minted:<name>} becomes its run-time token.
func resolvePlaceholders(n *yaml.Node, c fixtureChoice) {
	switch n.Kind {
	case yaml.ScalarNode:
		switch n.Value {
		case "{register}":
			n.Value = c.Register
		case "{leaf}":
			n.Value = c.Leaf
		case "{locator}":
			n.Value = c.Locator
		default:
			if m := mintedPlaceholderRE.FindStringSubmatch(n.Value); m != nil {
				n.Value = mintToken(m[1])
			}
		}
	case yaml.SequenceNode:
		var items []*yaml.Node
		for _, item := range n.Content {
			if item.Kind == yaml.ScalarNode && item.Value == "{leaf-claims}" {
				for _, id := range c.LeafClaims {
					items = append(items, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id})
				}
				continue
			}
			resolvePlaceholders(item, c)
			items = append(items, item)
		}
		n.Content = items
	default:
		for _, item := range n.Content {
			resolvePlaceholders(item, c)
		}
	}
}

// tomlString is a TOML basic string.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// tomlValue is one values node as an inline TOML value.
func tomlValue(t *testing.T, n *yaml.Node) string {
	t.Helper()
	switch n.Kind {
	case yaml.ScalarNode:
		switch n.ShortTag() {
		case "!!str":
			return tomlString(n.Value)
		case "!!int", "!!float", "!!bool":
			return n.Value
		}
		t.Fatalf("line %d: no TOML form for %s", n.Line, n.ShortTag())
	case yaml.SequenceNode:
		var items []string
		for _, item := range n.Content {
			items = append(items, tomlValue(t, item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case yaml.MappingNode:
		var pairs []string
		for i := 0; i < len(n.Content); i += 2 {
			pairs = append(pairs, n.Content[i].Value+" = "+tomlValue(t, n.Content[i+1]))
		}
		return "{ " + strings.Join(pairs, ", ") + " }"
	}
	t.Fatalf("line %d: no TOML form for this node", n.Line)
	return ""
}

// renderTOML is a values document as kb_tools' values file: one [[entry]]
// table per entry.
func renderTOML(t *testing.T, values *yaml.Node) string {
	t.Helper()
	if values.Kind != yaml.MappingNode || len(values.Content) != 2 || values.Content[0].Value != entryKey {
		t.Fatalf("line %d: a step's values hold exactly the %s list", values.Line, entryKey)
	}
	var blocks []string
	for _, e := range values.Content[1].Content {
		lines := []string{"[[entry]]"}
		for i := 0; i < len(e.Content); i += 2 {
			lines = append(lines, e.Content[i].Value+" = "+tomlValue(t, e.Content[i+1]))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

// TestStageOpScript renders the op script against one KB: per step
// NN-<op>.yaml for kbase and NN-<op>.toml for kb_tools, steps.tsv naming each
// step's number, op, --create and minted name, and choice.yaml recording
// what the placeholders became.
func TestStageOpScript(t *testing.T) {
	if *opscriptScript == "" {
		t.Skip("no op script named; test-integration-write-arxiv names one")
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
		base := filepath.Join(*opscriptOut, fmt.Sprintf("%02d-%s", i+1, step.Op))
		kbaseValues, err := yaml.Marshal(&step.Values)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, base+".yaml", string(kbaseValues))
		writeFile(t, base+".toml", renderTOML(t, &step.Values))
		name := step.Name
		if name == "" {
			name = "-"
		}
		tsv = append(tsv, fmt.Sprintf("%02d\t%s\t%t\t%s", i+1, step.Op, step.Create, name))
	}
	writeFile(t, filepath.Join(*opscriptOut, "steps.tsv"), strings.Join(tsv, "\n")+"\n")
	recorded, err := yaml.Marshal(map[string]any{"register": choice.Register, "leaf": choice.Leaf,
		"leaf-claims": choice.LeafClaims, "locator": choice.Locator})
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

// difference is one way two kb-roots disagree once each toolchain's minted
// ids are read as the same node.
type difference struct {
	Path   string `yaml:"path"`
	Kind   string `yaml:"kind"`
	Detail string `yaml:"detail"`
}

var (
	idListLineRE  = regexp.MustCompile(`^(\s*[a-z-]+: \[)([^\]]*)(\]\s*)$`)
	jsonStringsRE = regexp.MustCompile(`\[("[^"\\]*"(?:, "[^"\\]*")*)\]`)
)

// comparable is a file's text with kb_tools' minted ids spelled as kbase's
// and the order of id lists dropped — a frontmatter list, a JSON array of
// strings, an index file's records: the ids are random on each side, so
// anything ordered by id is ordered differently.
func comparable(text string, rename *strings.Replacer, records bool) []string {
	sorted := func(list string) string {
		items := strings.Split(list, ", ")
		slices.Sort(items)
		return strings.Join(items, ", ")
	}
	lines := strings.Split(rename.Replace(text), "\n")
	for i, line := range lines {
		if records {
			lines[i] = jsonStringsRE.ReplaceAllStringFunc(line, func(m string) string { return "[" + sorted(m[1:len(m)-1]) + "]" })
		} else if m := idListLineRE.FindStringSubmatch(line); m != nil {
			lines[i] = m[1] + sorted(m[2]) + m[3]
		}
	}
	if records {
		slices.Sort(lines)
	}
	return lines
}

func treeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCompareOpScript records every difference between the kb-root kbase's
// run left and the one kb_tools' run left, a claim-graph sheet both runs left
// with other content tagged as the one named exception, and fails on any
// other: a sheet one run left and the other did not is a difference.
func TestCompareOpScript(t *testing.T) {
	if *opscriptKbase == "" {
		t.Skip("no results named; test-integration-write-arxiv names them")
	}
	idLines, err := os.ReadFile(*opscriptIDs)
	if err != nil {
		t.Fatal(err)
	}
	var pairs []string
	for _, line := range strings.Split(strings.TrimSpace(string(idLines)), "\n") {
		if f := strings.Fields(line); len(f) == 3 {
			pairs = append(pairs, f[2], f[1])
		}
	}
	rename := strings.NewReplacer(pairs...)
	kbase, tools := treeFiles(t, *opscriptKbase), treeFiles(t, *opscriptTools)
	var paths []string
	for p := range kbase {
		paths = append(paths, p)
	}
	for p := range tools {
		if _, ok := kbase[p]; !ok {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	var diffs []difference
	for _, p := range paths {
		a, inKbase := kbase[p]
		b, inTools := tools[p]
		switch {
		case !inTools:
			diffs = append(diffs, difference{p, "content", "only in kbase's result"})
		case !inKbase:
			diffs = append(diffs, difference{p, "content", "only in kb_tools' result"})
		case a == b:
		default:
			records := path.Base(path.Dir(p)) == kb.IndexDir && path.Ext(p) == kb.IndexFileExt
			left, right := comparable(a, strings.NewReplacer(), records), comparable(b, rename, records)
			if slices.Equal(left, right) {
				continue
			}
			kind := "content"
			if base := path.Base(p); base == kb.ClaimGraphFile || base == kb.ClaimGraphDigestFile {
				kind = "sheet"
			}
			diffs = append(diffs, difference{p, kind, firstDifference(left, right)})
		}
	}
	data, err := yaml.Marshal(map[string]any{"differences": diffs})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, *opscriptResult, string(data))
	for _, d := range diffs {
		if d.Kind != "sheet" {
			t.Errorf("%s: %s", d.Path, d.Detail)
		}
	}
}

func firstDifference(kbase, tools []string) string {
	for i := 0; i < max(len(kbase), len(tools)); i++ {
		var a, b string
		if i < len(kbase) {
			a = kbase[i]
		}
		if i < len(tools) {
			b = tools[i]
		}
		if a != b {
			return fmt.Sprintf("line %d: kbase %q, kb_tools %q", i+1, a, b)
		}
	}
	return ""
}
