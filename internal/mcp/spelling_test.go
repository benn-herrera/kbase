package mcp

import (
	"testing"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
)

// spellingTree is a root holding find, an operation of the query family; build,
// a tool of its own taking repeated volume roots; and auth, which no tool binds.
func spellingTree() *cobra.Command {
	root := &cobra.Command{Use: "kbase"}
	root.PersistentFlags().String("config-dir", "", "")
	root.PersistentFlags().String("project-dir", "", "")
	find := InFamily(Bind(&cobra.Command{Use: "find"}), "query")
	find.Flags().StringArray("kind", nil, "")
	find.Flags().Float64("max", 0, "")
	build := Bind(&cobra.Command{Use: "build [<volume-root>...]"})
	build.Flags().Bool("resume", false, "")
	build.Flags().Bool("no-inference", false, "")
	build.Flags().String("state-dir", "", "")
	build.Flags().Int("ask-concurrency", 0, "")
	batch := Bind(&cobra.Command{Use: "write"})
	batch.Flags().String("values", "", "")
	batch.Flags().Bool("dry-run", false, "")
	batch.Flags().Bool("commit", false, "")
	root.AddCommand(find, build, batch, Exclude(&cobra.Command{Use: "auth"}, "a key arrives on stdin"))
	return root
}

// TestSpellingPipesAWriteBatchAtTheCommandLine: a call of kbase write is, at
// the command line, its batch piped in as a shell user writes it —
// printf's format escaping % and \, the word quoted for the shell — with the
// batch options as flags; over MCP it is the write tool's call, its entries
// carried whole.
func TestSpellingPipesAWriteBatchAtTheCommandLine(t *testing.T) {
	root := spellingTree()
	empty := argmark.Call("write", map[string]any{"entry": []any{}})
	cite := argmark.Call("write", map[string]any{"dry-run": true, "entry": []map[string]any{{
		"operation": "add-citation", "id": "clm-aaaaaa", "document": "it's 100%.md", "line": 7, "snippet": "clm-aaaaaa"}}})
	for _, tc := range []struct {
		name    string
		text    string
		overMCP bool
		want    string
	}{
		{"the empty batch at the command line", empty, false, `printf 'entry: []\n' | kbase write`},
		{"the empty batch over MCP", empty, true, `write {"entry":[]}`},
		{"an entry and an option at the command line", cite, false,
			`printf 'entry:\n  - document: it'"'"'s 100%%.md\n    id: clm-aaaaaa\n    line: 7\n    operation: add-citation\n    snippet: clm-aaaaaa\n' | kbase write --dry-run`},
		{"an entry and an option over MCP", cite, true,
			`write {"dry-run":true,"entry":[{"document":"it's 100%.md","id":"clm-aaaaaa","line":7,"operation":"add-citation","snippet":"clm-aaaaaa"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Spelling(root, tc.overMCP).Spell(tc.text); got != tc.want {
				t.Errorf("Spell =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestSpellingRendersACallForItsSurface(t *testing.T) {
	root := spellingTree()
	findCall := argmark.Call("find", map[string]any{"kind": []string{"claim"}, "max": 0.65})
	recorded := argmark.Recorded("build", []string{"/p a.tex", "--resume", "--no-inference", "--ask-concurrency", "3",
		"--state-dir", "/s", "--config-dir", "/c", "--project-dir", "/r"})
	placeholder := argmark.Call("build", map[string]any{"volume-root": []string{"<volume-root>"}})
	for _, tc := range []struct {
		name    string
		text    string
		overMCP bool
		want    string
	}{
		{"a family member over MCP", findCall, true, `query {"operation":"find","arguments":{"kind":["claim"],"max":0.65}}`},
		{"a family member at the command line", findCall, false, "kbase find --kind claim --max 0.65"},
		{"a recorded argv over MCP, the server's flag left out", recorded, true,
			`build {"ask-concurrency":3,"no-inference":true,"project-dir":"/r","resume":true,"state-dir":"/s","volume-root":["/p a.tex"]}`},
		{"a recorded argv at the command line", recorded, false,
			"kbase build '/p a.tex' --ask-concurrency 3 --config-dir /c --no-inference --project-dir /r --resume --state-dir /s"},
		{"a repeated positional's placeholder at the command line", placeholder, false, "kbase build <volume-root>..."},
		{"a repeated positional's placeholder over MCP", placeholder, true, `build {"volume-root":["<volume-root>"]}`},
		{"a command no tool binds over MCP", argmark.Call("auth", nil), true, "kbase auth"},
		{"a command with no arguments over MCP", argmark.Call("build", nil), true, "build {}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Spelling(root, tc.overMCP).Spell(tc.text); got != tc.want {
				t.Errorf("Spell = %q, want %q", got, tc.want)
			}
		})
	}
}
