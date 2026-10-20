// Package write is the KB's metadata write API: values in, canonical bytes
// out, each write proven by reading it back before it replaces a file.
//
// render composes every metadata byte; values checks a values document
// against an op's closed key vocabulary; store reads, splices, proves and
// replaces; ops gives each op its semantics.
package write

import (
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
)

// The renderers below are values in, text out: they read no file, refuse
// nothing and return no trailing newline. Derived fields are written at the
// placeholder refresh recognizes and replaces, never at a value.

const (
	// emDashSeparator cuts a rests-on bullet's target head from its title;
	// a hyphen does not cut, and the head would run on over the title.
	emDashSeparator = " — "
	bulletIndent    = "  "
)

var (
	claimIDFullRE = regexp.MustCompile(`^` + kb.IDBody("clm") + `$`)
	workIDFullRE  = regexp.MustCompile(`^` + kb.WorkIDPattern + `$`)
)

// collapse is the reader's whitespace normalization applied at write time,
// so what is stored is what the reader returns.
func collapse(text string) string { return kb.NormalizeSpace(text) }

func isClaimID(token string) bool { return claimIDFullRE.MatchString(token) }

func isWorkID(token string) bool { return workIDFullRE.MatchString(token) }

// formatScore renders an authored value: nil is the pending literal, a number
// its shortest round-tripping form, so the stored text reads back as the same
// number.
func formatScore(v *float64) string {
	if v == nil {
		return kb.PendingLiteral
	}
	return kb.PyFloatRepr(*v)
}

// bearingOf is a values document's optional bearing as the edge's: an absent
// value is the pending judgement.
func bearingOf(v *float64) kb.Bearing {
	if v == nil {
		return kb.Bearing{Pending: true}
	}
	return kb.Bearing{Value: *v}
}

// premiseTarget is one rests-on bullet's values: a clm- id, an "Axiom …"
// token or a work- id; the referent's title; the author's context; the bearing
// the bullet carries; and, for a demoted target alone, its origin.
type premiseTarget struct {
	Target, Title, Context string
	Bearing                *float64
	Origin                 string
}

// pair is a claim id and a bearing: one entry of an analysis's or experiment's
// lift or refutation list, of an experiment's support list, or of a claim's
// refutation list. A nil score is the pending judgement.
type pair struct {
	ID    string
	Score *float64
}

type experimentDecl struct {
	ExpID, Status string
	Rigor         *float64
	Supports      []pair
	Lifts         []pair
	Refutes       []pair
}

type analysisDecl struct {
	AnaID   string
	Lifts   []pair
	Refutes []pair
}

func renderIDMarker(nodeID string) string { return "<!-- id: " + nodeID + " -->" }

func renderTier2Marker(nodeID string) string { return "<!-- claim-rigor: " + nodeID + " -->" }

// Tier2Marker is the in-body marker mark-claim-in-leaf appends for nodeID.
func Tier2Marker(nodeID string) string { return renderTier2Marker(nodeID) }

// markerProbe is an id the marker renderers accept, used only to read back
// the opening token each writes in front of it.
const markerProbe = "clm-aaaaaa"

func markerOpener(marker string) string {
	return strings.TrimRight(marker[:strings.Index(marker, markerProbe)], " ")
}

// MarkerOpeners are what each marker the write API appends to an authored
// line opens with: the id marker and the Tier-2 marker.
func MarkerOpeners() []string {
	return []string{markerOpener(renderIDMarker(markerProbe)), markerOpener(renderTier2Marker(markerProbe))}
}

// renderRestsOnBullet is one rests-on sub-bullet: its target, the referent's
// title after an em dash, the claim target's strength placeholder, the bearing
// every target carries, and the author's context last.
func renderRestsOnBullet(t premiseTarget) string {
	var b strings.Builder
	b.WriteString(bulletIndent + "- " + t.Target)
	if t.Title != "" {
		b.WriteString(emDashSeparator + collapse(t.Title))
	}
	if isClaimID(t.Target) {
		b.WriteString(" " + kb.RenderStrengthAnnotation(kb.PendingLiteral))
	}
	b.WriteString(" " + kb.RenderBearingAnnotation(bearingOf(t.Bearing)))
	if t.Context != "" {
		b.WriteString(" [" + collapse(t.Context) + "]")
	}
	return b.String()
}

// renderReferencesBullet is one references sub-bullet: a claim bullet with
// no parenthetical, since nothing is derived for the edge.
func renderReferencesBullet(t premiseTarget) string { return claimBullet(t, "") }

