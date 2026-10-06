package write

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// entryKey is the one top-level key of a values document: a list of entries,
// each one edit or one print, the op fixing what an entry means.
const entryKey = "entry"

// excerptMaxChars is the citation gate's bound on an excerpt.
const excerptMaxChars = 240

var (
	readerNumberRE = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	invariantRE    = regexp.MustCompile(`^INVARIANT-[A-Z]+[0-9]+$`)
	axiomTokenRE   = regexp.MustCompile(`^Axiom [0-9]+$`)
	citationKeyRE  = regexp.MustCompile(`^` + strings.TrimPrefix(kb.WorkIDPattern, kb.WorkPrefix+"-") + `$`)
	blankLineRE    = regexp.MustCompile(`\n[` + strings.Replace(kb.PyWhitespace, `\n`, "", 1) + `]*\n`)
)

var (
	experimentStatuses = []string{"run", "pending"}
)

// valueError is a checker's refusal, positioned at the node it failed on.
type valueError struct {
	field   string
	detail  string
	node    *yaml.Node
	line    int
	allowed []string
}

func refuse(field string, node *yaml.Node, format string, args ...any) *valueError {
	return &valueError{field: field, node: node, detail: fmt.Sprintf(format, args...)}
}

// check reads one value node into its typed form.
type check func(n *yaml.Node, field string) (any, *valueError)

type field struct {
	name     string
	check    check
	required bool
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

// proseLine is the line of a prose value's offset-th line: a block scalar's
// text opens on the line after its indicator.
func proseLine(n *yaml.Node, offset int) int {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return n.Line + 1 + offset
	}
	return n.Line
}

// checkScore is a number in [0, 1] or the pending literal, read as nil.
func checkScore(what string) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		domain := "[0, 1] or the literal " + kb.PendingLiteral
		if isString(n) {
			if n.Value == kb.PendingLiteral {
				return (*float64)(nil), nil
			}
			return nil, refuse(field, n, "expected a number in %s, got %q", domain, n.Value)
		}
		number, err := requireNumber(n, field, domain)
		if err != nil {
			return nil, err
		}
		if number < 0 || number > 1 {
			return nil, refuse(field, n, "%s is outside [0, 1] — %s. Use a value in %s", kb.PyFloatRepr(number), what, domain)
		}
		return &number, nil
	}
}

var (
	checkRigor = checkScore("the domain hand-authored confidence and quality values are verifier-enforced to")
	// checkFraction is an on-point fraction; zero says the support bears
	// nothing on the claim, a judgement distinct from pending.
	checkFraction      = checkScore("the share of the supporting work that bears on this claim")
	checkWorkStrength  = checkScore("the domain a cited work's standing carries, foundational at 1 and spurious at 0")
	checkApplicability = checkFraction
)

// checkStrength is a strengthens pair's strength: [0, 1], with no pending
// form, since the verifier rejects a null strength.
func checkStrength(n *yaml.Node, field string) (any, *valueError) {
	if isString(n) {
		return nil, refuse(field, n, "expected a number, got %q; a strengthens pair carries no pending form", n.Value)
	}
	number, err := requireNumber(n, field, "[0, 1]")
	if err != nil {
		return nil, err
	}
	if number < 0 || number > 1 {
		return nil, refuse(field, n, "%s is outside [0, 1], the domain a strengthens edge's strength carries", kb.PyFloatRepr(number))
	}
	return number, nil
}

func checkExcerpt(n *yaml.Node, field string) (any, *valueError) {
	v, err := checkProse(n, field)
	if err != nil {
		return nil, err
	}
	if size := utf8.RuneCountInString(kb.Strip(v.(string))); size > excerptMaxChars {
		return nil, refuse(field, n, "is %d characters, over the %d-character bound the citation checker enforces. "+
			"Quote the clause, not the section", size, excerptMaxChars)
	}
	return v, nil
}

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
			"to the filesystem — a register at <kb-root>/part3/claim-quality.md is 'part3/claim-quality.md'", text)
	}
	return text, nil
}

