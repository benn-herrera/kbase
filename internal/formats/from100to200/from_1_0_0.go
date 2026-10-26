// Package from100to200 is the 1.0.0 → 2.0.0 conversion step: a 1.0.0
// KB's files as bytes in, the 2.0.0 form's files as bytes out, plus the paths
// that form no longer has.
package from100to200

import (
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format100"
	"kbase/internal/formats/format200"
)

// The 1.0.0 register keys the 2.0.0 form renames, by the kind of entry that
// authors them: the entry's marker decides, so a key of another kind is
// carried as text and read by nobody.
var renamedFields = map[string]map[string]string{
	"clm":  {"confidence": "rigor", "depends-on": "rests-on", "solidity": "strength"},
	"sup":  {"quality": "rigor", "depends-on": "rests-on", "solidity": "strength", "supports": "lifts"},
	"work": {"strength": "rigor"},
}

// annotationTokenRE is a trailing parenthetical that is an annotation of
// either form, and so no context.
var annotationTokenRE = regexp.MustCompile(`^\((?:strength|solidity|bearing|applicability|origin)\b[^()]*\)$`)

// Convert100To200 is the 1.0.0 → 2.0.0 step: the register's score lines and
// premise bullets in the three quantities' names, every scoring edge carrying
// a bearing, each claim's argument split out into the analysis beside it, the
// register file under the name the 2.0.0 form reads, the axiom declarations in
// the document the 2.0.0 form reads, and the derived index files the 2.0.0
// form no longer has declared obsolete — the index is derived, and a refresh
// writes it.
func Convert100To200(in common.Files) (common.Files, []string, error) {
	out := make(common.Files, len(in))
	axioms, obsolete, err := axioms100To200(in)
	if err != nil {
		return nil, nil, err
	}
	sp, err := planSplits(in)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range slices.Sorted(maps.Keys(in)) {
		if slices.Contains(obsolete, p) {
			continue
		}
		b := in[p]
		switch {
		case p == format100.AgentsPath:
			out[p] = b
		case path.Ext(p) == ".md":
			renamed := registerPath200(p)
			out[renamed] = document100To200(string(b), sp, p)
			if renamed != p {
				obsolete = append(obsolete, p)
			}
		case p == format100.IndexDir+"/depends-on.yaml" || p == format100.IndexDir+"/supported-by.yaml":
			obsolete = append(obsolete, p)
		default:
			out[p] = b
		}
	}
	if axioms != nil {
		out[format200.AxiomsPath] = axioms
	}
	return out, obsolete, nil
}

// registerPath200 is p with a 1.0.0 register's basename renamed to the 2.0.0
// form; every other path stands. The match is the basename alone, so a
// document merely naming "claim-quality" is untouched.
func registerPath200(p string) string {
	if path.Base(p) != format100.Register100Name {
		return p
	}
	return path.Join(path.Dir(p), format200.RegisterFile)
}

// linkTargetREs match the path part of a Markdown link target, up to any
// `#anchor`, angle bracket or title: group 1 is the text before it, group 2
// the path. The first is an inline `[text](target)`, the second a
// reference-style `[label]: target`.
var linkTargetREs = []*regexp.Regexp{
	regexp.MustCompile(`(\]\(\s*<?)([^)\s>#]+)`),
	regexp.MustCompile(`(?m)^(\s{0,3}\[[^\]]+\]:\s*<?)([^\s>#]+)`),
}

// linkTargets100To200 is text with every Markdown link target naming a
// 1.0.0 register renamed to the 2.0.0 form, its anchor, brackets and title
// kept.
func linkTargets100To200(text string) string {
	for _, re := range linkTargetREs {
		text = re.ReplaceAllStringFunc(text, func(m string) string {
			sub := re.FindStringSubmatch(m)
			return sub[1] + registerPath200(sub[2])
		})
	}
	return text
}

