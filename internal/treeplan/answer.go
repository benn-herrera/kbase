package treeplan

import (
	"fmt"

	"kbase/internal/survey"
	"kbase/internal/text"
)

// GroupingAnswer is one container call's answer in the taxonomy descent (§3.2): the
// model was shown a numbered candidate list — that container's direct children
// — and grouped them.
//
// It carries no path, no filename and no link. Titles and scopes are the whole
// of what the model writes here, and they are for the nodes it is creating
// (I-2). The names come from Slug and the paths from the tree, downstream of
// this type.
type GroupingAnswer struct {
	Groups []AnswerGroup
}

// AnswerGroup is one group of candidates the model made.
//
// Members is the group's candidate indices, zero-based into the list the call
// presented. The design states the answer the other way round — a group index
// per candidate — which is the same information; this shape is the one where
// the partition check has something to catch. With one group index per
// candidate a drop and a duplicate are unrepresentable, and §3.3's first
// post-condition ("no drops, no duplicates" — the silently-missing-chapter
// failure) would be a check that can never fire. Here both are ordinary
// malformed answers, which is what they are when a model writes JSON.
type AnswerGroup struct {
	Title string
	Scope string
	// Kind is KindLeaf or KindIndex. KindEntryPoint is not a model-facing
	// answer, and neither is an annex: annexes are declared by configuration
	// only (O-12), because a model that can place material out of scope can
	// hide its own failures.
	Kind    Kind
	Members []int
}

// CheckAnswer runs §3.3's per-call post-conditions over one container call's
// answer, before the descent goes any deeper.
//
// candidates is how many direct children the call presented. depth is the
// level the answer's groups will sit at — the container's level plus one, with
// the entry-point at level 1.
//
// The checks are the three that are decidable from one call: partition,
// fan-out, and titles. The remaining post-conditions (G-1, G-2, coverage, path
// uniqueness) need the whole tree and are Compose's, where §2.7's operators
// have run.
//
// depth is not checked against a cap, because there is none (R-3, ruled
// 2026-08-17): a tree is as deep as the source's own nesting makes it, and a
// group flagged `index` at any level is an ordinary answer. What remains is the
// seam's own contract — a call must place its groups BELOW the entry-point —
// which is our arithmetic and therefore a defect when it is wrong.
func (v *Verifier) CheckAnswer(a GroupingAnswer, candidates, depth int) error {
	if candidates <= 0 {
		return DefectError{Reason: fmt.Sprintf(
			"a container call presented %d candidates; a call with nothing to group is a call nobody should describe",
			candidates)}
	}
	if candidates > v.p.Budgets.CandidateCap {
		return DefectError{Reason: fmt.Sprintf(
			"a container call presented %d candidates against a cap of %d; pre-batching is mechanical, so this is our arithmetic",
			candidates, v.p.Budgets.CandidateCap)}
	}
	if depth < 2 {
		return DefectError{Reason: fmt.Sprintf(
			"a container call places groups at level %d; the entry-point is level 1 and every group is below it", depth)}
	}

	if len(a.Groups) == 0 {
		return RejectionError{Reason: "no groups; every candidate belongs to one group"}
	}
	if len(a.Groups) > v.p.Budgets.FanOutCap {
		return RejectionError{Subject: fmt.Sprintf("%d groups", len(a.Groups)),
			Reason: "too many groups; combine them into fewer"}
	}

	// Partition: every candidate in exactly one group. Counting occurrences
	// catches the drop and the duplicate with the same pass, and an
	// out-of-range member is caught before it can index anything.
	seen := make([]int, candidates)
	for gi, g := range a.Groups {
		if len(g.Members) == 0 {
			return RejectionError{Subject: fmt.Sprintf("group %d", gi+1),
				Reason: "a group holds no entries; every group holds at least one"}
		}
		for _, m := range g.Members {
			if m < 0 || m >= candidates {
				return RejectionError{Subject: fmt.Sprintf("group %d entry %d", gi+1, m),
					Reason: "an entry number is not on the list"}
			}
			seen[m]++
		}
	}
	for i, n := range seen {
		if n == 0 {
			return RejectionError{Subject: fmt.Sprintf("candidate %d", i+1),
				Reason: "an entry was left out; every entry belongs to one group"}
		}
		if n > 1 {
			return RejectionError{Subject: fmt.Sprintf("candidate %d", i+1),
				Reason: "an entry is in two groups; every entry belongs to one"}
		}
	}

	titles := make(map[string]bool, len(a.Groups))
	for gi, g := range a.Groups {
		subject := fmt.Sprintf("group %d", gi+1)
		title := normalizeLabel(g.Title)
		if title == "" {
			return RejectionError{Subject: subject, Reason: "a group has no title"}
		}
		if titles[title] {
			return RejectionError{Subject: subject, Reason: "two groups share a title; each needs its own"}
		}
		titles[title] = true
		if normalizeLabel(g.Scope) == "" {
			return RejectionError{Subject: subject, Reason: "a group has no one-line scope"}
		}
		switch g.Kind {
		case KindLeaf, KindIndex:
		default:
			return RejectionError{Subject: subject, Reason: "a group is neither a page nor a section"}
		}
	}
	return nil
}

// normalizeLabel is the one spelling of what a title or a scope line becomes.
//
// It collapses whitespace (a title the model wrapped across lines is one line)
// and caps at survey.WordCap, the same promise the survey artifact's own
// labels carry — these strings are rendered into every parent's down-link list
// and paid for in the artifact every stage reads. Truncating is safe because
// both are labels; nothing they label moves.
func normalizeLabel(s string) string { return text.CapWords(s, survey.WordCap) }
