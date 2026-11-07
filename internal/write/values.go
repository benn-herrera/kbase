package write

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// entryKey is the top-level key of a batch: a list of entries, each
// one edit or one print, the op fixing what an entry means.
const entryKey = "entry"

// dryRunKey is the batch option beside entryKey that writes nothing.
const dryRunKey = "dry-run"

// commitKey is the batch option beside entryKey that makes git the batch's
// undo.
const commitKey = "commit"

// operationKey names an entry's op in a Batch.
const operationKey = "operation"

var (
	readerNumberRE = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	blankLineRE    = regexp.MustCompile(`\n[` + strings.Replace(kb.PyWhitespace, `\n`, "", 1) + `]*\n`)
)

// valueError is a checker's refusal, positioned at the node it failed on.
type valueError struct {
	field   string
	detail  string
	node    *yaml.Node
	line    int
	allowed []string
	remedy  string
}

func refuse(field string, node *yaml.Node, format string, args ...any) *valueError {
	return &valueError{field: field, node: node, detail: fmt.Sprintf(format, args...)}
}

// check reads one value node into its typed form.
type check func(n *yaml.Node, field string) (any, *valueError)

// field is one key a values entry or table may carry. typ and about are the
// key's type and description as its op's help lists them: a caller-issued op's
// keys carry both, a build-only op's and a table's neither.
type field struct {
	name       string
	check      check
	required   bool
	typ, about string
}

func fieldNames(fields []field) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.name
	}
	slices.Sort(names)
	return names
}

// resolved follows an alias to the node it names.
func resolved(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func typename(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "a table"
	case yaml.SequenceNode:
		return "an array"
	}
	switch n.ShortTag() {
	case "!!bool":
		return "a boolean"
	case "!!int":
		return "an integer"
	case "!!float":
		return "a float"
	case "!!str":
		return "a string"
	case "!!null":
		return "a null"
	}
	return "a " + strings.TrimPrefix(n.ShortTag(), "!!")
}

func isString(n *yaml.Node) bool { return n.Kind == yaml.ScalarNode && n.ShortTag() == "!!str" }

func requireString(n *yaml.Node, field, expected string) (string, *valueError) {
	if !isString(n) {
		return "", refuse(field, n, "expected %s, got %s", expected, typename(n))
	}
	return n.Value, nil
}

// requireNumber accepts an integer or a float the reader can read back. A
// value whose shortest form is exponent notation, nan or inf is not in the
// readers' number grammar, and would read back as another number or not at
// all, so it is refused before any domain check.
func requireNumber(n *yaml.Node, field, domain string) (float64, *valueError) {
	tag := n.ShortTag()
	if n.Kind != yaml.ScalarNode || (tag != "!!int" && tag != "!!float") {
		return 0, refuse(field, n, "expected a number in %s, got %s", domain, typename(n))
	}
	var number float64
	if err := n.Decode(&number); err != nil {
		return 0, refuse(field, n, "expected a number in %s, got %q", domain, n.Value)
	}
	if token := kb.PyFloatRepr(number); !readerNumberRE.MatchString(token) {
		return 0, refuse(field, n, "is written to disk as %q, which is not a number in the grammar every reader of "+
			"these files uses (-?\\d+(?:\\.\\d+)?) — exponent notation, nan and inf are all outside it, and a value "+
			"spelled that way is read back as a different number or dropped entirely. Supply it as a plain decimal", token)
	}
	return number, nil
}

func requireSequence(n *yaml.Node, field, of string) ([]*yaml.Node, *valueError) {
	if n.Kind != yaml.SequenceNode {
		return nil, refuse(field, n, "expected an array of %s, got %s", of, typename(n))
	}
	if len(n.Content) == 0 {
		return nil, refuse(field, n, "is an empty array; omit the key rather than supplying it empty")
	}
	return n.Content, nil
}

// checkProse is a single-paragraph prose value, returned as supplied. A blank
// line is refused rather than collapsed: the reader's fold stops at one and
// drops the rest. U+0008 and U+000C are refused because they are what a
// JSON writer's \b and \f make of a TeX \beta or \frac.
func checkProse(n *yaml.Node, field string) (any, *valueError) {
	text, err := requireString(n, field, "prose")
	if err != nil {
		return nil, err
	}
	if i := strings.IndexAny(text, "\b\f"); i >= 0 {
		r, _ := utf8.DecodeRuneInString(text[i:])
		return nil, refuse(field, n, "carries the control character U+%04X — a JSON \\%s escape where a TeX "+
			"backslash was meant (\\beta, \\frac). Escape the backslash, or supply the value as YAML",
			r, map[rune]string{'\b': "b", '\f': "f"}[r])
	}
	if kb.Strip(text) == "" {
		return nil, refuse(field, n, "expected prose, got an empty value")
	}
	lead := len(text) - len(kb.LStrip(text))
	body := kb.Strip(text)
	if m := blankLineRE.FindStringIndex(body); m != nil {
		offset := strings.Count(text[:lead], "\n") + strings.Count(body[:m[0]], "\n") + 1
		e := refuse(field, n, "carries a blank line. This field is single-paragraph by the reader's grammar — the fold "+
			"stops at a paragraph break and discards everything after it — so the value is refused here rather "+
			"than truncated on disk or silently joined. Rewrite it as one paragraph")
		e.line = proseLine(n, offset)
		return nil, e
	}
	return text, nil
}

