package sheet

import (
	"cmp"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"kbase/internal/kb"
)

const (
	frameColor = "#333333"
	ghostColor = "#c0392b"
	linkColor  = "#1f6fb2"
	ellipsis   = "…"
	// wordCut is what a demoted edge is called on a sheet: a depends edge
	// the build's cycle breaking cut.
	wordCut      = "cut"
	ghostMeaning = "an id no record carries"
	cutColor     = "#c0392b"
	cutGlyph     = "┈┈┈▷"
	cutLegend    = "cut from depends: part of a circle (cited / inferred)"
)

// nodeStyle is how one kind draws; a fill of "" is the node's band's.
type nodeStyle struct {
	kind, shape, style string
	penwidth           int
	fill, meaning      string
}

// nodeStyles is every kind but kindGhost, in legend order: a claim's three,
// then every other node kind of kb.NodeKinds.
var nodeStyles = []nodeStyle{
	{kindBlock, "box", "filled,rounded", 2, "", "labelled block — theorem, lemma, … (bold frame)"},
	{kindEquation, "box", "filled", 1, "", "equation"},
	{kindProse, "box", "filled,rounded", 1, "", "prose claim"},
	{kb.NodeKindSupport, "box", "filled,rounded,dashed", 1, "", "analytical support"},
	{kb.NodeKindExperiment, "component", "filled", 1, "#e8f4fd", "experiment"},
	{kb.NodeKindInvariant, "box", "filled", 2, "#d9d9d9", "structural invariant"},
	{kb.NodeKindAxiom, "box", "filled", 2, "#d9d9d9", "axiom"},
	{kb.NodeKindWork, "note", "filled", 1, "#f0e6ff", "external work"},
}

// styleOf is kind's style; a node type outside kb.NodeKinds draws as prose.
func styleOf(kind string) nodeStyle {
	of := func(k string) int { return slices.IndexFunc(nodeStyles, func(s nodeStyle) bool { return s.kind == k }) }
	i := of(kind)
	if i < 0 {
		i = of(kindProse)
	}
	return nodeStyles[i]
}

// plural is how the digest counts a kind.
func plural(kind string) string {
	if kind == kindProse {
		return kind
	}
	return kind + "s"
}

// ladderFills is the fill of each rung of kb.BuildBandLadder, in its order;
// pendingFill is kb.UnknownBandSlug's.
var (
	ladderFills = []string{"#b7e4c0", "#e3f2b5", "#ffe1a8", "#ffb8a1", "#e3a7c4"}
	pendingFill = "#eeeeee"
)

// band is one band as the legend lists it.
type band struct{ slug, label, fill string }

// bands is the ladder's bands and then the pending one, in legend order.
var bands = func() []band {
	var out []band
	for i, b := range kb.BuildBandLadder {
		out = append(out, band{b.Slug, b.Label, ladderFills[i]})
	}
	return append(out, band{kb.UnknownBandSlug, kb.PendingLiteral, pendingFill})
}()

// bandFills and bandLabels are bands by slug.
var bandFills, bandLabels = func() (map[string]string, map[string]string) {
	fills, labels := map[string]string{}, map[string]string{}
	for _, b := range bands {
		fills[b.slug], labels[b.slug] = b.fill, b.label
	}
	return fills, labels
}()

// stroke is how one relation, or one provenance of depends or demoted, draws.
// provenance is kb.OriginCited or kb.OriginInferred for those two relations
// and "" for the rest. word is what the digest's counts call it, tip what an
// edge's tooltip does, and legend its legend row's text, one row for strokes
// sharing it; a reverse stroke is emitted target → source with dir=back, so
// the premise sits at the head.
type stroke struct {
	relation, provenance string
	word, tip            string
	reverse, premise     bool
	color, extra, glyph  string
	legend               string
}

