package index

import (
	"bufio"
	"bytes"
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

// kbase's refresh and verify measured against kb_tools' over the same inputs.
// test-integration-verify-arxiv runs TestStageKbToolsComparisons to stage the
// inputs, runs each toolchain over them, then runs TestKbToolsComparisons
// over what they left. Without its flags each test skips.
var (
	kbtoolsStage        = flag.Bool("kbtools.stage", false, "stage the inputs rather than compare the toolchains' results")
	kbtoolsOut          = flag.String("kbtools.out", "", "the directory inputs are staged in and results read from")
	kbtoolsMiniKB       = flag.String("kbtools.minikb", "", "kb_tools' committed mini-kb fixture")
	kbtoolsMaterialized = flag.String("kbtools.materialized", "", "mini-kb as kb_tools' refresh leaves it")
	kbtoolsFixtures     = flag.String("kbtools.fixtures", "", "the directory holding kb_tools-built KBs as <id>/kb-root")
	kbtoolsIDs          = flag.String("kbtools.ids", "", "space-separated fixture ids")
)

// The staged layout: refresh/<case>/{kbtools,kbase}/kb-root, listed in
// refresh/cases.txt; verify/<variant>/kb-root, listed with the refresh each
// needs after its change in verify/variants.txt.
const (
	refreshCases    = "refresh/cases.txt"
	verifyVariants  = "verify/variants.txt"
	afterNone       = "none"
	afterKbTools    = "kbtools-refresh"
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
	writeFile(t, filepath.Join(*kbtoolsOut, name), string(data))
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

// treeDiff is every path whose bytes differ between two trees, the sheet
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
			if filepath.ToSlash(rel) == kb.ClaimGraphFile {
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
	anySolidity    = regexp.MustCompile(`(?m)^- solidity: .*$`)
	annotation     = regexp.MustCompile(`\(solidity [^)]*\)`)
	footerLine     = regexp.MustCompile(`(?m)^> \*\*Leaf references:\*\*.*\n`)
	subtreeField   = regexp.MustCompile(`(?m)^subtree-(claims|experiments): .*$`)
	pendingScore   = regexp.MustCompile(`(?m)^(- (?:confidence|quality|strength): )\*pending\*$`)
	pendingApplies = regexp.MustCompile(`\(applicability \*pending\*\)`)
)

// erase stales every derived field: the index gone, solidity lines and
// annotations wrong, footers dropped, aggregates wrong.
func erase(t *testing.T, root string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, kb.IndexDir)); err != nil {
		t.Fatal(err)
	}
	rewriteMarkdown(t, root, func(text string) string {
		text = anySolidity.ReplaceAllString(text, "- solidity: 0.33 (stale)")
		text = annotation.ReplaceAllString(text, "(solidity 0.11)")
		text = footerLine.ReplaceAllString(text, "")
		return subtreeField.ReplaceAllString(text, "subtree-$1: [clm-stale1]")
	})
}

