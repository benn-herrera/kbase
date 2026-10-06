package claimgraph

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"kbase/internal/kb"
	"kbase/internal/records"
)

// The display names this package classifies, case-folded. A name outside
// both is admitted as not-claim-bearing and counted, never refused.
var (
	claimBearing    = map[string]bool{"theorem": true, "lemma": true, "proposition": true, "corollary": true, "conjecture": true, "claim": true, "result": true}
	notClaimBearing = map[string]bool{"proof": true, "remark": true, "definition": true, "example": true, "assumption": true, "notation": true, "problem": true}
)

const proofEnvironment = "proof"

// notAClaimTarget is every classified non-result name but proof: a fragment
// naming one of these blocks ends a reference with no target, while one
// naming a proof has the claims that proof establishes as a better answer.
func notAClaimTarget(name string) bool {
	n := strings.ToLower(name)
	return notClaimBearing[n] && n != proofEnvironment
}

func classified(name string) bool {
	n := strings.ToLower(name)
	return claimBearing[n] || notClaimBearing[n]
}

var (
	// printedNameRE is the printed word and number a display line opens with.
	printedNameRE = regexp.MustCompile(`^\*\*([^*]+)\*\*`)
	// citationSpanRE is every citation, by its keys and its rendering.
	citationSpanRE = kb.PyRE(`(?s)<span\s+class="citation"\s+data-cites="(?P<keys>[^"]*)"\s*>(?P<rendering>.*?)</span>`)
	// bibliographyEntryRE is one rendered reference-list entry.
	bibliographyEntryRE = regexp.MustCompile(`(?s)<div id="ref-([^"]+)"[^>]*>(.*?)</div>`)
	tagRE               = regexp.MustCompile(`<[^>]*>`)
	emphasisRunRE       = regexp.MustCompile(`\*+`)
	// precedingWordRE is the last run of non-space characters ahead of a
	// position, past any whitespace and opening bracket.
	precedingWordRE = regexp.MustCompile(`([^` + kb.PyWhitespace + `(\[]+)[` + kb.PyWhitespace + `(\[]*\z`)
	listGapRE       = regexp.MustCompile(`^(?:` + listSeparator + `)\z`)
)

const (
	fenceOpen  = "``` math"
	fenceClose = "```"
	// precedingWindow bounds the characters searched for the word before an
	// anchor; it is longer than any word.
	precedingWindow = 80
)

// Block is one labelled blockquote. Display is the locator span off its
// display line and Title the words that span shows; both are "" together on
// a block whose display line yields neither.
type Block struct {
	Document    string
	Environment string
	Identifier  string
	Title       string
	Display     string
	// Start is the label line and End the line after the block's last.
	Start, End int
	order      int
}

// ClaimBearing is whether the block is a claim site the graph can carry: a
// result-stating name, and a display line that yielded title and locator.
func (b Block) ClaimBearing() bool {
	return claimBearing[strings.ToLower(b.Environment)] && b.Display != ""
}

// MathFence is one display-maths fence and the equation labels inside it.
type MathFence struct {
	Document   string
	Start, End int
	Labels     []string
	within     int
}

// Anchor is one rewritten cross-reference, one per label it names.
type Anchor struct {
	Document      string
	Line          int
	ReferenceType string
	Href          string
	// Target is the document the href names, "" for a bare fragment.
	Target             string
	Fragment           string
	Label              string
	HostingEnvironment string
	// PrecedingWord is the word the page shows before the anchor, carried
	// across a printed list; "" where nothing precedes it.
	PrecedingWord string
	within        int
	opening       bool
}

// Citation is one inline citation key and the state it ended in.
type Citation struct {
	Document string
	Key      string
	State    string
	Line     int
	within   int
}

// Work is one entry of a rendered reference list.
type Work struct {
	Document string
	Key      string
	Text     string
}

// Proof is one proof block and the claim-bearing blocks it establishes. Head
// is the opening emphasis run's anchors by href and label.
type Proof struct {
	Document   string
	Start, End int
	Subjects   []Block
	Head       [][2]string
	order      int
}

func (p Proof) names(a Anchor) bool { return slices.Contains(p.Head, [2]string{a.Href, a.Label}) }

// Inventory is kb_claimgraph stage B: every claim site the tree carries.
type Inventory struct {
	Blocks    []Block
	Fences    []MathFence
	Anchors   []Anchor
	Citations []Citation
	Works     []Work
	Proofs    []Proof
}

func (inv *Inventory) claimBlocks() []Block {
	var out []Block
	for _, b := range inv.Blocks {
		if b.ClaimBearing() {
			out = append(out, b)
		}
	}
	return out
}

