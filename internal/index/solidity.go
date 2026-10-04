// Package index derives the metadata layer's derived fields and the
// .index/*.jsonl files from a KB's authored metadata, and checks a KB's
// freshness, links and citations against them.
package index

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
)

// Result is one claim's derived solidity: the derivation (gating, min)
// branch, the experimental (max) branch and their max, with the inputs the
// derivation consumed — local quality after any support lift, and the
// dependency minimum. Nil is pending.
type Result struct {
	Derivation, Experimental, Final, LocalQuality, MinDep *float64
}

// Solidity is the one computation every derived solidity is read from.
type Solidity struct {
	Results     map[string]Result
	SupSolidity map[string]*float64
}

// Finals is every claim's non-pending final solidity.
func (s Solidity) Finals() map[string]float64 {
	out := map[string]float64{}
	for id, r := range s.Results {
		if r.Final != nil {
			out[id] = *r.Final
		}
	}
	return out
}

// SupFinals is every support's non-pending solidity.
func (s Solidity) SupFinals() map[string]float64 {
	out := map[string]float64{}
	for id, v := range s.SupSolidity {
		if v != nil {
			out[id] = *v
		}
	}
	return out
}

// CycleError is a depends-on graph with a cycle; Members is every node the
// walk could not reach, sorted.
type CycleError struct{ Members []string }

func (e CycleError) Error() string {
	return fmt.Sprintf("claim depends-on graph has a cycle among %d claim(s): %s", len(e.Members), strings.Join(e.Members, ", "))
}

func ptr(v float64) *float64 { return &v }

// RoundHalfUp2 rounds to two decimals, ties away from zero, on the value's
// shortest decimal spelling.
func RoundHalfUp2(v float64) float64 {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	frac += "000"
	cents, _ := new(big.Int).SetString(whole+frac[:2], 10)
	if frac[2] >= '5' {
		cents.Add(cents, big.NewInt(1))
	}
	digits := cents.String()
	for len(digits) < 3 {
		digits = "0" + digits
	}
	out := digits[:len(digits)-2] + "." + digits[len(digits)-2:]
	if neg {
		out = "-" + out
	}
	r, _ := strconv.ParseFloat(out, 64)
	return r
}

