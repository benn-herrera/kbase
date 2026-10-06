package claimgraph

import (
	"cmp"
	"path/filepath"
	"slices"

	"kbase/internal/atomicfile"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
)

// A leaf's read state: no reading has reached it; its whole outcome is in the
// record and its writes may not all have landed; landed.
const (
	ReadUnread  = "unread"
	ReadPlanned = "planned"
	ReadLanded  = "landed"
)

// What reading a leaf came to: a claim minted; read, minting nothing; no
// paragraph to ask about.
const (
	OutcomeMinted        = "minted"
	OutcomeNoClaim       = "no-claim"
	OutcomeNothingToRead = "nothing-to-read"
)

// A paragraph verdict, and the cause a defaulted one carries: no offered
// letter after the one re-ask, or a yes whose paragraph the write path cannot
// place exactly once.
const (
	JudgementClaim     = "claim"
	JudgementNotAClaim = "not-a-claim"
	JudgementDefaulted = "defaulted"
	CauseNoLetter      = "no-letter"
	CauseUnplaceable   = "unplaceable"
)

// DefaultCauses is every cause, in the order a report states them.
var DefaultCauses = []string{CauseNoLetter, CauseUnplaceable}

// How a candidate's classification was reached: answered, re-asked once and
// answered, asked and answered with no offered letter twice, or no reader
// there to ask. The last two take the draft.
const (
	ClassifyAnswered  = "answered"
	ClassifyReasked   = "re-asked"
	ClassifyDefaulted = "defaulted"
	ClassifyDrafted   = "drafted"
)

const nodePassAbout = "The claim-graph node pass's build record: each leaf's read state and outcome, and a verdict for every " +
	"paragraph of readable prose the pass asked about — claim, not a claim, or defaulted with its cause — " +
	"identified by the 0-based line it begins on. It joins to the tree at this build's own boundary commits, " +
	"not to a KB a maintainer later edits."

const classificationAbout = "The claim-graph classification's build record: for each edge candidate, keyed by its source and target " +
	"claim ids, the letters its ask offered, the letter chosen (null where none was), and how it was reached. " +
	"It joins to the tree at this build's own boundary commits, not to a KB a maintainer later edits."

const unmarkedAbout = "The claim-graph unmarked-reference build record: the shortlist of ordered claim pairs planned for asking " +
	"(null until the stage plans one), and for each pair asked, keyed by its source and target claim ids, the " +
	"letters its ask offered, the letter chosen (null where none was), and how it was reached. It joins to the " +
	"tree at this build's own boundary commits, not to a KB a maintainer later edits."

// unanswered is the planned pairs r holds no outcome for, in plan order.
func unanswered(r buildrecords.UnmarkedRecord) []pair {
	held := map[pair]bool{}
	for _, e := range r.Pairs {
		held[pair{e.Source, e.Target}] = true
	}
	var out []pair
	if r.Planned != nil {
		for _, p := range *r.Planned {
			if !held[pair{p[0], p[1]}] {
				out = append(out, pair{p[0], p[1]})
			}
		}
	}
	return out
}

// encodeRecord is v as its record file holds it.
func encodeRecord(v any) ([]byte, error) { return kb.RecordYAML(v) }

// orEmpty is s, an empty slice where it is nil, so the record writes [] as
// kb_tools does rather than null.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// sortedPairs is entries ordered by pair, every list in them written.
func sortedPairs(entries []buildrecords.CandidateEntry) []buildrecords.CandidateEntry {
	out := slices.Clone(orEmpty(entries))
	for i := range out {
		out[i].Offered = orEmpty(out[i].Offered)
	}
	slices.SortFunc(out, func(a, b buildrecords.CandidateEntry) int {
		return comparePairs(pair{a.Source, a.Target}, pair{b.Source, b.Target})
	})
	return out
}

// nodePassBytes is the node-pass record as WriteNodePass lands it: leaves by
// path, each leaf's verdicts by line.
func nodePassBytes(r buildrecords.NodePassRecord) ([]byte, error) {
	leaves := make(map[string]buildrecords.LeafEntry, len(r.Leaves))
	for p, e := range r.Leaves {
		e.Claims = orEmpty(e.Claims)
		e.Verdicts = slices.Clone(orEmpty(e.Verdicts))
		slices.SortStableFunc(e.Verdicts, func(a, b buildrecords.ParagraphVerdict) int { return cmp.Compare(a.Line, b.Line) })
		leaves[p] = e
	}
	return encodeRecord(buildrecords.NodePassRecord{About: nodePassAbout, Leaves: leaves})
}

// WriteNodePass lands the node-pass record whole at the repository root.
func WriteNodePass(repoRoot string, r buildrecords.NodePassRecord) error {
	b, err := nodePassBytes(r)
	if err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(repoRoot, buildrecords.NodePassFile), b, nil)
}

// classificationBytes is the classification record as WriteClassification
// lands it: the candidates ordered by pair.
func classificationBytes(r buildrecords.ClassificationRecord) ([]byte, error) {
	return encodeRecord(buildrecords.ClassificationRecord{About: classificationAbout, Candidates: sortedPairs(r.Candidates)})
}

// WriteClassification lands the classification record whole.
func WriteClassification(repoRoot string, r buildrecords.ClassificationRecord) error {
	b, err := classificationBytes(r)
	if err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(repoRoot, buildrecords.ClassificationFile), b, nil)
}

// unmarkedBytes is the unmarked record as WriteUnmarked lands it: the plan in
// its order, the pairs sorted.
func unmarkedBytes(r buildrecords.UnmarkedRecord) ([]byte, error) {
	return encodeRecord(buildrecords.UnmarkedRecord{About: unmarkedAbout, Planned: r.Planned, Pairs: sortedPairs(r.Pairs)})
}

// WriteUnmarked lands the unmarked record whole.
func WriteUnmarked(repoRoot string, r buildrecords.UnmarkedRecord) error {
	b, err := unmarkedBytes(r)
	if err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(repoRoot, buildrecords.UnmarkedFile), b, nil)
}
