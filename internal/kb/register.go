package kb

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"kbase/internal/log"
)

// RegisterFile is the name of every claim-quality register.
const RegisterFile = "claim-quality.md"

// QualityFieldKeys is every "- <key>:" line a claim entry's Quality section
// may carry; each folding field runs until the next of them.
var QualityFieldKeys = []string{"confidence", "solidity", "rationale", "depends-on", "references", RelationDemoted, "strengthen-by"}

func fieldBreak(excluding string) *regexp.Regexp {
	var keys []string
	for _, k := range QualityFieldKeys {
		if k != excluding {
			keys = append(keys, k)
		}
	}
	return PyRE(`^- (` + strings.Join(keys, "|") + `):`)
}

// The canonical id marker as the metadata verifier reads it, exactly spaced:
// a claim's, anywhere; and a claim's, a support's or a work's opening a line.
// The parsers below read it with any spacing.
var (
	CanonicalClaimMarkerRE = regexp.MustCompile(`<!-- id: (` + IDBody("clm") + `) -->`)
	CanonicalEntryMarkerRE = regexp.MustCompile(`^<!-- id: (` + IDBody("clm", "sup") + `|` + WorkIDPattern + `) -->`)
)

var (
	canonicalClaimIDRE = PyRE(`^<!--\s*id:\s*(` + IDBody("clm") + `)\s*-->`)
	canonicalAnyIDRE   = PyRE(`^<!--\s*id:\s*(` + IDBody("clm", "sup") + `)\s*-->`)
	canonicalNodeIDRE  = PyRE(`^<!--\s*id:\s*(` + IDBody() + `|` + WorkIDPattern + `)\s*-->`)

	claimIDRE     = PyRE(IDBody("clm"))
	invariantRE   = PyRE(`INVARIANT-[A-Z]+[0-9]+`)
	axiomRE       = PyRE(`Axiom [0-9]+`)
	workKeyRE     = PyRE(`^` + WorkIDPattern)
	numberRE      = PyRE(`-?\d+(?:\.\d+)?`)
	firstParenRE  = PyRE(`\(([^()]*)\)`)
	traceRE       = PyRE(`\[[^\]]*\]\s*$`)
	placeholderRE = PyRE(`^\s*-\s*\*\(`)
	bulletLeadRE  = PyRE(`^\s*-\s*`)
	bracketRE     = PyRE(`\[([^\[\]]*)\]\s*$`)
	solidityInRE  = PyRE(`solidity\s+(-?\d+(?:\.\d+)?)`)
	originRE      = PyRE(`\(origin\s+([^()]*)\)`)
	// ApplicabilityRE is a work-target bullet's "applicability <value>"; group
	// 1 is the value.
	ApplicabilityRE = PyRE(`applicability\s+(-?\d+(?:\.\d+)?|\*pending\*)`)
	subBulletRE     = PyRE(`^\s+-\s+`)
	sbBulletRE      = PyRE(`^(\s+)-\s+(.*)$`)
	supportsPair    = PyRE(`^\s*(?:-\s*)?(` + IDBody("clm") + `)\s*:\s*(-?\d+(?:\.\d+)?|\*pending\*)\s*$`)

	breakAfterRationale    = fieldBreak("rationale")
	breakAfterDependsOn    = fieldBreak("depends-on")
	breakAfterReferences   = fieldBreak("references")
	breakAfterDemoted      = fieldBreak(RelationDemoted)
	breakAfterStrengthenBy = fieldBreak("strengthen-by")
	supportRationaleBreak  = PyRE(`^- (quality|solidity|rationale|depends-on|supports):`)
	supportDependsOnBreak  = PyRE(`^- (quality|solidity|rationale|supports):`)
	workRationaleBreak     = PyRE(`^- (strength|rationale):`)
)

// Fraction is an on-point fraction: absent, pending, or a value.
type Fraction struct {
	Set, Pending bool
	Value        float64
}

// Edge is one forward claim-graph edge as its source's entry records it.
// Origin is a demoted edge's, as its bullet spells it; "" where it has none.
type Edge struct {
	Source, Target, Relation, TargetKind string
	TargetSolidityRecorded               *float64
	Strength                             *float64
	Context                              *string
	Fraction                             Fraction
	Origin                               string
}

