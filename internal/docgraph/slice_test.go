package docgraph

// The slice comparator: kbase's document-graph tree against kb_tools' for the
// same paper, as kb_tools' own readers see both (tools/slice/README.md is the
// dump's schema), plus run-to-run determinism of the tree and records, the negative
// controls, cross-paper citation states, and the corpus census. Each test is
// driven by its flag and skips without it; the recipe
// test-integration-slice-arxiv supplies them over artifacts it guarantees.

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/latex/prepass"
	"kbase/internal/log"
	"kbase/internal/records"
)

var (
	sliceOut         = flag.String("slice.out", "", "the slice evidence directory holding one directory per paper")
	sliceCompare     = flag.String("slice.compare", "", "space-separated paper ids to compare against kb_tools' reference")
	sliceUnparseable = flag.String("slice.unparseable", "", "the negative control that must refuse as unparseable")
	sliceUnloadable  = flag.String("slice.unloadable", "", "the negative control that must refuse naming unloaded includes")
	sliceStates      = flag.String("slice.states", "", "space-separated paper directories whose citation states are pooled")
	sliceCensus      = flag.String("slice.census", "", "space-separated paper ids to take the census of")
	sliceRefs        = flag.String("slice.refs", "", "the directory holding kb_tools' reference per paper: <id>/kb-root, or <id>/driver-stop.yaml")
	sliceRollup      = flag.String("slice.rollup", "", "space-separated paper ids whose comparisons and build checks are rolled up")
)

// Artifacts the recipe leaves in each paper's directory.
const (
	dumpFile         = "dump.json"
	comparisonFile   = "comparison.yaml"
	recordsCheckFile = "records.yaml"
	censusFile       = "census.yaml"
	volumeRootFile   = "volume-root.txt"
	bibliographyTxt  = "bibliographies.txt"
	statesFile       = "citation-states.yaml"
	rollupFile       = "rollup.yaml"
	driverStopFile   = "driver-stop.yaml"
)

func sliceArgs(t *testing.T, ids *string) (string, []string) {
	t.Helper()
	if *sliceOut == "" || strings.TrimSpace(*ids) == "" {
		t.Skip("no slice evidence named; test-integration-slice-arxiv supplies it")
	}
	return *sliceOut, strings.Fields(*ids)
}

// --- the dump -----------------------------------------------------------------

type dumpDocument struct {
	Roots []dumpRoot `json:"roots"`
}

type dumpRoot struct {
	Path        string                      `json:"path"`
	Role        string                      `json:"role"`
	Paper       string                      `json:"paper"`
	Errors      map[string]string           `json:"errors"`
	Tree        []treeEntry                 `json:"tree"`
	Tokens      map[string][]string         `json:"tokens"`
	Forms       map[string]map[string]int   `json:"forms"`
	Conformance *conformance                `json:"conformance"`
	Inventory   map[string][]map[string]any `json:"inventory"`
}

type treeEntry struct {
	Path     string   `json:"path"`
	Children []string `json:"children"`
}

type conformance struct {
	Passed  bool `json:"passed"`
	Failure *struct {
		Check  string `json:"check"`
		Detail string `json:"detail"`
	} `json:"failure"`
}

// readDump returns the paper's kbase root and, where one was dumped, its ref
// root.
func readDump(path, paper string) (kbase, ref *dumpRoot, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var doc dumpDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	for i := range doc.Roots {
		r := &doc.Roots[i]
		if r.Paper != paper {
			continue
		}
		switch r.Role {
		case "kbase":
			kbase = r
		case "ref":
			ref = r
		}
	}
	if kbase == nil {
		return nil, nil, fmt.Errorf("%s holds no kbase root for %s", path, paper)
	}
	return kbase, ref, nil
}

// --- comparison.yaml ----------------------------------------------------------

type checkResult struct {
	Check       string   `yaml:"check"`
	Passed      bool     `yaml:"passed"`
	Detail      string   `yaml:"detail,omitempty"`
	Evidence    []string `yaml:"evidence"`
	Differences []string `yaml:"differences,omitempty"`
}

type paperComparison struct {
	Paper  string        `yaml:"paper"`
	Passed bool          `yaml:"passed"`
	Checks []checkResult `yaml:"checks"`
}

// maxListed bounds the differences one check lists; the count of the rest is
// stated.
const maxListed = 100

