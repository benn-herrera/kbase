package claimgraph

import (
	"cmp"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// claimSite is a claim's identity before an id exists: its document and its
// own display line.
type claimSite struct{ document, locator string }

// CitedWork is one external work a claim of this corpus rests on. Named is
// whether a bibliography answered its key, so its title is the reference
// list's text rather than the key itself.
type CitedWork struct {
	Key, Title string
	Named      bool
}

// ID is the work's node id, derived from its key.
func (w CitedWork) ID() string { return kb.WorkPrefix + "-" + w.Key }

// endcap is every external work a claim rests on, and by which claims: per
// claim site, the work ids it rests on, sorted and deduplicated.
type endcap struct {
	works    []CitedWork
	pairings map[claimSite][]string
}

// scanEndcap joins the citations to the runs of lines a claim owns: its own
// block, and the body of the proof establishing it, the pairing recorded under
// the subject's site.
func scanEndcap(inv *Inventory) endcap {
	sites := map[int][]claimSite{}
	for _, b := range inv.claimBlocks() {
		sites[b.order] = append(sites[b.order], claimSite{b.Document, b.Display})
	}
	for _, p := range inv.Proofs {
		for _, s := range p.Subjects {
			sites[p.order] = append(sites[p.order], claimSite{s.Document, s.Display})
		}
	}
	rendered := map[string]string{}
	for _, w := range inv.Works {
		rendered[w.Key] = w.Text
	}
	cited := map[string]CitedWork{}
	pairings := map[claimSite]map[string]bool{}
	for _, c := range inv.Citations {
		hosting := sites[c.within]
		if c.within == 0 || len(hosting) == 0 {
			continue
		}
		if _, ok := cited[c.Key]; !ok {
			text, named := rendered[c.Key]
			title := c.Key
			if named && text != "" {
				title = text
			}
			cited[c.Key] = CitedWork{Key: c.Key, Title: title, Named: named}
		}
		for _, s := range hosting {
			if pairings[s] == nil {
				pairings[s] = map[string]bool{}
			}
			pairings[s][CitedWork{Key: c.Key}.ID()] = true
		}
	}
	e := endcap{pairings: map[claimSite][]string{}}
	for _, w := range cited {
		e.works = append(e.works, w)
	}
	slices.SortFunc(e.works, func(a, b CitedWork) int { return strings.Compare(a.Key, b.Key) })
	for s, ids := range pairings {
		for id := range ids {
			e.pairings[s] = append(e.pairings[s], id)
		}
		slices.Sort(e.pairings[s])
	}
	return e
}

// sortedSites is the endcap's claim sites in order.
func (e endcap) sortedSites() []claimSite {
	out := make([]claimSite, 0, len(e.pairings))
	for s := range e.pairings {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b claimSite) int {
		return cmp.Or(strings.Compare(a.document, b.document), strings.Compare(a.locator, b.locator))
	})
	return out
}

func workRationale(w CitedWork) string {
	if w.Named {
		return "Cited by a claim of this corpus as " + w.Key + ", and resolved against the corpus's own " +
			"bibliography to the reference-list entry this entry is titled with. The work is outside " +
			"the corpus, so nothing here derives from it and its standing is unassessed."
	}
	return "Cited by a claim of this corpus as " + w.Key + ", which is the whole of what the corpus says " +
		"about it: no bibliography answered the key, so this entry is titled with the key itself. The " +
		"work is outside the corpus and its standing is unassessed."
}
