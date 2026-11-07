package main

import (
	"flag"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	toolresult "kbase/internal/result"
	"kbase/internal/write"
)

// documentedKeys is each subcommand's own result keys, in their documented
// order: the contract a model reading a result relies on.
var documentedKeys = map[string][]string{
	"build":               {"state-dir", "roles", "through", "no-inference", "resumed", "reset", "stages", "resume", "warnings"},
	"status":              {"state-dir", "state", "pid", "started", "updated", "ended", "no-inference", "stages", "current", "recent-refusals", "recent-fallbacks", "cache-entries", "cache-bytes", "resume"},
	"score-status":        {"state-dir", "state", "pid", "started", "run", "pass", "counts", "scored", "failed-items", "resume"},
	"launch":              {"state", "state-dir", "pid", "run", "ledger", "resume", "wait", "warnings"},
	"cancel":              {"state-dir", "pid", "resume"},
	"build-trail":         {"state-dir", "head", "boundaries", "started-at", "dirty"},
	"init":                {"written", "removed"},
	"update-format":       {"written", "removed", "unfenced", "sheet", "dot", "commits", "uncommitted", "ids", "bare-ids", "no-edge", "strengthen-by"},
	"verify":              {"findings"},
	"write-op":            {"dry-run", "entries", "written", "removed", "sheet", "dot", "commits", "uncommitted"},
	"mapping-query":       {"results"},
	"complete-query":      {"count", "results"},
	"models":              {"provider", "provider-rule", "provider-layer", "models", "warnings"},
	"configure-providers": {"provider", "layer", "written", "models", "next", "warnings"},
	"configure-models":    {"models", "written", "warnings"},
	"show-config":         {"layers", "providers", "config", "selection", "roles", "diagnostics"},
	"probe":               {"provider", "provider-rule", "provider-layer", "probed", "model", "temperature", "thinking", "first-token-ms", "wall-ms", "finish-reason", "reply", "usage", "warnings"},
	"export-latex":        {"written", "volumes", "report"},
	"score":               {"state-dir", "pass", "roles", "run", "ledger", "counts", "ask", "skip", "scored", "resume", "warnings"},
	"auth add":            {"provider"},
	"auth remove":         {"provider"},
	"auth list":           {"providers"},
}

// documentedSubKeys is the keys of each mapping a result nests, in order.
var documentedSubKeys = map[string][]string{
	"build.stages":                {"stage", "commit", "dropped", "report"},
	"status.stages":               {"done", "not-done", "rewind"},
	"build-trail.boundaries":      {"stage", "commit", "subject", "command"},
	"build-trail.started-at":      {"commit", "command"},
	"status.current":              {"stage", "done", "total"},
	"write-op.entries":            {"entry", "operation", "outcome", "document", "line", "id", "heading", "rewritten", "link", "source", "target", "relation", "restored", "removed-edges"},
	"write-op.commits":            {"commit", "label", "paths"},
	"update-format.commits":       {"commit", "label", "paths"},
	"update-format.ids":           {"from", "to"},
	"update-format.bare-ids":      {"path", "line", "id"},
	"update-format.no-edge":       {"id", "text"},
	"update-format.strengthen-by": {"id", "text"},
	"configure-models.models":     {"decision", "text", "scoring"},
	"score.counts":                {"total", "done", "failed", "skipped", "remaining"},
	"score-status.counts":         {"total", "done", "failed", "skipped", "remaining"},
	"probe.usage":                 {"reported", "prompt-tokens", "completion-tokens", "reasoning-tokens", "cached-tokens"},
	"show-config.layers":          {"path", "layer", "exists", "providers-exists", "config-exists"},
	"show-config.providers":       {"name", "base-url", "api", "decision-api", "key-source", "key-file", "upgrade", "layer", "usable", "fault"},
	"show-config.config":          {"models", "sampling"},
	"show-config.selection":       {"provider", "provider-rule", "detail"},
	"build.roles":                 {"role", "provider", "provider-rule", "provider-layer", "model"},
	"score.roles":                 {"role", "provider", "provider-rule", "provider-layer", "model"},
	"show-config.roles":           {"role", "provider", "provider-rule", "provider-layer", "model", "base-url", "sources", "temperature", "thinking", "detail"},
	"migration":                   {"from", "to", "note", "warnings"},
	"build.cancelled":             {"stage", "unit"},
	"score.cancelled":             {"item"},
	"item":                        {"check", "path", "entry", "key", "line", "column", "allowed", "remedy", "detail"},
}

