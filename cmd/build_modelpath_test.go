package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"kbase/internal/config"
	"kbase/internal/distill"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/treeplan"
)

// The verb's model-in-loop path, driven by a scripted mock through the same
// composition the binary runs: the taxonomy descent designs the tree, the
// level stages write the summaries, and stage 8 renders their conclusions
// into the section pages.
//
// Nothing here is live: the "model" is a canned response script behind
// providerOptions' NewClient seam, so these are hermetic unit tests of the
// live-mode composition — the only place the model stages meet the
// mechanical spine before a real provider does. The real-provider
// counterparts are the test-integration-build-live-* recipes.

// liveCorpus is shaped so every container call presents exactly TWO entries:
// two documents at the root, two top-level sections in each. That is what lets
// one scripted answer be a legal answer to every container without the test
// knowing the descent's internals.
// Every section is written over the content floor (dissect.MinTokens): a
// section under it is merged into its neighbour when the tree plan is composed,
// and this corpus's whole point is that each document presents two of them.
const (
	liveDocOne = `# Alpha

The alpha section of document one, with enough material in it to be a page of
its own rather than a fragment the tree plan folds into the section next door.

# Beta

The beta section of document one, with enough material in it to be a page of
its own rather than a fragment the tree plan folds into the section next door.
`
	liveDocTwo = `# Gamma

The gamma section of document two, with enough material in it to be a page of
its own rather than a fragment the tree plan folds into the section next door.

# Delta

The delta section of document two, with enough material in it to be a page of
its own rather than a fragment the tree plan folds into the section next door.
`
)

func writeLiveCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{"one.md": liveDocOne, "two.md": liveDocTwo} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// The two answer grammars, written the way a model writes them: one `group`
// line per group, and three labelled text blocks.
//
// They are spelled out here rather than derived from the parsers' own constants
// on purpose. This is the end-to-end test of the model path, so what it must
// send is what a model sends — a fixture built from the reader's constants would
// agree with the reader by construction, which is the one thing this test is
// not allowed to assume.
type scriptedGroup struct {
	title, scope, kind string
	members            []int
}

func groupingAnswer(groups ...scriptedGroup) string {
	var b strings.Builder
	for _, g := range groups {
		nums := make([]string, 0, len(g.members))
		for _, m := range g.members {
			nums = append(nums, strconv.Itoa(m))
		}
		fmt.Fprintf(&b, "group :: %s :: %s :: %s :: %s\n",
			g.title, g.scope, g.kind, strings.Join(nums, ", "))
	}
	return b.String()
}

func summaryAnswer(framing, heading, conclusions string) string {
	return "FRAMING:\n" + framing + "\n\nHEADING:\n" + heading + "\n\nCONCLUSIONS:\n" + conclusions + "\n"
}

// liveScript is the scripted conversation: three container answers, then a
// summary for every section node. The taxonomy calls are one serial lane and
// come first, in order; the summaries follow, and are identical because their
// lanes fan out across workers and the order between two domains is not one.
func liveScript(t *testing.T) []model.Response {
	t.Helper()
	group := func(title string) string {
		return groupingAnswer(scriptedGroup{
			title: title, scope: "what a reader finds in " + title,
			kind: "section", members: []int{1, 2},
		})
	}
	summary := summaryAnswer("This section holds the material below it.",
		"What it settles", "Alpha and Beta are documented here.")

	out := []model.Response{
		{Content: group("The Corpus"), FinishReason: "stop"},
		{Content: group("Document One"), FinishReason: "stop"},
		{Content: group("Document Two"), FinishReason: "stop"},
	}
	// Four section nodes get a summary: the entry-point, the domain the root's
	// group created, and the two the documents' groups created.
	for range 4 {
		out = append(out, model.Response{Content: summary, FinishReason: "stop"})
	}
	return out
}

