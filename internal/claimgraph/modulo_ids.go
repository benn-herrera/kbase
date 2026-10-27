package claimgraph

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"kbase/internal/buildrecords"
	"kbase/internal/index"
	"kbase/internal/kb"
)

// ComparedKB is one side of a comparison modulo node ids: a kb-root/ and the
// paths of its build records. Name is how a difference names the side.
type ComparedKB struct {
	Name, KBRoot, NodePass, Classification, Unmarked, Cull string
}

// BuiltBy is the KB kbase built in repo, named name.
func BuiltBy(name, repo string) ComparedKB {
	return ComparedKB{Name: name, KBRoot: filepath.Join(repo, kb.KBDir), NodePass: filepath.Join(repo, buildrecords.NodePassFile),
		Classification: filepath.Join(repo, buildrecords.ClassificationFile), Unmarked: filepath.Join(repo, buildrecords.UnmarkedFile),
		Cull: filepath.Join(repo, buildrecords.CullFile)}
}

// ModuloIDsCheck is one aspect of two KBs compared modulo node ids: the items
// each side holds, and every difference.
type ModuloIDsCheck struct {
	Name        string
	Counts      [2]int
	Differences []string
}

// CompareModuloIDs compares two KBs as data, every claim named by its title
// and hosting leaf rather than by its id: claim records, edge records by their
// endpoints so named, works, claim count per leaf, and the build records, as
// data and byte for byte (recordBytes). Ids
// are minted at random, so this is the equality two builds of the same inputs
// owe each other. A claim the cull removed is named by the claim it was culled
// into. The cull record is compared where both sides hold one, since only a
// build that reached the claim-graph stages writes it.
func CompareModuloIDs(a, b ComparedKB) ([]ModuloIDsCheck, error) {
	av, err := readModuloIDsView(a.KBRoot)
	if err != nil {
		return nil, err
	}
	bv, err := readModuloIDsView(b.KBRoot)
	if err != nil {
		return nil, err
	}
	sized := func(name string, na, nb int, diffs []string) ModuloIDsCheck {
		return ModuloIDsCheck{Name: name, Counts: [2]int{na, nb}, Differences: diffs}
	}
	checks := []ModuloIDsCheck{
		sized("claim records by (title, host)", len(av.entries), len(bv.entries), multisetDiff(a.Name, b.Name, av.entries, bv.entries)),
		sized("edge records by endpoint title and host", len(av.edges), len(bv.edges), multisetDiff(a.Name, b.Name, av.edges, bv.edges)),
		sized("works", len(av.works), len(bv.works), multisetDiff(a.Name, b.Name, av.works, bv.works)),
		sized("claim count per leaf", len(av.counts), len(bv.counts), mapDiff(a.Name, b.Name, av.counts, bv.counts)),
	}

	var an, bn buildrecords.NodePassRecord
	var ac, bc buildrecords.ClassificationRecord
	var au, bu buildrecords.UnmarkedRecord
	var errs []string
	as, bs := builtTree(a.KBRoot), builtTree(b.KBRoot)
	for _, r := range []struct {
		src  *kb.Source
		path string
		v    any
	}{{as, a.NodePass, &an}, {bs, b.NodePass, &bn}, {as, a.Classification, &ac}, {bs, b.Classification, &bc}, {as, a.Unmarked, &au}, {bs, b.Unmarked, &bu}} {
		if ok, err := buildrecords.Read(r.src, r.path, r.v); err != nil {
			errs = append(errs, err.Error())
		} else if !ok {
			errs = append(errs, r.path+" does not exist")
		}
	}
	var acu, bcu buildrecords.CullRecord
	aCull, err := buildrecords.Read(as, a.Cull, &acu)
	if err != nil {
		errs = append(errs, err.Error())
	}
	bCull, err := buildrecords.Read(bs, b.Cull, &bcu)
	if err != nil {
		errs = append(errs, err.Error())
	}
	switch {
	case aCull && !bCull:
		errs = append(errs, b.Cull+" does not exist")
	case bCull && !aCull:
		errs = append(errs, a.Cull+" does not exist")
	}
	if len(errs) > 0 {
		return append(checks, ModuloIDsCheck{Name: "build records", Differences: errs}), nil
	}
	av.nameCulled(acu)
	bv.nameCulled(bcu)
	checks = append(checks,
		sized("node-pass record", len(an.Leaves), len(bn.Leaves), nodePassDiff(a.Name, b.Name, an, bn)),
		sized("classification record, less the ids", len(ac.Candidates), len(bc.Candidates), classificationDiff(a.Name, b.Name, ac, bc, av, bv)),
		sized("unmarked record, less the ids", len(au.Pairs), len(bu.Pairs), unmarkedDiff(a.Name, b.Name, au, bu, av, bv)))
	var culled *buildrecords.CullRecord
	if aCull {
		culled = &acu
		checks = append(checks, sized("cull record, less the ids", len(acu.Entries), len(bcu.Entries), cullDiff(a.Name, b.Name, acu, bcu, av, bv)))
	}
	byteChecks, err := recordBytes(a, b, an, ac, au, culled, newIDRenaming(av, bv))
	return append(checks, byteChecks...), err
}

