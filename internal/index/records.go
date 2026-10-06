package index

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
)

// record is one index record, its keys in written order.
type record = []kb.IndexField

// Records is every index file's records, by name.
type Records map[string][]record

func fractionValue(f kb.Fraction) any {
	switch {
	case !f.Set:
		return nil
	case f.Pending:
		return kb.PendingLiteral
	}
	return f.Value
}

// fieldsOf is each row's record.
func fieldsOf[T interface{ IndexFields() []kb.IndexField }](rows []T) []record {
	out := make([]record, len(rows))
	for i, r := range rows {
		out[i] = r.IndexFields()
	}
	return out
}

func buildClaims(st kb.State, sol Solidity) []kb.NodeRow {
	cites := map[string]int{}
	for _, leaf := range st.Leaves {
		for _, c := range leaf.Claims {
			cites[c]++
		}
	}
	var out []kb.NodeRow
	for _, e := range st.ClaimEntries {
		r := sol.Results[e.ID]
		depends := 0
		for _, d := range e.DependsOn {
			if d.Relation == kb.RelationDepends {
				depends++
			}
		}
		out = append(out, kb.NodeRow{NodeType: "claim", ID: e.ID, Title: e.Title,
			CanonicalPath: e.CanonicalPath, CanonicalAnchor: e.CanonicalAnchor,
			Confidence: e.Confidence, DerivationSolidity: r.Derivation,
			ExperimentalSolidity: r.Experimental, Solidity: r.Final,
			BuildStatus: BuildStatusPhrase(r.Final), BuildBand: BuildBand(r.Final),
			Rationale: e.Rationale, DependsOnCount: depends,
			StrengthenByCount: len(e.StrengthenBy), CitationCount: cites[e.ID]})
	}
	for _, x := range st.Experiments {
		out = append(out, kb.NodeRow{NodeType: "experiment", ID: x.ID, Title: x.Title,
			CanonicalPath: x.CanonicalPath, CanonicalAnchor: x.CanonicalAnchor, Status: x.Status})
	}
	for _, s := range st.Supports {
		out = append(out, kb.NodeRow{NodeType: "support", ID: s.ID, Title: s.Title,
			CanonicalPath: s.CanonicalPath, CanonicalAnchor: s.CanonicalAnchor,
			Quality: s.Quality, Solidity: sol.SupSolidity[s.ID]})
	}
	for _, n := range st.FrameworkNodes {
		out = append(out, kb.NodeRow{NodeType: n.NodeType, ID: n.ID, Title: n.Title,
			CanonicalPath: n.CanonicalPath, CanonicalAnchor: n.CanonicalAnchor})
	}
	for _, w := range st.Works {
		out = append(out, kb.NodeRow{NodeType: "work", ID: w.ID, Title: w.Title,
			CanonicalPath: w.CanonicalPath, CanonicalAnchor: w.CanonicalAnchor, Strength: w.Strength})
	}
	slices.SortStableFunc(out, func(a, b kb.NodeRow) int {
		return cmp.Or(strings.Compare(a.NodeType, b.NodeType), strings.Compare(a.ID, b.ID))
	})
	return out
}

func edgeRow(source, target, relation, targetKind string, recorded, strength *float64, context *string, fraction kb.Fraction) kb.EdgeRow {
	return kb.EdgeRow{Source: source, Target: target, Relation: relation, TargetKind: targetKind,
		TargetSolidityRecorded: recorded, Strength: strength, Context: context, Fraction: fractionValue(fraction)}
}

func buildDependsOn(st kb.State) []kb.EdgeRow {
	var out []kb.EdgeRow
	emit := func(e kb.Edge) {
		r := edgeRow(e.Source, e.Target, e.Relation, e.TargetKind, e.TargetSolidityRecorded, e.Strength, e.Context, e.Fraction)
		if e.Origin != "" {
			r.Origin = &e.Origin
		}
		out = append(out, r)
	}
	for _, e := range st.ClaimEntries {
		for _, d := range slices.Concat(e.DependsOn, e.References, e.Demoted) {
			emit(d)
		}
	}
	for _, s := range st.Supports {
		for _, d := range s.DependsOn {
			emit(d)
		}
		for _, p := range s.Supports {
			out = append(out, edgeRow(s.ID, p.ClaimID, kb.RelationSupports, "claim", nil, nil, nil, p.Fraction))
		}
	}
	for _, x := range st.Experiments {
		for _, p := range x.Strengthens {
			out = append(out, edgeRow(x.ID, p.ClaimID, kb.RelationStrengthens, "claim", nil, ptr(p.Strength), nil, kb.Fraction{}))
		}
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

func buildSupportedBy(st kb.State, sol Solidity) []kb.SupportedByRow {
	var out []kb.SupportedByRow
	for _, s := range st.Supports {
		for _, p := range s.Supports {
			out = append(out, kb.SupportedByRow{ClaimID: p.ClaimID, SupID: s.ID, Fraction: fractionValue(p.Fraction), SupSolidity: sol.SupSolidity[s.ID]})
		}
	}
	slices.SortStableFunc(out, func(a, b kb.SupportedByRow) int {
		return cmp.Or(strings.Compare(a.ClaimID, b.ClaimID), strings.Compare(a.SupID, b.SupID))
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

// CoverageError is an edge naming a framework node or external work that no
// authored source stands up.
type CoverageError struct{ Msg string }

func (e CoverageError) Error() string { return e.Msg }

func checkCoverage(claims []kb.NodeRow, dependsOn []kb.EdgeRow) error {
	frameworkPresent, workPresent := map[string]bool{}, map[string]bool{}
	for _, r := range claims {
		switch {
		case slices.Contains(kb.FrameworkKinds, r.NodeType):
			frameworkPresent[r.ID] = true
		case r.NodeType == "work":
			workPresent[r.ID] = true
		}
	}
	framework := map[string]bool{}
	works := map[string]bool{}
	for _, e := range dependsOn {
		switch {
		case slices.Contains(kb.FrameworkKinds, e.TargetKind) && !frameworkPresent[e.Target]:
			framework[e.Target] = true
		case e.TargetKind == "work" && !workPresent[e.Target]:
			works[e.Target] = true
		}
	}
	if len(framework) > 0 {
		missing := sortedSet(framework)
		return CoverageError{fmt.Sprintf("%d depends-on edge target(s) reference framework nodes absent from the rebuilt index: %s; "+
			"the framework source (%s, or legacy %s) did not yield them", len(missing), strings.Join(missing, ", "), kb.InvariantsFile, kb.AgentsFile)}
	}
	if len(works) > 0 {
		missing := sortedSet(works)
		return CoverageError{fmt.Sprintf("%d rests-on edge target(s) name external works with no register entry: %s",
			len(missing), strings.Join(missing, ", "))}
	}
	return nil
}

// BuildRecords is every index file's records for st.
func BuildRecords(st kb.State) (Records, error) {
	sol, err := ComputeSolidity(st)
	if err != nil {
		return nil, err
	}
	claims := buildClaims(st, sol)
	dependsOn := buildDependsOn(st)
	if err := checkCoverage(claims, dependsOn); err != nil {
		return nil, err
	}
	return Records{
		kb.IndexClaims: fieldsOf(claims), kb.IndexDependsOn: fieldsOf(dependsOn), kb.IndexStrengthenBy: fieldsOf(buildStrengthenBy(st)),
		kb.IndexSupportedBy: fieldsOf(buildSupportedBy(st, sol)), kb.IndexCites: fieldsOf(buildCites(st)),
		kb.IndexSubtreeAggregates: fieldsOf(buildSubtreeAggregates(st)),
	}, nil
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