func TestBuildLiveWritesTaxonomyAndSummaries(t *testing.T) {
	root := writeLiveCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	client := model.NewScriptedMockPerConsult(liveScript(t))

	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root: root,
		Out:  out,
		Live: &providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config: config.Config{
				Provider: "solo",
				Models:   config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"},
			},
			NewClient: func(model.Endpoint) model.Client { return client },
			Stderr:    &stderr,
		},
		BuildDate: pinnedBuildDate,
		// Kept, because the run record this test reads lives at the temp-work
		// root and an ordinary run tears it down with everything else there.
		KeepTempWork: true,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Logger:       log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the live build did not deliver:\n%s", stdout.String())
	}

	// The tree is the model's, not the source's: the descent's own titles are
	// on the nodes, and no mechanical proposal produces them.
	for _, title := range []string{"The Corpus", "Document One", "Document Two"} {
		found := false
		for _, n := range res.Plan.Nodes {
			if n.Title == title {
				found = true
			}
		}
		if !found {
			t.Errorf("the tree plan holds no node the descent titled %q", title)
		}
	}

	// The conclusions block reached the delivered page, under the model's own
	// heading and above the down-link list.
	var section string
	for _, n := range res.Plan.Nodes {
		if n.Kind == treeplan.KindIndex {
			section = n.Path
			break
		}
	}
	page, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(section)))
	if err != nil {
		t.Fatalf("read the delivered section %s: %v", section, err)
	}
	body := string(page)
	for _, want := range []string{
		"This section holds the material below it.",
		"## What it settles",
		"Alpha and Beta are documented here.",
		"## Derivations and Detail",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the delivered section is missing %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "## What it settles") > strings.Index(body, "## Derivations and Detail") {
		t.Errorf("the conclusions block renders below the down-link list:\n%s", body)
	}

	// The run record carries what a live run dialed, asked and spent — and no
	// credential.
	data, err := os.ReadFile(recordPath(out))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live == nil {
		t.Fatal("the run record carries no live block")
	}
	if rec.Live.Model != "gemma-4-31b" || rec.Live.Tier != config.TierHeavy {
		t.Errorf("live = %+v, want the heavy tier's configured model", rec.Live)
	}
	if rec.Live.TaxonomyCalls != 3 {
		t.Errorf("%d taxonomy calls recorded, want one per container", rec.Live.TaxonomyCalls)
	}
	if rec.Summaries != 4 {
		t.Errorf("%d summaries, want one per section node", rec.Summaries)
	}
	if strings.Contains(string(data), testAPIKey) {
		t.Error("the run record carries the API key")
	}
}

// liveDocThree is the third document of the mixed corpus: one section, so the
// descent enumerates it as a plain candidate the root's answer can place on a
// page of its own. Its body is the string the assertions below hunt for — a raw
// page body is either in a prompt or it is not.
const liveDocThree = `# Epsilon

The epsilon section of document three, which the root's answer places on a page of
its own rather than under a section of the knowledge base — so the entry-point
holds a page beside a section, which is the shape the leaf-group card exists for.
`

// writeMixedCorpus writes a corpus the descent can shape into a MIXED
// entry-point: two multi-section documents to become sections, and one
// single-section document to become a page of the root's own.
//
// The names are ordinal-prefixed because the root's candidate list is the
// corpus's PATH order, and the scripted answer below names its members by
// number: `3` has to be the single-section document rather than whichever
// filename happens to sort third.
func writeMixedCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{
		"1-one.md": liveDocOne, "2-two.md": liveDocTwo, "3-three.md": liveDocThree,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// TestBuildLiveSummarisesSiblingLeavesAsAGroup is the B-5 scheme through the
// verb's own composition: a node holding both pages and sections costs two
// summary calls, and the second one reads no page body.
//
// The assertions are on the prompts the mock RECEIVED, because that is where a
// stage's input composition is observable: the artifacts record what came back,
// and every scripted answer here is the same. What the delivered tree shows is
// only that a summary arrived.
func TestBuildLiveSummarisesSiblingLeavesAsAGroup(t *testing.T) {
	root := writeMixedCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	// The root's answer: document three on a page of its own, the other two under
	// one section. Every other container gets one section over everything it
	// holds.
	rootGroups := groupingAnswer(
		scriptedGroup{title: "Document Three", scope: "what a reader finds in document three",
			kind: "page", members: []int{3}},
		scriptedGroup{title: "The Corpus", scope: "what a reader finds in the corpus",
			kind: "section", members: []int{1, 2}},
	)
	group := groupingAnswer(scriptedGroup{
		title: "A Section", scope: "what a reader finds here",
		kind: "section", members: []int{1, 2},
	})
	summary := summaryAnswer("This section holds the material below it.",
		"What it settles", "The material is documented here.")
	client := newStageFake(group, "1", summary)
	client.root = rootGroups

	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root: root,
		Out:  out,
		Live: &providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config: config.Config{
				Provider: "solo",
				Models:   config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"},
			},
			NewClient: func(model.Endpoint) model.Client { return client },
			Stderr:    &stderr,
		},
		BuildDate:    pinnedBuildDate,
		KeepTempWork: true,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Logger:       log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the live build did not deliver:\n%s", stdout.String())
	}

	// The tree is the shape the scheme is about, or the rest of this test proves
	// nothing: the entry-point holds a page AND a section.
	var entry string
	for _, n := range res.Plan.Nodes {
		if n.Kind == treeplan.KindEntryPoint {
			entry = n.Path
		}
	}
	var pages, sections int
	for _, n := range res.Plan.Nodes {
		if n.Parent != entry {
			continue
		}
		if n.Kind == treeplan.KindLeaf {
			pages++
			continue
		}
		sections++
	}
	if pages != 1 || sections != 1 {
		t.Fatalf("the entry-point holds %d pages and %d sections; this test needs one of each", pages, sections)
	}

	// Of the entry-point's TWO summary calls, one reads its page and one reads
	// only summary-class material — the leaf-group card and its section's summary.
	// Nothing reads both, which is the whole of the ruling.
	const (
		wholeKB   = "This is the whole knowledge base"
		pageBytes = "The epsilon section of document three"
	)
	var groupCalls, blendCalls int
	for _, p := range client.seenPrompts(stageSummaries) {
		if !strings.Contains(p, wholeKB) {
			continue
		}
		if strings.Contains(p, pageBytes) {
			groupCalls++
			continue
		}
		blendCalls++
		if !strings.Contains(p, "This section holds the material below it.") {
			t.Errorf("the entry-point's blended call read no summary at all:\n%s", p)
		}
	}
	if groupCalls != 1 || blendCalls != 1 {
		t.Errorf("the entry-point made %d group calls and %d blended calls, want one of each",
			groupCalls, blendCalls)
	}

	// The call count, stated: one per section node plus one card per mixed node.
	// Four sections here — the entry-point, The Corpus, and one per document —
	// and one card.
	if _, calls := client.seen(stageSummaries); calls != 5 {
		t.Errorf("%d summary calls, want four sections plus the entry-point's card", calls)
	}
	// And the card is not a section's summary: the run record counts what the
	// delivered tree renders, which the card never reaches.
	data, err := os.ReadFile(recordPath(out))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Summaries != 4 {
		t.Errorf("%d summaries recorded, want one per section node and none for the card", rec.Summaries)
	}
}