// idRenaming maps a claim id of one KB to the id another gives the claim of
// the same register, title and host, keeping each id it cannot map.
type idRenaming struct {
	from     map[string]claimKey
	to       map[claimKey][]string
	unmapped map[string]bool
}

func newIDRenaming(from, to moduloIDsView) *idRenaming {
	r := &idRenaming{from: from.claims, to: map[claimKey][]string{}, unmapped: map[string]bool{}}
	for id, k := range to.claims {
		r.to[k] = append(r.to[k], id)
	}
	return r
}

func (r *idRenaming) id(id string) string {
	if k, ok := r.from[id]; ok && len(r.to[k]) == 1 {
		return r.to[k][0]
	}
	r.unmapped[id] = true
	return id
}

func (r *idRenaming) entries(entries []buildrecords.CandidateEntry) []buildrecords.CandidateEntry {
	out := slices.Clone(entries)
	for i := range out {
		out[i].Source, out[i].Target = r.id(out[i].Source), r.id(out[i].Target)
	}
	return out
}

// cull is c with every id renamed. A pair whose renamed ids fall out of id
// order is turned round, its verdict with it, so each pair stands as the
// cull's writer keys it.
func (r *idRenaming) cull(c buildrecords.CullRecord) buildrecords.CullRecord {
	out := buildrecords.CullRecord{About: c.About}
	for _, e := range c.Entries {
		e.A, e.B = r.id(e.A), r.id(e.B)
		if e.A > e.B {
			e.A, e.B = e.B, e.A
			switch e.Verdict {
			case buildrecords.VerdictAAddsNothing:
				e.Verdict = buildrecords.VerdictBAddsNothing
			case buildrecords.VerdictBAddsNothing:
				e.Verdict = buildrecords.VerdictAAddsNothing
			}
		}
		out.Entries = append(out.Entries, e)
	}
	for _, k := range c.Culls {
		out.Culls = append(out.Culls, buildrecords.Cull{ID: r.id(k.ID), Into: r.id(k.Into)})
	}
	for _, l := range c.Links {
		l.A, l.B = r.id(l.A), r.id(l.B)
		if l.A > l.B {
			l.A, l.B = l.B, l.A
		}
		out.Links = append(out.Links, l)
	}
	return out
}