// document100To200 is one document in the 2.0.0 form: the frontmatter's node
// declarations — the analyses it hosts among them — every register entry and
// the analysis each claim's argument moves to, the Tier-2 marker under its new
// word, the analyses' markers beside their claims', the links to the renamed
// register, and the authored work ids under their three-letter prefix. The
// entry point's stamp is the save's, and is left as the conversion found it.
func document100To200(text string, sp splits, p string) []byte {
	lines := strings.Split(text, "\n")
	lines = frontmatter100To200(lines, sp.hosted(p))
	lines = entries100To200(lines, sp.registered(p))
	joined := format100.Tier2Marker100RE.ReplaceAllString(strings.Join(lines, "\n"), "<!--${1}claim-rigor${2}")
	joined = linkTargets100To200(joined)
	joined = format100.WorkTokenRE.ReplaceAllStringFunc(joined, func(id string) string {
		return "wrk-" + strings.TrimPrefix(id, "work-")
	})
	return []byte(sp.placeMarkers(joined, p))
}

// frontmatter100To200 renames a declared experiment's or analysis's pair list,
// gives every experiment the local value 1.0.0 had no field for, moves an
// analysis node's declaration to the 2.0.0 section and id key, and declares
// each analysis the document hosts beside the analyses it already declares.
func frontmatter100To200(lines []string, analyses []split) []string {
	start, end := frontmatterBounds(lines)
	if start < 0 {
		return lines
	}
	out := append([]string{}, lines[:start+1]...)
	section := ""
	nodeList := -1 // where the analysis-nodes entries begin, -1 where none stands
	for i := start + 1; i < end; {
		trimmed := strings.TrimSpace(lines[i])
		if key, isKey := topKey(lines[i]); isKey {
			_, value, _ := strings.Cut(lines[i], ":")
			section = ""
			if strings.TrimSpace(value) == "" {
				section = key
			}
			nodeList = -1
			switch {
			case key == "support-nodes" || key == format200.AnalysisNodesKey:
				// The analysis declarations: an entry this conversion writes
				// joins the list the KB already has, so the key opens one list
				// and not a second. A value written on the key line — the empty
				// flow list — gives way to those entries.
				renamed := leadingSpace(lines[i]) + format200.AnalysisNodesKey + ":"
				switch {
				case len(analyses) > 0:
					out = append(out, renamed)
					nodeList = len(out)
				case key == format200.AnalysisNodesKey:
					out = append(out, lines[i])
				default:
					out = append(out, renamed+value)
				}
			default:
				out = append(out, lines[i])
			}
			i++
			continue
		}
		if (section == "experiment-nodes" || section == "support-nodes") && strings.HasPrefix(trimmed, "- ") {
			j := nodeBlockEnd(lines, i, end)
			out = append(out, convertNode(lines[i:j], section)...)
			i = j
			if nodeList >= 0 {
				nodeList = len(out)
			}
			continue
		}
		out = append(out, lines[i])
		i++
		if nodeList >= 0 {
			nodeList = len(out)
		}
	}
	if len(analyses) > 0 {
		block := analysisNodesBlock(analyses)
		if nodeList < 0 {
			nodeList = len(out)
			block = append([]string{format200.AnalysisNodesKey + ":"}, block...)
		}
		out = slices.Insert(out, nodeList, block...)
	}
	return append(out, lines[end:]...)
}

// nodeBlockEnd is one past a declared node's block: its members and pair
// lists, deeper than the bullet that opens it.
func nodeBlockEnd(lines []string, start, end int) int {
	indent := len(leadingSpace(lines[start]))
	j := start + 1
	for j < end {
		if strings.TrimSpace(lines[j]) == "" {
			j++
			continue
		}
		if _, isKey := topKey(lines[j]); isKey || len(leadingSpace(lines[j])) <= indent {
			break
		}
		j++
	}
	return j
}

// convertNode renames one declared node's pair list and its 2.0.0 identity,
// and, for a node carrying no rigor of its own, inserts its local value after
// the status line that opens it.
func convertNode(block []string, section string) []string {
	out := make([]string, 0, len(block)+1)
	rigor := slices.ContainsFunc(block, func(line string) bool { return strings.HasPrefix(strings.TrimSpace(line), "rigor:") })
	for _, line := range block {
		if m := format100.AnalysisIDRE.FindStringSubmatch(line); m != nil {
			line = m[1] + "- ana-id: ana-" + strings.TrimPrefix(m[2], "sup-")
		}
		if m := format100.NodeKeyRE.FindStringSubmatch(line); m != nil {
			key := m[2]
			if section == "experiment-nodes" && key == "strengthens" || section == "support-nodes" && key == "supports" {
				line = m[1] + "lifts:" + m[3]
			}
		}
		out = append(out, line)
		if !rigor && strings.HasPrefix(strings.TrimSpace(line), "status:") {
			out = append(out, leadingSpace(line)+"rigor: 1.0")
			rigor = true
		}
	}
	if !rigor {
		for i, line := range out {
			if strings.HasPrefix(strings.TrimSpace(line), "- exp-id:") {
				out = slices.Insert(out, i+1, leadingSpace(line)+"rigor: 1.0")
				break
			}
		}
	}
	return out
}

