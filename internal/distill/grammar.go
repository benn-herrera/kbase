package distill

import (
	"fmt"
	"strings"

	"kbase/internal/treeplan"
	"kbase/internal/version"
)

// §4's shared grammar. Three elements appear on more than one node kind, and
// this file is the one implementation of each (I-1, [MAD1: F-9]):
//
//   - RelPath — the relative target of every link kbase emits.
//   - UpLink — line 1 of every class-A file except the entry-point.
//   - Provenance.Footer — the last line of every node kind (SPEC §7, F-12).
//
// Stage 5 renders whole leaves here and stage 8 (internal/assemble) imports
// these three for the index and entry-point kinds. The dependency runs
// downstream — assemble → distill — which is the same direction the pipeline
// runs, and it is what keeps "one implementation, two call sites" a fact about
// the code rather than a note in a design document.
const (
	// upArrow is the up-link's marker, U+2191, from the exemplar's own link
	// grammar (kb-root/CONVENTIONS.md:12-19).
	upArrow = "↑"

	// footerOpen and footerClose bracket the provenance receipt. It is an HTML
	// comment because it is metadata a reader should not have to read and a
	// renderer must not display, and because I-11 rules out frontmatter.
	footerOpen  = "<!-- built from "
	footerClose = " -->"

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

// UpLink renders line 1 of a class-A page: the parent's title, verbatim from
// the tree plan, pointing at the parent's path relative to this page.
func UpLink(parentTitle, rel string) string {
	return "[" + upArrow + " " + parentTitle + "](" + rel + ")"
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

// leaf renders the whole §4.1 file: up-link, H1, verbatim body, footer.
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
func leaf(n treeplan.Node, parentTitle string, body []byte, p Provenance) []byte {
	var b strings.Builder
	b.Grow(len(body) + 256)
	b.WriteString(UpLink(parentTitle, RelPath(n.Path, n.Parent)))
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