func checkOneOf(vocabulary []string, what string) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		text, err := requireString(n, field, "a string")
		if err != nil {
			return nil, err
		}
		if !slices.Contains(vocabulary, text) {
			return nil, refuse(field, n, "%q is outside the closed %s vocabulary %v", text, what, vocabulary)
		}
		return text, nil
	}
}

var (
	checkKind   = checkOneOf(kb.DocumentKinds, "kind")
	checkStatus = checkOneOf(experimentStatuses, "status")
)

// idChecker checks a node id's shape; existence is the op's question. The
// authored placeholder ids match the grammar and are still refused.
func idChecker(kinds ...string) check {
	re := regexp.MustCompile(`^` + kb.IDBody(kinds...) + `$`)
	var spelled []string
	for _, k := range kinds {
		spelled = append(spelled, k+"-[a-z0-9]{6}")
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
	checkClaimID      = idChecker("clm")
	checkExperimentID = idChecker("exp")
	checkSupportID    = idChecker("sup")
	checkEntryID      = idChecker("clm", "sup")
)

func checkWorkID(n *yaml.Node, field string) (any, *valueError) {
	token, err := requireString(n, field, "an id matching "+kb.WorkIDPattern)
	if err != nil {
		return nil, err
	}
	if !workIDFullRE.MatchString(token) {
		return nil, refuse(field, n, "%q is not an external-work id matching %s", token, kb.WorkIDPattern)
	}
	return token, nil
}

func checkCitationKey(n *yaml.Node, field string) (any, *valueError) {
	token, err := requireString(n, field, "a citation key")
	if err != nil {
		return nil, err
	}
	if !citationKeyRE.MatchString(token) {
		return nil, refuse(field, n, "%q is not a citation key — it must open and close alphanumeric, so the token has "+
			"no trailing punctuation for a depends-on bullet's head to argue about", token)
	}
	return token, nil
}

// checkRationaleID is any register entry's id: a rationale is one field on a
// claim, a support and a work alike.
func checkRationaleID(n *yaml.Node, field string) (any, *valueError) {
	if isString(n) && strings.HasPrefix(n.Value, kb.WorkPrefix+"-") {
		return checkWorkID(n, field)
	}
	return checkEntryID(n, field)
}

// checkDependsTarget is a clm- id, a work- id, or a framework token in its
// authored spelling, the one the bullet-head scanner reads.
func checkDependsTarget(n *yaml.Node, field string) (any, *valueError) {
	token, err := requireString(n, field, "a claim id, a framework token or a work id")
	if err != nil {
		return nil, err
	}
	if invariantRE.MatchString(token) || axiomTokenRE.MatchString(token) {
		return token, nil
	}
	if strings.HasPrefix(token, kb.WorkPrefix+"-") {
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
			e := refuse(dotted(container, k.Value), k, "is given twice in one table")
			e.allowed = allowed
			return nil, e
		}
		seen[k.Value] = true
		pairs = append(pairs, [2]*yaml.Node{k, v})
	}
	return pairs, nil
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
			if err.allowed == nil {
				err.allowed = allowed
			}
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

// dependsOnValue is one requested edge: a target, its context, a work
// target's applicability (nil is pending), and a demoted target's origin.
type dependsOnValue struct {
	ID, Context   string
	Applicability *float64
	Origin        string
}

var dependsOnFields = []field{
	{"id", checkDependsTarget, true},
	{"context", checkProse, false},
	{"applicability", checkApplicability, false},
}

var referencesFields = []field{
	{"id", checkClaimID, true},
	{"context", checkProse, false},
}

var demotedFields = []field{
	{"id", checkClaimID, true},
	{"origin", checkOneOf(kb.Origins, "origin"), true},
}

var strengthensFields = []field{
	{"id", checkClaimID, true},
	{"strength", checkStrength, true},
}

var supportsFields = []field{
	{"id", checkClaimID, true},
	{"fraction", checkFraction, true},
}

var experimentNodeFields = []field{
	{"exp-id", checkExperimentID, true},
	{"status", checkStatus, true},
	{"strengthens", checkStrengthens, false},
}

var supportNodeFields = []field{
	{"sup-id", checkSupportID, true},
	{"supports", checkSupports, false},
}

func optionalString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func optionalScore(m map[string]any, key string) *float64 {
	v, _ := m[key].(*float64)
	return v
}

func pairs(m map[string]any, key string) []pair {
	var out []pair
	for _, v := range listOf(m, key) {
		out = append(out, v.(pair))
	}
	return out
}

func listOf(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

var checkDependsOn = tableList(dependsOnFields, "{ id, context, applicability } tables",
	func(m map[string]any, n *yaml.Node, label string) (any, *valueError) {
		id := m["id"].(string)
		if _, ok := m["applicability"]; ok && !strings.HasPrefix(id, kb.WorkPrefix+"-") {
			e := refuse(label+".applicability", n, "%q is not an external work, and applicability is a work pairing's "+
				"quantity alone — a claim target's paren carries its derived solidity and a framework target's carries "+
				"its context, so there is nowhere for this value to land", id)
			e.allowed = fieldNames(dependsOnFields)
			return nil, e
		}
		return dependsOnValue{ID: id, Context: optionalString(m, "context"), Applicability: optionalScore(m, "applicability")}, nil
	})

var checkReferences = tableList(referencesFields, "{ id, context } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		return dependsOnValue{ID: m["id"].(string), Context: optionalString(m, "context")}, nil
	})

var checkDemoted = tableList(demotedFields, "{ id, origin } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		return dependsOnValue{ID: m["id"].(string), Origin: m["origin"].(string)}, nil
	})