// recordBytes compares each of a's build records with b's byte for byte. The
// node-pass record names no claim by id and is compared as it stands. The
// classification and unmarked records are compared after a's ids are renamed
// to b's and the record is written again as a's build writes it; a's file
// must itself be that writer's output. A plan lists its sources in id order,
// each source's pairs in rank order, so the renamed plan is re-ordered by its
// new source ids, each source's pairs kept in a's order. The cull record, where
// acu is not nil, is compared as the classification record is.
func recordBytes(a, b ComparedKB, an buildrecords.NodePassRecord, ac buildrecords.ClassificationRecord, au buildrecords.UnmarkedRecord, acu *buildrecords.CullRecord, rename *idRenaming) ([]ModuloIDsCheck, error) {
	rc, ru := ac, au
	rc.Candidates = rename.entries(ac.Candidates)
	ru.Pairs = rename.entries(au.Pairs)
	if au.Planned != nil {
		planned := make([]buildrecords.PlannedPair, len(*au.Planned))
		for i, p := range *au.Planned {
			planned[i] = buildrecords.PlannedPair{rename.id(p[0]), rename.id(p[1])}
		}
		slices.SortStableFunc(planned, func(x, y buildrecords.PlannedPair) int { return strings.Compare(x[0], y[0]) })
		ru.Planned = &planned
	}
	type side struct {
		name, ours, theirs string
		// written is a's record as its writer spells it; renamed, where set,
		// is that with b's ids, compared in place of a's file.
		written, renamed func() ([]byte, error)
	}
	sides := []side{
		{"node-pass record bytes", a.NodePass, b.NodePass, func() ([]byte, error) { return nodePassBytes(an) }, nil},
		{"classification record bytes, ids renamed", a.Classification, b.Classification,
			func() ([]byte, error) { return classificationBytes(ac) }, func() ([]byte, error) { return classificationBytes(rc) }},
		{"unmarked record bytes, ids renamed", a.Unmarked, b.Unmarked,
			func() ([]byte, error) { return unmarkedBytes(au) }, func() ([]byte, error) { return unmarkedBytes(ru) }},
	}
	if acu != nil {
		rcu := rename.cull(*acu)
		sides = append(sides, side{"cull record bytes, ids renamed", a.Cull, b.Cull,
			func() ([]byte, error) { return cullBytes(*acu) }, func() ([]byte, error) { return cullBytes(rcu) }})
	}
	var unmapped []string
	for id := range rename.unmapped {
		unmapped = append(unmapped, fmt.Sprintf("%s's id %s names no single claim of %s", a.Name, id, b.Name))
	}
	slices.Sort(unmapped)
	var out []ModuloIDsCheck
	as, bs := builtTree(a.KBRoot), builtTree(b.KBRoot)
	for _, s := range sides {
		ours, err := as.ReadFile(s.ours)
		if err != nil {
			return nil, err
		}
		theirs, err := bs.ReadFile(s.theirs)
		if err != nil {
			return nil, err
		}
		written, err := s.written()
		if err != nil {
			return nil, err
		}
		var diffs []string
		if !bytes.Equal(ours, written) {
			diffs = append(diffs, firstLineDifference(s.ours, "its writer's spelling", ours, written))
		}
		compared := ours
		if s.renamed != nil {
			if compared, err = s.renamed(); err != nil {
				return nil, err
			}
			diffs = append(diffs, unmapped...)
		}
		if !bytes.Equal(compared, theirs) {
			diffs = append(diffs, firstLineDifference(a.Name, b.Name, compared, theirs))
		}
		out = append(out, ModuloIDsCheck{Name: s.name, Counts: [2]int{len(compared), len(theirs)}, Differences: diffs})
	}
	return out, nil
}

// firstLineDifference names the first line at which x and y differ.
func firstLineDifference(nx, ny string, x, y []byte) string {
	xl, yl := strings.Split(string(x), "\n"), strings.Split(string(y), "\n")
	for i := range max(len(xl), len(yl)) {
		var p, q string
		if i < len(xl) {
			p = xl[i]
		}
		if i < len(yl) {
			q = yl[i]
		}
		if p != q {
			return fmt.Sprintf("line %d: %s %q, %s %q", i+1, nx, p, ny, q)
		}
	}
	return "the bytes differ"
}

