package write

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// recordEdit is an op over the records, applying one values entry at a time to
// the batch. A field op writes one field of one record per entry: field is the
// values key carrying the value, every other key naming the record; every
// other op has no field. prior refuses a batch before any entry applies, and
// finish once every entry has. A machine op writes as a run, whatever
// provenance its caller names.
type recordEdit struct {
	field   string
	apply   func(b *recordBatch, e entry) error
	prior   func(entries []entry) error
	finish  func(b *recordBatch, entries []entry) error
	machine bool
}

// recordBatch is one op's records as its entries edit them, each rigor and
// bearing written with the manually-scored flag provenance decides; hosts is
// each hosted node's leaf, and leaves each leaf the batch marks, by
// kb-root-relative path. created is the entry that entered each keyed node —
// a work, an axiom — in this batch. ids, minted and adopted are an insert's
// report, and resolved resolve-demoted's.
type recordBatch struct {
	src        *kb.Source
	root       string
	g          kb.Graph
	provenance Provenance
	hosts      map[string]string
	documents  []string
	leaves     map[string]*leafEdit
	leafOrder  []string
	depends    index.DependsGraph
	created    map[string]int
	ids        []string
	minted     []string
	adopted    []Adoption
	resolved   []Resolution
}

// leafEdit is one leaf the batch marks: its file, the bytes the disk held and
// the text the batch read when it first opened it, the text composed since,
// and the node ids it marked.
type leafEdit struct {
	target     string
	disk       *string
	read, text string
	marked     []string
}

// conflicts refuses each entry naming the record an earlier entry names with
// a value that differs, compared collapsed; the same value twice is no
// conflict. An insert names no record by its values and is not judged.
func (r *recordEdit) conflicts(entries []entry) error {
	if r.field == "" {
		return nil
	}
	type named struct {
		entry int
		value string
	}
	first := map[string]named{}
	var refusals result.Refusal
	for _, e := range entries {
		var keys []string
		for _, k := range slices.Sorted(maps.Keys(e.Values)) {
			if k != r.field {
				keys = append(keys, k+"="+e.str(k))
			}
		}
		identity := strings.Join(keys, ", ")
		value := collapse(e.str(r.field))
		if score, ok := e.Values[r.field].(*float64); ok {
			value = formatScore(score)
		}
		earlier, seen := first[identity]
		if !seen {
			first[identity] = named{e.Index, value}
			continue
		}
		if earlier.value != value {
			refusals = append(refusals, result.Item{Key: r.field, Entry: e.Index,
				Detail: fmt.Sprintf("entry %d names the record entry %d names (%s) with another %s, %s against %s; a batch writes one value to a field",
					e.Index, earlier.entry, identity, r.field, value, earlier.value)})
		}
	}
	if len(refusals) > 0 {
		return refusals
	}
	return nil
}

// degradedRefusal is the refusal of a write over a KB whose load, rows, is
// degraded, one item per fault the load resolved for the walk, and false
// where the load is not degraded.
func degradedRefusal(rows index.Rows) (Result, bool) {
	items := rows.DegradedItems()
	if items == nil {
		return Result{}, false
	}
	return Result{Outcome: result.Refused, Refusals: items}, true
}

// recordFile is one record file, kb-root-relative, and its bytes.
type recordFile struct {
	rel  string
	data []byte
}

// recordFiles is g's two record files as kb renders them, the edge file first:
// the order they are written in.
func recordFiles(g kb.Graph) []recordFile {
	return []recordFile{{kb.GraphEdgesFile, kb.EncodeEdges(g)}, {kb.GraphNodesFile, kb.EncodeNodes(g)}}
}

