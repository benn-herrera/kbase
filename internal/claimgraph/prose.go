package claimgraph

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/write"
)

// The node pass reads a leaf's readable prose: every line outside its
// headings and its claim-bearing, proof and definition blocks. A paragraph of
// it holding a reference that resolves to a document is obligated, owed
// exactly one verdict.

const definitionEnvironment = "definition"

// excludedLines is every line of a leaf's marker-stripped body the node pass
// does not read as prose: its headings, and its claim-bearing, proof and
// definition blocks'.
func excludedLines(body string, blocks []Block) map[int]bool {
	out := headingLines(body)
	for _, b := range blocks {
		env := strings.ToLower(b.Environment)
		if b.ClaimBearing() || env == proofEnvironment || env == definitionEnvironment {
			for n := b.Start; n < b.End; n++ {
				out[n] = true
			}
		}
	}
	return out
}

// leafReading is one leaf as the node pass reads it: its bytes on disk, the
// marker-stripped body the ask is shown, its fences and claim-bearing blocks,
// and the render over the readable region.
type leafReading struct {
	document, text string
	fences         []MathFence
	claimBlocks    []Block
	render         render
}

func readLeaf(doc Document, inv *Inventory) *leafReading {
	r := &leafReading{document: doc.Path, text: doc.Text}
	var blocks []Block
	for _, b := range inv.Blocks {
		if b.Document == doc.Path {
			blocks = append(blocks, b)
			if b.ClaimBearing() {
				r.claimBlocks = append(r.claimBlocks, b)
			}
		}
	}
	for _, f := range inv.Fences {
		if f.Document == doc.Path {
			r.fences = append(r.fences, f)
		}
	}
	body := stripMarkers(doc.Text)
	r.render = renderText(body, r.fences, excludedLines(body, blocks))
	return r
}

// obligated is the paragraphs holding a resolving reference, in order.
func (r *leafReading) obligated(inv *Inventory) []paragraph {
	holding := map[int]bool{}
	for _, a := range inv.Anchors {
		if a.Document == r.document && a.Target != "" {
			if p, ok := r.render.paragraphAt(a.Line); ok {
				holding[p.start] = true
			}
		}
	}
	var out []paragraph
	for _, p := range r.render.paragraphs {
		if holding[p.start] {
			out = append(out, p)
		}
	}
	return out
}

// standing is where a reference stands: in an excluded block, where the block
// and proof rules apply; in prose judged a claim or not; or in prose no
// recorded verdict judged — never asked, unread, or defaulted.
type standing int

const (
	standingOutside standing = iota
	standingClaim
	standingNotAClaim
	standingUnjudged
)

func (r *leafReading) standing(a Anchor, entry LeafEntry) standing {
	p, ok := r.render.paragraphAt(a.Line)
	if !ok {
		return standingOutside
	}
	for _, v := range entry.Verdicts {
		if v.Line != p.start {
			continue
		}
		switch v.Verdict {
		case JudgementClaim:
			return standingClaim
		case JudgementNotAClaim:
			return standingNotAClaim
		}
		break
	}
	return standingUnjudged
}

// marks is whether id's Tier-2 marker sits in p: the paragraph its claim was
// minted from.
func (r *leafReading) marks(p paragraph, id string) bool {
	lines := kb.SplitLines(r.text)
	marker := write.Tier2Marker(id)
	for _, n := range p.lines {
		if n < len(lines) && strings.Contains(lines[n], marker) {
			return true
		}
	}
	return false
}

// claimOf is the claim of hosted whose marker sits in a's paragraph.
func (r *leafReading) claimOf(a Anchor, hosted []ClaimNode) (ClaimNode, bool) {
	p, ok := r.render.paragraphAt(a.Line)
	if !ok {
		return ClaimNode{}, false
	}
	for _, n := range hosted {
		if r.marks(p, n.ID) {
			return n, true
		}
	}
	return ClaimNode{}, false
}

// minOpeningWords is the fewest canonical words a sentence carries to be a
// claim's opening: measured, every folded collision at three words or fewer
// was a structural label.
const minOpeningWords = 4

// titleMaxChars is the longest a prose claim's derived title runs.
const titleMaxChars = 120

// opensAClaim is whether a sentence may be where a result begins: enough
// canonical words, and not inside a maths fence — a claim may contain an
// equation and may not be one.
func opensAClaim(s sentence, fences []MathFence) bool {
	if len(strings.Fields(write.CanonicalForm(s.text))) < minOpeningWords {
		return false
	}
	return !slices.ContainsFunc(fences, func(f MathFence) bool { return f.Start <= s.line && s.line < f.End })
}