// strokes is every stroke, in legend and count order. kb.RelationReferences
// has none: a references edge is not drawn.
var strokes = []stroke{
	{kb.RelationDepends, kb.OriginCited, kb.OriginCited, "depends, " + kb.OriginCited, false, true, "#222222", "penwidth=1.2", "━━━▶",
		kb.OriginCited + " — depends, marked in the text; points to the premise"},
	{kb.RelationDepends, kb.OriginInferred, kb.OriginInferred, "depends, " + kb.OriginInferred, false, true, "#2471a3", "style=dashed penwidth=1.3", "╍╍╍▶",
		kb.OriginInferred + " — depends, found with no mark in the text; points to the premise"},
	{kb.RelationStrengthens, "", kb.RelationStrengthens, kb.RelationStrengthens, true, true, linkColor, "style=dashed arrowtail=empty", "╍╍╍▷",
		kb.RelationStrengthens + " — points to the claim lifted"},
	{kb.RelationSupports, "", kb.RelationSupports, kb.RelationSupports, true, true, "#2e7d32", "arrowtail=empty", "━━━▷",
		kb.RelationSupports + " — points to the claim lifted"},
	{kb.RelationRestsOn, "", kb.RelationRestsOn, kb.RelationRestsOn, false, true, "#7d3c98", "style=dashed arrowhead=diamond", "╍╍╍◆",
		kb.RelationRestsOn + " — points to the cited work"},
	{kb.RelationDemoted, kb.OriginCited, kb.OriginCited + " " + wordCut, wordCut + ", " + kb.OriginCited, false, false, cutColor,
		"style=dotted penwidth=1.4 arrowhead=open constraint=false", cutGlyph, cutLegend},
	{kb.RelationDemoted, kb.OriginInferred, kb.OriginInferred + " " + wordCut, wordCut + ", " + kb.OriginInferred, false, false, cutColor,
		"style=dotted penwidth=1.0 arrowhead=open constraint=false", cutGlyph, cutLegend},
}

// strokeFor is relation's stroke, a depends or demoted edge's by whether it
// is inferred; nil for a relation not drawn.
func strokeFor(relation string, inferred bool) *stroke {
	for i := range strokes {
		s := &strokes[i]
		if s.relation == relation && (s.provenance == "" || (s.provenance == kb.OriginInferred) == inferred) {
			return s
		}
	}
	return nil
}

var (
	linkRE       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	inlineMathRE = regexp.MustCompile("\\$`([^`]*)`\\$")
	markRE       = regexp.MustCompile("[*`]")
)

// dotSheet is one sheet's DOT text and its kb-root-relative path.
type dotSheet struct{ path, dot string }

// sheets is the DOT text of every sheet of g: the root's full sheet, then,
// where two or more volumes hold nodes, the volume digest and each volume's
// sheet in key order.
func (g *graph) sheets() []dotSheet {
	out := []dotSheet{{kb.ClaimGraphFile, g.full()}}
	if !g.multiVolume() {
		return out
	}
	out = append(out, dotSheet{kb.ClaimGraphDigestFile, g.digest()})
	for _, v := range g.volumes {
		if v.key != "" {
			out = append(out, dotSheet{v.key + "/" + kb.ClaimGraphFile, g.volumeSheet(v)})
		}
	}
	return out
}

// multiVolume is whether two or more volumes hold nodes; the root bucket is
// not a volume.
func (g *graph) multiVolume() bool {
	n := 0
	for _, v := range g.volumes {
		if v.key != "" {
			n++
		}
	}
	return n >= 2
}

// members is the nodes of the volume keyed key, by id.
func (g *graph) members(key string) []node {
	var out []node
	for _, n := range g.nodes {
		if n.kind != kindGhost && n.group == key {
			out = append(out, n)
		}
	}
	return out
}

// drawn is the nodes some edge touches, by id.
func (g *graph) drawn(connected map[string]bool) []node {
	var out []node
	for _, n := range g.nodes {
		if connected[n.id] {
			out = append(out, n)
		}
	}
	return out
}