// argument is the material of a claim's entry that argues for the claim: the
// analysis takes it, and the claim keeps none of it.
type argument struct {
	rigor    string   // the entry's own "- rigor:" line, "" where it carries none
	premises []string // its premise bullets in the 2.0.0 form, and their continuations
	lines    []string // its rationale field line and its continuations
}

// entry is the analysis's register entry: the claim's heading and marker, the
// claim's rigor as the analysis's own, the argument's premises and its
// rationale. An analysis whose argument states no premise carries no premise
// list, which the walk reads as the analysis's own method.
func (a argument) entry(heading, id string) []string {
	out := []string{heading, "<!-- id: " + id + " -->", "", "### Rigor"}
	if a.rigor != "" {
		out = append(out, a.rigor)
	}
	if len(a.premises) > 0 {
		out = append(out, "- "+format200.ListRestsOn+":")
		out = append(out, a.premises...)
	}
	return append(out, a.lines...)
}

// entries100To200 rewrites every register entry's headings and field lines by
// its kind, its premise bullets to the 2.0.0 shape, and each claim's argument
// into the analysis beside it — the claim keeping its statement, its rigor, its
// derived line and its rework items and nothing else. A line before the first
// marker, and a key of another kind, stand as they are.
//
// plans is the analyses whose claim's entry stands in this register, by claim
// id: a second entry for one id is read by nobody and is left as it stands.
func entries100To200(lines []string, plans map[string]split) []string {
	var out []string
	kind, list, heading := "", "", ""
	analysis := ""
	splitDone := map[string]bool{}
	var arg argument
	flush := func() {
		if analysis == "" {
			return
		}
		if n := len(out); n > 0 && strings.TrimSpace(out[n-1]) != "" {
			out = append(out, "")
		}
		out = append(out, arg.entry(heading, analysis)...)
		out = append(out, "")
		analysis, arg = "", argument{}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			heading = line
		case trimmed == "---":
			flush()
			kind, list = "", ""
		}
		switch {
		case format100.EntryMarkerRE.MatchString(trimmed):
			id := format100.EntryMarkerRE.FindStringSubmatch(trimmed)[1]
			if strings.HasPrefix(id, "sup-") {
				line = strings.Replace(line, id, "ana-"+strings.TrimPrefix(id, "sup-"), 1)
			}
			kind, _, _ = strings.Cut(id, "-")
			list = ""
			if s, ok := plans[id]; ok && heading != "" && !splitDone[id] {
				splitDone[id] = true
				analysis, arg = s.analysis, argument{}
			}
		default:
			if kind != "" {
				switch trimmed {
				case "### Quality":
					line = "### Rigor"
				case "### Solidity":
					line = "### Strength"
				}
			}
			if m := format100.FieldLineRE.FindStringSubmatch(line); m != nil {
				key := m[2]
				if renamed, ok := renamedFields[kind][key]; ok {
					line = m[1] + "- " + renamed + ":" + m[3]
					key = renamed
				}
				switch key {
				case "rests-on":
					list = "premises"
				case "references", "demoted", "strengthen-by", "rationale", "rigor", "strength", "lifts", "refutes":
					list = "other"
				}
				if analysis != "" && kind == "clm" {
					switch key {
					case "rigor":
						arg.rigor = line
					case "rests-on":
						continue
					case "rationale":
						arg.lines = append(arg.lines, line)
						list = "rationale"
						continue
					}
				}
				break
			}
			if kind != "" && list == "premises" && isSubBullet(line) {
				if analysis != "" {
					arg.premises = append(arg.premises, premiseLines100To200(line)...)
					continue
				}
				out = append(out, premiseLines100To200(line)...)
				continue
			}
			if analysis != "" && (list == "premises" || list == "rationale") &&
				strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
				if list == "premises" {
					arg.premises = append(arg.premises, line)
				} else {
					arg.lines = append(arg.lines, line)
				}
				continue
			}
		}
		out = append(out, line)
	}
	flush()
	return out
}

