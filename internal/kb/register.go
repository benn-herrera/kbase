package kb

import (
	"math"
	"regexp"
	"strconv"

	"kbase/internal/formats/format400"
)

// The name of every summary page, kb-root's and each volume's, its title, and
// the headings every entry renders under its own title's.
const (
	RegisterFile           = format400.SummaryFile
	SummaryTitle           = format400.SummaryTitle
	SummaryRigorHeading    = format400.SummaryRigorHeading
	SummaryStrengthHeading = format400.SummaryStrengthHeading
)

// Volumes is the volumes among paths, kb-root-relative documents: the
// top-level directories holding an index.md.
func Volumes(paths []string) map[string]bool { return format400.Volumes(paths) }

// SummaryPage is the kb-root-relative summary page a node hosted at host has
// its entry on: the page of the volume host lies in, or the root page where it
// lies in none.
func SummaryPage(host string, volumes map[string]bool) string {
	return format400.SummaryPage(host, volumes)
}

// SummaryPagePositions is every kb-root-relative path a summary page can
// stand at, sorted: RegisterFile and each volume's page.
func SummaryPagePositions(volumes map[string]bool) []string {
	return format400.SummaryPagePositions(volumes)
}

// SummaryEntryHeadings is the text of every heading a summary page renders
// for an entry titled title, in order: the title's, its rigor section's and,
// where its node carries a derived strength, its strength section's.
func SummaryEntryHeadings(title string, derived bool) []string {
	return format400.SummaryEntryHeadings(title, derived)
}

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
// carries none. ReplacedRelation is a demoted record's, "" on every other.
type Edge struct {
	Source, Target, Relation, TargetKind string
	ReplacedRelation                     string
	Context                              *string
	Bearing                              Bearing
	ManuallyScored                       *bool
}

func scrubbedLines(text string) []string { return SplitLines(StripCodeFences(text)) }

// ClaimIDs is every clm- id in s, as \b(clm-...)\b finds them.
func ClaimIDs(s string) []string { return findAllBounded(claimIDRE, s) }

var (
	nodeIDRE   = PyRE(IDBody())
	hostedIDRE = PyRE(IDBody(HostedIDKinds...))
)

// NodeIDs is every node id of any kind in s, as \b(...)\b finds them.
func NodeIDs(s string) []string { return findAllBounded(nodeIDRE, s) }

// hostedIDs is every clm-, exp- or ana- id in s, as \b(...)\b finds them: the
// ids a statement marker hosts.
func hostedIDs(s string) []string { return findAllBounded(hostedIDRE, s) }

// WordBounded is every match of re in s bounded by Python's \b at both ends;
// exact for a pattern opening on a literal its body cannot contain.
func WordBounded(re *regexp.Regexp, s string) []string { return findAllBounded(re, s) }
