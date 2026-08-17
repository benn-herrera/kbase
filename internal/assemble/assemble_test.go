package assemble

import (
	"path"
	"strings"
	"testing"

	"kbase/internal/distill"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// The fixture is a two-document corpus assembled the way the pipeline
// assembles one: stage 5 renders the pages, stage 8 renders the indexes and
// the fixtures, and stage 9 runs over the result.
//
// It is hand-authored rather than driven through the Markdown adapter because
// this package is downstream of the survey artifact and its tests are too (the
// format seam, ARCHITECTURE §4). Every check below then mutates one thing
// about that tree and names the gate that must catch it.

// Every span here is written over the content floor the tree plan enforces
// (dissect.MinTokens): a preamble under it is merged into the section below it
// rather than becoming a page, and this fixture needs both of each document's
// spans to be pages of their own.
const (
	filler = "\nThis paragraph carries the span it sits in over the minimum amount of " +
		"material a delivered page may hold, so the fixture keeps its shape: two " +
		"documents, a preamble and a section each, four pages under two indexes.\n"

	onePreamble = "The first document.\n" + filler + "\n"
	oneSection  = "# First\n\nFirst body.\n" + filler
	twoPreamble = "The second document.\n" + filler + "\n"
	twoSection  = "# Second\n\nSecond body.\n" + filler

	oneBody = onePreamble + oneSection
	twoBody = twoPreamble + twoSection
	oneHead = len(onePreamble)
	twoHead = len(twoPreamble)
)

type scene struct {
	plan     treeplan.TreePlan
	renderer *Renderer
	verifier *treeplan.Verifier
	leaves   map[string]distill.Leaf
	files    map[string][]byte
	est      tokens.Estimator
}

func newScene(t *testing.T) scene {
	t.Helper()
	corpus, err := ingest.New([]ingest.SourceDoc{
		{Path: "one.md", Bytes: []byte(oneBody)},
		{Path: "two.md", Bytes: []byte(twoBody)},
	})
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	est := tokens.Estimator{}
	files := []survey.File{
		docFile(corpus, 0, "one.md", oneBody, oneHead, "First"),
		docFile(corpus, 1, "two.md", twoBody, twoHead, "Second"),
	}
	art, err := survey.Assemble(corpus, files, log.Discard())
	if err != nil {
		t.Fatalf("survey.Assemble: %v", err)
	}
	proposal, err := treeplan.SourceStructureProposal(art, "Test Corpus", "a two-document corpus", nil)
	if err != nil {
		t.Fatalf("SourceStructureProposal: %v", err)
	}
	v, err := treeplan.NewVerifier(art, corpus, treeplan.DefaultParams())
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	plan, err := v.Compose(proposal, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	prov := distill.Provenance{CorpusHash: art.Corpus.ContentHash, BuildDate: "2026-08-13"}
	d, err := distill.New(plan, art, corpus, nil, prov)
	if err != nil {
		t.Fatalf("distill.New: %v", err)
	}
	descriptors, err := d.Descriptors()
	if err != nil {
		t.Fatalf("Descriptors: %v", err)
	}
	r, err := NewRenderer(plan, prov, nil, descriptors)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	sc := scene{plan: plan, renderer: r, verifier: v, est: est,
		leaves: map[string]distill.Leaf{}, files: map[string][]byte{}}
	for _, n := range plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			l, err := d.Render(n)
			if err != nil {
				t.Fatalf("Render %s: %v", n.Path, err)
			}
			sc.leaves[n.Path] = l
			sc.files[n.Path] = l.Data
			continue
		}
		data, err := r.Node(n)
		if err != nil {
			t.Fatalf("Node %s: %v", n.Path, err)
		}
		sc.files[n.Path] = data
	}
	for _, f := range r.Fixtures() {
		sc.files[f.Path] = f.Data
	}
	return sc
}

func docFile(c ingest.Corpus, i int, path, body string, head int, title string) survey.File {
	return survey.File{
		Path: path, SHA256: c.Docs[i].SHA256, Bytes: len(body),
		Tokens:   tokens.Estimator{}.EstimateBytes([]byte(body)),
		Preamble: &survey.Section{Level: 0, Start: 0, End: head},
		Sections: []survey.Section{{Level: 1, Title: title, Start: head, End: len(body)}},
	}
}

