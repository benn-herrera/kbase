package claimgraph

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/result"
)

const leafText = "[↑ V](index.md)\n" +
	"\n" +
	"# A\n" +
	"\n" +
	"We show that every widget is a gadget. Fig. 3 shows it,\n" +
	"and the bound holds.\n" +
	"\n" +
	"> **Theorem 1**.\n" +
	"> Every widget is a gadget.\n" +
	"\n" +
	"Then $x. Y$ holds and so\n" +
	"``` math\n" +
	"a = b\n" +
	"```\n" +
	"follows.\n"

func TestRenderParagraphs(t *testing.T) {
	excluded := excludedLines(leafText, []Block{{Environment: "Theorem", Display: "**Theorem 1**.", Start: 7, End: 9}})
	r := renderText(leafText, []MathFence{{Start: 11, End: 14}}, excluded)
	var texts []string
	for _, s := range r.sentences {
		texts = append(texts, fmt.Sprintf("%s@%d/%d %s", s.label, s.line, s.paragraph, s.text))
	}
	want := []string{
		"S1@4/1 We show that every widget is a gadget.",
		"S2@4/1 Fig. 3 shows it, and the bound holds.",
		"S3@10/2 Then $x. Y$ holds and so",
		"S4@11/2 ``` math a = b ```",
		"S5@14/2 follows.",
	}
	if !slices.Equal(texts, want) {
		t.Errorf("sentences =\n%s\nwant\n%s", strings.Join(texts, "\n"), strings.Join(want, "\n"))
	}
	if len(r.paragraphs) != 2 || r.paragraphs[0].start != 4 || r.paragraphs[1].start != 10 || !slices.Equal(r.paragraphs[1].lines, []int{10, 11, 12, 13, 14}) {
		t.Errorf("paragraphs = %+v", r.paragraphs)
	}
	if !strings.Contains(r.text, "# A\n") || !strings.Contains(r.text, "> **Theorem 1**.\n") || strings.Contains(r.text, ": # A") {
		t.Errorf("the heading and the block are not shown unlabelled:\n%s", r.text)
	}
	if !strings.Contains(r.text, "S3: Then $x. Y$ holds and so\nS4:\n``` math\na = b\n```\nS5: follows.\n") {
		t.Errorf("the fence is not shown line by line under one label:\n%s", r.text)
	}
	if got := r.spanOf(r.paragraphs[1]).locator(); got != "S3-S5" {
		t.Errorf("the fenced paragraph's locator = %q", got)
	}
	if got := r.spanOf(r.paragraphs[0]).locator(); got != "S1-S2" {
		t.Errorf("locator = %q", got)
	}
	if got := r.spanOf(r.paragraphs[0]).excerpt(); got != "We show that every widget is a gadget. Fig. 3 shows it, and the bound holds." {
		t.Errorf("excerpt = %q", got)
	}
	if got := r.spanOf(r.paragraphs[1]).excerpt(); got != "Then $x. Y$ holds and so ``` math a = b ``` follows." {
		t.Errorf("the fenced paragraph's excerpt = %q", got)
	}
}

// leafWithReferences is a leaf opening on its up-link, then a body whose
// sentence carries a cross-reference and a citation.
const leafWithReferences = "[↑ V](index.md)\n" +
	"\n" +
	"# A\n" +
	"\n" +
	"By <a href=\"b.md#l\" data-reference-type=\"ref\" data-reference=\"lem:l\">Lemma 2</a> the bound holds\n" +
	"<span class=\"citation\" data-cites=\"k01\">(K 2001)</span>.\n"

const leafWithReferencesSentence = "By <a href=\"b.md#l\" data-reference-type=\"ref\" data-reference=\"lem:l\">Lemma 2</a> the bound holds " +
	"<span class=\"citation\" data-cites=\"k01\">(K 2001)</span>."