// editRecords applies every entry in turn to the records as their files hold
// them and to the leaves it marks, hosts being each hosted node's leaf, then
// writes each record file whose records moved, whole, and each leaf whose text
// moved, once each marker reads back where it was placed and the disk still
// holds what was read. A file that did not move is not written. An entry's
// refusal does not stop the batch: each later entry is judged against the
// records as the entries before it left them, and the batch is refused with
// every entry's items, writing nothing. Any other error stops it at once. The
// batch is returned once it is opened, with what its entries reported.
func editRecords(src *kb.Source, edit *recordEdit, entries []entry, provenance Provenance, hosts map[string]string) (outcome, *recordBatch, error) {
	if err := edit.conflicts(entries); err != nil {
		return outcome{}, nil, err
	}
	if edit.prior != nil {
		if err := edit.prior(entries); err != nil {
			return outcome{}, nil, err
		}
	}
	if edit.machine {
		provenance = ProvenanceMachine
	}
	read := map[string]*string{}
	for _, rel := range []string{kb.GraphNodesFile, kb.GraphEdgesFile} {
		disk, err := onDisk(src, src.KBPath(rel))
		if err != nil {
			return outcome{}, nil, err
		}
		read[rel] = disk
	}
	g, err := kb.LoadAuthoredGraph(src)
	if err != nil {
		return outcome{}, nil, err
	}
	b := &recordBatch{src: src, root: kb.ResolvePath(src.Root()), g: g, provenance: provenance, hosts: maps.Clone(hosts),
		leaves: map[string]*leafEdit{}, created: map[string]int{}}
	before := recordFiles(g)
	var refusals result.Refusal
	for _, e := range entries {
		err := edit.apply(b, e)
		if err == nil {
			continue
		}
		items, refused := entryRefusal(err, e.Index)
		if !refused {
			return outcome{}, b, err
		}
		refusals = append(refusals, items...)
	}
	if len(refusals) > 0 {
		return outcome{}, b, refusals
	}
	if edit.finish != nil {
		if err := edit.finish(b, entries); err != nil {
			return outcome{}, b, err
		}
	}
	after := recordFiles(b.g)
	if err := readBackMarkers(b); err != nil {
		return outcome{}, b, err
	}
	var moved []recordFile
	for i, f := range after {
		if !bytes.Equal(f.data, before[i].data) {
			moved = append(moved, f)
		}
	}
	// On a migrated KB neither record file on disk is in this form, and the
	// next conversion takes both as they stand once the node file does: both
	// are written, the node file last.
	if src.Migrated() && len(moved) > 0 {
		moved = after
	}
	type change struct {
		subject, target string
		disk            *string
		data            []byte
	}
	var changes []change
	for _, f := range moved {
		changes = append(changes, change{f.rel, src.KBPath(f.rel), read[f.rel], f.data})
	}
	for _, subject := range b.leafOrder {
		if l := b.leaves[subject]; l.text != l.read {
			changes = append(changes, change{subject, l.target, l.disk, []byte(l.text)})
		}
	}
	for _, c := range changes {
		if !stillMatches(src, c.target, c.disk) {
			return outcome{status: statusRetry, reason: reasonContended, subject: c.subject,
				detail: "changed between the read and the replace; a concurrent writer intervened"}, b, nil
		}
	}
	var written []string
	for _, c := range changes {
		if err := index.WriteKBFile(src, c.target, c.data); err != nil {
			return outcome{}, b, &interruptedError{written: written, failed: c.subject, cause: err}
		}
		written = append(written, c.subject)
	}
	if len(written) == 0 {
		return outcome{status: statusUnchanged}, b, nil
	}
	return outcome{status: statusWritten, written: written}, b, nil
}

// readBackMarkers refuses a leaf whose composed text does not carry the
// statement marker of a node the batch marked there.
func readBackMarkers(b *recordBatch) error {
	for _, subject := range b.leafOrder {
		l := b.leaves[subject]
		lines := kb.MarkerLines(l.text)
		for _, id := range l.marked {
			if _, ok := lines[id]; !ok {
				return &readbackFailed{subject: subject + ":" + id, mismatched: []string{"claim-rigor marker"}}
			}
		}
	}
	return nil
}

// entryRefusal is err's refusal items, each that names no entry naming the
// entry at index, and false where err is not a refusal.
func entryRefusal(err error, index int) (result.Refusal, bool) {
	var refused result.Refusal
	if !errors.As(err, &refused) {
		return nil, false
	}
	items := slices.Clone(refused)
	for i := range items {
		if items[i].Entry == 0 {
			items[i].Entry = index
		}
	}
	return items, true
}

// record is the first of records match holds, nil where none does.
func record[T any](records []T, match func(T) bool) *T {
	if i := slices.IndexFunc(records, match); i >= 0 {
		return &records[i]
	}
	return nil
}