// checkLine is a batch's prose value: one line, read collapsed — every
// whitespace run one space, the ends trimmed. A line break (CR or LF) is
// refused, and so are U+0008 and U+000C, a JSON writer's \b and \f where a TeX
// \beta or \frac was meant. Where allowEmpty is false a value that collapses to
// "" is refused.
func checkLine(allowEmpty bool) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		text, err := lineAsGiven(n, field)
		if err != nil {
			return nil, err
		}
		line := kb.NormalizeSpace(text)
		if line == "" && !allowEmpty {
			return nil, refuse(field, n, "expected one line of prose, got an empty value")
		}
		return line, nil
	}
}

// checkVerbatimLine is a one-line value used as written: a span matched
// against the document, never collapsed.
func checkVerbatimLine(n *yaml.Node, field string) (any, *valueError) {
	text, err := lineAsGiven(n, field)
	if err != nil {
		return nil, err
	}
	if kb.NormalizeSpace(text) == "" {
		return nil, refuse(field, n, "expected one line of prose, got an empty value")
	}
	return text, nil
}

// lineAsGiven is a prose value's text as given, refused where it is not a
// string or carries a line break, U+0008 or U+000C.
func lineAsGiven(n *yaml.Node, field string) (string, *valueError) {
	text, err := requireString(n, field, "one line of prose")
	if err != nil {
		return "", err
	}
	if i := strings.IndexAny(text, "\r\n\b\f"); i >= 0 {
		r, _ := utf8.DecodeRuneInString(text[i:])
		return "", refuse(field, n, "carries U+%04X; a prose value is one line, with no line break, U+0008 or U+000C "+
			"(a JSON \\b or \\f where a TeX backslash was meant: escape the backslash). Join the lines with a space", r)
	}
	return text, nil
}

// proseLine is the line of a prose value's offset-th line: a block scalar's
// text opens on the line after its indicator.
func proseLine(n *yaml.Node, offset int) int {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return n.Line + 1 + offset
	}
	return n.Line
}

// domainTop is a domain's upper bound as a refusal writes it: the reader's own
// domain is [0, 1] in every message, not the "1.0" PyFloatRepr keeps for a
// stored value.
func domainTop(ceiling float64) string {
	return strings.TrimSuffix(kb.PyFloatRepr(ceiling), ".0")
}

// checkScore is a number in [0, ceiling] or the pending literal, read as nil.
// The domain is the reader's: BearingFromValue is the one predicate deciding
// that a token is in [0, 1], and ceiling is the op's own bound inside it.
func checkScore(what string, ceiling float64) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		top := "[0, " + domainTop(ceiling) + "]"
		domain := top + " or the literal " + kb.PendingLiteral
		if isString(n) {
			if b, ok := kb.BearingFromValue(n.Value); ok && b.Pending {
				return (*float64)(nil), nil
			}
			return nil, refuse(field, n, "expected a number in %s, got %q", domain, n.Value)
		}
		number, err := requireNumber(n, field, domain)
		if err != nil {
			return nil, err
		}
		// The domain is judged on the token the file will carry: what a reader
		// reads back is the form the writer emits, not the one supplied.
		b, ok := kb.BearingFromValue(kb.PyFloatRepr(number))
		if !ok || b.Value > ceiling {
			return nil, refuse(field, n, "%s is outside %s — %s. Use a value in %s", kb.PyFloatRepr(number), top, what, domain)
		}
		return &b.Value, nil
	}
}

var (
	// checkRigor's ceiling is the rigor tables' maximum: the whole of the rigor
	// scale, so a value at it is an ordinary authored value. A bearing's ceiling
	// is the format's own top, which its table reserves nothing from either.
	checkRigor   = checkScore("the domain a node's own rigor carries", kb.RigorMaximum)
	checkBearing = checkScore("the domain one scoring edge's bearing carries", 1)
)