// descendDoc is the shape rule R-C descends into: a top-level section over the
// leaf budget that carries subsections of its own, plus a small second section
// so the document container has more than one entry to group.
//
// The sizes are what make it the rule's own case rather than an approximation:
// the `# Alpha` subtree runs past the 4000-token leaf budget while each of its
// two subsections stays comfortably under it, so Alpha descends and nothing
// below it does. Its lead-in prose is over the content floor as well, so the
// body page survives as a page instead of merging forward into its first
// subsection — which is the page this fixture is here to see delivered.
func descendDoc() string {
	var sb strings.Builder
	para := func(n int) {
		for range n {
			sb.WriteString(splitPara + "\n\n")
		}
	}
	sb.WriteString("# Alpha\n\n")
	para(4)
	sb.WriteString("## Alpha One\n\n")
	para(16)
	sb.WriteString("## Alpha Two\n\n")
	para(16)
	sb.WriteString("# Beta\n\nThe Beta section, which is short — one paragraph of material, " +
		"which is all it takes to be a page rather than a fragment folded into the section above it.\n")
	return sb.String()
}

// TestBuildLiveDescendsIntoAnOversizedSection is section descent through the
// verb's whole composition: the taxonomy stage asks about a SECTION container,
// its subsections become pages of their own, and the mechanical splitter is
// never reached.
//
// It is the end-to-end counterpart of treeplan's Skeleton unit table. What that
// one proves is which sections the rule calls containers; what this one proves
// is that the shipped stage-3 seam asks about them — the candidate list, the
// answer, the fold, the composed artifact and the delivered tree, over a
// corpus whose only oversized section has the author's own headings inside it.
//
// The claim it makes against the status quo ante is the one the design opened
// with: this corpus used to deliver `# Alpha` as a mechanically halved page
// pair with no title of its own. It now delivers a subtopic index over three
// pages the author named.
func TestBuildLiveDescendsIntoAnOversizedSection(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "one.md"), []byte(descendDoc()), 0o600); err != nil {
		t.Fatalf("write the corpus document: %v", err)
	}
	out := filepath.Join(t.TempDir(), "kb")

	// The document's own call: two entries, the descended section and the small
	// one, under one heading. Alpha carries entries of its own, so its groups
	// attach below this node when its turn comes.
	docGroup := groupingAnswer(scriptedGroup{
		title: "The Document", scope: "what a reader finds in the document",
		kind: "section", members: []int{1, 2},
	})
	// Alpha's own call: three entries — its body and its two subsections, in
	// that order — grouped under a subtopic index. This is the question that
	// did not exist before descent.
	sectionGroup := groupingAnswer(scriptedGroup{
		title: "Alpha Sections", scope: "what a reader finds under Alpha",
		kind: "section", members: []int{1, 2, 3},
	})
	summary := summaryAnswer("This section holds the material below it.",
		"What it settles", "The material is documented here.")
	client := newStageFake(docGroup, "1", summary)
	client.section = sectionGroup

	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root: dir,
		Out:  out,
		Live: &providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config: config.Config{
				Provider: "solo",
				Models:   config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"},
			},
			NewClient: func(model.Endpoint) model.Client { return client },
			Stderr:    &stderr,
		},
		BuildDate:    pinnedBuildDate,
		KeepTempWork: true,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Logger:       log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the descended build did not deliver:\n%s", stdout.String())
	}

	// Nothing was cut mechanically. The oversized section was going to be split
	// either way; descent is that split made semantic, so the splitter has
	// nothing left to do here (R-1/R-C).
	for _, g := range res.Plan.Groups {
		if g.Parts > 1 {
			t.Errorf("group %s split into %d parts; the author's own headings were the cut", g.ID, g.Parts)
		}
	}

	// The three pages Alpha's own call created: its body under the preamble
	// naming convention (R-2), and one per subsection.
	byTitle := map[string]treeplan.Node{}
	for _, n := range res.Plan.Nodes {
		byTitle[n.Title] = n
	}
	for _, want := range []string{"Introduction to Alpha", "Alpha One", "Alpha Two"} {
		n, ok := byTitle[want]
		if !ok {
			t.Fatalf("the tree plan holds no node titled %q; the descent did not reach Alpha's entries", want)
		}
		if n.Kind != treeplan.KindLeaf {
			t.Errorf("%q is a %s, want a page", want, n.Kind)
		}
	}

	// The body page carries the container's OWN heading line, which is the
	// deliberate carve-out the body span was defined around: no byte of the
	// source reaches no page.
	body, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(byTitle["Introduction to Alpha"].Path)))
	if err != nil {
		t.Fatalf("read the body page: %v", err)
	}
	if !strings.Contains(string(body), "\n# Alpha\n") {
		t.Errorf("the body page does not carry its container's heading line:\n%s", body)
	}
	one, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(byTitle["Alpha One"].Path)))
	if err != nil {
		t.Fatalf("read the subsection page: %v", err)
	}
	if !strings.Contains(string(one), "\n## Alpha One\n") {
		t.Errorf("the subsection page does not open on its own subsection:\n%s", one)
	}

	// The subtopic level, which is what ARCHITECTURE §1's "entry-point → domain
	// index → subtopic index → leaf" claims and what this corpus could not have
	// before: Alpha's pages sit under an index whose own parent is an index.
	parent := byTitle["Alpha One"].Parent
	node, ok := res.Plan.Node(parent)
	if !ok || node.Kind != treeplan.KindIndex {
		t.Fatalf("%q's parent %q is not an index", "Alpha One", parent)
	}
	if grand, ok := res.Plan.Node(node.Parent); !ok || grand.Kind != treeplan.KindIndex {
		t.Fatalf("%q sits directly under the entry-point; the descent added no subtopic level", parent)
	}

	// And the depth it reached is stated where nothing else states it: no gate
	// adjudicates depth (R-3), so the run record is the measurement.
	data, err := os.ReadFile(recordPath(out))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.MaxDepth != 4 {
		t.Errorf("maxDepth = %d, want 4: entry-point, the document's section, Alpha's, and its pages", rec.MaxDepth)
	}
	if rec.Live == nil || rec.Live.TaxonomyCalls != 2 {
		t.Errorf("live = %+v, want two container calls — the document's and Alpha's", rec.Live)
	}
}

