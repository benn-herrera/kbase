package from100to200

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format100"
	"kbase/internal/formats/format200"
)

// A 1.0.0 claim fuses its statement with the argument for it: one register
// entry carries the judged rigor beside the premise list that argued it, and
// the derived strength is the minimum over both. 2.0.0 derives a claim's
// strength from its premises alone, so the fusion cannot survive the move as
// it stands. Every 1.0.0 claim therefore splits: the argument — its premise
// list and its rationale — becomes an analysis of its own, the claim keeps its
// statement and its rigor, and it rests on that analysis alone. An argument
// with no premise of its own is an analysis as it stands, the walk deriving its
// own rigor rather than a claim's zero.
const (
	// analysisPrefix opens the analysis minted for a claim; idBodyAlphabet is
	// the alphabet a minted id's body draws on, in the order the live format
	// runs it. A step to an earlier form spells both rather than importing the
	// live format's definition.
	analysisPrefix = "ana-"
	idBodyAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

	// bearingFull is the bearing a claim's own edge to its analysis carries:
	// the analysis is the claim's whole argument, so nothing weights it.
	bearingFull = "1.0"
)

// split is one 1.0.0 claim's argument as the analysis it becomes: the claim's
// id, the analysis minted for it, the register holding the claim's entry, and
// the leaf hosting the analysis.
type split struct {
	claim, analysis string
	register, host  string
}

// splits is every claim a 1.0.0 KB fuses, by claim id.
type splits struct{ byClaim map[string]split }

// planSplits is every 1.0.0 claim's analysis. A KB holding no 1.0.0 register
// fuses nothing and splits nothing.
func planSplits(in common.Files) (splits, error) {
	sp := splits{byClaim: map[string]split{}}
	var registers []string
	for p := range in {
		if path.Base(p) == format100.Register100Name {
			registers = append(registers, p)
		}
	}
	if len(registers) == 0 {
		return sp, nil
	}
	slices.Sort(registers)
	docs, err := format200.Documents(in)
	if err != nil {
		return sp, err
	}
	var claims []split
	seen := map[string]bool{}
	for _, p := range registers {
		for _, id := range registerClaims(string(in[p])) {
			if seen[id] {
				// A claim keyed by two registers is one node: the 2.0.x
				// inventory keeps the first entry, and so does the split.
				continue
			}
			seen[id] = true
			claims = append(claims, split{claim: id, register: p})
		}
	}
	hosts := hostsOf(in, docs, claims)
	held := heldAnalysisBodies(in, registers, docs)
	for _, c := range claims {
		c.host = hosts[c.claim]
		if c.host == "" {
			// A claim no leaf carries keeps its argument where it is. The
			// split would move its premises onto an analysis no leaf declares
			// — which the 2.0.x load reads as no node — and leave the claim
			// resting on nothing.
			continue
		}
		c.analysis = analysisID(strings.TrimPrefix(c.claim, "clm-"), held)
		sp.byClaim[c.claim] = c
	}
	return sp, nil
}