// edgeEnds is every id some edge touches; a node outside it is unattached.
func edgeEnds(edges []edge) map[string]bool {
	out := map[string]bool{}
	for _, e := range edges {
		out[e.source], out[e.target] = true, true
	}
	return out
}

// emitted is e's tail and head as written into the DOT, the premise at the
// head.
func (e edge) emitted() (string, string) {
	if e.stroke.reverse {
		return e.target, e.source
	}
	return e.source, e.target
}

// full is the full sheet: one cluster per volume, every drawn edge, each
// cluster's unattached nodes as a table.
func (g *graph) full() string {
	connected := edgeEnds(g.edges)
	var clusters []string
	for k, v := range g.volumes {
		clusters = append(clusters, g.ownCluster(k, v, connected, "")...)
	}
	lines := g.claimsGraph(g.kbTitle, clusters, g.drawn(connected), reduce(g.edges))
	if g.multiVolume() {
		lines = append(lines, sheetLink("volume digest →", kb.ClaimGraphDigestFile, "min")...)
	}
	return strings.Join(append(lines, "}"), "\n") + "\n"
}

// volumeSheet is v's sheet: its nodes, the nodes one edge from them
// clustered by volume, and the edges with an end in v, every href relative
// to v's directory.
func (g *graph) volumeSheet(v volume) string {
	inside := map[string]bool{}
	for _, n := range g.members(v.key) {
		inside[n.id] = true
	}
	var edges []edge
	for _, e := range g.edges {
		if inside[e.source] || inside[e.target] {
			edges = append(edges, e)
		}
	}
	connected := edgeEnds(edges)
	var clusters []string
	for k, other := range g.volumes {
		if other.key == v.key {
			clusters = append(clusters, g.ownCluster(k, other, connected, v.key)...)
			continue
		}
		var body []string
		for _, n := range g.members(other.key) {
			if connected[n.id] {
				body = append(body, "    "+nodeStatement(n, v.key))
			}
		}
		if len(body) > 0 {
			clusters = append(clusters, cluster(k, other.title, count(len(body), "neighbour"), body)...)
		}
	}
	lines := g.claimsGraph(v.title, clusters, g.drawn(connected), reduce(edges))
	return strings.Join(append(lines, "}"), "\n") + "\n"
}

// ownCluster is v's cluster k: its attached nodes drawn, the rest listed.
func (g *graph) ownCluster(k int, v volume, connected map[string]bool, base string) []string {
	members := g.members(v.key)
	var body []string
	var unattached []node
	for _, n := range members {
		if connected[n.id] {
			body = append(body, "    "+nodeStatement(n, base))
		} else {
			unattached = append(unattached, n)
		}
	}
	if len(unattached) > 0 {
		body = append(body, fmt.Sprintf(`    "unattached_%d" [shape=plaintext fontsize=8 label=<%s>]`, k, unattachedTable(unattached, base)))
	}
	return cluster(k, v.title, fmt.Sprintf("%s · %d unattached", count(len(members), "node"), len(unattached)), body)
}

func cluster(k int, title, subtitle string, body []string) []string {
	return slices.Concat([]string{
		fmt.Sprintf("  subgraph cluster_%d {", k),
		fmt.Sprintf(`    graph [style="rounded" color="#888888" bgcolor="#fafafa" labeljust=l fontsize=12 label=<<b>%s</b><br/><font point-size="9">%s</font>>]`,
			escape(title), escape(subtitle)),
	}, body, []string{"  }"})
}

