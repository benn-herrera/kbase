package claimgraph

import (
	"slices"
	"strings"

	"kbase/internal/kb"
)

// The target-end rules, as the report names the route that settled a pair.
const (
	byIdentifier = "identifier"
	byEquation   = "equation"
	bySoleClaim  = "sole-claim"
)

// structuralKinds are the numbered things a document's own structure prints —
// sectioning units, floats, footnotes — as the page names them before a
// reference, case-folded, abbreviations included. None is a result.
var structuralKinds = map[string]bool{
	"section": true, "subsection": true, "subsubsection": true, "chapter": true, "appendix": true,
	"figure": true, "table": true, "footnote": true,
	"sec": true, "subsec": true, "ch": true, "chap": true, "app": true, "fig": true, "tab": true, "§": true, "§§": true,
}

// Relation is what a candidate's source is to its target.
type Relation string

const (
	SupportedBy  Relation = "supported-by"
	InSupportOf  Relation = "in-support-of"
	MentionedBy  Relation = "mention"
	harvestRef            = "reference"
	harvestNamed          = "hand-named"
)

// relations is every relation in its one order.
var relations = []Relation{SupportedBy, InSupportOf, MentionedBy}

// candidateClass is what one provenance of a pair is.
type candidateClass int

const (
	proofDirected candidateClass = iota
	equationTarget
	claimToClaim
)

// offered is the relations each class may be offered: every set holds
// supported-by and mention, so an intersection never empties and the draft
// is always offered.
var offered = map[candidateClass]map[Relation]bool{
	proofDirected:  {SupportedBy: true, MentionedBy: true},
	equationTarget: {SupportedBy: true, MentionedBy: true},
	claimToClaim:   {SupportedBy: true, InSupportOf: true, MentionedBy: true},
}

func classOf(directed bool, target ClaimNode) candidateClass {
	switch {
	case directed:
		return proofDirected
	case target.Equation != "":
		return equationTarget
	}
	return claimToClaim
}

// candidate is one ordered pair whose source names its target, however it
// was harvested, with the relations it may be offered and its draft.
type candidate struct {
	source, target ClaimNode
	offered        []Relation
	draft          Relation
	harvests       []string
	// passages are the paragraphs of each anchor or mention that produced the
	// pair, sorted and deduplicated: what a classify ask shows of it.
	passages []string
}

func (c candidate) pair() pair { return pair{c.source.ID, c.target.ID} }

// written is the one record a candidate classified as rel writes: a depends
// edge either way round, or a references record.
func written(c candidate, rel Relation) (depends bool, p pair) {
	switch rel {
	case SupportedBy:
		return true, c.pair()
	case InSupportOf:
		return true, pair{c.target.ID, c.source.ID}
	}
	return false, c.pair()
}

// claimOf is the claim a block carries, joined by locator as readGraph bound it.
func claimOf(b *Block, hosted []ClaimNode) (ClaimNode, bool) {
	if b == nil {
		return ClaimNode{}, false
	}
	for _, n := range hosted {
		if n.Locator == b.Display {
			return n, true
		}
	}
	return ClaimNode{}, false
}

// fragmentClaim is the claim whose identifier the anchor's fragment names:
// the cross-reference join asked at its definition end.
func fragmentClaim(a Anchor, hosted []ClaimNode) (ClaimNode, bool) {
	if a.Fragment == "" {
		return ClaimNode{}, false
	}
	for _, n := range hosted {
		if n.Identifier == a.Fragment {
			return n, true
		}
	}
	return ClaimNode{}, false
}

// boundProof is a proof block as stage D needs it: the claims it establishes.
type boundProof struct {
	subjects []ClaimNode
	proof    Proof
}

func bindProofClaims(g *AuthoredGraph, inv *Inventory) map[int]boundProof {
	out := map[int]boundProof{}
	for _, p := range inv.Proofs {
		bp := boundProof{proof: p}
		for _, s := range p.Subjects {
			if n, ok := claimOf(&s, g.hostedBy(s.Document)); ok {
				bp.subjects = append(bp.subjects, n)
			}
		}
		out[p.order] = bp
	}
	return out
}

// namesNoPremise is whether the word before an anchor names a kind no premise
// relation can hold: a structural kind, or a classified non-result block name.
// A plural reads as its singular and a trailing stop marks an abbreviation.
func namesNoPremise(word string) bool {
	if word == "" {
		return false
	}
	kind := strings.TrimRight(strings.ToLower(word), ".")
	in := func(k string) bool { return structuralKinds[k] || notAClaimTarget(k) }
	return in(kind) || (strings.HasSuffix(kind, "s") && in(kind[:len(kind)-1]))
}

// attribution is what the narrowing produced: every candidate, classed and
// drafted; the containment-directed pairs counted by the route that settled
// their target; the pairs a ring demoted; and the pairs the word before an
// anchor kept from opening.
type attribution struct {
	candidates  []candidate
	routes      map[string]int
	demoted     []pair
	wordDropped []pair
}