func newCheck(name string, evidence []string, differences []string) checkResult {
	if len(differences) > maxListed {
		differences = append(differences[:maxListed], fmt.Sprintf("... and %d more", len(differences)-maxListed))
	}
	return checkResult{Check: name, Passed: len(differences) == 0, Evidence: evidence, Differences: differences}
}

func writeYAML(path string, v any) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// --- the Slice-1 checks ----------------------------------------------------------

func TestSliceCompare(t *testing.T) {
	out, ids := sliceArgs(t, sliceCompare)
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			dir := filepath.Join(out, id)
			c := comparePaper(dir, id)
			if err := writeYAML(filepath.Join(dir, comparisonFile), c); err != nil {
				t.Fatal(err)
			}
			for _, ch := range c.Checks {
				if !ch.Passed {
					t.Errorf("%s: %s failed — see %s", id, ch.Check, filepath.Join(dir, comparisonFile))
				}
			}
		})
	}
}

func comparePaper(dir, id string) paperComparison {
	dump := filepath.Join(dir, dumpFile)
	c := paperComparison{Paper: id}
	built, stopped := compareBuilds(dir, id)
	c.Checks = append(c.Checks, built)
	if stopped {
		c.Passed = built.Passed
		return c
	}
	kbase, ref, err := readDump(dump, id)
	switch {
	case err != nil:
		c.Checks = append(c.Checks, newCheck("dump", []string{dump}, []string{err.Error()}))
	case ref == nil:
		c.Checks = append(c.Checks, newCheck("dump", []string{dump}, []string{"no ref root for " + id}))
	default:
		var errs []string
		for _, r := range []*dumpRoot{ref, kbase} {
			for field, e := range r.Errors {
				errs = append(errs, fmt.Sprintf("%s root %s: %s", r.Role, field, e))
			}
		}
		slices.Sort(errs)
		if len(errs) > 0 {
			c.Checks = append(c.Checks, newCheck("dump", []string{dump}, errs))
		}
		c.Checks = append(c.Checks,
			newCheck("tree", []string{dump}, compareTree(ref, kbase)),
			newCheck("per-leaf content", []string{dump}, compareTokens(ref, kbase)),
			newCheck("load-bearing forms", []string{dump}, compareForms(ref, kbase)),
			newCheck("inventory", []string{dump}, compareInventory(ref, kbase)),
		)
	}
	run1, run2 := filepath.Join(dir, "run-1", "kb-root"), filepath.Join(dir, "run-2", "kb-root")
	rec1, rec2 := filepath.Join(dir, "state-1", records.Dir), filepath.Join(dir, "state-2", records.Dir)
	c.Checks = append(c.Checks, newCheck("determinism", []string{run1, run2, rec1, rec2},
		append(compareDirs(run1, run2), compareDirs(rec1, rec2)...)))
	c.Passed = true
	for _, ch := range c.Checks {
		c.Passed = c.Passed && ch.Passed
	}
	return c
}

// driverStop is kb_tools' driver stopping short of its document-graph commit,
// as the reference recipe records it.
type driverStop struct {
	Exit int    `yaml:"exit"`
	Log  string `yaml:"log"`
}

