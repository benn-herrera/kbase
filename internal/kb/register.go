package kb

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"kbase/internal/log"
)

// RegisterFile is the name of every claim-rigor register.
const RegisterFile = "claim-rigor.md"

// ListRestsOn is the register list key a claim's or analysis's premises are
// authored under. It is a list key, not a relation name.
const ListRestsOn = "rests-on"

// RigorFieldKeys is every "- <key>:" line a claim entry's Rigor section
// may carry; each folding field runs until the next of them.
var RigorFieldKeys = []string{"rigor", "strength", "rationale", ListRestsOn, RefutesKey, "references", RelationDemoted, "strengthen-by"}

// analysisFieldKeys is every "- <key>:" line an analysis entry's Rigor
// section may carry.
var analysisFieldKeys = []string{"rigor", "strength", "rationale", ListRestsOn, LiftsKey, RefutesKey}

// fieldBreakIn matches any "- <key>:" line of keys other than excluding: where
// a folding field of those keys ends.
func fieldBreakIn(keys []string, excluding string) *regexp.Regexp {
	var rest []string
	for _, k := range keys {
		if k != excluding {
			rest = append(rest, k)
		}
	}
	return PyRE(`^- (` + strings.Join(rest, "|") + `):`)
}

// fieldBreak is fieldBreakIn over a claim entry's keys.
func fieldBreak(excluding string) *regexp.Regexp { return fieldBreakIn(RigorFieldKeys, excluding) }

// The canonical id marker as the metadata verifier reads it, exactly spaced:
// a claim's, anywhere; and a claim's, an analysis's or a work's opening a
// line. The parsers below read it with any spacing.
var (
	CanonicalClaimMarkerRE = regexp.MustCompile(`<!-- id: (` + IDBody("clm") + `) -->`)
	CanonicalEntryMarkerRE = regexp.MustCompile(`^<!-- id: (` + IDBody("clm", "ana") + `|` + WorkIDPattern + `) -->`)
)

var (
	canonicalClaimIDRE = PyRE(`^<!--\s*id:\s*(` + IDBody("clm") + `)\s*-->`)
	canonicalAnyIDRE   = PyRE(`^<!--\s*id:\s*(` + IDBody("clm", "ana") + `)\s*-->`)
	canonicalNodeIDRE  = PyRE(`^<!--\s*id:\s*(` + IDBody() + `|` + WorkIDPattern + `)\s*-->`)

	claimIDRE     = PyRE(IDBody("clm"))
	premiseIDRE   = PyRE(IDBody("ana", "exp"))
	axiomRE       = PyRE(`Axiom ` + AxiomLabel)
	workKeyRE     = PyRE(`^` + WorkIDPattern)
	numberRE      = PyRE(`-?\d+(?:\.\d+)?`)
	firstParenRE  = PyRE(`\(([^()]*)\)`)
	traceRE       = PyRE(`\[[^\]]*\]\s*$`)
	placeholderRE = PyRE(`^\s*-\s*\*\(`)
	bulletLeadRE  = PyRE(`^\s*-\s*`)
	bracketRE     = PyRE(`\[([^\[\]]*)\]\s*$`)
	strengthInRE  = PyRE(`strength\s+(-?\d+(?:\.\d+)?|\*pending\*)`)
	originRE      = PyRE(`\(origin\s+([^()]*)\)`)
	// BearingRE is one "(bearing …)" annotation; group 1 is its value as
	// written, which BearingFromValue judges.
	BearingRE   = PyRE(`\(bearing\s+([^()]*)\)`)
	pairLineRE  = PyRE(`^\s*(?:-\s*)?(` + IDBody("clm", "ana") + `)\s*:\s*(.*)$`)
	subBulletRE = PyRE(`^\s+-\s+`)
	sbBulletRE  = PyRE(`^(\s+)-\s+(.*)$`)

	breakAfterRationale    = fieldBreak("rationale")
	breakAfterPremises     = fieldBreak(ListRestsOn)
	breakAfterRefutes      = fieldBreak(RefutesKey)
	breakAfterReferences   = fieldBreak("references")
	breakAfterDemoted      = fieldBreak(RelationDemoted)
	breakAfterStrengthenBy = fieldBreak("strengthen-by")
	analysisRationaleBreak = fieldBreakIn(analysisFieldKeys, "rationale")
	analysisPremisesBreak  = fieldBreakIn(analysisFieldKeys, ListRestsOn)
	analysisLiftsBreak     = fieldBreakIn(analysisFieldKeys, LiftsKey)
	analysisRefutesBreak   = fieldBreakIn(analysisFieldKeys, RefutesKey)
	workRationaleBreak     = PyRE(`^- (rigor|rationale):`)
)

// Bearing is one scoring edge's scalar, or the judgement that it has not been
// made yet: a number in [0, 1], or pending. Zero Value is the judgement 0.0;
// whether a bearing was made is Pending's, never Value's.
type Bearing struct {
	Value   float64
	Pending bool
}

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

