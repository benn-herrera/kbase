package claimgraph

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"kbase/internal/atomicfile"
)

// The two tracked build records, at the repository root, beside kb-root/:
// build state a later stage of the same build reads, which never enters the
// KB itself.
const (
	NodePassFile       = "kb-build-node-pass.yaml"
	ClassificationFile = "kb-build-classification.yaml"
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

// NodePassRecord is the node pass's build state, per leaf.
type NodePassRecord struct {
	About  string               `yaml:"about"`
	Leaves map[string]LeafEntry `yaml:"leaves"`
}

// LeafEntry is one leaf's read state, outcome, planned claims and verdicts.
type LeafEntry struct {
	State    string             `yaml:"state"`
	Outcome  *string            `yaml:"outcome"`
	Claims   []PlannedClaim     `yaml:"claims"`
	Verdicts []ParagraphVerdict `yaml:"verdicts"`
}

// PlannedClaim is a claim a leaf's outcome mints: its title and locator.
type PlannedClaim struct {
	Title   string `yaml:"title"`
	Locator string `yaml:"locator"`
}

// ParagraphVerdict is one asked paragraph's verdict, by the 0-based line it
// begins on; Cause is set exactly where the judgement is defaulted.
type ParagraphVerdict struct {
	Line    int     `yaml:"line"`
	Verdict string  `yaml:"verdict"`
	Cause   *string `yaml:"cause"`
}

// ClassificationRecord is every classified edge candidate, ordered by pair.
type ClassificationRecord struct {
	About      string           `yaml:"about"`
	Candidates []CandidateEntry `yaml:"candidates"`
}

// CandidateEntry is one candidate's classification: the letters offered, the
// one chosen (nil where none was), and how it was reached.
type CandidateEntry struct {
	Source     string              `yaml:"source"`
	Target     string              `yaml:"target"`
	Offered    []string            `yaml:"offered"`
	Letter     *string             `yaml:"letter"`
	Outcome    string              `yaml:"outcome"`
	Confidence *map[string]float64 `yaml:"confidence"`
}

func writeRecord(path string, v any) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return atomicfile.Write(path, buf.Bytes(), nil)
}

// readRecord reads the record at path into v; false where none stands.
func readRecord(path string, v any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := yaml.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("%s does not read as a build record: %w", path, err)
	}
	return true, nil
}

// WriteNodePass lands the node-pass record whole at the repository root.
func WriteNodePass(repoRoot string, r NodePassRecord) error {
	r.About = nodePassAbout
	return writeRecord(filepath.Join(repoRoot, NodePassFile), r)
}

// ReadNodePass is the node-pass record, or false where none stands.
func ReadNodePass(repoRoot string) (NodePassRecord, bool, error) {
	var r NodePassRecord
	ok, err := readRecord(filepath.Join(repoRoot, NodePassFile), &r)
	return r, ok, err
}

// WriteClassification lands the classification record whole.
func WriteClassification(repoRoot string, r ClassificationRecord) error {
	r.About = classificationAbout
	if r.Candidates == nil {
		r.Candidates = []CandidateEntry{}
	}
	return writeRecord(filepath.Join(repoRoot, ClassificationFile), r)
}

// ReadClassification is the classification record, or false where none stands.
func ReadClassification(repoRoot string) (ClassificationRecord, bool, error) {
	var r ClassificationRecord
	ok, err := readRecord(filepath.Join(repoRoot, ClassificationFile), &r)
	return r, ok, err
}
