// Package tree plan is pipeline stage 3's artifact and the deterministic
// verifier that produces it (ARCHITECTURE.md §4; TEMP_DESIGN_V020_RESHAPE §2,
// §3.3).
//
// The tree plan is the single source of tree truth (I-1): every path, title,
// parent edge, scope line and leaf→span assignment in the delivered tree is
// read from here. No later stage derives a path from a heading, a filename
// from a title, or a parent from a directory walk — two stages computing the
// same name is a DRY violation with a drift failure mode, and the exemplar
// demonstrates the drift.
//
// # Two artifacts, one boundary
//
// The tree plan owns which BYTES a leaf draws from; stage 4's cut list owns
// where inside them the boundaries fall. That is why a span lives on a SplitGroup
// and not on a Node: §2.4 mechanically splits an oversized span into n parts
// and stage 4 then MOVES the interior boundaries, so a per-leaf {start,end}
// would be stale on arrival. At group granularity the tree plan states only
// what it never revises — the span, the budget, and the part count.
// `parts == 1` is not a special case: an un-split leaf is a one-part group, so
// every leaf reads its bytes the same way.
//
// # Nothing here calls a model
//
// The package is entirely mechanical. It composes an answer the taxonomy stage
// collected (GroupingAnswer, TreeProposal) into a verified artifact, running dissect.Split
// itself to prove every leaf span is cuttable under budget. What it cannot
// discharge it REFUSES, and the refusal is typed:
//
//   - Rejection — a failure the model could be responsible for. The stage-3
//     seam turns it into the one informed retry's retry note (Note is the
//     model-facing rendering, and it carries no path and no offset).
//   - DefectError — a failure no legal answer could produce: a duplicate path
//     when the namer guarantees uniqueness, a group nobody names, a candidate
//     list longer than the cap the batching is supposed to enforce. The seam
//     maps it to pipeline.ErrVerifierDefect, which aborts rather than retries.
//
// This package deliberately does not import internal/pipeline: the
// classification belongs where the knowledge is, and the mapping to the
// runner's error vocabulary is one line at the seam that owns both.
package treeplan

import (
	"encoding/json"
	"fmt"
	"io"
)

// SchemaVersion identifies the artifact shape. Consumers (stages 4, 5, 6, 8
// and 9) check it rather than guessing from the fields present, and a shape
// change bumps it.
const SchemaVersion = "kbase.treeplan/1"

// Kind is what a node is. The set is closed: three shapes, and the grammar of
// each is fixed (§4).
type Kind string

const (
	// KindEntryPoint is the corpus root — an index with no up-link. Exactly
	// one node carries it, and it is the only node with no parent.
	KindEntryPoint Kind = "entry-point"
	// KindIndex is a routing node: framing, a conclusions block, and the
	// down-link list its children's titles and scopes are rendered from.
	KindIndex Kind = "index"
	// KindLeaf is a verbatim slice of one source file, named by a SplitGroup.
	KindLeaf Kind = "leaf"
)

// TreePlan is one job's whole tree plan.
//
// Field order is the marshal order, nodes are depth-first with parents before
// children, and nothing here is a map — the artifact is byte-identical for
// identical input, which is what makes stage 3 resumable and stage 9's
// conformance check meaningful.
type TreePlan struct {
	Schema string `json:"schema"`
	// CorpusHash is survey.Totals.ContentHash: the identity of the bytes this
	// tree plans. It is the artifact's only provenance.
	CorpusHash string `json:"corpusHash"`
	// Budgets is what the tree was verified against. It travels with the
	// artifact because a later stage's own boundedness proof (G-2, the
	// entry-point ceiling) is stated in these numbers, and re-deriving them
	// from a package constant is the second source I-1 exists to forbid.
	Budgets Budgets      `json:"budgets"`
	Nodes   []Node       `json:"nodes"`
	Groups  []SplitGroup `json:"groups"`
	Annexes []Annex      `json:"annexes,omitempty"`
	// CrossParentMerges is how many content-floor merges re-homed a sub-floor
	// span under an index other than the one its own page sat under
	// (Verifier.floor). It travels with the artifact because nothing else can
	// recover it: composition is where the event happens, a resumed run reuses
	// this artifact rather than re-composing, and no gate can see the repair
	// afterwards. Omitted when there were none — the ordinary case, and an
	// explicit zero would read as a measurement where it is an absence.
	CrossParentMerges int `json:"crossParentMerges,omitempty"`
}