// moduloIDsView is one KB as CompareModuloIDs reads it: every claim record by
// id with its title and its host, read off its statement marker, every work
// record, every edge record, and per leaf the claims it hosts.
type moduloIDsView struct {
	claims  map[string]claimKey
	entries []string
	works   []string
	edges   []string
	counts  map[string]int
}

// claimKey names a claim by what both sides give it: title and host, or, for a
// claim the cull removed, the name of the claim it was culled into.
type claimKey struct{ title, host, into string }

func (k claimKey) String() string {
	if k.into != "" {
		return "culled into " + k.into
	}
	return fmt.Sprintf("%s | %s", k.title, k.host)
}

// nameCulled names each claim c culls by the claim it was culled into, which
// the KB still holds. Two claims culled into one claim share a name, and so
// map to no single claim of the other side.
func (v moduloIDsView) nameCulled(c buildrecords.CullRecord) {
	for _, k := range c.Culls {
		into := k.Into
		if key, ok := v.claims[k.Into]; ok {
			into = key.String()
		}
		v.claims[k.ID] = claimKey{into: into}
	}
}

func readModuloIDsView(kbRoot string) (moduloIDsView, error) {
	v := moduloIDsView{claims: map[string]claimKey{}, counts: map[string]int{}}
	src := builtTree(kbRoot)
	g, err := kb.LoadGraph(src)
	if err != nil {
		return v, err
	}
	rows, err := index.Derive(src)
	if err != nil {
		return v, err
	}
	for p, kind := range rows.DocumentKinds {
		if kind == kb.DocumentLeaf {
			v.counts[p] = 0
		}
	}
	for _, n := range g.Claims {
		k := claimKey{title: n.Title, host: rows.Hosts[n.ID]}
		v.claims[n.ID] = k
		v.entries = append(v.entries, k.String())
		if k.host != "" {
			v.counts[k.host]++
		}
	}
	for _, w := range g.Works {
		v.works = append(v.works, fmt.Sprintf("%s | %s | %s", w.ID, w.Title, w.Rationale))
	}
	name := func(id string) string {
		if k, ok := v.claims[id]; ok {
			return k.String()
		}
		return id
	}
	for _, e := range g.Edges() {
		relation := e.Relation
		if e.ReplacedRelation != "" {
			relation += " " + e.ReplacedRelation
		}
		v.edges = append(v.edges, fmt.Sprintf("%s  -[%s]->  %s", name(e.Source), relation, name(e.Target)))
	}
	return v, nil
}

// multisetDiff is every item one side holds more often than the other.
func multisetDiff(na, nb string, a, b []string) []string {
	count := map[string]int{}
	for _, s := range a {
		count[s]++
	}
	for _, s := range b {
		count[s]--
	}
	var out []string
	for s, n := range count {
		switch {
		case n > 0:
			out = append(out, fmt.Sprintf("only in %s (x%d): %s", na, n, s))
		case n < 0:
			out = append(out, fmt.Sprintf("only in %s (x%d): %s", nb, -n, s))
		}
	}
	slices.Sort(out)
	return out
}

func mapDiff[V comparable](na, nb string, a, b map[string]V) []string {
	var out []string
	for _, k := range sortedUnion(a, b) {
		x, inA := a[k]
		y, inB := b[k]
		if inA != inB || x != y {
			out = append(out, fmt.Sprintf("%s: %s %v (present %t), %s %v (present %t)", k, na, x, inA, nb, y, inB))
		}
	}
	return out
}