// registerClaims is every claim a register binds to a heading, in document
// order. An entry standing above no heading is no entry to the 2.0.0 load,
// which reads an entry from the heading it sits under.
func registerClaims(text string) []string {
	var out []string
	heading := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			heading = true
			continue
		}
		if m := format100.EntryMarkerRE.FindStringSubmatch(strings.TrimSpace(line)); m != nil && heading {
			if strings.HasPrefix(m[1], "clm-") {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// heldAnalysisBodies is every id body the KB's analyses already hold: a 1.0.0
// analysis's own suffix, which the conversion writes under the analysis
// prefix. Both places a 1.0.0 KB declares one are read — a register entry and
// a leaf's support-nodes declaration.
func heldAnalysisBodies(in common.Files, registers, docs []string) map[string]bool {
	held := map[string]bool{}
	hold := func(id string) {
		if body, ok := strings.CutPrefix(id, "sup-"); ok {
			held[body] = true
		}
	}
	for _, p := range registers {
		for _, line := range strings.Split(string(in[p]), "\n") {
			if m := format100.EntryMarkerRE.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				hold(m[1])
			}
		}
	}
	for _, rel := range docs {
		for _, line := range strings.Split(string(in[path.Join(format200.KBDir, rel)]), "\n") {
			if m := format100.AnalysisIDRE.FindStringSubmatch(line); m != nil {
				hold(m[2])
			}
		}
	}
	return held
}

// analysisID is the analysis minted for a claim: the claim's own suffix under
// the analysis prefix, and where the KB already holds that id, the next body in
// the order a minted id's digits run. A 1.0.0 KB whose analysis and claim share
// a suffix is the case that forces it — the two would otherwise convert to one
// id, which the load refuses.
func analysisID(body string, held map[string]bool) string {
	for held[body] {
		body = nextBody(body)
	}
	held[body] = true
	return analysisPrefix + body
}

// nextBody is one id body past another, its characters read as digits of
// idBodyAlphabet. The walk past a held body ends: a KB holds finitely many ids
// and the alphabet spells far more than one KB can.
func nextBody(body string) string {
	b := []byte(body)
	for i := len(b) - 1; i >= 0; i-- {
		j := strings.IndexByte(idBodyAlphabet, b[i])
		if j < 0 || j == len(idBodyAlphabet)-1 {
			b[i] = idBodyAlphabet[0]
			continue
		}
		b[i] = idBodyAlphabet[j+1]
		return string(b)
	}
	return string(b)
}

// hostsOf is the leaf each claim's statement is read from, by claim id: the
// first leaf in path order that both lists and marks it, else the first that
// lists it, else the first that marks it. The ranking mirrors the 3.0.0 claim
// rule (from200to300's placeMarkers): a listing leaf is where the claim's
// statement lands, so the analysis must follow the listing leaf and not the
// marker's leaf — otherwise a claim marked in one leaf and listed in another
// strands the analysis away from its claim. A claim no leaf carries gets no
// host, and the split leaves it as it stands.
//
// The markers are the 1.0.0 bytes' own: the host is chosen before the
// conversion renames the marker's word.
func hostsOf(in common.Files, docs []string, claims []split) map[string]string {
	type leaf struct {
		path           string
		listed, marked map[string]bool
	}
	var leaves []leaf
	for _, rel := range docs {
		text := string(in[path.Join(format200.KBDir, rel)])
		kind, listed := leafClaims(text)
		if kind != format200.DocumentLeaf {
			continue
		}
		l := leaf{path: path.Join(format200.KBDir, rel), listed: map[string]bool{}, marked: map[string]bool{}}
		for _, id := range listed {
			l.listed[id] = true
		}
		for _, m := range claimMarker100RE.FindAllStringSubmatch(text, -1) {
			l.marked[m[1]] = true
		}
		leaves = append(leaves, l)
	}
	hosts := make(map[string]string, len(claims))
	for _, c := range claims {
		host, best := "", 3
		for _, l := range leaves {
			rank := 3
			switch {
			case l.listed[c.claim] && l.marked[c.claim]:
				rank = 0
			case l.listed[c.claim]:
				rank = 1
			case l.marked[c.claim]:
				rank = 2
			}
			if rank < best {
				host, best = l.path, rank
			}
		}
		hosts[c.claim] = host
	}
	return hosts
}

// hosted is the analyses the document at p hosts, in claim id order.
func (sp splits) hosted(p string) []split {
	var out []split
	for _, s := range sp.byClaim {
		if s.host == p {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b split) int { return strings.Compare(a.claim, b.claim) })
	return out
}

// registered is the analyses whose claim's entry stands in the register at p,
// by claim id.
func (sp splits) registered(p string) map[string]split {
	out := map[string]split{}
	for id, s := range sp.byClaim {
		if s.register == p {
			out[id] = s
		}
	}
	return out
}

// claimMarker100RE is a statement marker naming one claim as a 1.0.0 leaf
// carries it: the spelling the hosts are found under, before the conversion
// renames the marker's word.
var claimMarker100RE = regexp.MustCompile(`<!--\s*claim-quality\s*:\s*(clm-[a-z0-9]{6})\s*-->`)

// claimMarkerRE is that marker as the converted leaf carries it, under the word
// format100.Tier2Marker100RE renames it to.
var claimMarkerRE = regexp.MustCompile(`<!--\s*claim-rigor\s*:\s*(clm-[a-z0-9]{6})\s*-->`)

// analysisMarker is the statement marker hosting one analysis.
func analysisMarker(id string) string { return "<!-- claim-rigor: " + id + " -->" }

// placeMarkers is text with each analysis's statement marker beside its
// claim's: on the claim's own marker line, after the claim's marker, so the
// claim's block reads where it read before and the analysis's section opens
// over it. A claim the leaf lists but does not mark takes its marker on its own
// line after the leaf's first heading — where the 3.0.0 conversion writes that
// claim's own marker, and where nothing joins the heading's text, which titles
// the nodes the leaf hosts.
func (sp splits) placeMarkers(text, p string) string {
	var after []string
	for _, a := range sp.hosted(p) {
		var placed bool
		if text, placed = insertMarkerAfterClaim(text, a); !placed {
			after = append(after, analysisMarker(a.analysis))
		}
	}
	if len(after) > 0 {
		text = afterFirstHeading(text, after...)
	}
	return text
}

// insertMarkerAfterClaim is text with a's marker after the first marker naming
// its claim, and whether one stood.
func insertMarkerAfterClaim(text string, a split) (string, bool) {
	for _, m := range claimMarkerRE.FindAllStringSubmatchIndex(text, -1) {
		if text[m[2]:m[3]] != a.claim {
			continue
		}
		at := m[1]
		return text[:at] + " " + analysisMarker(a.analysis) + text[at:], true
	}
	return text, false
}

// afterFirstHeading is text with one line per marker after its first heading,
// outside any fenced block, and unchanged where it has none.
func afterFirstHeading(text string, markers ...string) string {
	lines := strings.Split(text, "\n")
	fences := format200.Fences(lines)
	for i, line := range lines {
		if slices.ContainsFunc(fences, func(f format200.Fence) bool { return f.Start <= i && i < f.End }) {
			continue
		}
		if format200.FirstHeading(line) == "" {
			continue
		}
		out := slices.Concat(lines[:i+1], markers, lines[i+1:])
		return strings.Join(out, "\n")
	}
	return text
}

// leafClaims is a leaf's kind and the claims its claims: key lists, as its
// frontmatter spells them.
func leafClaims(text string) (kind string, claims []string) {
	lines := strings.Split(text, "\n")
	start, end := frontmatterBounds(lines)
	if start < 0 {
		return "", nil
	}
	listed := false
	for _, line := range lines[start+1 : end] {
		if key, isKey := topKey(line); isKey {
			listed = key == "claims"
			if key == "kind" {
				_, value, _ := strings.Cut(line, ":")
				kind = strings.TrimSpace(value)
			}
			if !listed {
				continue
			}
		}
		if listed {
			claims = append(claims, format100.ClaimTokenRE.FindAllString(line, -1)...)
		}
	}
	return kind, claims
}

// analysisNodesBlock is the leaf frontmatter a hosted analysis is declared by:
// one analysis-nodes entry each, lifting the claim it argues for at full
// bearing.
func analysisNodesBlock(analyses []split) []string {
	var out []string
	for _, a := range analyses {
		out = append(out,
			"  - "+format200.AnaIDKey+": "+a.analysis,
			"    "+format200.LiftsKey+":",
			"      - "+a.claim+": "+bearingFull)
	}
	return out
}
