package index

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
)

// kbase's refresh and verify measured against the reference's over the same
// inputs. TestStageReferenceComparisons stages the inputs, a caller runs each
// toolchain over them, then TestReferenceComparisons compares what they left.
// Without its flags each test skips.
var (
	referenceStage        = flag.Bool("reference.stage", false, "stage the inputs rather than compare the toolchains' results")
	referenceOut          = flag.String("reference.out", "", "the directory inputs are staged in and results read from")
	referenceMiniKB       = flag.String("reference.minikb", "", "the reference's committed mini-kb fixture")
	referenceMaterialized = flag.String("reference.materialized", "", "mini-kb as the reference's refresh leaves it")
	referenceFixtures     = flag.String("reference.fixtures", "", "the directory holding the reference's KBs as <id>/kb-root")
	referenceIDs          = flag.String("reference.ids", "", "space-separated fixture ids")
)

// The staged layout: refresh/<case>/{reference,kbase}/kb-root, listed in
// refresh/cases.txt; verify/<variant>/kb-root, listed with the refresh each
// needs after its change in verify/variants.txt.
const (
	refreshCases    = "refresh/cases.txt"
	verifyVariants  = "verify/variants.txt"
	afterNone       = "none"
	afterReference  = "kbtools-refresh"
	afterKbase      = "kbase-refresh"
	kbaseVerifyFile = "kbase-verify.yaml"
)

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeFile(t *testing.T, p, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeEvidence(t *testing.T, name string, v any) {
	t.Helper()
	data, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(*referenceOut, name), string(data))
}

// exitCode is the exit code a step left in <path>.exit.
func exitCode(t *testing.T, path string) int {
	t.Helper()
	code, err := strconv.Atoi(strings.TrimSpace(readFile(t, path+".exit")))
	if err != nil {
		t.Fatalf("%s.exit: %v", path, err)
	}
	return code
}

// lines is a staged list file's lines, each split into fields.
func lines(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out [][]string
	s := bufio.NewScanner(f)
	for s.Scan() {
		if fields := strings.Fields(s.Text()); len(fields) > 0 {
			out = append(out, fields)
		}
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// treeDiff is every path whose bytes differ between two trees, the sheets
// aside.
func treeDiff(t *testing.T, a, b string) []string {
	t.Helper()
	files := func(root string) map[string][]byte {
		out := map[string][]byte{}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			if base := filepath.Base(rel); base == kb.ClaimGraphFile || base == kb.ClaimGraphDigestFile {
				return nil
			}
			data, err := os.ReadFile(p)
			out[filepath.ToSlash(rel)] = data
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	fa, fb := files(a), files(b)
	diff := []string{}
	for p, da := range fa {
		if db, ok := fb[p]; !ok || !bytes.Equal(da, db) {
			diff = append(diff, p)
		}
	}
	for p := range fb {
		if _, ok := fa[p]; !ok {
			diff = append(diff, p)
		}
	}
	slices.Sort(diff)
	return diff
}

var (
	anyStrength  = regexp.MustCompile(`(?m)^- strength: .*$`)
	annotation   = regexp.MustCompile(`\(strength [^)]*\)`)
	footerLine   = regexp.MustCompile(`(?m)^> \*\*Leaf references:\*\*.*\n`)
	subtreeField = regexp.MustCompile(`(?m)^subtree-(claims|experiments): .*$`)
	pendingScore = regexp.MustCompile(`(?m)^(- (?:rigor|strength): )\*pending\*$`)
	pendingArc   = regexp.MustCompile(`\(bearing \*pending\*\)`)
)

// erase stales every derived field: the index gone, strength lines and
// premise annotations wrong, footers dropped, aggregates wrong.
func erase(t *testing.T, root string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, kb.IndexDir)); err != nil {
		t.Fatal(err)
	}
	rewriteMarkdown(t, root, func(text string) string {
		text = anyStrength.ReplaceAllString(text, "- strength: 0.33 (stale)")
		text = annotation.ReplaceAllString(text, "(strength 0.11)")
		text = footerLine.ReplaceAllString(text, "")
		return subtreeField.ReplaceAllString(text, "subtree-$1: [clm-stale1]")
	})
}

// score replaces every pending authored value and bearing with a value from a
// fixed cycle that crosses every band and includes 0.
func score(t *testing.T, root string) {
	t.Helper()
	values := []string{"0.95", "0.70", "0.50", "0.30", "0.10", "0.85", "0.00", "1.00", "0.65", "0.45"}
	n := 0
	next := func() string { n++; return values[n%len(values)] }
	rewriteMarkdown(t, root, func(text string) string {
		text = pendingScore.ReplaceAllStringFunc(text, func(m string) string {
			return pendingScore.ReplaceAllString(m, "${1}") + next()
		})
		return pendingArc.ReplaceAllStringFunc(text, func(string) string { return "(bearing " + next() + ")" })
	})
}

func rewriteMarkdown(t *testing.T, root string, edit func(string) string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if next := edit(string(data)); next != string(data) {
			return os.WriteFile(p, []byte(next), 0o644)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A variant is mini-kb as the reference's refresh leaves it, changed the way
// one of the reference's verifier tests changes it.
type variant struct {
	name   string
	mutate func(t *testing.T, root string)
	// after is the refresh the reference's test runs once it has changed the KB.
	after string
}

func replaceIn(rel, old, new string) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		p := filepath.Join(root, rel)
		text := readFile(t, p)
		if !strings.Contains(text, old) {
			t.Fatalf("%s no longer holds %q", rel, old)
		}
		writeFile(t, p, strings.Replace(text, old, new, 1))
	}
}

func appendTo(rel, text string) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		writeFile(t, filepath.Join(root, rel), readFile(t, filepath.Join(root, rel))+text)
	}
}

