package claimgraph

import "strings"

// referencedEquation is one labelled equation a counting cross-reference
// names and nothing else in the graph holds.
type referencedEquation struct {
	document, label string
	line            int
}

// unheld is every equation a counting reference names that no claim-bearing
// block and no proof holds, in scan order, one per (document, label).
func unheld(inv *Inventory, counting []Anchor) []referencedEquation {
	blocks := map[int]Block{}
	for _, b := range inv.Blocks {
		blocks[b.order] = b
	}
	type key struct{ document, label string }
	named := map[key]bool{}
	for _, a := range counting {
		if a.Target != "" {
			named[key{a.Target, a.Label}] = true
		}
	}
	var out []referencedEquation
	seen := map[key]bool{}
	for _, f := range inv.Fences {
		if b, ok := blocks[f.within]; ok && f.within != 0 && (b.ClaimBearing() || strings.EqualFold(b.Environment, proofEnvironment)) {
			continue
		}
		for _, label := range f.Labels {
			k := key{f.Document, label}
			if !named[k] || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, referencedEquation{f.Document, label, f.Start})
		}
	}
	return out
}
