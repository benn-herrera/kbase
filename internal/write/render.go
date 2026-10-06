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
	// emDashSeparator cuts a depends-on bullet's target head from its title;
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

// formatScore renders an authored score: nil is the pending literal, a number
// its shortest round-tripping form, so the stored text reads back as the same
// number.
func formatScore(v *float64) string {
	if v == nil {
		return kb.PendingLiteral
	}
	return kb.PyFloatRepr(*v)
}

// dependsOnTarget is one depends-on bullet's values: a clm- id, a framework
// token or a work- id; the referent's title; the author's context; for a
// work target alone, the pairing's applicability (nil is pending); and for a
// demoted target alone, its origin.
type dependsOnTarget struct {
	Target, Title, Context string
	Applicability          *float64
	Origin                 string
}

// pair is a claim id and a score: a supports fraction (nil is pending) or a
// strengthens strength (never nil).
type pair struct {
	ID    string
	Score *float64
}

type experimentDecl struct {
	ExpID, Status string
	Strengthens   []pair
}

type supportDecl struct {
	SupID    string
	Supports []pair
}

func renderIDMarker(nodeID string) string { return "<!-- id: " + nodeID + " -->" }

func renderTier2Marker(nodeID string) string { return "<!-- claim-quality: " + nodeID + " -->" }

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

// renderDependsOnBullet is one depends-on sub-bullet. A claim target carries
// the solidity placeholder, a work target its applicability, and a framework
// target its context in parentheses, where the reader takes a framework
// edge's context from.
func renderDependsOnBullet(t dependsOnTarget) string {
	var b strings.Builder
	b.WriteString(bulletIndent + "- " + t.Target)
	switch {
	case isClaimID(t.Target) || isWorkID(t.Target):
		if t.Title != "" {
			b.WriteString(emDashSeparator + collapse(t.Title))
		}
		if isWorkID(t.Target) {
			b.WriteString(" " + renderApplicabilityAnnotation(formatScore(t.Applicability)))
		} else {
			b.WriteString(" " + kb.SolidityAnnotationPending)
		}
		if t.Context != "" {
			b.WriteString(" [" + collapse(t.Context) + "]")
		}
	case t.Context != "":
		b.WriteString(" (" + collapse(t.Context) + ")")
	}
	return b.String()
}

// renderReferencesBullet is one references sub-bullet: a claim bullet with
// no parenthetical, since nothing is derived for the edge.
func renderReferencesBullet(t dependsOnTarget) string { return claimBullet(t, "") }

// renderDemotedBullet is one demoted sub-bullet: a references bullet with
// its origin annotated before any context.
func renderDemotedBullet(t dependsOnTarget) string {
	return claimBullet(t, kb.RenderOriginAnnotation(t.Origin))
}

// claimBullet is a claim target's bullet: its title, an annotation where one
// is given, its context.
func claimBullet(t dependsOnTarget, annotation string) string {
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

func renderApplicabilityAnnotation(valueText string) string {
	return "(applicability " + valueText + ")"
}

func renderStrengthenByBullet(text string) string { return bulletIndent + "- " + collapse(text) }

// renderNoEdgeLine is the entry-level foreign-domain exemption. It is in no
// fold's key list, so it must sit above every folding field.
func renderNoEdgeLine(reason string) string { return "- no-edge: " + collapse(reason) }

// renderSupportsPairLine is a support's fan-out pair in its register entry's
// staging block.
func renderSupportsPairLine(claimID string, fraction *float64) string {
	return bulletIndent + "- " + claimID + ": " + formatScore(fraction)
}

// renderCitation is the sanctioned authority-citation form.
func renderCitation(excerpt, kbPath, anchor string) string {
	return `["` + collapse(excerpt) + `"](` + kbPath + "#" + anchor + ")"
}

// registerEntry is a claim or support entry's values. ScoreField is
// "confidence" or "quality".
type registerEntry struct {
	NodeID, Title, ScoreField string
	Score                     *float64
	Rationale                 string
	DependsOn                 []dependsOnTarget
	StrengthenBy              []string
	Supports                  []pair
	NoEdge                    string
}

// renderEntry composes a claim or support entry, heading above marker. Field
// order is the reader's: each folding field runs until the next key, so the
// staging block sits under the score where the unconditional solidity line
// closes it, and no-edge sits above every folding field.
func renderEntry(e registerEntry) string {
	lines := []string{
		"## " + collapse(e.Title),
		renderIDMarker(e.NodeID),
		"",
		kb.LeafReferencesPendingFooter,
		"",
		"### Quality",
		"- " + e.ScoreField + ": " + formatScore(e.Score),
	}
	if len(e.Supports) > 0 {
		lines = append(lines, "- supports:")
		for _, p := range e.Supports {
			lines = append(lines, renderSupportsPairLine(p.ID, p.Score))
		}
	}
	if e.NoEdge != "" {
		lines = append(lines, renderNoEdgeLine(e.NoEdge))
	}
	if len(e.DependsOn) > 0 {
		lines = append(lines, "- depends-on:")
		for _, t := range e.DependsOn {
			lines = append(lines, renderDependsOnBullet(t))
		}
	}
	lines = append(lines, kb.SolidityPendingLine, "- rationale: "+collapse(e.Rationale))
	if len(e.StrengthenBy) > 0 {
		lines = append(lines, "- strengthen-by:")
		for _, item := range e.StrengthenBy {
			lines = append(lines, renderStrengthenByBullet(item))
		}
	}
	return strings.Join(lines, "\n")
}

// renderWorkEntry composes an external work's entry: no solidity line, no
// depends-on and no leaf-references footer, since nothing derives one and the
// node is terminal.
func renderWorkEntry(nodeID, title string, strength *float64, rationale string) string {
	return strings.Join([]string{
		"## " + collapse(title),
		renderIDMarker(nodeID),
		"",
		"### Quality",
		"- strength: " + formatScore(strength),
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
	SupportNodes    []supportDecl
}

// scoreNode is an authored score in frontmatter: the pending literal a
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
	if len(d.Strengthens) > 0 {
		n.Content = append(n.Content, kb.FrontmatterString(kb.StrengthensKey), pairList(d.Strengthens))
	}
	return n
}

// supportNode is one support declaration as a support-nodes entry.
func supportNode(d supportDecl) *yaml.Node {
	n := kb.FrontmatterMapping(kb.FrontmatterString(kb.SupIDKey), kb.FrontmatterString(d.SupID))
	if len(d.Supports) > 0 {
		n.Content = append(n.Content, kb.FrontmatterString(kb.SupportsKey), pairList(d.Supports))
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

// renderSupportNodes is the support-nodes key and its entries as frontmatter
// lines.
func renderSupportNodes(decls []supportDecl) ([]string, error) {
	var items []*yaml.Node
	for _, d := range decls {
		items = append(items, supportNode(d))
	}
	return kb.RenderFrontmatterField(kb.SupportNodesKey, kb.FrontmatterList(items))
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
	if len(v.SupportNodes) > 0 {
		rendered, err := renderSupportNodes(v.SupportNodes)
		if err != nil {
			return "", err
		}
		lines = append(lines, rendered...)
	}
	return kb.WrapFrontmatter(lines), nil
}
