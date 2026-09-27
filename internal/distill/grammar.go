package distill

import (
	"fmt"
	"strings"

	"kbase/internal/treeplan"
	"kbase/internal/version"
)

// §4's shared grammar. Four elements appear on more than one node kind, and
// this file is the one implementation of each (I-1, [MAD1: F-9]):
//
//   - Frontmatter — the Location block at the start of every class-A file.
//   - RelPath — the relative target of every link kbase emits.
//   - The navigation block — the up-link, on every class-A file except the
//     entry-point, and the continuation edges of a split part beneath it.
//   - Provenance.Footer — the last line of every node kind (SPEC §4.6, F-12).
//
// Their readers live here too — ParseFrontmatter, ParseNavLink and NavBlock —
// because a grammar with its writer in one package and its parser in another
// is two grammars that happen to agree today. Stage 9 asks these.
//
// Stage 5 renders whole leaves here and stage 8 (internal/assemble) imports
// them for the index and entry-point kinds. The dependency runs
// downstream — assemble → distill — which is the same direction the pipeline
// runs, and it is what keeps "one implementation, two call sites" a fact about
// the code rather than a note in a design document.
const (
	// UpMarker is the up-link's marker, U+2191, from the exemplar's own link
	// grammar (kb-root/CONVENTIONS.md:12-19).
	UpMarker = "↑"

	// PrevMarker and NextMarker are the continuation edges of a split group:
	// U+2190 and U+2192, the same one-glyph-then-space grammar the up-link
	// uses, because a part's siblings are a projection of the tree plan
	// exactly as its parent is [MAD2: B-4]. They are mechanical tree
	// navigation, not an inferred "related topic" edge — the thing
	// ARCHITECTURE §3 forbids — and the leaf envelope emits them with no model
	// involvement.
	PrevMarker = "←"
	NextMarker = "→"

	// UpLinkPrefix opens an up-link line. It is exported so that a reader
	// outside this package can recognise one without respelling the opening
	// bracket, which would be a second grammar. Guarantee 2 does NOT count with
	// it: the up-link count is taken from NavBlock, positionally, and this
	// prefix names a line's shape rather than its place (see NavBlock).
	UpLinkPrefix = navPrefix + UpMarker + " "

	// navPrefix opens every navigation link, whatever its direction.
	navPrefix = "["

	// footerOpen and footerClose bracket the provenance receipt. It is an HTML
	// comment because it is metadata a reader should not have to read and a
	// renderer must not display — unlike the Location block, which a reader
	// (agent or human) is meant to see and which owns file-start instead.
	footerOpen  = "<!-- built from "
	footerClose = " -->"

	// frontmatterFence opens and closes the Location block. Three hyphens is
	// the convention every Markdown tool already reads as an out-of-band
	// block, which is what keeps the block from rendering as page content.
	frontmatterFence = "---"

	// locationField is the one field kbase writes: the page's own delivered
	// path, root-relative. Written with a trailing colon because that is how
	// it appears on the line, so the writer and the reader share the literal.
	locationField = "Location:"

	// atx is the Markdown heading marker the leaf template de-duplicates
	// against (§4.1).
	atx = "#"

	// headingSpan joins the first and last heading of a split part's
	// descriptor. The glyph is NextMarker's in its ordinary "through" sense:
	// one arrow vocabulary on a page, read the same way in a bullet as in the
	// navigation block.
	headingSpan = " " + NextMarker + " "

	// markdownActive are the characters that mean something to a Markdown
	// reader INSIDE a line: the link brackets, the emphasis pair, the code
	// span, raw HTML's angle brackets, and the backslash that escapes them all.
	// Block-level characters are absent on purpose — a `#` or a `|` in the
	// middle of a bullet is text, and escaping it would be noise in the
	// delivered bytes for no gain.
	markdownActive = "\\`*_[]()<>"
)