// checkPath is a kb-root-relative path; containment is the store's question.
func checkPath(n *yaml.Node, field string) (any, *valueError) {
	text, err := requireString(n, field, "a kb-root-relative path")
	if err != nil {
		return nil, err
	}
	if kb.Strip(text) == "" {
		return nil, refuse(field, n, "expected a kb-root-relative path, got an empty value")
	}
	if filepath.IsAbs(text) || strings.HasPrefix(text, "/") {
		return nil, refuse(field, n, "%q is absolute. Paths are relative to kb-root/, not to the repository root and not "+
			"to the filesystem — a leaf at <kb-root>/part3/results.md is 'part3/results.md'", text)
	}
	return text, nil
}

// checkPositive is a whole number from 1.
func checkPositive(n *yaml.Node, field string) (any, *valueError) {
	var v int
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" || n.Decode(&v) != nil || v < 1 {
		return nil, refuse(field, n, "expected a whole number from 1, got %s", strings.TrimSpace(typename(n)+" "+n.Value))
	}
	return v, nil
}

func checkOneOf(vocabulary []string, what string) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		text, err := requireString(n, field, "a string")
		if err != nil {
			err.allowed = vocabulary
			return nil, err
		}
		if !slices.Contains(vocabulary, text) {
			err := refuse(field, n, "%q is outside the closed %s vocabulary %v", text, what, vocabulary)
			err.allowed = vocabulary
			return nil, err
		}
		return text, nil
	}
}

var (
	checkStatus   = checkOneOf(kb.ExperimentStatuses, "status")
	checkRelation = checkOneOf(kb.ScoringRelations, "relation")
)

// authoredRelations is every relation an op writes an edge of, in the
// relations table's order: all but demoted, whose records only the build
// writes.
var authoredRelations = append(slices.Clone(kb.ScoringRelations), kb.RelationReferences)

// checkAuthoredRelation takes one of authoredRelations; demoted is refused
// with demotedRemedy, the ops that resolve a demoted record.
func checkAuthoredRelation(demotedRemedy string) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		v, err := checkOneOf(authoredRelations, "relation")(n, field)
		if err != nil && isString(n) && n.Value == kb.RelationDemoted {
			err.detail = "a demoted record is the build's, and this op writes no demoted record"
			err.remedy = demotedRemedy
		}
		return v, err
	}
}

// idChecker checks a node id's shape; existence is the op's question. The
// authored placeholder ids match the grammar and are still refused.
func idChecker(kinds ...string) check {
	re := regexp.MustCompile(`^` + kb.IDBody(kinds...) + `$`)
	var spelled []string
	for _, k := range kinds {
		spelled = append(spelled, kb.IDBody(k))
	}
	expected := strings.Join(spelled, " or ")
	return func(n *yaml.Node, field string) (any, *valueError) {
		token, err := requireString(n, field, "an id matching "+expected)
		if err != nil {
			return nil, err
		}
		if kb.IDPlaceholders[token] {
			return nil, refuse(field, n, "%q is the authored placeholder id, which is never a real node", token)
		}
		if !re.MatchString(token) {
			return nil, refuse(field, n, "%q is not an id matching %s", token, expected)
		}
		return token, nil
	}
}

var (
	checkClaimID      = idChecker(kb.ClaimPrefix)
	checkExperimentID = idChecker(kb.ExperimentPrefix)
	checkEntryID      = idChecker(kb.ClaimPrefix, kb.AnalysisPrefix)
	// checkHostedID is a node a statement marker hosts: a claim, an analysis
	// or an experiment.
	checkHostedID = idChecker(kb.ClaimPrefix, kb.AnalysisPrefix, kb.ExperimentPrefix)
	checkAxiomID  = idChecker(kb.AxiomPrefix)
	checkWorkID   = idChecker(kb.WorkPrefix)
)

// checkSourceID is a scoring edge's source: a claim, an analysis, an
// experiment, or the axiom or external work a founds edge draws from.
func checkSourceID(n *yaml.Node, field string) (any, *valueError) {
	switch {
	case isString(n) && strings.HasPrefix(n.Value, kb.AxiomPrefix+"-"):
		return checkAxiomID(n, field)
	case isString(n) && strings.HasPrefix(n.Value, kb.WorkPrefix+"-"):
		return checkWorkID(n, field)
	}
	return checkHostedID(n, field)
}

// checkCitationKey is a work's label: its citation key.
func checkCitationKey(n *yaml.Node, field string) (any, *valueError) {
	token, err := requireString(n, field, "a citation key")
	if err != nil {
		return nil, err
	}
	if !kb.WorkLabel.MatchString(token) {
		return nil, refuse(field, n, "%q is not a citation key: it must open and close alphanumeric, with only letters, digits and _.:+/- between", token)
	}
	return token, nil
}

// legacyInvariantPrefix opens the 1.x spelling of an axiom citation.
const legacyInvariantPrefix = "INVARIANT-"

// legacyAxiomPrefix opens the 3.0.0 spelling of an axiom citation.
const legacyAxiomPrefix = "Axiom "

