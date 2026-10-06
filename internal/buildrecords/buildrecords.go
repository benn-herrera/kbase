// Package buildrecords is the schema of the tracked build records the claim
// graph writes at the repository root, beside kb-root/, where they live, and
// how they are read back: YAML documents of kb_tools' record schema, which
// the sheet reads for edge provenance.
package buildrecords

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
)

// The record files, at the repository root.
const (
	NodePassFile       = "kb-build-node-pass.yaml"
	ClassificationFile = "kb-build-classification.yaml"
	UnmarkedFile       = "kb-build-unmarked.yaml"
)

// RecordFiles is every repository-relative path a build record lives at.
var RecordFiles = []string{NodePassFile, ClassificationFile, UnmarkedFile}

// NodePassRecord is the node pass's build state, per leaf.
type NodePassRecord struct {
	About  string               `json:"about" yaml:"about"`
	Leaves map[string]LeafEntry `json:"leaves" yaml:"leaves"`
}

// LeafEntry is one leaf's read state, outcome, planned claims and verdicts.
type LeafEntry struct {
	State    string             `json:"state" yaml:"state"`
	Outcome  *string            `json:"outcome" yaml:"outcome"`
	Claims   []PlannedClaim     `json:"claims" yaml:"claims"`
	Verdicts []ParagraphVerdict `json:"verdicts" yaml:"verdicts"`
}

// PlannedClaim is a claim a leaf's outcome mints: its title and locator.
type PlannedClaim struct {
	Title   string `json:"title" yaml:"title"`
	Locator string `json:"locator" yaml:"locator"`
}

// ParagraphVerdict is one asked paragraph's verdict, by the 0-based line it
// begins on; Cause is set exactly where the judgement is defaulted.
type ParagraphVerdict struct {
	Line    int     `json:"line" yaml:"line"`
	Verdict string  `json:"verdict" yaml:"verdict"`
	Cause   *string `json:"cause" yaml:"cause"`
}

// ClassificationRecord is every classified edge candidate, ordered by pair.
type ClassificationRecord struct {
	About      string           `json:"about" yaml:"about"`
	Candidates []CandidateEntry `json:"candidates" yaml:"candidates"`
}

// CandidateEntry is one candidate's classification: the letters offered, the
// one chosen (nil where none was), and how it was reached.
type CandidateEntry struct {
	Source     string              `json:"source" yaml:"source"`
	Target     string              `json:"target" yaml:"target"`
	Offered    []string            `json:"offered" yaml:"offered"`
	Letter     *string             `json:"letter" yaml:"letter"`
	Outcome    string              `json:"outcome" yaml:"outcome"`
	Confidence *map[string]float64 `json:"confidence" yaml:"confidence"`
}

// UnmarkedRecord is the unmarked-reference asks' build state: the planned
// shortlist — nil until the stage plans one — and every pair any run asked,
// in the classification record's entry shape, ordered by pair.
type UnmarkedRecord struct {
	About   string           `json:"about" yaml:"about"`
	Planned *[]PlannedPair   `json:"planned" yaml:"planned"`
	Pairs   []CandidateEntry `json:"pairs" yaml:"pairs"`
}

// PlannedPair is one planned (source, target).
type PlannedPair [2]string

// Read reads the record at path into v; false where none stands.
func Read(src *kb.Source, path string, v any) (bool, error) {
	data, err := src.ReadFile(path)
	if kb.ErrNotExist(err) {
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

// ReadNodePass is the node-pass record, or false where none stands.
func ReadNodePass(src *kb.Source) (NodePassRecord, bool, error) {
	var r NodePassRecord
	ok, err := Read(src, src.Path(NodePassFile), &r)
	return r, ok, err
}

// ReadClassification is the classification record, or false where none stands.
func ReadClassification(src *kb.Source) (ClassificationRecord, bool, error) {
	var r ClassificationRecord
	ok, err := Read(src, src.Path(ClassificationFile), &r)
	return r, ok, err
}

// ReadUnmarked is the unmarked record, or false where none stands.
func ReadUnmarked(src *kb.Source) (UnmarkedRecord, bool, error) {
	var r UnmarkedRecord
	ok, err := Read(src, src.Path(UnmarkedFile), &r)
	return r, ok, err
}