// Node is one delivered file.
//
// Path is store- and KB-relative and slash-separated: it is the same string
// under temp-work/tree/, under leaves/, and in the delivered output, so a node
// has one name everywhere. Title is the node's H1 verbatim as rendered — no
// stage re-derives it, and no stage types it twice.
type Node struct {
	Path   string `json:"path"`
	Kind   Kind   `json:"kind"`
	Parent string `json:"parent"`
	Title  string `json:"title"`
	// Scope is the one-line description a parent's down-link renders after
	// the title. It is authored once, here, and never re-derived by stage 6:
	// routing survives a worthless summary because routing is not in the
	// summary (I-4).
	Scope string `json:"scope"`
	// SplitGroup is the id of the SplitGroup whose span this leaf draws from; empty on
	// an index or the entry-point.
	SplitGroup string `json:"group,omitempty"`
	// Part is k in 1..SplitGroup.Parts — which part of that span this leaf is.
	// Zero on an index or the entry-point.
	Part int `json:"part,omitempty"`
}

// SplitGroup is one leaf span: the bytes, the budget they were sized against, and
// how many leaves they become.
//
// It is the whole of what the tree plan claims about a split. The positions of
// the cuts INSIDE a multi-part group are stage 4's business and the cut list
// is their sole statement — the schema carries no offset a later stage may
// move, so I-1 holds for split leaves as written rather than by exception.
type SplitGroup struct {
	ID     string `json:"id"`
	Source Span   `json:"source"`
	Budget int    `json:"budget"`
	// Parts is n ≥ 1. n == 1 means the span is one leaf and is never cut.
	Parts int `json:"parts"`
}

// Span is a half-open byte range into one ingested corpus file. It is
// single-file by construction: a group over two files could never be one leaf,
// which is what §2.7's dissolution operator exists to repair.
//
// It is NOT survey.Span: this one names the file it points into, and
// survey.Span does not (it is always read against a file already in hand).
type Span struct {
	File  string `json:"file"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// Annex is a corpus territory deliberately not distilled (SPEC §3).
//
// Prefixes are declared by configuration only (O-12): a model that can place
// material out of scope can hide its own failures. Convention is authored
// mechanically from the survey (§2.8) — the path grammar, the dominant file
// extension, and one worked example — so it is re-derivable and provable at
// stage 9 like everything else.
type Annex struct {
	Prefix     string `json:"prefix"`
	Convention string `json:"convention"`
}

// WriteJSON writes the artifact as indented JSON with a trailing newline.
//
// The discipline is the survey artifact's, for the same reason: HTML escaping
// off, because titles and scopes come from documents and legitimately contain
// `<` and `&`; fixed field order; no map on the marshal path. Identical input
// produces identical bytes.
func (s TreePlan) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return fmt.Errorf("treeplan: write artifact: %w", err)
	}
	return nil
}

// ReadJSON decodes an artifact and refuses one this build does not understand.
//
// A decoder is not optional here: stages 4, 5, 6, 8 and 9 all consume the
// tree plan, and every one of them reads it back off disk after a resume. The
// schema check and the unknown-field refusal are the boundary contract — a
// field this build has never heard of means the artifact was written by
// another build, and silently ignoring it is how a stale tree plan gets
// delivered.
func ReadJSON(r io.Reader) (TreePlan, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var s TreePlan
	if err := dec.Decode(&s); err != nil {
		return TreePlan{}, fmt.Errorf("treeplan: read artifact: %w", err)
	}
	if s.Schema != SchemaVersion {
		return TreePlan{}, fmt.Errorf("treeplan: artifact declares schema %q, this build reads %q",
			s.Schema, SchemaVersion)
	}
	return s, nil
}

// Node returns the node at path.
func (s TreePlan) Node(path string) (Node, bool) {
	for _, n := range s.Nodes {
		if n.Path == path {
			return n, true
		}
	}
	return Node{}, false
}

// SplitGroup returns the group with id.
func (s TreePlan) SplitGroup(id string) (SplitGroup, bool) {
	for _, g := range s.Groups {
		if g.ID == id {
			return g, true
		}
	}
	return SplitGroup{}, false
}
