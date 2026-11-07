package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kbase/internal/argmark"
	"kbase/internal/log"
	"kbase/internal/mcp"
)

var (
	helpFrames = flag.String("help.frames", "", "the frames kbase mcp answered to one resources/read per help resource")
	helpDir    = flag.String("help.dir", "", "the directory holding <command>.txt, what `kbase <command> --help` printed")
)

// helpOf is a help rendering's parts in order, each not empty, a blank line
// between two, ending in one newline.
func helpOf(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p = strings.TrimRightFunc(p, unicode.IsSpace); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n\n") + "\n"
}

// listDescriptions is each line of an argument list less its name and type —
// what each surface states alike, as each takes a name, a shorthand, a list
// and a mapping its own way — "" for a line with no description.
func listDescriptions(list string) []string {
	var out []string
	for _, line := range strings.Split(list, "\n") {
		_, description, _ := strings.Cut(line, "): ")
		out = append(out, description)
	}
	return out
}

// usageLine is the line opening a command's --help, its synopsis.
func usageLine(c *cobra.Command) string { return "Usage: " + c.UseLine() }

// helpParts is c's argument list as each surface reads it, and its CLI note,
// each unspelled.
func helpParts(t *testing.T, c *cobra.Command) (mcpList, cliList, cliNote string) {
	t.Helper()
	mcpList, err := mcp.ArgumentList(c, true)
	if err != nil {
		t.Fatal(err)
	}
	cliList, err = mcp.ArgumentList(c, false)
	if err != nil {
		t.Fatal(err)
	}
	cliNote, err = mcp.CLINote(c)
	if err != nil {
		t.Fatal(err)
	}
	return mcpList, cliList, cliNote
}

// cliHelpSpelled and mcpHelpSpelled are help text as each surface spells it.
func cliHelpSpelled(text string) string { return mcp.Spelling(rootCmd, false).Spell(text) }
func mcpHelpSpelled(text string) string { return mcp.Spelling(rootCmd, true).Spell(text) }

// boundCommandsOf is the commands under root a tool binds.
func boundCommandsOf(root *cobra.Command) []*cobra.Command {
	var bound []*cobra.Command
	for _, c := range root.Commands() {
		if c.Annotations[mcp.BindingAnnotation] == mcp.Bound {
			bound = append(bound, c)
		}
	}
	return bound
}