// premiseLines100To200 is one premise bullet in the 2.0.0 form: the 1.0.0
// citation and context as the 2.0.0 reader takes them, and the bearing the old
// form authored as a gate.
func premiseLines100To200(line string) []string {
	return premise100To200(citation100To200(axiomContext100To200(line)))
}

func isSubBullet(line string) bool {
	return len(line) > 0 && leadingSpace(line) != line && strings.HasPrefix(strings.TrimLeft(line, " \t"), "- ")
}

// axiomContext100To200 moves a framework bullet's context out of the trailing
// parenthetical the 1.0.0 reader took it from and into the bracket the 2.0.0
// form writes it in.
func axiomContext100To200(line string) string {
	content := strings.TrimRight(line, " \t")
	if !format100.CitationRE.MatchString(content) && !format100.AxiomTokenRE.MatchString(content) {
		return line
	}
	m := format100.TrailingParenRE.FindStringSubmatch(content)
	if m == nil || annotationTokenRE.MatchString(m[0]) {
		return line
	}
	return strings.TrimRight(content[:len(content)-len(m[0])], " \t") + " [" + strings.TrimSpace(m[1]) + "]"
}

// citation100To200 rewrites a legacy invariant citation to the axiom token
// the 2.0.0 register reads.
func citation100To200(line string) string {
	return format100.CitationRE.ReplaceAllString(line, "Axiom $1")
}

// premise100To200 is one premise bullet in the 2.0.0 form: one bullet per
// scoring target, each carrying its bearing; a claim bullet keeps the
// recorded source strength where the bullet recorded it, and the others
// record a pending one.
func premise100To200(line string) []string {
	indent := leadingSpace(line)
	body := strings.TrimLeft(line, " \t")
	content := strings.TrimSpace(strings.TrimPrefix(body, "- "))
	if strings.HasPrefix(content, "*(") {
		return []string{line}
	}
	head := premiseHead(content)
	targets := scoringTargets(head)
	if len(targets) < 2 {
		return []string{indent + "- " + simplePremise(content, targets)}
	}
	recorded := format100.SolidityAnnRE.FindStringSubmatch(content)
	bearing := format100.ApplicabilityAnnRE.FindStringSubmatch(content)
	context := format100.ContextRE.FindString(content)
	title := bulletTitle(content, targets)
	var out []string
	for i, t := range targets {
		var b strings.Builder
		b.WriteString(indent + "- " + t.text)
		if title != "" {
			b.WriteString(" — " + title)
		}
		if !t.work && !t.axiom {
			b.WriteString(" " + strengthAnnotation(i, targets, recorded))
		}
		b.WriteString(" " + bearingAnnotation(bearing))
		if context != "" {
			b.WriteString(" " + context)
		}
		out = append(out, b.String())
	}
	return out
}

// simplePremise rewrites one bullet's scoring annotations in place and gives
// a bullet naming a scoring target the bearing 1.0.0 left unrecorded. A bullet
// already carrying a bearing is a 2.0.0 bullet and is left as it stands.
func simplePremise(content string, targets []scoringToken) string {
	out := format100.ApplicabilityAnnRE.ReplaceAllString(content, "(bearing $1)")
	out = format100.SolidityAnnRE.ReplaceAllString(out, "(strength $1)")
	if len(targets) == 0 || format200.BearingAnnRE.MatchString(out) {
		return out
	}
	return insertBearing(out)
}

// insertBearing puts "(bearing 1.0)" after the strength annotation, else
// before the trailing context, else at the end of the line.
func insertBearing(s string) string {
	if m := format200.StrengthAnnRE.FindStringIndex(s); m != nil {
		return s[:m[1]] + " (bearing 1.0)" + s[m[1]:]
	}
	if m := format100.ContextRE.FindStringIndex(s); m != nil {
		return strings.TrimRight(s[:m[0]], " \t") + " (bearing 1.0) " + s[m[0]:]
	}
	return s + " (bearing 1.0)"
}