func put(rel, text string) func(*testing.T, string) {
	return func(t *testing.T, root string) { writeFile(t, filepath.Join(root, rel), text) }
}

func remove(rel string) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		if err := os.Remove(filepath.Join(root, rel)); err != nil {
			t.Fatal(err)
		}
	}
}

// restamp replaces the first match of re's group 1 in rel with a value far
// from it, as the reference's stale-value tests do.
func restamp(rel string, re *regexp.Regexp) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		p := filepath.Join(root, rel)
		text := readFile(t, p)
		m := re.FindStringSubmatchIndex(text)
		if m == nil {
			t.Fatalf("%s holds no %s", rel, re)
		}
		wrong := "0.01"
		if v, _ := strconv.ParseFloat(text[m[2]:m[3]], 64); v < 0.5 {
			wrong = "0.99"
		}
		writeFile(t, p, text[:m[2]]+wrong+text[m[3]:])
	}
}

// nested is pairs, one per line, indented to sit under a node-list entry's key.
func nested(pairs string) []string {
	var out []string
	for _, l := range strings.Split(pairs, "\n") {
		if l != "" {
			out = append(out, "    "+l)
		}
	}
	return out
}

func expLeaf(expID, status, strengthens, extra, title string) string {
	var fields []string
	if extra != "" {
		fields = append(fields, extra)
	}
	fields = append(fields, "experiment-nodes:", "  - exp-id: "+expID, "    status: "+status, "    rigor: 1.0", "    lifts:")
	fields = append(fields, nested(strengthens)...)
	return "---\nkind: leaf\n" + strings.Join(fields, "\n") + "\n---\n[↑ Mini-KB Common](index.md)\n\n# " + title + "\n\nSynthetic experiment leaf body.\n"
}

func refLeaf(experiments, title string) string {
	return "---\nkind: leaf\nno-claim: \"references the bench experiment only\"\nexperiments: [" + experiments +
		"]\n---\n[↑ Mini-KB Common](index.md)\n\n## " + title + "\n\nSynthetic by-methodology leaf referencing an experiment.\n"
}

func anaLeaf(anaID, lifts, title string) string {
	return "---\nkind: leaf\nno-claim: \"hosts an analysis node only\"\nanalysis-nodes:\n  - ana-id: " + anaID + "\n    lifts:\n" +
		strings.Join(nested(lifts), "\n") + "\n---\n[↑ Mini-KB Common](index.md)\n\n## " + title + "\n\nSynthetic analysis leaf body.\n"
}

const blocklessDoc = "[↑ Mini-KB Common](../index.md)\n\n## Blockless\n\nSynthetic body.\n"

func edge(source, target, relation, kind string, bearing any) string {
	arc, err := json.Marshal(bearing)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(`--- {"source": %q, "target": %q, "relation": %q, "target_kind": %q, "source_strength_recorded": null, "bearing": %s, "context": null}`+"\n",
		source, target, relation, kind, string(arc))
}

func stage(block string) func(*testing.T, string) {
	anchor := "- rationale: synthetic free-standing support; its strength equals its rigor (no deps)."
	return replaceIn("common/claim-rigor.md", anchor, anchor+"\n"+block)
}