// TestHelpRenderingsAreTheirParts: for every bound command, each of its three
// help renderings is exactly its parts, spelled for its surface —
// `kbase <command> --help` the usage line, the shared body, the argument list
// and the CLI note; kbase://help/<command> the shared body, the argument list
// and the MCP note; and the description of a tool the command alone binds its
// summary, its MCP note where the command is probe or update-format, the
// commands mcp.NoteEveryCall marks, and where its full help is — with the
// notes each command declares, and with both notes replaced. With every
// command noted, a mark on any other command, or none on either, fails its
// description.
func TestHelpRenderingsAreTheirParts(t *testing.T) {
	bound := boundCommandsOf(rootCmd)
	if len(bound) == 0 {
		t.Fatal("no command is bound")
	}
	notes := func(c *cobra.Command) (mcpNote, cliNote string) {
		return "Over MCP, " + c.Name() + " differs.", "At the command line, " + c.Name() + " differs."
	}
	for _, noted := range []bool{false, true} {
		if noted {
			for _, c := range bound {
				saved := c.Annotations
				t.Cleanup(func() { c.Annotations = saved })
				c.Annotations = maps.Clone(saved)
				c.Annotations[mcp.MCPNoteAnnotation], c.Annotations[mcp.CLINoteAnnotation] = notes(c)
			}
		}

		srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
		descriptions := map[string]string{}
		for _, tool := range toolsListedBy(t, srv) {
			descriptions[tool.Name] = tool.Description
		}
		dir := realPath(t, t.TempDir())
		for _, c := range bound {
			mcpList, cliList, cliNote := helpParts(t, c)
			if !slices.Equal(listDescriptions(cliList), listDescriptions(mcpList)) {
				t.Errorf("%s: its argument lists differ by more than how each surface names and types an argument:\n%s\n%s", c.Name(), cliList, mcpList)
			}
			mcpNote := c.Annotations[mcp.MCPNoteAnnotation]
			if noted {
				mcpNote, _ = notes(c)
			}
			printed, code := kbase(t, dir, c.Name(), "--help")
			if want := cliHelpSpelled(helpOf(usageLine(c), c.Long, cliList, cliNote)); code != 0 || printed != want {
				t.Errorf("notes %v: kbase %s --help exits %d, prints\n%s\nwant its usage line, its body, its argument list and its CLI note\n%s", noted, c.Name(), code, printed, want)
			}
			if got, want := readResource(t, srv, mcp.HelpURIPrefix+c.Name()), mcpHelpSpelled(helpOf(c.Long, mcpList, mcpNote)); got != want {
				t.Errorf("notes %v: %s%s reads\n%s\nwant its body, its argument list and its MCP note\n%s", noted, mcp.HelpURIPrefix, c.Name(), got, want)
			}
			if c.Annotations[mcp.FamilyAnnotation] != "" {
				continue
			}
			want := c.Short + ". "
			if mcpNote != "" && (c == probeCmd || c.Name() == "update-format") {
				want += mcpNote + " "
			}
			want = mcpHelpSpelled(want + "Full help: " + mcp.HelpURIPrefix + c.Name() + ".")
			if got := descriptions[c.Name()]; got != want {
				t.Errorf("notes %v: the %s tool's description is %q, want %q", noted, c.Name(), got, want)
			}
		}
	}
}

// TestTheRootHelpIsItsParts: `kbase --help` prints the root's usage line, its
// shared body, its argument list, which names each persistent flag, its
// subcommands with their summaries, as cobra's usage block lists them, and its
// CLI note, with the note it declares and with one set; and the body carries
// the batch contract in full and the edge-direction sentence the instructions
// carry. The root's body and mcp's name each CLI-only command but cobra's own.
func TestTheRootHelpIsItsParts(t *testing.T) {
	if !strings.Contains(rootCmd.Long, batchHelp) {
		t.Errorf("the root's help body does not carry the batch contract:\n%s", rootCmd.Long)
	}
	if !strings.Contains(rootCmd.Long, mcp.EdgeDirection) {
		t.Errorf("the root's help body does not carry the edge-direction sentence %q:\n%s", mcp.EdgeDirection, rootCmd.Long)
	}
	server, _, err := rootCmd.Find([]string{mcpCommandName})
	if err != nil {
		t.Fatal(err)
	}
	cobraOwn := cobraCommands()
	var cliOnlyNames []string
	for _, c := range rootCmd.Commands() {
		if c.Annotations[mcp.BindingAnnotation] == mcp.Excluded && !cobraOwn[c.Name()] {
			cliOnlyNames = append(cliOnlyNames, c.Name())
		}
	}
	if len(cliOnlyNames) == 0 {
		t.Fatal("the tree declares no CLI-only command, so the check proves nothing")
	}
	for _, c := range []*cobra.Command{rootCmd, server} {
		for _, name := range cliOnlyNames {
			if !slices.Contains(instructionsTokens(c.Long), name) {
				t.Errorf("%s's help body does not name the CLI-only command %s:\n%s", c.Name(), name, c.Long)
			}
		}
	}
	_, list, note := helpParts(t, rootCmd)
	if note != "" {
		t.Errorf("the root's CLI note is %q, want none: it declares none, and the global flags line is a subcommand's", note)
	}
	for _, flags := range []*pflag.FlagSet{rootCmd.PersistentFlags(), rootCmd.LocalNonPersistentFlags()} {
		flags.VisitAll(func(f *pflag.Flag) {
			name := argmark.Flag(f.Name)
			if f.Shorthand != "" {
				name = "-" + f.Shorthand + ", " + name
			}
			if f.Name != "help" && !strings.Contains(list, "\n  "+name+" (") {
				t.Errorf("the root's argument list names no %s:\n%s", name, list)
			}
		})
	}
	if !strings.Contains(list, argmark.Flag("version")) {
		t.Errorf("the root's argument list names no --version:\n%s", list)
	}
	subcommands := "Commands:"
	for _, c := range rootCmd.Commands() {
		if c.IsAvailableCommand() || c.Name() == "help" {
			subcommands += fmt.Sprintf("\n  %-*s  %s", c.NamePadding(), c.Name(), c.Short)
		}
	}
	saved := rootCmd.Annotations
	t.Cleanup(func() { rootCmd.Annotations = saved })
	dir := realPath(t, t.TempDir())
	for _, note := range []string{saved[mcp.CLINoteAnnotation], "At the command line, kbase differs."} {
		rootCmd.Annotations = maps.Clone(saved)
		if rootCmd.Annotations == nil {
			rootCmd.Annotations = map[string]string{}
		}
		rootCmd.Annotations[mcp.CLINoteAnnotation] = note
		printed, code := kbase(t, dir, "--help")
		if want := cliHelpSpelled(helpOf(usageLine(rootCmd), rootCmd.Long, list, subcommands, note)); code != 0 || printed != want {
			t.Errorf("note %q: kbase --help exits %d, prints\n%s\nwant its usage line, its body, its argument list, its subcommands and its CLI note\n%s", note, code, printed, want)
		}
	}
}