// renderDemotedBullet is one demoted sub-bullet: a references bullet with
// its origin annotated before any context. It carries no bearing.
func renderDemotedBullet(t premiseTarget) string {
	return claimBullet(t, kb.RenderOriginAnnotation(t.Origin))
}

// claimBullet is a claim target's bullet: its title, an annotation where one
// is given, its context.
func claimBullet(t premiseTarget, annotation string) string {
	out := bulletIndent + "- " + t.Target
	if t.Title != "" {
		out += emDashSeparator + collapse(t.Title)
	}
	if annotation != "" {
		out += " " + annotation
	}
	if t.Context != "" {
		out += " [" + collapse(t.Context) + "]"
	}
	return out
}

func renderStrengthenByBullet(text string) string { return bulletIndent + "- " + collapse(text) }

// renderNoEdgeLine is the entry-level foreign-domain exemption. It is in no
// fold's key list, so it must sit above every folding field.
func renderNoEdgeLine(reason string) string { return "- no-edge: " + collapse(reason) }

// renderPairLine is one pair of an analysis's, a claim's or an experiment's
// register-staged list: the claim and the bearing the edge is judged with.
func renderPairLine(claimID string, bearing *float64) string {
	return bulletIndent + "- " + claimID + ": " + formatScore(bearing)
}

// renderRefutesList is an entry's refutation list: one pair line per edge, the
// claim the entry refutes and the bearing it caps it with.
func renderRefutesList(refutes []pair) []string {
	lines := []string{"- " + kb.RefutesKey + ":"}
	for _, p := range refutes {
		lines = append(lines, renderPairLine(p.ID, p.Score))
	}
	return lines
}

// renderCitation is the sanctioned authority-citation form.
func renderCitation(excerpt, kbPath, anchor string) string {
	return `["` + collapse(excerpt) + `"](` + kbPath + "#" + anchor + `)`
}

// registerEntry is a claim or analysis entry's values.
type registerEntry struct {
	NodeID, Title  string
	Rigor          *float64
	Rationale      string
	Premises       []premiseTarget
	StrengthenBy   []string
	Lifts, Refutes []pair
	NoEdge         string
}

// renderEntry composes a claim or analysis entry, its two headings above their
// markers' section. Field order is the reader's: each folding field runs until
// the next key, so the no-edge line sits above every folding field and the
// premise lists follow the rationale; the derived line sits alone under its own
// heading, after every authored field.
func renderEntry(e registerEntry) string {
	lines := []string{
		"## " + collapse(e.Title),
		renderIDMarker(e.NodeID),
		"",
		kb.LeafReferencesPendingFooter,
		"",
		"### Rigor",
		"- rigor: " + formatScore(e.Rigor),
	}
	if e.NoEdge != "" {
		lines = append(lines, renderNoEdgeLine(e.NoEdge))
	}
	lines = append(lines, "- rationale: "+collapse(e.Rationale))
	if len(e.Premises) > 0 {
		lines = append(lines, "- "+kb.ListRestsOn+":")
		for _, t := range e.Premises {
			lines = append(lines, renderRestsOnBullet(t))
		}
	}
	if len(e.Lifts) > 0 {
		lines = append(lines, "- "+kb.LiftsKey+":")
		for _, p := range e.Lifts {
			lines = append(lines, renderPairLine(p.ID, p.Score))
		}
	}
	if len(e.Refutes) > 0 {
		lines = append(lines, renderRefutesList(e.Refutes)...)
	}
	if len(e.StrengthenBy) > 0 {
		lines = append(lines, "- strengthen-by:")
		for _, item := range e.StrengthenBy {
			lines = append(lines, renderStrengthenByBullet(item))
		}
	}
	lines = append(lines, "", "### Strength", kb.StrengthPendingLine)
	return strings.Join(lines, "\n")
}

// renderWorkEntry composes an external work's entry: one heading and no
// strength line, no rests-on and no leaf-references footer, since a work has no
// derived value and the node is terminal.
func renderWorkEntry(nodeID, title string, rigor *float64, rationale string) string {
	return strings.Join([]string{
		"## " + collapse(title),
		renderIDMarker(nodeID),
		"",
		"### Rigor",
		"- rigor: " + formatScore(rigor),
		"- rationale: " + collapse(rationale),
	}, "\n")
}

// frontmatterValues is the authored half of a document's frontmatter block;
// refresh's roll-ups are absent by construction. A nil PathStable or NoClaim
// is an absent field.
type frontmatterValues struct {
	Kind            string
	PathStable      *string
	Claims          []string
	NoClaim         *string
	Experiments     []string
	ExperimentNodes []experimentDecl
	Analyses        []analysisDecl
}