// claimsGraph is a sheet of claims but its links and closing brace: header,
// clusters, ghosts, strokes and legend.
func (g *graph) claimsGraph(title string, clusters []string, drawn []node, edges []edge) []string {
	lines := slices.Concat([]string{
		`digraph "claim-graph" {`,
		fmt.Sprintf(`  graph [rankdir=TB newrank=true nodesep=0.25 ranksep=0.5 pad=0.3 fontname="Helvetica" outputorder=edgesfirst labelloc=t label=<<b>%s</b>>]`, escape(title)),
		fmt.Sprintf(`  node [fontname="Helvetica" fontsize=9 margin="0.08,0.04" color="%s"]`, frameColor),
		"  edge [arrowsize=0.7]",
	}, clusters)
	for _, n := range drawn {
		if n.kind == kindGhost {
			lines = append(lines, fmt.Sprintf(`  %s [shape=box style="dashed" penwidth=1 color=%s label=%s tooltip=%s]`,
				quoted(n.id), quoted(ghostColor), quoted(n.id), quoted(n.id+" — no record carries this id")))
		}
	}
	emitted := slices.Clone(edges)
	slices.SortStableFunc(emitted, func(a, b edge) int {
		at, ah := a.emitted()
		bt, bh := b.emitted()
		return cmp.Or(strings.Compare(at, bt), strings.Compare(ah, bh), strings.Compare(a.stroke.relation, b.stroke.relation))
	})
	for _, e := range emitted {
		lines = append(lines, "  "+edgeStatement(e))
	}
	if l := legend(claimsLegendRows(drawn, edges)); l != "" {
		lines = append(lines, "  "+l)
	}
	return lines
}

func nodeStatement(n node, base string) string {
	s := styleOf(n.kind)
	fill := cmp.Or(s.fill, bandFills[n.band])
	return fmt.Sprintf(`%s [shape=%s style=%s penwidth=%d fillcolor=%s label=%s href=%s tooltip=%s]`,
		quoted(n.id), s.shape, quoted(s.style), s.penwidth, quoted(fill), quoted(labelLines(n)...), quoted(relative(n.href, base)), quoted(tooltip(n)))
}

func edgeStatement(e edge) string {
	s := e.stroke
	tail, head := e.emitted()
	direction := ""
	if s.reverse {
		direction = "dir=back "
	}
	return fmt.Sprintf(`%s -> %s [%scolor=%s %s tooltip=%s]`, quoted(tail), quoted(head), direction, quoted(s.color), s.extra,
		quoted(fmt.Sprintf("%s → %s (%s)", e.source, e.target, s.tip)))
}

func unattachedTable(nodes []node, base string) string {
	var rows strings.Builder
	for _, n := range nodes {
		fmt.Fprintf(&rows, `<tr><td align="left" href="%s" tooltip="%s">%s — %s</td></tr>`,
			escape(relative(n.href, base)), escape(tooltip(n)), escape(n.id), escape(cut(displayTitle(n.title), 48)))
	}
	return fmt.Sprintf(`<table border="0" cellborder="0" cellspacing="0" cellpadding="1"><tr><td align="left"><b>unattached (%d)</b></td></tr>%s</table>`,
		len(nodes), rows.String())
}

// tooltip is "<id> [<kind>, <band label>] <title>", the band omitted for a
// node with none.
func tooltip(n node) string {
	classes := n.kind
	if n.band != "" {
		classes += ", " + bandLabels[n.band]
	}
	return fmt.Sprintf("%s [%s] %s", n.id, classes, strings.Join(strings.Fields(n.title), " "))
}

// labelLines is a node's id over its title wrapped to at most three lines.
func labelLines(n node) []string {
	const width, most = 28, 3
	wrapped := wrap(displayTitle(n.title), width)
	if len(wrapped) > most {
		last := []rune(wrapped[most-1])
		wrapped = append(wrapped[:most-1], string(last[:min(len(last), width-1)])+ellipsis)
	}
	return append([]string{n.id}, wrapped...)
}

// displayTitle is a title as a box shows it: link text for a link, maths
// without its fence, no emphasis or code marks, whitespace collapsed.
func displayTitle(title string) string {
	text := inlineMathRE.ReplaceAllString(linkRE.ReplaceAllString(title, "$1"), "$1")
	return strings.Join(strings.Fields(markRE.ReplaceAllString(text, "")), " ")
}