var checkStrengthens = tableList(strengthensFields, "{ id, strength } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		strength := m["strength"].(float64)
		return pair{ID: m["id"].(string), Score: &strength}, nil
	})

var checkSupports = tableList(supportsFields, "{ id, fraction } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		return pair{ID: m["id"].(string), Score: optionalScore(m, "fraction")}, nil
	})

var checkExperimentNodes = tableList(experimentNodeFields, "{ exp-id, status, strengthens } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		return experimentDecl{ExpID: m["exp-id"].(string), Status: m["status"].(string), Strengthens: pairs(m, "strengthens")}, nil
	})

var checkSupportNodes = tableList(supportNodeFields, "{ sup-id, supports } tables",
	func(m map[string]any, _ *yaml.Node, _ string) (any, *valueError) {
		return supportDecl{SupID: m["sup-id"].(string), Supports: pairs(m, "supports")}, nil
	})

func scalarList(one check, of string) check {
	return func(n *yaml.Node, field string) (any, *valueError) {
		items, err := requireSequence(n, field, of)
		if err != nil {
			return nil, err
		}
		var out []string
		for i, item := range items {
			v, err := one(resolved(item), field)
			if err != nil {
				err.field = fmt.Sprintf("%s[%d]", field, i+1)
				return nil, err
			}
			out = append(out, v.(string))
		}
		return out, nil
	}
}

var (
	checkClaimIDList      = scalarList(checkClaimID, "claim ids")
	checkExperimentIDList = scalarList(checkExperimentID, "experiment ids")
	checkProseList        = scalarList(func(n *yaml.Node, f string) (any, *valueError) { return checkProse(n, f) }, "prose items")
)

// insertCommon is the register inserts' shared keys; each insert adds the
// one key only its own entry kind renders.
var insertCommon = []field{
	{"register", checkPath, true},
	{"title", checkProse, true},
	{"rigor", checkRigor, true},
	{"rationale", checkProse, true},
	{"depends-on", checkDependsOn, false},
	{"no-edge", checkProse, false},
}