func TestRenderBeginsAtTheHeading(t *testing.T) {
	r := renderText(leafWithReferences, nil, excludedLines(leafWithReferences, nil))
	if !strings.HasPrefix(r.text, "# A\n") || strings.Contains(r.text, "[↑ V]") {
		t.Errorf("the render does not begin at the heading:\n%s", r.text)
	}
	if len(r.sentences) != 1 {
		t.Fatalf("sentences = %+v", r.sentences)
	}
	s := r.sentences[0]
	if s.line != 4 || !strings.HasPrefix(leafWithReferences[s.start:], "By <a href=") || !strings.HasSuffix(leafWithReferences[:s.end], "</span>.") {
		t.Errorf("S1 at line %d, bytes %q; want line 4 and the raw sentence of the original text", s.line, leafWithReferences[s.start:s.end])
	}
	if p, ok := r.paragraphAt(5); !ok || p.start != 4 {
		t.Errorf("the paragraph at line 5 = %+v, %v; want the one opening at line 4", p, ok)
	}
}

func TestRenderShowsSentencesAsThePageShowsThem(t *testing.T) {
	leaf := &leafReading{document: "v/a.md", text: leafWithReferences}
	leaf.render = renderText(leafWithReferences, nil, excludedLines(leafWithReferences, nil))
	shown := "S1: By Lemma 2 the bound holds (K 2001)."
	if !strings.Contains(leaf.render.text, shown+"\n") || strings.Contains(leaf.render.text, "<a ") || strings.Contains(leaf.render.text, "<span") {
		t.Errorf("the sentence is not shown as the page shows it:\n%s", leaf.render.text)
	}
	asked := leaf.askedParagraphs(nil)
	if len(asked) != 1 || asked[0].labelled() != shown {
		t.Fatalf("asked = %+v", asked)
	}
	if got := asked[0].span.excerpt(); got != leafWithReferencesSentence {
		t.Errorf("excerpt = %q, want the raw span", got)
	}
	yes := true
	claims, _ := leaf.judge([]paragraphAnswer{{asked[0], &yes}})
	if len(claims) != 1 || claims[0].line != 4 {
		t.Errorf("claims = %+v, want the raw span located at line 4", claims)
	}
}

func TestAskedParagraphsAndTitles(t *testing.T) {
	leaf := &leafReading{document: "v/a.md", text: leafText, fences: []MathFence{{Start: 11, End: 14}}}
	leaf.render = renderText(leafText, leaf.fences, excludedLines(leafText, []Block{{Environment: "Theorem", Display: "**Theorem 1**.", Start: 7, End: 9}}))
	asked := leaf.askedParagraphs(nil)
	var names []string
	for _, a := range asked {
		names = append(names, fmt.Sprintf("%s#%d", a.name(), a.position))
	}
	if want := []string{"S1-S2#1", "S3-S5#2"}; !slices.Equal(names, want) {
		t.Errorf("asked = %q, want the paragraphs holding an opening sentence", names)
	}
	if got := derivedTitle(asked[0], leaf.fences, map[string]bool{}); got != "We show that every widget is a gadget." {
		t.Errorf("title = %q", got)
	}
	if got := derivedTitle(asked[0], leaf.fences, map[string]bool{"We show that every widget is a gadget.": true}); got != "We show that every widget is a gadget. (¶1)" {
		t.Errorf("a taken title = %q, want its position suffixed", got)
	}
	long := askedParagraph{span: span{{text: strings.Repeat("word ", 30) + "end."}}, position: 2}
	if got := derivedTitle(long, nil, map[string]bool{}); !strings.HasSuffix(got, "word…") || len([]rune(got)) > titleMaxChars+1 {
		t.Errorf("a long title = %q, want it cut at a word boundary within %d", got, titleMaxChars)
	}
	cited := askedParagraph{span: span{{text: `By <a href="b.md#l" data-reference-type="ref" data-reference="l">Lemma 2</a> we show the claim.`}}}
	if got := derivedTitle(cited, nil, map[string]bool{}); got != "By Lemma 2 we show the claim." {
		t.Errorf("a title over an anchor = %q, want what the page shows", got)
	}
}