// relative is href, relative to kb-root, made relative to the directory
// base; href itself where base is kb-root.
func relative(href, base string) string {
	if base == "" || href == "" {
		return href
	}
	p, anchor, marked := strings.Cut(href, "#")
	if rel, err := filepath.Rel(filepath.FromSlash(base), filepath.FromSlash(p)); err == nil {
		p = filepath.ToSlash(rel)
	}
	if marked {
		p += "#" + anchor
	}
	return p
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// cut is text cut to width characters, the last an ellipsis where cut.
func cut(text string, width int) string {
	rs := []rune(text)
	if len(rs) <= width {
		return text
	}
	return string(rs[:width-1]) + ellipsis
}

// legend is the legend node, "" where no row explains anything.
func legend(rows []string) string {
	if len(rows) == 0 {
		return ""
	}
	return `legend [shape=plaintext label=<<table border="1" cellborder="0" cellspacing="2" cellpadding="3" color="#999999">` +
		`<tr><td colspan="2" align="left"><b>legend</b></td></tr><tr>` + strings.Join(rows, "</tr><tr>") + "</tr></table>>]"
}

// claimsLegendRows is a row per kind drawn, per band drawn and per stroke
// drawn.
func claimsLegendRows(drawn []node, edges []edge) []string {
	kinds, held := map[string]bool{}, map[string]bool{}
	for _, n := range drawn {
		kinds[n.kind], held[n.band] = true, true
	}
	var rows []string
	for _, s := range nodeStyles {
		if kinds[s.kind] {
			dashed := ""
			if slices.Contains(strings.Split(s.style, ","), "dashed") {
				dashed = ` style="dashed"`
			}
			rows = append(rows, fmt.Sprintf(`<td bgcolor="%s" border="%d" color="%s"%s>%s</td><td align="left">%s</td>`,
				escape(cmp.Or(s.fill, "#ffffff")), s.penwidth, frameColor, dashed, escape(s.kind), escape(s.meaning)))
		}
	}
	if kinds[kindGhost] {
		rows = append(rows, fmt.Sprintf(`<td border="1" color="%s" style="dashed">%s</td><td align="left">%s</td>`, ghostColor, kindGhost, ghostMeaning))
	}
	for _, b := range bands {
		if held[b.slug] {
			rows = append(rows, fmt.Sprintf(`<td bgcolor="%s" border="1" color="%s"> </td><td align="left">%s</td>`, escape(b.fill), frameColor, escape(b.label)))
		}
	}
	return append(rows, strokeRows(edges)...)
}

// strokeRows is a legend row per legend text some edge's stroke carries, in
// stroke order, drawn as the first stroke carrying it.
func strokeRows(edges []edge) []string {
	drawn := map[string]bool{}
	for _, e := range edges {
		drawn[e.stroke.legend] = true
	}
	var rows []string
	for _, s := range strokes {
		if drawn[s.legend] {
			delete(drawn, s.legend)
			rows = append(rows, fmt.Sprintf(`<td><font color="%s"><b>%s</b></font></td><td align="left">%s</td>`,
				escape(s.color), escape(s.glyph), escape(s.legend)))
		}
	}
	return rows
}

// strokeCounts is edges counted by stroke, "3 cited · 1 rests-on", in
// legend order.
func strokeCounts(edges []edge) string {
	var parts []string
	for i := range strokes {
		s := &strokes[i]
		n := 0
		for _, e := range edges {
			if e.stroke == s {
				n++
			}
		}
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s.word))
		}
	}
	return strings.Join(parts, " · ")
}

// sheetLink is the plaintext node a root sheet links the other by, kept to
// the first or last rank.
func sheetLink(label, href, rank string) []string {
	return []string{
		fmt.Sprintf(`  "sheet-link" [shape=plaintext fontsize=10 fontcolor="%s" label=%s href=%s]`, linkColor, quoted(label), quoted(href)),
		fmt.Sprintf(`  { rank=%s; "sheet-link" }`, rank),
	}
}

