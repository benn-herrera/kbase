package format300

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// CheckMetadata is the refusal class the strict load raises for a record file
// that breaks its schema.
const CheckMetadata = "metadata"

// GraphFault is one refusal of the strict load: the check it fails, the
// kb-root-relative file and 1-based line it names, the field, the closed set
// the value must come from, the remedy, and one sentence.
type GraphFault struct {
	Check, Path string
	Line        int
	Key         string
	Allowed     string
	Remedy      string
	Detail      string
}

// GraphLoadError is every fault one strict load found, nodes file first.
type GraphLoadError struct{ Faults []GraphFault }

func (e *GraphLoadError) Error() string {
	parts := make([]string, len(e.Faults))
	for i, f := range e.Faults {
		parts[i] = fmt.Sprintf("%s:%d: %s", f.Path, f.Line, f.Detail)
	}
	return strings.Join(parts, "; ")
}

// DecodeNodes reads the node file's bytes data, at kb-root-relative path rel,
// into g strictly: a file that does not parse, an unknown or missing field, a
// value of the wrong type or outside its domain, an id outside its kind's
// grammar and a node id twice are each a GraphFault, returned in file order.
func DecodeNodes(data []byte, rel string, g *Graph) []GraphFault {
	return decodeFile(data, rel, g, decodeNodes)
}

// DecodeEdges reads the edge file's bytes data, at kb-root-relative path rel,
// into g strictly: a file that does not parse, an unknown or missing field, a
// value of the wrong type or outside its domain, an id outside its kind's
// grammar and a pairing BearingLegal refuses are each a GraphFault, returned
// in file order.
func DecodeEdges(data []byte, rel string, g *Graph) []GraphFault {
	return decodeFile(data, rel, g, decodeEdges)
}

func decodeFile(data []byte, rel string, g *Graph, decode func(records []graphRecord, g *Graph)) []GraphFault {
	var faults []GraphFault
	sink := &graphSink{rel: rel, faults: &faults}
	if records, ok := parseGraphFile(data, sink); ok {
		decode(records, g)
	}
	return faults
}

// graphSink collects one file's faults.
type graphSink struct {
	rel    string
	faults *[]GraphFault
}

func (s *graphSink) add(line int, key, allowed, format string, args ...any) {
	*s.faults = append(*s.faults, GraphFault{Check: CheckMetadata, Path: s.rel, Line: line, Key: key, Allowed: allowed,
		Detail: fmt.Sprintf(format, args...)})
}

var yamlErrorLineRE = regexp.MustCompile(`line (\d+)`)

// parseGraphFile is a record file's records; ok false, with one fault, where
// it is not one YAML document holding a list of mappings with no key twice.
func parseGraphFile(data []byte, sink *graphSink) (records []graphRecord, ok bool) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		line := 1
		if m := yamlErrorLineRE.FindStringSubmatch(err.Error()); m != nil {
			line, _ = strconv.Atoi(m[1])
		}
		if errors.Is(err, io.EOF) {
			sink.add(line, "", "", "holds no YAML document; a file with no records is []")
		} else {
			sink.add(line, "", "", "does not parse as YAML: %v", err)
		}
		return nil, false
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		sink.add(more.Line, "", "", "holds more than one YAML document")
		return nil, false
	}
	root := doc.Content[0]
	if root.Kind != yaml.SequenceNode {
		sink.add(root.Line, "", "", "is not a list of records")
		return nil, false
	}
	for _, item := range root.Content {
		if item.Kind != yaml.MappingNode {
			sink.add(item.Line, "", "", "a record that is not a mapping of fields to values")
			continue
		}
		r := graphRecord{line: item.Line, keys: map[string]*yaml.Node{}, values: map[string]*yaml.Node{}, sink: sink}
		for i := 0; i+1 < len(item.Content); i += 2 {
			k, v := item.Content[i], item.Content[i+1]
			if k.Kind != yaml.ScalarNode {
				sink.add(k.Line, "", "", "does not parse: a key that is not a string")
				return nil, false
			}
			if _, twice := r.values[k.Value]; twice {
				sink.add(k.Line, k.Value, "", "does not parse: field %q repeats in one record", k.Value)
				return nil, false
			}
			r.order = append(r.order, k.Value)
			r.keys[k.Value], r.values[k.Value] = k, v
		}
		records = append(records, r)
	}
	return records, true
}

// graphRecord is one record's fields as parsed, and where its faults go.
type graphRecord struct {
	line   int
	order  []string
	keys   map[string]*yaml.Node
	values map[string]*yaml.Node
	sink   *graphSink
}

