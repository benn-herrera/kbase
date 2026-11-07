package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
)

// atTheCommandLine is marked text as a result at the command line
// spells it.
func atTheCommandLine(text string) string { return mcp.Spelling(rootCmd, false).Spell(text) }

// cliSpelled matches name as the command line spells a flag, --name, standing
// as a word of its own.
func cliSpelled(name string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9-])--` + regexp.QuoteMeta(name) + `($|[^A-Za-z0-9-])`)
}

// bareWord matches name standing as a word of its own, as MCP spells an
// argument.
func bareWord(name string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9-])` + regexp.QuoteMeta(name) + `($|[^A-Za-z0-9-])`)
}

// mcpArguments is every argument a tool the server lists takes, the query
// family's members' included, and the positional ones of each command whose
// tool takes them.
func mcpArguments(t *testing.T, srv mcp.Server) (names []string, positionals map[*cobra.Command][]string) {
	t.Helper()
	positionals = map[*cobra.Command][]string{}
	add := func(name string) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, tool := range toolsListedBy(t, srv) {
		for name := range tool.InputSchema.Properties {
			add(name)
		}
	}
	for _, c := range boundCommandsOf(rootCmd) {
		if c.Annotations[mcp.FamilyAnnotation] == write.Batch {
			continue
		}
		if fields, ok := write.OpVocabulary(c.Name()); ok {
			for _, f := range fields {
				add(f.Name)
			}
			continue
		}
		declared, schema, err := mcp.CommandSchema(c)
		if err != nil {
			t.Fatal(err)
		}
		for name := range schema.Properties {
			add(name)
		}
		for _, p := range declared {
			positionals[c] = append(positionals[c], p.Name)
		}
	}
	return names, positionals
}

// TestNoCLISpellingInMCPRenderings: no text an MCP reader receives as help —
// a tool's description, its schema's descriptions, kbase://help/<command> of
// every bound command, kbase://help/query — spells an argument any listed
// tool takes as the command line does, --name, and none of a positional
// command's renderings spells its positional <name>.
func TestNoCLISpellingInMCPRenderings(t *testing.T) {
	srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
	names, positionals := mcpArguments(t, srv)
	renderings := map[string]string{mcp.HelpURIPrefix + queryFamily: readResource(t, srv, mcp.HelpURIPrefix+queryFamily)}
	for _, tool := range toolsListedBy(t, srv) {
		renderings["the "+tool.Name+" tool's description"] = tool.Description
		for name, p := range tool.InputSchema.Properties {
			description, _ := p["description"].(string)
			renderings["the "+tool.Name+" tool's "+name] = description
		}
	}
	commandRenderings := map[*cobra.Command][]string{}
	for _, c := range boundCommandsOf(rootCmd) {
		uri := mcp.HelpURIPrefix + c.Name()
		renderings[uri] = readResource(t, srv, uri)
		commandRenderings[c] = append(commandRenderings[c], renderings[uri])
		_, schema, err := mcp.CommandSchema(c)
		if err != nil {
			t.Fatal(err)
		}
		for name, p := range schema.Properties {
			renderings["the "+c.Name()+" argument "+name] = p.Description
			commandRenderings[c] = append(commandRenderings[c], p.Description)
		}
	}
	for where, text := range renderings {
		for _, name := range names {
			if cliSpelled(name).MatchString(text) {
				t.Errorf("%s spells --%s:\n%s", where, name, text)
			}
		}
	}
	for c, ps := range positionals {
		for _, text := range commandRenderings[c] {
			for _, p := range ps {
				if strings.Contains(text, "<"+p+">") {
					t.Errorf("%s's MCP rendering spells <%s>:\n%s", c.Name(), p, text)
				}
			}
		}
	}
}