func TestJudgeRecordsEveryVerdict(t *testing.T) {
	text := "[↑ V](index.md)\n\n# A\n\nWe show the bound holds for all x.\n\nThe bound holds for all x.\n\nWe recall the notation used here.\n\nThe bound holds for all x.\n"
	leaf := &leafReading{document: "v/a.md", text: text}
	leaf.render = renderText(text, nil, excludedLines(text, nil))
	asked := leaf.askedParagraphs(nil)
	if len(asked) != 4 {
		t.Fatalf("asked %d paragraphs", len(asked))
	}
	yes, no := true, false
	claims, verdicts := leaf.judge([]paragraphAnswer{{asked[3], &yes}, {asked[0], &yes}, {asked[2], &no}, {asked[1], nil}})
	var got []string
	for _, v := range verdicts {
		s := fmt.Sprintf("%d %s", v.Line, v.Verdict)
		if v.Cause != nil {
			s += " " + *v.Cause
		}
		got = append(got, s)
	}
	want := []string{"4 claim", "6 defaulted no-letter", "8 not-a-claim", "10 defaulted unplaceable"}
	if !slices.Equal(got, want) {
		t.Errorf("verdicts = %q, want %q (a yes whose paragraph appears twice is unplaceable)", got, want)
	}
	if len(claims) != 1 || claims[0].title != "We show the bound holds for all x." || claims[0].line != 4 {
		t.Errorf("claims = %+v", claims)
	}
}

// letters answers each candidate with the letter its target is scripted to.
type letters struct {
	mu    sync.Mutex
	by    map[string]string
	asked []string
}

func (l *letters) read(_ context.Context, q asks.LetterQuestion) (asks.Reply, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.asked = append(l.asked, q.Group+"->"+q.Item)
	return asks.Reply{Text: l.by[q.Group+"->"+q.Item]}, nil
}

func TestClassifyThroughTheLetterSeam(t *testing.T) {
	node := func(id string) ClaimNode { return ClaimNode{ID: id, Document: "v/a.md", Title: "Claim " + id} }
	mk := func(s, tg string, offered []Relation, draft Relation) candidate {
		return candidate{source: node(s), target: node(tg), offered: offered, draft: draft, passages: []string{s + " names " + tg}}
	}
	all := []Relation{SupportedBy, InSupportOf, MentionedBy}
	directed := []Relation{SupportedBy, MentionedBy}
	candidates := []candidate{
		mk("clm-a", "clm-b", directed, SupportedBy),
		mk("clm-b", "clm-a", all, MentionedBy),
		mk("clm-b", "clm-c", all, MentionedBy),
		mk("clm-c", "clm-d", all, MentionedBy),
	}
	repo := t.TempDir()
	if err := WriteClassification(repo, buildrecords.ClassificationRecord{}); err != nil {
		t.Fatal(err)
	}
	reader := &letters{by: map[string]string{
		"clm-a->clm-b": "A", "clm-b->clm-a": "A", "clm-b->clm-c": "B", "clm-c->clm-d": "C",
	}}
	statement := func(n ClaimNode) string { return n.Title }
	var units []string
	total := 0
	progress := Progress{
		Units: func(n int) error { total = n; return nil },
		Unit:  func(name string) error { units = append(units, name); return nil },
	}
	c, err := classify(context.Background(), repo, candidates, statement, Options{Reader: reader.read, Progress: progress})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || !slices.Equal(units, []string{"clm-a", "clm-b", "clm-c"}) {
		t.Errorf("units %q of %d, want one per source's ask group", units, total)
	}
	if want := []pair{{"clm-a", "clm-b"}, {"clm-b", "clm-a"}, {"clm-c", "clm-b"}}; !slices.Equal(c.edges, want) {
		t.Errorf("depends edges = %v, want every answer written, the ring both ways and in-support-of target to source: %v", c.edges, want)
	}
	if want := []pair{{"clm-c", "clm-d"}}; !slices.Equal(c.refs, want) {
		t.Errorf("references = %v, want the mention", c.refs)
	}
	rec, _, _ := buildrecords.ReadClassification(recordsAt(repo))
	if len(rec.Candidates) != 4 || rec.Candidates[0].Outcome != string(asks.Answered) || !slices.Equal(rec.Candidates[0].Offered, []string{"A", "C"}) {
		t.Errorf("record = %+v", rec.Candidates)
	}

	again := &letters{by: map[string]string{}}
	if _, err := classify(context.Background(), repo, candidates, statement, Options{Reader: again.read}); err != nil || len(again.asked) != 0 {
		t.Errorf("a resume asked %q, %v; want nothing the record holds", again.asked, err)
	}
}

