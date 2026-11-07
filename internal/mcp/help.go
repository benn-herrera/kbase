package mcp

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kbase/internal/argmark"
	"kbase/internal/mcp/protocol"
	"kbase/internal/write"
)

// HelpURIPrefix leads a help resource's URI: a family tool's name or a
// command's follows it.
const HelpURIPrefix = "kbase://help/"

// helpURI is the URI of the help resource named name.
func helpURI(name string) string { return HelpURIPrefix + name }

// HelpResources serves the help of each family tool the server serves, then
// of each command under root that a tool binds: for a family, what its
// operations share and their list; for a command, its shared help body and
// its MCP note. None needs a KB.
func HelpResources(root *cobra.Command) ResourceFamily {
	return ResourceFamily{prefix: HelpURIPrefix, bind: func(s serving) protocol.Resources {
		return helpResources{root, s.families}
	}}
}

type helpResources struct {
	root     *cobra.Command
	families []Family
}

func (r helpResources) List() ([]protocol.Resource, error) {
	var listed []protocol.Resource
	for _, f := range r.families {
		listed = append(listed, helpResource(f.Name))
	}
	for _, c := range boundCommands(r.root) {
		listed = append(listed, helpResource(c.Name()))
	}
	return listed, nil
}

// helpResource is the listed help resource of the family or command named
// name: its URI, and its name, "<name> help".
func helpResource(name string) protocol.Resource {
	return protocol.Resource{URI: helpURI(name), Name: name + " help", MimeType: "text/plain"}
}

func (r helpResources) Read(res protocol.Resource) (string, error) {
	name := strings.TrimPrefix(res.URI, HelpURIPrefix)
	commands := boundCommands(r.root)
	if i := slices.IndexFunc(r.families, func(f Family) bool { return f.Name == name }); i >= 0 {
		return familyHelp(r.root, r.families[i], familyMembers(commands, name)), nil
	}
	i := slices.IndexFunc(commands, func(c *cobra.Command) bool { return c.Name() == name })
	if i < 0 {
		return "", fmt.Errorf("no tool binds a command named %q", name)
	}
	return commandHelp(commands[i])
}

// familyHelp is f's help resource: its operations, listed from the members
// its tool's operation names, each with its summary, then what they share,
// spelled over MCP from the tree under root.
func familyHelp(root *cobra.Command, f Family, members []*cobra.Command) string {
	lines := make([]string, 0, len(members))
	for _, c := range members {
		lines = append(lines, "  "+c.Name()+": "+c.Short)
	}
	return Spelling(root, true).Spell(fmt.Sprintf("The %s tool's operations:\n%s\n\nAn operation's full help is %s<operation>.\n\n%s\n",
		f.Name, strings.Join(lines, "\n"), HelpURIPrefix, strings.TrimSpace(f.Help)))
}

// commandHelp is cmd's help resource: its shared help body, then its argument
// list, then its MCP note, spelled over MCP.
func commandHelp(cmd *cobra.Command) (string, error) {
	list, err := ArgumentList(cmd, true)
	if err != nil {
		return "", err
	}
	return Spelling(cmd.Root(), true).Spell(helpText(cmd.Long, list, cmd.Annotations[MCPNoteAnnotation])), nil
}

// CLIHelp is what `kbase <command> --help` prints for cmd, spelled at the
// command line: its usage line, its shared help body, its argument list, the
// subcommands it has, then its CLI note.
func CLIHelp(cmd *cobra.Command) (string, error) {
	list, err := ArgumentList(cmd, false)
	if err != nil {
		return "", err
	}
	note, err := CLINote(cmd)
	if err != nil {
		return "", err
	}
	return Spelling(cmd.Root(), false).Spell(helpText("Usage: "+cmd.UseLine(), cmd.Long, list, subcommandList(cmd), note)), nil
}

