package kb

import (
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/log"
)

// State is the KB's authored metadata after one load.
type State struct {
	ClaimEntries   []ClaimEntry
	Leaves         []LeafRecord
	Indexes        []IndexRecord
	FrameworkNodes []FrameworkNode
	Experiments    []ExperimentNode
	Supports       []SupportNode
	Works          []ExternalWork
}

// comparePathParts orders slash paths the way Python orders Paths: segment
// by segment.
func comparePathParts(a, b string) int {
	return slices.Compare(strings.Split(a, "/"), strings.Split(b, "/"))
}

// walkFiles is every regular file of the KB whose name match accepts and
// none of whose directories below kb-root is excluded, as kb-root-relative
// slash paths in Python's Path order.
func walkFiles(src *Source, match func(name string) bool) ([]string, error) {
	root := src.Root()
	var out []string
	err := src.walk(root, func(name string) bool { return ExcludeDirs[name] }, func(p string) error {
		if !match(filepath.Base(p)) || !src.IsFile(p) {
			return nil
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
func Documents(src *Source) ([]string, error) {
	return walkFiles(src, func(name string) bool {
		return strings.HasSuffix(name, ".md") && !ExcludeNames[name]
	})
}

// Registers is every claim-quality register outside the excluded
// directories, as kb-root-relative slash paths.
func Registers(src *Source) ([]string, error) {
	return walkFiles(src, func(name string) bool { return name == RegisterFile })
}

func readRel(src *Source, rel string) (string, error) { return src.ReadText(src.KBPath(rel)) }

// KnownClaimIDs is every clm- id a register's canonical marker keys.
func KnownClaimIDs(src *Source) (map[string]bool, error) {
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

// Discover loads the KB's authored metadata: every register's entries, every
// document's node bodies, and the framework nodes. A clm- token naming no
// registered claim is dropped and reported to lg. A MalformedError names
// metadata no reader can take.
func Discover(src *Source, lg log.Logger) (State, error) {
	known, err := KnownClaimIDs(src)
	if err != nil {
		return State{}, err
	}
	regs, err := Registers(src)
	if err != nil {
		return State{}, err
	}
	var st State
	supportQ := map[string]SupportQuality{}
	for _, rel := range regs {
		text, err := readRel(src, rel)
		if err != nil {
			return State{}, err
		}
		st.Works = append(st.Works, ParseWorkEntries(text, rel)...)
		st.ClaimEntries = append(st.ClaimEntries, ParseClaimEntries(text, rel, known, lg)...)
		_, sq := ParseSupportEntries(text, rel, known, lg)
		for id, q := range sq {
			supportQ[id] = q
		}
	}
	docs, err := Documents(src)
	if err != nil {
		return State{}, err
	}
	var hosted []SupportNode
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
		exps, err := ParseExperimentLeaf(text, rel)
		if err != nil {
			return State{}, err
		}
		st.Experiments = append(st.Experiments, exps...)
		sups, err := ParseSupportLeaf(text, rel)
		if err != nil {
			return State{}, err
		}
		hosted = append(hosted, sups...)
		if leaf == nil && len(exps) == 0 && len(sups) == 0 {
			idx, err := parseIndex(text, rel)
			if err != nil {
				return State{}, err
			}
			if idx != nil {
				st.Indexes = append(st.Indexes, *idx)
			}
		}
	}
	for _, s := range hosted {
		q := supportQ[s.ID]
		s.Quality, s.DependsOn, s.Rationale, s.Solidity, s.SolidityTrace = q.Quality, q.DependsOn, q.Rationale, q.Solidity, q.SolidityTrace
		st.Supports = append(st.Supports, s)
	}
	if st.FrameworkNodes, err = ParseFrameworkNodes(src); err != nil {
		return State{}, err
	}
	slices.SortStableFunc(st.Works, func(a, b ExternalWork) int { return strings.Compare(a.ID, b.ID) })
	return st, nil
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
func AuthoredIDs(src *Source) (ids map[string]IDRecord, duplicates map[string][]string, err error) {
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

// RegisterCensus is one kind's canonical markers against the records its
// parser returns, in one register.
type RegisterCensus struct {
	RegisterPath, Kind string
	Markers, Records   []string
}

// Consistent is whether every marker of the kind produced a record.
func (c RegisterCensus) Consistent() bool { return len(c.Markers) == len(c.Records) }

// Lost is the marked ids no record came back for, sorted.
func (c RegisterCensus) Lost() []string {
	recs := map[string]bool{}
	for _, r := range c.Records {
		recs[r] = true
	}
	var out []string
	for _, m := range c.Markers {
		if !recs[m] {
			out = append(out, m)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// FanOutRecord is one supports edge as each authored end records it: the
// register's staged fraction and the hosting leaf's declared one, each
// absent (Set false) where that end does not name the beneficiary.
type FanOutRecord struct {
	SupID, ClaimID, RegisterPath, LeafPath string
	Staged, Declared                       Fraction
	RegisterStages                         bool
}

// DoubleEntered is whether both ends are live and so must agree.
func (r FanOutRecord) DoubleEntered() bool { return r.RegisterStages && r.LeafPath != "" }

// Agrees is whether both ends record the edge at the same fraction.
func (r FanOutRecord) Agrees() bool {
	if !r.Staged.Set || !r.Declared.Set {
		return false
	}
	if r.Staged.Pending || r.Declared.Pending {
		return r.Staged.Pending && r.Declared.Pending
	}
	return r.Staged.Value == r.Declared.Value
}

// Ends is which authored ends record the edge.
func (r FanOutRecord) Ends() []string {
	var out []string
	if r.Staged.Set {
		out = append(out, "register")
	}
	if r.Declared.Set {
		out = append(out, "leaf")
	}
	return out
}

// RegisteredEntry is a located entry and the register it is in.
type RegisteredEntry struct {
	RegisterPath string
	Entry        RegisterEntry
}

// RegisterWalk is the census, the marker binding and the fan-out of every
// register, from one pass.
type RegisterWalk struct {
	Census  []RegisterCensus
	Entries []RegisteredEntry
	FanOut  []FanOutRecord
}

// MisBound is every bound marker under a heading not its own.
func (w RegisterWalk) MisBound() []RegisteredEntry {
	var order []string
	per := map[string][]RegisterEntry{}
	for _, e := range w.Entries {
		if _, ok := per[e.RegisterPath]; !ok {
			order = append(order, e.RegisterPath)
		}
		per[e.RegisterPath] = append(per[e.RegisterPath], e.Entry)
	}
	var out []RegisteredEntry
	for _, p := range order {
		for _, e := range MisBoundEntries(per[p]) {
			out = append(out, RegisteredEntry{p, e})
		}
	}
	return out
}

// Unreconciled is every double-entered fan-out edge whose ends disagree.
func (w RegisterWalk) Unreconciled() []FanOutRecord {
	var out []FanOutRecord
	for _, r := range w.FanOut {
		if r.DoubleEntered() && !r.Agrees() {
			out = append(out, r)
		}
	}
	return out
}

// WalkRegisters reads every register once for its census, its binding and
// its staged fan-out against the hosting leaves in st.
func WalkRegisters(st State, src *Source) (RegisterWalk, error) {
	hosting := map[string]SupportNode{}
	for _, s := range st.Supports {
		hosting[s.ID] = s
	}
	regs, err := Registers(src)
	if err != nil {
		return RegisterWalk{}, err
	}
	var w RegisterWalk
	for _, rel := range regs {
		text, err := readRel(src, rel)
		if err != nil {
			return RegisterWalk{}, err
		}
		located := LocateRegisterEntries(text)
		for _, e := range located {
			w.Entries = append(w.Entries, RegisteredEntry{rel, e})
		}
		var claimRecords []string
		for _, c := range ParseClaimEntries(text, rel, nil, log.Discard()) {
			claimRecords = append(claimRecords, c.ID)
		}
		supRecords, _ := ParseSupportEntries(text, rel, nil, log.Discard())
		for _, kc := range []struct {
			kind    string
			records []string
		}{{"clm", claimRecords}, {"sup", supRecords}} {
			c := RegisterCensus{RegisterPath: rel, Kind: kc.kind, Records: kc.records}
			for _, e := range located {
				if e.Kind == kc.kind {
					c.Markers = append(c.Markers, e.NodeID)
				}
			}
			w.Census = append(w.Census, c)
		}
		order, staged := ParseStagedSupports(text)
		slices.Sort(order)
		for _, sup := range order {
			pairs := staged[sup]
			stagedBy := map[string]Fraction{}
			for _, p := range pairs {
				stagedBy[p.ClaimID] = p.Fraction
			}
			declaredBy := map[string]Fraction{}
			host, hosted := hosting[sup]
			if hosted {
				for _, p := range host.Supports {
					declaredBy[p.ClaimID] = p.Fraction
				}
			}
			var claims []string
			for c := range stagedBy {
				claims = append(claims, c)
			}
			for c := range declaredBy {
				if _, ok := stagedBy[c]; !ok {
					claims = append(claims, c)
				}
			}
			slices.Sort(claims)
			for _, c := range claims {
				r := FanOutRecord{SupID: sup, ClaimID: c, RegisterPath: rel, Staged: stagedBy[c], Declared: declaredBy[c], RegisterStages: len(pairs) > 0}
				if hosted {
					r.LeafPath = host.CanonicalPath
				}
				w.FanOut = append(w.FanOut, r)
			}
		}
	}
	return w, nil
}
