package format300

import (
	"fmt"
	"slices"
	"strings"
)

// StreamUnsafe escapes what a string carries raw but a reader of a KB YAML
// file takes as a line break or refuses as unprintable: U+2028 and U+2029, DEL
// and the C1 controls, NEL among them, the byte-order mark, and the
// noncharacters U+FFFE and U+FFFF. Every one can stand only inside a string,
// where the escape reads back as the character.
var StreamUnsafe = func() *strings.Replacer {
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

// QuoteRecordString is s as a record file writes a string: double-quoted,
// escaping \, ", every control character below U+0020 but tab, and what
// StreamUnsafe lists.
func QuoteRecordString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\' || r == '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 && r != '\t':
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return StreamUnsafe.Replace(b.String())
}

// recordWriter renders one record: its first key opens the list item, the
// rest stand under it.
type recordWriter struct {
	b    strings.Builder
	keys int
}

func (w *recordWriter) key(k string) {
	if w.keys == 0 {
		w.b.WriteString("- ")
	} else {
		w.b.WriteString("  ")
	}
	w.keys++
	w.b.WriteString(k + ":")
}

func (w *recordWriter) scalar(k, text string) {
	w.key(k)
	w.b.WriteString(" " + text + "\n")
}

func (w *recordWriter) str(k, v string) { w.scalar(k, QuoteRecordString(v)) }

func (w *recordWriter) boolean(k string, v bool) { w.scalar(k, fmt.Sprint(v)) }

func (w *recordWriter) rigor(v *float64) {
	if v == nil {
		w.str(RigorKey, PendingLiteral)
		return
	}
	w.scalar(RigorKey, PyFloatRepr(*v))
}

func (w *recordWriter) bearing(b Bearing) {
	if b.Pending {
		w.str(BearingKey, PendingLiteral)
		return
	}
	w.scalar(BearingKey, PyFloatRepr(b.Value))
}

func (w *recordWriter) list(k string, items []string) {
	if len(items) == 0 {
		w.scalar(k, "[]")
		return
	}
	w.key(k)
	w.b.WriteString("\n")
	for _, it := range items {
		w.b.WriteString("    - " + QuoteRecordString(it) + "\n")
	}
}

// keyedRecord is one record's text and the key its file orders it by.
type keyedRecord struct {
	sortKey []string
	text    string
}

func renderRecord(sortKey []string, write func(w *recordWriter)) keyedRecord {
	var w recordWriter
	write(&w)
	return keyedRecord{sortKey, w.b.String()}
}

func joinRecords(recs []keyedRecord) []byte {
	if len(recs) == 0 {
		return []byte("[]\n")
	}
	slices.SortStableFunc(recs, func(a, b keyedRecord) int { return slices.Compare(a.sortKey, b.sortKey) })
	var b strings.Builder
	for _, r := range recs {
		b.WriteString(r.text)
	}
	return []byte(b.String())
}

// EncodeNodes is the node file's bytes for g: every node record in id byte
// order, keys in its type's field order.
func EncodeNodes(g Graph) []byte {
	var recs []keyedRecord
	for _, n := range g.Claims {
		recs = append(recs, renderRecord([]string{n.ID}, func(w *recordWriter) {
			w.str(fieldID, n.ID)
			w.str(fieldTitle, n.Title)
			w.rigor(n.Rigor)
			w.boolean(ManuallyScoredKey, n.ManuallyScored)
			w.str(fieldRationale, n.Rationale)
			w.str(fieldNoEdge, n.NoEdge)
			w.list(fieldStrengthenBy, n.StrengthenBy)
		}))
	}
	for _, n := range g.Analyses {
		recs = append(recs, renderRecord([]string{n.ID}, func(w *recordWriter) {
			w.str(fieldID, n.ID)
			w.str(fieldTitle, n.Title)
			w.rigor(n.Rigor)
			w.boolean(ManuallyScoredKey, n.ManuallyScored)
			w.str(fieldRationale, n.Rationale)
			w.str(fieldNoEdge, n.NoEdge)
		}))
	}
	for _, n := range g.Experiments {
		recs = append(recs, renderRecord([]string{n.ID}, func(w *recordWriter) {
			w.str(fieldID, n.ID)
			w.str(fieldTitle, n.Title)
			w.str(StatusKey, n.Status)
			w.rigor(n.Rigor)
			w.boolean(ManuallyScoredKey, n.ManuallyScored)
		}))
	}
	for _, n := range g.Works {
		recs = append(recs, renderRecord([]string{n.ID}, func(w *recordWriter) {
			w.str(fieldID, n.ID)
			w.str(fieldTitle, n.Title)
			w.rigor(n.Rigor)
			w.str(fieldRationale, n.Rationale)
		}))
	}
	for _, n := range g.Axioms {
		recs = append(recs, renderRecord([]string{n.ID}, func(w *recordWriter) {
			w.str(fieldID, n.ID)
			w.str(fieldTitle, n.Title)
		}))
	}
	return joinRecords(recs)
}

// relationRank is relation's place in Relations, spelled to sort as text.
func relationRank(relation string) string {
	return fmt.Sprintf("%02d", slices.Index(Relations, relation))
}

// EdgeKey is an edge record's identity as the edge file orders it: relation,
// a demoted record's replaced relation ("" for any other), source, target.
// slices.Compare over two keys is the identity order.
func EdgeKey(relation, replaced, source, target string) []string {
	if replaced != "" {
		replaced = relationRank(replaced)
	}
	return []string{relationRank(relation), replaced, source, target}
}

// EncodeEdges is the edge file's bytes for g: every edge record by identity —
// relation in Relations order, a demoted record's replaced relation in the
// same order, then source, then target — keys in its type's field order.
func EncodeEdges(g Graph) []byte {
	var recs []keyedRecord
	for _, e := range g.Edges() {
		recs = append(recs, renderRecord(nil, func(w *recordWriter) {
			w.str(fieldRelation, e.Relation)
			w.str(fieldSource, e.Source)
			w.str(fieldTarget, e.Target)
			w.str(fieldContext, e.Context)
			switch e.Relation {
			case RelationReferences:
			case RelationDemoted:
				w.str(fieldReplacedRelation, e.ReplacedRelation)
				w.bearing(*e.Bearing)
				w.boolean(ManuallyScoredKey, e.ManuallyScored)
			case RelationFounds:
				w.bearing(*e.Bearing)
			default:
				w.bearing(*e.Bearing)
				w.boolean(ManuallyScoredKey, e.ManuallyScored)
			}
		}))
	}
	return joinRecords(recs)
}