// checkPremiseTarget is a premise's source by its id: a claim, an experiment,
// a work or an axiom. The superseded axiom spellings, "Axiom <label>" and
// INVARIANT-<label>, are refused: an axiom is named by its minted id.
func checkPremiseTarget(n *yaml.Node, field string) (any, *valueError) {
	token, err := requireString(n, field, "a claim, experiment, work or axiom id")
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(token, legacyAxiomPrefix) || strings.HasPrefix(token, legacyInvariantPrefix) {
		return nil, refuse(field, n, "%q is a superseded axiom spelling; name the axiom by its minted id, %s-<minted>: %s", token, kb.AxiomPrefix, kb.NodeLister)
	}
	switch {
	case strings.HasPrefix(token, kb.AxiomPrefix+"-"):
		return checkAxiomID(n, field)
	case strings.HasPrefix(token, kb.ExperimentPrefix+"-"):
		return checkExperimentID(n, field)
	case strings.HasPrefix(token, kb.WorkPrefix+"-"):
		return checkWorkID(n, field)
	}
	return checkClaimID(n, field)
}

// mappingPairs is a mapping node's key and value nodes, refusing a duplicate
// key: a second value for one key is a value one of the two writers loses.
func mappingPairs(n *yaml.Node, container string, allowed []string) ([][2]*yaml.Node, *valueError) {
	var pairs [][2]*yaml.Node
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], resolved(n.Content[i+1])
		if seen[k.Value] {
			e := refuse(dotted(container, keyName(k.Value)), k, "is given twice in one table")
			e.allowed = allowed
			return nil, e
		}
		seen[k.Value] = true
		pairs = append(pairs, [2]*yaml.Node{k, v})
	}
	return pairs, nil
}

// keyName is a batch's key as a refusal names it, an empty key
// spelled as YAML spells it so the item still names its key.
func keyName(key string) string {
	if key == "" {
		return `""`
	}
	return key
}

func dotted(container, key string) string {
	if container == "" {
		return key
	}
	return container + "." + key
}

// checkTable checks one nested table against a closed sub-vocabulary: an
// unknown or missing key is refused there exactly as at entry level.
func checkTable(n *yaml.Node, label string, specs []field) (map[string]any, *valueError) {
	allowed := fieldNames(specs)
	if n.Kind != yaml.MappingNode {
		e := refuse(label, n, "expected a table taking %v, got %s", allowed, typename(n))
		e.allowed = allowed
		return nil, e
	}
	pairs, err := mappingPairs(n, label, allowed)
	if err != nil {
		return nil, err
	}
	given := map[string]*yaml.Node{}
	for _, p := range pairs {
		if !slices.ContainsFunc(specs, func(f field) bool { return f.name == p[0].Value }) {
			e := refuse(dotted(label, p[0].Value), p[0], "unknown key; this table takes %v", allowed)
			e.allowed = allowed
			return nil, e
		}
		given[p[0].Value] = p[1]
	}
	out := map[string]any{}
	for _, spec := range specs {
		v, ok := given[spec.name]
		if !ok {
			if spec.required {
				e := refuse(dotted(label, spec.name), n, "required key is missing; nothing is defaulted")
				e.allowed = allowed
				return nil, e
			}
			continue
		}
		checked, err := spec.check(v, spec.name)
		if err != nil {
			err.field = dotted(label, err.field)
			return nil, err
		}
		out[spec.name] = checked
	}
	return out, nil
}

func tableList(specs []field, of string, build func(map[string]any, *yaml.Node, string) (any, *valueError)) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		items, err := requireSequence(n, field, of)
		if err != nil {
			return nil, err
		}
		var out []any
		for i, item := range items {
			label := fmt.Sprintf("%s[%d]", field, i+1)
			checked, err := checkTable(resolved(item), label, specs)
			if err != nil {
				return nil, err
			}
			v, err := build(checked, item, label)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
}

// restsOnValue is one requested premise: the id it names, the author's
// context, its bearing (nil is the pending judgement), and whether the caller
// supplied the bearing.
type restsOnValue struct {
	ID, Context string
	Bearing     *float64
	Judged      bool
}

var restsOnFields = []field{
	{"id", checkPremiseTarget, true, "", ""},
	{"context", checkProse, false, "", ""},
	{"bearing", checkBearing, false, "", ""},
}

var referencesFields = []field{
	{"id", checkClaimID, true, "", ""},
	{"context", checkProse, false, "", ""},
}

func optionalString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func optionalScore(m map[string]any, key string) *float64 {
	v, _ := m[key].(*float64)
	return v
}

func listOf(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

var checkRestsOn = tableList(restsOnFields, "{ id, context, bearing } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		_, judged := m["bearing"]
		return restsOnValue{ID: m["id"].(string), Context: optionalString(m, "context"), Bearing: optionalScore(m, "bearing"), Judged: judged}, nil
	})