// recordInventory is the inventory the records alone give: every reader and
// placement fact, in kb_tools' scan order — documents by path, then where
// each item sits in its document. The page-defined readings (locator span,
// title, line extents, the word before an anchor, a work's text) and the
// proof-to-subject binding are left to Scan.
func recordInventory(recs []records.Record) Inventory {
	recs = scanOrder(recs)
	blockNames := map[int]string{}
	for _, r := range recs {
		if r.Kind == records.KindBlock {
			blockNames[r.Order] = r.Name
		}
	}
	var inv Inventory
	for _, r := range recs {
		switch r.Kind {
		case records.KindBlock:
			inv.Blocks = append(inv.Blocks, Block{Document: r.Document, Environment: r.Name, Identifier: r.Identifier, order: r.Order})
			if strings.EqualFold(r.Name, proofEnvironment) {
				inv.Proofs = append(inv.Proofs, Proof{Document: r.Document, order: r.Order, Head: proofHead(recs, r.Order)})
			}
		case records.KindFence:
			inv.Fences = append(inv.Fences, MathFence{Document: r.Document, Labels: nonNil(r.EquationLabels), within: r.Within})
		case records.KindReference:
			_, fragment, _ := strings.Cut(r.Href, "#")
			for _, label := range anchorLabels(r.Type, r.Labels) {
				inv.Anchors = append(inv.Anchors, Anchor{Document: r.Document, ReferenceType: r.Type, Href: r.Href,
					Target: r.Target, Fragment: fragment, Label: label, HostingEnvironment: blockNames[r.Within],
					within: r.Within, opening: r.Opening})
			}
		case records.KindCitation:
			for i, key := range r.Keys {
				inv.Citations = append(inv.Citations, Citation{Document: r.Document, Key: key, State: r.States[i], within: r.Within})
			}
		case records.KindWork:
			inv.Works = append(inv.Works, Work{Document: r.Document, Key: r.Key})
		}
	}
	return inv
}

// scanOrder is recs in kb_tools' scan order: documents by path, then record
// order, which is where each fact sits on its page.
func scanOrder(recs []records.Record) []records.Record {
	recs = slices.Clone(recs)
	slices.SortStableFunc(recs, func(a, b records.Record) int {
		return cmp.Or(strings.Compare(a.Document, b.Document), cmp.Compare(a.Order, b.Order))
	})
	return recs
}

