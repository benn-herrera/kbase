// Package index derives the metadata layer's derived fields and the
// .index/*.yaml files from a KB's authored metadata, and checks a KB's
// freshness, links and citations against them.
package index

import (
	"fmt"
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

// Finals is every claim's non-pending final strength.
func (s Strength) Finals() map[string]float64 {
	out := map[string]float64{}
	for id, r := range s.Results {
		if r.Final != nil {
			out[id] = *r.Final
		}
	}
	return out
}

// AnalysisFinals is every analysis's non-pending strength.
func (s Strength) AnalysisFinals() map[string]float64 {
	out := map[string]float64{}
	for id, r := range s.AnalysisResults {
		if r.Final != nil {
			out[id] = *r.Final
		}
	}
	return out
}

// CycleError is a premise graph with a cycle; Members is every node the walk
// could not reach, sorted.
type CycleError struct{ Members []string }

func (e CycleError) Error() string {
	return fmt.Sprintf("claim premise graph has a cycle among %d node(s): %s", len(e.Members), strings.Join(e.Members, ", "))
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
// lift standing over it is the whole expression; no premise at all leaves the
// derivation 0, which is the base a lift and a cap compose around.
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
// its premises' contributions, or 0 where no premise contributes — a rigor caps
// what the premises establish and is no premise itself. The final is that value
// raised to the largest lift and capped by the smallest refutation. A pending
// rigor over a premise set leaves the derivation pending, and a lift still
// stands it; over no premise it caps nothing, and the node reads 0.
func derive(rigor *float64, premises, lifts, caps []kb.Edge, strength map[string]*float64) Result {
	r := Result{Rigor: rigor}
	r.MinDep = MinDependencyStrength(premises, strength)
	switch {
	case r.MinDep == nil:
		r.Derivation = ptr(0)
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
// that set are not ordered and not waited on.
func derivationOrder(nodes []string, deps map[string][]string) ([]string, error) {
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
	var order []string
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
	if len(order) != len(indegree) {
		var members []string
		for id, d := range indegree {
			if d > 0 {
				members = append(members, id)
			}
		}
		slices.Sort(members)
		return nil, CycleError{members}
	}
	return order, nil
}

// ComputeStrength derives every claim's and analysis's strength.
//
// A node's derivation is min(its rigor, every participating premise's
// contribution), and 0 where no premise participates: rigor caps what the
// premises establish and is no premise itself, so a node nothing carries reads
// 0 whatever its rigor, judged or not. A claim or analysis premise contributes
// its bearing times its final strength, and a work or axiom premise its bearing
// times the source's rigor — a work's judged once by a person, an axiom's 1.0 by
// rule. A final strength is min(max(derivation, every lift), every cap), a lift
// contributing its bearing times the lifter's strength and a refutation capping
// at 1 − that product, the smallest cap winning. Every figure is carried at full
// precision: a dependent multiplies the value its source published, not a
// rounded one. A claim's refutations, an analysis's and an experiment's lifts
// and refutations all land on the node they name. An edge whose bearing or
// whose source's strength is pending is excluded; a pending rigor leaves the
// node's derivation pending where a premise participates and its final open to a
// lift. A non-run experiment's edges are excluded as an unjudged premise's are.
// A cycle among claims and analyses is a CycleError, scored or not.
func ComputeStrength(st kb.State) (Strength, error) {
	claims := map[string]kb.ClaimEntry{}
	var claimOrder []string
	for _, e := range st.ClaimEntries {
		if _, ok := claims[e.ID]; !ok {
			claimOrder = append(claimOrder, e.ID)
		}
		claims[e.ID] = e
	}
	analyses := map[string]kb.AnalysisNode{}
	var analysisOrder []string
	for _, s := range st.Analyses {
		if _, ok := analyses[s.ID]; !ok {
			analysisOrder = append(analysisOrder, s.ID)
		}
		analyses[s.ID] = s
	}
	nodes := slices.Concat(claimOrder, analysisOrder)
	inWalk := map[string]bool{}
	for _, id := range nodes {
		inWalk[id] = true
	}

	// premises, lifts and caps are every scoring edge authored off the node's
	// own lists — a claim's refutations, an analysis's lifts and refutations, an
	// experiment's supports, lifts and refutations — by the node they land on.
	premises, lifts, caps := map[string][]kb.Edge{}, map[string][]kb.Edge{}, map[string][]kb.Edge{}
	addArrows := func(edges []kb.Edge, into map[string][]kb.Edge) {
		for _, e := range edges {
			into[e.Target] = append(into[e.Target], e)
		}
	}
	for _, e := range st.ClaimEntries {
		addArrows(e.Refutes, caps)
	}
	for _, s := range st.Analyses {
		addArrows(s.Lifts, lifts)
		addArrows(s.Refutes, caps)
	}
	for _, x := range st.Experiments {
		if x.Status != "run" {
			continue
		}
		addArrows(x.Supports, premises)
		addArrows(x.Lifts, lifts)
		addArrows(x.Refutes, caps)
	}

	deps := map[string][]string{}
	premisesOf := func(id string, edges []kb.Edge) {
		for _, e := range edges {
			if kb.Scoring(e.Relation) && inWalk[e.Source] {
				deps[id] = append(deps[id], e.Source)
			}
		}
	}
	for id, e := range claims {
		premisesOf(id, e.Premises)
	}
	for id, s := range analyses {
		premisesOf(id, s.Premises)
	}
	for _, arrows := range []map[string][]kb.Edge{lifts, caps} {
		for target, edges := range arrows {
			if inWalk[target] {
				premisesOf(target, edges)
			}
		}
	}
	order, err := derivationOrder(nodes, deps)
	if err != nil {
		return Strength{}, err
	}

	// strength holds every source's value the walk has: an axiom its rigor of
	// 1.0, a work its rigor, an experiment its rigor where it ran, and a claim
	// or analysis the final value computed below.
	strength := map[string]*float64{}
	for _, n := range st.Axioms {
		strength[n.ID] = ptr(kb.AxiomRigor)
	}
	for _, w := range st.Works {
		strength[w.ID] = w.Rigor
	}
	for _, x := range st.Experiments {
		if x.Status == "run" {
			strength[x.ID] = x.Rigor
		}
	}

	out := Strength{Results: map[string]Result{}, AnalysisResults: map[string]Result{}}
	for _, id := range order {
		var rigor *float64
		var local []kb.Edge
		if e, ok := claims[id]; ok {
			rigor, local = e.Rigor, e.Premises
		} else {
			rigor, local = analyses[id].Rigor, analyses[id].Premises
		}
		r := derive(rigor, slices.Concat(local, premises[id]), lifts[id], caps[id], strength)
		strength[id] = r.Final
		if _, ok := claims[id]; ok {
			out.Results[id] = r
		} else {
			out.AnalysisResults[id] = r
		}
	}
	return out, nil
}
