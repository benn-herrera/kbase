package treeplan

import (
	"errors"
	"strings"
	"testing"
)

// Check is the sole statement of what a valid tree plan is, and stage 9 runs it
// over an artifact it did not build. These are the tamperings that artifact
// could arrive with — each one a claim the composed checks have to catch on
// their own, without anything the composer remembers.
func TestCheckCatchesTampering(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(*TreePlan)
		wantDefect bool
	}{{
		name:       "a duplicated path",
		mutate:     func(s *TreePlan) { s.Nodes[3].Path = s.Nodes[2].Path },
		wantDefect: true,
	}, {
		name:       "a parent that is not there",
		mutate:     func(s *TreePlan) { s.Nodes[2].Parent = "elsewhere/index.md" },
		wantDefect: true,
	}, {
		name:       "a child outside its parent's directory",
		mutate:     func(s *TreePlan) { s.Nodes[2].Path = "somewhere-else.md" },
		wantDefect: true,
	}, {
		name:       "a leaf naming a group nobody holds",
		mutate:     func(s *TreePlan) { s.Nodes[2].SplitGroup = "g9999" },
		wantDefect: true,
	}, {
		name:       "a part number past the group's count",
		mutate:     func(s *TreePlan) { s.Nodes[2].Part = 99 },
		wantDefect: true,
	}, {
		name:       "a part count the splitter disagrees with",
		mutate:     func(s *TreePlan) { s.Groups[0].Parts++ },
		wantDefect: true,
	}, {
		name:       "a group nobody draws on",
		mutate:     func(s *TreePlan) { s.Groups = append(s.Groups, s.Groups[0]) },
		wantDefect: true,
	}, {
		name:       "a budget the group was not cut against",
		mutate:     func(s *TreePlan) { s.Groups[0].Budget = 7 },
		wantDefect: true,
	}, {
		name:       "a second entry-point",
		mutate:     func(s *TreePlan) { s.Nodes[1].Kind = KindEntryPoint },
		wantDefect: true,
	}, {
		name:       "another corpus",
		mutate:     func(s *TreePlan) { s.CorpusHash = "0000" },
		wantDefect: true,
	}, {
		name: "a span shrunk so material falls out of the tree",
		mutate: func(s *TreePlan) {
			// Shrunk by a tail, not down to a stub: a span under the content
			// floor is refused by the group checks before coverage is ever
			// asked, and what this tampering is about is coverage.
			s.Groups[len(s.Groups)-1].Source.End -= 40
		},
	}, {
		name: "a span shrunk under the content floor",
		mutate: func(s *TreePlan) {
			g := &s.Groups[len(s.Groups)-1]
			g.Source.End = g.Source.Start + 8
		},
		wantDefect: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			v, s := composeFixture(t)
			if err := v.Check(s); err != nil {
				t.Fatalf("the untampered fixture does not verify: %v", err)
			}
			tc.mutate(&s)
			err := v.Check(s)
			if err == nil {
				t.Fatal("the tampered artifact verified")
			}
			var defect DefectError
			if got := errors.As(err, &defect); got != tc.wantDefect {
				t.Fatalf("defect = %v, want %v (err: %v)", got, tc.wantDefect, err)
			}
			if !tc.wantDefect {
				if _, ok := AsRejection(err); !ok {
					t.Fatalf("want a rejection, got %T: %v", err, err)
				}
			}
		})
	}
}