// CLINote is cmd's CLI note as its --help renders it, its markers unspelled:
// the note it declares; then, where a development build carries flags of
// cmd's that no tool takes, those flags as an argument list names them; then,
// below the root, the line naming the global flags, which the root's help
// lists.
func CLINote(cmd *cobra.Command) (string, error) {
	flags := cmd.LocalNonPersistentFlags()
	excluded, err := flagProperties(cmd.Name(), flags, true)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(helpText(cmd.Annotations[CLINoteAnnotation],
		listText(excludedFlagsHeading, flagArguments(excluded, flags, false)), globalFlagsLine(cmd)), "\n"), nil
}

// globalFlagsLine names the root's persistent flags, which every subcommand
// takes, and points to the root's help for them; "" for the root itself.
func globalFlagsLine(cmd *cobra.Command) string {
	if !cmd.HasParent() {
		return ""
	}
	var names []string
	cmd.Root().PersistentFlags().VisitAll(func(f *pflag.Flag) { names = append(names, argmark.Flag(f.Name)) })
	if len(names) == 0 {
		return ""
	}
	return "Global flags: " + strings.Join(names, ", ") + "; " + cmd.Root().Name() + " --help describes them."
}

// argumentsHeading opens a help's argument list.
const argumentsHeading = "Arguments:"

// excludedFlagsHeading opens the CLI note's list of the flags a development
// build carries and no tool takes.
const excludedFlagsHeading = "This development build's flags, which no MCP tool takes:"

// argument is one line of a help's argument list: its name, marked as the help
// body marks it, its type, whether a call requires it or may repeat it, and
// its description, markers unspelled.
type argument struct {
	name, typ            string
	required, repeatable bool
	description          string
}

// ArgumentList is cmd's argument list as the surface overMCP names reads it,
// its markers unspelled, "" where it takes no argument: kbase write's batch
// keys, by name, each bare but that the command line's batch options are the
// flags it takes, entry a list on both surfaces; a values op's entry keys, in
// vocabulary order, each bare, as the body names them on both surfaces; any
// other command's positionals in order, then its own flags by name, as a tool
// takes them; the root, its persistent flags and its own.
func ArgumentList(cmd *cobra.Command, overMCP bool) (string, error) {
	var args []argument
	_, unparsed := DeclaredPositionals(cmd)
	switch fields, values := write.OpVocabulary(cmd.Name()); {
	case !cmd.HasParent():
		flags := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)
		flags.AddFlagSet(cmd.PersistentFlags())
		flags.AddFlagSet(cmd.LocalNonPersistentFlags())
		properties, err := flagProperties(cmd.Name(), flags, false)
		if err != nil {
			return "", err
		}
		args = flagArguments(properties, flags, overMCP)
	case cmd.Annotations[BindingAnnotation] != Bound && unparsed != nil:
		// cobra's own help command declares "[command]", which is no
		// positional <name>; its usage line shows it.
		properties, err := flagProperties(cmd.Name(), cmd.LocalNonPersistentFlags(), false)
		if err != nil {
			return "", err
		}
		args = flagArguments(properties, cmd.LocalNonPersistentFlags(), overMCP)
	case cmd.Name() == write.Batch:
		// At the command line the batch options are kbase write's flags, and
		// entry is a key of the batch, which takes it as a list, as
		// the tool does.
		schema := batchSchema()
		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			marked := name
			if !overMCP && name != entryProperty {
				marked = argmark.Flag(name)
			}
			const asTheToolTakesIt = true
			args = append(args, propertyArgument(marked, schema.Properties[name], slices.Contains(schema.Required, name), asTheToolTakesIt))
		}
	case values:
		for _, f := range fields {
			args = append(args, argument{name: f.Name, typ: f.Type, required: f.Required, description: f.Description})
		}
	default:
		positionals, schema, err := commandArguments(cmd)
		if err != nil {
			return "", err
		}
		for _, p := range positionals {
			args = append(args, propertyArgument(argmark.Positional(p.Name), schema.Properties[p.Name], !p.Optional, overMCP))
			delete(schema.Properties, p.Name)
		}
		args = append(args, flagArguments(schema.Properties, cmd.LocalNonPersistentFlags(), overMCP)...)
	}
	return listText(argumentsHeading, args), nil
}