// compareBuilds: kbase builds the paper with every one of its own checks
// green, and kb_tools builds it too; or both refuse it. A paper only one side
// builds fails. stopped is whether either side left no tree to compare.
func compareBuilds(dir, id string) (checkResult, bool) {
	resultPath := filepath.Join(dir, "run-1.result.yaml")
	refTree := filepath.Join(*sliceRefs, id, "kb-root")
	stopPath := filepath.Join(*sliceRefs, id, driverStopFile)
	evidence := []string{resultPath}
	res, err := readBuildResult(resultPath)
	if err != nil {
		return newCheck("build", evidence, []string{err.Error()}), true
	}
	var stop *driverStop
	if data, err := os.ReadFile(stopPath); err == nil {
		stop = &driverStop{}
		if err := yaml.Unmarshal(data, stop); err != nil {
			return newCheck("build", append(evidence, stopPath), []string{err.Error()}), true
		}
		evidence = append(evidence, stopPath, stop.Log)
	} else if _, err := os.Stat(refTree); err == nil {
		evidence = append(evidence, refTree)
	} else {
		return newCheck("build", evidence, []string{"kb_tools' reference holds neither a tree nor a driver stop at " + filepath.Dir(refTree)}), true
	}
	refused := res.Outcome == "refused"
	switch {
	case refused && stop != nil:
		c := newCheck("build", evidence, nil)
		c.Detail = fmt.Sprintf("both refuse: kbase %q; kb_tools' driver stopped with exit %d", res.Refusals, stop.Exit)
		return c, true
	case refused:
		return newCheck("build", evidence, []string{fmt.Sprintf("kbase refused %q and kb_tools built the paper", res.Refusals)}), true
	case stop != nil:
		return newCheck("build", evidence, []string{fmt.Sprintf("kbase's outcome is %s and kb_tools' driver stopped with exit %d", res.Outcome, stop.Exit)}), true
	}
	var diffs []string
	if res.Outcome != "bounded" {
		diffs = append(diffs, "kbase's outcome is "+res.Outcome)
	}
	for _, line := range res.Checks {
		if strings.HasPrefix(line, StatusFail+" ") {
			diffs = append(diffs, line)
		}
	}
	return newCheck("build", evidence, diffs), res.Outcome != "bounded" && res.Outcome != "failed"
}

// compareTree: identical path set, and identical child order per document.
func compareTree(ref, kbase *dumpRoot) []string {
	if ref.Tree == nil || kbase.Tree == nil {
		return []string{"a tree is missing from the dump"}
	}
	children := func(r *dumpRoot) map[string][]string {
		m := map[string][]string{}
		for _, e := range r.Tree {
			m[e.Path] = e.Children
		}
		return m
	}
	rc, kc := children(ref), children(kbase)
	var diffs []string
	for _, p := range sortedUnion(rc, kc) {
		r, inRef := rc[p]
		k, inKbase := kc[p]
		switch {
		case !inKbase:
			diffs = append(diffs, "only in ref: "+p)
		case !inRef:
			diffs = append(diffs, "only in kbase: "+p)
		case !slices.Equal(r, k):
			diffs = append(diffs, fmt.Sprintf("%s: children ref %q, kbase %q", p, r, k))
		}
	}
	return diffs
}

// compareTokens: every document's token stream equal; each difference names
// the document and the first token offset where the streams part.
func compareTokens(ref, kbase *dumpRoot) []string {
	var diffs []string
	for _, p := range sortedUnion(ref.Tokens, kbase.Tokens) {
		r, inRef := ref.Tokens[p]
		k, inKbase := kbase.Tokens[p]
		if !inRef || !inKbase || slices.Equal(r, k) {
			continue
		}
		i := 0
		for i < len(r) && i < len(k) && r[i] == k[i] {
			i++
		}
		diffs = append(diffs, fmt.Sprintf("%s: token offset %d: ref %q, kbase %q (ref %d tokens, kbase %d)",
			p, i, window(r, i), window(k, i), len(r), len(k)))
	}
	return diffs
}

func window(tokens []string, i int) []string {
	return tokens[min(i, len(tokens)):min(i+6, len(tokens))]
}

// formFields are the dump's per-document form counts, in report order.
var formFields = []string{"uplink_lines", "anchors", "citation_spans", "math_fences", "labelled_blocks"}

// compareForms: conformance passes on kbase's tree, and every document's form
// counts equal, labelled blocks with an identifier included.
func compareForms(ref, kbase *dumpRoot) []string {
	var diffs []string
	switch {
	case kbase.Conformance == nil:
		diffs = append(diffs, "kbase conformance: no verdict (see the dump's errors)")
	case !kbase.Conformance.Passed:
		diffs = append(diffs, fmt.Sprintf("kbase conformance failed: %s: %s", kbase.Conformance.Failure.Check, kbase.Conformance.Failure.Detail))
	}
	refIDs, kbaseIDs := identifiedBlocks(ref), identifiedBlocks(kbase)
	for _, p := range sortedUnion(ref.Forms, kbase.Forms) {
		for _, f := range formFields {
			if r, k := ref.Forms[p][f], kbase.Forms[p][f]; r != k {
				diffs = append(diffs, fmt.Sprintf("%s: %s ref %d, kbase %d", p, f, r, k))
			}
		}
		if r, k := refIDs[p], kbaseIDs[p]; r != k {
			diffs = append(diffs, fmt.Sprintf("%s: labelled blocks with an identifier ref %d, kbase %d", p, r, k))
		}
	}
	return diffs
}