// opFields is every op's closed key vocabulary: an unknown key is refused,
// never ignored, and a required key left out is refused by name.
var opFields = map[string][]field{
	"insert-claim-entry":   append(slices.Clone(insertCommon), field{"strengthen-by", checkProseList, false}),
	"insert-support-entry": append(slices.Clone(insertCommon), field{"supports", checkSupports, false}),
	"insert-experiment-entry": {
		{"document", checkPath, true},
		{"status", checkStatus, true},
		{"strengthens", checkStrengthens, false},
	},
	"insert-work-entry": {
		{"register", checkPath, true},
		{"key", checkCitationKey, true},
		{"title", checkProse, true},
		{"strength", checkWorkStrength, true},
		{"rationale", checkProse, true},
	},
	"set-work-strength": {
		{"id", checkWorkID, true},
		{"strength", checkWorkStrength, true},
	},
	"set-applicability": {
		{"id", checkClaimID, true},
		{"work", checkWorkID, true},
		{"applicability", checkApplicability, true},
	},
	"set-rigor": {
		{"id", checkEntryID, true},
		{"rigor", checkRigor, true},
	},
	"set-rationale": {
		{"id", checkRationaleID, true},
		{"rationale", checkProse, true},
	},
	"add-depends-on": {
		{"id", checkEntryID, true},
		{"depends-on", checkDependsOn, false},
		{"references", checkReferences, false},
	},
	AddBuildEdges: {
		{"id", checkEntryID, true},
		{"depends-on", checkDependsOn, false},
		{"references", checkReferences, false},
		{kb.RelationDemoted, checkDemoted, false},
	},
	ResolveDemoted: {
		{"id", checkClaimID, true},
		{"target", checkClaimID, true},
		{"action", checkOneOf(resolveActions, "action"), true},
	},
	"set-frontmatter": {
		{"document", checkPath, true},
		{"kind", checkKind, true},
		{"path-stable", checkProse, false},
		{"claims", checkClaimIDList, false},
		{"no-claim", checkProse, false},
		{"experiments", checkExperimentIDList, false},
		{"experiment-node", checkExperimentNodes, false},
		{"support-node", checkSupportNodes, false},
	},
	"mark-claim-in-leaf": {
		{"document", checkPath, true},
		{"id", checkClaimID, true},
		{"locator", checkProse, true},
	},
	"set-on-point-fraction": {
		{"id", checkSupportID, true},
		{"claim", checkClaimID, true},
		{"fraction", checkFraction, true},
	},
	"render-citation": {
		{"excerpt", checkExcerpt, true},
		{"cited-document", checkPath, true},
		{"anchor", checkProse, true},
		{"citing-document", checkPath, true},
	},
}

// entry is one accepted values entry: its 1-based position and its checked
// values, keyed by the op's vocabulary.
type entry struct {
	Index  int
	Values map[string]any
}

func (e entry) str(key string) string     { return optionalString(e.Values, key) }
func (e entry) score(key string) *float64 { return optionalScore(e.Values, key) }
func (e entry) list(key string) []string  { v, _ := e.Values[key].([]string); return v }
func (e entry) has(key string) bool       { _, ok := e.Values[key]; return ok }

func (e entry) dependsOn(key string) []dependsOnValue {
	var out []dependsOnValue
	for _, v := range listOf(e.Values, key) {
		out = append(out, v.(dependsOnValue))
	}
	return out
}

func (e entry) pairs(key string) []pair { return pairs(e.Values, key) }

func (e entry) experimentNodes() []experimentDecl {
	var out []experimentDecl
	for _, v := range listOf(e.Values, "experiment-node") {
		out = append(out, v.(experimentDecl))
	}
	return out
}

func (e entry) supportNodes() []supportDecl {
	var out []supportDecl
	for _, v := range listOf(e.Values, "support-node") {
		out = append(out, v.(supportDecl))
	}
	return out
}