// flagArguments is the flags of flags that properties describes, by name, as
// the surface overMCP names lists them, each required where RequireFlag
// declares it: at the command line, a flag with a shorthand as -s, --name, as
// pflag lists it; over MCP, which takes no shorthand, by its name alone.
func flagArguments(properties map[string]protocol.Property, flags *pflag.FlagSet, overMCP bool) []argument {
	args := make([]argument, 0, len(properties))
	for _, name := range slices.Sorted(maps.Keys(properties)) {
		f := flags.Lookup(name)
		marked := argmark.Flag(name)
		if !overMCP && f.Shorthand != "" {
			marked = "-" + f.Shorthand + ", " + marked
		}
		args = append(args, propertyArgument(marked, properties[name], requiredFlag(f), overMCP))
	}
	return args
}

// propertyArgument is the argument named name that p types and describes, as
// the surface overMCP names takes it: an array is a list of its items' type
// over MCP and its items' type, repeatable, at the command line; a mapping is
// an object of its keys over MCP and, at the command line, a string of the
// keys' values joined by commas.
func propertyArgument(name string, p protocol.Property, required, overMCP bool) argument {
	a := argument{name: name, typ: p.Type, required: required, description: p.Description}
	switch {
	case p.Items != nil && overMCP:
		a.typ = "list of " + p.Items.Type
	case p.Items != nil:
		a.typ, a.repeatable = p.Items.Type, true
	case p.Properties != nil && overMCP:
		a.typ += " {" + strings.Join(p.Required, ", ") + "}"
	case p.Properties != nil:
		a.typ = "string <" + strings.Join(p.Required, ">,<") + ">"
	}
	return a
}

// listText is heading over one line per argument of args — its name, then
// its type, required and repeatable in parentheses, then its description —
// or "" where args is empty.
func listText(heading string, args []argument) string {
	if len(args) == 0 {
		return ""
	}
	lines := []string{heading}
	for _, a := range args {
		line := "  " + a.name
		var facts []string
		if a.typ != "" {
			facts = append(facts, a.typ)
		}
		if a.required {
			facts = append(facts, "required")
		}
		if a.repeatable {
			facts = append(facts, "repeatable")
		}
		if len(facts) > 0 {
			line += " (" + strings.Join(facts, ", ") + ")"
		}
		if a.description != "" {
			line += ": " + a.description
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// subcommandList is cmd's subcommands, each with its summary, as cobra's
// usage block lists them; "" where it has none.
func subcommandList(cmd *cobra.Command) string {
	var lines []string
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() || c.Name() == "help" {
			lines = append(lines, fmt.Sprintf("  %-*s  %s", c.NamePadding(), c.Name(), c.Short))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "Commands:\n" + strings.Join(lines, "\n")
}

// toolDescription is the description of the tool cmd alone binds: its
// summary, then its MCP note where NoteEveryCall marks it, then where its full
// help is, spelled over MCP.
func toolDescription(cmd *cobra.Command) string {
	parts := []string{cmd.Short + "."}
	if note := cmd.Annotations[MCPNoteAnnotation]; note != "" && cmd.Annotations[noteEveryCallAnnotation] == "true" {
		parts = append(parts, strings.TrimSpace(note))
	}
	return Spelling(cmd.Root(), true).Spell(strings.Join(append(parts, "Full help: "+helpURI(cmd.Name())+"."), " "))
}

// helpText is each of parts that is not empty, in order, a blank line between
// two, ending in one newline.
func helpText(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p = strings.TrimRightFunc(p, unicode.IsSpace); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n\n") + "\n"
}