// TestMarkedArgumentsRenderOnBothSurfaces: every argument a bound command's
// help marks is spelled --name or <name> in `kbase <command> --help` and name
// in kbase://help/<command> — a mark in its usage line's flags in the usage
// block and in the schema's description, one in its summary in `kbase --help`
// and its tool's description — and no rendering on either surface keeps a
// marker.
func TestMarkedArgumentsRenderOnBothSurfaces(t *testing.T) {
	srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
	descriptions := map[string]string{}
	for _, tool := range toolsListedBy(t, srv) {
		descriptions[tool.Name] = tool.Description
	}
	dir := realPath(t, t.TempDir())
	rootHelp, _ := kbase(t, dir, "--help")
	cli := func(a argmark.Argument) string {
		if a.Positional {
			return "<" + a.Name + ">"
		}
		return "--" + a.Name
	}
	marked := 0
	for _, c := range boundCommandsOf(rootCmd) {
		printed, _ := kbase(t, dir, c.Name(), "--help")
		resource := readResource(t, srv, mcp.HelpURIPrefix+c.Name())
		_, schema, err := mcp.CommandSchema(c)
		if err != nil {
			t.Fatal(err)
		}
		for where, text := range map[string]string{"--help": printed, "the resource": resource, "the description": descriptions[c.Name()]} {
			if argmark.Marked(text) {
				t.Errorf("%s's %s keeps a marker:\n%s", c.Name(), where, text)
			}
		}
		// Each source of the command's help, with the rendering of each surface
		// that reads it; a family member has no tool description of its own.
		type source struct {
			text              string
			cliWhere, cliText string
			mcpWhere, mcpText string
		}
		description := "its tool's description"
		if c.Annotations[mcp.FamilyAnnotation] != "" {
			description = ""
		}
		sources := []source{
			{c.Long, "--help", printed, "its resource", resource},
			{c.Annotations[mcp.MCPNoteAnnotation], "", "", "its resource", resource},
			{c.Annotations[mcp.CLINoteAnnotation], "--help", printed, "", ""},
			{c.Short, "kbase --help", rootHelp, description, descriptions[c.Name()]},
		}
		c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if p, ok := schema.Properties[f.Name]; ok {
				sources = append(sources, source{f.Usage, "--help", printed, "the " + f.Name + " description", p.Description})
			}
		})
		for _, s := range sources {
			for _, a := range argmark.Arguments(s.text) {
				marked++
				if s.cliWhere != "" && !strings.Contains(s.cliText, cli(a)) {
					t.Errorf("%s marks %s, which %s does not spell %s", c.Name(), a.Name, s.cliWhere, cli(a))
				}
				if s.mcpWhere != "" && (!bareWord(a.Name).MatchString(s.mcpText) || strings.Contains(s.mcpText, cli(a))) {
					t.Errorf("%s marks %s, which %s does not spell %s", c.Name(), a.Name, s.mcpWhere, a.Name)
				}
			}
		}
	}
	if marked == 0 {
		t.Fatal("no bound command's help marks an argument")
	}
}

// TestEveryMarkerNamesAnArgument: every argument any command's help — its
// summary, its body, its notes, its flags' usages — or a family's text marks
// is one some command takes: a flag of the tree, or a positional its usage
// line declares.
func TestEveryMarkerNamesAnArgument(t *testing.T) {
	flags, positionals := map[string]bool{}, map[string]bool{}
	var texts []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		texts = append(texts, c.Short, c.Long, c.Annotations[mcp.MCPNoteAnnotation], c.Annotations[mcp.CLINoteAnnotation])
		for _, set := range []*pflag.FlagSet{c.LocalFlags(), c.PersistentFlags()} {
			set.VisitAll(func(f *pflag.Flag) {
				flags[f.Name] = true
				texts = append(texts, f.Usage)
			})
		}
		if declared, err := mcp.DeclaredPositionals(c); err == nil {
			for _, p := range declared {
				positionals[p.Name] = true
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	for _, f := range toolFamilies() {
		texts = append(texts, f.Summary, f.Help)
	}
	for _, text := range texts {
		for _, a := range argmark.Arguments(text) {
			if a.Positional && !positionals[a.Name] || !a.Positional && !flags[a.Name] {
				t.Errorf("a help text marks %+v, which no command takes:\n%s", a, text)
			}
		}
	}
}

// TestNoCLIOutputCarriesAMarker: what cobra prints itself — the shell
// completion choices with their descriptions, each command's summary and
// each flag's usage, and the completion scripts — and every command's --help
// carry no marker: each reaches the shell as the command line spells it.
func TestNoCLIOutputCarriesAMarker(t *testing.T) {
	dir := realPath(t, t.TempDir())
	invocations := [][]string{{"__complete", ""}, {"__complete", "--"}, {"__completeNoDesc", ""}, {"--help"}}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		invocations = append(invocations, []string{"completion", shell})
	}
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		for _, sub := range c.Commands() {
			p := append(slices.Clone(path), sub.Name())
			invocations = append(invocations, append(append([]string{"__complete"}, p...), "--"),
				append(append([]string{"__complete"}, p...), ""), append(slices.Clone(p), "--help"))
			walk(sub, p)
		}
	}
	walk(rootCmd, nil)
	// cobra adds its completion request command to the tree the call is made
	// in and leaves it there, which the tool table's tests would read.
	t.Cleanup(func() {
		for _, c := range rootCmd.Commands() {
			if c.Name() == cobra.ShellCompRequestCmd {
				rootCmd.RemoveCommand(c)
			}
		}
	})
	for _, args := range invocations {
		out, _ := kbase(t, dir, args...)
		if argmark.Marked(out) {
			t.Errorf("kbase %q prints a marker:\n%s", args, out)
		}
	}
}

