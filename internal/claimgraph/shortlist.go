package claimgraph

import (
	"cmp"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"kbase/internal/kb"
	"kbase/internal/write"
)

// shortlistK is how many of a source's best-ranked targets the
// unmarked-reference stage asks about.
const shortlistK = 5

// shortlistStopwords are words too common to tell two claims apart, and the
// LaTeX control words the fold leaves behind.
var shortlistStopwords = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(`the and for that this with from are is be as by of to in on at or an it its not no
		which when where then than these those their there such can may must will would into
		under over between each every both only also more most any all one two has have had
		was were been being what how why who whom our we us but if so do does done per via
		text mathrm frac left right cdot quad mathbb begin end href data reference type class
		span id label eqref ref qquad operatorname displaystyle`) {
		out[w] = true
	}
	return out
}()

var (
	// shortlistSymbolRE is a maths symbol: a control word or a single letter,
	// carrying one subscript or none — a bare letter needs the subscript,
	// since x alone tells no two claims apart.
	shortlistSymbolRE = regexp.MustCompile(`\\[A-Za-z]+(?:_\{[^{}]*\}|_[A-Za-z0-9])?|[A-Za-z](?:_\{[^{}]*\}|_[A-Za-z0-9])`)
	// bracedSubscriptRE is a one-character braced subscript; it is unbraced
	// where that character is a word character.
	bracedSubscriptRE = regexp.MustCompile(`_\{(.)\}`)
)

// statementMaths is the LaTeX of every inline maths span, then of every
// display fence, in text order.
func statementMaths(text string) []string {
	lines := kb.SplitLines(text)
	fences, _ := mathFenceExtents(lines)
	var out []string
	for _, span := range kb.InlineMathRE.FindAllString(text, -1) {
		out = append(out, span[2:len(span)-2])
	}
	for _, f := range fences {
		out = append(out, strings.Join(lines[f[0]+1:f[1]-1], "\n"))
	}
	return out
}

func statementSymbols(text string) []string {
	var out []string
	for _, span := range statementMaths(text) {
		for _, raw := range shortlistSymbolRE.FindAllString(span, -1) {
			name, _, _ := strings.Cut(strings.TrimLeft(raw, `\`), "_")
			if shortlistStopwords[strings.ToLower(name)] {
				continue
			}
			symbol := bracedSubscriptRE.ReplaceAllStringFunc(strings.ReplaceAll(raw, " ", ""), func(m string) string {
				if r, _ := utf8.DecodeRuneInString(m[2:]); kb.IsWord(r) {
					return "_" + string(r)
				}
				return m
			})
			out = append(out, "m:"+symbol)
		}
	}
	return out
}

// shortlistTerms is text's terms: its folded words longer than two letters,
// then its maths symbols, prefixed m:.
func shortlistTerms(text string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(write.CanonicalForm(text), kb.IsSpace) {
		if utf8.RuneCountInString(w) > 2 && !allDigits(w) && !shortlistStopwords[w] {
			out = append(out, w)
		}
	}
	return append(out, statementSymbols(text)...)
}

func allDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// pySum is Python's sum over floats: left to right, Neumaier-compensated.
func pySum(values []float64) float64 {
	sum, c := 0.0, 0.0
	for _, x := range values {
		t := sum + x
		if math.Abs(sum) >= math.Abs(x) {
			c += (sum - t) + x
		} else {
			c += (x - t) + sum
		}
		sum = t
	}
	if c != 0 && !math.IsInf(c, 0) && !math.IsNaN(c) {
		sum += c
	}
	return sum
}

// termVector is an L2-normalised term-weight vector, its terms in the order
// they first occur, since the order a sum adds in is part of its value.
type termVector struct {
	terms   []string
	weights map[string]float64
}

// weighTerms is the sublinear-tf vector of words over idf, normalised.
func weighTerms(words []string, idf map[string]float64) termVector {
	v := termVector{weights: map[string]float64{}}
	tf := map[string]int{}
	for _, w := range words {
		if tf[w] == 0 {
			v.terms = append(v.terms, w)
		}
		tf[w]++
	}
	squares := make([]float64, 0, len(v.terms))
	kept := v.terms[:0]
	for _, term := range v.terms {
		weight, ok := idf[term]
		if !ok {
			continue
		}
		w := (1 + math.Log(float64(tf[term]))) * weight
		kept = append(kept, term)
		v.weights[term] = w
		squares = append(squares, float64(w*w))
	}
	v.terms = kept
	norm := math.Sqrt(pySum(squares))
	if norm == 0 {
		norm = 1
	}
	for _, term := range v.terms {
		v.weights[term] /= norm
	}
	return v
}

func termCosine(a, b termVector) float64 {
	if len(a.terms) > len(b.terms) {
		a, b = b, a
	}
	products := make([]float64, len(a.terms))
	for i, term := range a.terms {
		products[i] = float64(a.weights[term] * b.weights[term])
	}
	return pySum(products)
}

// rankShortlist is, per source, every other node of statements best first by
// the cosine of TF-IDF vectors — sublinear tf, smoothed IDF over the
// statements — ties to the lower id, less every pair whose unordered form is
// already a candidate.
func rankShortlist(statements map[string]string, sources []string, candidatePairs []pair) map[string][]string {
	excluded := map[pair]bool{}
	for _, p := range candidatePairs {
		excluded[p] = true
		excluded[pair{p.target, p.source}] = true
	}
	ids := make([]string, 0, len(statements))
	bags := map[string][]string{}
	frequency := map[string]int{}
	for id, s := range statements {
		ids = append(ids, id)
		bags[id] = shortlistTerms(s)
		seen := map[string]bool{}
		for _, term := range bags[id] {
			if !seen[term] {
				seen[term] = true
				frequency[term]++
			}
		}
	}
	slices.Sort(ids)
	idf := map[string]float64{}
	for term, df := range frequency {
		idf[term] = math.Log(float64(1+len(statements))/float64(1+df)) + 1
	}
	vectors := map[string]termVector{}
	for id, words := range bags {
		vectors[id] = weighTerms(words, idf)
	}
	ranked := map[string][]string{}
	for _, source := range sources {
		type scored struct {
			score  float64
			target string
		}
		var all []scored
		for _, target := range ids {
			if target != source && !excluded[pair{source, target}] {
				all = append(all, scored{termCosine(vectors[source], vectors[target]), target})
			}
		}
		slices.SortFunc(all, func(x, y scored) int {
			if c := cmp.Compare(-x.score, -y.score); c != 0 {
				return c
			}
			return strings.Compare(x.target, y.target)
		})
		targets := make([]string, len(all))
		for i, s := range all {
			targets[i] = s.target
		}
		ranked[source] = targets
	}
	return ranked
}