func (s scene) verification() Verification {
	return Verification{
		Plan: s.plan, Renderer: s.renderer, Files: cloneFiles(s.files),
		Leaves: s.leaves, Verifier: s.verifier, Est: s.est,
	}
}

func cloneFiles(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// The green run: every gate passes over a tree the pipeline built.
func TestVerifyGreen(t *testing.T) {
	sc := newScene(t)
	rep, err := Verify(sc.verification())
	if err != nil {
		t.Fatalf("Verify over a well-formed tree: %v", err)
	}
	if !rep.Passed() {
		t.Fatal("Passed() is false on a run with no error")
	}
	if len(rep.Checks) != 10 {
		t.Fatalf("the report holds %d checks, want the ten of §4.8", len(rep.Checks))
	}
	for i, c := range rep.Checks {
		if c.Number != i+1 {
			t.Errorf("check %d is numbered %d", i+1, c.Number)
		}
	}
	if rep.ClassB != len(Manifest()) {
		t.Errorf("class B holds %d files, the manifest names %d", rep.ClassB, len(Manifest()))
	}
	if rep.ClassA != len(sc.plan.Nodes) {
		t.Errorf("class A holds %d files, the plan holds %d nodes", rep.ClassA, len(sc.plan.Nodes))
	}
}

// One mutation per gate: the tree is broken in one way, and the gate whose
// subject that is must refuse it.
//
// "Must refuse it", not "must be the first to refuse it". The ten gates
// overlap deliberately — a page with its up-link cut also stops being its own
// re-derivation, and a fixture pointed somewhere else also has a dangling link
// — and asserting an ordering would be asserting that the overlap does not
// exist. What matters is that each gate catches its own case, which is what
// makes a failure message point at the cause.
func TestVerifyGates(t *testing.T) {
	tests := []struct {
		name   string
		check  int
		break_ func(t *testing.T, sc scene, v *Verification)
	}{{
		name:  "a link to a file the tree does not deliver",
		check: 1,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyLeaf(t, sc)
			v.Files[p] = []byte(strings.Replace(string(v.Files[p]), "\n\n", "\n\nsee [x](../nowhere.md)\n\n", 1))
			v.Leaves = withData(sc.leaves, p, v.Files[p])
		},
	}, {
		name:  "a page with no up-link",
		check: 2,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyLeaf(t, sc)
			var kept []string
			for _, l := range strings.Split(string(v.Files[p]), "\n") {
				if strings.HasPrefix(l, distill.UpLinkPrefix) {
					continue
				}
				kept = append(kept, l)
			}
			v.Files[p] = []byte(strings.Join(kept, "\n"))
			v.Leaves = withData(sc.leaves, p, v.Files[p])
		},
	}, {
		name:  "an up-link whose label and target name different nodes",
		check: 2,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyLeaf(t, sc)
			node := nodeAt(t, sc, p)
			v.Files[p] = []byte(strings.Replace(string(v.Files[p]),
				distill.UpLinkPrefix+node.Parent+"]", distill.UpLinkPrefix+"elsewhere/index.md]", 1))
			v.Leaves = withData(sc.leaves, p, v.Files[p])
		},
	}, {
		// A continuation edge is the same pair of projections as the up-link
		// [MAD2: B-4], so the same gate reads it: a part pointed at one sibling
		// and labelled with another is the defect a copy or a hand edit makes.
		name:  "a continuation link whose label and target name different nodes",
		check: 2,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyLeaf(t, sc)
			node := nodeAt(t, sc, p)
			up := distill.UpLink(p, node.Parent)
			v.Files[p] = []byte(strings.Replace(string(v.Files[p]),
				up+"\n", up+"\n[→ elsewhere/other.md](index.md)\n", 1))
			v.Leaves = withData(sc.leaves, p, v.Files[p])
		},
	}, {
		name:  "the entry-point was not delivered",
		check: 3,
		break_: func(t *testing.T, sc scene, v *Verification) {
			delete(v.Files, sc.renderer.EntryPointPath())
		},
	}, {
		// The case in-degree could not see [MAD2: B-9]: every member of the
		// island is linked from another class-A page, and no descent from the
		// entry-point arrives at any of them.
		name:  "a severed island the entry-point no longer names",
		check: 4,
		break_: func(t *testing.T, sc scene, v *Verification) {
			entry := sc.renderer.EntryPointPath()
			island, rest := anyIndex(t, sc), otherIndex(t, sc)
			v.Files[entry] = []byte(strings.Replace(string(v.Files[entry]),
				"("+distill.RelPath(entry, island)+")", "("+distill.RelPath(entry, rest)+")", 1))
		},
	}, {
		// The exclusion guarantee 4 states in words: a page whose only inbound
		// edge is its own link to itself is reachable from nothing.
		name:  "a page whose only inbound link is its own",
		check: 4,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyLeaf(t, sc)
			parent := nodeAt(t, sc, p).Parent
			v.Files[parent] = []byte(strings.Replace(string(v.Files[parent]),
				"("+distill.RelPath(parent, p)+")", "("+distill.RelPath(parent, parent)+")", 1))
			v.Files[p] = []byte(strings.Replace(string(v.Files[p]),
				"\n\n", "\n\nsee [this very page]("+path.Base(p)+")\n\n", 1))
			v.Leaves = withData(sc.leaves, p, v.Files[p])
		},
	}, {
		name:  "a fixture that does not link to the entry-point",
		check: 4,
		break_: func(t *testing.T, sc scene, v *Verification) {
			v.Files[ReadmeFixture] = []byte(strings.ReplaceAll(
				string(v.Files[ReadmeFixture]), "("+sc.renderer.EntryPointPath()+")", "(elsewhere.md)"))
		},
	}, {
		name:  "a delivered file that is neither a node nor a fixture",
		check: 5,
		break_: func(t *testing.T, sc scene, v *Verification) {
			v.Files["stray.md"] = []byte("# Stray\n")
		},
	}, {
		name:  "a tree plan whose groups do not cover the corpus",
		check: 6,
		break_: func(t *testing.T, sc scene, v *Verification) {
			plan := v.Plan
			plan.Groups = append([]treeplan.SplitGroup(nil), plan.Groups...)
			plan.Groups[0].Source.End = plan.Groups[0].Source.Start + 1
			v.Plan = plan
		},
	}, {
		name:  "a page whose bytes are not its re-derivation",
		check: 7,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyLeaf(t, sc)
			v.Files[p] = append([]byte("tampered\n"), v.Files[p]...)
		},
	}, {
		name:  "an entry-point past its token ceiling",
		check: 8,
		break_: func(t *testing.T, sc scene, v *Verification) {
			plan := v.Plan
			plan.Budgets.EntryPointTokens = 1
			v.Plan = plan
		},
	}, {
		name:  "an index with no down-link list",
		check: 9,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyIndex(t, sc)
			v.Files[p] = []byte(strings.Replace(string(v.Files[p]), derivationsHeading, "## Elsewhere", 1))
		},
	}, {
		name:  "a page declaring another page's Location",
		check: 10,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyLeaf(t, sc)
			v.Files[p] = []byte(strings.Replace(string(v.Files[p]),
				"Location: "+p, "Location: somewhere/else.md", 1))
			v.Leaves = withData(sc.leaves, p, v.Files[p])
		},
	}, {
		name:  "a page delivered without its frontmatter block",
		check: 10,
		break_: func(t *testing.T, sc scene, v *Verification) {
			p := anyIndex(t, sc)
			_, body, _ := distill.ParseFrontmatter(v.Files[p])
			v.Files[p] = []byte(strings.Join(body, "\n"))
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := newScene(t)
			v := sc.verification()
			tt.break_(t, sc, &v)
			rep, err := Verify(v)
			if err == nil {
				t.Fatalf("the gates accepted a tree with %s", tt.name)
			}
			var failed []int
			caught := false
			for _, c := range rep.Checks {
				if c.OK {
					continue
				}
				failed = append(failed, c.Number)
				if c.Number == tt.check {
					caught = true
				}
			}
			if !caught {
				t.Errorf("gate %d passed over its own case; gates %v refused instead (%v)", tt.check, failed, err)
			}
		})
	}
}