// TestTheWriteHelpListsTheBatchAsEachSurfaceTakesIt: `kbase write --help`
// lists dry-run and commit as the flags the command takes, and entry, a key of
// the batch and never a flag, bare and typed a list, not repeatable;
// kbase://help/write lists all three bare, as the write tool takes them.
func TestTheWriteHelpListsTheBatchAsEachSurfaceTakesIt(t *testing.T) {
	cli, code := kbase(t, realPath(t, t.TempDir()), "write", "--help")
	if code != 0 {
		t.Fatalf("kbase write --help exits %d:\n%s", code, cli)
	}
	srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
	for _, surface := range []struct {
		help string
		want map[string]string
	}{
		{cli, map[string]string{"--commit": "boolean", "--dry-run": "boolean", "entry": "list of object, required"}},
		{readResource(t, srv, mcp.HelpURIPrefix+"write"), map[string]string{"commit": "boolean", "dry-run": "boolean", "entry": "list of object, required"}},
	} {
		listed := map[string]string{}
		_, list, _ := strings.Cut(surface.help, "\nArguments:\n")
		for _, line := range strings.Split(list, "\n") {
			m := listedArgument.FindStringSubmatch(line)
			if m == nil {
				break
			}
			listed[m[1]] = m[2]
		}
		if !maps.Equal(listed, surface.want) {
			t.Errorf("the write help lists %v, want %v:\n%s", listed, surface.want, surface.help)
		}
	}
}