// variants are the reference's verifier tests that change mini-kb, in their
// order, and two of kbase's own.
var variants = []variant{
	{name: "as-materialized"},
	{name: "stale-cites", mutate: func(t *testing.T, root string) {
		p := filepath.Join(root, ".index/cites.yaml")
		var kept []string
		for _, l := range strings.Split(readFile(t, p), "\n") {
			if l != "" {
				kept = append(kept, l)
			}
		}
		writeFile(t, p, strings.Join(kept[1:], "\n")+"\n")
	}},
	{name: "missing-strengthen-by", mutate: remove(".index/strengthen-by.yaml")},
	{name: "malformed-claims", mutate: appendTo(".index/claims.yaml", "not-a-json\n")},
	{name: "absent-sheet", mutate: remove(kb.ClaimGraphFile)},
	{name: "hand-edited-drawn-sheet", mutate: replaceIn(kb.ClaimGraphFile, "</svg>", "<!-- hand-edited -->\n</svg>")},
	{name: "orphan-edge", mutate: appendTo(".index/edges.yaml", `--- {"source": "clm-aa1111", "target": "clm-zzz999", "relation": "supports", "target_kind": "claim", "source_strength_recorded": null, "bearing": "1.0", "context": null}`+"\n")},
	{name: "target-kind-mismatch", mutate: appendTo(".index/edges.yaml", edge("INVARIANT-S2", "clm-aa1111", "supports", "claim", "1.0"))},
	{name: "stale-strength-line", mutate: restamp("common/claim-rigor.md", regexp.MustCompile(`(?m)^- strength: (0\.\d+) \(`))},
	{name: "stale-annotation", mutate: restamp("claim-rigor.md", regexp.MustCompile(`\(strength (0\.\d+)\)`))},
	{name: "cycle", mutate: replaceIn("common/claim-rigor.md", "- rests-on:\n  - INVARIANT-S2 (context A",
		"- rests-on:\n  - clm-hh8888 — Double-Framework-Edge Claim H (strength 0.70) (bearing 0.50)\n  - INVARIANT-S2 (context A")},
	{name: "orphan-rigor-block", mutate: appendTo("claim-rigor.md",
		"\n\n---\n\n### Rigor\n- rigor: *pending*\n- strength: *pending*\n- rationale: *pending*\n- strengthen-by:\n  - *pending*\n")},
	{name: "experiment-orphan-target", after: afterReference, mutate: put("common/exp-orphan.md", expLeaf("exp-orph01", "run", "  - clm-zzz999: \"1.0\"", "", "Orphan-Target Experiment"))},
	{name: "experiment-pending", after: afterReference, mutate: put("common/exp-bench.md", expLeaf("exp-bench1", "pending", "  - clm-gg7777: 0.80", "", "Pending Bench Experiment"))},
	{name: "experiment-bad-id", after: afterReference, mutate: put("common/exp-bad.md", expLeaf("exp-TOOLONG9", "run", "  - clm-gg7777: 0.50", "", "Negative Experiment"))},
	{name: "experiment-bad-status", after: afterReference, mutate: put("common/exp-bad.md", expLeaf("exp-neg001", "halfway", "  - clm-gg7777: 0.50", "", "Negative Experiment"))},
	{name: "experiment-strength-out-of-range", after: afterReference, mutate: put("common/exp-bad.md", expLeaf("exp-neg001", "run", "  - clm-gg7777: 5.0", "", "Negative Experiment"))},
	{name: "experiment-cohosting-claims", after: afterReference, mutate: put("common/exp-co.md", expLeaf("exp-neg001", "run", "  - clm-gg7777: 0.50", "claims: [clm-aa1111]", "Negative Experiment"))},
	{name: "reference-orphan", after: afterReference, mutate: put("common/ref-orphan.md", refLeaf("exp-zzzzzz", "Orphan Ref Leaf"))},
	{name: "reference-to-claim", after: afterReference, mutate: put("common/ref-claim.md", refLeaf("clm-gg7777", "Claim-Ref Leaf"))},
	{name: "experiment-leaf-with-reference", after: afterReference, mutate: put("common/exp-bench.md",
		expLeaf("exp-bench1", "run", "  - clm-gg7777: 0.80", "experiments: [exp-bench1]", "Self-Referencing Experiment"))},
	{name: "subtree-experiments-drift", mutate: func(t *testing.T, root string) {
		p := filepath.Join(root, "common/index.md")
		writeFile(t, p, regexp.MustCompile(`subtree-experiments: \[[^\]]*\]`).ReplaceAllString(readFile(t, p), "subtree-experiments: []"))
	}},
	{name: "analysis-orphan-target", after: afterReference, mutate: put("common/ana-orphan.md", anaLeaf("ana-orph01", "  - clm-zzz999: 1.0", "Orphan-Target Analysis"))},
	{name: "analysis-bearing-below-zero", after: afterReference, mutate: put("common/ana-bad.md", anaLeaf("ana-neg001", "  - clm-bb2222: \"-0.5\"", "Negative Analysis"))},
	{name: "analysis-bearing-above-one", after: afterReference, mutate: put("common/ana-bad.md", anaLeaf("ana-neg001", "  - clm-bb2222: \"1.5\"", "Negative Analysis"))},
	{name: "analysis-bad-id", after: afterReference, mutate: put("common/ana-bad.md", anaLeaf("ana-TOOLONG9", "  - clm-bb2222: 1.0", "Negative Analysis"))},
	{name: "supports-edge-to-experiment", mutate: appendTo(".index/edges.yaml", edge("ana-free01", "exp-bench1", "supports", "claim", "1.0"))},
	{name: "supports-bearing-in-index", mutate: appendTo(".index/edges.yaml", edge("ana-free01", "clm-bb2222", "supports", "claim", "1.5"))},
	{name: "lifts-bearing-in-index", mutate: appendTo(".index/edges.yaml", edge("exp-bench1", "clm-bb2222", "lifts", "claim", "5.0"))},
	{name: "zero-bearing", after: afterReference, mutate: replaceIn("common/ana-free.md", "clm-sb2222: 0.50", "clm-sb2222: \"0.0\"")},
	{name: "marker-above-heading", mutate: replaceIn("common/claim-rigor.md",
		"## Pending Upstream Claim F\n<!-- id: clm-ff6666 -->", "<!-- id: clm-ff6666 -->\n## Pending Upstream Claim F")},
	{name: "marker-under-another-heading", mutate: replaceIn("common/claim-rigor.md",
		"## Pending Upstream Claim F\n<!-- id: clm-ff6666 -->", "## Preamble\n\nnot an entry.\n\n<!-- id: clm-ff6666 -->\n\n## Pending Upstream Claim F")},
	{name: "staged-fan-out-agrees", mutate: stage("- lifts:\n  - clm-sb1111: \"1.0\"\n  - clm-sb2222: \"0.50\"\n")},
	{name: "staged-fan-out-extra", mutate: stage("- lifts:\n  - clm-sb1111: \"1.0\"\n  - clm-sb2222: \"0.50\"\n  - clm-sb3333: \"0.25\"\n")},
	{name: "staged-fan-out-short", mutate: stage("- lifts:\n  - clm-sb1111: \"1.0\"\n")},
	{name: "staged-fan-out-differs", mutate: stage("- lifts:\n  - clm-sb1111: \"1.0\"\n  - clm-sb2222: \"0.75\"\n")},
	{name: "blockless-leaf", mutate: put("common/blockless.md", blocklessDoc)},
	{name: "blockless-index", mutate: put("common/blockless/index.md", blocklessDoc)},
	{name: "refreshed-by-kbase", after: afterKbase, mutate: remove(kb.ClaimGraphFile)},
}