// neutralize renders one run of CORPUS bytes as inert text on a page kbase
// composes.
//
// A leaf body is verbatim source and is exempt from this by contract (§4.2
// item 4): its bytes are the document's own and the reader wants them. An
// index page is the opposite — kbase writes every byte of it — so corpus text
// quoted onto one has to arrive as text. Without this, a heading like
// "`](x)` form" copied into a down-link bullet gives the page a live link
// destination the tree does not deliver, and guarantee 1 refuses a delivery
// nobody can repair [GO M-7].
//
// It ESCAPES rather than strips: the label keeps the words the author wrote,
// and a backslash-escaped `]` is exactly what Destinations skips (scan.go's
// escaped), so the neutralization the renderer performs and the one the gate
// observes are the same fact. Control characters become spaces, since a
// newline would split a bullet in two and no escape prevents that.
func neutralize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7f:
			b.WriteByte(' ')
		case strings.ContainsRune(markdownActive, r):
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Provenance is the receipt every delivered page carries (SPEC §4.6): the
// identity of the bytes it was built from, when it was built, and the app
// version — which implies the embedded prompt set.
//
// BuildDate is a job parameter rather than a call to time.Now inside the
// renderer, and that is load-bearing. §5 requires the rendered tree to be a
// pure function of its inputs, and §4.8 guarantee 7 re-derives every leaf and
// compares bytes; a clock read inside the template would break both the moment
// a run crossed midnight. Making the date an input keeps the render pure, and
// putting it in the stage's parameter digest keeps a build on a new day from
// resuming onto pages stamped with an older one.
type Provenance struct {
	// CorpusHash is survey.Totals.ContentHash — the source identity SPEC §4.6
	// asks for, and the same string the tree plan carries.
	CorpusHash string
	// BuildDate is the UTC calendar date, YYYY-MM-DD. A date rather than a
	// timestamp: staleness is what SPEC §4.6 displays, and an hour is not a
	// staleness signal for a documentation corpus.
	BuildDate string
}

// Validate refuses a provenance that would stamp a page with nothing.
//
// It is checked once, at construction, rather than at each render: a footer is
// on every page, so an empty field would otherwise be discovered as a hundred
// identical defects instead of one.
func (p Provenance) Validate() error {
	if strings.TrimSpace(p.CorpusHash) == "" {
		return fmt.Errorf("distill: the provenance receipt names no source corpus")
	}
	if strings.TrimSpace(p.BuildDate) == "" {
		return fmt.Errorf("distill: the provenance receipt carries no build date")
	}
	return nil
}

// Footer renders the receipt as the last line of a delivered page.
func (p Provenance) Footer() string {
	return footerOpen + "corpus sha256:" + p.CorpusHash + " @ " + p.BuildDate +
		"; kbase " + version.Current + footerClose
}

// HasFooter reports whether rendered page bytes end in a provenance receipt.
// It is the check-9 predicate, here rather than in the verifier so that the
// shape and its test are one edit apart.
func HasFooter(data []byte) bool {
	line := lastNonEmptyLine(string(data))
	return strings.HasPrefix(line, footerOpen) && strings.HasSuffix(line, footerClose)
}

// Frontmatter renders the block every class-A page opens with: the page's own
// delivered path, root-relative (SPEC §4.9).
//
// One field, and the label slot of the up-link below it, are the only places a
// delivered page states an address. A page that has been copied, moved or
// rebased wrongly says so here, which is what makes stage 9's Location gate an
// integrity tripwire rather than a restatement of what the renderer just did.
func Frontmatter(nodePath string) string {
	return frontmatterFence + "\n" + locationField + " " + nodePath + "\n" + frontmatterFence
}

// ParseFrontmatter reads a page's opening block: the Location it declares, and the
// lines after the closing fence.
//
// The reader is FORGIVING by ruling: a field this build does not know is
// skipped rather than refused, so a later kbase pass may add one without
// invalidating every page an earlier one delivered. What it does not forgive
// is the block's own shape — a page whose first line is not the fence, or
// whose block never closes, carries no frontmatter at all, and ok is false.
// The gate that asks for a Location is where that becomes a refusal.
func ParseFrontmatter(page []byte) (location string, body []string, ok bool) {
	lines := strings.Split(string(page), "\n")
	if len(lines) == 0 || lines[0] != frontmatterFence {
		return "", lines, false
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == frontmatterFence {
			return location, lines[i+1:], true
		}
		if v, found := strings.CutPrefix(lines[i], locationField); found {
			location = strings.TrimSpace(v)
		}
	}
	return "", lines, false
}