// Check 5 compares an index against a fresh render, so a tree plan that
// changed under a delivered tree is caught even when every link still
// resolves — the §0.3 drift class.
func TestVerifyCatchesTitleDrift(t *testing.T) {
	sc := newScene(t)
	v := sc.verification()
	plan := v.Plan
	plan.Nodes = append([]treeplan.Node(nil), plan.Nodes...)
	for i := range plan.Nodes {
		if plan.Nodes[i].Kind == treeplan.KindIndex {
			plan.Nodes[i].Title = "Renamed"
			break
		}
	}
	v.Plan = plan
	r, err := NewRenderer(plan, distill.Provenance{CorpusHash: plan.CorpusHash, BuildDate: "2026-08-13"}, nil, nil)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	v.Renderer = r
	if _, err := Verify(v); err == nil {
		t.Fatal("a delivered tree whose titles no longer match the plan was accepted")
	}
}

func TestEntryPointGrammar(t *testing.T) {
	sc := newScene(t)
	entry := sc.files[sc.renderer.EntryPointPath()]
	text := string(entry)
	for _, want := range []string{domainsHeading, usingHeading, contractText, definitionsPointer} {
		if !strings.Contains(text, want) {
			t.Errorf("the entry-point is missing a mandatory block:\n%s", want)
		}
	}
	if strings.Contains(text, distill.UpLinkPrefix) {
		t.Error("the entry-point carries an up-link")
	}
	if loc, _, ok := distill.ParseFrontmatter(entry); !ok || loc != sc.renderer.EntryPointPath() {
		t.Errorf("the entry-point declares Location %q (block found: %t), want its own path", loc, ok)
	}
	if strings.Contains(text, annexHeading) {
		t.Error("the entry-point carries an annex section with no annexes declared")
	}
	if !distill.HasFooter(entry) {
		t.Error("the entry-point does not end in its provenance receipt")
	}
}

