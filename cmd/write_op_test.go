package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
)

// TestAnEntryReportsItsOpsKeys: an add's, an edit's and a move-node entry in
// a batch result carry entry, operation, outcome and id; an edge op's entry carries source,
// target and relation, and a restore-demoted entry also restored; a
// remove-node entry carries id and removed-edges, each an edges row; an
// add-citation entry carries document, line, id and link.
func TestAnEntryReportsItsOpsKeys(t *testing.T) {
	edge := write.EntryResult{Outcome: toolresult.Done, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: "supports"}
	edgeEntry := func(n int, op string) write.EntryResult {
		e := edge
		e.Entry, e.Operation = n, op
		return e
	}
	res := write.Result{Entries: []write.EntryResult{
		{Entry: 1, Operation: write.AddClaim, Outcome: toolresult.Done, ID: "clm-aaaaaa"},
		{Entry: 2, Operation: "edit-claim", Outcome: toolresult.Unchanged, ID: "clm-aaaaaa"},
		edgeEntry(3, "add-edge"), edgeEntry(4, "edit-edge"), edgeEntry(5, "remove-edge"),
		{Entry: 6, Operation: write.RestoreDemoted, Outcome: toolresult.Done, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: "demoted",
			Restored: []string{"supports"}},
		{Entry: 7, Operation: "remove-node", Outcome: toolresult.Done, ID: "clm-aaaaaa",
			RemovedEdges: []kb.EdgeRow{{Source: "clm-bbbbbb", Target: "clm-aaaaaa", Relation: "supports", Bearing: "0.4"}}},
		{Entry: 8, Operation: "move-node", Outcome: toolresult.Done, ID: "clm-aaaaaa"},
		{Entry: 9, Operation: "add-citation", Outcome: toolresult.Done, ID: "clm-aaaaaa", Document: "a.md", Line: 3,
			Link: `[The Result](b.md#b "clm-aaaaaa")`},
	}}
	var entries []toolresult.Record
	for _, f := range batchFields(res, write.Batch, io.Discard) {
		if f.Key == "entries" {
			entries = f.Value.([]toolresult.Record)
		}
	}
	keys := func(r toolresult.Record) []string {
		var out []string
		for _, f := range r {
			out = append(out, f.Key)
		}
		return out
	}
	for i, want := range [][]string{{"entry", "operation", "outcome", "id"}, {"entry", "operation", "outcome", "id"},
		{"entry", "operation", "outcome", "source", "target", "relation"}, {"entry", "operation", "outcome", "source", "target", "relation"},
		{"entry", "operation", "outcome", "source", "target", "relation"},
		{"entry", "operation", "outcome", "source", "target", "relation", "restored"},
		{"entry", "operation", "outcome", "id", "removed-edges"}, {"entry", "operation", "outcome", "id"},
		{"entry", "operation", "outcome", "document", "line", "id", "link"}} {
		if i >= len(entries) || !slices.Equal(keys(entries[i]), want) {
			t.Errorf("entry %d keys = %v, want %v", i+1, entries, want)
		}
	}
	if len(entries) < 7 || len(entries[6]) < 5 {
		return
	}
	rows, _ := entries[6][4].Value.([]toolresult.Record)
	if want := []string{"source", "target", "relation", "bearing", "context", "manually_scored", "replaced_relation"}; len(rows) != 1 || !slices.Equal(keys(rows[0]), want) {
		t.Errorf("removed-edges = %v, want one edges row keyed %v", rows, want)
	}
}