// The refined cuts stage, end to end.
//
// Rojo has no split group at the shipped budget and neither does the corpus
// above, which is the ordinary outcome over documentation-sized sections — so
// the only way to exercise stage 4's live shape is a corpus built to have
// some. writeSplitCorpus is that: two documents whose first section is
// comfortably over the leaf budget, so stage 3 sizes both their groups into
// parts and stage 4 has real interior boundaries to adjudicate in two lanes.

// splitPara is one paragraph of the oversized section: enough bytes that a
// handful of them cross the leaf budget, and a paragraph break after each, so
// the splitter has candidates to choose between and the menu has entries
// either side of every incumbent.
const splitPara = "The reconciler walks the project file and the live tree together, comparing " +
	"each declared property against the one the tree currently holds and applying only " +
	"the differences. A property the file does not name is left exactly as it was, which " +
	"is what makes a partial project file a legal one. Where a difference cannot be " +
	"applied the whole reconciliation is reported and abandoned rather than half-performed, " +
	"because a half-applied tree is harder to diagnose than one that never moved at all. "

// splitDoc is a document whose first top-level section runs well past the leaf
// budget, followed by a small one — so a page covering that section whole is
// sized into several parts and the boundaries between them are stage 4's work.
//
// The long section carries subsections of its own, one every few paragraphs,
// and under section descent they are what makes this fixture work at all.
// Descent would turn that section into a container and its subsections into
// pages — the split that has to happen anyway, made semantic — so the scripted
// answers below deliberately put the WHOLE container on one page instead
// (buildSplit). That is a legal answer and its consequence is ruled: the model
// never rules on size, so an over-budget page from any answer is split
// mechanically, uniformly (R-5, ruled 2026-08-17). The subsections are then
// also what a delivered part's descriptor is drawn from, so a corpus without
// them could not show two sibling bullets being told apart [MAD2: B-4].
func splitDoc(first, second string, paras int) string {
	var sb strings.Builder
	sb.WriteString("# " + first + "\n\n")
	for i := range paras {
		if i%10 == 0 {
			fmt.Fprintf(&sb, "## %s stage %d\n\n", first, i/10+1)
		}
		sb.WriteString(splitPara + "\n\n")
	}
	// Short, but over the content floor: a section under it is merged into the
	// oversized one above and the document stops having two sections at all.
	sb.WriteString("# " + second + "\n\nThe " + second + " section, which is short — one paragraph " +
		"of material, which is all it takes to be a page rather than a fragment folded into " +
		"the long section above it.\n")
	return sb.String()
}