var checkReferences = tableList(referencesFields, "{ id, context } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		return restsOnValue{ID: m["id"].(string), Context: optionalString(m, "context")}, nil
	})

// placementFields are the keys naming the line an add places its node's
// statement marker on.
var placementFields = []field{
	{"document", checkPath, true, "string", "the leaf, kb-root-relative, whose line takes the statement marker"},
	{"line", checkPositive, true, "integer", "that line's number, 1-based"},
	{"snippet", checkLine(false), true, "string", "a few distinctive words of that line, not the whole line: they only verify it"},
}

// editID is an edit's id: a node of any kind, so that one of another kind is
// refused when it applies, naming the edit op for its kind.
var editID = field{"id", checkSourceID, true, "string", ""}

// idAbout is editID described as about describes it, for one op's help.
func idAbout(about string) field {
	f := editID
	f.about = about
	return f
}

// kindEditID is the id key of the edit op for kind, whose ids open with
// prefix.
func kindEditID(kind, prefix string) field {
	return idAbout("the " + kind + "'s id (" + prefix + "-…); another kind's is refused, naming its edit op")
}

// scoreType is a rigor's or a bearing's type, as an op's help lists it.
const scoreType = `number or "*pending*"`

// The node ops' shared keys: each as an add takes it, then as an edit does.
var (
	addedTitle      = field{"title", checkLine(false), true, "string", "the node's title, one line, unique within its kind"}
	editedTitle     = field{"title", checkLine(false), false, "string", "the node's new title, one line, unique within its kind"}
	addedRationale  = field{"rationale", checkLine(true), true, "string", `the node's rationale, one line; "" for none`}
	editedRationale = field{"rationale", checkLine(true), false, "string", `the node's new rationale, one line; "" for none`}
	addedRigor      = field{"rigor", checkRigor, false, scoreType, `the node's rigor, in [0, 1] (default: "*pending*"); one given, "*pending*" included, marks it manually scored`}
	editedRigor     = field{"rigor", checkRigor, false, scoreType, "the node's new rigor, in [0, 1]; one given, the standing value included, marks it manually scored"}
)

// withTraceAbout is the rationale key f of a claim or an analysis, its
// description pointing a caller asking what would strengthen the node to
// trace. A work's rationale has no such pointer: no scoring edge ends at a
// work.
func withTraceAbout(f field) field {
	f.about += "; not what would strengthen the node: " +
		argmark.Call("trace", map[string]any{"id": "<id>", "direction": "in", "sort": "contribution", "order": "ascending"}) + " answers that"
	return f
}

// edgeSourceAbout and edgeTargetAbout describe the keys naming an edge's ends.
const (
	edgeSourceAbout = "the id of the edge's source node"
	edgeTargetAbout = "the id of the edge's target node"
)

// judgedRelationsAbout closes a bearing's description: the relations whose
// edge a given bearing marks manually scored.
var judgedRelationsAbout = "where its relation is one a score run judges (" + strings.Join(kb.BearingWalkRelations, ", ") + ")"