// TestEveryCommandHasALong: every command whose --help the root's help
// function renders has a shared help body, since mcp.CLIHelp renders Long
// alone where cobra's own template fell back to Short, and mcp.CLIHelp renders
// it; every subcommand's CLI note ends naming the global flags; and, but in
// cobra's own help and completion, every positional declared in a usage line
// is an argument list line with a type and a description.
func TestEveryCommandHasALong(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Long == "" {
			t.Errorf("%q has no Long; its --help prints no body (Short %q)", c.CommandPath(), c.Short)
		}
		if _, err := mcp.CLIHelp(c); err != nil {
			t.Errorf("%q: its --help does not render: %v", c.CommandPath(), err)
		}
		if c.HasParent() {
			if note, err := mcp.CLINote(c); err != nil || !strings.HasSuffix(note, globalFlagsLine) {
				t.Errorf("%q: its CLI note %q does not end naming the global flags", c.CommandPath(), note)
			}
		}
		var declared []mcp.Positional
		if !cobrasOwn(c) {
			var err error
			if declared, err = mcp.DeclaredPositionals(c); err != nil {
				t.Errorf("%q: %v", c.CommandPath(), err)
			}
		}
		list, err := mcp.ArgumentList(c, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range declared {
			if !slices.ContainsFunc(strings.Split(list, "\n"), func(line string) bool {
				m := listedArgument.FindStringSubmatch(line)
				return m != nil && m[1] == argmark.Positional(p.Name)
			}) {
				t.Errorf("%q: <%s> is no argument list line with a type and a description:\n%s", c.CommandPath(), p.Name, list)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}

// cobrasOwn is whether c is cobra's help or completion command, or under one.
func cobrasOwn(c *cobra.Command) bool {
	for ; c.HasParent(); c = c.Parent() {
		if c.Parent() == c.Root() {
			return c.Name() == "help" || c.Name() == "completion"
		}
	}
	return false
}

// globalFlagsLine closes every subcommand's CLI note, unspelled.
var globalFlagsLine = "Global flags: " + argmark.Flag("config-dir") + ", " + argmark.Flag("log-file") + ", " +
	argmark.Flag("log-level") + ", " + argmark.Flag("project-dir") + "; kbase --help describes them."

// TestHelpResourcesShareTheBinarysHelpBody checks each help resource the
// binary served against the file of what the binary printed for that
// command's --help: each is exactly its parts spelled for its surface — the
// resource the shared body, the argument list and the MCP note; --help the
// usage line, the same body and list, and the CLI note — so the two differ
// only by their notes and the usage line. It skips unless the
// test-integration-mcp recipe names both.
func TestHelpResourcesShareTheBinarysHelpBody(t *testing.T) {
	if *helpFrames == "" || *helpDir == "" {
		t.Skip("no frames named; the test-integration-mcp recipe supplies them")
	}
	data, err := os.ReadFile(*helpFrames)
	if err != nil {
		t.Fatal(err)
	}
	served := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var frame struct {
			Result struct {
				Contents []struct {
					URI  string `json:"uri"`
					Text string `json:"text"`
				} `json:"contents"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("a frame that is no JSON-RPC message: %v\n%s", err, line)
		}
		if len(frame.Result.Contents) == 0 {
			continue // the initialize answer, or a failed read, which leaves its name unserved
		}
		c := frame.Result.Contents[0]
		served[strings.TrimPrefix(c.URI, mcp.HelpURIPrefix)] = c.Text
	}
	files, err := filepath.Glob(filepath.Join(*helpDir, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	printed := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		printed[strings.TrimSuffix(filepath.Base(f), ".txt")] = string(b)
	}
	if len(served) == 0 {
		t.Fatal("no help resource was read")
	}
	for name, text := range served {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil || cmd == rootCmd {
			t.Errorf("%s: a resource naming no command", name)
			continue
		}
		help, ok := printed[name]
		mcpList, cliList, cliNote := helpParts(t, cmd)
		switch {
		case !ok:
			t.Errorf("%s: a resource with no --help file", name)
		case !slices.Equal(listDescriptions(cliList), listDescriptions(mcpList)):
			t.Errorf("%s: its argument lists differ by more than how each surface names and types an argument", name)
		case text != mcpHelpSpelled(helpOf(cmd.Long, mcpList, cmd.Annotations[mcp.MCPNoteAnnotation])):
			t.Errorf("%s: the resource is not the shared body, the argument list and the MCP note:\n%s", name, text)
		case help != cliHelpSpelled(helpOf(usageLine(cmd), cmd.Long, cliList, cliNote)):
			t.Errorf("%s: `kbase %s --help` is not the usage line, the shared body, the argument list and the CLI note:\n%s", name, name, help)
		}
	}
	for name := range printed {
		if _, ok := served[name]; !ok {
			t.Errorf("%s: a --help file with no resource frame", name)
		}
	}
}