// TestARealBatchReportsItsOpsKeys: kbase write batches that add two claims,
// cite the first in another leaf in the naming form and move its marker, then
// remove the second, report each move-node, add-citation and remove-node entry
// with the keys and values of its row: the citation's link the text its leaf
// now holds, and the removal's removed-edges empty.
func TestARealBatchReportsItsOpsKeys(t *testing.T) {
	repo := committedKB(t)
	writeRepo(t, repo, map[string]string{
		"kb-root/entry-point.md": "# KB\n\n- [a](a.md)\n- [b](b.md)\n",
		"kb-root/a.md":           "[↑ KB](entry-point.md)\n\n# A\n\nThe first result.\n\nThe second result.\n\nThe first result again.\n",
		"kb-root/b.md":           "[↑ KB](entry-point.md)\n\n# B\n\nNothing yet.\n",
	})
	added := writeBatch(t, repo, write.Batch, "entry:\n"+
		"- operation: add-claim\n  title: First Result\n  rationale: Shown.\n  document: a.md\n  line: 5\n  snippet: first result\n"+
		"- operation: add-claim\n  title: Second Result\n  rationale: Shown.\n  document: a.md\n  line: 7\n  snippet: second result\n", false)
	ids := added.entryIDs()
	if added.Outcome != toolresult.Done || len(ids) != 2 {
		t.Fatalf("adds: %s\n%v", added.Outcome, added.Values)
	}
	first, second := ids[0], ids[1]
	writeRepo(t, repo, map[string]string{"kb-root/b.md": "[↑ KB](entry-point.md)\n\n# B\n\nAs " + first + " shows, it holds.\n"})

	type entryWant struct {
		keys   []string
		values map[string]any
	}
	// check fails t unless d is done and its entries' keys and values are
	// want's, in order.
	check := func(d resultDoc, want ...entryWant) {
		t.Helper()
		n, items := d.Nodes["entries"], d.items("entries")
		if d.Outcome != toolresult.Done || n == nil || len(n.Content) != len(want) || len(items) != len(want) {
			t.Fatalf("want %d done entries: %s\n%v", len(want), d.Outcome, d.Values)
		}
		for i, w := range want {
			if keys := mappingKeys(n.Content[i]); !slices.Equal(keys, w.keys) {
				t.Errorf("entry %d keys = %q, want %q", i+1, keys, w.keys)
			}
			for k, v := range w.values {
				if !reflect.DeepEqual(items[i][k], v) {
					t.Errorf("entry %d %s = %#v, want %#v", i+1, k, items[i][k], v)
				}
			}
		}
	}

	link := "[First Result](a.md#a \"" + first + "\")"
	check(writeBatch(t, repo, write.Batch, "entry:\n"+
		"- operation: add-citation\n  id: "+first+"\n  document: b.md\n  line: 5\n  snippet: "+first+"\n"+
		"- operation: move-node\n  id: "+first+"\n  document: a.md\n  line: 9\n  snippet: first result again\n", false),
		entryWant{[]string{"entry", "operation", "outcome", "document", "line", "id", "link"},
			map[string]any{"entry": 1, "operation": write.AddCitation, "outcome": toolresult.Done, "document": "b.md", "line": 5, "id": first, "link": link}},
		entryWant{[]string{"entry", "operation", "outcome", "id"},
			map[string]any{"entry": 2, "operation": "move-node", "outcome": toolresult.Done, "id": first}})
	b, err := os.ReadFile(filepath.Join(repo, "kb-root", "b.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "As " + link + " shows, it holds."; !strings.Contains(string(b), want) {
		t.Errorf("b.md =\n%s\nwant it to hold %q", b, want)
	}

	check(writeBatch(t, repo, write.Batch, "entry:\n- operation: remove-node\n  id: "+second+"\n", false),
		entryWant{[]string{"entry", "operation", "outcome", "id", "removed-edges"},
			map[string]any{"entry": 1, "operation": "remove-node", "outcome": toolresult.Done, "id": second, "removed-edges": []any{}}})
}

// TestKBTextPassesThroughUnspelled: a claim whose title holds what reads as an
// argument marker, added through a batch, comes back byte-identical in find's
// and show's results at the command line and over MCP, text and structured
// content alike, while a refusal's marked flag in the same session is still
// spelled as its surface takes it.
func TestKBTextPassesThroughUnspelled(t *testing.T) {
	repo := committedKB(t)
	writeRepo(t, repo, map[string]string{"kb-root/a.md": "[↑ KB](entry-point.md)\n\n# A\n\nClaim text.\n"})
	title := "A " + string(rune(0xE000)) + "-x" + string(rune(0xE001)) + " Result"
	added := writeBatch(t, repo, write.Batch, "entry:\n- operation: add-claim\n  title: \""+title+"\"\n  rationale: Shown.\n"+
		"  document: a.md\n  line: 5\n  snippet: Claim text.\n", false)
	if added.Outcome != toolresult.Done || len(added.entryIDs()) != 1 {
		t.Fatalf("add-claim: %s\n%v", added.Outcome, added.Values)
	}
	id := added.entryIDs()[0]
	// titleOf is the title of id's row among results, a mapping or a list of
	// them.
	titleOf := func(results any) any {
		rows, ok := results.([]any)
		if !ok {
			rows = []any{results}
		}
		for _, r := range rows {
			if row, _ := r.(map[string]any); row["id"] == id {
				return row["title"]
			}
		}
		return nil
	}

	srv := servedServer(rootCmd, mcp.LocatedAt(repo), &bytes.Buffer{}, log.Discard())
	for _, q := range []struct {
		op   string
		argv []string
		args map[string]any
	}{{"find", []string{"find"}, nil}, {"show", []string{"show", id}, map[string]any{"id": id}}} {
		out, _ := kbase(t, repo, q.argv...)
		if got := titleOf(checkDocument(t, q.op, out).Values["results"]); got != title || !strings.Contains(out, title) {
			t.Errorf("%s at the command line: title %q, want %q as it stands\n%s", q.op, got, title, out)
		}
		name, args := asOperation(t, q.op, q.args)
		text, doc, _ := callToolResult(t, srv, name, args)
		if got := titleOf(doc["results"]); got != title || !strings.Contains(text, title) {
			t.Errorf("%s over MCP: title %q, want %q as it stands\n%s", q.op, got, title, text)
		}
	}

	doc, _ := callOperation(t, srv, "find", map[string]any{"limit": 1})
	requireSpelled(t, "mcp", refusalItem(t, doc), "limit", map[string][]string{"detail": {"limit", "sort"}})
	out, _ := kbase(t, repo, "find", "--limit", "1")
	requireSpelled(t, "cli", cliItem(t, "find", out), "--limit", map[string][]string{"detail": {"limit", "sort"}})
}

// TestAnMCPRefusalNamingADocumentKeepsItsLine: a batch run as the MCP write
// tool runs it (the batch as the arguments' JSON, ArgumentEntry set) has a
// remove-node refused for a citation in a.md; the item naming a.md keeps that
// line's number, and a refusal of an unknown top-level key, which names no
// document, carries no line.
func TestAnMCPRefusalNamingADocumentKeepsItsLine(t *testing.T) {
	repo := committedKB(t)
	writeRepo(t, repo, map[string]string{"kb-root/a.md": "[↑ KB](entry-point.md)\n\n# A\n\nClaim text.\n\nSee Claim text.\n"})
	mcpBatch := func(batch string) resultDoc {
		t.Helper()
		var out bytes.Buffer
		if _, err := runWriteOp(writeOpOptions{Op: write.Batch, WorkDir: repo, Stdin: strings.NewReader(batch), ArgumentEntry: true,
			Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()}); err != nil {
			t.Fatal(err)
		}
		return checkDocument(t, write.Batch, out.String())
	}
	added := mcpBatch(`{"entry":[{"operation":"add-claim","title":"A Result","rationale":"Shown.","document":"a.md","line":5,"snippet":"Claim text."}]}`)
	if added.Outcome != toolresult.Done || len(added.entryIDs()) != 1 {
		t.Fatalf("add-claim: %s\n%v", added.Outcome, added.Values)
	}
	id := added.entryIDs()[0]
	cited := mcpBatch(`{"entry":[{"operation":"add-citation","document":"a.md","line":7,"snippet":"Claim text","id":"` + id + `"}]}`)
	if cited.Outcome != toolresult.Done {
		t.Fatalf("add-citation: %s\n%v", cited.Outcome, cited.Values)
	}

	removed := mcpBatch(`{"entry":[{"operation":"remove-node","id":"` + id + `"}]}`)
	if removed.Outcome != toolresult.Refused {
		t.Fatalf("remove-node of a cited claim: %s\n%v", removed.Outcome, removed.Values)
	}
	var found bool
	for _, r := range removed.items("refusals") {
		if r["path"] == "a.md" {
			found = true
			if r["line"] != 7 {
				t.Errorf("refusal naming a.md has line %v, want 7", r["line"])
			}
		}
	}
	if !found {
		t.Errorf("no refusal names a.md: %v", removed.Values["refusals"])
	}

	unknown := mcpBatch(`{"entry":[],"bogus":1}`)
	if unknown.Outcome != toolresult.Refused || len(unknown.items("refusals")) == 0 {
		t.Fatalf("an unknown top-level key: %s\n%v", unknown.Outcome, unknown.Values)
	}
	for _, r := range unknown.items("refusals") {
		if _, has := r["line"]; has {
			t.Errorf("a refusal naming no path carries line %v", r["line"])
		}
	}
}
