package format_2_0_0

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/migrate/common"
)

// State is the KB's authored metadata after one load.
type State struct {
	ClaimEntries []ClaimEntry
	Leaves       []LeafRecord
	Indexes      []IndexRecord
	Axioms       []AxiomNode
	Experiments  []ExperimentNode
	Analyses     []AnalysisNode
	Works        []ExternalWork
}

// comparePathParts orders slash paths the way Python orders Paths: segment
// by segment.
func comparePathParts(a, b string) int {
	return slices.Compare(strings.Split(a, "/"), strings.Split(b, "/"))
}

// SymlinkError is a KB document that is a symbolic link: a load refuses it
// rather than reading through it.
type SymlinkError struct{ Path string }

func (e *SymlinkError) Error() string {
	return fmt.Sprintf("%s is a symbolic link, and a KB document is read only where it stands", e.Path)
}

// Files is every regular file of the KB whose name match accepts and none of
// whose directories below kb-root is excluded, as kb-root-relative slash
// paths in Python's Path order. A file match accepts that is a symbolic link
// is refused rather than read through.
func Files(src common.Files, match func(name string) bool) ([]string, error) {
	root := KBDir
	var out []string
	err := walkFiles(src, root, func(name string) bool { return ExcludeDirs[name] }, func(p string) error {
		if !match(filepath.Base(p)) || !hasFile(src, p) {
			return nil
		}
		if false {
			return &SymlinkError{Path: p}
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	slices.SortFunc(out, comparePathParts)
	return out, err
}

// Documents is every document of the KB — every .md file outside the
// excluded directories and names — as kb-root-relative slash paths.
func Documents(src common.Files) ([]string, error) {
	return Files(src, func(name string) bool {
		return strings.HasSuffix(name, ".md") && !ExcludeNames[name]
	})
}

// Registers is every claim-rigor register outside the excluded
// directories, as kb-root-relative slash paths.
func Registers(src common.Files) ([]string, error) {
	return Files(src, func(name string) bool { return name == RegisterFile })
}

func readRel(src common.Files, rel string) (string, error) {
	return readText(src, path.Join(KBDir, rel))
}

// KnownClaimIDs is every clm- id a register's canonical marker keys.
func KnownClaimIDs(src common.Files) (map[string]bool, error) {
	regs, err := Registers(src)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, rel := range regs {
		text, err := readRel(src, rel)
		if err != nil {
			return nil, err
		}
		for _, id := range CanonicalClaimIDs(text) {
			known[id] = true
		}
	}
	return known, nil
}

// KnownAxiomIDs is every axiom id the KB's axiom source declares.
func KnownAxiomIDs(src common.Files) (map[string]bool, error) {
	axioms, err := ParseAxioms(src)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, a := range axioms {
		known[a.ID] = true
	}
	return known, nil
}

// AnnotationFault is one malformed register bullet: the list field it was read
// from, the file and physical line it opens on, and what the reader refused.
type AnnotationFault struct {
	Field   string
	Path    string
	Line    int
	Allowed string
	Remedy  string
	Detail  string
}

// AnnotationError is every annotation fault one load found, in register order.
type AnnotationError struct{ Faults []AnnotationFault }

func (e *AnnotationError) Error() string {
	parts := make([]string, len(e.Faults))
	for i, f := range e.Faults {
		parts[i] = fmt.Sprintf("%s:%d: %s: %s", f.Path, f.Line, f.Field, f.Detail)
	}
	return strings.Join(parts, "; ")
}

// Discover loads the KB's authored metadata: every register's entries, every
// document's node bodies, and the axioms its declaration document states. A
// clm-, ana-, exp- or "Axiom <label>" token naming no node is dropped and
// reported to lg. A MalformedError names metadata no reader can take, and one
// AnnotationError carries every malformed scored bullet or pair line.
func Discover(src common.Files, lg func(path string, line int, detail string)) (State, error) {
	known, err := KnownClaimIDs(src)
	if err != nil {
		return State{}, err
	}
	axiomIDs, err := KnownAxiomIDs(src)
	if err != nil {
		return State{}, err
	}
	authored, _, err := AuthoredIDs(src)
	if err != nil {
		return State{}, err
	}
	regs, err := Registers(src)
	if err != nil {
		return State{}, err
	}
	var st State
	var faults []AnnotationFault
	analysisQ := map[string]AnalysisEntry{}
	for _, rel := range regs {
		text, err := readRel(src, rel)
		if err != nil {
			return State{}, err
		}
		st.Works = append(st.Works, ParseWorkEntries(text, rel)...)
		sink := readSink{faults: &faults}
		st.ClaimEntries = append(st.ClaimEntries, parseClaimEntries(text, rel, known, axiomIDs, authored, lg, sink)...)
		_, aq, err := parseAnalysisEntries(text, rel, known, axiomIDs, authored, lg, sink)
		if err != nil {
			return State{}, err
		}
		for id, q := range aq {
			analysisQ[id] = q
		}
	}
	if len(faults) > 0 {
		return State{}, &AnnotationError{Faults: faults}
	}
	docs, err := Documents(src)
	if err != nil {
		return State{}, err
	}
	var hosted []AnalysisNode
	for _, rel := range docs {
		text, err := readRel(src, rel)
		if err != nil {
			return State{}, err
		}
		leaf, err := ParseLeaf(text, rel)
		if err != nil {
			return State{}, err
		}
		if leaf != nil {
			st.Leaves = append(st.Leaves, *leaf)
		}
		exps, err := parseExperimentLeaf(text, rel, &faults)
		if err != nil {
			return State{}, err
		}
		st.Experiments = append(st.Experiments, exps...)
		anas, err := parseAnalysisLeaf(text, rel, &faults)
		if err != nil {
			return State{}, err
		}
		hosted = append(hosted, anas...)
		if leaf == nil && len(exps) == 0 && len(anas) == 0 {
			idx, err := parseIndex(text, rel)
			if err != nil {
				return State{}, err
			}
			if idx != nil {
				st.Indexes = append(st.Indexes, *idx)
			}
		}
	}
	if len(faults) > 0 {
		return State{}, &AnnotationError{Faults: faults}
	}
	for _, a := range hosted {
		q := analysisQ[a.ID]
		a.Rigor, a.Premises, a.Rationale, a.Strength, a.StrengthTrace = q.Rigor, q.Premises, q.Rationale, q.Strength, q.StrengthTrace
		a.Lifts, a.Refutes = unionPairs(a.Lifts, q.Lifts), unionPairs(a.Refutes, q.Refutes)
		st.Analyses = append(st.Analyses, a)
	}
	if st.Axioms, err = ParseAxioms(src); err != nil {
		return State{}, err
	}
	st.readAuthoredRigor()
	slices.SortStableFunc(st.Works, func(a, b ExternalWork) int { return strings.Compare(a.ID, b.ID) })
	return st, nil
}

// readAuthoredRigor is every authored rigor as the system reads it: State is
// where a document's numbers become the system's, so a value standing above
// RigorMaximum becomes RigorMaximum here, for every node kind at once. The
// readers stay document readers — the bytes a file holds are what they return,
// which is how the write path leaves a rigor it found as it is — and an
// authored value at the maximum, an axiom's 1.00 included, is nothing this
// rule touches.
func (st *State) readAuthoredRigor() {
	for i := range st.Works {
		st.Works[i].Rigor = AuthoredRigor(st.Works[i].Rigor)
	}
	for i := range st.ClaimEntries {
		st.ClaimEntries[i].Rigor = AuthoredRigor(st.ClaimEntries[i].Rigor)
	}
	for i := range st.Analyses {
		st.Analyses[i].Rigor = AuthoredRigor(st.Analyses[i].Rigor)
	}
	for i := range st.Experiments {
		st.Experiments[i].Rigor = AuthoredRigor(st.Experiments[i].Rigor)
	}
}

// unionPairs is declared plus every staged pair whose target declared does not
// carry: one edge per target, the leaf's declaration winning where both homes
// record the same one.
func unionPairs(declared, staged []Edge) []Edge {
	seen := map[string]bool{}
	out := declared
	for _, e := range declared {
		seen[e.Target] = true
	}
	for _, e := range staged {
		if !seen[e.Target] {
			seen[e.Target] = true
			out = append(out, e)
		}
	}
	return out
}

// IDRecord is where one authored node id lives: the register keying its
// canonical entry and the document hosting its body, each "" where none does.
type IDRecord struct {
	Kind, RegisterPath, HostingLeaf string
}

var anyNodeIDFullRE = PyRE(`^` + IDBody() + `$`)

// AuthoredIDs inventories every authored node id from the registers and the
// documents' frontmatter, never from .index/. A register path is the first
// register keying the id; a hosting document is the first declaring one, else
// the first citing one. Duplicates is every id keyed by more than one
// canonical register entry, with each keying register, repeats kept.
func AuthoredIDs(src common.Files) (ids map[string]IDRecord, duplicates map[string][]string, err error) {
	regs, err := Registers(src)
	if err != nil {
		return nil, nil, err
	}
	keyed := map[string][]string{}
	for _, rel := range regs {
		text, err := readRel(src, rel)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range LocateRegisterEntries(text) {
			keyed[e.NodeID] = append(keyed[e.NodeID], rel)
		}
	}
	docs, err := Documents(src)
	if err != nil {
		return nil, nil, err
	}
	declared, cited := map[string]string{}, map[string]string{}
	for _, rel := range docs {
		text, err := readRel(src, rel)
		if err != nil {
			return nil, nil, err
		}
		fields, err := documentFrontmatter(text, rel)
		if err != nil {
			return nil, nil, err
		}
		if fields == nil {
			continue
		}
		for _, id := range DeclaredNodeIDs(fields) {
			if _, ok := declared[id]; !ok {
				declared[id] = rel
			}
		}
		for _, key := range []string{"claims", "experiments"} {
			items, err := fields.ListOrEmpty(key)
			if err != nil {
				return nil, nil, malformed("%s: %s: %v", rel, key, err)
			}
			for _, id := range items {
				if _, ok := cited[id]; !ok && anyNodeIDFullRE.MatchString(id) {
					cited[id] = rel
				}
			}
		}
	}
	ids = map[string]IDRecord{}
	duplicates = map[string][]string{}
	add := func(id string) {
		if _, ok := ids[id]; ok {
			return
		}
		kind, _, _ := strings.Cut(id, "-")
		rec := IDRecord{Kind: kind, HostingLeaf: declared[id]}
		if rec.HostingLeaf == "" {
			rec.HostingLeaf = cited[id]
		}
		if regs := keyed[id]; len(regs) > 0 {
			rec.RegisterPath = regs[0]
		}
		ids[id] = rec
	}
	for id, regs := range keyed {
		add(id)
		if len(regs) > 1 {
			duplicates[id] = regs
		}
	}
	for id := range declared {
		add(id)
	}
	for id := range cited {
		add(id)
	}
	return ids, duplicates, nil
}