func TestClassifyDefaultsToTheDraft(t *testing.T) {
	src := ClaimNode{ID: "clm-a", Document: "v/a.md", Title: "A"}
	tgt := ClaimNode{ID: "clm-b", Document: "v/b.md", Title: "B"}
	candidates := []candidate{{source: src, target: tgt, offered: []Relation{SupportedBy, MentionedBy}, draft: SupportedBy, passages: []string{"p"}}}
	repo := t.TempDir()
	if err := WriteClassification(repo, buildrecords.ClassificationRecord{}); err != nil {
		t.Fatal(err)
	}
	var fallbacks []result.Item
	progress := Progress{Fallback: func(it result.Item) error { fallbacks = append(fallbacks, it); return nil }}
	drafted, err := classify(context.Background(), repo, candidates, func(n ClaimNode) string { return n.Title }, Options{Progress: progress})
	if err != nil || drafted.outcomes[candidates[0].pair()] != ClassifyDrafted || !slices.Equal(drafted.edges, []pair{{"clm-a", "clm-b"}}) {
		t.Fatalf("with no reader = %+v, %v", drafted, err)
	}
	if len(fallbacks) != 0 {
		t.Errorf("a run that cannot ask reported fallbacks %+v; its drafts were never asked", fallbacks)
	}
	unreadable := &letters{by: map[string]string{"clm-a->clm-b": "B"}}
	got, err := classify(context.Background(), repo, candidates, func(n ClaimNode) string { return n.Title }, Options{Reader: unreadable.read, Progress: progress})
	if err != nil {
		t.Fatal(err)
	}
	if len(fallbacks) != 1 || fallbacks[0].Check != checkDefaulted || fallbacks[0].Path != "v/a.md" || !strings.Contains(fallbacks[0].Detail, "clm-a -> clm-b") {
		t.Errorf("fallbacks = %+v, want the defaulted candidate named under its source's document", fallbacks)
	}
	if len(unreadable.asked) != 2 {
		t.Errorf("asked %q, want a drafted entry asked again by a run that can ask, and re-asked once", unreadable.asked)
	}
	if got.outcomes[candidates[0].pair()] != ClassifyDefaulted || !slices.Equal(got.edges, []pair{{"clm-a", "clm-b"}}) {
		t.Errorf("an unoffered letter twice = %+v, want defaulted to the supported-by draft", got)
	}
}

func TestStandingReadsTheRecord(t *testing.T) {
	text := "[↑ V](index.md)\n\n# A\n\nWe show the bound holds for all x.\n\nWe recall the notation used here.\n"
	leaf := &leafReading{document: "v/a.md", text: text}
	leaf.render = renderText(text, nil, excludedLines(text, nil))
	cause := CauseNoLetter
	entry := buildrecords.LeafEntry{Verdicts: []buildrecords.ParagraphVerdict{{Line: 4, Verdict: JudgementClaim}, {Line: 6, Verdict: JudgementDefaulted, Cause: &cause}}}
	for line, want := range map[int]standing{2: standingOutside, 4: standingClaim, 6: standingUnjudged} {
		if got := leaf.standing(Anchor{Line: line}, entry); got != want {
			t.Errorf("standing at line %d = %d, want %d", line, got, want)
		}
	}
	entry.Verdicts[1] = buildrecords.ParagraphVerdict{Line: 6, Verdict: JudgementNotAClaim}
	if got := leaf.standing(Anchor{Line: 6}, entry); got != standingNotAClaim {
		t.Errorf("standing under a not-a-claim verdict = %d", got)
	}
	if got := leaf.standing(Anchor{Line: 4}, buildrecords.LeafEntry{}); got != standingUnjudged {
		t.Errorf("standing with no verdict = %d", got)
	}
}
