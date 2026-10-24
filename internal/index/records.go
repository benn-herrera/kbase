package index

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/log"
)

// record is one index record, its keys in written order.
type record = []kb.IndexField

// Records is every index file's records, by name.
type Records map[string][]record

// bearingValue is an edge's bearing column: the text BearingText returns where
// its relation allows a bearing, null where it allows none.
func bearingValue(e kb.Edge, sourceKind string) any {
	if !kb.BearingLegal(e.Relation, sourceKind, e.TargetKind) {
		return nil
	}
	return kb.BearingText(e.Bearing)
}

// fieldsOf is each row's record.
func fieldsOf[T interface{ IndexFields() []kb.IndexField }](rows []T) []record {
	out := make([]record, len(rows))
	for i, r := range rows {
		out[i] = r.IndexFields()
	}
	return out
}

// kindsOf is every authored node's kind by id, read off the state the walk
// read.
func kindsOf(st kb.State) map[string]string {
	out := map[string]string{}
	for _, e := range st.ClaimEntries {
		out[e.ID] = kb.NodeKindClaim
	}
	for _, s := range st.Analyses {
		out[s.ID] = kb.NodeKindAnalysis
	}
	for _, x := range st.Experiments {
		out[x.ID] = kb.NodeKindExperiment
	}
	for _, w := range st.Works {
		out[w.ID] = kb.NodeKindWork
	}
	for _, n := range st.Axioms {
		out[n.ID] = n.NodeType
	}
	return out
}

func buildClaims(st kb.State, strength Strength) []kb.NodeRow {
	cites := map[string]int{}
	for _, leaf := range st.Leaves {
		for _, c := range leaf.Claims {
			cites[c]++
		}
	}
	var out []kb.NodeRow
	for _, e := range st.ClaimEntries {
		r := strength.Results[e.ID]
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindClaim, ID: e.ID, Title: e.Title,
			CanonicalPath: e.CanonicalPath, CanonicalAnchor: e.CanonicalAnchor,
			Rigor: e.Rigor, DerivationStrength: r.Derivation, Strength: r.Final,
			BuildStatus: BuildStatusPhrase(r.Final), BuildBand: BuildBand(r.Final),
			Rationale: e.Rationale, PremiseCount: len(e.Premises),
			StrengthenByCount: len(e.StrengthenBy), CitationCount: cites[e.ID]})
	}
	for _, x := range st.Experiments {
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindExperiment, ID: x.ID, Title: x.Title,
			CanonicalPath: x.CanonicalPath, CanonicalAnchor: x.CanonicalAnchor, Status: x.Status, Rigor: x.Rigor})
	}
	for _, s := range st.Analyses {
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindAnalysis, ID: s.ID, Title: s.Title,
			CanonicalPath: s.CanonicalPath, CanonicalAnchor: s.CanonicalAnchor,
			Rigor: s.Rigor, Strength: strength.AnalysisResults[s.ID].Final})
	}
	for _, n := range st.Axioms {
		out = append(out, kb.NodeRow{NodeType: n.NodeType, ID: n.ID, Title: n.Title,
			CanonicalPath: n.CanonicalPath, CanonicalAnchor: n.CanonicalAnchor, Rigor: ptr(kb.AxiomRigor)})
	}
	for _, w := range st.Works {
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindWork, ID: w.ID, Title: w.Title,
			CanonicalPath: w.CanonicalPath, CanonicalAnchor: w.CanonicalAnchor, Rigor: w.Rigor})
	}
	slices.SortStableFunc(out, func(a, b kb.NodeRow) int {
		return cmp.Or(strings.Compare(a.NodeType, b.NodeType), strings.Compare(a.ID, b.ID))
	})
	return out
}

// authoredEdges is every edge the KB's bullets and pair lines author: a claim's
// premises, refutations, references and demoted edges, an analysis's premises,
// lifts and refutations, and an experiment's supports, lifts and refutations.
func authoredEdges(st kb.State) []kb.Edge {
	var out []kb.Edge
	for _, e := range st.ClaimEntries {
		out = append(out, slices.Concat(e.Premises, e.Refutes, e.References, e.Demoted)...)
	}
	for _, s := range st.Analyses {
		out = append(out, slices.Concat(s.Premises, s.Lifts, s.Refutes)...)
	}
	for _, x := range st.Experiments {
		out = append(out, slices.Concat(x.Supports, x.Lifts, x.Refutes)...)
	}
	return out
}