// bundle is the edges crossing from one volume to another, premise strokes
// apart from the rest.
type bundle struct {
	tail, head int
	premise    bool
	edges      []edge
}

// bundles is every bundle, by tail and head volume, premise first; a ghost
// end belongs to no volume.
func (g *graph) bundles() []bundle {
	bucket := map[string]int{}
	for k, v := range g.volumes {
		bucket[v.key] = k
	}
	bucketOf := map[string]int{}
	for _, n := range g.nodes {
		if n.kind != kindGhost {
			bucketOf[n.id] = bucket[n.group]
		}
	}
	var out []bundle
	for _, e := range g.edges {
		tail, head := e.emitted()
		t, ok := bucketOf[tail]
		h, ok2 := bucketOf[head]
		if !ok || !ok2 || t == h {
			continue
		}
		i := slices.IndexFunc(out, func(b bundle) bool { return b.tail == t && b.head == h && b.premise == e.stroke.premise })
		if i < 0 {
			out = append(out, bundle{tail: t, head: h, premise: e.stroke.premise})
			i = len(out) - 1
		}
		out[i].edges = append(out[i].edges, e)
	}
	slices.SortFunc(out, func(a, b bundle) int {
		return cmp.Or(cmp.Compare(a.tail, b.tail), cmp.Compare(a.head, b.head), compareBool(b.premise, a.premise))
	})
	return out
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

// digest is one box per volume and one bundle per pair of volumes edges
// cross between.
func (g *graph) digest() string {
	connected := edgeEnds(g.edges)
	lines := []string{
		`digraph "claim-graph-digest" {`,
		`  graph [rankdir=LR nodesep=0.5 ranksep=1.2 pad=0.3 fontname="Helvetica"]`,
		`  node [shape=plaintext fontname="Helvetica" fontsize=10]`,
		`  edge [fontname="Helvetica" fontsize=9 arrowsize=0.8]`,
	}
	for k, v := range g.volumes {
		lines = append(lines, fmt.Sprintf(`  "v_%d" [href=%s label=<%s>]`, k, quoted(v.href), g.volumeTable(v, connected)))
	}
	var crossing []edge
	for _, b := range g.bundles() {
		label := quoted(strokeCounts(b.edges))
		ends := fmt.Sprintf(`"v_%d" -> "v_%d"`, b.tail, b.head)
		if b.premise {
			lines = append(lines, fmt.Sprintf(`  %s [label=%s penwidth=%.1f color="#222222"]`, ends, label, math.Min(1+float64(len(b.edges))/4, 6)))
		} else {
			// A bundle of cuts of either origin draws as the cited cut.
			s := strokeFor(kb.RelationDemoted, false)
			lines = append(lines, fmt.Sprintf(`  %s [label=%s color=%s %s]`, ends, label, quoted(s.color), s.extra))
		}
		crossing = append(crossing, b.edges...)
	}
	if l := legend(strokeRows(crossing)); l != "" {
		lines = append(lines, "  "+l)
	}
	lines = append(lines, sheetLink("← full claim graph", kb.ClaimGraphFile, "max")...)
	return strings.Join(append(lines, "}"), "\n") + "\n"
}

// volumeTable is v's digest box: its title, its nodes by kind, its
// unattached count, its edges within by stroke, and its sheet's link.
func (g *graph) volumeTable(v volume, connected map[string]bool) string {
	members := g.members(v.key)
	inside, kinds := map[string]bool{}, map[string]int{}
	unattached := 0
	for _, n := range members {
		inside[n.id] = true
		kinds[n.kind]++
		if !connected[n.id] {
			unattached++
		}
	}
	var within []edge
	for _, e := range g.edges {
		if inside[e.source] && inside[e.target] {
			within = append(within, e)
		}
	}
	var byKind []string
	for _, s := range nodeStyles {
		if n := kinds[s.kind]; n > 0 {
			byKind = append(byKind, fmt.Sprintf("%d %s", n, plural(s.kind)))
		}
	}
	nodes := count(len(members), "node")
	if len(byKind) > 0 {
		nodes += ": " + strings.Join(byKind, ", ")
	}
	rows := []string{"<b>" + escape(cut(v.title, 44)) + "</b>", escape(nodes), escape(fmt.Sprintf("%d unattached", unattached))}
	if c := strokeCounts(within); c != "" {
		rows = append(rows, escape("within: "+c))
	}
	cells := `<tr><td align="left">` + strings.Join(rows, `</td></tr><tr><td align="left">`) + "</td></tr>"
	if v.key != "" {
		cells += fmt.Sprintf(`<tr><td align="left" href="%s"><font color="%s">claim graph →</font></td></tr>`, escape(v.key+"/"+kb.ClaimGraphFile), linkColor)
	}
	return `<table border="1" cellborder="0" cellspacing="0" cellpadding="3" color="#888888" bgcolor="#fafafa">` + cells + "</table>"
}

// escape is Python's html.escape(s, quote=True), for text inside an
// HTML-like label.
var escape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;").Replace

// quoted is a quoted DOT string of lines joined by DOT's line break, a quote
// and a backslash in each kept literal.
func quoted(lines ...string) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = dotText(l)
	}
	return `"` + strings.Join(parts, `\n`) + `"`
}