// Coverage is boundary-set tiling: for every file the group spans exactly tile
// the material the survey found, and every endpoint is a section start the
// survey drew.
//
// The mutations are over a DESCENDED tree — a container covered by its subtree
// rather than by one span, which the old containment predicate could not
// express — so these prove the gate is strictly stronger where it was widened,
// and not merely wider.
func TestCheckRefusesATilingThatIsNotOne(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(*testing.T, *TreePlan)
		wantDefect bool
		wantIn     string
	}{{
		name: "a gap between two pages",
		mutate: func(t *testing.T, s *TreePlan) {
			// The boundary moves on one side only: the bytes between the two
			// pages are on neither of them.
			s.Groups[0].Source.End -= 40
		},
		wantIn: "on no page",
	}, {
		name: "material left off the front of a file",
		mutate: func(t *testing.T, s *TreePlan) {
			s.Groups[0].Source.Start += 40
		},
		wantIn: "on no page",
	}, {
		name: "material left off the end of a file",
		mutate: func(t *testing.T, s *TreePlan) {
			s.Groups[len(s.Groups)-1].Source.End -= 40
		},
		wantIn: "on no page",
	}, {
		name: "two pages over the same bytes",
		mutate: func(t *testing.T, s *TreePlan) {
			s.Groups[0].Source.End += 40
		},
		wantDefect: true,
		wantIn:     "overlap",
	}, {
		// The one only the boundary set catches on this shape: the spans still
		// tile the file with no gap and no overlap, but the boundary between
		// two of them is a byte nobody's heading is at.
		name: "a page that begins inside a section",
		mutate: func(t *testing.T, s *TreePlan) {
			s.Groups[0].Source.End += 40
			s.Groups[1].Source.Start += 40
		},
		wantIn: "inside a section",
	}, {
		// The front-matter block is surveyed as an out-of-band range, not as a
		// section: it is outside the coverage universe, so a page drawing on it
		// is drawing on material nothing planned.
		name: "a page reaching into the front matter",
		mutate: func(t *testing.T, s *TreePlan) {
			s.Groups[0].Source.Start = 0
		},
		wantDefect: true,
		wantIn:     "outside the material",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			v, s := descendedFixture(t)
			if err := v.Check(s); err != nil {
				t.Fatalf("the untampered fixture does not verify: %v", err)
			}
			tc.mutate(t, &s)
			err := v.Check(s)
			if err == nil {
				t.Fatal("the tampered artifact verified")
			}
			var defect DefectError
			if got := errors.As(err, &defect); got != tc.wantDefect {
				t.Fatalf("defect = %v, want %v (err: %v)", got, tc.wantDefect, err)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("refusal %q does not name %q", err, tc.wantIn)
			}
			if !tc.wantDefect {
				r, ok := AsRejection(err)
				if !ok {
					t.Fatalf("want a rejection, got %T: %v", err, err)
				}
				assertNoteIsPromptable(t, r)
			}
		})
	}
}

// descendedFixture is one document whose sections tile it through a container:
// a page, then a container's body and its two subsections. Its groups are in
// file byte order, which is what the mutations above index into.
func descendedFixture(t *testing.T) (*Verifier, TreePlan) {
	t.Helper()
	doc := docSpec{path: "one.md", title: "One", meta: true, secs: []secSpec{
		{title: "Aside", paras: 2, words: 45},
		{title: "Cluster", paras: 2, words: 45, subs: []secSpec{
			{title: "Workers", paras: 2, words: 50}, {title: "Scheduler", paras: 2, words: 50}}},
	}}
	v, art := verifierFor(t, testParams(), doc)
	p, err := SourceStructureProposal(art, testBudgets(), "Corpus", "all", nil)
	if err != nil {
		t.Fatalf("SourceStructureProposal: %v", err)
	}
	s, err := v.Compose(p, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(s.Groups) != 4 {
		t.Fatalf("groups = %d, want four: a page, a body and two subsections", len(s.Groups))
	}
	return v, s
}

// The composed tree plan's own budgets travel with it, so a consumer that reads
// them back gets the numbers the tree was verified against and not this
// build's constants.
func TestArtifactCarriesTheBudgetsItWasVerifiedAgainst(t *testing.T) {
	_, s := composeFixture(t)
	if s.Budgets != testBudgets() {
		t.Fatalf("budgets = %+v, want %+v", s.Budgets, testBudgets())
	}
}

func TestDefaultBudgetsAreCoherent(t *testing.T) {
	if err := DefaultBudgets().Validate(); err != nil {
		t.Fatalf("the shipped operating point does not validate: %v", err)
	}
}