// score replaces every pending authored score and applicability with a value
// from a fixed cycle that crosses every band and includes 0.
func score(t *testing.T, root string) {
	t.Helper()
	values := []string{"0.95", "0.70", "0.50", "0.30", "0.10", "0.85", "0.00", "1.00", "0.65", "0.45"}
	n := 0
	next := func() string { n++; return values[n%len(values)] }
	rewriteMarkdown(t, root, func(text string) string {
		text = pendingScore.ReplaceAllStringFunc(text, func(m string) string {
			return pendingScore.ReplaceAllString(m, "${1}") + next()
		})
		return pendingApplies.ReplaceAllStringFunc(text, func(string) string { return "(applicability " + next() + ")" })
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

// A variant is mini-kb as kb_tools' refresh leaves it, changed the way one
// of kb_tools' verifier tests changes it.
type variant struct {
	name   string
	mutate func(t *testing.T, root string)
	// after is the refresh kb_tools' test runs once it has changed the KB.
	after string
	// divergence is the named divergence the metadata gates differ by.
	divergence string
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
// from it, as kb_tools' stale-value tests do.
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

func expLeaf(expID, status, strengthens, extra, title string) string {
	fields := []string{"exp-id: " + expID, "status: " + status}
	if extra != "" {
		fields = append(fields, extra)
	}
	fields = append(fields, "strengthens:")
	body := strings.Join(fields, "\n")
	if strengthens != "" {
		body += "\n" + strengthens
	}
	return "[↑ Mini-KB Common](index.md)\n\n<!-- kb-frontmatter\nkind: leaf\n" + body + "\n-->\n\n# " + title + "\n\nSynthetic experiment leaf body.\n"
}

func refLeaf(experiments, title string) string {
	return "[↑ Mini-KB Common](index.md)\n\n<!-- kb-frontmatter\nkind: leaf\nno-claim: \"references the bench experiment only\"\nexperiments: [" +
		experiments + "]\n-->\n\n## " + title + "\n\nSynthetic by-methodology leaf referencing an experiment.\n"
}

func supLeaf(supID, supports, title string) string {
	return "[↑ Mini-KB Common](index.md)\n\n<!-- kb-frontmatter\nkind: leaf\nno-claim: \"hosts a support node only\"\nsup-id: " + supID +
		"\nsupports:\n" + supports + "\n-->\n\n## " + title + "\n\nSynthetic support leaf body.\n"
}

const blocklessDoc = "[↑ Mini-KB Common](../index.md)\n\n## Blockless\n\nSynthetic body.\n"

func edge(source, target, relation, kind, strength, fraction string) string {
	return fmt.Sprintf(`{"source": %q, "target": %q, "relation": %q, "target_kind": %q, "target_solidity_recorded": null, "strength": %s, "context": null, "fraction": %s}`+"\n",
		source, target, relation, kind, strength, fraction)
}

func stage(block string) func(*testing.T, string) {
	anchor := "- rationale: synthetic free-standing support; sup_solidity equals quality (no deps)."
	return replaceIn("common/claim-quality.md", anchor, anchor+"\n"+block)
}

// variants are kb_tools' verifier tests (tests/test_check_index.py) that
// change mini-kb, in its order, and two of kbase's own.
var variants = []variant{
	{name: "as-materialized"},
	{name: "stale-cites", mutate: func(t *testing.T, root string) {
		p := filepath.Join(root, ".index/cites.jsonl")
		var kept []string
		for _, l := range strings.Split(readFile(t, p), "\n") {
			if l != "" {
				kept = append(kept, l)
			}
		}
		writeFile(t, p, strings.Join(kept[1:], "\n")+"\n")
	}},
	{name: "missing-strengthen-by", mutate: remove(".index/strengthen-by.jsonl")},
	{name: "malformed-claims", mutate: appendTo(".index/claims.jsonl", "not-a-json\n")},
	{name: "absent-sheet", mutate: remove(kb.ClaimGraphFile)},
	{name: "hand-edited-drawn-sheet", mutate: replaceIn(kb.ClaimGraphFile, "</svg>", "<!-- hand-edited -->\n</svg>"),
		divergence: "claim-graph.svg: kbase checks only its own placeholder sheet (SPEC §3, §4)"},
	{name: "orphan-edge", mutate: appendTo(".index/depends-on.jsonl", `{"source": "clm-aa1111", "target": "clm-zzz999", "target_solidity_recorded": null, "context": null}`+"\n")},
	{name: "target-kind-mismatch", mutate: appendTo(".index/depends-on.jsonl", edge("clm-aa1111", "INVARIANT-S2", "depends", "claim", "null", "null"))},
	{name: "stale-solidity-line", mutate: restamp("common/claim-quality.md", regexp.MustCompile(`(?m)^- solidity: (0\.\d+) \(`))},
	{name: "stale-annotation", mutate: restamp("claim-quality.md", regexp.MustCompile(`\(solidity (0\.\d+)\)`))},
	{name: "cycle", mutate: replaceIn("common/claim-quality.md", "- depends-on:\n  - INVARIANT-S2 (context A",
		"- depends-on:\n  - clm-hh8888 — Double-Framework-Edge Claim H (solidity 0.70)\n  - INVARIANT-S2 (context A")},
	{name: "orphan-quality-block", mutate: appendTo("claim-quality.md",
		"\n\n---\n\n### Quality\n- confidence: *pending*\n- solidity: *pending*\n- rationale: *pending*\n- strengthen-by:\n  - *pending*\n")},
	{name: "experiment-orphan-target", after: afterKbTools, mutate: put("common/exp-orphan.md", expLeaf("exp-orph01", "run", "  - clm-zzz999: 1.0", "", "Orphan-Target Experiment"))},
	{name: "experiment-pending", after: afterKbTools, mutate: put("common/exp-bench.md", expLeaf("exp-bench1", "pending", "  - clm-gg7777: 0.80", "", "Pending Bench Experiment"))},
	{name: "experiment-bad-id", after: afterKbTools, mutate: put("common/exp-bad.md", expLeaf("exp-TOOLONG9", "run", "  - clm-gg7777: 0.50", "", "Negative Experiment"))},
	{name: "experiment-bad-status", after: afterKbTools, mutate: put("common/exp-bad.md", expLeaf("exp-neg001", "halfway", "  - clm-gg7777: 0.50", "", "Negative Experiment"))},
	{name: "experiment-strength-out-of-range", after: afterKbTools, mutate: put("common/exp-bad.md", expLeaf("exp-neg001", "run", "  - clm-gg7777: 5.0", "", "Negative Experiment"))},
	{name: "experiment-cohosting-claims", after: afterKbTools, mutate: put("common/exp-co.md", expLeaf("exp-neg001", "run", "  - clm-gg7777: 0.50", "claims: [clm-aa1111]", "Negative Experiment"))},
	{name: "reference-orphan", after: afterKbTools, mutate: put("common/ref-orphan.md", refLeaf("exp-zzzzzz", "Orphan Ref Leaf"))},
	{name: "reference-to-claim", after: afterKbTools, mutate: put("common/ref-claim.md", refLeaf("clm-gg7777", "Claim-Ref Leaf"))},
	{name: "experiment-leaf-with-reference", after: afterKbTools, mutate: put("common/exp-bench.md",
		expLeaf("exp-bench1", "run", "  - clm-gg7777: 0.80", "experiments: [exp-bench1]", "Self-Referencing Experiment"))},
	{name: "subtree-experiments-drift", mutate: func(t *testing.T, root string) {
		p := filepath.Join(root, "common/index.md")
		writeFile(t, p, regexp.MustCompile(`subtree-experiments: \[[^\]]*\]`).ReplaceAllString(readFile(t, p), "subtree-experiments: []"))
	}},
	{name: "support-orphan-target", after: afterKbTools, mutate: put("common/sup-orphan.md", supLeaf("sup-orph01", "  - clm-zzz999: 1.0", "Orphan-Target Support"))},
	{name: "support-fraction-below-zero", after: afterKbTools, mutate: put("common/sup-bad.md", supLeaf("sup-neg001", "  - clm-bb2222: -0.5", "Negative Support"))},
	{name: "support-fraction-above-one", after: afterKbTools, mutate: put("common/sup-bad.md", supLeaf("sup-neg001", "  - clm-bb2222: 1.5", "Negative Support"))},
	{name: "support-bad-id", after: afterKbTools, mutate: put("common/sup-bad.md", supLeaf("sup-TOOLONG9", "  - clm-bb2222: 1.0", "Negative Support"))},
	{name: "supports-edge-to-experiment", mutate: appendTo(".index/depends-on.jsonl", edge("sup-free01", "exp-bench1", "supports", "claim", "null", "1.0"))},
	{name: "supports-fraction-in-index", mutate: appendTo(".index/depends-on.jsonl", edge("sup-free01", "clm-bb2222", "supports", "claim", "null", "1.5"))},
	{name: "strengthens-strength-in-index", mutate: appendTo(".index/depends-on.jsonl", edge("exp-bench1", "clm-bb2222", "strengthens", "claim", "5.0", "null"))},
	{name: "zero-on-point-fraction", after: afterKbTools, mutate: replaceIn("common/sup-free.md", "clm-sb2222: 0.50", "clm-sb2222: 0.0")},
	{name: "marker-above-heading", mutate: replaceIn("common/claim-quality.md",
		"## Pending Upstream Claim F\n<!-- id: clm-ff6666 -->", "<!-- id: clm-ff6666 -->\n## Pending Upstream Claim F")},
	{name: "marker-under-another-heading", mutate: replaceIn("common/claim-quality.md",
		"## Pending Upstream Claim F\n<!-- id: clm-ff6666 -->", "## Preamble\n\nnot an entry.\n\n<!-- id: clm-ff6666 -->\n\n## Pending Upstream Claim F")},
	{name: "staged-fan-out-agrees", mutate: stage("- supports:\n  - clm-sb1111: 1.0\n  - clm-sb2222: 0.50\n")},
	{name: "staged-fan-out-extra", mutate: stage("- supports:\n  - clm-sb1111: 1.0\n  - clm-sb2222: 0.50\n  - clm-sb3333: 0.25\n")},
	{name: "staged-fan-out-short", mutate: stage("- supports:\n  - clm-sb1111: 1.0\n")},
	{name: "staged-fan-out-differs", mutate: stage("- supports:\n  - clm-sb1111: 1.0\n  - clm-sb2222: 0.75\n")},
	{name: "blockless-leaf", mutate: put("common/blockless.md", blocklessDoc)},
	{name: "blockless-index", mutate: put("common/blockless/index.md", blocklessDoc)},
	{name: "refreshed-by-kbase", after: afterKbase, mutate: remove(kb.ClaimGraphFile),
		divergence: "claim-graph.svg: kb_tools' sheet check is red on kbase's placeholder until kb_tools' refresh runs (SPEC §3)"},
}

type refreshInput struct {
	name, src string
	mutate    func(*testing.T, string)
}

func refreshInputs() []refreshInput {
	inputs := []refreshInput{{"mini-kb", *kbtoolsMiniKB, nil}}
	for _, id := range strings.Fields(*kbtoolsIDs) {
		src := filepath.Join(*kbtoolsFixtures, id, kb.KBDir)
		inputs = append(inputs,
			refreshInput{id + "/as-built", src, nil},
			refreshInput{id + "/stale", src, erase},
			refreshInput{id + "/scored", src, func(t *testing.T, root string) { score(t, root); erase(t, root) }})
	}
	return inputs
}

// TestStageKbToolsComparisons stages every input both toolchains run over.
// Refresh inputs: mini-kb as committed, and each fixture as built, with its
// derived fields stale, and with every pending score valued — one copy per
// toolchain. Verify inputs: each variant of mini-kb.
func TestStageKbToolsComparisons(t *testing.T) {
	if !*kbtoolsStage || *kbtoolsOut == "" {
		t.Skip("no staging asked for; test-integration-verify-arxiv asks for it")
	}
	for _, dir := range []string{"refresh", "verify"} {
		if err := os.RemoveAll(filepath.Join(*kbtoolsOut, dir)); err != nil {
			t.Fatal(err)
		}
	}
	var cases strings.Builder
	for _, in := range refreshInputs() {
		for _, side := range []string{"kbtools", "kbase"} {
			root := filepath.Join(*kbtoolsOut, "refresh", in.name, side, kb.KBDir)
			copyTree(t, in.src, root)
			if in.mutate != nil {
				in.mutate(t, root)
			}
		}
		cases.WriteString(in.name + "\n")
	}
	writeFile(t, filepath.Join(*kbtoolsOut, refreshCases), cases.String())
	var list strings.Builder
	for _, v := range variants {
		root := filepath.Join(*kbtoolsOut, "verify", v.name, kb.KBDir)
		copyTree(t, *kbtoolsMaterialized, root)
		if v.mutate != nil {
			v.mutate(t, root)
		}
		after := v.after
		if after == "" {
			after = afterNone
		}
		fmt.Fprintf(&list, "%s %s\n", v.name, after)
	}
	writeFile(t, filepath.Join(*kbtoolsOut, verifyVariants), list.String())
}

type refreshCase struct {
	Case        string   `yaml:"case"`
	KbToolsExit int      `yaml:"kbtools-exit"`
	KbaseExit   int      `yaml:"kbase-exit"`
	Differences []string `yaml:"differences"`
}

// gate is one gate's verdict from each toolchain: for the link and citation
// gates each side's finding count (-1 where kb_tools failed without one),
// for the metadata gate kb_tools' exit code and kbase's finding count.
type gate struct {
	KbTools int `yaml:"kbtools"`
	Kbase   int `yaml:"kbase"`
}

type verdict struct {
	Variant    string `yaml:"variant"`
	Links      gate   `yaml:"links"`
	Metadata   gate   `yaml:"metadata"`
	Citations  gate   `yaml:"citations"`
	Divergence string `yaml:"divergence,omitempty"`
	Agrees     bool   `yaml:"agrees"`
}

var (
	gatingErrorsRE = regexp.MustCompile(`gating errors: (\d+)`)
	citationLineRE = regexp.MustCompile(`(?m)^  \[`)
)

// kbaseGates is a verify result's findings counted by gate.
func kbaseGates(t *testing.T, path string) (links, metadata, citations int) {
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
		switch {
		case f.Check == "dead link", f.Check == "unknown id":
			links++
		case strings.HasPrefix(f.Check, "citation "):
			citations++
		default:
			metadata++
		}
	}
	return links, metadata, citations
}

// TestKbToolsComparisons reads what each toolchain left over the staged
// inputs. Refresh: both exit 0 and every byte of kb-root but the sheet
// agrees. Verify: the link and citation gates report the same number of
// findings, and the metadata gates pass and fail together except where a
// named divergence says otherwise.
func TestKbToolsComparisons(t *testing.T) {
	if *kbtoolsStage || *kbtoolsOut == "" {
		t.Skip("no results named; test-integration-verify-arxiv names them")
	}
	var refreshed []refreshCase
	for _, c := range lines(t, filepath.Join(*kbtoolsOut, refreshCases)) {
		dir := filepath.Join(*kbtoolsOut, "refresh", c[0])
		res := refreshCase{
			Case:        c[0],
			KbToolsExit: exitCode(t, filepath.Join(dir, "kbtools-refresh")),
			KbaseExit:   exitCode(t, filepath.Join(dir, "kbase-refresh")),
			Differences: treeDiff(t, filepath.Join(dir, "kbtools", kb.KBDir), filepath.Join(dir, "kbase", kb.KBDir)),
		}
		refreshed = append(refreshed, res)
		if res.KbToolsExit != 0 || res.KbaseExit != 0 || len(res.Differences) > 0 {
			t.Errorf("refresh %s: %+v", c[0], res)
		}
	}
	writeEvidence(t, "refresh-agreement.yaml", refreshed)

	divergence := map[string]string{}
	for _, v := range variants {
		divergence[v.name] = v.divergence
	}
	var verdicts []verdict
	for _, v := range lines(t, filepath.Join(*kbtoolsOut, verifyVariants)) {
		dir := filepath.Join(*kbtoolsOut, "verify", v[0])
		vd := verdict{Variant: v[0], Divergence: divergence[v[0]]}
		if m := gatingErrorsRE.FindStringSubmatch(readFile(t, filepath.Join(dir, "kbtools-links.log"))); m != nil {
			vd.Links.KbTools, _ = strconv.Atoi(m[1])
		} else if exitCode(t, filepath.Join(dir, "kbtools-links")) != 0 {
			vd.Links.KbTools = -1
		}
		vd.Citations.KbTools = len(citationLineRE.FindAllString(readFile(t, filepath.Join(dir, "kbtools-citations.log")), -1))
		if exitCode(t, filepath.Join(dir, "kbtools-citations")) != 0 && vd.Citations.KbTools == 0 {
			vd.Citations.KbTools = -1
		}
		vd.Metadata.KbTools = exitCode(t, filepath.Join(dir, "kbtools-metadata"))
		vd.Links.Kbase, vd.Metadata.Kbase, vd.Citations.Kbase = kbaseGates(t, filepath.Join(dir, kbaseVerifyFile))
		metaAgree := (vd.Metadata.KbTools != 0) == (vd.Metadata.Kbase > 0)
		vd.Agrees = vd.Links.KbTools == vd.Links.Kbase && vd.Citations.KbTools == vd.Citations.Kbase && metaAgree == (vd.Divergence == "")
		verdicts = append(verdicts, vd)
		if !vd.Agrees {
			t.Errorf("verify %s: %+v", v[0], vd)
		}
	}
	writeEvidence(t, "verify-agreement.yaml", verdicts)
}