// writeSplitCorpus writes TWO oversized documents, so the job has two split
// groups and stage 4 has two folds running in two lanes. One would exercise
// the fold; two are what exercise the stage — a response has to reach the fold
// that owns its span and no other.
func writeSplitCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{
		"one.md": splitDoc("Alpha", "Beta", 100),
		"two.md": splitDoc("Gamma", "Delta", 60),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// stageFake answers each stage's question by RECOGNISING it rather than by
// counting calls.
//
// A live build asks three different things — group these entries, choose this
// boundary, summarise this section — and the summary stages fan their lanes
// out across workers, so a positional script would encode a call order the
// pipeline never promised. Recognition is the one thing the mock fabric
// cannot do (it walks a queue or re-serves one slot); everything below it,
// the SSE chunking ConsultDrained reads through included, is model's own.
type stageFake struct {
	grouping string
	boundary string
	summary  string

	// root, when set, answers the CORPUS ROOT's container call where grouping
	// answers every other container. It exists for the one shape a single
	// grouping answer cannot produce: a root that holds a page of its own beside
	// a section, which is the mixed container the leaf-group card is for.
	root string

	// section, when set, answers a SECTION container's call — a section the
	// source's own headings descend into (treeplan.Skeleton), whose entries are
	// its body and its subsections. It is a separate slot for the same reason
	// root is: a document's call and its descended section's call present
	// different numbers of entries, and one canned partition cannot be a legal
	// answer to both.
	section string

	mu sync.Mutex
	// models is the model id each kind of call went out under — the proof
	// that refinement resolved the LIGHT tier while the other two resolved
	// the heavy one.
	models map[string]string
	counts map[string]int
	// prompts is every prompt of each kind, as it went out. A stage's input
	// composition is only observable here: the artifacts record what came back.
	prompts map[string][]string
}

func newStageFake(grouping, boundary, summary string) *stageFake {
	return &stageFake{
		grouping: grouping, boundary: boundary, summary: summary,
		models: map[string]string{}, counts: map[string]int{}, prompts: map[string][]string{},
	}
}

// answer classifies one request and returns what to say back. The markers are
// each stage's own rendered material: the refinement menu, and the summary
// definition's instruction.
func (f *stageFake) answer(req model.Request) (string, string) {
	var sb strings.Builder
	for _, m := range req.Messages {
		sb.WriteString(m.Content)
	}
	prompt := sb.String()
	switch {
	case strings.Contains(prompt, "Candidate positions:"):
		return stageCuts, f.boundary
	case strings.Contains(prompt, "Write that section's own summary"):
		return stageSummaries, f.summary
	case f.root != "" && strings.Contains(prompt, "This folder is called"):
		return stageTreePlan, f.root
	case f.section != "" && strings.Contains(prompt, "This section is called"):
		return stageTreePlan, f.section
	default:
		return stageTreePlan, f.grouping
	}
}

func (f *stageFake) Consult(ctx context.Context, req model.Request) (model.Response, error) {
	if err := ctx.Err(); err != nil {
		return model.Response{}, err
	}
	kind, content := f.answer(req)
	var sb strings.Builder
	for _, m := range req.Messages {
		sb.WriteString(m.Content)
	}
	f.mu.Lock()
	f.models[kind] = req.Model
	f.counts[kind]++
	f.prompts[kind] = append(f.prompts[kind], sb.String())
	f.mu.Unlock()
	return model.Response{Content: content, FinishReason: "stop"}, nil
}

func (f *stageFake) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	resp, err := f.Consult(ctx, req)
	if err != nil {
		return nil, err
	}
	// The real chunking, so the drain this pipeline actually performs is the
	// one under test rather than a second implementation of it.
	return model.NewScriptedMock([]model.Response{resp}, nil).ConsultStream(ctx, model.Request{})
}