// refusalItem is the one item a refused result closes on.
func refusalItem(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	items, _ := doc["refusals"].([]any)
	if doc["outcome"] != toolresult.Refused || len(items) == 0 {
		t.Fatalf("want a refusal: %v", doc)
	}
	item, _ := items[0].(map[string]any)
	return item
}

// requireSpelled: the item's key is key and each of the texts named, its
// detail and its remedy among them, names every one of names, all spelled for
// the surface — bare over MCP, --name at the command line.
func requireSpelled(t *testing.T, surface string, item map[string]any, key string, names map[string][]string) {
	t.Helper()
	if item["key"] != key {
		t.Errorf("%s: key %v, want %s: %v", surface, item["key"], key, item)
	}
	for field, want := range names {
		text, _ := item[field].(string)
		for _, name := range want {
			if surface == "mcp" && (!bareWord(name).MatchString(text) || cliSpelled(name).MatchString(text)) ||
				surface == "cli" && !cliSpelled(name).MatchString(text) {
				t.Errorf("%s: the %s %q does not name %s as the surface spells it", surface, field, text, name)
			}
		}
	}
}

// cliItem is the first refusal of the CLI result out.
func cliItem(t *testing.T, verb, out string) map[string]any {
	t.Helper()
	items := checkResult(t, verb, out).items("refusals")
	if len(items) == 0 {
		t.Fatalf("%s: want a refusal\n%s", verb, out)
	}
	return items[0]
}