// keysFor is the documented-keys entry a subcommand's result, whose top-level
// keys are keys, is read against. Only a score run's results carry run.
func keysFor(verb string, keys []string) string {
	scoreRun := slices.Contains(keys, "run")
	switch {
	case slices.Contains(write.Ops(), verb) || verb == write.Batch:
		return "write-op"
	case verb == "show" || verb == "stats":
		return "mapping-query"
	case verb == "status" && scoreRun:
		return "score-status"
	case verb == "wait" && scoreRun:
		// wait on a score run prints the run's stored result.
		return "score"
	case verb == "wait":
		// wait prints the status result for the store it waited on.
		return "status"
	case slices.Contains([]string{"find", "citations-of", "edges", "trace"}, verb):
		return "complete-query"
	}
	return verb
}

// parsedResult is one parsed result: its outcome, its keys in order,
// and its values.
type parsedResult struct {
	Outcome string
	Keys    []string
	Values  map[string]any
	Nodes   map[string]*yaml.Node
}

// items is the items under key, each as a mapping.
func (d parsedResult) items(key string) []map[string]any {
	var out []map[string]any
	list, _ := d.Values[key].([]any)
	for _, v := range list {
		m, _ := v.(map[string]any)
		out = append(out, m)
	}
	return out
}

// entryIDs is the id each of a write batch's entries reports, in entry order:
// the node an insert entered or adopted.
func (d parsedResult) entryIDs() []string {
	var ids []string
	for _, e := range d.items("entries") {
		if id, ok := e["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func parseResult(t *testing.T, stdout string) parsedResult {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(stdout), &root); err != nil || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		t.Fatalf("stdout is not one YAML mapping: %v\n%s", err, stdout)
	}
	d := parsedResult{Values: map[string]any{}, Nodes: map[string]*yaml.Node{}}
	m := root.Content[0]
	for i := 0; i < len(m.Content); i += 2 {
		k := m.Content[i].Value
		var v any
		if err := m.Content[i+1].Decode(&v); err != nil {
			t.Fatalf("key %s: %v", k, err)
		}
		d.Keys = append(d.Keys, k)
		d.Values[k] = v
		d.Nodes[k] = m.Content[i+1]
	}
	d.Outcome, _ = d.Values["outcome"].(string)
	return d
}

// mappingKeys is a mapping node's keys in order.
func mappingKeys(n *yaml.Node) []string {
	var keys []string
	for i := 0; i < len(n.Content); i += 2 {
		keys = append(keys, n.Content[i].Value)
	}
	return keys
}

// inOrder reports where keys fall outside documented or out of its order.
func inOrder(keys, documented []string) string {
	at := -1
	for _, k := range keys {
		i := slices.Index(documented, k)
		if i < 0 {
			return "undocumented key " + k
		}
		if i <= at {
			return "key " + k + " out of order"
		}
		at = i
	}
	return ""
}

// checkResult fails t unless stdout parses as verb's result: outcome first,
// kb-root next on every subcommand that resolves a KB, the subcommand's own
// keys in documented order, and the key its outcome closes on, each nested
// mapping and item in its documented order.
func checkResult(t *testing.T, verb, stdout string) parsedResult {
	t.Helper()
	d := parseResult(t, stdout)
	entry := keysFor(verb, d.Keys)
	own, ok := documentedKeys[entry]
	if !ok {
		t.Fatalf("%s documents no result keys", verb)
	}
	keys := d.Keys
	if len(keys) == 0 || keys[0] != "outcome" {
		t.Fatalf("%s: keys %q, want outcome first", verb, keys)
	}
	keys = keys[1:]
	if !slices.Contains([]string{"models", "configure-providers", "configure-models", "show-config", "probe", "auth add", "auth remove", "auth list"}, verb) && len(keys) > 0 && keys[0] == "kb-root" {
		keys = keys[1:]
		// A migrated KB's record is conditional: present after kb-root and
		// nowhere else, so a result carrying one is checked without it.
		if len(keys) > 0 && keys[0] == "migration" {
			keys = keys[1:]
		}
	} else if verb == "init" {
		// A kb-root refusal names a standing kb-root/, so the result must
		// carry the key; one that names none — the usage refusal, the
		// worktree refusal, the lock retry — carries none.
		for _, item := range d.items(toolresult.RefusalsKey) {
			if item["check"] == checkKBRoot {
				t.Fatalf("%s: a refusal names a standing kb-root; the result carries no kb-root key: %q", verb, d.Keys)
			}
		}
	}
	closing := map[string]string{toolresult.Refused: toolresult.RefusalsKey, toolresult.Retry: toolresult.RefusalsKey,
		toolresult.Failed: toolresult.FailuresKey, toolresult.Cancelled: toolresult.CancelledKey}[d.Outcome]
	switch d.Outcome {
	case toolresult.Done, toolresult.Unchanged, toolresult.Bounded:
	case toolresult.Refused, toolresult.Retry, toolresult.Failed, toolresult.Cancelled:
		if len(keys) == 0 || keys[len(keys)-1] != closing {
			t.Fatalf("%s: outcome %s closes on %q, want %s last", verb, d.Outcome, keys, closing)
		}
		keys = keys[:len(keys)-1]
	default:
		t.Fatalf("%s: outcome %q is no outcome", verb, d.Outcome)
	}
	if problem := inOrder(keys, own); problem != "" {
		t.Errorf("%s: %s in %q, documented %q", verb, problem, d.Keys, own)
	}
	nested := func(key, documented string) {
		n := d.Nodes[key]
		if n == nil {
			return
		}
		var maps []*yaml.Node
		switch n.Kind {
		case yaml.MappingNode:
			maps = []*yaml.Node{n}
		case yaml.SequenceNode:
			maps = n.Content
		}
		for _, m := range maps {
			if m.Kind != yaml.MappingNode {
				t.Errorf("%s: %s holds a %v, want mappings", verb, key, m.Kind)
				continue
			}
			if problem := inOrder(mappingKeys(m), documentedSubKeys[documented]); problem != "" {
				t.Errorf("%s: %s: %s in %q", verb, key, problem, mappingKeys(m))
			}
			if documented == "item" {
				checkItem(t, verb, m)
			}
		}
	}
	for _, key := range []string{toolresult.RefusalsKey, toolresult.FailuresKey, "failed-items", "recent-refusals", "recent-fallbacks", "findings", "warnings"} {
		nested(key, "item")
	}
	nested("migration", "migration")
	// Every record the table nests under this verb's result is checked, in the
	// table's own names: a record naming a key the table does not name, or
	// stating its keys out of order, fails here, and a nested record added to
	// the table needs nothing else to be checked. `cancelled` is documented per
	// verb — a build names the stage and unit it stopped at, a score run the
	// item it stopped before — so it is reached through the same loop.
	for _, documented := range slices.Sorted(maps.Keys(documentedSubKeys)) {
		sub, ok := strings.CutPrefix(documented, entry+".")
		if !ok || strings.Contains(sub, ".") {
			continue
		}
		nested(sub, documented)
	}
	return d
}

// checkItem fails t unless an item carries a detail and one of check, path
// and key.
func checkItem(t *testing.T, verb string, m *yaml.Node) {
	t.Helper()
	keys := mappingKeys(m)
	if !slices.Contains(keys, "detail") || !slices.ContainsFunc(keys, func(k string) bool { return k == "check" || k == "path" || k == "key" }) {
		t.Errorf("%s: item %q wants detail and one of check, path, key", verb, keys)
	}
}

var (
	resultFile    = flag.String("result.file", "", "a result ./bin/kbase wrote")
	resultVerb    = flag.String("result.verb", "", "the subcommand that wrote it")
	resultOutcome = flag.String("result.outcome", "", "the outcome it must report")
)

// TestResult parses a result ./bin/kbase wrote against its
// subcommand's documented keys; the integration recipes supply both.
func TestResult(t *testing.T) {
	if *resultFile == "" {
		t.Skip("no result named; the integration recipes supply one")
	}
	b, err := os.ReadFile(*resultFile)
	if err != nil {
		t.Fatal(err)
	}
	d := checkResult(t, *resultVerb, string(b))
	if *resultOutcome != "" && d.Outcome != *resultOutcome {
		t.Errorf("outcome %q, want %q", d.Outcome, *resultOutcome)
	}
}

func TestInOrder(t *testing.T) {
	documented := []string{"a", "b", "c"}
	for _, tc := range []struct {
		keys []string
		ok   bool
	}{
		{[]string{"a", "c"}, true},
		{nil, true},
		{[]string{"c", "a"}, false},
		{[]string{"a", "a"}, false},
		{[]string{"a", "d"}, false},
	} {
		if got := inOrder(tc.keys, documented) == ""; got != tc.ok {
			t.Errorf("inOrder(%q) ok = %t, want %t", tc.keys, got, tc.ok)
		}
	}
	if !strings.Contains(inOrder([]string{"z"}, documented), "undocumented") {
		t.Error("an undocumented key is not named")
	}
}