// fields refuses every field of the record outside want and every one of want
// it lacks.
func (r graphRecord) fields(want []string) {
	for _, k := range r.order {
		if !slices.Contains(want, k) {
			r.sink.add(r.keys[k].Line, k, strings.Join(want, ", "), "unknown field %q", k)
		}
	}
	for _, k := range want {
		if _, ok := r.values[k]; !ok {
			r.sink.add(r.line, k, "", "missing field %q; an empty value is spelled \"\" or []", k)
		}
	}
}

func (r graphRecord) wrongType(n *yaml.Node, key, expected string) {
	r.sink.add(n.Line, key, "", "%s is %s, not %s", key, describeNode(n), expected)
}

func describeNode(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a list"
	case yaml.ScalarNode:
		return fmt.Sprintf("%q (%s)", n.Value, strings.TrimPrefix(n.ShortTag(), "!!"))
	}
	return "an alias"
}

func isStringNode(n *yaml.Node) bool { return n.Kind == yaml.ScalarNode && n.ShortTag() == "!!str" }

// oneLine is v where it holds no line break; a fault otherwise.
func (r graphRecord) oneLine(n *yaml.Node, key string) string {
	if strings.ContainsAny(n.Value, "\n\r") {
		r.sink.add(n.Line, key, "", "%s holds a line break; a record's string is one line", key)
	}
	return n.Value
}

// str is the string field key, "" where it is absent or faulted.
func (r graphRecord) str(key string) string {
	n, ok := r.values[key]
	if !ok {
		return ""
	}
	if !isStringNode(n) {
		r.wrongType(n, key, "a string")
		return ""
	}
	return r.oneLine(n, key)
}

func (r graphRecord) title() string {
	t := r.str(fieldTitle)
	if n, ok := r.values[fieldTitle]; ok && isStringNode(n) && t == "" {
		r.sink.add(n.Line, fieldTitle, "", "title is empty; a node's title is a non-empty line")
	}
	return t
}

// oneOf is the string field key where it is one of allowed.
func (r graphRecord) oneOf(key string, allowed []string) string {
	v := r.str(key)
	if n, ok := r.values[key]; ok && isStringNode(n) && !slices.Contains(allowed, v) {
		r.sink.add(n.Line, key, strings.Join(allowed, ", "), "%s %q is none of the values it takes", key, v)
		return ""
	}
	return v
}

func (r graphRecord) boolean(key string) bool {
	n, ok := r.values[key]
	if !ok {
		return false
	}
	var b bool
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!bool" || n.Decode(&b) != nil {
		r.wrongType(n, key, "true or false")
		return false
	}
	return b
}

func (r graphRecord) list(key string) []string {
	n, ok := r.values[key]
	if !ok {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		r.wrongType(n, key, "a list of strings")
		return nil
	}
	var out []string
	for _, item := range n.Content {
		if !isStringNode(item) {
			r.wrongType(item, key, "a string")
			continue
		}
		out = append(out, r.oneLine(item, key))
	}
	return out
}

// domainTop is the most a bearing is written as: the top of the domain the
// decoder checks a bearing against and scoreDomain states.
const domainTop = 1.0

// scoreDomain is the closed set a rigor and a bearing are written from.
var scoreDomain = fmt.Sprintf("a number in [0, %s], or %q", strings.TrimSuffix(PyFloatRepr(domainTop), ".0"), PendingLiteral)

// score is a number-or-pending field: its number, or pending; ok false where
// the field is absent or faulted. A number is refused below 0, above top, and
// infinite: no writer spells an infinity the decoder reads back as a number.
func (r graphRecord) score(key string, top float64) (v float64, pending, ok bool) {
	n, present := r.values[key]
	if !present {
		return 0, false, false
	}
	allowed := scoreDomain
	switch {
	case isStringNode(n) && n.Value == PendingLiteral:
		return 0, true, true
	case n.Kind == yaml.ScalarNode && (n.ShortTag() == "!!int" || n.ShortTag() == "!!float"):
		if err := n.Decode(&v); err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > top {
			r.sink.add(n.Line, key, allowed, "%s %s is outside its domain", key, n.Value)
			return 0, false, false
		}
		return v, false, true
	}
	r.wrongType(n, key, allowed)
	return 0, false, false
}

// rigor is the rigor field as its file holds it: nil where pending.
func (r graphRecord) rigor() *float64 {
	v, pending, ok := r.score(RigorKey, math.Inf(1))
	if !ok || pending {
		return nil
	}
	return &v
}

func (r graphRecord) bearing() Bearing {
	v, pending, _ := r.score(BearingKey, domainTop)
	return Bearing{Value: v, Pending: pending}
}