// ParseBearing reads one "(bearing …)" annotation as the bearing it carries.
func ParseBearing(s string) (Bearing, bool) {
	m := BearingRE.FindStringSubmatch(s)
	if m == nil {
		return Bearing{}, false
	}
	return BearingFromValue(m[1])
}

// Edge is one directed claim-graph edge: Source is the supporter, Target the
// supported, and TargetKind the target's kind. SourceStrengthRecorded is the
// source's derived strength as the bullet records it, nil where it records
// none or records a pending one. Bearing is the edge's scalar where its
// relation allows one. Origin is a demoted edge's, as its bullet spells it; ""
// where it has none.
type Edge struct {
	Source, Target, Relation, TargetKind string
	SourceStrengthRecorded               *float64
	Context                              *string
	Bearing                              Bearing
	Origin                               string
}

// StrengthenByItem is one strengthen-by bullet of a claim's Rigor section.
type StrengthenByItem struct {
	ClaimID      string
	ItemIdx      int
	Text         string
	MentionedIDs []string
}

// ClaimEntry is one canonical clm- entry of a register.
type ClaimEntry struct {
	ID, Title, CanonicalPath, CanonicalAnchor string
	Rigor, Strength                           *float64
	BuildStatus                               *string
	Rationale                                 string
	Premises                                  []Edge
	StrengthenBy                              []StrengthenByItem
	Refutes                                   []Edge
	References                                []Edge
	Demoted                                   []Edge
	StrengthTrace                             string
}

// AnalysisEntry is the register-resident half of an analysis node.
type AnalysisEntry struct {
	Rigor, Strength                               *float64
	StrengthTrace, Rationale, Title, Path, Anchor string
	Premises                                      []Edge
	Lifts, Refutes                                []Edge
}

// ExternalWork is a cited work this corpus does not contain.
type ExternalWork struct {
	ID, Key, Title, CanonicalPath, CanonicalAnchor string
	Rigor                                          *float64
	Rationale                                      string
}

// RegisterEntry is one canonical marker and the heading the reader binds it
// to: the preceding "## " heading. Line indices are into the register's
// fence-scrubbed str.splitlines(); -1 is none.
type RegisterEntry struct {
	NodeID, Kind, HeadingTitle         string
	MarkerLine, HeadingLine, RigorLine int
	Adjacent                           bool
}

// Bound is whether a heading precedes the marker, so the parsers read it.
func (e RegisterEntry) Bound() bool { return e.HeadingLine >= 0 }

// sectionHeading opens the section an entry's authored fields sit in.
const sectionHeading = "### Rigor"

func scrubbedLines(text string) []string { return SplitLines(StripCodeFences(text)) }

// LocateRegisterEntries is every canonical marker of every kind in one
// register, in document order, unbound ones included.
func LocateRegisterEntries(text string) []RegisterEntry {
	lines := scrubbedLines(text)
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = Strip(l)
	}
	var found []RegisterEntry
	heading, title := -1, ""
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			heading, title = i, Strip(line[3:])
			continue
		}
		m := canonicalNodeIDRE.FindStringSubmatch(stripped[i])
		if m == nil {
			continue
		}
		adjacent := false
		if heading >= 0 {
			first := heading + 1
			for first < len(stripped) && stripped[first] == "" {
				first++
			}
			adjacent = first == i
		}
		rigor := -1
		for j := i + 1; j < len(lines); j++ {
			if stripped[j] == sectionHeading {
				rigor = j
				break
			}
			if strings.HasPrefix(lines[j], "## ") {
				break
			}
		}
		kind, _, _ := strings.Cut(m[1], "-")
		e := RegisterEntry{NodeID: m[1], Kind: kind, MarkerLine: i, HeadingLine: heading, RigorLine: rigor, Adjacent: adjacent}
		if heading >= 0 {
			e.HeadingTitle = title
		}
		found = append(found, e)
	}
	return found
}

// MisBoundEntries is, of one register's entries, those bound under a heading
// not their own: their Rigor section lies past the next "## ", or another
// marker shares their heading.
func MisBoundEntries(entries []RegisterEntry) []RegisterEntry {
	perHeading := map[int]int{}
	for _, e := range entries {
		if e.Bound() {
			perHeading[e.HeadingLine]++
		}
	}
	var out []RegisterEntry
	for _, e := range entries {
		if e.Bound() && (e.RigorLine < 0 || perHeading[e.HeadingLine] > 1) {
			out = append(out, e)
		}
	}
	return out
}

// EntryLocation is a bound entry's edit span: RigorStart is the line after
// its "### Rigor" heading, RigorEnd exclusive; both -1 where it has none.
// The span holds the entry's derived line as well as its authored fields.
type EntryLocation struct {
	NodeID                                        string
	HeadingLine, MarkerLine, RigorStart, RigorEnd int
}