// parseValues checks a values document — YAML, or JSON, which YAML reads —
// against op's vocabulary. It returns every entry, or every refusal and no
// entry: a batch is all-or-nothing.
func parseValues(data []byte, op string) ([]entry, []result.Item) {
	fields, ok := opFields[op]
	if !ok {
		panic("write: no vocabulary for op " + op)
	}
	if !utf8.Valid(data) {
		line := bytes.Count(data[:invalidUTF8At(data)], []byte("\n")) + 1
		return nil, []result.Item{{Key: entryKey, Line: line, Allowed: []string{entryKey},
			Detail: "is not valid UTF-8. A values document is read as strict UTF-8 and never decoded with replacement characters"}}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, []result.Item{{Key: entryKey, Line: yamlErrorLine(err), Allowed: []string{entryKey},
			Detail: "is not parseable YAML or JSON: " + err.Error()}}
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, []result.Item{{Key: entryKey, Line: 1, Allowed: []string{entryKey},
			Detail: "the document is empty; every op reads its values from the entry list"}}
	}
	root := resolved(doc.Content[0])
	if root.Kind != yaml.MappingNode {
		return nil, []result.Item{{Key: entryKey, Line: root.Line, Column: root.Column, Allowed: []string{entryKey},
			Detail: fmt.Sprintf("expected a table holding the entry list, got %s", typename(root))}}
	}
	var refusals []result.Item
	top, verr := mappingPairs(root, "", []string{entryKey})
	if verr != nil {
		return nil, []result.Item{verr.located(0)}
	}
	var entries *yaml.Node
	for _, p := range top {
		if p[0].Value != entryKey {
			refusals = append(refusals, result.Item{Key: p[0].Value, Line: p[0].Line, Column: p[0].Column, Allowed: []string{entryKey},
				Detail: "is not a key of a values document. The only top-level key is entry; the op travels on the command line and is never named inside the document"})
			continue
		}
		entries = p[1]
	}
	switch {
	case entries == nil:
		return nil, append(refusals, result.Item{Key: entryKey, Line: root.Line, Column: root.Column, Allowed: []string{entryKey},
			Detail: "is missing; every op reads its values from the entry list"})
	case entries.Kind != yaml.SequenceNode:
		return nil, append(refusals, result.Item{Key: entryKey, Line: entries.Line, Column: entries.Column, Allowed: []string{entryKey},
			Detail: fmt.Sprintf("expected a list of tables, got %s", typename(entries))})
	case len(entries.Content) == 0:
		return nil, append(refusals, result.Item{Key: entryKey, Line: entries.Line, Column: entries.Column, Allowed: []string{entryKey},
			Detail: "is empty; a values document with no entries names no work"})
	}
	var accepted []entry
	for i, raw := range entries.Content {
		values, entryRefusals := checkEntry(resolved(raw), i+1, fields)
		refusals = append(refusals, entryRefusals...)
		if len(entryRefusals) == 0 {
			accepted = append(accepted, entry{Index: i + 1, Values: values})
		}
	}
	if len(refusals) > 0 {
		return nil, refusals
	}
	return accepted, nil
}

// checkEntry checks one entry: unknown keys, then missing required keys and
// values, every refusal collected.
func checkEntry(n *yaml.Node, index int, fields []field) (map[string]any, []result.Item) {
	allowed := fieldNames(fields)
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
			refusals = append(refusals, result.Item{Key: p[0].Value, Entry: index, Line: p[0].Line, Column: p[0].Column, Allowed: allowed,
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
			if err.allowed == nil {
				err.allowed = allowed
			}
			refusals = append(refusals, err.located(index))
			continue
		}
		values[spec.name] = checked
	}
	return values, refusals
}

func (e *valueError) located(index int) result.Item {
	r := result.Item{Key: e.field, Entry: index, Detail: e.detail, Allowed: e.allowed}
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
