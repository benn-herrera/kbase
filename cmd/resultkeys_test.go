package main

import (
	"flag"
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
	"build":              {"state-dir", "through", "no-inference", "resumed", "restored", "stages", "resume"},
	"status":             {"state-dir", "state", "pid", "started", "updated", "ended", "no-inference", "stages", "current", "recent-refusals", "recent-fallbacks", "resume"},
	"cancel":             {"state-dir", "pid", "resume"},
	"refresh":            {"written"},
	"verify":             {},
	"write-op":           {"ids", "minted", "adopted", "written", "refreshed"},
	"render-citation":    {"citations"},
	"list-query":         {"count", "offset", "truncated", "results"},
	"mapping-query":      {"results"},
	"render-claim-graph": {"written", "sheet"},
	"models":             {"provider", "models"},
	"configure":          {"provider", "models", "written"},
}

// documentedSubKeys is the keys of each mapping a result nests, in order.
var documentedSubKeys = map[string][]string{
	"build.stages":     {"stage", "commit", "dropped", "report"},
	"status.stages":    {"stage", "commit"},
	"status.current":   {"stage", "units-done", "units-total"},
	"write-op.adopted": {"entry", "id", "differs"},
	"configure.models": {"heavy", "light"},
	"cancelled":        {"stage", "unit"},
	"item":             {"check", "path", "entry", "key", "line", "column", "allowed", "remedy", "detail"},
}

// keysFor is the documented-keys entry a subcommand's result is read
// against.
func keysFor(verb string) string {
	switch {
	case slices.Contains(write.Ops(), verb):
		return "write-op"
	case verb == "show" || verb == "stats":
		return "mapping-query"
	case slices.Contains([]string{"deps", "gated-on", "cited-by", "find", "referenced-by", "solidity-below", "subtree", "weak-points"}, verb):
		return "list-query"
	}
	return verb
}

// resultDoc is one parsed result document: its outcome, its keys in order,
// and its values.
type resultDoc struct {
	Outcome string
	Keys    []string
	Values  map[string]any
	Nodes   map[string]*yaml.Node
}

// items is the items under key, each as a mapping.
func (d resultDoc) items(key string) []map[string]any {
	var out []map[string]any
	list, _ := d.Values[key].([]any)
	for _, v := range list {
		m, _ := v.(map[string]any)
		out = append(out, m)
	}
	return out
}

func parseResult(t *testing.T, stdout string) resultDoc {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(stdout), &root); err != nil || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		t.Fatalf("stdout is not one YAML mapping: %v\n%s", err, stdout)
	}
	d := resultDoc{Values: map[string]any{}, Nodes: map[string]*yaml.Node{}}
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

// checkDocument fails t unless stdout parses as verb's result: outcome first,
// kb-root next on every subcommand that resolves a KB, the subcommand's own
// keys in documented order, and the key its outcome closes on, each nested
// mapping and item in its documented order.
func checkDocument(t *testing.T, verb, stdout string) resultDoc {
	t.Helper()
	d := parseResult(t, stdout)
	entry := keysFor(verb)
	own, ok := documentedKeys[entry]
	if !ok {
		t.Fatalf("%s documents no result keys", verb)
	}
	keys := d.Keys
	if len(keys) == 0 || keys[0] != "outcome" {
		t.Fatalf("%s: keys %q, want outcome first", verb, keys)
	}
	keys = keys[1:]
	if verb != "models" && verb != "configure" && len(keys) > 0 && keys[0] == "kb-root" {
		keys = keys[1:]
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
	for _, key := range []string{toolresult.RefusalsKey, toolresult.FailuresKey, "recent-refusals", "recent-fallbacks"} {
		nested(key, "item")
	}
	nested(toolresult.CancelledKey, "cancelled")
	for _, sub := range []string{"stages", "current", "adopted", "models"} {
		if _, ok := documentedSubKeys[entry+"."+sub]; ok {
			nested(sub, entry+"."+sub)
		}
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
	resultDocument = flag.String("result.doc", "", "a result document ./bin/kbase wrote")
	resultVerb     = flag.String("result.verb", "", "the subcommand that wrote it")
	resultOutcome  = flag.String("result.outcome", "", "the outcome it must report")
)

// TestResultDocument parses a result document ./bin/kbase wrote against its
// subcommand's documented keys; the integration recipes supply both.
func TestResultDocument(t *testing.T) {
	if *resultDocument == "" {
		t.Skip("no result document named; the integration recipes supply one")
	}
	b, err := os.ReadFile(*resultDocument)
	if err != nil {
		t.Fatal(err)
	}
	d := checkDocument(t, *resultVerb, string(b))
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