// FirstLine is the first line of a page body that carries anything — the
// up-link, on a class-A page that is not the entry-point. It is here rather
// than in each reader because "the line under the block" is a statement about
// the grammar, and the blank line between them is part of it.
func FirstLine(lines []string) string {
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}

// UpLink renders a class-A page's up-link: the parent's ROOT-RELATIVE path as
// the label, the same parent as a page-relative target (ruled 2026-08-15).
//
// Both projections are computed here from one pair of tree-plan paths, so a
// label that names a different node than its target is not a defect this
// renderer can produce — and stage 9 asserts the agreement over delivered
// bytes, where a copy or a rebase could still break it.
//
// The label carries the global address rather than the parent's title because
// the title is already one hop away and the raw target is already visible: the
// slot is spent on what a reader cannot otherwise see without `../` arithmetic.
func UpLink(nodePath, parentPath string) string { return navLink(UpMarker, nodePath, parentPath) }

// PrevLink and NextLink render a split part's continuation edges: the sibling
// part, in the same two projections and by the same rule as the up-link, so a
// reader who lands mid-series can walk it in either direction without
// re-ascending to the index [MAD2: B-4].
func PrevLink(nodePath, siblingPath string) string {
	return navLink(PrevMarker, nodePath, siblingPath)
}

func NextLink(nodePath, siblingPath string) string {
	return navLink(NextMarker, nodePath, siblingPath)
}

// navLink is the one renderer behind all three: marker, the destination's
// root-relative path as the label, the destination relative to the emitting
// page as the target.
func navLink(marker, nodePath, toPath string) string {
	return navPrefix + marker + " " + toPath + "](" + RelPath(nodePath, toPath) + ")"
}

// NavLink is one navigation edge read back off a delivered page: which
// direction it points, and the two projections of the node it names.
type NavLink struct {
	Marker string
	Label  string
	Target string
}

// ParseNavLink splits a navigation line back into its parts. It is the reader
// half of navLink — all three directions through one parser, here so that the
// writer and the reader cannot drift.
func ParseNavLink(line string) (NavLink, bool) {
	for _, m := range []string{UpMarker, PrevMarker, NextMarker} {
		rest, found := strings.CutPrefix(line, navPrefix+m+" ")
		if !found {
			continue
		}
		label, rest, found := strings.Cut(rest, "](")
		if !found {
			return NavLink{}, false
		}
		target, found := strings.CutSuffix(rest, ")")
		if !found {
			return NavLink{}, false
		}
		return NavLink{Marker: m, Label: label, Target: target}, true
	}
	return NavLink{}, false
}

// NavBlock is the navigation block a class-A page opens its body with: the
// run of navigation lines that begins at the first line under the frontmatter
// block and ends at the first line that is not one.
//
// It is POSITIONAL, and that is the point. A leaf body is verbatim source
// (I-3) and may legitimately contain a navigation-shaped line of its own — a
// docs site with its own prev/next furniture is the ordinary case, not an
// exotic one — so a check that read the whole page would refuse deliveries
// nobody can repair, which is the class of defect the fenced up-link was
// [MAD2: B-11]. The envelope puts every link it emits above the body and a
// blank line below them, so the run stops before the first source byte.
func NavBlock(body []string) []NavLink {
	var out []NavLink
	for _, l := range body {
		if strings.TrimSpace(l) == "" {
			if len(out) > 0 {
				return out
			}
			continue
		}
		link, ok := ParseNavLink(l)
		if !ok {
			return out
		}
		out = append(out, link)
	}
	return out
}

