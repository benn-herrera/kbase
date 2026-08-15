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
//   - UpLink — the first line after the block, on every class-A file except
//     the entry-point.
//   - Provenance.Footer — the last line of every node kind (SPEC §7, F-12).
//
// Their readers live here too — ParseFrontmatter and ParseUpLink — because a
// grammar with its writer in one package and its parser in another is two
// grammars that happen to agree today. Stage 9 asks these.
//
// Stage 5 renders whole leaves here and stage 8 (internal/assemble) imports
// them for the index and entry-point kinds. The dependency runs
// downstream — assemble → distill — which is the same direction the pipeline
// runs, and it is what keeps "one implementation, two call sites" a fact about
// the code rather than a note in a design document.
const (
	// upArrow is the up-link's marker, U+2191, from the exemplar's own link
	// grammar (kb-root/CONVENTIONS.md:12-19).
	upArrow = "↑"

	// UpLinkPrefix opens an up-link line. It is exported because stage 9
	// counts and locates up-links with it, and a second spelling of the
	// opening bracket in the verifier would be a second grammar.
	UpLinkPrefix = "[" + upArrow + " "

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
)

// Provenance is the receipt every delivered page carries (SPEC §7): the
// identity of the bytes it was built from, when it was built, and the app
// version — which implies the embedded prompt set.
//
// BuildDate is a job parameter rather than a call to time.Now inside the
// renderer, and that is load-bearing. §8.1 requires the rendered tree to be a
// pure function of its inputs, and §9 check 7 re-derives every leaf and
// compares bytes; a clock read inside the template would break both the moment
// a run crossed midnight. Making the date an input keeps the render pure, and
// putting it in the stage's parameter digest keeps a build on a new day from
// resuming onto pages stamped with an older one.
type Provenance struct {
	// CorpusHash is survey.Totals.ContentHash — the source identity SPEC §7
	// asks for, and the same string the tree plan carries.
	CorpusHash string
	// BuildDate is the UTC calendar date, YYYY-MM-DD. A date rather than a
	// timestamp: staleness is what SPEC §7 displays, and an hour is not a
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
func UpLink(nodePath, parentPath string) string {
	return UpLinkPrefix + parentPath + "](" + RelPath(nodePath, parentPath) + ")"
}

// ParseUpLink splits an up-link line back into its two projections. It is the
// reader half of UpLink, here so that the two cannot drift.
func ParseUpLink(line string) (label, target string, ok bool) {
	rest, found := strings.CutPrefix(line, UpLinkPrefix)
	if !found {
		return "", "", false
	}
	label, rest, found = strings.Cut(rest, "](")
	if !found {
		return "", "", false
	}
	target, found = strings.CutSuffix(rest, ")")
	if !found {
		return "", "", false
	}
	return label, target, true
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

// leaf renders the whole §4.1 file: frontmatter, up-link, H1, verbatim body,
// footer.
//
// The block owns file-start, above the up-link. A verbatim slice that carries
// the SOURCE document's own frontmatter therefore renders it as body text,
// which is intentional and stated in SPEC §4.9: the envelope wraps the body,
// and the body is never edited.
//
// The heading de-duplication is §4.1's provisional, implemented as stated: a
// slice that already opens with its own ATX heading keeps it and gets no
// synthesised H1, because a span starting at a survey.Section starts at the
// heading line itself and the alternative double-titles every leaf. The
// consequence, stated so it is a decision rather than a surprise: a leaf cut
// from a level-2 section opens at `##` and carries no H1 at all. The node's
// title still identifies it everywhere navigation happens — the parent's
// down-link, the child's up-link, the tree plan — because none of those read a
// heading (I-1).
func leaf(n treeplan.Node, body []byte, p Provenance) []byte {
	var b strings.Builder
	b.Grow(len(body) + 256)
	b.WriteString(Frontmatter(n.Path))
	b.WriteString("\n\n")
	b.WriteString(UpLink(n.Path, n.Parent))
	b.WriteString("\n\n")
	if !opensWithHeading(body) {
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

// opensWithHeading reports whether a slice's first non-empty line is an ATX
// heading. Setext headings (an underline of `=` or `-`) are deliberately not
// detected: recognising one means reading the NEXT line to classify this one,
// and a leaf that gets one redundant H1 above its own setext title is a
// cosmetic defect where a mis-detected `---` frontmatter fence would be a
// structural one.
func opensWithHeading(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		return strings.HasPrefix(t, atx)
	}
	return false
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