var dotText = strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace

// wrap is Python's textwrap.wrap(text, width): chunks are whitespace runs and
// words, a word split after a hyphen inside it; a chunk longer than the width
// is broken.
func wrap(text string, width int) []string {
	return wrapChunks(chunks(text), width)
}

func chunks(text string) []string {
	var out []string
	rs := []rune(text)
	for i := 0; i < len(rs); {
		j := i + 1
		space := unicode.IsSpace(rs[i])
		for j < len(rs) && unicode.IsSpace(rs[j]) == space {
			if !space && rs[j-1] == '-' && j >= 2 && isWordRune(rs[j-2]) && unicode.IsLetter(rs[j]) {
				break
			}
			j++
		}
		chunk := string(rs[i:j])
		if space {
			chunk = strings.Repeat(" ", j-i)
		}
		out = append(out, chunk)
		i = j
	}
	return out
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

func runeLen(s string) int { return len([]rune(s)) }

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

// wrapChunks is TextWrapper._wrap_chunks with drop_whitespace,
// break_long_words and break_on_hyphens.
func wrapChunks(chunks []string, width int) []string {
	slices.Reverse(chunks)
	var lines []string
	for len(chunks) > 0 {
		var cur []string
		curLen := 0
		if isBlank(chunks[len(chunks)-1]) && len(lines) > 0 {
			chunks = chunks[:len(chunks)-1]
		}
		for len(chunks) > 0 {
			l := runeLen(chunks[len(chunks)-1])
			if curLen+l > width {
				break
			}
			cur = append(cur, chunks[len(chunks)-1])
			chunks = chunks[:len(chunks)-1]
			curLen += l
		}
		if len(chunks) > 0 && runeLen(chunks[len(chunks)-1]) > width {
			chunk := []rune(chunks[len(chunks)-1])
			spaceLeft := width - curLen
			if width < 1 {
				spaceLeft = 1
			}
			end := spaceLeft
			if len(chunk) > spaceLeft {
				if hyphen := lastIndexRune(chunk[:spaceLeft], '-'); hyphen > 0 && strings.Trim(string(chunk[:hyphen]), "-") != "" {
					end = hyphen + 1
				}
			}
			cur = append(cur, string(chunk[:end]))
			chunks[len(chunks)-1] = string(chunk[end:])
		}
		if len(cur) > 0 && isBlank(cur[len(cur)-1]) {
			cur = cur[:len(cur)-1]
		}
		if len(cur) > 0 {
			lines = append(lines, strings.Join(cur, ""))
		}
	}
	return lines
}

func lastIndexRune(rs []rune, r rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] == r {
			return i
		}
	}
	return -1
}