// proofHead is every opening-run reference of the proof block at order, by
// href and label, sorted and deduplicated.
func proofHead(recs []records.Record, order int) [][2]string {
	var head [][2]string
	for _, r := range recs {
		if r.Kind != records.KindReference || !r.Opening || r.Within != order {
			continue
		}
		for _, l := range anchorLabels(r.Type, r.Labels) {
			if p := [2]string{r.Href, l}; !slices.Contains(head, p) {
				head = append(head, p)
			}
		}
	}
	slices.SortFunc(head, func(a, b [2]string) int { return cmp.Or(strings.Compare(a[0], b[0]), strings.Compare(a[1], b[1])) })
	return head
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// scanInventory joins the records to the page: each document's records, kind
// by kind and in order, to the page constructs kb_tools' readers find there,
// which give the readings the page defines. Records and page disagreeing about
// how many of a construct a document carries means the records describe some
// other tree.
func scanInventory(t *Tree, recs []records.Record) (Inventory, error) {
	recs = scanOrder(recs)
	inv := recordInventory(recs)
	for _, r := range recs {
		if _, ok := t.Documents[r.Document]; !ok {
			return Inventory{}, stopf("records", "record %d (%s) names %s, which is no document of this tree", r.Order, r.Kind, r.Document)
		}
	}
	type span struct{ lo, hi int }
	group := func(n int, doc func(int) string) map[string]span {
		out := map[string]span{}
		for i := 0; i < n; i++ {
			s, ok := out[doc(i)]
			if !ok {
				s.lo = i
			}
			s.hi = i + 1
			out[doc(i)] = s
		}
		return out
	}
	blocks := group(len(inv.Blocks), func(i int) string { return inv.Blocks[i].Document })
	fences := group(len(inv.Fences), func(i int) string { return inv.Fences[i].Document })
	works := group(len(inv.Works), func(i int) string { return inv.Works[i].Document })
	refsByDoc := map[string][]records.Record{}
	citesByDoc := map[string][]records.Record{}
	for _, r := range recs {
		switch r.Kind {
		case records.KindReference:
			refsByDoc[r.Document] = append(refsByDoc[r.Document], r)
		case records.KindCitation:
			citesByDoc[r.Document] = append(citesByDoc[r.Document], r)
		}
	}
	anchorAt := 0
	citeAt := 0
	for _, p := range t.Paths {
		doc := t.Documents[p]
		b := blocks[p]
		if err := readBlocks(doc, inv.Blocks[b.lo:b.hi]); err != nil {
			return Inventory{}, err
		}
		f := fences[p]
		if err := readFences(doc, inv.Fences[f.lo:f.hi]); err != nil {
			return Inventory{}, err
		}
		n := 0
		for _, r := range refsByDoc[p] {
			n += len(anchorLabels(r.Type, r.Labels))
		}
		if err := readAnchors(doc, refsByDoc[p], inv.Anchors[anchorAt:anchorAt+n]); err != nil {
			return Inventory{}, err
		}
		anchorAt += n
		n = 0
		for _, r := range citesByDoc[p] {
			n += len(r.Keys)
		}
		if err := readCitations(doc, citesByDoc[p], inv.Citations[citeAt:citeAt+n]); err != nil {
			return Inventory{}, err
		}
		citeAt += n
		w := works[p]
		if err := readWorks(doc, inv.Works[w.lo:w.hi]); err != nil {
			return Inventory{}, err
		}
	}
	bindProofs(&inv)
	return inv, nil
}

func disagree(doc, construct string, page, recorded int) error {
	return stopf("records", "%s: the page carries %d %s and the records %d; the records describe another tree", doc, page, construct, recorded)
}

// readBlocks reads each block's extent and display line off its label line.
// A result-stating block's span grows until no other display line of the
// document contains it, decided against the lines rather than the spans
// handed out, so two alike blocks both grow.
func readBlocks(doc Document, blocks []Block) error {
	lines := doc.Lines()
	type labelled struct{ start, end int }
	var found []labelled
	var displays []string
	for i, line := range lines {
		name, ok := kb.LabelLine(line)
		if !ok {
			continue
		}
		if len(found) < len(blocks) && name != blocks[len(found)].Environment {
			return stopf("records", "%s:%d: the page's label line names %q where the records name %q; the records describe another tree", doc.Path, i+1, name, blocks[len(found)].Environment)
		}
		end := i
		for end < len(lines) && strings.HasPrefix(lines[end], ">") {
			end++
		}
		found = append(found, labelled{i, end})
		displays = append(displays, displayOf(lines, i+2, end))
	}
	if len(found) != len(blocks) {
		return disagree(doc.Path, "labelled blocks", len(found), len(blocks))
	}
	for i := range blocks {
		b := &blocks[i]
		b.Start, b.End = found[i].start, found[i].end
		title, display, ok := readDisplayLine(displays[i])
		if ok && claimBearing[strings.ToLower(b.Environment)] {
			others := append(slices.Clone(displays[:i]), displays[i+1:]...)
			title, display, ok = distinct(title, display, displays[i], others)
		}
		if ok {
			b.Title, b.Display = pageText(title), display
		}
	}
	return nil
}

// displayOf is a block's content, markers off, unquoted, collapsed to one line.
func displayOf(lines []string, start, end int) string {
	if start >= end {
		return ""
	}
	return kb.NormalizeSpace(strings.Join(kb.SplitLines(unquote(stripMarkers(strings.Join(lines[start:end], "\n")))), " "))
}

// readDisplayLine is the block's title and the span carrying it: the optional
// argument where the author wrote one, else the printed word and number.
func readDisplayLine(display string) (title, span string, ok bool) {
	if title, span, ok := optionalArgument(display); ok {
		return title, span, true
	}
	head := printedNameRE.FindStringIndex(display)
	if head == nil {
		return "", "", false
	}
	end := head[1]
	if strings.HasPrefix(display[end:], ".") {
		end++
	}
	return plainSpan(display[:end]), display[:end], true
}

// optionalArgument is the parenthesised title after the printed name, read
// balanced so a title's own parentheses survive, through the closing bracket
// and the reader's sentence period.
func optionalArgument(display string) (string, string, bool) {
	head := printedNameRE.FindStringIndex(display)
	if head == nil {
		return "", "", false
	}
	cursor := head[1]
	for cursor < len(display) && display[cursor] == ' ' {
		cursor++
	}
	if cursor >= len(display) || display[cursor] != '(' {
		return "", "", false
	}
	depth, closing := 0, -1
	for i := cursor; i < len(display); i++ {
		switch display[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if display[i] == ')' && depth == 0 {
			closing = i
			break
		}
	}
	if closing < 0 {
		return "", "", false
	}
	tail := closing + 1
	if tail < len(display) && display[tail] == '.' {
		tail++
	}
	return display[cursor+1 : closing], display[:tail], true
}

// plainSpan is a display span as a heading: emphasis markers off, no stop.
func plainSpan(span string) string {
	return kb.Strip(strings.TrimRight(kb.Strip(emphasisRunRE.ReplaceAllString(span, "")), "."))
}

// distinct grows the span along display until no line of others contains it.
func distinct(title, span, display string, others []string) (string, string, bool) {
	for slices.ContainsFunc(others, func(o string) bool { return strings.Contains(o, span) }) {
		tail := strings.TrimLeft(display[len(span):], " ")
		if tail == "" {
			return "", "", false
		}
		word, _, _ := strings.Cut(tail, " ")
		span = display[:len(display)-len(tail)] + word
		title = plainSpan(span)
	}
	return title, span, true
}

// mathFenceExtents is each display-maths fence over unquoted lines as
// (opening line, line after the closer), and the opening line of a fence left
// unclosed, or -1.
func mathFenceExtents(lines []string) ([][2]int, int) {
	var extents [][2]int
	opened := -1
	for i, line := range lines {
		s := kb.Strip(line)
		if opened < 0 {
			if s == fenceOpen {
				opened = i
			}
		} else if strings.HasPrefix(s, fenceClose) {
			extents = append(extents, [2]int{opened, i + 1})
			opened = -1
		}
	}
	return extents, opened
}

func readFences(doc Document, fences []MathFence) error {
	extents, unclosed := mathFenceExtents(kb.SplitLines(unquote(doc.Text)))
	if unclosed >= 0 {
		return stopf("math-fence", "%s:%d: a display-maths fence opens and never closes. Point 9 makes the fence a guarantee, and an unclosed one swallows every claim site after it", doc.Path, unclosed+1)
	}
	if len(extents) != len(fences) {
		return disagree(doc.Path, "display-maths fences", len(extents), len(fences))
	}
	for i := range fences {
		fences[i].Start, fences[i].End = extents[i][0], extents[i][1]
	}
	return nil
}

// readAnchors places each reference on its line and reads the word the page
// shows before it. anchors are refs expanded one per label.
func readAnchors(doc Document, refs []records.Record, anchors []Anchor) error {
	text := unquote(stripMarkers(doc.Text))
	matches := anchorRE.FindAllStringIndex(text, -1)
	if len(matches) != len(refs) {
		return disagree(doc.Path, "cross-reference anchors", len(matches), len(refs))
	}
	at := 0
	prevEnd, prevWord, hasPrev := 0, "", false
	for i, m := range matches {
		line := strings.Count(text[:m[0]], "\n")
		var word string
		if hasPrev && prevEnd <= m[0] && listGapRE.MatchString(text[prevEnd:m[0]]) {
			word = prevWord
		} else {
			word = precedingWord(text, m[0])
		}
		closes := strings.Index(text[m[1]:], "</a>")
		prevEnd, prevWord, hasPrev = m[1], word, true
		if closes >= 0 {
			prevEnd = m[1] + closes + len("</a>")
		}
		for range anchorLabels(refs[i].Type, refs[i].Labels) {
			anchors[at].Line, anchors[at].PrecedingWord = line, word
			at++
		}
	}
	return nil
}

func precedingWord(text string, start int) string {
	lo := start
	for n := 0; n < precedingWindow && lo > 0; n++ {
		_, size := utf8.DecodeLastRuneInString(text[:lo])
		lo -= size
	}
	m := precedingWordRE.FindStringSubmatch(text[lo:start])
	if m == nil {
		return ""
	}
	return m[1]
}

func readCitations(doc Document, cites []records.Record, citations []Citation) error {
	text := unquote(stripMarkers(doc.Text))
	matches := citationSpanRE.FindAllStringIndex(text, -1)
	if len(matches) != len(cites) {
		return disagree(doc.Path, "citation spans", len(matches), len(cites))
	}
	at := 0
	for i, m := range matches {
		line := strings.Count(text[:m[0]], "\n")
		for range cites[i].Keys {
			citations[at].Line = line
			at++
		}
	}
	return nil
}

func readWorks(doc Document, works []Work) error {
	matches := bibliographyEntryRE.FindAllStringSubmatch(unquote(stripMarkers(doc.Text)), -1)
	if len(matches) != len(works) {
		return disagree(doc.Path, "reference-list entries", len(matches), len(works))
	}
	for i, m := range matches {
		works[i].Text = kb.NormalizeSpace(tagRE.ReplaceAllString(m[2], ""))
	}
	return nil
}

// bindProofs reads which claim each proof establishes and its extent. Where
// the opening run names anchors, each anchor inside the proof that the run
// names and whose fragment names a claim-bearing block of its target is a
// subject, repeats kept; where it names none, the block immediately above,
// where that block states a result.
func bindProofs(inv *Inventory) {
	byOrder := map[int]Block{}
	byDoc := map[string][]Block{}
	for _, b := range inv.Blocks {
		byOrder[b.order] = b
		byDoc[b.Document] = append(byDoc[b.Document], b)
	}
	for i := range inv.Proofs {
		p := &inv.Proofs[i]
		self := byOrder[p.order]
		p.Start, p.End = self.Start, self.End
		if len(p.Head) > 0 {
			for _, a := range inv.Anchors {
				if a.Document != p.Document || a.within != p.order || !p.names(a) || a.Target == "" || a.Fragment == "" {
					continue
				}
				if s, ok := fragmentBlock(a.Fragment, byDoc[a.Target], true); ok {
					p.Subjects = append(p.Subjects, s)
				}
			}
			continue
		}
		doc := byDoc[p.Document]
		if at := slices.IndexFunc(doc, func(b Block) bool { return b.order == p.order }); at > 0 && doc[at-1].ClaimBearing() {
			p.Subjects = []Block{doc[at-1]}
		}
	}
}

// fragmentBlock is the block whose identifier the fragment names, restricted
// to claim-bearing blocks where claims is set.
func fragmentBlock(fragment string, blocks []Block, claims bool) (Block, bool) {
	if fragment == "" {
		return Block{}, false
	}
	for _, b := range blocks {
		if (!claims || b.ClaimBearing()) && b.Identifier == fragment {
			return b, true
		}
	}
	return Block{}, false
}

// census is the stage-B report: every count in its zero form.
func (inv *Inventory) census() []Finding {
	envs := map[string]int{}
	var envNames []string
	unclassified := map[string]int{}
	var unclassifiedNames []string
	unreadable := map[string]int{}
	var unreadableDocs []string
	hosting := map[string]bool{}
	for _, b := range inv.Blocks {
		if envs[b.Environment] == 0 {
			envNames = append(envNames, b.Environment)
		}
		envs[b.Environment]++
		if !classified(b.Environment) {
			if unclassified[b.Environment] == 0 {
				unclassifiedNames = append(unclassifiedNames, b.Environment)
			}
			unclassified[b.Environment]++
		}
		if claimBearing[strings.ToLower(b.Environment)] && b.Display == "" {
			if unreadable[b.Document] == 0 {
				unreadableDocs = append(unreadableDocs, b.Document)
			}
			unreadable[b.Document]++
		}
		if b.ClaimBearing() {
			hosting[b.Document] = true
		}
	}
	slices.Sort(envNames)
	slices.Sort(unclassifiedNames)
	slices.Sort(unreadableDocs)
	labels, resolved, eqref := 0, 0, 0
	for _, f := range inv.Fences {
		labels += len(f.Labels)
	}
	for _, a := range inv.Anchors {
		if a.Target != "" {
			resolved++
		}
		if a.ReferenceType == "eqref" {
			eqref++
		}
	}
	states := map[string]int{}
	for _, c := range inv.Citations {
		states["inline-"+c.State]++
	}
	states["reference-list"] = len(inv.Works)
	unreadableTotal := 0
	for _, n := range unreadable {
		unreadableTotal += n
	}
	return []Finding{
		fact("stage-B-blocks", field("claim-bearing", len(inv.claimBlocks())), field("labelled", len(inv.Blocks)),
			field("hosting-documents", len(hosting)), field("environments", counts(envNames, envs))),
		fact("stage-B-unclassified", field("names", counts(unclassifiedNames, unclassified))),
		fact("stage-B-unreadable", field("blocks", unreadableTotal), field("documents", counts(unreadableDocs, unreadable))),
		fact("stage-B-maths", field("fences", len(inv.Fences)), field("equation-labels", labels)),
		fact("stage-B-anchors", field("anchors", len(inv.Anchors)), field("resolving", resolved), field("eqref", eqref)),
		fact("stage-B-citations", field("citations", counts([]string{"inline-" + records.StateResolved, "inline-" + records.StateUnanswered,
			"inline-" + records.StateKeyOnly, "reference-list"}, states))),
	}
}