func sortedUnion[V any](a, b map[string]V) []string {
	var keys []string
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// nodePassDiff compares the two node-pass records as data: they carry no ids.
func nodePassDiff(na, nb string, a, b buildrecords.NodePassRecord) []string {
	render := func(r buildrecords.NodePassRecord) map[string]string {
		out := map[string]string{}
		for p, e := range r.Leaves {
			data, _ := yaml.Marshal(e)
			out[p] = string(data)
		}
		return out
	}
	diffs := mapDiff(na, nb, render(a), render(b))
	if a.About != b.About {
		diffs = append(diffs, "about differs")
	}
	return diffs
}

// classificationDiff compares the classification records as data, each
// candidate named by its claims' titles and hosts rather than ids.
func classificationDiff(na, nb string, a, b buildrecords.ClassificationRecord, av, bv moduloIDsView) []string {
	diffs := multisetDiff(na, nb, renderEntries(a.Candidates, av), renderEntries(b.Candidates, bv))
	if a.About != b.About {
		diffs = append(diffs, "about differs")
	}
	return diffs
}

// renderEntries is each pair-keyed entry with its claims named by title and
// host rather than id.
func renderEntries(entries []buildrecords.CandidateEntry, v moduloIDsView) []string {
	var out []string
	for _, c := range entries {
		src, tgt := v.claims[c.Source].String(), v.claims[c.Target].String()
		c.Source, c.Target = "", ""
		data, _ := yaml.Marshal(c)
		out = append(out, src+"  ->  "+tgt+"\n"+string(data))
	}
	return out
}

// unmarkedDiff compares the unmarked records as data: whether each holds a
// plan, the planned pairs and the asked pairs, claims named by title and host
// rather than id. A plan's order follows its source ids, so the pairs compare
// as a multiset.
func unmarkedDiff(na, nb string, a, b buildrecords.UnmarkedRecord, av, bv moduloIDsView) []string {
	planned := func(r buildrecords.UnmarkedRecord, v moduloIDsView) []string {
		if r.Planned == nil {
			return []string{"no plan"}
		}
		out := []string{"a plan"}
		for _, p := range *r.Planned {
			out = append(out, v.claims[p[0]].String()+"  ->  "+v.claims[p[1]].String())
		}
		return out
	}
	diffs := multisetDiff(na, nb, planned(a, av), planned(b, bv))
	diffs = append(diffs, multisetDiff(na, nb, renderEntries(a.Pairs, av), renderEntries(b.Pairs, bv))...)
	if a.About != b.About {
		diffs = append(diffs, "about differs")
	}
	return diffs
}

// cullDiff compares the cull records as data: the asks, the culls and the
// surviving links, claims named by title and host rather than id. A pair is
// keyed in id order, so each is rendered unordered, a verdict naming the claim
// that adds nothing.
func cullDiff(na, nb string, a, b buildrecords.CullRecord, av, bv moduloIDsView) []string {
	render := func(r buildrecords.CullRecord, v moduloIDsView) []string {
		name := func(id string) string { return v.claims[id].String() }
		unordered := func(x, y string) string {
			p := []string{name(x), name(y)}
			slices.Sort(p)
			return p[0] + "  ~  " + p[1]
		}
		var out []string
		for _, e := range r.Entries {
			judged := unordered(e.A, e.B) + ": " + e.Verdict
			switch e.Verdict {
			case buildrecords.VerdictAAddsNothing:
				judged = name(e.A) + " adds nothing to " + name(e.B)
			case buildrecords.VerdictBAddsNothing:
				judged = name(e.B) + " adds nothing to " + name(e.A)
			}
			out = append(out, fmt.Sprintf("pass %d, %s: %s", e.Pass, e.Outcome, judged))
		}
		for _, c := range r.Culls {
			out = append(out, "cull: "+name(c.ID)+"  ->  "+name(c.Into))
		}
		for _, l := range r.Links {
			out = append(out, "link: "+unordered(l.A, l.B)+" ("+l.Harvest+")")
		}
		return out
	}
	diffs := multisetDiff(na, nb, render(a, av), render(b, bv))
	if a.About != b.About {
		diffs = append(diffs, "about differs")
	}
	return diffs
}
