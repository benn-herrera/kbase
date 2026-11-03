package mcp

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	"kbase/internal/write"
)

// SurfaceEnv tells a run a tool call launched that its result document is
// read over MCP: Launch.start sets it to SurfaceMCP in the child's
// environment, and the child spells its document for MCP where it is so set.
const (
	SurfaceEnv = "KBASE_RESULT_SURFACE"
	SurfaceMCP = "mcp"
)

// Spelling is how the surface reading a text spells its markers, a call read
// from the command tree under root: over MCP where overMCP, the call of the
// tool binding its command; at the command line otherwise, a kbase command
// line, in help as in a result. A command no tool binds is a command line on
// both surfaces.
func Spelling(root *cobra.Command, overMCP bool) argmark.Spelling {
	return argmark.Spelling{MCP: overMCP, Call: func(inv argmark.Invocation) string {
		cmd := commandNamed(root, inv.Command)
		args := inv.Args
		if inv.Argv != nil {
			args = argvArguments(cmd, inv.Argv)
		}
		if overMCP && cmd != nil && cmd.Annotations[BindingAnnotation] == Bound {
			if call, ok := toolCall(cmd, args); ok {
				return call
			}
		}
		return commandLine(cmd, inv.Command, args)
	}}
}

// commandNamed is root's subcommand named name, nil where none is.
func commandNamed(root *cobra.Command, name string) *cobra.Command {
	i := slices.IndexFunc(root.Commands(), func(c *cobra.Command) bool { return c.Name() == name })
	if i < 0 {
		return nil
	}
	return root.Commands()[i]
}

// toolCall is cmd called with args as the tool binding it takes the call: the
// tool's name, then its arguments as compact JSON, keys sorted, carrying only
// the arguments the tool's schema has — a family member's under arguments,
// beside the operation naming it. It reads the schema's names alone, spelling
// no description.
func toolCall(cmd *cobra.Command, args map[string]any) (string, bool) {
	_, schema, err := commandArguments(cmd)
	if err != nil {
		return "", false
	}
	if cmd.Name() == write.Batch {
		schema = batchSchema()
	}
	family := cmd.Annotations[FamilyAnnotation]
	kept := map[string]any{}
	for name, v := range args {
		if _, ok := schema.Properties[name]; ok || name == projectDirProperty && family == "" {
			kept[name] = v
		}
	}
	text, err := argmark.JSON(kept)
	if err != nil {
		return "", false
	}
	if family == "" {
		return cmd.Name() + " " + text, true
	}
	op, err := argmark.JSON(cmd.Name())
	if err != nil {
		return "", false
	}
	return family + " " + `{"` + operationProperty + `":` + op + `,"` + argumentsProperty + `":` + text + `}`, true
}

// commandLine is the command named command called with args as the command
// line takes it — kbase, the command, its positionals, then --name value per
// flag in name order, a true bool flag bare — each word quoted where a shell
// would split or expand it but for a placeholder's angle brackets. A
// placeholder standing for a repeated positional reads <name>..., as the usage
// line spells it. A command reading a values document (readsValues) takes
// every argument but its own flags in that document, piped in on stdin
// (pipedValues).
func commandLine(cmd *cobra.Command, command string, args map[string]any) string {
	var document map[string]any
	if readsValues(cmd) {
		document, args = valuesDocument(cmd, args)
	}
	line := commandWords(cmd, command, args)
	if piped, ok := pipedValues(document); ok {
		return piped + " | " + line
	}
	return line
}

// readsValues is whether cmd reads a values document on stdin: kbase write's
// batch, or a write op's.
func readsValues(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	_, values := write.OpVocabulary(cmd.Name())
	return values || cmd.Name() == write.Batch
}

// valuesDocument is args split into the values document — every argument
// that is not a flag of cmd — and the flags.
func valuesDocument(cmd *cobra.Command, args map[string]any) (document, flags map[string]any) {
	document, flags = map[string]any{}, map[string]any{}
	for name, v := range args {
		if cmd.Flags().Lookup(name) != nil || cmd.InheritedFlags().Lookup(name) != nil {
			flags[name] = v
		} else {
			document[name] = v
		}
	}
	return document, flags
}