// TestARefusalNamesItsArgumentAsItsSurfaceTakesIt: one refusal on each path a
// result takes to its reader names its argument, in key and in prose, bare
// over MCP and --name or <name> at the command line — a family operation, a
// command tool run in-process, the build tool's own refusal before it starts
// a run, and status re-emitting a recorded item and the command that resumes
// the run.
func TestARefusalNamesItsArgumentAsItsSurfaceTakesIt(t *testing.T) {
	t.Run("a family operation", func(t *testing.T) {
		srv, _ := mcpServer(t)
		repo, _ := srv.Located.Root()
		doc, _ := callOperation(t, srv, "find", map[string]any{"limit": 1})
		requireSpelled(t, "mcp", refusalItem(t, doc), "limit", map[string][]string{"detail": {"limit", "sort"}})
		out, _ := kbase(t, repo, "find", "--limit", "1")
		requireSpelled(t, "cli", cliItem(t, "find", out), "--limit", map[string][]string{"detail": {"limit", "sort"}})
	})
	t.Run("a command tool in-process", func(t *testing.T) {
		dir := exportDir(t)
		srv := servedServer(rootCmd, mcp.LocatedAt(dir), &bytes.Buffer{}, log.Discard())
		doc, _ := callTool(t, srv, "export-latex", map[string]any{"volume": []any{"nope"}})
		requireSpelled(t, "mcp", refusalItem(t, doc), "volume", map[string][]string{"detail": {"volume"}})
		out, _ := kbase(t, dir, "export-latex", "--volume", "nope")
		requireSpelled(t, "cli", cliItem(t, "export-latex", out), "--volume", map[string][]string{"detail": {"volume"}})
	})
	t.Run("the build tool before it starts a run", func(t *testing.T) {
		repo := realPath(t, t.TempDir())
		gitRepo(t, repo)
		state := realPath(t, t.TempDir())
		writeValues(t, state, buildstore.StoreFormatFile, "99\n")
		srv := servedServer(rootCmd, mcp.LocatedAt(repo), &bytes.Buffer{}, log.Discard())
		doc, _ := callTool(t, srv, "build", map[string]any{"volume-root": []any{"paper.tex"}, "state-dir": state})
		requireSpelled(t, "mcp", refusalItem(t, doc), "state-dir", map[string][]string{"remedy": {"state-dir"}})
		out, _ := kbase(t, repo, "build", "paper.tex", "--state-dir", state)
		requireSpelled(t, "cli", cliItem(t, "build", out), "--state-dir", map[string][]string{"remedy": {"state-dir"}})
	})
	t.Run("status re-emitting a recorded item", func(t *testing.T) {
		repo := realPath(t, t.TempDir())
		gitRepo(t, repo)
		// The failed build's kb-root/ stands: with none, status reads no build.
		if err := os.MkdirAll(filepath.Join(repo, kb.KBDir), 0o755); err != nil {
			t.Fatal(err)
		}
		state := realPath(t, t.TempDir())
		recordFailedBuild(t, state)
		srv := servedServer(rootCmd, mcp.LocatedAt(repo), &bytes.Buffer{}, log.Discard())
		doc, _ := callTool(t, srv, "status", map[string]any{"state-dir": state})
		out, _ := kbase(t, repo, "status", "--state-dir", state)
		cli := checkResult(t, "status", out).Values
		for surface, d := range map[string]map[string]any{"mcp": doc, "cli": cli} {
			items, _ := d["recent-refusals"].([]any)
			if len(items) != 1 {
				t.Fatalf("%s: recent-refusals %v", surface, d["recent-refusals"])
			}
			item, _ := items[0].(map[string]any)
			key := map[string]string{"mcp": "state-dir", "cli": "--state-dir"}[surface]
			requireSpelled(t, surface, item, key, map[string][]string{"detail": {"state-dir"}})
		}
		wantMCP := `build {"no-inference":true,"resume":true,"state-dir":"` + state + `"}`
		if doc["resume"] != wantMCP {
			t.Errorf("status over MCP resumes with %v, want the build tool's call %s", doc["resume"], wantMCP)
		}
		if wantCLI := "kbase build --no-inference --resume --state-dir " + state; cli["resume"] != wantCLI {
			t.Errorf("status at the command line resumes with %v, want %s", cli["resume"], wantCLI)
		}
	})
}

// recordFailedBuild writes, into the state store state, the progress record
// of one build that failed on an item naming its state store, recording the
// command that resumes it as the build records one.
func recordFailedBuild(t *testing.T, state string) {
	t.Helper()
	item := toolresult.Item{Check: buildstore.CheckError, Key: argmark.Flag("state-dir"),
		Detail: "the build could not write the " + argmark.Flag("state-dir") + " store"}
	var lines []string
	for _, e := range []buildstore.Event{
		{Time: "2026-01-01T00:00:00Z", Event: buildstore.EventRun, NoInference: true,
			Resume: []string{"kbase", "build", "--resume", "--no-inference", "--state-dir", state}},
		{Time: "2026-01-01T00:00:01Z", Event: buildstore.EventEnd, Outcome: toolresult.Failed, Items: []toolresult.Item{item}},
	} {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(b))
	}
	writeValues(t, state, buildstore.ProgressFile, strings.Join(lines, "\n")+"\n")
}

// TestARemedysCommandLineIsItsSurfacesCall: the remedy naming update-format
// is the update-format tool's call over MCP and its command line at the
// command line.
func TestARemedysCommandLineIsItsSurfacesCall(t *testing.T) {
	for _, tc := range []struct {
		overMCP bool
		want    string
	}{{true, "update-format {}"}, {false, "kbase update-format"}} {
		if got := mcp.Spelling(rootCmd, tc.overMCP).Spell(index.UpdateFormatRemedy); got != tc.want {
			t.Errorf("over MCP %v: the remedy reads %q, want %q", tc.overMCP, got, tc.want)
		}
	}
}