func (f *stageFake) ListModels(context.Context) ([]model.ModelInfo, error) {
	return []model.ModelInfo{}, nil
}

func (f *stageFake) seen(kind string) (string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.models[kind], f.counts[kind]
}

func (f *stageFake) seenPrompts(kind string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.prompts[kind]...)
}

// buildSplit runs a live build over the split corpus with the given boundary
// answer and returns the result, the run record, the console output and the
// directory the tree was delivered into.
func buildSplit(t *testing.T, boundary string) (buildResult, buildRun, string, string) {
	t.Helper()
	root := writeSplitCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	// The corpus root's answer: one section over the two documents, so each
	// document's own call has a node to attach its groups to.
	rootGroup := groupingAnswer(scriptedGroup{
		title: "The Documents", scope: "what a reader finds in the documents",
		kind: "section", members: []int{1, 2},
	})
	// Each DOCUMENT's answer: both of its entries on pages. The first entry is
	// an oversized section the source's own headings descend into, so this is
	// the answer that hands the splitter a real span to cut — one page over the
	// whole container, mechanically split because it is over budget (R-5), and
	// the container's own call is then never made. A `section` answer here
	// would descend instead and there would be no boundary in the job at all,
	// which is the point of the ruling: the split path is what happens to an
	// over-budget page, whoever named it.
	group := groupingAnswer(scriptedGroup{
		title: "The Material", scope: "what a reader finds here",
		kind: "page", members: []int{1, 2},
	})
	summary := summaryAnswer("This section holds the material below it.",
		"What it settles", "The reconciler is documented here.")
	client := newStageFake(group, boundary, summary)
	client.root = rootGroup

	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root: root,
		Out:  out,
		Live: &providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config: config.Config{
				Provider: "solo",
				Models:   config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"},
			},
			NewClient: func(model.Endpoint) model.Client { return client },
			Stderr:    &stderr,
		},
		BuildDate: pinnedBuildDate,
		// Kept, because the run record this test reads lives at the temp-work
		// root and an ordinary run tears it down with everything else there.
		KeepTempWork: true,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Logger:       log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the live build did not deliver:\n%s", stdout.String())
	}

	// The light tier answered the boundary calls and the heavy one everything
	// else. Nothing else in this test could tell a tier mix-up from a working
	// build: both models are the same mock.
	if id, calls := client.seen(stageCuts); id != "gemma-4-26b-a4b" || calls == 0 {
		t.Errorf("the boundary calls went to %q over %d calls, want the light tier's model", id, calls)
	}
	if id, _ := client.seen(stageTreePlan); id != "gemma-4-31b" {
		t.Errorf("the taxonomy calls went to %q, want the heavy tier's model", id)
	}

	data, err := os.ReadFile(recordPath(out))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live == nil {
		t.Fatal("the run record carries no live block")
	}
	// Two split groups and more than one boundary in the job: a single-fold,
	// single-boundary run would pass every assertion below while proving
	// nothing about the routing between folds or the serial scan inside one.
	if rec.SplitGroups < 2 || rec.Live.BoundariesAdjudicated < 2 {
		t.Fatalf("the corpus produced %d split groups and %d boundaries; the stage under test needs several of each",
			rec.SplitGroups, rec.Live.BoundariesAdjudicated)
	}
	return res, rec, stdout.String(), out
}