func identifiedBlocks(r *dumpRoot) map[string]int {
	m := map[string]int{}
	for _, b := range r.Inventory["blocks"] {
		if b["identifier"] != nil {
			m[str(b["document"])]++
		}
	}
	return m
}

// inventoryLists are the inventory's readings, in report order.
var inventoryLists = []string{"blocks", "fences", "anchors", "citations", "works", "proofs"}

// positional are the line-position fields the dump's README excludes from
// item equality: line placement is presentation.
var positional = map[string][]string{
	"blocks":    {"start", "end"},
	"fences":    {"start", "end"},
	"anchors":   {"line"},
	"citations": {"line"},
	"proofs":    {"start", "end"},
}

// withoutPositions is an inventory item with its positional fields, and its
// subjects' positional fields, removed, as canonical JSON.
func withoutPositions(list string, item map[string]any) string {
	clean := map[string]any{}
	for k, v := range item {
		if !slices.Contains(positional[list], k) {
			clean[k] = v
		}
	}
	if subjects, ok := clean["subjects"].([]any); ok {
		stripped := make([]any, len(subjects))
		for i, s := range subjects {
			sm, _ := s.(map[string]any)
			var b map[string]any
			if err := json.Unmarshal([]byte(withoutPositions("blocks", sm)), &b); err == nil {
				stripped[i] = b
			}
		}
		clean["subjects"] = stripped
	}
	data, err := json.Marshal(clean)
	if err != nil {
		return fmt.Sprintf("<unencodable: %v>", err)
	}
	return string(data)
}

// compareInventory: kb_tools' inventory over kbase's tree equals it over its
// own, item by item in scan order, positional fields excluded.
func compareInventory(ref, kbase *dumpRoot) []string {
	if ref.Inventory == nil || kbase.Inventory == nil {
		return []string{"an inventory is missing from the dump"}
	}
	var diffs []string
	for _, list := range inventoryLists {
		r, k := ref.Inventory[list], kbase.Inventory[list]
		if len(r) != len(k) {
			diffs = append(diffs, fmt.Sprintf("%s: ref %d items, kbase %d", list, len(r), len(k)))
		}
		for i := range min(len(r), len(k)) {
			if a, b := withoutPositions(list, r[i]), withoutPositions(list, k[i]); a != b {
				diffs = append(diffs, fmt.Sprintf("%s[%d]: ref %s, kbase %s", list, i, a, b))
			}
		}
	}
	return diffs
}

// compareDirs: the same files under both, byte for byte.
func compareDirs(a, b string) []string {
	files := func(root string) (map[string][]byte, error) {
		m := map[string][]byte{}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			m[rel], err = os.ReadFile(p)
			return err
		})
		return m, err
	}
	fa, err := files(a)
	if err != nil {
		return []string{err.Error()}
	}
	fb, err := files(b)
	if err != nil {
		return []string{err.Error()}
	}
	var diffs []string
	for _, p := range sortedUnion(fa, fb) {
		x, inA := fa[p]
		y, inB := fb[p]
		switch {
		case !inA || !inB:
			diffs = append(diffs, fmt.Sprintf("%s present under only one of %s, %s", p, a, b))
		case !bytes.Equal(x, y):
			diffs = append(diffs, fmt.Sprintf("%s differs between %s and %s", p, a, b))
		}
	}
	return diffs
}

