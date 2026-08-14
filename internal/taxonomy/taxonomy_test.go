package taxonomy

import (
	"errors"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/log/logtest"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/treeplan"
)

// TestDescentComposesATreePlan: the clean path. Every container answers, the
// fold folds, and the stage's ONE artifact is a tree plan that passes every
// composed post-condition — which it does because Compose ran Check over it,
// not because this test re-states them.
func TestDescentComposesATreePlan(t *testing.T) {
	lg := &logtest.Capture{}
	d := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	if d.Calls() != 5 {
		// root, the folder, and one per document: the question set is
		// mechanical, so its size is an assertion and not an observation.
		t.Fatalf("the descent enumerated %d containers, want 5", d.Calls())
	}
	client := model.NewScriptedMockPerConsult(script(d, perCandidate))
	client.RecordCalls = true

	res, dir, err := runDescent(t, d, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || len(res.Failures) != 0 || !res.DeliveryReady() {
		t.Fatalf("result = %+v, want one clean artifact", res)
	}
	if res.FallbackCount != 0 {
		t.Errorf("a no-fallback seam reported %d fallbacks", res.FallbackCount)
	}

	plan := planIn(t, dir)
	if got := plan.Nodes[0]; got.Kind != treeplan.KindEntryPoint || got.Title != "Fixture knowledge base" {
		t.Errorf("the first node is %+v, want the corpus's own entry-point", got)
	}
	// The model's own titles reached the tree, and the source's structure is
	// what the descent walked: a section per document under a section per
	// folder.
	want := []string{"guide", "Document One", "Document Two", "Introduction"}
	for _, title := range want {
		if !hasTitle(plan, title) {
			t.Errorf("the tree plan holds no node titled %q; titles: %v", title, titlesOf(plan))
		}
	}
	if pages := leafCount(plan); pages != 7 {
		t.Errorf("%d pages, want one per surveyed section (7)", pages)
	}
	if len(indexPaths(plan)) < 4 {
		t.Errorf("sections = %v, want the entry-point and one per container", indexPaths(plan))
	}
	if !lg.Has(t, "info", "verify", "ok") {
		t.Error("the composed tree plan's verification was not recorded")
	}

	// Every container call carried its own numbered candidate list, and the
	// answer is the only thing the model was asked for.
	calls := client.Calls()
	if len(calls) != d.Calls() {
		t.Fatalf("%d calls, want one per container (%d)", len(calls), d.Calls())
	}
	first := calls[0].Request.Messages[0].Content
	for _, entry := range []string{"1. guide", "2. Introduction"} {
		if !strings.Contains(first, entry) {
			t.Errorf("the root call's candidate list is missing %q:\n%s", entry, first)
		}
	}
	// The declared effort is the registration site's, passed through untouched.
	if got := calls[0].Request; !requestThinking(got) {
		t.Error("the call did not carry the declared effort")
	}
}

// TestDescentRetriesMalformedJSON: a response that is not the object is a
// REJECTION, retried once with the reason. Prose where data was asked for is
// the most likely failure of this seam, and it must cost one visible retry
// rather than the job.
func TestDescentRetriesMalformedJSON(t *testing.T) {
	lg := &logtest.Capture{}
	d := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	responses := script(d, perCandidate)
	// The first container answers with prose, then with the object.
	queue := append([]model.Response{{Content: "I would group these by topic.", FinishReason: "stop"}}, responses...)
	client := model.NewScriptedMockPerConsult(queue)
	client.RecordCalls = true

	res, dir, err := runDescent(t, d, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.DeliveryReady() {
		t.Fatalf("result = %+v, want the retry to have recovered the run", res)
	}
	if got := len(planIn(t, dir).Nodes); got == 0 {
		t.Fatal("the composed artifact holds no nodes")
	}
	if !lg.Has(t, "info", "reason", "answer with the JSON object described and nothing else") {
		t.Error("the malformed answer was not recorded as a rejection")
	}
	// The retry carried the mechanical reason: a blind resend would be hoping
	// temperature fixes it.
	retry := client.Calls()[1].Request.Messages[0].Content
	if !strings.Contains(retry, "previous attempt rejected") {
		t.Errorf("the retry carried no corrective note:\n%s", retry)
	}
}

// TestDescentRejectsAPartitionViolation: an entry left out of every group is
// the "silently missing a chapter" failure, refused at the seam with a note
// that names no path and no number.
func TestDescentRejectsAPartitionViolation(t *testing.T) {
	lg := &logtest.Capture{}
	d := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	dropped := func(a *ask) answer {
		out := perCandidate(a)
		return answer{Groups: out.Groups[:1]}
	}
	// The first container drops an entry; every container then answers well.
	queue := append([]model.Response{{Content: mustJSON(dropped(d.asks[0])), FinishReason: "stop"}},
		script(d, perCandidate)...)
	client := model.NewScriptedMockPerConsult(queue)
	client.RecordCalls = true

	res, _, err := runDescent(t, d, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.DeliveryReady() {
		t.Fatalf("result = %+v, want the retry to have recovered the run", res)
	}
	note := client.Calls()[1].Request.Messages[0].Content
	if !strings.Contains(note, "an entry was left out") {
		t.Errorf("the retry did not carry the partition failure:\n%s", note)
	}
	if strings.Contains(note, planUnit) {
		t.Error("the retry note names a store path; a rejection note carries no path")
	}
}

// TestDescentExhaustedFailsTheUnit: this is a NO-FALLBACK seam. Two bad
// answers to one container and the unit fails — no artifact, no partial tree,
// and the job refuses emission. The alternative would be a mechanically
// grouped tree delivered under a quiet degraded count (O-1).
func TestDescentExhaustedFailsTheUnit(t *testing.T) {
	lg := &logtest.Capture{}
	d := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	client := model.NewScriptedMock([]model.Response{{Content: "no.", FinishReason: "stop"}}, nil)

	res, dir, err := runDescent(t, d, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DeliveryReady() {
		t.Fatal("a descent whose container never verified was reported emit-ready")
	}
	// Two failures, and both are the point: the container whose answer never
	// verified after the informed retry, and the artifact its lane was
	// composing — poisoned, so the whole descent is redone rather than
	// half-recorded.
	if len(res.Failures) != 2 {
		t.Fatalf("failures = %+v, want the failed container and the artifact it poisoned", res.Failures)
	}
	var kinds []pipeline.FailureKind
	for _, f := range res.Failures {
		kinds = append(kinds, f.Kind)
	}
	if kinds[0] != pipeline.FailureVerification || kinds[1] != pipeline.FailureUpstream {
		t.Errorf("failure kinds = %v, want verification then upstream", kinds)
	}
	if got := res.Failures[0].ModelAttempts; got != 2 {
		t.Errorf("%d model attempts, want the first attempt plus one informed retry", got)
	}
	if res.Failures[1].Path != planUnit {
		t.Errorf("the poisoned unit is %s, want the artifact the fold owes", res.Failures[1].Path)
	}
	work, err := pipeline.OpenTempWork(dir, log.Discard())
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	if _, err := work.ArtifactStore().Get(planUnit); err == nil {
		t.Error("a tree plan was written for a descent that failed")
	}
}

// TestDescentDefectAborts: a response for a unit this descent does not hold is
// nobody's answer, so it is a kbase defect rather than a rejection — the
// worker aborts instead of retrying a question that was never asked wrong.
func TestDescentDefectAborts(t *testing.T) {
	lg := &logtest.Capture{}
	d := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	if _, err := d.verify("treeplan/nowhere.container", "{}"); !errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Errorf("verify of an unknown unit returned %v, want a verifier defect", err)
	}
}

// TestDescentRefusesASecondRun: a Designer is one run's. The working proposal
// is seeded once and never reset, so a second fold over the same instance
// would append a second copy of every domain to a tree that already holds them.
func TestDescentRefusesASecondRun(t *testing.T) {
	lg := &logtest.Capture{}
	d := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	client := model.NewScriptedMockPerConsult(script(d, perCandidate))
	if _, _, err := runDescent(t, d, client, lg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := d.verify(planUnit, "{}"); !errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Errorf("a spent descent accepted another answer: %v", err)
	}
}

// TestPageGroupOfSeveralEntriesBecomesSeveralPages: a page is a single span of
// a single document, so a group of several entries cannot be one. It becomes
// one page per entry, each keeping the source's own title — §2.7's dissolution
// applied where the titles are still in hand.
func TestPageGroupOfSeveralEntriesBecomesSeveralPages(t *testing.T) {
	lg := &logtest.Capture{}
	docs := []docSpec{{path: "one.md", title: "Only Document", secs: []secSpec{
		{"Alpha", 40}, {"Beta", 40}, {"Gamma", 40}}}}
	d := designerFor(t, lg, testBudgets(), docs...)
	allOnePage := func(a *ask) answer {
		members := make([]int, 0, len(a.cands))
		for i := range a.cands {
			members = append(members, i+1)
		}
		return answer{Groups: []answerGroup{{
			Title: "Everything", Scope: "the whole document", Kind: kindPage, Members: members}}}
	}
	client := model.NewScriptedMockPerConsult(script(d, allOnePage))

	res, dir, err := runDescent(t, d, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.DeliveryReady() {
		t.Fatalf("result = %+v, want a clean run", res)
	}
	plan := planIn(t, dir)
	if got := leafCount(plan); got != 3 {
		t.Errorf("%d pages, want one per entry of the page group", got)
	}
	for _, title := range []string{"Alpha", "Beta", "Gamma"} {
		if !hasTitle(plan, title) {
			t.Errorf("the dissolved page group lost the source title %q; titles: %v", title, titlesOf(plan))
		}
	}
	// Each page carries its OWN scope line. The model wrote one scope for what
	// it thought was one page; repeating it three times would be a routing
	// surface that does not route.
	scopes := map[string]int{}
	for _, n := range plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			scopes[n.Scope]++
		}
	}
	if len(scopes) != 3 {
		t.Errorf("the dissolved pages share scope lines: %v", scopes)
	}
}

// TestAnswersAboveAPageAreNotUsed: the question set is enumerated from the
// SOURCE tree, so a container the answer above it put on a page still gets its
// call. Its answer is discarded and recorded, and the run is otherwise clean —
// the alternative is a lane whose task list depends on answers that have not
// been given.
func TestAnswersAboveAPageAreNotUsed(t *testing.T) {
	lg := &logtest.Capture{}
	docs := []docSpec{
		{path: "one.md", title: "Document One", secs: []secSpec{{"Alpha", 40}, {"Beta", 40}}},
		{path: "two.md", title: "Document Two", secs: []secSpec{{"Gamma", 40}, {"Delta", 40}}},
	}
	d := designerFor(t, lg, testBudgets(), docs...)
	// The root puts both documents on pages, so neither document's own call
	// has anywhere to attach.
	rootPages := func(a *ask) answer {
		if a.c.kind != candFolder {
			return perCandidate(a)
		}
		out := perCandidate(a)
		for i := range out.Groups {
			out.Groups[i].Kind = kindPage
		}
		return out
	}
	client := model.NewScriptedMockPerConsult(script(d, rootPages))

	res, dir, err := runDescent(t, d, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.DeliveryReady() {
		t.Fatalf("result = %+v, want a clean run", res)
	}
	if got := leafCount(planIn(t, dir)); got != 2 {
		t.Errorf("%d pages, want one per document", got)
	}
	if !lg.Has(t, "info", "reason", "a group above it placed this material on a page") {
		t.Error("the unused answers were not recorded")
	}
}

func hasTitle(plan treeplan.TreePlan, title string) bool {
	for _, n := range plan.Nodes {
		if n.Title == title {
			return true
		}
	}
	return false
}

func titlesOf(plan treeplan.TreePlan) []string {
	out := make([]string, 0, len(plan.Nodes))
	for _, n := range plan.Nodes {
		out = append(out, n.Title)
	}
	return out
}

func leafCount(plan treeplan.TreePlan) int {
	n := 0
	for _, node := range plan.Nodes {
		if node.Kind == treeplan.KindLeaf {
			n++
		}
	}
	return n
}

// requestThinking reads the effort off a request as the wire carries it: the
// chat-template kwargs model.DefaultRequest sets from the declared value.
func requestThinking(req model.Request) bool {
	on, _ := req.ChatTemplateKwargs["thinking"].(bool)
	return on
}

// TestDescentIsReusedWholeOnResume: resume is stage-granular here, and both
// halves of that are asserted. A completed descent's artifact is proven, so a
// second run over the same job directory spends no call at all — which is only
// true if a fresh Designer over the same corpus derives the SAME parameter
// digest, since a digest that moved between instances would redo the whole
// descent on every resume and nothing would look wrong.
func TestDescentIsReusedWholeOnResume(t *testing.T) {
	lg := &logtest.Capture{}
	dir := t.TempDir()
	first := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	client := model.NewScriptedMockPerConsult(script(first, perCandidate))
	res, err := runDescentIn(t, dir, first, client, lg)
	if err != nil || !res.DeliveryReady() {
		t.Fatalf("the first run did not complete: %+v (%v)", res, err)
	}

	// A Designer is one run's, so the resume gets a new one — and a mock with
	// no responses at all, so any call it makes is a failure rather than an
	// assertion about a counter.
	second := designerFor(t, lg, testBudgets(), fixtureDocs()...)
	res, err = runDescentIn(t, dir, second, model.NewScriptedMock(nil, nil), lg)
	if err != nil {
		t.Fatalf("the resume failed: %v", err)
	}
	if res.Reused != 1 || res.Produced != 0 || !res.DeliveryReady() {
		t.Fatalf("result = %+v, want the proven tree plan reused whole", res)
	}
}