// opFields is every op's closed key vocabulary: an unknown key is refused,
// never ignored, and a required key left out is refused by name.
var opFields = map[string][]field{
	EditClaim:    {kindEditID(kb.NodeKindClaim, kb.ClaimPrefix), editedTitle, editedRigor, withTraceAbout(editedRationale)},
	EditAnalysis: {kindEditID(kb.NodeKindAnalysis, kb.AnalysisPrefix), editedTitle, editedRigor, withTraceAbout(editedRationale)},
	EditExperiment: {kindEditID(kb.NodeKindExperiment, kb.ExperimentPrefix), editedTitle,
		{"status", checkStatus, false, "string", "run or planned: whether the experiment has been run"}, editedRigor},
	EditWork: {kindEditID(kb.NodeKindWork, kb.WorkPrefix),
		{"label", checkCitationKey, false, "string", "the work's new citation key: letters, digits and _ . : + / -, opening and closing with a letter or digit; " +
			"unique among works and unused as a key by any citation span or reference-list entry; every use of the old key as a key is rewritten to it"},
		editedTitle,
		{"rigor", checkRigor, false, scoreType, "the work's new rigor, in [0, 1]; a work's rigor carries no manual mark"}, editedRationale},
	EditAxiom: {kindEditID(kb.NodeKindAxiom, kb.AxiomPrefix),
		{"label", checkLine(false), false, "string", "the axiom's new label, one line of any text, unique among axioms; its heading in axioms.md is rewritten"},
		editedTitle},
	RemoveNode: {idAbout("the id of the node to remove, of any kind")},
	MoveNode:   append([]field{idAbout("the id of the claim, analysis or experiment whose statement marker moves")}, placementFields...),
	AddCitation: {{"document", checkPath, true, "string", "the KB document, kb-root-relative, whose line holds the text to link"},
		{"line", checkPositive, true, "integer", "that line's number, 1-based"},
		{"snippet", checkVerbatimLine, true, "string", "the span of that line the link replaces, occurring there once: " +
			"the node's id, a verbatim span of its statement, or your paraphrase (with \"quote\")"},
		{"id", checkSourceID, true, "string", "the cited node's id"},
		{"quote", checkLine(false), false, "string", "where the snippet is your paraphrase, the passage it paraphrases, verbatim from the cited statement; " +
			"left out, the snippet is the node's id (naming form) or a verbatim span of its statement"}},
	AddClaim:    append([]field{addedTitle, withTraceAbout(addedRationale), addedRigor}, placementFields...),
	AddAnalysis: append([]field{addedTitle, withTraceAbout(addedRationale), addedRigor}, placementFields...),
	AddExperiment: append([]field{addedTitle, {"status", checkStatus, true, "string", "run or planned: whether the experiment has been run"},
		addedRigor}, placementFields...),
	AddWork: {
		{"label", checkCitationKey, true, "string", "the work's citation key: letters, digits and _ . : + / -, opening and closing with a letter or digit; unique among works; the work's id is minted"},
		addedTitle,
		addedRationale,
		{"rigor", checkRigor, false, scoreType, `the work's rigor, in [0, 1] (default: "*pending*"); a work's rigor carries no manual mark`},
	},
	AddAxiom: {
		{"label", checkLine(false), true, "string", "the axiom's label, one line of any text, unique among axioms; the axiom's id is minted"},
		addedTitle,
	},
	AddBuildEdges: {
		{"id", checkEntryID, true, "", ""},
		{kb.ListRestsOn, checkRestsOn, false, "", ""},
		{"references", checkReferences, false, "", ""},
	},
	AddEdge: {
		{"source", checkSourceID, true, "string", edgeSourceAbout},
		{"target", checkSourceID, true, "string", edgeTargetAbout},
		{"relation", checkAuthoredRelation("restore-demoted puts the demoted edge back, then re-run"), true, "string",
			"one of " + strings.Join(authoredRelations, ", ") + "; the edge reads source relation target"},
		{"bearing", checkBearing, false, scoreType, `in [0, 1] (default: "*pending*"), refused on ` + kb.RelationReferences +
			`; one given, "*pending*" included, marks the edge manually scored ` + judgedRelationsAbout},
		{"context", checkLine(true), false, "string", `one line, "" for none`},
	},
	EditEdge: {
		{"source", checkSourceID, true, "string", edgeSourceAbout},
		{"target", checkSourceID, true, "string", edgeTargetAbout},
		{"relation", checkAuthoredRelation("restore-demoted puts the demoted edge back, or remove-edge removes the demoted record; then re-run"), true,
			"string", "the standing edge's relation, one of " + strings.Join(authoredRelations, ", ") + "; " + kb.RelationDemoted + " is refused"},
		{"bearing", checkBearing, false, scoreType, "the edge's new bearing, in [0, 1], refused on " + kb.RelationReferences +
			"; one given, the standing value included, marks the edge manually scored " + judgedRelationsAbout},
		{"context", checkLine(true), false, "string", `the edge's new context, one line, "" for none`},
	},
	RemoveEdge: {
		{"source", checkSourceID, true, "string", edgeSourceAbout},
		{"target", checkSourceID, true, "string", edgeTargetAbout},
		{"relation", checkOneOf(kb.Relations, "relation"), true, "string", "the edge record's relation, one of " + strings.Join(kb.Relations, ", ")},
	},
	RestoreDemoted: {
		{"source", checkHostedID, true, "string", "the id of the demoted record's source node"},
		{"target", checkHostedID, true, "string", "the id of the demoted record's target node"},
		{"relation", checkOneOf([]string{kb.RelationDemoted}, "relation"), true, "string", kb.RelationDemoted + ", and no other"},
	},
	DemoteClaimEdge: {
		{"source", checkHostedID, true, "", ""},
		{"target", checkHostedID, true, "", ""},
		{"relation", checkRelation, true, "", ""},
	},
	SplitClaim: {
		{"id", checkClaimID, true, "", ""},
		{"title", checkProse, true, "", ""},
		{"rationale", checkProse, true, "", ""},
		{"document", checkPath, true, "", ""},
		{"line", checkPositive, true, "", ""},
		{"statement-end", checkPositive, false, "", ""},
		{"statement-line", checkPositive, false, "", ""},
	},
	CiteBareIDs: {},
}

// entry is one accepted values entry: its 1-based position, its op, and its
// checked values, keyed by the op's vocabulary.
type entry struct {
	Index  int
	Op     string
	Values map[string]any
}