type reached struct {
	offered  map[Relation]bool
	harvests map[string]bool
	passages map[string]bool
}

// referenceLine is the author's words around a reference: its whole
// blank-line paragraph, markers off and unquoted first, collapsed to one
// line. line is 0-based in that numbering.
func referenceLine(t *Tree, document string, line int) string {
	lines := pageLines(t, document)
	if line >= len(lines) {
		return ""
	}
	start, end := line, line+1
	for start > 0 && kb.Strip(lines[start-1]) != "" {
		start--
	}
	for end < len(lines) && kb.Strip(lines[end]) != "" {
		end++
	}
	return kb.NormalizeSpace(strings.Join(lines[start:end], " "))
}

// narrow is kb_claimgraph stage D's narrowing: every edge candidate the corpus
// states, from a cross-reference or a hand-written name, classed and drafted.
// The node pass's verdicts in rec decide the source end of a reference in
// readable prose.
func narrow(t *Tree, g *AuthoredGraph, inv *Inventory, rec NodePassRecord) attribution {
	blocks := map[int]Block{}
	for _, b := range inv.Blocks {
		blocks[b.order] = b
	}
	fencesByDoc := map[string][]MathFence{}
	for _, f := range inv.Fences {
		fencesByDoc[f.Document] = append(fencesByDoc[f.Document], f)
	}
	blocksByDoc := map[string][]Block{}
	for _, b := range inv.Blocks {
		blocksByDoc[b.Document] = append(blocksByDoc[b.Document], b)
	}
	proofs := bindProofClaims(g, inv)
	settled := map[pair]string{}
	reach := map[pair]*reached{}
	unopened := map[pair]bool{}
	leaves := map[string]*leafReading{}
	judged := func(a Anchor) (standing, ClaimNode, bool) {
		if leaves[a.Document] == nil {
			leaves[a.Document] = readLeaf(t.Documents[a.Document], inv)
		}
		leaf := leaves[a.Document]
		s := leaf.standing(a, rec.Leaves[a.Document])
		if s != standingClaim {
			return s, ClaimNode{}, false
		}
		n, ok := leaf.claimOf(a, g.hostedBy(a.Document))
		return s, n, ok
	}
	type site struct {
		document string
		line     int
	}
	passages := map[site]string{}

	add := func(p pair, off map[Relation]bool, at site, harvest string) {
		r, ok := reach[p]
		if !ok {
			r = &reached{offered: map[Relation]bool{}, harvests: map[string]bool{}, passages: map[string]bool{}}
			for rel, on := range off {
				r.offered[rel] = on
			}
			reach[p] = r
		}
		for rel := range r.offered {
			if !off[rel] {
				delete(r.offered, rel)
			}
		}
		if _, ok := passages[at]; !ok {
			passages[at] = referenceLine(t, at.document, at.line)
		}
		r.passages[passages[at]] = true
		r.harvests[harvest] = true
	}

	for _, a := range inv.Anchors {
		if a.Target == "" {
			continue
		}
		targets := g.hostedBy(a.Target)
		minted, hasMinted := g.equationNode(a.Target, a.Label)
		if len(targets) == 0 && !hasMinted {
			continue
		}
		toEnds, route := targetEnd(a, targets, fencesByDoc[a.Target], blocksByDoc[a.Target], blocks, proofs, minted, hasMinted)
		if len(toEnds) == 0 {
			continue
		}
		var fromEnds []ClaimNode
		directed := false
		switch s, claim, found := judged(a); s {
		case standingNotAClaim:
		case standingClaim:
			if found {
				fromEnds = []ClaimNode{claim}
			}
		default:
			fromEnds, directed = sourceEnd(a, blocks, g.hostedBy(a.Document), proofs)
		}
		unheldWord := route != byIdentifier && route != byEquation && namesNoPremise(a.PrecedingWord)
		for _, s := range fromEnds {
			for _, tg := range toEnds {
				if s.ID == tg.ID {
					continue
				}
				p := pair{s.ID, tg.ID}
				if directed && route != "" {
					if _, ok := settled[p]; !ok {
						settled[p] = route
					}
				} else if unheldWord {
					unopened[p] = true
					continue
				}
				add(p, offered[classOf(directed, tg)], site{a.Document, a.Line}, harvestRef)
			}
		}
	}

	for _, h := range harvestHandNamed(t, g, inv) {
		add(pair{h.source, h.target}, offered[classOf(false, g.node(h.target))], site{h.document, h.line}, harvestNamed)
	}

	var settledPairs []pair
	for p := range settled {
		settledPairs = append(settledPairs, p)
	}
	slices.SortFunc(settledPairs, comparePairs)
	demoted := cycleEdges(settledPairs)
	onRing := map[pair]bool{}
	for _, p := range demoted {
		onRing[p] = true
	}
	routes := map[string]int{}
	for p, name := range settled {
		if !onRing[p] {
			routes[name]++
		}
	}
	var order []pair
	for p := range reach {
		order = append(order, p)
	}
	slices.SortFunc(order, comparePairs)
	out := attribution{routes: routes, demoted: demoted}
	for _, p := range order {
		r := reach[p]
		c := candidate{source: g.node(p.source), target: g.node(p.target), draft: MentionedBy}
		if _, ok := settled[p]; ok && !onRing[p] {
			c.draft = SupportedBy
		}
		for _, rel := range relations {
			if r.offered[rel] {
				c.offered = append(c.offered, rel)
			}
		}
		for h := range r.harvests {
			c.harvests = append(c.harvests, h)
		}
		slices.Sort(c.harvests)
		for ps := range r.passages {
			c.passages = append(c.passages, ps)
		}
		slices.Sort(c.passages)
		out.candidates = append(out.candidates, c)
	}
	for p := range unopened {
		if _, ok := reach[p]; !ok {
			out.wordDropped = append(out.wordDropped, p)
		}
	}
	slices.SortFunc(out.wordDropped, comparePairs)
	return out
}