// TestBuildLiveRefinesCuts: the model's choice is what the delivered pages are
// cut at.
//
// Answering "1" takes the earliest candidate the menu offers, which is behind
// the mechanical cut in every window with a candidate to spare — so a run that
// moved every boundary is a run whose answers were mapped back to offsets,
// verified against the working list, and composed, rather than one that fell
// through to the fallback and looked the same from outside.
func TestBuildLiveRefinesCuts(t *testing.T) {
	res, rec, stdout, _ := buildSplit(t, "1")

	if rec.Live.BoundariesFellBack != 0 || rec.Live.BoundaryRejections != 0 {
		t.Errorf("live = %+v, want every boundary adjudicated cleanly:\n%s", rec.Live, stdout)
	}
	if rec.Live.BoundariesMoved != rec.Live.BoundariesAdjudicated {
		t.Errorf("%d of %d boundaries moved; the model's choice was not taken",
			rec.Live.BoundariesMoved, rec.Live.BoundariesAdjudicated)
	}
	if rec.Live.LightModel != "gemma-4-26b-a4b" {
		t.Errorf("lightModel = %q, want the configured light tier", rec.Live.LightModel)
	}

	// The split groups' parts are on disk as separate pages, and each fold
	// adjudicated one boundary fewer than its group has parts.
	split := map[string]bool{}
	for _, g := range res.Plan.Groups {
		if g.Parts > 1 {
			split[g.ID] = true
		}
	}
	parts := 0
	for _, n := range res.Plan.Nodes {
		if n.Kind == treeplan.KindLeaf && split[n.SplitGroup] {
			parts++
		}
	}
	if want := rec.Live.BoundariesAdjudicated + rec.SplitGroups; parts != want {
		t.Errorf("%d split-group pages against %d boundaries over %d groups, want %d",
			parts, rec.Live.BoundariesAdjudicated, rec.SplitGroups, want)
	}
}

// TestBuildDeliversSplitPartNavigation: a reader who lands on one part of a
// split document can walk the series and can choose between its bullets.
//
// Everything asserted here is MECHANICAL — the edges are a projection of
// groups[].parts and the descriptors are read off the cut list — but it is
// asserted over a tree whose boundaries the model chose, because that is the
// state the delivered artifact is actually in [MAD2: B-4, B-8]. Where the
// boundaries fell is the model's; that the pages point at each other is not.
func TestBuildDeliversSplitPartNavigation(t *testing.T) {
	res, _, _, out := buildSplit(t, "1")

	// The parts of each split group, in part order, and the index above them.
	families := map[string][]treeplan.Node{}
	for _, n := range res.Plan.Nodes {
		if n.Kind != treeplan.KindLeaf {
			continue
		}
		if g, ok := res.Plan.SplitGroup(n.SplitGroup); ok && g.Parts > 1 {
			families[g.ID] = append(families[g.ID], n)
		}
	}
	if len(families) < 2 {
		t.Fatalf("the corpus produced %d split families; this test needs the corpus's two", len(families))
	}

	read := func(p string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("read the delivered page %s: %v", p, err)
		}
		return string(data)
	}

	// One family of three or more is what puts a middle part — the only one
	// carrying both edges — under test at all.
	longest := 0
	for _, parts := range families {
		longest = max(longest, len(parts))
	}
	if longest < 3 {
		t.Fatalf("the largest split family has %d parts; the first/middle/last asymmetry needs three", longest)
	}

	for _, parts := range families {
		for i, n := range parts {
			page := read(n.Path)
			_, body, ok := distill.ParseFrontmatter([]byte(page))
			if !ok {
				t.Fatalf("%s does not open with its frontmatter block:\n%s", n.Path, page)
			}
			var want []string
			want = append(want, distill.UpLink(n.Path, n.Parent))
			if i > 0 {
				want = append(want, distill.PrevLink(n.Path, parts[i-1].Path))
			}
			if i < len(parts)-1 {
				want = append(want, distill.NextLink(n.Path, parts[i+1].Path))
			}
			var got []string
			for _, l := range distill.NavBlock(body) {
				got = append(got, "["+l.Marker+" "+l.Label+"]("+l.Target+")")
			}
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("%s opens with:\n%s\nwant:\n%s", n.Path, strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			// Every part states the identity it was routed by, whatever the
			// slice happens to open with [MAD2: B-8].
			if h := "\n# " + n.Title + "\n"; !strings.Contains(page, h) {
				t.Errorf("%s carries no %q:\n%s", n.Path, strings.TrimSpace(h), page)
			}
		}

		// The index's bullets are choosable: one line per part, each carrying
		// its own post-cut descriptor, and no two of them identical.
		index := read(parts[0].Parent)
		seen := map[string]bool{}
		for _, n := range parts {
			bullet := ""
			for _, l := range strings.Split(index, "\n") {
				if strings.HasPrefix(l, "- [") && strings.Contains(l, "]("+path.Base(n.Path)+")") {
					bullet = l
				}
			}
			if bullet == "" {
				t.Fatalf("%s lists no bullet for %s:\n%s", parts[0].Parent, n.Path, index)
			}
			if !strings.Contains(bullet, "(this part: ") {
				t.Errorf("the bullet for %s carries no descriptor: %q", n.Path, bullet)
			}
			if seen[bullet] {
				t.Errorf("two parts share the bullet %q — the index hands the reader a coin", bullet)
			}
			seen[bullet] = true
		}
	}
}

