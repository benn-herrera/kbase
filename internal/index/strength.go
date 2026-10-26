// Package index derives the metadata layer's derived fields and the
// .index/*.yaml files from a KB's authored metadata, and checks a KB's
// freshness, links and citations against them.
package index

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
)

// Result is one claim's or analysis's derived strength: the local value it
// starts from, the least premise contribution, the derivation the min
// publishes, the largest lift, the smallest cap and their composition. Nil is
// pending.
type Result struct {
	Rigor, MinDep, Derivation, Lift, Cap, Final *float64
}

// Strength is the one computation every derived strength is read from.
type Strength struct {
	Results         map[string]Result // every claim's result, by id
	AnalysisResults map[string]Result // every analysis's result, by id
}

// finals is every claim's non-pending final strength.
func (s Strength) finals() map[string]float64 {
	out := map[string]float64{}
	for id, r := range s.Results {
		if r.Final != nil {
			out[id] = *r.Final
		}
	}
	return out
}

func ptr(v float64) *float64 { return &v }

// strengthTolerance is how far a spelling may sit from the value it publishes
// and still read back as that value.
const strengthTolerance = 1e-9

// FormatStrengthRecord is a strength as the KB's record carries it — the
// "- strength:" line and a premise bullet's "(strength …)" annotation — in a
// spelling that reads back as the walk's value: four decimals, trailing zeros
// trimmed, and more decimals where four would lose the value. A nil value is
// the pending literal.
func FormatStrengthRecord(v *float64) string {
	if v == nil {
		return kb.PendingLiteral
	}
	for decimals := 4; decimals <= 15; decimals++ {
		s := strconv.FormatFloat(*v, 'f', decimals, 64)
		if back, err := strconv.ParseFloat(s, 64); err == nil && math.Abs(back-*v) < strengthTolerance {
			return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
		}
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// FormatStrengthDisplay is a strength as an unowned surface shows it — a
// trace, a verify message, a show field — at two decimals, or the pending
// literal.
func FormatStrengthDisplay(v *float64) string {
	if v == nil {
		return kb.PendingLiteral
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// BuildStatusPhrase is the band phrase for a strength, or nil for a pending
// one.
func BuildStatusPhrase(v *float64) *string {
	if b := kb.BandFor(v); b != nil {
		return &b.StatusPhrase
	}
	return nil
}

// BuildBand is the band slug for a strength.
func BuildBand(v *float64) string {
	if b := kb.BandFor(v); b != nil {
		return b.Slug
	}
	return kb.UnknownBandSlug
}

// minOf is Python's min over a non-empty list: the first least value.
func minOf(first float64, rest ...float64) float64 {
	m := first
	for _, v := range rest {
		if v < m {
			m = v
		}
	}
	return m
}

// RenderStrengthTrace is the arithmetic suffix for the value a node's strength
// took, leading space included, or "" where the line has nothing to explain.
// The expression nests innermost-first: the min over the premises, the lift
// above it, then the cap around both. A pending rigor has no min to show, so a
// lift standing over it is the whole expression; with no premise at all the
// derivation is the node's bare value — 0 for a claim, its rigor for an
// analysis — the base a lift and a cap compose around.
func RenderStrengthTrace(r Result) string {
	if r.Final == nil {
		return ""
	}
	if r.Rigor == nil {
		if r.Lift == nil {
			return ""
		}
		expr := "lift " + FormatStrengthDisplay(r.Lift)
		if r.Cap != nil && *r.Cap < *r.Lift {
			expr = "min(" + expr + ", " + FormatStrengthDisplay(r.Cap) + ")"
		}
		return " [= " + expr + "]"
	}
	expr := FormatStrengthDisplay(r.Derivation)
	composed := false
	if r.MinDep != nil {
		expr = "min(" + FormatStrengthDisplay(r.Rigor) + ", " + FormatStrengthDisplay(r.MinDep) + ")"
		composed = true
	}
	position := r.Derivation
	if r.Lift != nil && (position == nil || *r.Lift > *position) {
		expr = "max(" + expr + ", " + FormatStrengthDisplay(r.Lift) + ")"
		position = r.Lift
		composed = true
	}
	if r.Cap != nil && (position == nil || *r.Cap < *position) {
		expr = "min(" + expr + ", " + FormatStrengthDisplay(r.Cap) + ")"
		composed = true
	}
	if !composed {
		return ""
	}
	return " [= " + expr + "]"
}

// MinDependencyStrength is the least contribution a node's scoring premises
// feed it: each edge's bearing times its source's strength, or nil where no
// edge participates. An edge whose bearing or whose source's strength is
// pending is excluded — it contributes nothing — as is one naming no source
// the walk holds.
func MinDependencyStrength(premises []kb.Edge, strength map[string]*float64) *float64 {
	var deps []float64
	for _, e := range premises {
		if e.Relation != kb.RelationSupports && e.Relation != kb.RelationFounds {
			continue
		}
		if e.Bearing.Pending {
			continue
		}
		s := strength[e.Source]
		if s == nil {
			continue
		}
		deps = append(deps, e.Bearing.Value**s)
	}
	if len(deps) == 0 {
		return nil
	}
	return ptr(minOf(deps[0], deps[1:]...))
}

// liftStrength is the largest lift any participating lifter feeds target:
// bearing times the lifter's strength, an excluded edge omitted.
func liftStrength(edges []kb.Edge, strength map[string]*float64) *float64 {
	var best *float64
	for _, e := range edges {
		if e.Bearing.Pending {
			continue
		}
		s := strength[e.Source]
		if s == nil {
			continue
		}
		if v := e.Bearing.Value * *s; best == nil || v > *best {
			best = ptr(v)
		}
	}
	return best
}

// capStrength is the smallest cap any participating refuter places on target:
// 1 − bearing × the refuter's strength, an excluded edge omitted.
func capStrength(edges []kb.Edge, strength map[string]*float64) *float64 {
	var best *float64
	for _, e := range edges {
		if e.Bearing.Pending {
			continue
		}
		s := strength[e.Source]
		if s == nil {
			continue
		}
		if v := 1 - e.Bearing.Value**s; best == nil || v < *best {
			best = ptr(v)
		}
	}
	return best
}

// derive is one node's result: the derivation is the least of its rigor and
// its premises' contributions, or bare where no premise contributes — a rigor
// caps what the premises establish and is no premise itself. bare is what the
// node's kind derives with nothing to stand on: 0 for a claim, which nothing
// substantiates, and its own rigor for an analysis, whose method is its
// evidence. The final is that value raised to the largest lift and capped by
// the smallest refutation. A pending rigor over a premise set leaves the
// derivation pending, and a lift still stands it; over no premise it caps
// nothing, and the node reads its kind's bare value.
func derive(rigor, bare *float64, premises, lifts, caps []kb.Edge, strength map[string]*float64) Result {
	r := Result{Rigor: rigor}
	r.MinDep = MinDependencyStrength(premises, strength)
	switch {
	case r.MinDep == nil:
		r.Derivation = bare
	case rigor != nil:
		r.Derivation = ptr(minOf(*rigor, *r.MinDep))
	}
	position := r.Derivation
	r.Lift = liftStrength(lifts, strength)
	r.Cap = capStrength(caps, strength)
	if r.Lift != nil && (position == nil || *r.Lift > *position) {
		position = r.Lift
	}
	if position != nil && r.Cap != nil && *r.Cap < *position {
		position = r.Cap
	}
	if position != nil {
		r.Final = ptr(*position)
	}
	return r
}

// derivationOrder is Kahn's order over every claim and analysis that carries a
// derived strength: a node after every node its premises name. Nodes outside
// that set are not ordered and not waited on. cycle is every node the order
// could not reach — a cycle's members and every node resting on one — sorted.
func derivationOrder(nodes []string, deps map[string][]string) (order, cycle []string) {
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for _, id := range nodes {
		indegree[id] = 0
	}
	for _, id := range nodes {
		seen := map[string]bool{}
		for _, d := range deps[id] {
			if seen[d] {
				continue
			}
			seen[d] = true
			if _, ok := indegree[d]; !ok {
				continue
			}
			indegree[id]++
			dependents[d] = append(dependents[d], id)
		}
	}
	var queue []string
	for id, d := range indegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	slices.Sort(queue)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		next := slices.Clone(dependents[node])
		slices.Sort(next)
		for _, d := range next {
			indegree[d]--
			if indegree[d] == 0 {
				queue = append(queue, d)
				slices.Sort(queue)
			}
		}
	}
	for id, d := range indegree {
		if d > 0 {
			cycle = append(cycle, id)
		}
	}
	slices.Sort(cycle)
	return order, cycle
}

// strengthWalk is what one strength computation reads: every claim's and
// analysis's rigor by id, each list in walk order; every other source's value —
// an axiom's, a work's, a run experiment's; and every scoring edge by the node
// it lands on: the supports and founds edges feeding its minimum, its lifts
// and its caps.
type strengthWalk struct {
	claimOrder, analysisOrder []string
	rigor                     map[string]*float64
	sources                   map[string]*float64
	premises, lifts, caps     map[string][]kb.Edge
}

func newStrengthWalk() strengthWalk {
	return strengthWalk{rigor: map[string]*float64{}, sources: map[string]*float64{},
		premises: map[string][]kb.Edge{}, lifts: map[string][]kb.Edge{}, caps: map[string][]kb.Edge{}}
}

// land files each edge under the node it lands on, by its relation's station.
func (w strengthWalk) land(edges []kb.Edge) {
	for _, e := range edges {
		switch e.Relation {
		case kb.RelationSupports, kb.RelationFounds:
			w.premises[e.Target] = append(w.premises[e.Target], e)
		case kb.RelationLifts:
			w.lifts[e.Target] = append(w.lifts[e.Target], e)
		case kb.RelationRefutes:
			w.caps[e.Target] = append(w.caps[e.Target], e)
		}
	}
}

// run is every claim's and analysis's result the derivation order reaches,
// and the nodes a cycle kept it from, which derive nothing.
func (w strengthWalk) run() (Strength, []string) {
	nodes := slices.Concat(w.claimOrder, w.analysisOrder)
	inWalk := map[string]bool{}
	for _, id := range nodes {
		inWalk[id] = true
	}
	deps := map[string][]string{}
	for _, id := range nodes {
		for _, e := range slices.Concat(w.premises[id], w.lifts[id], w.caps[id]) {
			if inWalk[e.Source] {
				deps[id] = append(deps[id], e.Source)
			}
		}
	}
	order, cycle := derivationOrder(nodes, deps)
	isClaim := map[string]bool{}
	for _, id := range w.claimOrder {
		isClaim[id] = true
	}
	strength := maps.Clone(w.sources)
	out := Strength{Results: map[string]Result{}, AnalysisResults: map[string]Result{}}
	for _, id := range order {
		// A claim nothing carries derives 0; an analysis nothing carries
		// derives its own rigor, its method being its evidence.
		bare := ptr(0.0)
		if !isClaim[id] {
			bare = w.rigor[id]
		}
		r := derive(w.rigor[id], bare, w.premises[id], w.lifts[id], w.caps[id], strength)
		strength[id] = r.Final
		if isClaim[id] {
			out.Results[id] = r
		} else {
			out.AnalysisResults[id] = r
		}
	}
	return out, cycle
}

// graphStrength derives every claim's and analysis's strength from g's records
// and edges, a non-run experiment supplying no value.
//
// A node's derivation is min(its rigor, every participating premise's
// contribution): rigor caps what the premises establish and is no premise
// itself. A node with no participating premise derives what its kind is worth
// bare — a claim reads 0, since nothing substantiates it, whatever its rigor,
// judged or not; an analysis reads its own rigor, its method being the evidence
// its premises would otherwise supply. A claim or analysis premise contributes
// its bearing times its final strength, and a work or axiom premise its bearing
// times the source's rigor — a work's judged once by a person, an axiom's 1.0 by
// rule. A final strength is min(max(derivation, every lift), every cap), a lift
// contributing its bearing times the lifter's strength and a refutation capping
// at 1 − that product, the smallest cap winning. Every figure is carried at full
// precision: a dependent multiplies the value its source published, not a
// rounded one. An edge whose bearing or whose source's strength is pending is
// excluded; a pending rigor leaves the node's derivation pending where a premise
// participates and its final open to a lift. A premise cycle refuses nothing:
// its members, and every node resting on one, derive nothing.
func graphStrength(g kb.Graph, edges []kb.Edge) Strength {
	w := newStrengthWalk()
	for _, n := range g.Claims {
		w.claimOrder = append(w.claimOrder, n.ID)
		w.rigor[n.ID] = n.Rigor
	}
	for _, n := range g.Analyses {
		w.analysisOrder = append(w.analysisOrder, n.ID)
		w.rigor[n.ID] = n.Rigor
	}
	for _, n := range g.Axioms {
		w.sources[n.ID] = ptr(kb.AxiomRigor)
	}
	for _, n := range g.Works {
		w.sources[n.ID] = n.Rigor
	}
	for _, n := range g.Experiments {
		if n.Status == kb.ExperimentRun {
			w.sources[n.ID] = n.Rigor
		}
	}
	w.land(edges)
	out, _ := w.run()
	return out
}