// strengthAnnotation is a split bullet's recorded source strength: the token
// the annotation was read from keeps its value, every other claim bullet
// records a pending one.
func strengthAnnotation(i int, targets []scoringToken, recorded []string) string {
	first := 0
	for first < len(targets) && (targets[first].work || targets[first].axiom) {
		first++
	}
	if i == first && recorded != nil {
		return "(strength " + strings.TrimSpace(recorded[1]) + ")"
	}
	return "(strength " + format100.PendingLiteral + ")"
}

// bearingAnnotation is a split bullet's bearing: the applicability the 1.0.0
// bullet recorded, on whichever target kind recorded it, or 1.0 where it
// recorded none — the full contribution the old form authored silently.
func bearingAnnotation(applicability []string) string {
	if applicability != nil {
		return "(bearing " + strings.TrimSpace(applicability[1]) + ")"
	}
	return "(bearing 1.0)"
}

// scoringTargets is every claim, work and axiom token of a bullet head, in the
// order they appear, each once.
func scoringTargets(head string) []scoringToken {
	var out []scoringToken
	for _, m := range format100.ClaimTokenRE.FindAllStringIndex(head, -1) {
		out = append(out, scoringToken{at: m[0], end: m[1], text: head[m[0]:m[1]]})
	}
	for _, m := range format100.WorkTokenRE.FindAllStringIndex(head, -1) {
		out = append(out, scoringToken{at: m[0], end: m[1], text: head[m[0]:m[1]], work: true})
	}
	for _, re := range []*regexp.Regexp{format100.CitationRE, format100.AxiomTokenRE} {
		for _, m := range re.FindAllStringIndex(head, -1) {
			out = append(out, scoringToken{at: m[0], end: m[1], text: head[m[0]:m[1]], axiom: true})
		}
	}
	slices.SortStableFunc(out, func(a, b scoringToken) int { return a.at - b.at })
	var unique []scoringToken
	for _, t := range out {
		if len(unique) == 0 || unique[len(unique)-1].text != t.text {
			unique = append(unique, t)
		}
	}
	return unique
}

// scoringToken is one claim, work or axiom token's span in a bullet head.
type scoringToken struct {
	at, end int
	text    string
	work    bool
	axiom   bool
}

// premiseHead is a premise bullet's head as the 1.0.0 reader cut it: the text
// before the first title separator or annotation.
func premiseHead(s string) string {
	cut := len(s)
	for _, sep := range []string{" — ", " ("} {
		if i := strings.Index(s, sep); i >= 0 && i < cut {
			cut = i
		}
	}
	return s[:cut]
}

// bulletTitle is the title a split bullet carries: the text after the last
// scoring target, less the separators that joined it and the annotations that
// follow it.
func bulletTitle(content string, targets []scoringToken) string {
	if len(targets) == 0 {
		return ""
	}
	tail := content[targets[len(targets)-1].end:]
	for _, sep := range []string{" (", " ["} {
		if i := strings.Index(tail, sep); i >= 0 {
			tail = tail[:i]
		}
	}
	return connectorless(tail)
}

// connectorless is a title with the separators joining it to its neighbours
// removed: the em dash, a comma, and the conjunction a hand-authored bullet
// may spell between two targets.
func connectorless(s string) string {
	for {
		before := s
		s = strings.TrimSpace(s)
		for _, sep := range []string{"—", ",", "&"} {
			s = strings.TrimSpace(strings.TrimPrefix(s, sep))
			s = strings.TrimSpace(strings.TrimSuffix(s, sep))
		}
		s = strings.TrimSpace(strings.TrimPrefix(s, "and "))
		s = strings.TrimSpace(strings.TrimSuffix(s, " and"))
		if s == "and" {
			s = ""
		}
		if s == before {
			return s
		}
	}
}

// frontmatterBounds is the lines between the document's opening "---" and
// its closing "---"; both -1 where the document opens otherwise.
func frontmatterBounds(lines []string) (start, end int) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return -1, -1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return 0, i
		}
	}
	return -1, -1
}

// topKey is a top-level frontmatter key's name, on an unindented line that
// holds one.
func topKey(line string) (string, bool) {
	if leadingSpace(line) != "" || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "#") {
		return "", false
	}
	key, _, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok || key == "" || strings.ContainsAny(key, " \t") {
		return "", false
	}
	return key, true
}

