package treeplan

import (
	"errors"
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
			s.Groups[len(s.Groups)-1].Source.End = s.Groups[len(s.Groups)-1].Source.Start + 8
		},
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
