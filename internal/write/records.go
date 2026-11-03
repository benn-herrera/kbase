package write

import (
	"bytes"
	"errors"
	"maps"
	"slices"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// recordEdit is an op over the records, applying one values entry at a time to
// the batch. prior refuses the op's entries before any entry of the batch
// applies. A machine op writes as a run, whatever provenance its caller names.
type recordEdit struct {
	apply   func(b *recordBatch, e entry) error
	prior   func(entries []entry) error
	machine bool
}

// recordBatch is one op's records as its entries edit them, each rigor and
// bearing written with the manually-scored flag provenance decides; hosts is
// each hosted node's leaf, and leaves each document the batch edits, by
// kb-root-relative path. report is what the entry applying now minted or
// edited, and valued the scored records it set a value on.
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
	report     entryReport
	valued     []scoredRecord
}

// entryReport is the node an entry minted or edited: its id; or, of a
// restore-demoted entry, the relations it put back; or, of a remove-node entry, the edge records it
// removed; or, of an add-citation entry, the node it cites, the citing
// document and 1-based line, and the link written there. An add-axiom entry's
// Heading is how its heading came to stand, and an edit-work entry's
// Rewritten the lines its label change rewrote. The zero value is an entry
// that named no node.
type entryReport struct {
	ID           string
	Heading      string
	Restored     []string
	RemovedEdges []kb.EdgeRow
	Document     string
	Line         int
	Link         string
	Rewritten    []Rewritten
}

// leafEdit is one document the batch edits — a leaf it marks, or axioms.md:
// its file, the bytes the disk held (nil where it stood absent) and the text
// the batch read when it first opened it, the text composed since, and the
// node ids it marked.
type leafEdit struct {
	target     string
	disk       *string
	read, text string
	marked     []string
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

// change is one file a batch replaces: its kb-root-relative subject, its path,
// the bytes the disk held when the batch read it (nil where it stood absent),
// and its new bytes.
type change struct {
	subject, target string
	disk            *string
	data            []byte
}

// applied is a batch's entries applied in memory: each entry's report, in
// order, and the record files and leaves whose bytes moved, the edge file,
// then the node file, then the leaves in the order the batch opened them.
type applied struct {
	entries []EntryResult
	changes []change
}

// applyEntries applies every entry in turn, in memory, to the records as their
// files hold them and to the leaves it marks, hosts being each hosted node's
// leaf, each entry judged against the records as the entries before it left
// them. The first refusal stops the batch and is returned, its items naming
// the entry; so is any other error. A file that did not move is no change, and
// each marker the batch placed must read back where it was placed.
func applyEntries(src *kb.Source, entries []entry, provenance Provenance, hosts map[string]string) (applied, error) {
	read := map[string]*string{}
	for _, rel := range []string{kb.GraphNodesFile, kb.GraphEdgesFile} {
		disk, err := onDisk(src, src.KBPath(rel))
		if err != nil {
			return applied{}, err
		}
		read[rel] = disk
	}
	g, err := kb.LoadGraph(src)
	if err != nil {
		return applied{}, err
	}
	b := &recordBatch{src: src, root: kb.ResolvePath(src.Root()), g: g, hosts: maps.Clone(hosts),
		leaves: map[string]*leafEdit{}}
	before := recordFiles(g)
	prev, prevLeaves := before, map[string]string{}
	var out applied
	for _, e := range entries {
		edit := writeOps[e.Op].edit
		b.provenance, b.report = provenance, entryReport{}
		if edit.machine {
			b.provenance = ProvenanceMachine
		}
		err := edit.apply(b, e)
		if err == nil {
			err = b.hashValuesSet()
		}
		if err != nil {
			if items, refused := entryRefusal(err, e.Index); refused {
				return applied{}, items
			}
			return applied{}, err
		}
		cur := recordFiles(b.g)
		outcome := result.Unchanged
		if !sameFiles(prev, cur) || leavesMoved(b, prevLeaves) {
			outcome = result.Done
		}
		out.entries = append(out.entries, EntryResult{Entry: e.Index, Operation: e.Op, Outcome: outcome, ID: b.report.ID, Heading: b.report.Heading,
			Restored: b.report.Restored, RemovedEdges: b.report.RemovedEdges, Document: b.report.Document, Line: b.report.Line, Link: b.report.Link,
			Rewritten: b.report.Rewritten})
		prev = cur
	}
	if err := readBackMarkers(b); err != nil {
		return applied{}, err
	}
	for i, f := range prev {
		if !bytes.Equal(f.data, before[i].data) {
			out.changes = append(out.changes, change{f.rel, src.KBPath(f.rel), read[f.rel], f.data})
		}
	}
	for _, subject := range b.leafOrder {
		if l := b.leaves[subject]; l.text != l.read {
			out.changes = append(out.changes, change{subject, l.target, l.disk, []byte(l.text)})
		}
	}
	return out, nil
}

// sameFiles is whether two renderings of the record files hold the same
// bytes.
func sameFiles(a, b []recordFile) bool {
	return slices.EqualFunc(a, b, func(x, y recordFile) bool { return bytes.Equal(x.data, y.data) })
}

// leavesMoved is whether any leaf the batch opened holds text other than
// seen records for it, recording each leaf's text in seen.
func leavesMoved(b *recordBatch, seen map[string]string) bool {
	moved := false
	for _, subject := range b.leafOrder {
		l := b.leaves[subject]
		last, ok := seen[subject]
		if !ok {
			last = l.read
		}
		if l.text != last {
			moved = true
		}
		seen[subject] = l.text
	}
	return moved
}

// readBackMarkers refuses a leaf whose composed text does not carry the
// statement marker of a node the batch marked there.
func readBackMarkers(b *recordBatch) error {
	for _, subject := range b.leafOrder {
		l := b.leaves[subject]
		lines := kb.MarkerLines(l.text)
		for _, id := range l.marked {
			if _, ok := lines[id]; !ok {
				return &readbackFailed{subject: subject + ":" + id, mismatched: []string{"statement marker"}}
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
	return result.Refusal{{Check: check, Key: key + "=" + id, Remedy: "correct " + key + ", or add the node first, then re-run",
		Detail: "names no node record this op writes, and the op writes a record that stands; it never creates one"}}
}
