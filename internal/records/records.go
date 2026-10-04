// Package records is the schema of the reader and placement facts the
// document graph writes into the build state store and the claim graph reads
// back, and where in the store they live.
package records

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"kbase/internal/atomicfile"
)

// Record kinds.
const (
	KindBlock     = "block"
	KindReference = "reference"
	KindCitation  = "citation"
	KindFence     = "fence"
	KindWork      = "work"
)

// Citation states, per key: resolved by a bibliography, offered one that did
// not answer it, or offered none.
const (
	StateResolved   = "resolved"
	StateUnanswered = "unanswered"
	StateKeyOnly    = "key-only"
)

// Record is one reader fact the claim graph needs, placed in the document it
// landed in. Records are written in the order the tree is read: documents by
// path, and within a document in the order the page carries them. Order is a
// record's position in that sequence, from 1. A value the page carries in an
// HTML attribute — an identifier, a reference's type, labels and href, a
// citation or work key — is spelled as the writer spells the attribute, which
// is how kb_tools' readers read it.
type Record struct {
	Kind     string `json:"kind"`
	Order    int    `json:"order"`
	Document string `json:"document"`
	// Within is the order of the author-declared block holding this fact on
	// the page, 0 for none.
	Within int `json:"within,omitempty"`

	// A block: the name its label line carries and its identifier.
	Name       string `json:"name,omitempty"`
	Identifier string `json:"identifier,omitempty"`

	// A reference: the macro type as pandoc spells it, the label attribute as
	// written, whether it sits in its block's opening emphasis run, the href
	// it is written with, and the document that href names ("" for a bare
	// fragment).
	Type    string `json:"type,omitempty"`
	Labels  string `json:"labels,omitempty"`
	Opening bool   `json:"opening,omitempty"`
	Href    string `json:"href,omitempty"`
	Target  string `json:"target,omitempty"`

	// A citation: its keys and each key's state.
	Keys   []string `json:"keys,omitempty"`
	States []string `json:"states,omitempty"`

	// A display-maths fence: the \label names inside it.
	EquationLabels []string `json:"equation_labels,omitempty"`

	// A work of the bibliography: the citation key its entry is keyed by.
	Key string `json:"key,omitempty"`
}

// Where the records live under a build's state directory, with the pre-pass
// census of the source they were read from beside them.
const (
	Dir          = "records"
	DocgraphFile = "docgraph.jsonl"
	PrepassFile  = "prepass.jsonl"
)

// Write lands the records and the pre-pass census under stateDir, each file
// replaced whole and atomically: a stage reading the store after a killed
// build finds a whole file or the one before it.
func Write[C any](stateDir string, recs []Record, censuses []C) error {
	dir := filepath.Join(stateDir, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := encodeJSONL(recs)
	if err != nil {
		return err
	}
	if err := atomicfile.Write(filepath.Join(dir, DocgraphFile), data, nil); err != nil {
		return err
	}
	if data, err = encodeJSONL(censuses); err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(dir, PrepassFile), data, nil)
}

// encodeJSONL is one JSON object per line, HTML characters as written.
func encodeJSONL[T any](items []T) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// Path is the document graph's record file under stateDir.
func Path(stateDir string) string {
	return filepath.Join(stateDir, Dir, DocgraphFile)
}

// Read is every record of the JSONL file at path, in file order.
func Read(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	for line := 1; sc.Scan(); line++ {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		out = append(out, r)
	}
	return out, sc.Err()
}