// endID is the id field key and the kind its grammar names, kind "" where it is
// absent or faulted.
func (r graphRecord) endID(key string) (id, kind string) {
	id = r.str(key)
	n, ok := r.values[key]
	if !ok || !isStringNode(n) {
		return id, ""
	}
	kind, inGrammar := NodeIDKind(id)
	if !inGrammar {
		r.sink.add(n.Line, key, "", "%s %q is outside its kind's id grammar", key, id)
		return id, ""
	}
	return id, kind
}

func decodeNodes(records []graphRecord, g *Graph) {
	seen := map[string]int{}
	for _, r := range records {
		if _, ok := r.values[fieldID]; !ok {
			r.sink.add(r.line, fieldID, "", "missing field %q", fieldID)
			continue
		}
		id, kind := r.endID(fieldID)
		if kind == "" {
			continue
		}
		r.fields(nodeFields[kind])
		if first, twice := seen[id]; twice {
			r.sink.add(r.keys[fieldID].Line, fieldID, "", "node id %q repeats; its first record opens on line %d", id, first)
		} else {
			seen[id] = r.line
		}
		switch kind {
		case NodeKindClaim:
			g.Claims = append(g.Claims, ClaimNode{ID: id, Title: r.title(), Rigor: r.rigor(),
				ManuallyScored: r.boolean(ManuallyScoredKey), Rationale: r.str(fieldRationale),
				NoEdge: r.str(fieldNoEdge), StrengthenBy: r.list(fieldStrengthenBy)})
		case NodeKindAnalysis:
			g.Analyses = append(g.Analyses, AnalysisNode{ID: id, Title: r.title(), Rigor: r.rigor(),
				ManuallyScored: r.boolean(ManuallyScoredKey), Rationale: r.str(fieldRationale), NoEdge: r.str(fieldNoEdge)})
		case NodeKindExperiment:
			g.Experiments = append(g.Experiments, ExperimentNode{ID: id, Title: r.title(),
				Status: r.oneOf(StatusKey, ExperimentStatuses), Rigor: r.rigor(), ManuallyScored: r.boolean(ManuallyScoredKey)})
		case NodeKindWork:
			g.Works = append(g.Works, WorkNode{ID: id, Title: r.title(), Rigor: r.rigor(), Rationale: r.str(fieldRationale)})
		case NodeKindAxiom:
			g.Axioms = append(g.Axioms, AxiomNode{ID: id, Title: r.title()})
		}
	}
}

// pairing refuses a scoring edge of relation between ends of these kinds where
// BearingLegal refuses it; an end already faulted is not judged.
func (r graphRecord) pairing(relation, sourceKind, targetKind string) {
	if relation == "" || sourceKind == "" || targetKind == "" || BearingLegal(relation, sourceKind, targetKind) {
		return
	}
	r.sink.add(r.line, fieldRelation, "", "%s does not join a %s source to a %s target", relation, sourceKind, targetKind)
}

func decodeEdges(records []graphRecord, g *Graph) {
	for _, r := range records {
		if _, ok := r.values[fieldRelation]; !ok {
			r.sink.add(r.line, fieldRelation, "", "missing field %q", fieldRelation)
			continue
		}
		relation := r.oneOf(fieldRelation, Relations)
		if relation == "" {
			continue
		}
		r.fields(edgeFields(relation))
		source, sourceKind := r.endID(fieldSource)
		target, targetKind := r.endID(fieldTarget)
		context := r.str(fieldContext)
		switch relation {
		case RelationReferences:
			g.References = append(g.References, UnscoredEdge{Source: source, Target: target, Context: context})
		case RelationDemoted:
			replaced := r.oneOf(fieldReplacedRelation, DemotedReplacedRelations)
			r.pairing(replaced, sourceKind, targetKind)
			g.Demoted = append(g.Demoted, DemotedEdge{Source: source, Target: target, Context: context,
				ReplacedRelation: replaced, Bearing: r.bearing(), ManuallyScored: r.boolean(ManuallyScoredKey)})
		case RelationFounds:
			r.pairing(relation, sourceKind, targetKind)
			g.Founds = append(g.Founds, BearingEdge{Source: source, Target: target, Context: context, Bearing: r.bearing()})
		case RelationSupports, RelationLifts, RelationRefutes:
			r.pairing(relation, sourceKind, targetKind)
			g.Scorable = append(g.Scorable, ScorableEdge{Relation: relation, Source: source, Target: target, Context: context,
				Bearing: r.bearing(), ManuallyScored: r.boolean(ManuallyScoredKey)})
		}
	}
}