// buildEdges is every scoring edge of the KB, push-authored or flipped, as an
// index row.
func buildEdges(st kb.State) []kb.EdgeRow {
	kinds := kindsOf(st)
	var out []kb.EdgeRow
	for _, e := range authoredEdges(st) {
		r := kb.EdgeRow{Source: e.Source, Target: e.Target, Relation: e.Relation, TargetKind: e.TargetKind,
			SourceStrengthRecorded: e.SourceStrengthRecorded, Bearing: bearingValue(e, kinds[e.Source]), Context: e.Context}
		if e.Origin != "" {
			r.Origin = &e.Origin
		}
		out = append(out, r)
	}
	context := func(r kb.EdgeRow) string {
		if r.Context == nil {
			return ""
		}
		return *r.Context
	}
	slices.SortStableFunc(out, func(a, b kb.EdgeRow) int {
		return cmp.Or(strings.Compare(a.Source, b.Source), strings.Compare(a.Target, b.Target), strings.Compare(context(a), context(b)))
	})
	return out
}

func buildStrengthenBy(st kb.State) []kb.StrengthenByRow {
	var out []kb.StrengthenByRow
	for _, e := range st.ClaimEntries {
		for _, sb := range e.StrengthenBy {
			out = append(out, kb.StrengthenByRow{ClaimID: sb.ClaimID, ItemIdx: sb.ItemIdx, Text: sb.Text, MentionedIDs: sb.MentionedIDs})
		}
	}
	slices.SortStableFunc(out, func(a, b kb.StrengthenByRow) int {
		return cmp.Or(strings.Compare(a.ClaimID, b.ClaimID), cmp.Compare(a.ItemIdx, b.ItemIdx))
	})
	return out
}

func buildCites(st kb.State) []kb.CiteRow {
	var out []kb.CiteRow
	for _, leaf := range st.Leaves {
		for _, c := range leaf.Claims {
			out = append(out, kb.CiteRow{ClaimID: c, LeafPath: leaf.Path, LeafKind: leaf.Kind, Tier2Marked: leaf.Tier2Marked[c]})
		}
	}
	slices.SortStableFunc(out, func(a, b kb.CiteRow) int {
		return cmp.Or(strings.Compare(a.ClaimID, b.ClaimID), strings.Compare(a.LeafPath, b.LeafPath))
	})
	return out
}

// Aggregate is an index or entry-point node's owned subtree union.
type Aggregate struct{ Claims, Experiments []string }

// isUnder is whether a document lies within the directory of an index node.
func isUnder(doc, indexPath string) bool {
	dir := path.Dir(indexPath)
	return dir == "." || strings.HasPrefix(doc, dir+"/")
}

func sortedSet(set map[string]bool) []string {
	out := []string{}
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// SubtreeAggregates is every index node's union of the claims its leaves
// host and the experiments its leaves own; an entry-point's spans the KB.
func SubtreeAggregates(st kb.State) map[string]Aggregate {
	out := map[string]Aggregate{}
	for _, idx := range st.Indexes {
		ep := idx.Kind == kb.DocumentEntryPoint
		claims, exps := map[string]bool{}, map[string]bool{}
		for _, leaf := range st.Leaves {
			if ep || isUnder(leaf.Path, idx.Path) {
				for _, c := range leaf.Claims {
					claims[c] = true
				}
			}
		}
		for _, x := range st.Experiments {
			if ep || isUnder(x.CanonicalPath, idx.Path) {
				exps[x.ID] = true
			}
		}
		out[idx.Path] = Aggregate{sortedSet(claims), sortedSet(exps)}
	}
	return out
}

func buildSubtreeAggregates(st kb.State) []kb.AggregateRow {
	agg := SubtreeAggregates(st)
	var out []kb.AggregateRow
	for _, idx := range st.Indexes {
		a := agg[idx.Path]
		out = append(out, kb.AggregateRow{NodePath: idx.Path, NodeKind: idx.Kind, SubtreeClaims: a.Claims, SubtreeExperiments: a.Experiments})
	}
	slices.SortStableFunc(out, func(a, b kb.AggregateRow) int { return strings.Compare(a.NodePath, b.NodePath) })
	return out
}

// CoverageError is an edge naming an axiom or external work that no authored
// source stands up: the key is the node id it names, and the path and line are
// the register bullet that cites it.
type CoverageError struct {
	Key, Path string
	Line      int
	Msg       string
}

func (e CoverageError) Error() string { return e.Msg }

// checkCoverage refuses an index whose edges name a source the rebuilt index
// does not carry: an axiom-shaped id is an axiom token the axiom source did not
// yield, and any other scoring source is an external work with no register
// entry.
func checkCoverage(st kb.State, claims []kb.NodeRow) error {
	present := map[string]bool{}
	for _, r := range claims {
		present[r.ID] = true
	}
	axioms := map[string]bool{}
	works := map[string]bool{}
	for _, e := range authoredEdges(st) {
		if present[e.Source] {
			continue
		}
		switch {
		case strings.HasPrefix(e.Source, kb.AxiomPrefix):
			axioms[e.Source] = true
		case kb.Scoring(e.Relation):
			works[e.Source] = true
		}
	}
	if len(axioms) > 0 {
		missing := sortedSet(axioms)
		return CoverageError{Key: missing[0], Msg: fmt.Sprintf("%d premise edge source(s) name axioms absent from the rebuilt index: %s; "+
			"the axiom source (%s, else legacy %s or %s) did not yield them", len(missing), strings.Join(missing, ", "),
			kb.AxiomsFile, kb.InvariantsFile, kb.AgentsFile)}
	}
	if len(works) > 0 {
		missing := sortedSet(works)
		return CoverageError{Key: missing[0], Msg: fmt.Sprintf("%d scoring edge source(s) name no node the rebuilt index carries: %s; "+
			"an external work's source is its register entry", len(missing), strings.Join(missing, ", "))}
	}
	return nil
}

// coverageAt is cov with the register bullet citing its key earliest: that
// register and the bullet's line.
func coverageAt(src *kb.Source, cov CoverageError) CoverageError {
	regs, err := kb.Registers(src)
	if err != nil {
		return cov
	}
	for _, rel := range regs {
		text, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			return cov
		}
		for _, c := range kb.WorkCitations(text, rel) {
			if c.ID == cov.Key {
				cov.Path, cov.Line = c.Path, c.Line
				return cov
			}
		}
	}
	return cov
}