// RelPath is the link target from one node to another: a pure function of two
// tree-plan paths, computed here and nowhere else (§2.2 — relative paths are
// derivable and therefore absent from the schema).
//
// Both arguments are node paths, not directories: the result is relative to
// the directory the FROM page sits in, which is what a Markdown renderer
// resolves a destination against.
func RelPath(from, to string) string {
	fromSegs := strings.Split(from, "/")
	toSegs := strings.Split(to, "/")
	// The last segment of `from` is the emitting file itself, so its directory
	// is everything before it.
	fromDir := fromSegs[:len(fromSegs)-1]

	i := 0
	for i < len(fromDir) && i < len(toSegs)-1 && fromDir[i] == toSegs[i] {
		i++
	}
	var out []string
	for range fromDir[i:] {
		out = append(out, "..")
	}
	out = append(out, toSegs[i:]...)
	return strings.Join(out, "/")
}

// leaf renders the whole §4.1 file: frontmatter, the navigation block, H1,
// verbatim body, footer.
//
// The block owns file-start, above the navigation. A verbatim slice that
// carries the SOURCE document's own frontmatter therefore renders it as body
// text, which is intentional and stated in SPEC §4.9: the envelope wraps the
// body, and the body is never edited.
//
// prev and next are the sibling parts of this leaf's own split group, empty
// where there is none — the first part has no previous and the last no next.
// They render immediately under the up-link and above the body: all mechanical
// navigation in one block, where a reader lands, and every line the gates read
// as grammar sitting above the first verbatim byte (NavBlock).
//
// The heading de-duplication is §4.1's provisional, NARROWED to its own stated
// reason [MAD2: B-8]: a synthesised H1 is suppressed only when the slice
// already opens with a heading whose text IS the node's title, so a page
// cannot be delivered stating an identity it was not routed by — or stating
// none at all, which is what a part ≥2 opening at some interior `##` used to
// do. A leaf whose opening heading says something else keeps that heading and
// gains the title above it; the redundancy is the cheap side of the trade.
func leaf(n treeplan.Node, prev, next string, body []byte, p Provenance) []byte {
	var b strings.Builder
	b.Grow(len(body) + 256)
	b.WriteString(Frontmatter(n.Path))
	b.WriteString("\n\n")
	b.WriteString(UpLink(n.Path, n.Parent))
	b.WriteString("\n")
	if prev != "" {
		b.WriteString(PrevLink(n.Path, prev))
		b.WriteString("\n")
	}
	if next != "" {
		b.WriteString(NextLink(n.Path, next))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if !opensWithTitle(body, n.Title) {
		b.WriteString("# ")
		b.WriteString(n.Title)
		b.WriteString("\n\n")
	}
	b.Write(body)
	if !endsWithNewline(body) {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(p.Footer())
	b.WriteString("\n")
	return []byte(b.String())
}

// opensWithTitle reports whether a slice's first non-empty line is an ATX
// heading whose own text is the node's title — the predicate §4.2 item 3's
// reason states, rather than the wider "any heading" it used to be.
//
// Setext headings (an underline of `=` or `-`) are deliberately not detected:
// recognising one means reading the NEXT line to classify this one, and a leaf
// that gets one redundant H1 above its own setext title is a cosmetic defect
// where a mis-detected `---` frontmatter fence would be a structural one.
func opensWithTitle(body []byte, title string) bool {
	for _, line := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		text, ok := headingText(t)
		return ok && text == title
	}
	return false
}

// headingText is an ATX heading's own text: the marker run, one space, the
// text, and an optional closing run of markers that is not part of it. A line
// whose markers are not followed by a space is not a heading (`#tag`), and
// neither is a run of more than six.
func headingText(line string) (string, bool) {
	marks := len(line) - len(strings.TrimLeft(line, atx))
	if marks == 0 || marks > 6 {
		return "", false
	}
	rest := line[marks:]
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
		return "", false
	}
	t := strings.TrimSpace(rest)
	// A closing sequence is a whole space-separated run of markers, so `# C#`
	// keeps its `#` and `## Title ##` does not.
	if i := strings.LastIndex(t, " "); i >= 0 && strings.Trim(t[i+1:], atx) == "" {
		t = strings.TrimSpace(t[:i])
	}
	return t, true
}

func endsWithNewline(b []byte) bool { return len(b) > 0 && b[len(b)-1] == '\n' }

// lastNonEmptyLine is the last line of a page that carries anything — the
// provenance footer on every well-formed one.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}