func sortedUnion[V any, W any](a map[string]V, b map[string]W) []string {
	var keys []string
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// --- Slice-2: negative controls and citation states --------------------------------

type buildResult struct {
	Outcome string `yaml:"outcome"`
	Items   []struct {
		Detail string `yaml:"detail"`
	} `yaml:"refusals"`
	Stages []struct {
		Stage  string `yaml:"stage"`
		Report string `yaml:"report"`
	} `yaml:"stages"`
	// Refusals and Checks are read off Items and the document-graph stage's
	// report.
	Refusals []string `yaml:"-"`
	Checks   []string `yaml:"-"`
}

// readBuildResult is a build's result document at path, its refusals' details
// and the document graph's own check lines read in.
func readBuildResult(path string) (buildResult, error) {
	var res buildResult
	data, err := os.ReadFile(path)
	if err == nil {
		err = yaml.Unmarshal(data, &res)
	}
	if err != nil {
		return res, err
	}
	for _, it := range res.Items {
		res.Refusals = append(res.Refusals, it.Detail)
	}
	for _, s := range res.Stages {
		if s.Stage != "document-graph" {
			continue
		}
		var report struct {
			Rows []struct {
				Checks []string `yaml:"checks"`
			} `yaml:"rows"`
		}
		data, err := os.ReadFile(s.Report)
		if err == nil {
			err = yaml.Unmarshal(data, &report)
		}
		if err != nil {
			return res, err
		}
		for _, r := range report.Rows {
			res.Checks = append(res.Checks, r.Checks...)
		}
	}
	return res, nil
}

func TestSliceControls(t *testing.T) {
	if *sliceOut == "" || (*sliceUnparseable == "" && *sliceUnloadable == "") {
		t.Skip("no negative controls named; test-integration-slice-arxiv supplies them")
	}
	for id, want := range map[string]func(root string, refusals []string) []string{
		*sliceUnparseable: unparseableRefusal,
		*sliceUnloadable:  unloadableRefusal,
	} {
		if id == "" {
			continue
		}
		t.Run(id, func(t *testing.T) {
			dir := filepath.Join(*sliceOut, id)
			c := paperComparison{Paper: id}
			resultPath, exitPath := filepath.Join(dir, "run-1.result.yaml"), filepath.Join(dir, "run-1.exit")
			diffs := controlRefusal(dir, resultPath, exitPath, want)
			c.Checks = []checkResult{newCheck("refusal", []string{resultPath, exitPath}, diffs)}
			c.Passed = c.Checks[0].Passed
			if err := writeYAML(filepath.Join(dir, comparisonFile), c); err != nil {
				t.Fatal(err)
			}
			if !c.Passed {
				t.Errorf("%s did not refuse as specified — see %s", id, filepath.Join(dir, comparisonFile))
			}
		})
	}
}

func controlRefusal(dir, resultPath, exitPath string, want func(string, []string) []string) []string {
	root, err := os.ReadFile(filepath.Join(dir, volumeRootFile))
	if err != nil {
		return []string{err.Error()}
	}
	exit, err := os.ReadFile(exitPath)
	if err != nil {
		return []string{err.Error()}
	}
	res, err := readBuildResult(resultPath)
	if err != nil {
		return []string{err.Error()}
	}
	var diffs []string
	if code := strings.TrimSpace(string(exit)); code != "1" {
		diffs = append(diffs, "exit "+code+", want 1")
	}
	if res.Outcome != "refused" {
		diffs = append(diffs, "outcome "+res.Outcome+", want refused")
	}
	return append(diffs, want(strings.TrimSpace(string(root)), res.Refusals)...)
}

// unparseableRefusal: the refusal names the paper and carries the reader's
// own error.
func unparseableRefusal(root string, refusals []string) []string {
	for _, r := range refusals {
		if strings.Contains(r, root) && strings.Contains(r, "unparseable source: ") && !strings.HasSuffix(r, "unparseable source: ") {
			return nil
		}
	}
	return []string{fmt.Sprintf("no refusal names %s and the reader's error: %q", root, refusals)}
}

var namedAtLine = regexp.MustCompile(`could not load include \S.*, named at (?:\S+ )?line \d+`)

// unloadableRefusal: one refusal per unloaded include, each naming the file
// and the line that named it.
func unloadableRefusal(root string, refusals []string) []string {
	if len(refusals) == 0 {
		return []string{"no refusal items"}
	}
	var diffs []string
	for _, r := range refusals {
		if !strings.Contains(r, root) || !namedAtLine.MatchString(r) {
			diffs = append(diffs, fmt.Sprintf("refusal does not name the paper, the file and its line: %q", r))
		}
	}
	return diffs
}

// citationStates counts a root's citation items by state.
func citationStates(r *dumpRoot) map[string]int {
	counts := map[string]int{}
	if r == nil {
		return counts
	}
	for _, c := range r.Inventory["citations"] {
		counts[str(c["state"])]++
	}
	return counts
}

// TestSliceCitationStates: every citation state kb_tools' trees exhibit over
// the pooled papers also occurs in kbase's.
func TestSliceCitationStates(t *testing.T) {
	out, dirs := sliceArgs(t, sliceStates)
	type side struct {
		Papers map[string]map[string]int `yaml:"papers"`
		Totals map[string]int            `yaml:"totals"`
	}
	type report struct {
		Passed  bool     `yaml:"passed"`
		Missing []string `yaml:"missing_from_kbase,omitempty"`
		Kbase   side     `yaml:"kbase"`
		Ref     side     `yaml:"ref"`
		Errors  []string `yaml:"errors,omitempty"`
	}
	newSide := func() side { return side{Papers: map[string]map[string]int{}, Totals: map[string]int{}} }
	rep := report{Kbase: newSide(), Ref: newSide()}
	for _, dir := range dirs {
		id := filepath.Base(dir)
		kbase, ref, err := readDump(filepath.Join(dir, dumpFile), id)
		if err != nil {
			rep.Errors = append(rep.Errors, err.Error())
			continue
		}
		if ref == nil {
			rep.Errors = append(rep.Errors, "no ref root for "+id)
		}
		for _, x := range []struct {
			s    *side
			root *dumpRoot
		}{{&rep.Kbase, kbase}, {&rep.Ref, ref}} {
			counts := citationStates(x.root)
			x.s.Papers[id] = counts
			for state, n := range counts {
				x.s.Totals[state] += n
			}
		}
	}
	for _, state := range []string{records.StateResolved, records.StateUnanswered, records.StateKeyOnly} {
		if rep.Ref.Totals[state] > 0 && rep.Kbase.Totals[state] == 0 {
			rep.Missing = append(rep.Missing, state)
		}
	}
	rep.Passed = len(rep.Errors) == 0 && len(rep.Missing) == 0
	path := filepath.Join(out, statesFile)
	if err := writeYAML(path, rep); err != nil {
		t.Fatal(err)
	}
	if !rep.Passed {
		t.Errorf("citation states: a state kb_tools' trees exhibit is missing from kbase's — see %s", path)
	}
}

// --- Slice-3: the census -------------------------------------------------------------

// apiEnumerations are pandoc-api 1.23's enumeration constructors and metadata
// wrappers: they carry a "t" in the JSON but are no document node.
var apiEnumerations = map[string]bool{
	"InlineMath": true, "DisplayMath": true, "SingleQuote": true, "DoubleQuote": true,
	"AuthorInText": true, "SuppressAuthor": true, "NormalCitation": true,
	"DefaultStyle": true, "Example": true, "Decimal": true, "LowerRoman": true, "UpperRoman": true,
	"LowerAlpha": true, "UpperAlpha": true, "DefaultDelim": true, "Period": true, "OneParen": true,
	"TwoParens": true, "AlignLeft": true, "AlignRight": true, "AlignCenter": true, "AlignDefault": true,
	"ColWidth": true, "ColWidthDefault": true, "MetaMap": true, "MetaList": true, "MetaBool": true,
	"MetaString": true, "MetaInlines": true, "MetaBlocks": true,
}

var (
	declarationRe = regexp.MustCompile(`\\(?:newtheorem|(?:re)?newenvironment)\b`)
	numberingRe   = map[string]*regexp.Regexp{
		"numberwithin":           regexp.MustCompile(`\\numberwithin\b`),
		"counterwithin":          regexp.MustCompile(`\\counterwithin\b`),
		"newtheorem_subordinate": regexp.MustCompile(`\\newtheorem\s*\*?\s*\{[^}]*\}\s*\{[^}]*\}\s*\[`),
	}
)

type leafSize struct {
	Path   string `yaml:"path"`
	Tokens int    `yaml:"tokens"`
}

type census struct {
	Paper                     string           `yaml:"paper"`
	Outcome                   string           `yaml:"outcome"`
	Refusals                  []string         `yaml:"refusals,omitempty"`
	ReadError                 string           `yaml:"read_error,omitempty"`
	UnnamedNodeTypes          map[string]int   `yaml:"unnamed_node_types"`
	Prepass                   []prepass.Census `yaml:"prepass"`
	DeclarationsOutsideVolume map[string]int   `yaml:"declarations_outside_volume_root"`
	NumberingDeclarations     map[string]int   `yaml:"numbering_declarations"`
	MathBytes                 int              `yaml:"math_bytes"`
	SourceBytes               int              `yaml:"source_bytes"`
	CitationStates            map[string]int   `yaml:"citation_states,omitempty"`
	LeafTokens                map[string]int   `yaml:"leaf_tokens_summary,omitempty"`
	Leaves                    []leafSize       `yaml:"leaf_tokens,omitempty"`
	DumpError                 string           `yaml:"dump_error,omitempty"`
}

// TestSliceCensus measures each paper, kbase side only, with no pass
// threshold: it fails only where it cannot take a measurement.
func TestSliceCensus(t *testing.T) {
	out, ids := sliceArgs(t, sliceCensus)
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			dir := filepath.Join(out, id)
			c, err := takeCensus(dir, id)
			if err != nil {
				t.Fatal(err)
			}
			if err := writeYAML(filepath.Join(dir, censusFile), c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func takeCensus(dir, id string) (census, error) {
	c := census{Paper: id, UnnamedNodeTypes: map[string]int{}, DeclarationsOutsideVolume: map[string]int{}, NumberingDeclarations: map[string]int{}}
	fixture := filepath.Join(dir, "run-1")
	rootRel, err := os.ReadFile(filepath.Join(dir, volumeRootFile))
	if err != nil {
		return c, err
	}
	root := filepath.Join(fixture, strings.TrimSpace(string(rootRel)))
	resultPath := filepath.Join(dir, "run-1.result.yaml")
	var res buildResult
	if _, err := os.Stat(resultPath); err == nil {
		if res, err = readBuildResult(resultPath); err != nil {
			return c, err
		}
	}
	c.Outcome, c.Refusals = res.Outcome, res.Refusals

	source, err := os.ReadFile(root)
	if err != nil {
		return c, err
	}
	pre := prepass.Apply(string(source))
	c.Prepass = pre.Censuses
	for name := range numberingRe {
		c.NumberingDeclarations[name] = 0
	}
	err = filepath.WalkDir(fixture, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "kb-root" || d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".tex" {
			return nil
		}
		text, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		c.SourceBytes += len(text)
		for name, re := range numberingRe {
			c.NumberingDeclarations[name] += len(re.FindAllIndex(text, -1))
		}
		if p != root {
			if n := len(declarationRe.FindAllIndex(text, -1)); n > 0 {
				rel, err := filepath.Rel(fixture, p)
				if err != nil {
					return err
				}
				c.DeclarationsOutsideVolume[filepath.ToSlash(rel)] = n
			}
		}
		return nil
	})
	if err != nil {
		return c, err
	}

	var bibs []string
	if data, err := os.ReadFile(filepath.Join(dir, bibliographyTxt)); err == nil {
		for _, b := range strings.Fields(string(data)) {
			bibs = append(bibs, filepath.Join(fixture, b))
		}
	}
	ast, _, err := read(context.Background(), log.Discard(), filepath.Dir(root), pre.Source, bibs)
	if err != nil {
		c.ReadError = err.Error()
	} else if doc, err := decodeAST(ast); err != nil {
		c.ReadError = err.Error()
	} else {
		countNodes(doc, c.UnnamedNodeTypes, &c.MathBytes)
	}

	kbase, _, err := readDump(filepath.Join(dir, dumpFile), id)
	if err != nil {
		c.DumpError = err.Error()
		return c, nil
	}
	c.CitationStates = citationStates(kbase)
	for _, e := range kbase.Tree {
		if len(e.Children) == 0 && e.Path != kb.EntryPointFile {
			c.Leaves = append(c.Leaves, leafSize{Path: e.Path, Tokens: len(kbase.Tokens[e.Path])})
		}
	}
	c.LeafTokens = summarize(c.Leaves)
	return c, nil
}

// countNodes counts the node types the code does not handle, and the bytes of
// every Math node's source text.
func countNodes(v any, unnamed map[string]int, mathBytes *int) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			countNodes(e, unnamed, mathBytes)
		}
	case node:
		if t, ok := x["t"].(string); ok {
			if !handledNodeTypes[t] && !apiEnumerations[t] {
				unnamed[t]++
			}
			if t == "Math" {
				*mathBytes += len(str(list(x["c"])[1]))
			}
		}
		for _, k := range sortedKeys(x) {
			countNodes(x[k], unnamed, mathBytes)
		}
	}
}

