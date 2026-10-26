package kb

import (
	"math"
	"regexp"
	"strconv"
)

// RegisterFile is the name of every claim-rigor register.
const RegisterFile = "claim-rigor.md"

// ListRestsOn is the register list key a claim's or analysis's premises are
// authored under. It is a list key, not a relation name.
const ListRestsOn = "rests-on"

var claimIDRE = PyRE(IDBody(ClaimPrefix))

// BearingFromValue reads one bearing value as written: a number in [0, 1], or
// the pending literal.
func BearingFromValue(s string) (Bearing, bool) {
	s = Strip(s)
	if s == PendingLiteral {
		return Bearing{Pending: true}, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || v < 0 || v > 1 {
		return Bearing{}, false
	}
	return Bearing{Value: v}, true
}

// Edge is one directed claim-graph edge: Source is the supporter, Target the
// supported, and TargetKind the target's kind. Context is the record's, nil
// where it has none. Bearing is the edge's scalar where its relation allows
// one. ManuallyScored is the record's manually-scored flag, nil where its type
// carries none.
type Edge struct {
	Source, Target, Relation, TargetKind string
	Context                              *string
	Bearing                              Bearing
	ManuallyScored                       *bool
}

func scrubbedLines(text string) []string { return SplitLines(StripCodeFences(text)) }

// ClaimIDs is every clm- id in s, as \b(clm-...)\b finds them.
func ClaimIDs(s string) []string { return findAllBounded(claimIDRE, s) }

var nodeIDRE = PyRE(IDBody())

// NodeIDs is every clm-, exp- or ana- id in s, as \b(...)\b finds them.
func NodeIDs(s string) []string { return findAllBounded(nodeIDRE, s) }

// WordBounded is every match of re in s bounded by Python's \b at both ends;
// exact for a pattern opening on a literal its body cannot contain.
func WordBounded(re *regexp.Regexp, s string) []string { return findAllBounded(re, s) }