// StrengthenByItem is one strengthen-by bullet of a claim's Quality section.
type StrengthenByItem struct {
	ClaimID      string
	ItemIdx      int
	Text         string
	MentionedIDs []string
}

// ClaimEntry is one canonical clm- entry of a register.
type ClaimEntry struct {
	ID, Title, CanonicalPath, CanonicalAnchor string
	Confidence, Solidity                      *float64
	BuildStatus                               *string
	Rationale                                 string
	DependsOn                                 []Edge
	StrengthenBy                              []StrengthenByItem
	References                                []Edge
	Demoted                                   []Edge
	SolidityTrace                             string
}

// SupportQuality is the register-resident half of a support node.
type SupportQuality struct {
	Quality, Solidity                             *float64
	SolidityTrace, Rationale, Title, Path, Anchor string
	DependsOn                                     []Edge
}

// ExternalWork is a cited work this corpus does not contain.
type ExternalWork struct {
	ID, Key, Title, CanonicalPath, CanonicalAnchor string
	Strength                                       *float64
	Rationale                                      string
}

// RegisterEntry is one canonical marker and the heading the reader binds it
// to: the preceding "## " heading. Line indices are into the register's
// fence-scrubbed str.splitlines(); -1 is none.
type RegisterEntry struct {
	NodeID, Kind, HeadingTitle           string
	MarkerLine, HeadingLine, QualityLine int
	Adjacent                             bool
}

// Bound is whether a heading precedes the marker, so the parsers read it.
func (e RegisterEntry) Bound() bool { return e.HeadingLine >= 0 }

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
		quality := -1
		for j := i + 1; j < len(lines); j++ {
			if stripped[j] == "### Quality" {
				quality = j
				break
			}
			if strings.HasPrefix(lines[j], "## ") {
				break
			}
		}
		kind, _, _ := strings.Cut(m[1], "-")
		e := RegisterEntry{NodeID: m[1], Kind: kind, MarkerLine: i, HeadingLine: heading, QualityLine: quality, Adjacent: adjacent}
		if heading >= 0 {
			e.HeadingTitle = title
		}
		found = append(found, e)
	}
	return found
}

// MisBoundEntries is, of one register's entries, those bound under a heading
// not their own: their Quality section lies past the next "## ", or another
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
		if e.Bound() && (e.QualityLine < 0 || perHeading[e.HeadingLine] > 1) {
			out = append(out, e)
		}
	}
	return out
}

// EntryLocation is a bound entry's edit span: QualityStart is the line after
// its "### Quality" heading, QualityEnd exclusive; both -1 where it has none.
type EntryLocation struct {
	NodeID                                            string
	HeadingLine, MarkerLine, QualityStart, QualityEnd int
}

