// Package from_1_0_0_to_2_0_0 is the 1.0.0 → 2.0.0 conversion step: a 1.0.0
// KB's files as bytes in, the 2.0.0 form's files as bytes out, plus the paths
// that form no longer has.
package from_1_0_0_to_2_0_0

import (
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/migrate/common"
	"kbase/internal/migrate/format_1_0_0"
	"kbase/internal/migrate/format_2_0_0"
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
// a bearing, the register file under the name the 2.0.0 form reads, the axiom
// declarations in the document the 2.0.0 form reads, and the derived index
// files the 2.0.0 form no longer has declared obsolete — the index is derived,
// and a refresh writes it.
func Convert100To200(in common.Files) (common.Files, []string, error) {
	out := make(common.Files, len(in))
	axioms, obsolete, err := axioms100To200(in)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range slices.Sorted(maps.Keys(in)) {
		if slices.Contains(obsolete, p) {
			continue
		}
		b := in[p]
		switch {
		case p == format_1_0_0.AgentsPath:
			out[p] = b
		case path.Ext(p) == ".md":
			renamed := registerPath200(p)
			out[renamed] = document100To200(string(b))
			if renamed != p {
				obsolete = append(obsolete, p)
			}
		case p == format_1_0_0.IndexDir+"/depends-on.yaml" || p == format_1_0_0.IndexDir+"/supported-by.yaml":
			obsolete = append(obsolete, p)
		default:
			out[p] = b
		}
	}
	if axioms != nil {
		out[format_2_0_0.AxiomsPath] = axioms
	}
	return out, obsolete, nil
}

// registerPath200 is p with a 1.0.0 register's basename renamed to the 2.0.0
// form; every other path stands. The match is the basename alone, so a
// document merely naming "claim-quality" is untouched.
func registerPath200(p string) string {
	if path.Base(p) != format_1_0_0.Register100Name {
		return p
	}
	return path.Join(path.Dir(p), format_2_0_0.Register200Name)
}

// document100To200 is one document in the 2.0.0 form: the frontmatter's node
// declarations, every register entry, the Tier-2 marker under its new word,
// and the authored work ids under their three-letter prefix. The entry point's
// stamp is the save's, and is left as the conversion found it.
func document100To200(text string) []byte {
	lines := strings.Split(text, "\n")
	lines = frontmatter100To200(lines)
	lines = entries100To200(lines)
	joined := format_1_0_0.Tier2Marker100RE.ReplaceAllString(strings.Join(lines, "\n"), "<!--${1}claim-rigor${2}")
	return []byte(format_1_0_0.WorkTokenRE.ReplaceAllStringFunc(joined, func(id string) string {
		return "wrk-" + strings.TrimPrefix(id, "work-")
	}))
}

// frontmatter100To200 renames a declared experiment's or analysis's pair list,
// gives every experiment the local value 1.0.0 had no field for, and moves an
// analysis node's declaration to the 2.0.0 section and id key.
func frontmatter100To200(lines []string) []string {
	start, end := frontmatterBounds(lines)
	if start < 0 {
		return lines
	}
	out := append([]string{}, lines[:start+1]...)
	section := ""
	for i := start + 1; i < end; {
		trimmed := strings.TrimSpace(lines[i])
		if key, isKey := topKey(lines[i]); isKey {
			section = key
			if key == "support-nodes" {
				out = append(out, leadingSpace(lines[i])+"analysis-nodes:")
			} else {
				out = append(out, lines[i])
			}
			i++
			continue
		}
		if (section == "experiment-nodes" || section == "support-nodes") && strings.HasPrefix(trimmed, "- ") {
			j := nodeBlockEnd(lines, i, end)
			out = append(out, convertNode(lines[i:j], section)...)
			i = j
			continue
		}
		out = append(out, lines[i])
		i++
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
		if m := format_1_0_0.AnalysisIDRE.FindStringSubmatch(line); m != nil {
			line = m[1] + "- ana-id: ana-" + strings.TrimPrefix(m[2], "sup-")
		}
		if m := format_1_0_0.NodeKeyRE.FindStringSubmatch(line); m != nil {
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

// entries100To200 rewrites every register entry's headings and field lines by
// its kind, and its premise bullets to the 2.0.0 shape. A line before the
// first marker, and a key of another kind, stand as they are.
func entries100To200(lines []string) []string {
	var out []string
	kind, list := "", ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case format_1_0_0.EntryMarkerRE.MatchString(trimmed):
			id := format_1_0_0.EntryMarkerRE.FindStringSubmatch(trimmed)[1]
			if strings.HasPrefix(id, "sup-") {
				line = strings.Replace(line, id, "ana-"+strings.TrimPrefix(id, "sup-"), 1)
			}
			kind, _, _ = strings.Cut(id, "-")
			list = ""
		default:
			if kind != "" {
				switch trimmed {
				case "### Quality":
					line = "### Rigor"
				case "### Solidity":
					line = "### Strength"
				}
			}
			if m := format_1_0_0.FieldLineRE.FindStringSubmatch(line); m != nil {
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
				break
			}
			if kind != "" && list == "premises" && isSubBullet(line) {
				out = append(out, premise100To200(citation100To200(axiomContext100To200(line)))...)
				continue
			}
		}
		out = append(out, line)
	}
	return out
}

func isSubBullet(line string) bool {
	return len(line) > 0 && leadingSpace(line) != line && strings.HasPrefix(strings.TrimLeft(line, " \t"), "- ")
}

// axiomContext100To200 moves a framework bullet's context out of the trailing
// parenthetical the 1.0.0 reader took it from and into the bracket the 2.0.0
// form writes it in.
func axiomContext100To200(line string) string {
	content := strings.TrimRight(line, " \t")
	if !format_1_0_0.CitationRE.MatchString(content) && !format_1_0_0.AxiomTokenRE.MatchString(content) {
		return line
	}
	m := format_1_0_0.TrailingParenRE.FindStringSubmatch(content)
	if m == nil || annotationTokenRE.MatchString(m[0]) {
		return line
	}
	return strings.TrimRight(content[:len(content)-len(m[0])], " \t") + " [" + strings.TrimSpace(m[1]) + "]"
}

// citation100To200 rewrites a legacy invariant citation to the axiom token
// the 2.0.0 register reads.
func citation100To200(line string) string {
	return format_1_0_0.CitationRE.ReplaceAllString(line, "Axiom $1")
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
	recorded := format_1_0_0.SolidityAnnRE.FindStringSubmatch(content)
	bearing := format_1_0_0.ApplicabilityAnnRE.FindStringSubmatch(content)
	context := format_1_0_0.ContextRE.FindString(content)
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
	out := format_1_0_0.ApplicabilityAnnRE.ReplaceAllString(content, "(bearing $1)")
	out = format_1_0_0.SolidityAnnRE.ReplaceAllString(out, "(strength $1)")
	if len(targets) == 0 || format_2_0_0.BearingAnnRE.MatchString(out) {
		return out
	}
	return insertBearing(out)
}

// insertBearing puts "(bearing 1.0)" after the strength annotation, else
// before the trailing context, else at the end of the line.
func insertBearing(s string) string {
	if m := format_2_0_0.StrengthAnnRE.FindStringIndex(s); m != nil {
		return s[:m[1]] + " (bearing 1.0)" + s[m[1]:]
	}
	if m := format_1_0_0.ContextRE.FindStringIndex(s); m != nil {
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
	return "(strength " + format_1_0_0.PendingLiteral + ")"
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
	for _, m := range format_1_0_0.ClaimTokenRE.FindAllStringIndex(head, -1) {
		out = append(out, scoringToken{at: m[0], end: m[1], text: head[m[0]:m[1]]})
	}
	for _, m := range format_1_0_0.WorkTokenRE.FindAllStringIndex(head, -1) {
		out = append(out, scoringToken{at: m[0], end: m[1], text: head[m[0]:m[1]], work: true})
	}
	for _, re := range []*regexp.Regexp{format_1_0_0.CitationRE, format_1_0_0.AxiomTokenRE} {
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
	if _, ok := in[format_2_0_0.AxiomsPath]; ok {
		return nil, nil, nil
	}
	if b, ok := in[format_1_0_0.InvariantsPath]; ok {
		doc, found, surplus := axiomsDocument(string(b), format_1_0_0.InvariantsPath)
		if !found {
			return nil, nil, nil
		}
		if surplus != nil {
			return nil, nil, surplus
		}
		return doc, []string{format_1_0_0.InvariantsPath}, nil
	}
	if b, ok := in[format_1_0_0.AgentsPath]; ok {
		if doc, found, _ := axiomsDocument(string(b), format_1_0_0.AgentsPath); found {
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
		case format_1_0_0.LegacyInvariantHeadingRE.MatchString(line):
			m := format_1_0_0.LegacyInvariantHeadingRE.FindStringSubmatch(line)
			decls = append(decls, axiomDeclaration{heading: "### Axiom " + m[1] + ": " + strip(m[2])})
			inDeclaration = true
		case format_1_0_0.LegacyAxiomBulletRE.MatchString(line):
			m := format_1_0_0.LegacyAxiomBulletRE.FindStringSubmatch(line)
			decls = append(decls, axiomDeclaration{heading: "### Axiom " + m[1] + ": " + strip(m[2]), body: []string{bulletBody(m[3])}})
			inDeclaration = false
		case format_1_0_0.SectionEndRE.MatchString(line):
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