func leadingSpace(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// ConversionError is a legacy document's content the conversion cannot carry:
// the document's repository-relative path, the 1-based line, and the line as
// written.
type ConversionError struct {
	Path string
	Line int
	Text string
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("%s line %d: %q stands outside every axiom declaration, and axioms.md holds declarations only", e.Path, e.Line, e.Text)
}

// axioms100To200 is the axioms.md a 1.0.0 KB's declarations become, and the
// legacy declaration document that made way for it. axioms.md is declared in
// the KB and wins; else invariants.md, which the converter removes, else
// AGENTS.md, which stays for the instructions beside its declarations and so
// is not obsolete. invariants.md is a declaration document and nothing else:
// content beside its declarations is a ConversionError, where AGENTS.md's
// instructions are simply not converted.
func axioms100To200(in common.Files) ([]byte, []string, error) {
	if _, ok := in[format200.AxiomsPath]; ok {
		return nil, nil, nil
	}
	if b, ok := in[format100.InvariantsPath]; ok {
		doc, found, surplus := axiomsDocument(string(b), format100.InvariantsPath)
		if !found {
			return nil, nil, nil
		}
		if surplus != nil {
			return nil, nil, surplus
		}
		return doc, []string{format100.InvariantsPath}, nil
	}
	if b, ok := in[format100.AgentsPath]; ok {
		if doc, found, _ := axiomsDocument(string(b), format100.AgentsPath); found {
			return doc, nil, nil
		}
	}
	return nil, nil, nil
}

// axiomDeclaration is one legacy axiom as axioms.md states it: a heading and
// the body beneath it.
type axiomDeclaration struct {
	heading string
	body    []string
}

// axiomsDocument is a legacy document's declarations as an axioms.md: one
// "### Axiom <label>: Title" heading per INVARIANT section and per axiom
// bullet, in the order the document states them, with the declaration's own
// text beneath it. The label is kept as written; nothing is renumbered. A
// section heading only groups declarations and stands; any other line outside
// a declaration is returned as the surplus, the first such line only.
func axiomsDocument(text, rel string) ([]byte, bool, *ConversionError) {
	lines := strings.Split(text, "\n")
	var decls []axiomDeclaration
	var surplus *ConversionError
	inDeclaration := false
	for i, line := range lines {
		switch {
		case format100.LegacyInvariantHeadingRE.MatchString(line):
			m := format100.LegacyInvariantHeadingRE.FindStringSubmatch(line)
			decls = append(decls, axiomDeclaration{heading: "### Axiom " + m[1] + ": " + strip(m[2])})
			inDeclaration = true
		case format100.LegacyAxiomBulletRE.MatchString(line):
			m := format100.LegacyAxiomBulletRE.FindStringSubmatch(line)
			decls = append(decls, axiomDeclaration{heading: "### Axiom " + m[1] + ": " + strip(m[2]), body: []string{bulletBody(m[3])}})
			inDeclaration = false
		case format100.SectionEndRE.MatchString(line):
			inDeclaration = false
		case inDeclaration:
			decls[len(decls)-1].body = append(decls[len(decls)-1].body, line)
		case strip(line) == "":
		default:
			if surplus == nil {
				surplus = &ConversionError{Path: rel, Line: i + 1, Text: line}
			}
		}
	}
	if len(decls) == 0 {
		return nil, false, surplus
	}
	var b strings.Builder
	for i, d := range decls {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(d.heading + "\n")
		body := trimBlankLines(d.body)
		if len(body) > 0 {
			b.WriteString("\n")
			for _, line := range body {
				b.WriteString(line + "\n")
			}
		}
	}
	return []byte(b.String()), true, surplus
}

// bulletBody is an axiom bullet's text after its bold title, less the
// separator that joined the two.
func bulletBody(rest string) string {
	s := strip(rest)
	for _, sep := range []string{"—", "–", "-"} {
		if strings.HasPrefix(s, sep) {
			return strip(strings.TrimPrefix(s, sep))
		}
	}
	return s
}

// trimBlankLines is lines without the empty ones at either end.
func trimBlankLines(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strip(lines[start]) == "" {
		start++
	}
	for end > start && strip(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

// isSpace is Python's str.isspace() for one character.
func isSpace(r rune) bool {
	switch {
	case r >= '\t' && r <= '\r', r >= 0x1c && r <= ' ', r == 0x85, r == 0xa0, r == 0x1680,
		r >= 0x2000 && r <= 0x200a, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

func strip(s string) string { return strings.TrimFunc(s, isSpace) }