// noRecord refuses a node id no record of the kinds an op writes carries;
// check is kb.CheckUnknownID where no record of any kind carries it.
func noRecord(check, key, id string) error {
	return result.Refusal{{Check: check, Key: key + "=" + id, Remedy: "correct " + key + ", or insert the node first, then re-run",
		Detail: "names no node record this op writes, and the op writes a record that stands; it never creates one"}}
}

// setRigor writes a node's rigor, and on a kind that carries the flag whether
// it is manually scored.
func setRigor(b *recordBatch, e entry) error {
	g := &b.g
	id, rigor, manual := e.str("id"), e.score("rigor"), b.provenance == ProvenanceCaller
	if n := record(g.Claims, func(n kb.ClaimNode) bool { return n.ID == id }); n != nil {
		n.Rigor, n.ManuallyScored = rigor, manual
		return nil
	}
	if n := record(g.Analyses, func(n kb.AnalysisNode) bool { return n.ID == id }); n != nil {
		n.Rigor, n.ManuallyScored = rigor, manual
		return nil
	}
	if n := record(g.Experiments, func(n kb.ExperimentNode) bool { return n.ID == id }); n != nil {
		n.Rigor, n.ManuallyScored = rigor, manual
		return nil
	}
	if n := record(g.Works, func(n kb.WorkNode) bool { return n.ID == id }); n != nil {
		n.Rigor = rigor
		return nil
	}
	return noRecord("", "id", id)
}

// setRationale writes a claim's, an analysis's or a work's rationale.
func setRationale(b *recordBatch, e entry) error {
	g := &b.g
	id, rationale := e.str("id"), collapse(e.str("rationale"))
	if n := record(g.Claims, func(n kb.ClaimNode) bool { return n.ID == id }); n != nil {
		n.Rationale = rationale
		return nil
	}
	if n := record(g.Analyses, func(n kb.AnalysisNode) bool { return n.ID == id }); n != nil {
		n.Rationale = rationale
		return nil
	}
	if n := record(g.Works, func(n kb.WorkNode) bool { return n.ID == id }); n != nil {
		n.Rationale = rationale
		return nil
	}
	return noRecord("", "id", id)
}

// setStatus writes an experiment's status.
func setStatus(b *recordBatch, e entry) error {
	g := &b.g
	id := e.str("id")
	if n := record(g.Experiments, func(n kb.ExperimentNode) bool { return n.ID == id }); n != nil {
		n.Status = e.str("status")
		return nil
	}
	return noRecord("", "id", id)
}

// setBearing writes an edge record's bearing, and on a relation that carries
// the flag whether it is manually scored. It never creates an edge, and
// refuses one a demoted record stands in for.
func setBearing(b *recordBatch, e entry) error {
	g := &b.g
	source, target, relation := e.str("id"), e.str("target"), e.str("relation")
	bearing := bearingOf(e.score("bearing"))
	sourceKind, _ := kb.NodeIDKind(source)
	targetKind, _ := kb.NodeIDKind(target)
	if !kb.BearingLegal(relation, sourceKind, targetKind) {
		return opRefusal("relation="+relation, fmt.Sprintf("does not join a %s source to a %s target: no bearing "+
			"judges that pairing", sourceKind, targetKind),
			"restore: correct relation or the ids, then re-run")
	}
	ends := func(s, t string) bool { return s == source && t == target }
	if relation == kb.RelationFounds {
		if r := record(g.Founds, func(r kb.BearingEdge) bool { return ends(r.Source, r.Target) }); r != nil {
			r.Bearing = bearing
			return nil
		}
	} else if r := record(g.Scorable, func(r kb.ScorableEdge) bool { return r.Relation == relation && ends(r.Source, r.Target) }); r != nil {
		r.Bearing, r.ManuallyScored = bearing, b.provenance == ProvenanceCaller
		return nil
	}
	name := index.EdgeName(relation, source, target)
	if slices.ContainsFunc(g.Demoted, func(r kb.DemotedEdge) bool { return r.ReplacedRelation == relation && ends(r.Source, r.Target) }) {
		return result.Refusal{{Check: index.CheckDemoted, Path: kb.GraphEdgesFile, Remedy: "restore it with resolve-demoted, then re-run",
			Detail: name + " is demoted, and a demoted record's bearing is the replaced edge's, carried until it is restored"}}
	}
	return opRefusal("id="+source, name+" has no edge record, and this op re-scores an edge; it never creates one",
		"restore: add the edge first, or correct id, target or relation, then re-run")
}