// askedParagraph is one paragraph the node pass asks about; position is its
// 1-based place among the leaf's readable paragraphs.
type askedParagraph struct {
	paragraph paragraph
	span      span
	position  int
}

func (a askedParagraph) name() string { return a.span.locator() }

// labelled is the paragraph's sentences as the render shows them.
func (a askedParagraph) labelled() string {
	lines := make([]string, len(a.span))
	for i, s := range a.span {
		lines[i] = s.label + ": " + s.text
	}
	return strings.Join(lines, "\n")
}

// askedParagraphs is every obligated paragraph and every other holding a
// sentence that could open a claim, in document order.
func (r *leafReading) askedParagraphs(obligated []paragraph) []askedParagraph {
	opening := map[int]bool{}
	for _, s := range r.render.sentences {
		if opensAClaim(s, r.fences) {
			opening[s.paragraph] = true
		}
	}
	for _, p := range obligated {
		opening[p.index] = true
	}
	var out []askedParagraph
	for i, p := range r.render.paragraphs {
		if opening[p.index] {
			out = append(out, askedParagraph{p, r.render.spanOf(p), i + 1})
		}
	}
	return out
}

// derivedTitle is the title a yes mints under: the paragraph's first sentence
// that could open a claim (its first where none could), as the page shows it,
// on one line, cut at the last word boundary within titleMaxChars, and
// suffixed with its position where the leaf already carries that title.
func derivedTitle(a askedParagraph, fences []MathFence, taken map[string]bool) string {
	if len(a.span) == 0 {
		return fmt.Sprintf("(¶%d)", a.position)
	}
	opener := a.span[0]
	for _, s := range a.span {
		if opensAClaim(s, fences) {
			opener = s
			break
		}
	}
	words := kb.NormalizeSpace(pageText(opener.text))
	if r := []rune(words); len(r) > titleMaxChars {
		cut := string(r[:titleMaxChars])
		if i := strings.LastIndex(cut, " "); i >= 0 {
			cut = cut[:i]
		} else {
			cr := []rune(cut)
			cut = string(cr[:len(cr)-1])
		}
		words = cut + "…"
	}
	if words != "" && !taken[words] {
		return words
	}
	return kb.LStrip(fmt.Sprintf("%s (¶%d)", words, a.position))
}

// proseClaim is one claim a yes mints, placed: its title, its excerpt — the
// whole paragraph, and the locator its marker lands by — and the line it
// located at.
type proseClaim struct {
	title, excerpt string
	line           int
}

// paragraphAnswer is one asked paragraph and what came back: whether it
// states a result, nil where no offered letter did.
type paragraphAnswer struct {
	asked        askedParagraph
	statesResult *bool
}

// judge is one leaf's verdicts and claims from its answers. A paragraph with
// no letter, and a yes whose slice the write path cannot place exactly once,
// are recorded defaulted with their cause.
func (r *leafReading) judge(answers []paragraphAnswer) ([]proseClaim, []ParagraphVerdict) {
	taken := map[string]bool{}
	for _, b := range r.claimBlocks {
		if b.Title != "" {
			taken[b.Title] = true
		}
	}
	answers = slices.Clone(answers)
	slices.SortStableFunc(answers, func(x, y paragraphAnswer) int { return x.asked.paragraph.start - y.asked.paragraph.start })
	var claims []proseClaim
	var verdicts []ParagraphVerdict
	defaulted := func(line int, cause string) ParagraphVerdict {
		return ParagraphVerdict{Line: line, Verdict: JudgementDefaulted, Cause: &cause}
	}
	for _, a := range answers {
		line := a.asked.paragraph.start
		switch {
		case a.statesResult == nil:
			verdicts = append(verdicts, defaulted(line, CauseNoLetter))
			continue
		case !*a.statesResult:
			verdicts = append(verdicts, ParagraphVerdict{Line: line, Verdict: JudgementNotAClaim})
			continue
		}
		title := derivedTitle(a.asked, r.fences, taken)
		taken[title] = true
		excerpt := a.asked.span.excerpt()
		lines := write.ExcerptLines(r.text, excerpt)
		if len(lines) != 1 {
			verdicts = append(verdicts, defaulted(line, CauseUnplaceable))
			continue
		}
		claims = append(claims, proseClaim{title, excerpt, lines[0]})
		verdicts = append(verdicts, ParagraphVerdict{Line: line, Verdict: JudgementClaim})
	}
	return claims, verdicts
}

// proseRationale is a prose claim's register rationale.
func proseRationale(document string) string {
	return "Identified in the prose of " + document + "; the span this entry names is anchored in that document by " +
		"this claim's Tier-2 marker. Neither dependency attribution nor rigor assessment has run over it."
}