// equationClaims is the claims an equation reference resolves through, and
// whether it is one: a label naming a fence of the target document is,
// whatever it resolves to, and an empty answer then means no pair at all.
func equationClaims(a Anchor, fences []MathFence, blocks map[int]Block, hosted []ClaimNode, proofs map[int]boundProof, minted ClaimNode, hasMinted bool) ([]ClaimNode, bool) {
	at := slices.IndexFunc(fences, func(f MathFence) bool { return slices.Contains(f.Labels, a.Label) })
	if at < 0 {
		return nil, false
	}
	mintedOnly := func() []ClaimNode {
		if hasMinted {
			return []ClaimNode{minted}
		}
		return nil
	}
	b, inBlock := blocks[fences[at].within]
	if fences[at].within == 0 || !inBlock {
		return mintedOnly(), true
	}
	if b.ClaimBearing() {
		if n, ok := claimOf(&b, hosted); ok {
			return []ClaimNode{n}, true
		}
		return nil, true
	}
	if strings.EqualFold(b.Environment, proofEnvironment) {
		if p, ok := proofs[b.order]; ok {
			return p.subjects, true
		}
		return nil, true
	}
	return mintedOnly(), true
}

// sourceEnd is the claims a reference may belong to and whether containment
// settled its direction: the claims a proof it sits in establishes; the
// claim-bearing block it sits in, which narrows and does not direct; else
// every claim its document hosts.
func sourceEnd(a Anchor, blocks map[int]Block, hosted []ClaimNode, proofs map[int]boundProof) ([]ClaimNode, bool) {
	if strings.EqualFold(a.HostingEnvironment, proofEnvironment) {
		p, ok := proofs[a.within]
		if !ok || p.proof.names(a) {
			return nil, false
		}
		if len(p.subjects) > 0 {
			return p.subjects, true
		}
		return hosted, false
	}
	if b, ok := blocks[a.within]; ok && a.within != 0 && b.ClaimBearing() {
		if n, ok := claimOf(&b, hosted); ok {
			return []ClaimNode{n}, false
		}
	}
	return hosted, false
}

// targetEnd is the claims a reference may name and the route that settled it.
// Order decides the route: the identifier, then a fragment naming a block
// classified as stating no result (no pair at all), then the equation join,
// then a document hosting exactly one claim.
func targetEnd(a Anchor, targets []ClaimNode, fences []MathFence, docBlocks []Block, blocks map[int]Block, proofs map[int]boundProof, minted ClaimNode, hasMinted bool) ([]ClaimNode, string) {
	if n, ok := fragmentClaim(a, targets); ok {
		return []ClaimNode{n}, byIdentifier
	}
	if b, ok := fragmentBlock(a.Fragment, docBlocks, false); ok && notAClaimTarget(b.Environment) {
		return nil, ""
	}
	if through, isEquation := equationClaims(a, fences, blocks, targets, proofs, minted, hasMinted); isEquation {
		if len(through) > 0 {
			return through, byEquation
		}
		return nil, ""
	}
	if len(targets) == 1 {
		return targets, bySoleClaim
	}
	return targets, ""
}

// cycleEdges is every edge lying on a cycle, sorted: an edge is on one
// exactly when its target reaches its source, a property of the edge set
// alone, so removing them all leaves an acyclic set.
func cycleEdges(edges []pair) []pair {
	following := map[string][]string{}
	for _, e := range edges {
		following[e.source] = append(following[e.source], e.target)
	}
	reaches := map[string]map[string]bool{}
	reachFrom := func(start string) map[string]bool {
		if r, ok := reaches[start]; ok {
			return r
		}
		r := map[string]bool{}
		pending := []string{start}
		for len(pending) > 0 {
			n := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			for _, next := range following[n] {
				if !r[next] {
					r[next] = true
					pending = append(pending, next)
				}
			}
		}
		reaches[start] = r
		return r
	}
	seen := map[pair]bool{}
	var out []pair
	for _, e := range edges {
		if reachFrom(e.target)[e.source] && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	slices.SortFunc(out, comparePairs)
	return out
}