// batch is a batch as accepted: its entries, in order, and whether
// it is a dry run and whether it commits.
type batch struct {
	entries []entry
	dryRun  bool
	commit  bool
}

func (e entry) str(key string) string     { return optionalString(e.Values, key) }
func (e entry) score(key string) *float64 { return optionalScore(e.Values, key) }
func (e entry) has(key string) bool       { _, ok := e.Values[key]; return ok }

// names is the node ids the entry's values name: its id, source and target.
func (e entry) names() []string {
	var ids []string
	for _, key := range []string{"id", "source", "target"} {
		if id := e.str(key); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// count is the whole number checkPositive read at key, 0 where absent.
func (e entry) count(key string) int { n, _ := e.Values[key].(int); return n }

func (e entry) restsOn(key string) []restsOnValue {
	var out []restsOnValue
	for _, v := range listOf(e.Values, key) {
		out = append(out, v.(restsOnValue))
	}
	return out
}

// parseValues checks a batch — YAML, or JSON, which YAML reads —
// as op's: a batch of op's entries, or, where op is Batch, of entries each
// naming its op under operation. This is the batch's shape phase: it returns
// every entry, or every refusal and no entry, each op's own checks over its
// entries included. A dry run is reported wherever the batch states it.
func parseValues(data []byte, op string) (batch, []result.Item) {
	top := []string{commitKey, dryRunKey, entryKey}
	if op != Batch {
		if _, ok := opFields[op]; !ok {
			panic("write: no vocabulary for op " + op)
		}
	}
	if !utf8.Valid(data) {
		line := bytes.Count(data[:invalidUTF8At(data)], []byte("\n")) + 1
		return batch{}, []result.Item{{Key: entryKey, Line: line, Allowed: top,
			Detail: "is not valid UTF-8. A batch is read as strict UTF-8 and never decoded with replacement characters"}}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return batch{}, []result.Item{{Key: entryKey, Line: yamlErrorLine(err), Allowed: top,
			Detail: "is not parseable YAML or JSON: " + err.Error()}}
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return batch{}, []result.Item{{Key: entryKey, Line: 1, Allowed: top,
			Detail: "the input is empty; a batch is a table holding the entry list, which may be empty (entry: [])"}}
	}
	root := resolved(doc.Content[0])
	if root.Kind != yaml.MappingNode {
		return batch{}, []result.Item{{Key: entryKey, Line: root.Line, Column: root.Column, Allowed: top,
			Detail: fmt.Sprintf("expected a table holding the entry list, got %s", typename(root))}}
	}
	var refusals []result.Item
	pairs, verr := mappingPairs(root, "", top)
	if verr != nil {
		return batch{}, []result.Item{verr.located(0)}
	}
	var b batch
	var entries *yaml.Node
	for _, p := range pairs {
		switch {
		case p[0].Value == entryKey:
			entries = p[1]
		case p[0].Value == dryRunKey || p[0].Value == commitKey:
			option := &b.dryRun
			if p[0].Value == commitKey {
				option = &b.commit
			}
			if p[1].Kind != yaml.ScalarNode || p[1].ShortTag() != "!!bool" || p[1].Decode(option) != nil {
				refusals = append(refusals, result.Item{Key: p[0].Value, Line: p[1].Line, Column: p[1].Column,
					Detail: fmt.Sprintf("expected true or false, got %s", typename(p[1]))})
			}
		default:
			refusals = append(refusals, result.Item{Key: keyName(p[0].Value), Line: p[0].Line, Column: p[0].Column, Allowed: top,
				Detail: "is not a key of a batch; its top-level keys are " + strings.Join(top, ", ")})
		}
	}
	switch {
	case entries == nil:
		return b, append(refusals, result.Item{Key: entryKey, Line: root.Line, Column: root.Column, Allowed: top,
			Detail: "is missing; a batch is a table holding the entry list, which may be empty (entry: [])"})
	case entries.Kind != yaml.SequenceNode:
		return b, append(refusals, result.Item{Key: entryKey, Line: entries.Line, Column: entries.Column, Allowed: top,
			Detail: fmt.Sprintf("expected a list of tables, got %s", typename(entries))})
	}
	for i, raw := range entries.Content {
		e, entryRefusals := checkBatchEntry(resolved(raw), i+1, op)
		refusals = append(refusals, entryRefusals...)
		if len(entryRefusals) == 0 {
			b.entries = append(b.entries, e)
		}
	}
	refusals = append(refusals, batchChecks(b.entries)...)
	if len(refusals) > 0 {
		b.entries = nil
		return b, refusals
	}
	return b, nil
}

// checkBatchEntry checks one entry as op's, or, where op is Batch, as the op
// its operation key names.
func checkBatchEntry(n *yaml.Node, index int, op string) (entry, []result.Item) {
	if op != Batch {
		values, refusals := checkEntry(n, index, op, fieldNames(opFields[op]))
		return entry{Index: index, Op: op, Values: values}, refusals
	}
	ops := Ops()
	if n.Kind != yaml.MappingNode {
		return entry{}, []result.Item{{Key: entryKey, Entry: index, Line: n.Line, Column: n.Column, Allowed: []string{operationKey},
			Detail: fmt.Sprintf("expected a table naming its op under operation, got %s", typename(n))}}
	}
	rest := *n
	rest.Content = nil
	var named *yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == operationKey && named == nil {
			named = resolved(n.Content[i+1])
			continue
		}
		rest.Content = append(rest.Content, n.Content[i], n.Content[i+1])
	}
	if named == nil {
		return entry{}, []result.Item{{Key: operationKey, Entry: index, Line: n.Line, Column: n.Column, Allowed: ops,
			Detail: "is required and was not supplied: each entry of a mixed batch names its op"}}
	}
	name, verr := checkOneOf(ops, "write op")(named, operationKey)
	if verr != nil {
		return entry{}, []result.Item{verr.located(index)}
	}
	values, refusals := checkEntry(&rest, index, name.(string), append(fieldNames(opFields[name.(string)]), operationKey))
	return entry{Index: index, Op: name.(string), Values: values}, refusals
}

// batchChecks is each op's own checks over its entries in the batch, before
// any entry applies: every refusal at once.
func batchChecks(entries []entry) []result.Item {
	var ops []string
	byOp := map[string][]entry{}
	for _, e := range entries {
		if _, seen := byOp[e.Op]; !seen {
			ops = append(ops, e.Op)
		}
		byOp[e.Op] = append(byOp[e.Op], e)
	}
	var refusals []result.Item
	for _, op := range ops {
		spec, ok := writeOps[op]
		if !ok || spec.edit.prior == nil {
			continue
		}
		var refused result.Refusal
		if err := spec.edit.prior(byOp[op]); errors.As(err, &refused) {
			refusals = append(refusals, refused...)
		}
	}
	return refusals
}

// checkEntry checks one entry against op's fields, allowed being every key it
// may carry: unknown keys, then missing required keys and values, every
// refusal collected. An edit's unknown key allows the fields its kind may
// change.
func checkEntry(n *yaml.Node, index int, op string, allowed []string) (map[string]any, []result.Item) {
	fields := opFields[op]
	slices.Sort(allowed)
	unknownAllowed := allowed
	if isNodeEdit(op) {
		unknownAllowed = editFields(op)
	}
	if n.Kind != yaml.MappingNode {
		return nil, []result.Item{{Key: entryKey, Entry: index, Line: n.Line, Column: n.Column, Allowed: allowed,
			Detail: fmt.Sprintf("expected a table, got %s", typename(n))}}
	}
	pairs, verr := mappingPairs(n, "", allowed)
	if verr != nil {
		return nil, []result.Item{verr.located(index)}
	}
	var refusals []result.Item
	given := map[string]*yaml.Node{}
	for _, p := range pairs {
		if !slices.ContainsFunc(fields, func(f field) bool { return f.name == p[0].Value }) {
			refusals = append(refusals, result.Item{Key: keyName(p[0].Value), Entry: index, Line: p[0].Line, Column: p[0].Column, Allowed: unknownAllowed,
				Detail: "is not a key this op takes. The vocabulary is closed and total — an unrecognized key is refused, never ignored"})
			continue
		}
		given[p[0].Value] = p[1]
	}
	values := map[string]any{}
	for _, spec := range fields {
		v, ok := given[spec.name]
		if !ok {
			if spec.required {
				refusals = append(refusals, result.Item{Key: spec.name, Entry: index, Line: n.Line, Column: n.Column, Allowed: allowed,
					Detail: "is required and was not supplied. No value is defaulted or inferred"})
			}
			continue
		}
		checked, err := spec.check(v, spec.name)
		if err != nil {
			refusals = append(refusals, err.located(index))
			continue
		}
		values[spec.name] = checked
	}
	return values, refusals
}

func (e *valueError) located(index int) result.Item {
	r := result.Item{Key: e.field, Entry: index, Detail: e.detail, Allowed: e.allowed, Remedy: e.remedy}
	if e.node != nil {
		r.Line, r.Column = e.node.Line, e.node.Column
	}
	if e.line > 0 {
		r.Line, r.Column = e.line, 0
	}
	return r
}

var yamlLineRE = regexp.MustCompile(`line (\d+)`)

func yamlErrorLine(err error) int {
	if m := yamlLineRE.FindStringSubmatch(err.Error()); m != nil {
		var line int
		fmt.Sscan(m[1], &line)
		return line
	}
	return 0
}

func invalidUTF8At(data []byte) int {
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			return i
		}
		i += size
	}
	return len(data)
}
