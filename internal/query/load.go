// Package query answers kb_tools' kb_cmd questions over a KB's derived index:
// the .index/*.jsonl files loaded as kb_cmd loads them, and each query's
// payload shaped as kb_cmd --json shapes it.
package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// requiredFiles are the index files a query loads; supported-by is not read.
var requiredFiles = []string{"claims", "depends-on", "strengthen-by", "cites", "subtree-aggregates"}

// Node is one claims.jsonl record. Which fields it carries is its kind's:
// nodeFields states each kind's list.
type Node struct {
	NodeType          string   `json:"node_type"`
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	CanonicalPath     string   `json:"canonical_path"`
	CanonicalAnchor   string   `json:"canonical_anchor"`
	Confidence        *float64 `json:"confidence"`
	Solidity          *float64 `json:"solidity"`
	BuildStatus       *string  `json:"build_status"`
	BuildBand         string   `json:"build_band"`
	Rationale         string   `json:"rationale"`
	DependsOnCount    int      `json:"depends_on_count"`
	StrengthenByCount int      `json:"strengthen_by_count"`
	CitationCount     int      `json:"citation_count"`
	Quality           *float64 `json:"quality"`
	Status            string   `json:"status"`
	Strength          *float64 `json:"strength"`
}

// Edge is one depends-on.jsonl record. Fraction is a float64, the pending
// literal or nil.
type Edge struct {
	Source                 string   `json:"source"`
	Target                 string   `json:"target"`
	TargetKind             string   `json:"target_kind"`
	TargetSolidityRecorded *float64 `json:"target_solidity_recorded"`
	Context                *string  `json:"context"`
	Relation               string   `json:"relation"`
	Strength               *float64 `json:"strength"`
	Fraction               any      `json:"fraction"`
}

// StrengthenByItem is one strengthen-by.jsonl record.
type StrengthenByItem struct {
	ClaimID      string   `json:"claim_id"`
	ItemIdx      int      `json:"item_idx"`
	Text         string   `json:"text"`
	MentionedIDs []string `json:"mentioned_ids"`
}

// Citation is one cites.jsonl record.
type Citation struct {
	ClaimID     string `json:"claim_id"`
	LeafPath    string `json:"leaf_path"`
	LeafKind    string `json:"leaf_kind"`
	Tier2Marked bool   `json:"tier2_marked"`
}

// SubtreeAggregate is one subtree-aggregates.jsonl record.
type SubtreeAggregate struct {
	NodePath      string   `json:"node_path"`
	NodeKind      string   `json:"node_kind"`
	SubtreeClaims []string `json:"subtree_claims"`
}

// Index is a KB's derived index as loaded. Nodes are sorted by id, kinds in
// kb.NodeKinds order where ids tie; every other list is in file order.
type Index struct {
	kbRoot       string
	nodes        []Node
	edges        []Edge
	strengthenBy []StrengthenByItem
	cites        []Citation
	aggregates   []SubtreeAggregate
}

// Load reads the derived index of the KB at kbRoot. A missing or unreadable
// file, a line that is not a JSON object of the file's shape, and a node of no
// known kind are an index.Refusal naming each one.
func Load(kbRoot string) (*Index, error) {
	dir := filepath.Join(kbRoot, kb.IndexDir)
	var missing []result.Item
	for _, name := range requiredFiles {
		if p := filepath.Join(dir, name+".jsonl"); !kb.IsFile(p) {
			missing = append(missing, result.Item{Check: checkIndex, Path: p, Remedy: index.RefreshRemedy, Detail: "index file missing: " + p})
		}
	}
	if missing != nil {
		return nil, index.Refusal{Items: missing}
	}
	ix := &Index{kbRoot: kbRoot}
	var bad []result.Item
	read := func(name string, decode func(line []byte) error) {
		bad = append(bad, readJSONL(filepath.Join(dir, name+".jsonl"), decode)...)
	}
	read("claims", func(line []byte) error {
		n := Node{NodeType: "claim"}
		if err := json.Unmarshal(line, &n); err != nil {
			return err
		}
		if !slices.Contains(kb.NodeKinds, n.NodeType) {
			return fmt.Errorf("unknown node_type %q on record id %q; known types: %v", n.NodeType, n.ID, kb.NodeKinds)
		}
		if n.NodeType == "support" {
			n.BuildBand = index.BuildBand(n.Solidity)
		}
		ix.nodes = append(ix.nodes, n)
		return nil
	})
	read("depends-on", func(line []byte) error {
		e := Edge{TargetKind: "claim", Relation: "depends"}
		if err := json.Unmarshal(line, &e); err != nil {
			return err
		}
		ix.edges = append(ix.edges, e)
		return nil
	})
	read("strengthen-by", func(line []byte) error { return appendDecoded(line, &ix.strengthenBy) })
	read("cites", func(line []byte) error { return appendDecoded(line, &ix.cites) })
	read("subtree-aggregates", func(line []byte) error { return appendDecoded(line, &ix.aggregates) })
	if bad != nil {
		return nil, index.Refusal{Items: bad}
	}
	slices.SortStableFunc(ix.nodes, func(a, b Node) int {
		if a.ID != b.ID {
			return strings.Compare(a.ID, b.ID)
		}
		return slices.Index(kb.NodeKinds, a.NodeType) - slices.Index(kb.NodeKinds, b.NodeType)
	})
	return ix, nil
}

func appendDecoded[T any](line []byte, into *[]T) error {
	var v T
	if err := json.Unmarshal(line, &v); err != nil {
		return err
	}
	*into = append(*into, v)
	return nil
}

// readJSONL decodes each non-blank line of a JSONL file, blank meaning
// whitespace only, and returns a problem for each line that is not a JSON
// object it could decode. Lines end at \n alone: the emitter writes U+2028
// and its kin raw inside strings.
func readJSONL(path string, decode func(line []byte) error) []result.Item {
	text, err := kb.ReadText(path)
	if err != nil {
		return []result.Item{{Check: checkIndex, Path: path, Detail: fmt.Sprintf("index file unreadable: %v", err)}}
	}
	var problems []result.Item
	for n, line := range strings.Split(text, "\n") {
		line = kb.Strip(line)
		if line == "" {
			continue
		}
		err := errNotObject
		if line[0] == '{' {
			err = decode([]byte(line))
		}
		if err != nil {
			problems = append(problems, result.Item{Check: checkIndex, Path: path, Line: n + 1, Detail: err.Error()})
		}
	}
	return problems
}

// checkIndex is the refusal class of an index a query cannot load.
const checkIndex = "index"

var errNotObject = errors.New("expected a JSON object")