type refreshInput struct {
	name, src string
	mutate    func(*testing.T, string)
}

func refreshInputs() []refreshInput {
	inputs := []refreshInput{{"mini-kb", *referenceMiniKB, nil}}
	for _, id := range strings.Fields(*referenceIDs) {
		src := filepath.Join(*referenceFixtures, id, kb.KBDir)
		inputs = append(inputs,
			refreshInput{id + "/as-built", src, nil},
			refreshInput{id + "/stale", src, erase},
			refreshInput{id + "/scored", src, func(t *testing.T, root string) { score(t, root); erase(t, root) }})
	}
	return inputs
}

// TestStageReferenceComparisons stages every input both toolchains run over.
// Refresh inputs: mini-kb as committed, and each fixture as built, with its
// derived fields stale, and with every pending score valued — one copy per
// toolchain. Verify inputs: each variant of mini-kb.
func TestStageReferenceComparisons(t *testing.T) {
	if !*referenceStage || *referenceOut == "" {
		t.Skip("no staging asked for; a caller supplies -reference.stage and -reference.out")
	}
	for _, dir := range []string{"refresh", "verify"} {
		if err := os.RemoveAll(filepath.Join(*referenceOut, dir)); err != nil {
			t.Fatal(err)
		}
	}
	var cases strings.Builder
	for _, in := range refreshInputs() {
		for _, side := range []string{"reference", "kbase"} {
			root := filepath.Join(*referenceOut, "refresh", in.name, side, kb.KBDir)
			copyTree(t, in.src, root)
			if in.mutate != nil {
				in.mutate(t, root)
			}
		}
		cases.WriteString(in.name + "\n")
	}
	writeFile(t, filepath.Join(*referenceOut, refreshCases), cases.String())
	var list strings.Builder
	for _, v := range variants {
		root := filepath.Join(*referenceOut, "verify", v.name, kb.KBDir)
		copyTree(t, *referenceMaterialized, root)
		if v.mutate != nil {
			v.mutate(t, root)
		}
		after := v.after
		if after == "" {
			after = afterNone
		}
		fmt.Fprintf(&list, "%s %s\n", v.name, after)
	}
	writeFile(t, filepath.Join(*referenceOut, verifyVariants), list.String())
}