func TestIndexRendersEmptySummariesLegally(t *testing.T) {
	sc := newScene(t)
	p := anyIndex(t, sc)
	text := string(sc.files[p])
	loc, body, ok := distill.ParseFrontmatter(sc.files[p])
	if !ok || loc != p {
		t.Errorf("an index does not open with its own Location block (%q, ok=%t):\n%s", loc, ok, text)
	}
	if !strings.HasPrefix(distill.FirstLine(body), distill.UpLinkPrefix) {
		t.Errorf("an index does not carry its up-link under the block:\n%s", text)
	}
	if !strings.Contains(text, derivationsHeading) {
		t.Errorf("an index has no down-link list:\n%s", text)
	}
	// Every child of the node is a line in the list, title and scope verbatim
	// from the plan.
	for _, c := range sc.renderer.Children(p) {
		want := "- [" + c.Title + "](" + distill.RelPath(p, c.Path) + ") — " + c.Scope
		if !strings.Contains(text, want) {
			t.Errorf("the down-link list is missing %q:\n%s", want, text)
		}
	}
}

// The gate reads the navigation BLOCK, not the page: a leaf's verbatim body
// may carry its source site's own prev/next furniture, and a check broader
// than the contract it enforces would refuse a delivery nobody can repair
// [MAD2: B-11].
func TestVerifyLeavesNavShapedSourceLinesAlone(t *testing.T) {
	sc := newScene(t)
	v := sc.verification()
	p := anyLeaf(t, sc)
	// Below the page's own heading, and pointing at a file the tree really
	// delivers so that guarantee 1 has no quarrel with it: the label is prose,
	// which is exactly what the up-link agreement rule forbids of page grammar.
	marker := "# " + nodeAt(t, sc, p).Title + "\n"
	text := string(v.Files[p])
	if !strings.Contains(text, marker) {
		t.Fatalf("%s carries no heading to insert below:\n%s", p, text)
	}
	v.Files[p] = []byte(strings.Replace(text, marker, marker+"\n[← Back to the guide](index.md)\n", 1))
	v.Leaves = withData(sc.leaves, p, v.Files[p])

	if _, err := Verify(v); err != nil {
		t.Fatalf("a source line shaped like a continuation link refused the delivery: %v", err)
	}
}