// pipedValues is document as YAML piped in by printf, its format escaping %
// and \ and each line break written \n, the word quoted for the shell; ok is
// false where document is empty or does not encode.
func pipedValues(document map[string]any) (string, bool) {
	if len(document) == 0 {
		return "", false
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(document); err != nil {
		return "", false
	}
	if err := enc.Close(); err != nil {
		return "", false
	}
	format := strings.NewReplacer(`\`, `\\`, "%", "%%", "\n", `\n`).Replace(b.String())
	return "printf " + shellWord(format), true
}

// commandWords is the kbase command line of command with args, as
// commandLine describes it, less any piped values document.
func commandWords(cmd *cobra.Command, command string, args map[string]any) string {
	var positionals []Positional
	if cmd != nil {
		if declared, err := DeclaredPositionals(cmd); err == nil {
			positionals = declared
		}
	}
	argv, flags := CommandLine(positionals, Mappings(cmd), args)
	if n := len(positionals); n > 0 && positionals[n-1].Variadic {
		for i := n - 1; i < len(argv); i++ {
			if wholePlaceholder.MatchString(argv[i]) {
				argv[i] += "..."
			}
		}
	}
	words := append([]string{"kbase", command}, argv...)
	for _, f := range flags {
		switch b, isBool := args[f.Name].(bool); {
		case isBool && b:
			words = append(words, "--"+f.Name)
		case isBool:
			words = append(words, "--"+f.Name+"=false")
		default:
			words = append(words, "--"+f.Name, f.Value)
		}
	}
	for i, w := range words {
		words[i] = shellWord(w)
	}
	return strings.Join(words, " ")
}

// placeholder is a word in angle brackets, which the one running a command
// line fills in.
var (
	placeholder      = regexp.MustCompile(`<[a-z][a-z0-9-]*>`)
	wholePlaceholder = regexp.MustCompile(`^<[a-z][a-z0-9-]*>$`)
)

// shellWord is w as a shell reads it back, quoted by buildstore.ShellJoin, but
// bare where only its placeholders would make it quoted.
func shellWord(w string) string {
	if bare := placeholder.ReplaceAllString(w, "x"); buildstore.ShellJoin([]string{bare}) == bare {
		return w
	}
	return buildstore.ShellJoin([]string{w})
}

// argvArguments is argv, the words a call recorded after its command, read
// with cmd's own flags — local and inherited — into the arguments mapping,
// each value typed as its flag is: a bool flag's a bool, an int's an int, a
// repeated flag's a list. A word no flag claims is a positional's value, the
// last positional taking every one after it; a flag cmd does not hold, or a
// cmd that is nil, reads as a string flag where it is given =value, else as a
// bool.
func argvArguments(cmd *cobra.Command, argv []string) map[string]any {
	args := map[string]any{}
	var positionals []Positional
	var lookup func(string) *pflag.Flag
	if cmd != nil {
		if declared, err := DeclaredPositionals(cmd); err == nil {
			positionals = declared
		}
		lookup = func(name string) *pflag.Flag {
			if f := cmd.Flags().Lookup(name); f != nil {
				return f
			}
			return cmd.InheritedFlags().Lookup(name)
		}
	}
	var loose []string
	for i := 0; i < len(argv); i++ {
		name, value, hasValue := strings.Cut(strings.TrimPrefix(argv[i], "--"), "=")
		if !strings.HasPrefix(argv[i], "--") || name == "" {
			loose = append(loose, argv[i])
			continue
		}
		var f *pflag.Flag
		if lookup != nil {
			f = lookup(name)
		}
		switch {
		case hasValue:
		case f == nil:
			args[name] = true
			continue
		case f.Value.Type() == "bool":
			value = "true"
		case i+1 < len(argv):
			i++
			value = argv[i]
		}
		args[name] = typedFlagValue(f, value, args[name])
	}
	for i, p := range positionals {
		switch {
		case i >= len(loose):
		case p.Variadic:
			args[p.Name] = anyList(loose[i:])
		default:
			args[p.Name] = loose[i]
		}
	}
	return args
}

// typedFlagValue is value as f's type takes it, appended to prior where f
// repeats; a value its type does not read, or no flag's, stays text.
func typedFlagValue(f *pflag.Flag, value string, prior any) any {
	if f == nil {
		return value
	}
	switch f.Value.Type() {
	case "bool":
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	case "int":
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	case "float64":
		if x, err := strconv.ParseFloat(value, 64); err == nil {
			return x
		}
	case "stringArray", "stringSlice":
		list, _ := prior.([]any)
		return append(list, value)
	}
	return value
}

func anyList(words []string) []any {
	out := make([]any, len(words))
	for i, w := range words {
		out[i] = w
	}
	return out
}
