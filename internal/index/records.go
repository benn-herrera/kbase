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

// IndexFiles is the derived index's file inventory, in emission order; each
// is .index/<name>.jsonl.
var IndexFiles = []string{"claims", "depends-on", "strengthen-by", "supported-by", "cites", "subtree-aggregates"}

// field is one key of a record; records keep their keys in the order built.
type field struct {
	key string
	val any
}

type record []field

// Records is every index file's records, by short name.
type Records map[string][]record

func optional(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func optionalString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func fractionValue(f kb.Fraction) any {
	switch {
	case !f.Set:
		return nil
	case f.Pending:
		return kb.PendingLiteral
	}
	return f.Value
}

// sortRecords is a stable sort by the key the record yields.
func sortRecords(recs []record, key func(record) []string) {
	slices.SortStableFunc(recs, func(a, b record) int { return slices.Compare(key(a), key(b)) })
}

func str(r record, i int) string {
	s, _ := r[i].val.(string)
	return s
}

func buildClaims(st kb.State, sol Solidity) []record {
	cites := map[string]int{}
	for _, leaf := range st.Leaves {
		for _, c := range leaf.Claims {
			cites[c]++
		}
	}
	var out []record
	for _, e := range st.ClaimEntries {
		r := sol.Results[e.ID]
		depends := 0
		for _, d := range e.DependsOn {
			if d.Relation == "depends" {
				depends++
			}
		}
		out = append(out, record{
			{"node_type", "claim"}, {"id", e.ID}, {"title", e.Title},
			{"canonical_path", e.CanonicalPath}, {"canonical_anchor", e.CanonicalAnchor},
			{"confidence", optional(e.Confidence)}, {"derivation_solidity", optional(r.Derivation)},
			{"experimental_solidity", optional(r.Experimental)}, {"solidity", optional(r.Final)},
			{"build_status", optionalString(BuildStatusPhrase(r.Final))}, {"build_band", BuildBand(r.Final)},
			{"rationale", e.Rationale}, {"depends_on_count", depends},
			{"strengthen_by_count", len(e.StrengthenBy)}, {"citation_count", cites[e.ID]},
		})
	}
	for _, x := range st.Experiments {
		out = append(out, record{{"node_type", "experiment"}, {"id", x.ID}, {"title", x.Title},
			{"canonical_path", x.CanonicalPath}, {"canonical_anchor", x.CanonicalAnchor}, {"status", x.Status}})
	}
	for _, s := range st.Supports {
		out = append(out, record{{"node_type", "support"}, {"id", s.ID}, {"title", s.Title},
			{"canonical_path", s.CanonicalPath}, {"canonical_anchor", s.CanonicalAnchor},
			{"quality", optional(s.Quality)}, {"solidity", optional(sol.SupSolidity[s.ID])}})
	}
	for _, n := range st.FrameworkNodes {
		out = append(out, record{{"node_type", n.NodeType}, {"id", n.ID}, {"title", n.Title},
			{"canonical_path", n.CanonicalPath}, {"canonical_anchor", n.CanonicalAnchor}})
	}
	for _, w := range st.Works {
		out = append(out, record{{"node_type", "work"}, {"id", w.ID}, {"title", w.Title},
			{"canonical_path", w.CanonicalPath}, {"canonical_anchor", w.CanonicalAnchor}, {"strength", optional(w.Strength)}})
	}
	sortRecords(out, func(r record) []string { return []string{str(r, 0), str(r, 1)} })
	return out
}

func edgeRecord(source, target, relation, targetKind string, recorded, strength *float64, context *string, fraction kb.Fraction) record {
	return record{{"source", source}, {"target", target}, {"relation", relation}, {"target_kind", targetKind},
		{"target_solidity_recorded", optional(recorded)}, {"strength", optional(strength)},
		{"context", optionalString(context)}, {"fraction", fractionValue(fraction)}}
}

func buildDependsOn(st kb.State) []record {
	var out []record
	emit := func(e kb.Edge) {
		out = append(out, edgeRecord(e.Source, e.Target, e.Relation, e.TargetKind, e.TargetSolidityRecorded, e.Strength, e.Context, e.Fraction))
	}
	for _, e := range st.ClaimEntries {
		for _, d := range slices.Concat(e.DependsOn, e.References) {
			emit(d)
		}
	}
	for _, s := range st.Supports {
		for _, d := range s.DependsOn {
			emit(d)
		}
		for _, p := range s.Supports {
			out = append(out, edgeRecord(s.ID, p.ClaimID, "supports", "claim", nil, nil, nil, p.Fraction))
		}
	}
	for _, x := range st.Experiments {
		for _, p := range x.Strengthens {
			out = append(out, edgeRecord(x.ID, p.ClaimID, "strengthens", "claim", nil, ptr(p.Strength), nil, kb.Fraction{}))
		}
	}
	sortRecords(out, func(r record) []string { return []string{str(r, 0), str(r, 1), str(r, 6)} })
	return out
}

func buildStrengthenBy(st kb.State) []record {
	var out []record
	for _, e := range st.ClaimEntries {
		for _, sb := range e.StrengthenBy {
			out = append(out, record{{"claim_id", sb.ClaimID}, {"item_idx", sb.ItemIdx}, {"text", sb.Text}, {"mentioned_ids", sb.MentionedIDs}})
		}
	}
	slices.SortStableFunc(out, func(a, b record) int {
		return cmp.Or(strings.Compare(str(a, 0), str(b, 0)), cmp.Compare(a[1].val.(int), b[1].val.(int)))
	})
	return out
}

func buildSupportedBy(st kb.State, sol Solidity) []record {
	var out []record
	for _, s := range st.Supports {
		for _, p := range s.Supports {
			out = append(out, record{{"claim_id", p.ClaimID}, {"sup_id", s.ID}, {"fraction", fractionValue(p.Fraction)},
				{"sup_solidity", optional(sol.SupSolidity[s.ID])}})
		}
	}
	sortRecords(out, func(r record) []string { return []string{str(r, 0), str(r, 1)} })
	return out
}

func buildCites(st kb.State) []record {
	var out []record
	for _, leaf := range st.Leaves {
		for _, c := range leaf.Claims {
			out = append(out, record{{"claim_id", c}, {"leaf_path", leaf.Path}, {"leaf_kind", leaf.Kind}, {"tier2_marked", leaf.Tier2Marked[c]}})
		}
	}
	sortRecords(out, func(r record) []string { return []string{str(r, 0), str(r, 1)} })
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

func buildSubtreeAggregates(st kb.State) []record {
	agg := SubtreeAggregates(st)
	var out []record
	for _, idx := range st.Indexes {
		a := agg[idx.Path]
		out = append(out, record{{"node_path", idx.Path}, {"node_kind", idx.Kind}, {"subtree_claims", a.Claims}, {"subtree_experiments", a.Experiments}})
	}
	sortRecords(out, func(r record) []string { return []string{str(r, 0)} })
	return out
}

// CoverageError is an edge naming a framework node or external work that no
// authored source stands up.
type CoverageError struct{ Msg string }

func (e CoverageError) Error() string { return e.Msg }

func checkCoverage(claims, dependsOn []record) error {
	frameworkPresent, workPresent := map[string]bool{}, map[string]bool{}
	for _, r := range claims {
		switch kind := str(r, 0); {
		case slices.Contains(kb.FrameworkKinds, kind):
			frameworkPresent[str(r, 1)] = true
		case kind == "work":
			workPresent[str(r, 1)] = true
		}
	}
	framework := map[string]bool{}
	works := map[string]bool{}
	for _, e := range dependsOn {
		switch kind, target := str(e, 3), str(e, 1); {
		case slices.Contains(kb.FrameworkKinds, kind) && !frameworkPresent[target]:
			framework[target] = true
		case kind == "work" && !workPresent[target]:
			works[target] = true
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
		"claims": claims, "depends-on": dependsOn, "strengthen-by": buildStrengthenBy(st),
		"supported-by": buildSupportedBy(st, sol), "cites": buildCites(st), "subtree-aggregates": buildSubtreeAggregates(st),
	}, nil
}

// Serialize is a file's JSONL text: one object per line, ", " and ": "
// separators, keys in built order, non-ASCII written raw, one trailing
// newline, and "" for no records.
func Serialize(recs []record) string {
	var b strings.Builder
	for _, r := range recs {
		b.WriteByte('{')
		for i, f := range r {
			if i > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, f.key)
			b.WriteString(": ")
			writeJSONValue(&b, f.val)
		}
		b.WriteString("}\n")
	}
	return b.String()
}

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