// A split part's bullet carries the group's shared scope AND its own post-cut
// descriptor, which is what makes two sibling bullets choosable [MAD2: B-4].
func TestIndexBulletsCarryPartDescriptors(t *testing.T) {
	sc := newScene(t)
	p := anyIndex(t, sc)
	kids := sc.renderer.Children(p)
	if len(kids) == 0 {
		t.Fatalf("%s holds no children to render a bullet for", p)
	}
	child := kids[0]
	bullet := "- [" + child.Title + "](" + distill.RelPath(p, child.Path) + ") — " + child.Scope

	render := func(descriptors map[string]string) string {
		t.Helper()
		prov := distill.Provenance{CorpusHash: sc.plan.CorpusHash, BuildDate: "2026-08-13"}
		r, err := NewRenderer(sc.plan, prov, nil, descriptors)
		if err != nil {
			t.Fatalf("NewRenderer: %v", err)
		}
		page, err := r.Node(nodeAt(t, sc, p))
		if err != nil {
			t.Fatalf("Node %s: %v", p, err)
		}
		return string(page)
	}

	described := render(map[string]string{child.Path: "First Movement → Second Movement"})
	if want := bullet + " (this part: First Movement → Second Movement)\n"; !strings.Contains(described, want) {
		t.Errorf("the down-link list is missing %q:\n%s", want, described)
	}
	// The shared scope survives beside it: the descriptor is added to the
	// bullet grammar, not substituted for the group's own scope line.
	if !strings.Contains(described, child.Scope) {
		t.Errorf("the part's bullet lost the group's scope:\n%s", described)
	}
	plain := render(nil)
	if want := bullet + "\n"; !strings.Contains(plain, want) {
		t.Errorf("a child with no descriptor did not render its plain bullet %q:\n%s", want, plain)
	}
}

func TestFixtureManifestIsClosed(t *testing.T) {
	sc := newScene(t)
	got := map[string]bool{}
	for _, f := range sc.renderer.Fixtures() {
		got[f.Path] = true
		if len(f.Data) == 0 {
			t.Errorf("the fixture %s rendered nothing", f.Path)
		}
	}
	for _, want := range Manifest() {
		if !got[want] {
			t.Errorf("the manifest names %s and Fixtures() does not render it", want)
		}
	}
	if len(got) != len(Manifest()) {
		t.Errorf("Fixtures() renders %d files, the manifest names %d", len(got), len(Manifest()))
	}
	for _, f := range sc.renderer.Fixtures() {
		if strings.HasSuffix(f.Path, ".md") && !strings.Contains(string(f.Data), generatedNotice) {
			t.Errorf("the fixture %s does not say it was generated", f.Path)
		}
	}
}

func anyLeaf(t *testing.T, sc scene) string {
	t.Helper()
	for _, n := range sc.plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			return n.Path
		}
	}
	t.Fatal("the fixture has no pages")
	return ""
}

// nodeAt is one delivered path's tree-plan node, for a mutation that has to
// know what the page says about itself.
func nodeAt(t *testing.T, sc scene, path string) treeplan.Node {
	t.Helper()
	for _, n := range sc.plan.Nodes {
		if n.Path == path {
			return n
		}
	}
	t.Fatalf("%s is not a node of the fixture's tree plan", path)
	return treeplan.Node{}
}

func anyIndex(t *testing.T, sc scene) string {
	t.Helper()
	for _, n := range sc.plan.Nodes {
		if n.Kind == treeplan.KindIndex {
			return n.Path
		}
	}
	t.Fatal("the fixture has no sections")
	return ""
}

// otherIndex is the second section — the one a severed island's entry-point
// link is diverted to, so the entry-point still names something delivered.
func otherIndex(t *testing.T, sc scene) string {
	t.Helper()
	first := anyIndex(t, sc)
	for _, n := range sc.plan.Nodes {
		if n.Kind == treeplan.KindIndex && n.Path != first {
			return n.Path
		}
	}
	t.Fatal("the fixture has only one section")
	return ""
}

// withData is the leaves map with one page's re-derivation replaced, for the
// mutations that edit a delivered page and must not then be caught by check 7
// before the gate under test sees them.
func withData(in map[string]distill.Leaf, path string, data []byte) map[string]distill.Leaf {
	out := make(map[string]distill.Leaf, len(in))
	for k, v := range in {
		out[k] = v
	}
	l := out[path]
	l.Data = data
	out[path] = l
	return out
}