// FormatSolidity is a score as the KB writes it: two decimals, or the
// pending literal.
func FormatSolidity(v *float64) string {
	if v == nil {
		return kb.PendingLiteral
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// BuildStatusPhrase is the band phrase for a solidity, or nil for a pending one.
func BuildStatusPhrase(v *float64) *string {
	if b := kb.BandFor(v); b != nil {
		return &b.StatusPhrase
	}
	return nil
}

// BuildBand is the band slug for a solidity.
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

// maxNonNil is the larger of two values where both are set, else whichever is.
func maxNonNil(a, b *float64) *float64 {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case *b > *a:
		return b
	}
	return a
}

// RenderSolidityTrace is the arithmetic suffix for the branch the
// computation took, leading space included, or "".
func RenderSolidityTrace(r Result) string {
	if r.Final == nil {
		return ""
	}
	if r.Experimental != nil && (r.Derivation == nil || *r.Experimental > *r.Derivation) {
		if r.Derivation == nil {
			return " [= experimental " + FormatSolidity(r.Experimental) + "]"
		}
		return " [= max(" + FormatSolidity(r.Derivation) + ", " + FormatSolidity(r.Experimental) + ")]"
	}
	return RenderMinTrace(r.LocalQuality, r.MinDep)
}

// RenderMinTrace is the weakest-link suffix " [= min(base, minDep)]", or ""
// where either is pending.
func RenderMinTrace(base, minDep *float64) string {
	if base == nil || minDep == nil {
		return ""
	}
	return " [= min(" + FormatSolidity(base) + ", " + FormatSolidity(minDep) + ")]"
}

// MinDependencySolidity is the least solidity a node's depends edges feed
// it — a framework target counting 1.0 — or nil where it has none or a claim
// target has no solidity.
func MinDependencySolidity(edges []kb.Edge, finals map[string]float64) *float64 {
	var deps []float64
	for _, e := range edges {
		if e.Relation != "depends" {
			continue
		}
		if e.TargetKind == "claim" {
			v, ok := finals[e.Target]
			if !ok {
				return nil
			}
			deps = append(deps, v)
		} else {
			deps = append(deps, 1.0)
		}
	}
	if len(deps) == 0 {
		return nil
	}
	return ptr(minOf(deps[0], deps[1:]...))
}

type supporter struct {
	supID    string
	fraction kb.Fraction
}

// dependencyOrder is Kahn's order over every claim and support: a node after
// every claim it depends on, a claim after every support supporting it.
func dependencyOrder(entries map[string]kb.ClaimEntry, sups map[string]kb.SupportNode, supporters map[string][]supporter) ([]string, error) {
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for id := range entries {
		indegree[id] = 0
	}
	for id := range sups {
		indegree[id] = 0
	}
	addEdge := func(before, after string) {
		_, b := indegree[before]
		_, a := indegree[after]
		if a && b {
			indegree[after]++
			dependents[before] = append(dependents[before], after)
		}
	}
	for id, e := range entries {
		for _, edge := range e.DependsOn {
			if edge.Relation == "depends" && edge.TargetKind == "claim" {
				addEdge(edge.Target, id)
			}
		}
		for _, s := range supporters[id] {
			addEdge(s.supID, id)
		}
	}
	for id, s := range sups {
		for _, edge := range s.DependsOn {
			if edge.Relation == "depends" && edge.TargetKind == "claim" {
				addEdge(edge.Target, id)
			}
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
		deps := slices.Clone(dependents[node])
		slices.Sort(deps)
		for _, d := range deps {
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

// ComputeSolidity derives every claim's and support's solidity.
//
// A claim's derivation is round2(min(local quality, each claim dependency's
// final, each gating work's strength)), framework dependencies counting 1.0;
// it is pending where its local quality is, or any claim dependency's final.
// Local quality is max(confidence, each supporting support's solidity times
// its on-point fraction), a pending lift excluded rather than poisoning. A
// rests-on edge gates by membership: fraction 0 skips it, a positive one puts
// the work's strength in the min, and a pending or absent fraction or
// strength makes the derivation pending. The experimental branch is the
// largest strength of a run experiment's strengthens edge; the final is the
// larger of the two branches. A support's solidity is round2(min(quality,
// its claim dependencies' finals)). A cycle anywhere among claims and
// supports is a CycleError, scored or not.
func ComputeSolidity(st kb.State) (Solidity, error) {
	entries := map[string]kb.ClaimEntry{}
	var entryOrder []string
	for _, e := range st.ClaimEntries {
		if _, ok := entries[e.ID]; !ok {
			entryOrder = append(entryOrder, e.ID)
		}
		entries[e.ID] = e
	}
	sups := map[string]kb.SupportNode{}
	for _, s := range st.Supports {
		sups[s.ID] = s
	}
	strengthOf := map[string]*float64{}
	for _, w := range st.Works {
		strengthOf[w.ID] = w.Strength
	}
	experimental := map[string]float64{}
	for _, exp := range st.Experiments {
		if exp.Status != "run" {
			continue
		}
		for _, p := range exp.Strengthens {
			if prev, ok := experimental[p.ClaimID]; !ok || p.Strength > prev {
				experimental[p.ClaimID] = p.Strength
			}
		}
	}
	expOf := func(id string) *float64 {
		if v, ok := experimental[id]; ok {
			return ptr(v)
		}
		return nil
	}
	supporters := map[string][]supporter{}
	for _, s := range st.Supports {
		for _, p := range s.Supports {
			supporters[p.ClaimID] = append(supporters[p.ClaimID], supporter{s.ID, p.Fraction})
		}
	}
	order, err := dependencyOrder(entries, sups, supporters)
	if err != nil {
		return Solidity{}, err
	}
	scored := map[string]bool{}
	for id, e := range entries {
		_, lifted := supporters[id]
		scored[id] = e.Confidence != nil || lifted
	}

	out := Solidity{Results: map[string]Result{}, SupSolidity: map[string]*float64{}}
	final := map[string]*float64{}
	for _, id := range entryOrder {
		if !scored[id] {
			x := expOf(id)
			out.Results[id] = Result{Experimental: x, Final: x}
			final[id] = x
		}
	}
	for _, node := range order {
		if s, ok := sups[node]; ok {
			out.SupSolidity[node] = supSolidity(s, final)
			continue
		}
		if !scored[node] {
			continue
		}
		entry := entries[node]
		local := entry.Confidence
		for _, sp := range supporters[node] {
			if sp.fraction.Pending {
				continue
			}
			sol := out.SupSolidity[sp.supID]
			if sol == nil {
				continue
			}
			if lift := *sol * sp.fraction.Value; local == nil || lift > *local {
				local = ptr(lift)
			}
		}
		var deps []float64
		pending := local == nil
		if !pending {
		edges:
			for _, edge := range entry.DependsOn {
				switch {
				case edge.Relation == "rests-on":
					f := edge.Fraction
					if f.Set && !f.Pending && f.Value == 0 {
						continue
					}
					if !f.Set || f.Pending {
						pending = true
						break edges
					}
					strength := strengthOf[edge.Target]
					if strength == nil {
						pending = true
						break edges
					}
					deps = append(deps, *strength)
				case edge.Relation != "depends":
				case edge.TargetKind == "claim":
					f := final[edge.Target]
					if f == nil {
						pending = true
						break edges
					}
					deps = append(deps, *f)
				default:
					deps = append(deps, 1.0)
				}
			}
		}
		var minDep, derivation *float64
		switch {
		case pending:
		case len(deps) > 0:
			minDep = ptr(minOf(deps[0], deps[1:]...))
			derivation = ptr(RoundHalfUp2(minOf(*local, deps...)))
		default:
			derivation = ptr(RoundHalfUp2(*local))
		}
		x := expOf(node)
		fin := maxNonNil(derivation, x)
		final[node] = fin
		out.Results[node] = Result{Derivation: derivation, Experimental: x, Final: fin, LocalQuality: local, MinDep: minDep}
	}
	return out, nil
}

func supSolidity(s kb.SupportNode, final map[string]*float64) *float64 {
	if s.Quality == nil {
		return nil
	}
	var deps []float64
	for _, e := range s.DependsOn {
		if e.Relation != "depends" {
			continue
		}
		if e.TargetKind == "claim" {
			f := final[e.Target]
			if f == nil {
				return nil
			}
			deps = append(deps, *f)
		} else {
			deps = append(deps, 1.0)
		}
	}
	if len(deps) == 0 {
		return s.Quality
	}
	return ptr(RoundHalfUp2(minOf(*s.Quality, deps...)))
}