func summarize(leaves []leafSize) map[string]int {
	if len(leaves) == 0 {
		return nil
	}
	sizes := make([]int, len(leaves))
	total := 0
	for i, l := range leaves {
		sizes[i] = l.Tokens
		total += l.Tokens
	}
	slices.Sort(sizes)
	return map[string]int{"leaves": len(sizes), "min": sizes[0], "median": sizes[len(sizes)/2], "max": sizes[len(sizes)-1], "total": total}
}

// --- the roll-up ---------------------------------------------------------------------

// buildCheckRe reads one line of a build's checks: status, check, detail.
var buildCheckRe = regexp.MustCompile(`^(PASS|FAIL|FACT) (\S+) `)

type rollupCheck struct {
	Passed int      `yaml:"passed"`
	Failed []string `yaml:"failed,omitempty"`
}

type rollupBuildCheck struct {
	Pass int      `yaml:"pass"`
	Fail []string `yaml:"fail,omitempty"`
	Fact []string `yaml:"fact,omitempty"`
}

type rollup struct {
	Papers int `yaml:"papers"`
	// Comparisons are each comparison check: the papers passing it, and the
	// papers failing it.
	Comparisons map[string]*rollupCheck `yaml:"comparisons"`
	// BuildChecks are each of run-1's own checks: the papers reporting it
	// passed, and those reporting a failure or a fact.
	BuildChecks map[string]*rollupBuildCheck `yaml:"build_checks"`
	Outcomes    map[string][]string          `yaml:"outcomes"`
	Errors      []string                     `yaml:"errors,omitempty"`
}

