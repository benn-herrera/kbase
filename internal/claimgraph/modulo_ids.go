package claimgraph

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/log"
)

// ComparedKB is one side of a comparison modulo node ids: a kb-root/ and the
// paths of its two build records. Name is how a difference names the side.
type ComparedKB struct {
	Name, KBRoot, NodePass, Classification string
}

// BuiltBy is the KB kbase built in repo, named name.
func BuiltBy(name, repo string) ComparedKB {
	return ComparedKB{Name: name, KBRoot: filepath.Join(repo, kb.KBDir),
		NodePass: filepath.Join(repo, NodePassFile), Classification: filepath.Join(repo, ClassificationFile)}
}

// ModuloIDsCheck is one aspect of two KBs compared modulo node ids: the items
// each side holds, and every difference.
type ModuloIDsCheck struct {
	Name        string
	Counts      [2]int
	Differences []string
}

// CompareModuloIDs compares two KBs as data, every claim named by its
// register, title and hosting leaves rather than by its id: register entries,
// edges by their endpoints so named, works, claim count and no-claim reason per
// leaf, and the two build records. Ids are minted at random, so this is the
// equality two builds of the same inputs owe each other, and the one kbase's
// claim graph owes kb_tools'.
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
		sized("register entries by (register, title, host)", len(av.entries), len(bv.entries), multisetDiff(a.Name, b.Name, av.entries, bv.entries)),
		sized("depends, rests-on and references edges by endpoint title and host", len(av.edges), len(bv.edges), multisetDiff(a.Name, b.Name, av.edges, bv.edges)),
		sized("works", len(av.works), len(bv.works), multisetDiff(a.Name, b.Name, av.works, bv.works)),
		sized("claim count per leaf", len(av.counts), len(bv.counts), mapDiff(a.Name, b.Name, av.counts, bv.counts)),
		sized("no-claim reason per leaf", len(av.reasons), len(bv.reasons), mapDiff(a.Name, b.Name, av.reasons, bv.reasons)),
	}

	var an, bn NodePassRecord
	var ac, bc ClassificationRecord
	var errs []string
	for _, r := range []struct {
		path string
		v    any
	}{{a.NodePass, &an}, {b.NodePass, &bn}, {a.Classification, &ac}, {b.Classification, &bc}} {
		if ok, err := readRecord(r.path, r.v); err != nil {
			errs = append(errs, err.Error())
		} else if !ok {
			errs = append(errs, r.path+" does not exist")
		}
	}
	if len(errs) > 0 {
		return append(checks, ModuloIDsCheck{Name: "build records", Differences: errs}), nil
	}
	return append(checks,
		sized("node-pass record", len(an.Leaves), len(bn.Leaves), nodePassDiff(a.Name, b.Name, an, bn)),
		sized("classification record, less the ids", len(ac.Candidates), len(bc.Candidates), classificationDiff(a.Name, b.Name, ac, bc, av, bv))), nil
}

// moduloIDsView is one KB as CompareModuloIDs reads it: every claim by id with
// its title and host, every work, every edge, and per leaf its claim count and
// no-claim reason.
type moduloIDsView struct {
	claims  map[string]claimKey
	entries []string
	works   []string
	edges   []string
	counts  map[string]int
	reasons map[string]string
}

// claimKey names a claim by what both sides give it: title and host.
type claimKey struct{ register, title, host string }

func (k claimKey) String() string { return fmt.Sprintf("%s | %s | %s", k.register, k.title, k.host) }

func readModuloIDsView(kbRoot string) (moduloIDsView, error) {
	v := moduloIDsView{claims: map[string]claimKey{}, counts: map[string]int{}, reasons: map[string]string{}}
	docs, err := kb.Documents(kbRoot)
	if err != nil {
		return v, err
	}
	host := map[string][]string{}
	for _, d := range docs {
		text, err := kb.ReadText(filepath.Join(kbRoot, d))
		if err != nil {
			return v, err
		}
		fm := kb.ParseFrontmatter(text)
		ids, err := fm.ListOrEmpty("claims")
		if err != nil {
			return v, err
		}
		if fm.Kind() == kb.DocumentLeaf {
			v.counts[d] = len(ids)
			if r, ok := fm.Get("no-claim"); ok {
				v.reasons[d] = r.Str
			}
		}
		for _, id := range ids {
			host[id] = append(host[id], d)
		}
	}
	regs, err := kb.Registers(kbRoot)
	if err != nil {
		return v, err
	}
	var entries []kb.ClaimEntry
	for _, rel := range regs {
		text, err := kb.ReadText(filepath.Join(kbRoot, rel))
		if err != nil {
			return v, err
		}
		for _, e := range kb.ParseClaimEntries(text, rel, nil, log.Discard()) {
			k := claimKey{rel, e.Title, strings.Join(host[e.ID], ",")}
			v.claims[e.ID] = k
			v.entries = append(v.entries, k.String())
			entries = append(entries, e)
		}
		for _, w := range kb.ParseWorkEntries(text, rel) {
			v.works = append(v.works, fmt.Sprintf("%s | %s | %s | %s", rel, w.ID, w.Title, w.Rationale))
		}
	}
	name := func(id string) string {
		if k, ok := v.claims[id]; ok {
			return k.String()
		}
		return id
	}
	for _, e := range entries {
		for _, edge := range slices.Concat(e.DependsOn, e.References) {
			v.edges = append(v.edges, fmt.Sprintf("%s  -[%s]->  %s", name(edge.Source), edge.Relation, name(edge.Target)))
		}
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
func nodePassDiff(na, nb string, a, b NodePassRecord) []string {
	render := func(r NodePassRecord) map[string]string {
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
func classificationDiff(na, nb string, a, b ClassificationRecord, av, bv moduloIDsView) []string {
	render := func(r ClassificationRecord, v moduloIDsView) []string {
		var out []string
		for _, c := range r.Candidates {
			src, tgt := v.claims[c.Source].String(), v.claims[c.Target].String()
			c.Source, c.Target = "", ""
			data, _ := yaml.Marshal(c)
			out = append(out, src+"  ->  "+tgt+"\n"+string(data))
		}
		return out
	}
	diffs := multisetDiff(na, nb, render(a, av), render(b, bv))
	if a.About != b.About {
		diffs = append(diffs, "about differs")
	}
	return diffs
}
