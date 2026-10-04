package claimgraph

// The slice's Records check: every reader and placement fact of kb_tools'
// stage B inventory over kbase's tree (tools/slice/README.md is the dump's
// schema) is carried by the inventory the records alone give, item by item in
// scan order; and every record names a document of that tree. The recipe
// test-integration-slice-arxiv supplies the flags; without them it skips.

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/records"
)

var (
	sliceOut     = flag.String("slice.out", "", "the slice evidence directory holding one directory per paper")
	sliceCompare = flag.String("slice.compare", "", "space-separated paper ids whose records are checked")
)

// sliceRecordsFile is each paper's Records verdict, beside its comparison.yaml.
const sliceRecordsFile = "records.yaml"

// inventoryLists are the inventory's readings, in report order.
var inventoryLists = []string{"blocks", "fences", "anchors", "citations", "works", "proofs"}

type dumpRoot struct {
	Role      string                      `json:"role"`
	Paper     string                      `json:"paper"`
	Tree      []struct{ Path string }     `json:"tree"`
	Inventory map[string][]map[string]any `json:"inventory"`
}

type sliceCheck struct {
	Check       string   `yaml:"check"`
	Passed      bool     `yaml:"passed"`
	Evidence    []string `yaml:"evidence"`
	Differences []string `yaml:"differences,omitempty"`
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// recordFields are the reader and placement fields of the README's "Coverage
// for the Records check", read off the inventory the records alone give.
func recordFields(inv Inventory) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, b := range inv.Blocks {
		out["blocks"] = append(out["blocks"], map[string]any{"document": b.Document, "environment": b.Environment, "identifier": nullable(b.Identifier)})
	}
	for _, f := range inv.Fences {
		out["fences"] = append(out["fences"], map[string]any{"document": f.Document, "labels": f.Labels})
	}
	for _, a := range inv.Anchors {
		out["anchors"] = append(out["anchors"], map[string]any{"document": a.Document, "reference_type": a.ReferenceType, "label": a.Label,
			"hosting_environment": nullable(a.HostingEnvironment), "href": a.Href, "target": nullable(a.Target), "fragment": a.Fragment})
	}
	for _, c := range inv.Citations {
		out["citations"] = append(out["citations"], map[string]any{"document": c.Document, "key": c.Key, "state": c.State})
	}
	for _, w := range inv.Works {
		out["works"] = append(out["works"], map[string]any{"document": w.Document, "key": w.Key})
	}
	for _, p := range inv.Proofs {
		head := [][]string{}
		for _, h := range p.Head {
			head = append(head, []string{h[0], h[1]})
		}
		out["proofs"] = append(out["proofs"], map[string]any{"document": p.Document, "head": head})
	}
	return out
}

func canonical(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<unencodable: %v>", err)
	}
	return string(data)
}

// compareRecords: the dumped inventory's reader and placement fields equal
// the record inventory's, item by item.
func compareRecords(recs []records.Record, kbase *dumpRoot) []string {
	if kbase.Inventory == nil {
		return []string{"kbase inventory is missing from the dump"}
	}
	fromRecords := recordFields(recordInventory(recs))
	var diffs []string
	for _, list := range inventoryLists {
		items, inv := fromRecords[list], kbase.Inventory[list]
		if len(items) != len(inv) {
			diffs = append(diffs, fmt.Sprintf("%s: records give %d items, the inventory %d", list, len(items), len(inv)))
		}
		for i := range min(len(items), len(inv)) {
			fields := make([]string, 0, len(items[i]))
			for f := range items[i] {
				fields = append(fields, f)
			}
			slices.Sort(fields)
			for _, f := range fields {
				if want, got := canonical(inv[i][f]), canonical(items[i][f]); want != got {
					diffs = append(diffs, fmt.Sprintf("%s[%d].%s (%s): inventory %s, records %s", list, i, f, inv[i]["document"], want, got))
				}
			}
		}
	}
	return diffs
}

// recordJoin: every record names a document of kbase's tree.
func recordJoin(recs []records.Record, kbase *dumpRoot) []string {
	present := map[string]bool{}
	for _, e := range kbase.Tree {
		present[e.Path] = true
	}
	var diffs []string
	for _, r := range recs {
		if !present[r.Document] {
			diffs = append(diffs, fmt.Sprintf("record %d (%s) names %q, no document of the tree", r.Order, r.Kind, r.Document))
		}
	}
	return diffs
}

func kbaseRoot(dump, paper string) (*dumpRoot, error) {
	data, err := os.ReadFile(dump)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Roots []dumpRoot `json:"roots"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", dump, err)
	}
	for i := range doc.Roots {
		if doc.Roots[i].Paper == paper && doc.Roots[i].Role == "kbase" {
			return &doc.Roots[i], nil
		}
	}
	return nil, fmt.Errorf("%s holds no kbase root for %s", dump, paper)
}

func newSliceCheck(name string, evidence, diffs []string) sliceCheck {
	return sliceCheck{Check: name, Passed: len(diffs) == 0, Evidence: evidence, Differences: diffs}
}

func TestSliceRecords(t *testing.T) {
	if *sliceOut == "" || strings.TrimSpace(*sliceCompare) == "" {
		t.Skip("no slice evidence named; test-integration-slice-arxiv supplies it")
	}
	for _, id := range strings.Fields(*sliceCompare) {
		t.Run(id, func(t *testing.T) {
			dir := filepath.Join(*sliceOut, id)
			dump, file := filepath.Join(dir, "dump.json"), records.Path(filepath.Join(dir, "state-1"))
			var checks []sliceCheck
			root, err := kbaseRoot(dump, id)
			recs, rerr := records.Read(file)
			switch {
			case err != nil:
				checks = append(checks, newSliceCheck("records", []string{dump}, []string{err.Error()}))
			case rerr != nil:
				checks = append(checks, newSliceCheck("records", []string{file}, []string{rerr.Error()}))
			default:
				checks = append(checks, newSliceCheck("records", []string{file, dump}, compareRecords(recs, root)),
					newSliceCheck("record join", []string{file, dump}, recordJoin(recs, root)))
			}
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(checks); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, sliceRecordsFile), buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range checks {
				if !c.Passed {
					t.Errorf("%s: %s failed — see %s", id, c.Check, filepath.Join(dir, sliceRecordsFile))
				}
			}
		})
	}
}