// LocateEntries is every bound entry of a register with its edit span.
func LocateEntries(text string) []EntryLocation {
	lines := scrubbedLines(text)
	var out []EntryLocation
	for _, e := range LocateRegisterEntries(text) {
		if !e.Bound() {
			continue
		}
		loc := EntryLocation{NodeID: e.NodeID, HeadingLine: e.HeadingLine, MarkerLine: e.MarkerLine, QualityStart: -1, QualityEnd: -1}
		if e.QualityLine >= 0 {
			loc.QualityStart = e.QualityLine + 1
			loc.QualityEnd = nextH2(lines, loc.QualityStart)
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

// qualityStart is the "### Quality" line an entry's marker reaches before the
// next "## ", or -1.
func qualityStart(lines []string, marker int) int {
	for j := marker + 1; j < len(lines); j++ {
		if Strip(lines[j]) == "### Quality" {
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

// ParseSolidityLine reads "- solidity: 0.X (phrase) [trace]": the value, the
// phrase in the first parenthetical, and the trailing bracket verbatim with
// one leading space.
func ParseSolidityLine(line string) (*float64, *string, string) {
	value := Strip(line)
	if _, after, ok := strings.Cut(line, ":"); ok {
		value = Strip(after)
	}
	var solidity *float64
	if m := numberRE.FindString(value); m != "" {
		v := parseFloat(m)
		solidity = &v
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
	return solidity, status, trace
}

// BulletHead is a depends-on bullet's text before its first " — " or " (".
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

var nodeIDRE = PyRE(IDBody())

// NodeIDs is every clm-, exp- or sup- id in s, as \b(...)\b finds them.
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
// reporting each clm- token it drops.
type bulletReader struct {
	known map[string]bool
	lg    log.Logger
	path  string
}

func (r bulletReader) claimTargets(head, source, bullet string) []string {
	var kept []string
	for _, cid := range ClaimIDs(head) {
		if r.known != nil && !r.known[cid] {
			r.lg.Info("dropped non-claim depends-on target", "location", r.path+":"+source, "target", cid, "bullet", NormalizeSpace(bullet))
			continue
		}
		kept = append(kept, cid)
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

func (r bulletReader) references(line, source string) []Edge {
	if placeholderRE.MatchString(line) {
		return nil
	}
	stripped := Strip(TrimBulletLead(line))
	context := bulletContext(stripped)
	var out []Edge
	for _, cid := range r.claimTargets(BulletHead(stripped), source, stripped) {
		out = append(out, Edge{Source: source, Target: cid, Relation: RelationReferences, TargetKind: "claim", Context: context})
	}
	return out
}

// demoted reads a demoted bullet as a references bullet carrying the origin
// its "(origin …)" annotation names, the last where several stand.
func (r bulletReader) demoted(line, source string) []Edge {
	edges := r.references(line, source)
	origin := ""
	if m := originRE.FindAllStringSubmatch(line, -1); m != nil {
		origin = Strip(m[len(m)-1][1])
	}
	for i := range edges {
		edges[i].Relation, edges[i].Origin = RelationDemoted, origin
	}
	return edges
}

func (r bulletReader) dependsOn(line, source string) []Edge {
	if placeholderRE.MatchString(line) {
		return nil
	}
	stripped := Strip(TrimBulletLead(line))
	head := BulletHead(stripped)
	var parenContext *string
	if m := firstParenRE.FindStringSubmatch(stripped); m != nil {
		s := Strip(m[1])
		parenContext = &s
	}
	bracketContext := bulletContext(stripped)
	var targetSolidity *float64
	if m := solidityInRE.FindStringSubmatch(stripped); m != nil {
		v := parseFloat(m[1])
		targetSolidity = &v
	}
	var edges []Edge
	for _, cid := range r.claimTargets(head, source, stripped) {
		edges = append(edges, Edge{Source: source, Target: cid, Relation: RelationDepends, TargetKind: "claim",
			TargetSolidityRecorded: targetSolidity, Context: bracketContext})
	}
	for _, label := range findAllBounded(invariantRE, head) {
		edges = append(edges, Edge{Source: source, Target: label, Relation: RelationDepends, TargetKind: "invariant", Context: parenContext})
	}
	for _, axiom := range findAllBounded(axiomRE, head) {
		edges = append(edges, Edge{Source: source, Target: "axiom-" + strings.TrimPrefix(axiom, "Axiom "),
			Relation: RelationDepends, TargetKind: "axiom", Context: parenContext})
	}
	var fraction Fraction
	if m := ApplicabilityRE.FindStringSubmatch(stripped); m != nil {
		fraction.Set = true
		if m[1] == PendingLiteral {
			fraction.Pending = true
		} else {
			fraction.Value = parseFloat(m[1])
		}
	}
	for _, work := range workTokens(head) {
		edges = append(edges, Edge{Source: source, Target: work, Relation: RelationRestsOn, TargetKind: "work",
			Context: bracketContext, Fraction: fraction})
	}
	return edges
}

// ParseDependsOnBullet is the edges one depends-on bullet line records, every
// clm- token kept.
func ParseDependsOnBullet(line, source string) []Edge {
	return bulletReader{lg: log.Discard()}.dependsOn(line, source)
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

// collectSubBullets gathers the indented bullets of one list field from
// qlines[i:], folding continuation lines into the bullet above; it stops at
// a line stop matches, or at a "---" rule where rule is set.
func collectSubBullets(qlines []string, i int, stop *regexp.Regexp, rule bool) ([]string, int) {
	var out []string
	for i < len(qlines) {
		next := qlines[i]
		ns := Strip(next)
		if stop.MatchString(ns) || (rule && ns == "---") {
			break
		}
		switch {
		case subBulletRE.MatchString(next):
			out = append(out, next)
		case ns == "":
		case len(out) > 0:
			out[len(out)-1] += " " + ns
		}
		i++
	}
	return out, i
}

// foldRationale reads a rationale field opening at qlines[i] and its
// continuation lines, up to a blank line or a line stop matches.
func foldRationale(qlines []string, i int, stop *regexp.Regexp) (string, int) {
	_, first, _ := strings.Cut(Strip(qlines[i]), ":")
	chunks := []string{Strip(first)}
	i++
	for i < len(qlines) {
		ns := Strip(qlines[i])
		if stop.MatchString(ns) || ns == "" {
			break
		}
		chunks = append(chunks, ns)
		i++
	}
	return NormalizeSpace(strings.Join(chunks, " ")), i
}

// ParseClaimEntries is every bound clm- entry of one register. With known
// set, a clm- token outside it is dropped from depends-on, references and
// strengthen-by mentions.
func ParseClaimEntries(text, rel string, known map[string]bool, lg log.Logger) []ClaimEntry {
	lines := scrubbedLines(text)
	r := bulletReader{known: known, lg: lg, path: rel}
	var out []ClaimEntry
	for _, e := range LocateRegisterEntries(text) {
		if e.Kind != "clm" || !e.Bound() {
			continue
		}
		entry := ClaimEntry{ID: e.NodeID, Title: e.HeadingTitle, CanonicalPath: rel, CanonicalAnchor: Slugify(e.HeadingTitle)}
		if qs := qualityStart(lines, e.MarkerLine); qs >= 0 {
			qlines := lines[qs+1 : nextH2(lines, qs+1)]
			for i := 0; i < len(qlines); {
				s := Strip(qlines[i])
				switch {
				case strings.HasPrefix(s, "- confidence:"):
					entry.Confidence = firstNumber(s)
					i++
				case strings.HasPrefix(s, "- solidity:"):
					entry.Solidity, entry.BuildStatus, entry.SolidityTrace = ParseSolidityLine(s)
					i++
				case strings.HasPrefix(s, "- rationale:"):
					entry.Rationale, i = foldRationale(qlines, i, breakAfterRationale)
				case strings.HasPrefix(s, "- depends-on:"):
					var bullets []string
					bullets, i = collectSubBullets(qlines, i+1, breakAfterDependsOn, false)
					for _, b := range bullets {
						entry.DependsOn = append(entry.DependsOn, r.dependsOn(b, e.NodeID)...)
					}
				case strings.HasPrefix(s, "- references:"):
					var bullets []string
					bullets, i = collectSubBullets(qlines, i+1, breakAfterReferences, true)
					for _, b := range bullets {
						entry.References = append(entry.References, r.references(b, e.NodeID)...)
					}
				case strings.HasPrefix(s, "- "+RelationDemoted+":"):
					var bullets []string
					bullets, i = collectSubBullets(qlines, i+1, breakAfterDemoted, true)
					for _, b := range bullets {
						entry.Demoted = append(entry.Demoted, r.demoted(b, e.NodeID)...)
					}
				case strings.HasPrefix(s, "- strengthen-by:"):
					i++
					var sb []string
					for i < len(qlines) {
						ns := Strip(qlines[i])
						if breakAfterStrengthenBy.MatchString(ns) || ns == "---" {
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

// ParseSupportEntries is every bound sup- entry's register half, in first
// occurrence order; a repeated id keeps its first position and last value.
func ParseSupportEntries(text, rel string, known map[string]bool, lg log.Logger) ([]string, map[string]SupportQuality) {
	lines := scrubbedLines(text)
	r := bulletReader{known: known, lg: lg, path: rel}
	var order []string
	out := map[string]SupportQuality{}
	for _, e := range LocateRegisterEntries(text) {
		if e.Kind != "sup" || !e.Bound() {
			continue
		}
		sq := SupportQuality{Title: e.HeadingTitle, Path: rel, Anchor: Slugify(e.HeadingTitle)}
		if qs := qualityStart(lines, e.MarkerLine); qs >= 0 {
			qlines := lines[qs+1 : nextH2(lines, qs+1)]
			for i := 0; i < len(qlines); {
				s := Strip(qlines[i])
				switch {
				case strings.HasPrefix(s, "- quality:"):
					sq.Quality = firstNumber(s)
					i++
				case strings.HasPrefix(s, "- solidity:"):
					sq.Solidity, _, sq.SolidityTrace = ParseSolidityLine(s)
					i++
				case strings.HasPrefix(s, "- rationale:"):
					sq.Rationale, i = foldRationale(qlines, i, supportRationaleBreak)
				case strings.HasPrefix(s, "- depends-on:"):
					var bullets []string
					bullets, i = collectSubBullets(qlines, i+1, supportDependsOnBreak, true)
					for _, b := range bullets {
						sq.DependsOn = append(sq.DependsOn, r.dependsOn(b, e.NodeID)...)
					}
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
	return order, out
}

// ParseWorkEntries is every bound work- entry of one register that has a
// Quality section.
func ParseWorkEntries(text, rel string) []ExternalWork {
	lines := scrubbedLines(text)
	var out []ExternalWork
	for _, e := range LocateRegisterEntries(text) {
		if e.Kind != WorkPrefix || !e.Bound() || e.QualityLine < 0 {
			continue
		}
		w := ExternalWork{ID: e.NodeID, Key: WorkKey(e.NodeID), Title: e.HeadingTitle, CanonicalPath: rel, CanonicalAnchor: Slugify(e.HeadingTitle)}
		qlines := lines[e.QualityLine+1 : nextH2(lines, e.QualityLine+1)]
		for i := 0; i < len(qlines); {
			s := Strip(qlines[i])
			switch {
			case strings.HasPrefix(s, "- strength:"):
				w.Strength = firstNumber(s)
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

// SupportPair is one supports: beneficiary and its on-point fraction.
type SupportPair struct {
	ClaimID  string
	Fraction Fraction
}

// ParseSupportPair reads one "<clm-id>: <fraction>" supports pair line.
func ParseSupportPair(line string) (SupportPair, bool) {
	m := supportsPair.FindStringSubmatch(line)
	if m == nil {
		return SupportPair{}, false
	}
	p := SupportPair{ClaimID: m[1], Fraction: Fraction{Set: true}}
	if m[2] == PendingLiteral {
		p.Fraction.Pending = true
	} else {
		p.Fraction.Value = parseFloat(m[2])
	}
	return p, true
}

// ParseStagedSupports is the supports: pairs staged in one register's sup-
// entries, every sup- entry keyed, in first occurrence order.
func ParseStagedSupports(text string) ([]string, map[string][]SupportPair) {
	var order []string
	staged := map[string][]SupportPair{}
	sup, inSupports := "", false
	for _, line := range scrubbedLines(text) {
		s := Strip(line)
		if m := canonicalNodeIDRE.FindStringSubmatch(s); m != nil {
			sup, inSupports = "", false
			if strings.HasPrefix(m[1], "sup-") {
				sup = m[1]
				if _, ok := staged[sup]; !ok {
					order = append(order, sup)
					staged[sup] = nil
				}
			}
			continue
		}
		if sup == "" {
			continue
		}
		if p, ok := ParseSupportPair(line); inSupports && ok {
			staged[sup] = append(staged[sup], p)
			continue
		}
		if strings.HasPrefix(s, "- ") {
			inSupports = s == "- supports:"
		}
	}
	return order, staged
}

// FooterBand is where one entry's leaf-references footer lives or would:
// BodyStart is the line after its marker, QualityStart its "### Quality"
// line, FooterLine an existing footer's line or -1.
type FooterBand struct {
	NodeID                              string
	BodyStart, QualityStart, FooterLine int
}

// LocateLeafReferenceFooters is every clm- and sup- entry's footer band, on
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
		qs := qualityStart(lines, i)
		if qs < 0 {
			continue
		}
		band := FooterBand{NodeID: m[1], BodyStart: i + 1, QualityStart: qs, FooterLine: -1}
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