// Rows is the KB's index rows, one set per index file, each in its file's
// order.
type Rows struct {
	Nodes        []kb.NodeRow
	Edges        []kb.EdgeRow
	StrengthenBy []kb.StrengthenByRow
	Cites        []kb.CiteRow
	Aggregates   []kb.AggregateRow
}

// Derive is the index rows of the KB src holds. A KB state the walk refuses
// is a result.Refusal; a coverage gap returns as a CoverageError and a premise
// cycle as a CycleError, unmapped.
func Derive(src *kb.Source) (Rows, error) {
	st, err := kb.Discover(src, log.Discard())
	if err != nil {
		return Rows{}, refusal(src, err)
	}
	return deriveRows(st)
}

func deriveRows(st kb.State) (Rows, error) {
	strength, err := ComputeStrength(st)
	if err != nil {
		return Rows{}, err
	}
	nodes := buildClaims(st, strength)
	if err := checkCoverage(st, nodes); err != nil {
		return Rows{}, err
	}
	return Rows{Nodes: nodes, Edges: buildEdges(st), StrengthenBy: buildStrengthenBy(st),
		Cites: buildCites(st), Aggregates: buildSubtreeAggregates(st)}, nil
}

func (r Rows) records() Records {
	return Records{
		kb.IndexClaims: fieldsOf(r.Nodes), kb.IndexEdges: fieldsOf(r.Edges), kb.IndexStrengthenBy: fieldsOf(r.StrengthenBy),
		kb.IndexCites: fieldsOf(r.Cites), kb.IndexSubtreeAggregates: fieldsOf(r.Aggregates),
	}
}

// BuildRecords is every index file's records for st.
func BuildRecords(st kb.State) (Records, error) {
	rows, err := deriveRows(st)
	if err != nil {
		return nil, err
	}
	return rows.records(), nil
}

// Serialize is a file's text as a YAML stream: per record one line, the
// document marker and the record as a JSON object — ", " and ": " separators,
// keys in built order, non-ASCII written raw but for streamUnsafe — and ""
// for no records.
func Serialize(recs []record) string {
	var b strings.Builder
	for _, r := range recs {
		var rec strings.Builder
		rec.WriteByte('{')
		for i, f := range r {
			if i > 0 {
				rec.WriteString(", ")
			}
			writeJSONString(&rec, f.Key)
			rec.WriteString(": ")
			writeJSONValue(&rec, f.Value)
		}
		rec.WriteByte('}')
		b.WriteString(kb.IndexRecordMarker)
		b.WriteString(streamUnsafe.Replace(rec.String()))
		b.WriteByte('\n')
	}
	return b.String()
}

// streamUnsafe escapes what a record's JSON carries raw but a reader of the
// stream takes as a line break or refuses as unprintable: U+2028 and U+2029,
// DEL and the C1 controls, NEL among them, the byte-order mark, and the
// noncharacters U+FFFE and U+FFFF. Every one can stand only inside a string,
// where the escape reads back as the character.
var streamUnsafe = func() *strings.Replacer {
	var pairs []string
	escape := func(r rune) { pairs = append(pairs, string(r), fmt.Sprintf(`\u%04x`, r)) }
	for r := rune(0x7f); r <= 0x9f; r++ {
		escape(r)
	}
	for _, r := range []rune{0x2028, 0x2029, 0xfeff, 0xfffe, 0xffff} {
		escape(r)
	}
	return strings.NewReplacer(pairs...)
}()

func writeJSONValue(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		writeJSONString(b, x)
	case float64:
		b.WriteString(kb.PyFloatRepr(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case []string:
		b.WriteByte('[')
		for i, s := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			writeJSONString(b, s)
		}
		b.WriteByte(']')
	default:
		panic(fmt.Sprintf("index: no JSON form for %T", v))
	}
}

func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