// LocateEntries is every bound entry of a register with its edit span.
func LocateEntries(text string) []EntryLocation {
	lines := scrubbedLines(text)
	var out []EntryLocation
	for _, e := range LocateRegisterEntries(text) {
		if !e.Bound() {
			continue
		}
		loc := EntryLocation{NodeID: e.NodeID, HeadingLine: e.HeadingLine, MarkerLine: e.MarkerLine, RigorStart: -1, RigorEnd: -1}
		if e.RigorLine >= 0 {
			loc.RigorStart = e.RigorLine + 1
			loc.RigorEnd = nextH2(lines, loc.RigorStart)
		}
		out = append(out, loc)
	}
	return out
}

// nextH2 is the first "## " line at or after start, or len(lines).
func nextH2(lines []string, start int) int {
	for j := start; j < len(lines); j++ {
		if strings.HasPrefix(lines[j], "## ") {
			return j
		}
	}
	return len(lines)
}

// rigorStart is the "### Rigor" line an entry's marker reaches before the
// next "## ", or -1.
func rigorStart(lines []string, marker int) int {
	for j := marker + 1; j < len(lines); j++ {
		if Strip(lines[j]) == sectionHeading {
			return j
		}
		if strings.HasPrefix(lines[j], "## ") {
			break
		}
	}
	return -1
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// firstNumber is the first number in the value after a line's first colon.
func firstNumber(line string) *float64 {
	_, value, ok := strings.Cut(line, ":")
	if !ok {
		return nil
	}
	if m := numberRE.FindString(Strip(value)); m != "" {
		v := parseFloat(m)
		return &v
	}
	return nil
}

// ParseStrengthLine reads "- strength: 0.X (phrase) [trace]": the value, the
// phrase in the first parenthetical, and the trailing bracket verbatim with
// one leading space.
func ParseStrengthLine(line string) (*float64, *string, string) {
	value := Strip(line)
	if _, after, ok := strings.Cut(line, ":"); ok {
		value = Strip(after)
	}
	var strength *float64
	if m := numberRE.FindString(value); m != "" {
		v := parseFloat(m)
		strength = &v
	}
	var status *string
	if m := firstParenRE.FindStringSubmatch(value); m != nil {
		s := Strip(m[1])
		status = &s
	}
	trace := ""
	if m := traceRE.FindString(value); m != "" {
		trace = " " + Strip(m)
	}
	return strength, status, trace
}

// BulletHead is a premise bullet's text before its first " — " or " (".
func BulletHead(stripped string) string {
	cut := len(stripped)
	if i := strings.Index(stripped, " — "); i >= 0 {
		cut = min(cut, i)
	}
	if i := strings.Index(stripped, " ("); i >= 0 {
		cut = min(cut, i)
	}
	return stripped[:cut]
}

// ClaimIDs is every clm- id in s, as \b(clm-...)\b finds them.
func ClaimIDs(s string) []string { return findAllBounded(claimIDRE, s) }

// premiseNodeIDs is every ana- or exp- id in s, as \b(...)\b finds them: the
// kinds a rests-on bullet names that its claim scan does not cover.
func premiseNodeIDs(s string) []string { return findAllBounded(premiseIDRE, s) }

// idKind is the node kind an id's prefix names, "" for any other id.
func idKind(id string) string {
	switch prefix, _, _ := strings.Cut(id, "-"); prefix {
	case "clm":
		return NodeKindClaim
	case "ana":
		return NodeKindAnalysis
	case "exp":
		return NodeKindExperiment
	case WorkPrefix:
		return NodeKindWork
	}
	return ""
}

var nodeIDRE = PyRE(IDBody())

// NodeIDs is every clm-, exp- or ana- id in s, as \b(...)\b finds them.
func NodeIDs(s string) []string { return findAllBounded(nodeIDRE, s) }

// TrimBulletLead is s without a leading "- " bullet marker and the
// whitespace around it.
func TrimBulletLead(s string) string { return bulletLeadRE.ReplaceAllString(s, "") }

// WordBounded is every match of re in s bounded by Python's \b at both ends;
// exact for a pattern opening on a literal its body cannot contain.
func WordBounded(re *regexp.Regexp, s string) []string { return findAllBounded(re, s) }

// ContainsWord is whether token occurs in s bounded by Python's \b.
func ContainsWord(s, token string) bool { return containsBounded(s, token) }

// bulletReader parses entry bullets against the register's known claim ids,
// reporting each token it drops and recording each annotation fault it finds at
// the bullet's line. A nil known, axioms or authored keeps every token it would
// otherwise resolve: a reader that resolves nothing refuses nothing.
type bulletReader struct {
	known    map[string]bool
	axioms   map[string]bool
	authored map[string]IDRecord
	lg       log.Logger
	path     string
	at       int // the physical line the bullet under read opens on
	sink     readSink
}

// readSink is what a register reader reports besides the records it returns:
// the annotation faults Discover refuses, and the works its bullets cite.
type readSink struct {
	faults *[]AnnotationFault
	cites  *[]WorkCitation
}

// WorkCitation is a work id as one register's rests-on bullet cites it: the
// register, the bullet's first physical line, and the id.
type WorkCitation struct {
	Path, ID string
	Line     int
}

// cite records the work the bullet under read names, where a reader was asked
// for citations.
func (r bulletReader) cite(id string) {
	if r.sink.cites != nil {
		*r.sink.cites = append(*r.sink.cites, WorkCitation{Path: r.path, ID: id, Line: r.at})
	}
}

// WorkCitations is every work one register's bullets cite, with the bullet's
// line, from the reader the edges come from.
func WorkCitations(text, rel string) []WorkCitation {
	var cites []WorkCitation
	sink := readSink{cites: &cites}
	parseClaimEntries(text, rel, nil, nil, nil, log.Discard(), sink)
	parseAnalysisEntries(text, rel, nil, nil, nil, log.Discard(), sink)
	return cites
}

// fault records one annotation fault at the bullet under read; a reader built
// without a sink, as a lookup is, drops it.
func (r bulletReader) fault(field string, f AnnotationFault) {
	if r.sink.faults == nil {
		return
	}
	f.Field, f.Path, f.Line = field, r.path, r.at
	*r.sink.faults = append(*r.sink.faults, f)
}

func (r bulletReader) claimTargets(head, source, bullet string) []string {
	var kept []string
	for _, cid := range ClaimIDs(head) {
		if r.known != nil && !r.known[cid] {
			r.lg.Info("dropped non-claim premise target", "location", r.path+":"+source, "target", cid, "bullet", NormalizeSpace(bullet))
			continue
		}
		kept = append(kept, cid)
	}
	return kept
}

// axiomTargets is the axiom ids of one bullet's "Axiom <label>" citations that
// the axiom source declares; one it does not is dropped and logged, as a
// clm- token naming no entry is.
func (r bulletReader) axiomTargets(head, source, bullet string) []string {
	var kept []string
	for _, id := range axiomTokens(head) {
		if r.axioms != nil && !r.axioms[id] {
			r.lg.Info("dropped undeclared axiom premise target", "location", r.path+":"+source, "target", id, "bullet", NormalizeSpace(bullet))
			continue
		}
		kept = append(kept, id)
	}
	return kept
}

// nodeTargets is the ana- and exp- ids of one bullet's head that the KB authors;
// one it does not is dropped and logged, as a clm- token naming no entry is.
func (r bulletReader) nodeTargets(head, source, bullet string) []string {
	var kept []string
	for _, id := range premiseNodeIDs(head) {
		if r.authored != nil {
			if _, ok := r.authored[id]; !ok {
				r.lg.Info("dropped unauthored premise target", "location", r.path+":"+source, "target", id, "bullet", NormalizeSpace(bullet))
				continue
			}
		}
		kept = append(kept, id)
	}
	return kept
}

func bulletContext(stripped string) *string {
	if m := bracketRE.FindStringSubmatch(stripped); m != nil {
		if raw := Strip(m[1]); !strings.HasPrefix(raw, "=") {
			return &raw
		}
	}
	return nil
}

// strengthRecorded is the source's derived strength a premise bullet records,
// nil where it records none or records a pending one.
func strengthRecorded(stripped string) *float64 {
	m := strengthInRE.FindStringSubmatch(stripped)
	if m == nil || m[1] == PendingLiteral {
		return nil
	}
	v := parseFloat(m[1])
	return &v
}

// uniqueTargets is ids without repeats, first position kept.
func uniqueTargets(ids []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// referenceList reads one references or demoted bullet; a bullet of either
// list that carries a bearing is misplaced, since neither edge takes one.
func (r bulletReader) referenceList(line, source, relation, field string) []Edge {
	if placeholderRE.MatchString(line) {
		return nil
	}
	stripped := Strip(TrimBulletLead(line))
	if BearingRE.MatchString(stripped) {
		r.fault(field, AnnotationFault{Remedy: "remove the bearing token",
			Detail: "bullet carries a bearing and names no scoring target"})
		return nil
	}
	context := bulletContext(stripped)
	var out []Edge
	for _, cid := range r.claimTargets(BulletHead(stripped), source, stripped) {
		out = append(out, Edge{Source: source, Target: cid, Relation: relation, TargetKind: NodeKindClaim, Context: context})
	}
	return out
}

func (r bulletReader) references(line, source string) []Edge {
	return r.referenceList(line, source, RelationReferences, "references")
}

// demoted reads a demoted bullet as a references bullet carrying the origin
// its "(origin …)" annotation names, the last where several stand.
func (r bulletReader) demoted(line, source string) []Edge {
	edges := r.referenceList(line, source, RelationDemoted, RelationDemoted)
	origin := ""
	if m := originRE.FindAllStringSubmatch(line, -1); m != nil {
		origin = Strip(m[len(m)-1][1])
	}
	for i := range edges {
		edges[i].Origin = origin
	}
	return edges
}

// premises reads one rests-on bullet as the edges it names, flipped the one
// way the register authors them: the named node is the source, the entry the
// target. A claim or experiment premise supports the entry and an axiom or work
// premise founds it, every one of them carrying the bullet's bearing. A premise
// whose pairing BearingLegal rejects is a fault and yields no edge, as is a
// malformed one; its faults are recorded for Discover.
func (r bulletReader) premises(line, entryID, entryKind string) []Edge {
	if placeholderRE.MatchString(line) {
		return nil
	}
	stripped := Strip(TrimBulletLead(line))
	head := BulletHead(stripped)
	works := workTokens(head)
	nodes := r.nodeTargets(head, entryID, stripped)
	named := uniqueTargets(slices.Concat(ClaimIDs(head), axiomTokens(head), premiseNodeIDs(head), works))
	bearings := BearingRE.FindAllStringSubmatch(stripped, -1)
	switch {
	case len(named) > 1:
		r.fault(ListRestsOn, AnnotationFault{Remedy: "split the bullet, one scoring target each",
			Detail: "bullet names " + strings.Join(named, ", ") + "; one bullet carries one scoring target"})
		return nil
	case len(bearings) > 1:
		r.fault(ListRestsOn, AnnotationFault{Remedy: "keep one bearing token",
			Detail: "bullet carries more than one (bearing …) token"})
		return nil
	case len(named) == 0 && len(bearings) > 0:
		r.fault(ListRestsOn, AnnotationFault{Remedy: "name the claim, experiment, axiom or work the bearing judges, or remove the token",
			Detail: "bullet carries a bearing and names no scoring target"})
		return nil
	}
	var bearing Bearing
	if len(bearings) == 1 {
		var ok bool
		if bearing, ok = BearingFromValue(bearings[0][1]); !ok {
			r.fault(ListRestsOn, AnnotationFault{Allowed: "[0, 1] or " + PendingLiteral,
				Remedy: "write a number in [0, 1] or " + PendingLiteral,
				Detail: "(bearing " + Strip(bearings[0][1]) + ")"})
			return nil
		}
	}
	claims := r.claimTargets(head, entryID, stripped)
	axioms := r.axiomTargets(head, entryID, stripped)
	if len(claims)+len(axioms)+len(nodes)+len(works) > 0 && len(bearings) == 0 {
		r.fault(ListRestsOn, AnnotationFault{Remedy: "add (bearing " + PendingLiteral + ") or a judged value",
			Detail: "bullet names a scoring target and carries no bearing"})
		return nil
	}
	recorded := strengthRecorded(stripped)
	context := bulletContext(stripped)
	var edges []Edge
	for _, id := range nodes {
		kind := idKind(id)
		if !BearingLegal(RelationSupports, kind, entryKind) {
			r.fault(ListRestsOn, AnnotationFault{Remedy: "rest on a claim, an experiment, an axiom or a work",
				Detail: RelationSupports + " from " + kind + " to " + entryKind + " is no legal pairing, so " + id + " is no premise of " + entryID})
			return nil
		}
		edges = append(edges, Edge{Source: id, Target: entryID, Relation: RelationSupports, TargetKind: entryKind,
			Context: context, Bearing: bearing})
	}
	for _, cid := range claims {
		edges = append(edges, Edge{Source: cid, Target: entryID, Relation: RelationSupports, TargetKind: entryKind,
			SourceStrengthRecorded: recorded, Context: context, Bearing: bearing})
	}
	for _, axiom := range axioms {
		edges = append(edges, Edge{Source: axiom, Target: entryID, Relation: RelationFounds, TargetKind: entryKind,
			Context: context, Bearing: bearing})
	}
	for _, work := range works {
		r.cite(work)
		edges = append(edges, Edge{Source: work, Target: entryID, Relation: RelationFounds, TargetKind: entryKind,
			Context: context, Bearing: bearing})
	}
	return edges
}

// ParsePremisesBullet is the edges one rests-on bullet line records, every
// clm- token kept and no fault recorded.
func ParsePremisesBullet(line, entryID, entryKind string) []Edge {
	return bulletReader{lg: log.Discard()}.premises(line, entryID, entryKind)
}

// ParsePair reads one "<clm- or ana-id>: <bearing>" scoring pair line as the
// push edge it authors. ok is false where the line is no pair line at all; a
// pair line whose value is no bearing is malformed, never pending.
func ParsePair(line, source, relation string) (Edge, bool, error) {
	m := pairLineRE.FindStringSubmatch(Strip(line))
	if m == nil {
		return Edge{}, false, nil
	}
	bearing, ok := BearingFromValue(m[2])
	if !ok {
		return Edge{}, true, malformed("a %s pair %q carries no bearing — a number in [0, 1] or %s", relation, Strip(line), PendingLiteral)
	}
	return Edge{Source: source, Target: m[1], Relation: relation, TargetKind: pairTargetKind(m[1]), Bearing: bearing}, true, nil
}

// pairs reads one pair-bullet list as the push edges it authors, recording each
// malformed pair line at its own line; base is the physical line the list's
// first bullet sits on. A faulty line yields no edge.
func (r bulletReader) pairs(bullets []subBullet, key, source, relation string, base int) []Edge {
	var out []Edge
	for _, b := range bullets {
		r.at = base + b.at
		e, ok, err := ParsePair(b.text, source, relation)
		if err != nil {
			r.fault(key, AnnotationFault{Remedy: "write a number in [0, 1] or " + PendingLiteral,
				Detail: err.Error()})
			continue
		}
		if ok {
			out = append(out, e)
		}
	}
	return out
}

// workTokens is every work id in s not preceded by a word character or a
// hyphen, scanned as a backtracking engine with that lookbehind scans.
func workTokens(s string) []string {
	var out []string
	for pos := 0; pos < len(s); {
		i := strings.Index(s[pos:], WorkPrefix+"-")
		if i < 0 {
			break
		}
		at := pos + i
		if at > 0 {
			if r, _ := utf8.DecodeLastRuneInString(s[:at]); isWord(r) || r == '-' {
				pos = at + 1
				continue
			}
		}
		m := workKeyRE.FindString(s[at:])
		if m == "" {
			pos = at + 1
			continue
		}
		out = append(out, m)
		pos = at + len(m)
	}
	return out
}

// axiomTokens is every "Axiom <label>" citation in s as the id it names.
func axiomTokens(s string) []string {
	var out []string
	for _, tok := range findAllBounded(axiomRE, s) {
		out = append(out, AxiomPrefix+strings.TrimPrefix(tok, "Axiom "))
	}
	return out
}

func (r bulletReader) strengthenBy(lines []string, source string) []StrengthenByItem {
	var items [][]string
	var current []string
	open := false
	for _, line := range lines {
		if m := sbBulletRE.FindStringSubmatch(line); m != nil && utf8.RuneCountInString(m[1]) <= 4 {
			if open {
				items = append(items, current)
			}
			current, open = []string{m[2]}, true
		} else if open {
			current = append(current, Strip(line))
		}
	}
	if open {
		items = append(items, current)
	}
	var out []StrengthenByItem
	for idx, chunks := range items {
		text := NormalizeSpace(strings.Join(chunks, " "))
		if text == "" {
			continue
		}
		candidates := ClaimIDs(text)
		slices.Sort(candidates)
		candidates = slices.Compact(candidates)
		mentioned := []string{}
		for _, c := range candidates {
			if r.known == nil || r.known[c] {
				mentioned = append(mentioned, c)
			} else {
				r.lg.Info("dropped non-claim mention in strengthen-by", "claim", source, "item", idx, "id", c)
			}
		}
		out = append(out, StrengthenByItem{ClaimID: source, ItemIdx: idx, Text: text, MentionedIDs: mentioned})
	}
	return out
}

// subBullet is one indented bullet of a list field and the 0-based line it
// opens on among the lines the field was collected from.
type subBullet struct {
	text string
	at   int
}

// atHeading is whether s opens a heading. An entry's authored fields end at
// one — its derived line sits under a heading of its own — so no field folds a
// heading into the item above it.
func atHeading(s string) bool {
	return strings.HasPrefix(s, "## ") || strings.HasPrefix(s, "### ")
}

// collectSubBullets gathers the indented bullets of one list field from
// qlines[i:], folding continuation lines into the bullet above; it stops at
// a line stop matches, at a heading, or at a "---" rule where rule is set.
func collectSubBullets(qlines []string, i int, stop *regexp.Regexp, rule bool) ([]subBullet, int) {
	var out []subBullet
	for i < len(qlines) {
		next := qlines[i]
		ns := Strip(next)
		if stop.MatchString(ns) || atHeading(ns) || (rule && ns == "---") {
			break
		}
		switch {
		case subBulletRE.MatchString(next):
			out = append(out, subBullet{text: next, at: i})
		case ns == "":
		case len(out) > 0:
			out[len(out)-1].text += " " + ns
		}
		i++
	}
	return out, i
}

// foldRationale reads a rationale field opening at qlines[i] and its
// continuation lines, up to a blank line, a heading or a line stop matches.
func foldRationale(qlines []string, i int, stop *regexp.Regexp) (string, int) {
	_, first, _ := strings.Cut(Strip(qlines[i]), ":")
	chunks := []string{Strip(first)}
	i++
	for i < len(qlines) {
		ns := Strip(qlines[i])
		if stop.MatchString(ns) || ns == "" || atHeading(ns) {
			break
		}
		chunks = append(chunks, ns)
		i++
	}
	return NormalizeSpace(strings.Join(chunks, " ")), i
}

// ParseClaimEntries is every bound clm- entry of one register. With known
// set, a clm- token outside it is dropped from rests-on, references and
// strengthen-by mentions; every ana- and exp- token a rests-on bullet names is
// kept, since this reader resolves none. Its bullets' annotation faults are
// Discover's to raise; this reader records none.
func ParseClaimEntries(text, rel string, known map[string]bool, lg log.Logger) []ClaimEntry {
	return parseClaimEntries(text, rel, known, nil, nil, lg, readSink{})
}

// parseClaimEntries is ParseClaimEntries, resolving "Axiom <label>" citations
// against axioms and ana- and exp- tokens against authored, where either is
// set, and reporting into sink where a reader is given one.
func parseClaimEntries(text, rel string, known, axioms map[string]bool, authored map[string]IDRecord, lg log.Logger, sink readSink) []ClaimEntry {
	lines := scrubbedLines(text)
	r := bulletReader{known: known, axioms: axioms, authored: authored, lg: lg, path: rel, sink: sink}
	var out []ClaimEntry
	for _, e := range LocateRegisterEntries(text) {
		if e.Kind != "clm" || !e.Bound() {
			continue
		}
		entry := ClaimEntry{ID: e.NodeID, Title: e.HeadingTitle, CanonicalPath: rel, CanonicalAnchor: Slugify(e.HeadingTitle)}
		if qs := rigorStart(lines, e.MarkerLine); qs >= 0 {
			qlines := lines[qs+1 : nextH2(lines, qs+1)]
			for i := 0; i < len(qlines); {
				s := Strip(qlines[i])
				switch {
				case strings.HasPrefix(s, "- rigor:"):
					entry.Rigor = firstNumber(s)
					i++
				case strings.HasPrefix(s, "- strength:"):
					entry.Strength, entry.BuildStatus, entry.StrengthTrace = ParseStrengthLine(s)
					i++
				case strings.HasPrefix(s, "- rationale:"):
					entry.Rationale, i = foldRationale(qlines, i, breakAfterRationale)
				case strings.HasPrefix(s, "- "+ListRestsOn+":"):
					var bullets []subBullet
					bullets, i = collectSubBullets(qlines, i+1, breakAfterPremises, false)
					for _, b := range bullets {
						r.at = qs + b.at + 2
						entry.Premises = append(entry.Premises, r.premises(b.text, e.NodeID, NodeKindClaim)...)
					}
				case strings.HasPrefix(s, "- "+RefutesKey+":"):
					var bullets []subBullet
					bullets, i = collectSubBullets(qlines, i+1, breakAfterRefutes, true)
					entry.Refutes = append(entry.Refutes, r.pairs(bullets, RefutesKey, e.NodeID, RelationRefutes, qs+2)...)
				case strings.HasPrefix(s, "- references:"):
					var bullets []subBullet
					bullets, i = collectSubBullets(qlines, i+1, breakAfterReferences, true)
					for _, b := range bullets {
						r.at = qs + b.at + 2
						entry.References = append(entry.References, r.references(b.text, e.NodeID)...)
					}
				case strings.HasPrefix(s, "- "+RelationDemoted+":"):
					var bullets []subBullet
					bullets, i = collectSubBullets(qlines, i+1, breakAfterDemoted, true)
					for _, b := range bullets {
						r.at = qs + b.at + 2
						entry.Demoted = append(entry.Demoted, r.demoted(b.text, e.NodeID)...)
					}
				case strings.HasPrefix(s, "- strengthen-by:"):
					i++
					var sb []string
					for i < len(qlines) {
						ns := Strip(qlines[i])
						if breakAfterStrengthenBy.MatchString(ns) || ns == "---" || atHeading(ns) {
							break
						}
						sb = append(sb, qlines[i])
						i++
					}
					entry.StrengthenBy = r.strengthenBy(sb, e.NodeID)
				default:
					i++
				}
			}
		}
		out = append(out, entry)
	}
	return out
}

// ParseAnalysisEntries is every bound ana- entry's register half, in first
// occurrence order; a repeated id keeps its first position and last value. Its
// bullets' annotation faults are Discover's to raise; this reader records
// none, and it resolves no ana- or exp- token a rests-on bullet names.
func ParseAnalysisEntries(text, rel string, known map[string]bool, lg log.Logger) ([]string, map[string]AnalysisEntry, error) {
	return parseAnalysisEntries(text, rel, known, nil, nil, lg, readSink{})
}

// parseAnalysisEntries is ParseAnalysisEntries, resolving "Axiom <label>"
// citations against axioms and ana- and exp- tokens against authored, where
// either is set, and reporting into sink where a reader is given one.
func parseAnalysisEntries(text, rel string, known, axioms map[string]bool, authored map[string]IDRecord, lg log.Logger, sink readSink) ([]string, map[string]AnalysisEntry, error) {
	lines := scrubbedLines(text)
	r := bulletReader{known: known, axioms: axioms, authored: authored, lg: lg, path: rel, sink: sink}
	var order []string
	out := map[string]AnalysisEntry{}
	for _, e := range LocateRegisterEntries(text) {
		if e.Kind != "ana" || !e.Bound() {
			continue
		}
		sq := AnalysisEntry{Title: e.HeadingTitle, Path: rel, Anchor: Slugify(e.HeadingTitle)}
		if qs := rigorStart(lines, e.MarkerLine); qs >= 0 {
			qlines := lines[qs+1 : nextH2(lines, qs+1)]
			for i := 0; i < len(qlines); {
				s := Strip(qlines[i])
				switch {
				case strings.HasPrefix(s, "- rigor:"):
					sq.Rigor = firstNumber(s)
					i++
				case strings.HasPrefix(s, "- strength:"):
					sq.Strength, _, sq.StrengthTrace = ParseStrengthLine(s)
					i++
				case strings.HasPrefix(s, "- rationale:"):
					sq.Rationale, i = foldRationale(qlines, i, analysisRationaleBreak)
				case strings.HasPrefix(s, "- "+ListRestsOn+":"):
					var bullets []subBullet
					bullets, i = collectSubBullets(qlines, i+1, analysisPremisesBreak, true)
					for _, b := range bullets {
						r.at = qs + b.at + 2
						sq.Premises = append(sq.Premises, r.premises(b.text, e.NodeID, NodeKindAnalysis)...)
					}
				case strings.HasPrefix(s, "- "+LiftsKey+":"):
					var bullets []subBullet
					bullets, i = collectSubBullets(qlines, i+1, analysisLiftsBreak, true)
					sq.Lifts = append(sq.Lifts, r.pairs(bullets, LiftsKey, e.NodeID, RelationLifts, qs+2)...)
				case strings.HasPrefix(s, "- "+RefutesKey+":"):
					var bullets []subBullet
					bullets, i = collectSubBullets(qlines, i+1, analysisRefutesBreak, true)
					sq.Refutes = append(sq.Refutes, r.pairs(bullets, RefutesKey, e.NodeID, RelationRefutes, qs+2)...)
				default:
					i++
				}
			}
		}
		if _, seen := out[e.NodeID]; !seen {
			order = append(order, e.NodeID)
		}
		out[e.NodeID] = sq
	}
	return order, out, nil
}

// ParseWorkEntries is every bound wrk- entry of one register that has a
// Rigor section.
func ParseWorkEntries(text, rel string) []ExternalWork {
	lines := scrubbedLines(text)
	var out []ExternalWork
	for _, e := range LocateRegisterEntries(text) {
		if e.Kind != WorkPrefix || !e.Bound() || e.RigorLine < 0 {
			continue
		}
		w := ExternalWork{ID: e.NodeID, Key: WorkKey(e.NodeID), Title: e.HeadingTitle, CanonicalPath: rel, CanonicalAnchor: Slugify(e.HeadingTitle)}
		qlines := lines[e.RigorLine+1 : nextH2(lines, e.RigorLine+1)]
		for i := 0; i < len(qlines); {
			s := Strip(qlines[i])
			switch {
			case strings.HasPrefix(s, "- rigor:"):
				w.Rigor = firstNumber(s)
				i++
			case strings.HasPrefix(s, "- rationale:"):
				w.Rationale, i = foldRationale(qlines, i, workRationaleBreak)
			default:
				i++
			}
		}
		out = append(out, w)
	}
	return out
}

// FooterBand is where one entry's leaf-references footer lives or would:
// BodyStart is the line after its marker, RigorStart its "### Rigor"
// line, FooterLine an existing footer's line or -1.
type FooterBand struct {
	NodeID                            string
	BodyStart, RigorStart, FooterLine int
}

// LocateLeafReferenceFooters is every clm- and ana- entry's footer band, on
// scrubbed lines, in first occurrence order; a repeated id keeps its first
// position and its last band.
func LocateLeafReferenceFooters(text string) []FooterBand {
	lines := scrubbedLines(text)
	var out []FooterBand
	at := map[string]int{}
	for i, line := range lines {
		m := canonicalAnyIDRE.FindStringSubmatch(Strip(line))
		if m == nil {
			continue
		}
		qs := rigorStart(lines, i)
		if qs < 0 {
			continue
		}
		band := FooterBand{NodeID: m[1], BodyStart: i + 1, RigorStart: qs, FooterLine: -1}
		for idx := band.BodyStart; idx < qs; idx++ {
			if strings.HasPrefix(lines[idx], LeafReferencesPrefix) {
				band.FooterLine = idx
				break
			}
		}
		if j, ok := at[band.NodeID]; ok {
			out[j] = band
			continue
		}
		at[band.NodeID] = len(out)
		out = append(out, band)
	}
	return out
}

// CanonicalClaimIDs is every clm- id a register's canonical markers key.
func CanonicalClaimIDs(text string) []string {
	var out []string
	for _, line := range scrubbedLines(text) {
		if m := canonicalClaimIDRE.FindStringSubmatch(Strip(line)); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}