// TestRollup tallies every paper's comparison and run-1's own checks; it
// fails only where an input it needs is unreadable.
func TestRollup(t *testing.T) {
	out, ids := sliceArgs(t, sliceRollup)
	r := rollup{Papers: len(ids), Comparisons: map[string]*rollupCheck{}, BuildChecks: map[string]*rollupBuildCheck{}, Outcomes: map[string][]string{}}
	for _, id := range ids {
		dir := filepath.Join(out, id)
		var c paperComparison
		data, err := os.ReadFile(filepath.Join(dir, comparisonFile))
		if err == nil {
			err = yaml.Unmarshal(data, &c)
		}
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
		// The Records check is the claim graph's, written beside the comparison.
		if data, err := os.ReadFile(filepath.Join(dir, recordsCheckFile)); err == nil {
			var recordChecks []checkResult
			if err := yaml.Unmarshal(data, &recordChecks); err != nil {
				r.Errors = append(r.Errors, err.Error())
			}
			c.Checks = append(c.Checks, recordChecks...)
		}
		for _, ch := range c.Checks {
			rc := r.Comparisons[ch.Check]
			if rc == nil {
				rc = &rollupCheck{}
				r.Comparisons[ch.Check] = rc
			}
			if ch.Passed {
				rc.Passed++
			} else {
				rc.Failed = append(rc.Failed, id)
			}
		}
		res, err := readBuildResult(filepath.Join(dir, "run-1.result.yaml"))
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
			continue
		}
		r.Outcomes[res.Outcome] = append(r.Outcomes[res.Outcome], id)
		seen := map[string]map[string]bool{}
		for _, line := range res.Checks {
			m := buildCheckRe.FindStringSubmatch(line)
			if m == nil {
				r.Errors = append(r.Errors, fmt.Sprintf("%s: unreadable check line %q", id, line))
				continue
			}
			if seen[m[2]] == nil {
				seen[m[2]] = map[string]bool{}
			}
			seen[m[2]][m[1]] = true
		}
		for name, statuses := range seen {
			bc := r.BuildChecks[name]
			if bc == nil {
				bc = &rollupBuildCheck{}
				r.BuildChecks[name] = bc
			}
			if statuses[StatusFail] {
				bc.Fail = append(bc.Fail, id)
			} else if statuses[StatusPass] {
				bc.Pass++
			}
			if statuses[StatusFact] {
				bc.Fact = append(bc.Fact, id)
			}
		}
	}
	if err := writeYAML(filepath.Join(out, rollupFile), r); err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) > 0 {
		t.Errorf("the roll-up could not read every input — see %s", filepath.Join(out, rollupFile))
	}
}