type refreshCase struct {
	Case        string   `yaml:"case"`
	KbToolsExit int      `yaml:"kbtools-exit"`
	KbaseExit   int      `yaml:"kbase-exit"`
	Differences []string `yaml:"differences"`
}

// gate is one gate's verdict from each toolchain: for the link gate each
// side's finding count (-1 where the reference failed without one), for the
// metadata gate the reference's exit code and kbase's finding count.
type gate struct {
	KbTools int `yaml:"kbtools"`
	Kbase   int `yaml:"kbase"`
}

type verdict struct {
	Variant  string `yaml:"variant"`
	Links    gate   `yaml:"links"`
	Metadata gate   `yaml:"metadata"`
	Agrees   bool   `yaml:"agrees"`
}

var gatingErrorsRE = regexp.MustCompile(`gating errors: (\d+)`)

// kbaseGates is a verify result's findings counted by gate.
func kbaseGates(t *testing.T, path string) (links, metadata int) {
	t.Helper()
	var doc struct {
		Refusals []struct {
			Check string `yaml:"check"`
		} `yaml:"refusals"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, path)), &doc); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	for _, f := range doc.Refusals {
		switch f.Check {
		case "dead link", "unknown id":
			links++
		default:
			metadata++
		}
	}
	return links, metadata
}

// TestReferenceComparisons reads what each toolchain left over the staged
// inputs. Refresh: both exit 0 and every byte of kb-root but the sheets
// agrees. Verify: the link gates report the same number of findings, and the
// metadata gates pass and fail together.
func TestReferenceComparisons(t *testing.T) {
	if *referenceStage || *referenceOut == "" {
		t.Skip("no results named; a caller supplies -reference.out")
	}
	var refreshed []refreshCase
	for _, c := range lines(t, filepath.Join(*referenceOut, refreshCases)) {
		dir := filepath.Join(*referenceOut, "refresh", c[0])
		res := refreshCase{
			Case:        c[0],
			KbToolsExit: exitCode(t, filepath.Join(dir, "kbtools-refresh")),
			KbaseExit:   exitCode(t, filepath.Join(dir, "kbase-refresh")),
			Differences: treeDiff(t, filepath.Join(dir, "reference", kb.KBDir), filepath.Join(dir, "kbase", kb.KBDir)),
		}
		refreshed = append(refreshed, res)
		if res.KbToolsExit != 0 || res.KbaseExit != 0 || len(res.Differences) > 0 {
			t.Errorf("refresh %s: %+v", c[0], res)
		}
	}
	writeEvidence(t, "refresh-agreement.yaml", refreshed)

	var verdicts []verdict
	for _, v := range lines(t, filepath.Join(*referenceOut, verifyVariants)) {
		dir := filepath.Join(*referenceOut, "verify", v[0])
		vd := verdict{Variant: v[0]}
		if m := gatingErrorsRE.FindStringSubmatch(readFile(t, filepath.Join(dir, "kbtools-links.log"))); m != nil {
			vd.Links.KbTools, _ = strconv.Atoi(m[1])
		} else if exitCode(t, filepath.Join(dir, "kbtools-links")) != 0 {
			vd.Links.KbTools = -1
		}
		vd.Metadata.KbTools = exitCode(t, filepath.Join(dir, "kbtools-metadata"))
		vd.Links.Kbase, vd.Metadata.Kbase = kbaseGates(t, filepath.Join(dir, kbaseVerifyFile))
		metaAgree := (vd.Metadata.KbTools != 0) == (vd.Metadata.Kbase > 0)
		vd.Agrees = vd.Links.KbTools == vd.Links.Kbase && metaAgree
		verdicts = append(verdicts, vd)
		if !vd.Agrees {
			t.Errorf("verify %s: %+v", v[0], vd)
		}
	}
	writeEvidence(t, "verify-agreement.yaml", verdicts)
}