// scoreNode is an authored value in frontmatter: the pending literal a
// string, a number its shortest round-tripping form.
func scoreNode(v *float64) *yaml.Node {
	if v == nil {
		return kb.FrontmatterString(kb.PendingLiteral)
	}
	return kb.FrontmatterNumber(kb.PyFloatRepr(*v))
}

// pairList is score pairs as a list of one-key mappings.
func pairList(pairs []pair) *yaml.Node {
	var items []*yaml.Node
	for _, p := range pairs {
		items = append(items, kb.FrontmatterMapping(kb.FrontmatterString(p.ID), scoreNode(p.Score)))
	}
	return kb.FrontmatterList(items)
}

// experimentNode is one experiment declaration as an experiment-nodes entry.
func experimentNode(d experimentDecl) *yaml.Node {
	n := kb.FrontmatterMapping(kb.FrontmatterString(kb.ExpIDKey), kb.FrontmatterString(d.ExpID),
		kb.FrontmatterString(kb.StatusKey), kb.FrontmatterString(d.Status))
	if d.Rigor != nil {
		n.Content = append(n.Content, kb.FrontmatterString(kb.RigorKey), scoreNode(d.Rigor))
	}
	for _, list := range []struct {
		key   string
		pairs []pair
	}{{kb.SupportsKey, d.Supports}, {kb.LiftsKey, d.Lifts}, {kb.RefutesKey, d.Refutes}} {
		if len(list.pairs) > 0 {
			n.Content = append(n.Content, kb.FrontmatterString(list.key), pairList(list.pairs))
		}
	}
	return n
}

// analysisNode is one analysis declaration as an analysis-nodes entry.
func analysisNode(d analysisDecl) *yaml.Node {
	n := kb.FrontmatterMapping(kb.FrontmatterString(kb.AnaIDKey), kb.FrontmatterString(d.AnaID))
	for _, list := range []struct {
		key   string
		pairs []pair
	}{{kb.LiftsKey, d.Lifts}, {kb.RefutesKey, d.Refutes}} {
		if len(list.pairs) > 0 {
			n.Content = append(n.Content, kb.FrontmatterString(list.key), pairList(list.pairs))
		}
	}
	return n
}

// renderExperimentNodes is the experiment-nodes key and its entries as
// frontmatter lines.
func renderExperimentNodes(decls []experimentDecl) ([]string, error) {
	var items []*yaml.Node
	for _, d := range decls {
		items = append(items, experimentNode(d))
	}
	return kb.RenderFrontmatterField(kb.ExperimentNodesKey, kb.FrontmatterList(items))
}

// renderAnalysisNodes is the analysis-nodes key and its entries as frontmatter
// lines.
func renderAnalysisNodes(decls []analysisDecl) ([]string, error) {
	var items []*yaml.Node
	for _, d := range decls {
		items = append(items, analysisNode(d))
	}
	return kb.RenderFrontmatterField(kb.AnalysisNodesKey, kb.FrontmatterList(items))
}

// renderFrontmatterBlock composes a document's frontmatter, its fences
// included: lists of ids inline, as refresh's field writer emits them; node
// declarations one mapping each under their kind's list.
func renderFrontmatterBlock(v frontmatterValues) (string, error) {
	type field struct {
		key   string
		value *yaml.Node
	}
	fields := []field{{"kind", kb.FrontmatterString(v.Kind)}}
	if v.PathStable != nil {
		fields = append(fields, field{"path-stable", kb.FrontmatterString(collapse(*v.PathStable))})
	}
	if len(v.Claims) > 0 {
		fields = append(fields, field{"claims", kb.FrontmatterIDList(v.Claims)})
	}
	if v.NoClaim != nil {
		fields = append(fields, field{"no-claim", kb.FrontmatterString(collapse(*v.NoClaim))})
	}
	if len(v.Experiments) > 0 {
		fields = append(fields, field{"experiments", kb.FrontmatterIDList(v.Experiments)})
	}
	var lines []string
	for _, f := range fields {
		rendered, err := kb.RenderFrontmatterField(f.key, f.value)
		if err != nil {
			return "", err
		}
		lines = append(lines, rendered...)
	}
	if len(v.ExperimentNodes) > 0 {
		rendered, err := renderExperimentNodes(v.ExperimentNodes)
		if err != nil {
			return "", err
		}
		lines = append(lines, rendered...)
	}
	if len(v.Analyses) > 0 {
		rendered, err := renderAnalysisNodes(v.Analyses)
		if err != nil {
			return "", err
		}
		lines = append(lines, rendered...)
	}
	return kb.WrapFrontmatter(lines), nil
}