// TestBuildLiveFallsBackOnRefusedBoundaries: refinement is a fallback-backed
// seam, so a model that never answers the question costs quality and never the
// delivery.
//
// Every boundary here is answered with prose rather than a menu number, which
// parseChoice rejects; the informed retry gets the same, and the boundary then
// keeps the mechanical cut. The gates still pass and the tree is still
// delivered — which is the whole claim ARCHITECTURE §3's monotone-safety rule
// makes about this seam.
func TestBuildLiveFallsBackOnRefusedBoundaries(t *testing.T) {
	_, rec, stdout, _ := buildSplit(t, "I would put the boundary a little later")

	if rec.Live.BoundariesFellBack != rec.Live.BoundariesAdjudicated {
		t.Errorf("%d of %d boundaries fell back; an answer that is not a menu number cannot be taken:\n%s",
			rec.Live.BoundariesFellBack, rec.Live.BoundariesAdjudicated, stdout)
	}
	if rec.Live.BoundariesMoved != 0 {
		t.Errorf("%d boundaries moved on answers nothing verified", rec.Live.BoundariesMoved)
	}
	// Two model attempts per boundary, both rejected: the first and the one
	// informed retry (pipeline's declared policy, dissect.Retry).
	if want := 2 * rec.Live.BoundariesAdjudicated; rec.Live.BoundaryRejections != want {
		t.Errorf("%d rejections over %d boundaries, want %d — one per attempt",
			rec.Live.BoundaryRejections, rec.Live.BoundariesAdjudicated, want)
	}
}

// TestBuildRefusesAnUnmappedTier: a live build needs BOTH tiers, and says
// which one is missing before it reads the corpus.
//
// Both are essential to a delivered tree in different ways — the heavy tier
// designs it and summarises it, the light tier decides where its pages are cut
// — so an unmapped one is refused at the same moment for the same reason: a
// misconfigured run must cost no tokens, and half a build is not a build. The
// client factory fails the test if it is ever reached, which is what makes
// "before anything is dialed" an assertion rather than a claim.
func TestBuildRefusesAnUnmappedTier(t *testing.T) {
	for _, tc := range []struct {
		name   string
		models config.ModelMap
		want   string
	}{
		{"no light tier", config.ModelMap{Heavy: "gemma-4-31b"}, config.TierLight},
		{"no heavy tier", config.ModelMap{Light: "gemma-4-26b-a4b"}, config.TierHeavy},
		{"neither tier", config.ModelMap{}, config.TierHeavy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, err := runBuild(context.Background(), buildOptions{
				Root: writeLiveCorpus(t),
				Out:  filepath.Join(t.TempDir(), "kb"),
				Live: &providerOptions{
					Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
					Config:    config.Config{Provider: "solo", Models: tc.models},
					NewClient: func(model.Endpoint) model.Client {
						t.Error("a build with an unmapped tier dialed a provider")
						return model.NewScriptedMock(nil, nil)
					},
					Stderr: &stderr,
				},
				BuildDate: pinnedBuildDate,
				Stdout:    &stdout,
				Stderr:    &stderr,
				Logger:    log.Discard(),
			})
			if err == nil {
				t.Fatal("a live build with an unmapped tier must refuse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to name the %s tier", err, tc.want)
			}
		})
	}
}

// TestBuildWithoutConfigDirMakesNoCall is the other half of the switch: the
// mechanical shape must stay mechanical. A client that fails on every call
// proves it — the build succeeds because nothing ever asks it anything.
func TestBuildWithoutConfigDirMakesNoCall(t *testing.T) {
	root := writeLiveCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	// Kept, because the run record this test reads lives at the temp-work root.
	res, stdout, _ := build(t, root, out, true)
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the mechanical build did not deliver:\n%s", stdout)
	}
	data, err := os.ReadFile(recordPath(out))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live != nil {
		t.Errorf("a run with no model in the loop reported %+v", rec.Live)
	}
	if rec.Summaries != 0 {
		t.Errorf("%d summaries were written with no model in the loop", rec.Summaries)
	}
}
