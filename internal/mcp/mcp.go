// Package mcp is the MCP server over the command tree: the kbase.mcp and
// kbase.readOnly annotations a command and a flag declare, the tool table
// derived from the bound commands, a call run as its command's RunE, the
// invocation directory a call names, the resources, the build watcher, and the
// detached build and score launches. What a call reaches beyond the command
// tree is the caller's, handed in as function values.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"kbase/internal/mcp/protocol"
	"kbase/internal/result"
	"kbase/internal/write"
)

// ReadOnlyAnnotation marks a command that writes nothing; its MCP tool
// carries the mark as its read-only hint.
const ReadOnlyAnnotation = "kbase.readOnly"

// ReadOnly declares cmd read-only.
func ReadOnly(cmd *cobra.Command) *cobra.Command { return annotate(cmd, ReadOnlyAnnotation, "true") }

// BindingAnnotation is a command's MCP binding, declared where the command is
// built: Bound where a tool binds it, by Bind, and Excluded where none does,
// by Exclude. A command declaring neither is bound by no tool.
const (
	BindingAnnotation = "kbase.mcp"
	Bound             = "bound"
	Excluded          = "excluded"
)

// Bind declares cmd bound: a tool binds it.
func Bind(cmd *cobra.Command) *cobra.Command { return annotate(cmd, BindingAnnotation, Bound) }

// ExclusionReasonAnnotation is an excluded command's reason for being bound
// by no tool, as Exclude records it.
const ExclusionReasonAnnotation = "kbase.mcp.reason"

// Exclude declares cmd excluded: no tool binds it, for reason. The reason is
// recorded under ExclusionReasonAnnotation for the tool table's test to read;
// no tool, help text or result carries it.
func Exclude(cmd *cobra.Command, reason string) *cobra.Command {
	return annotate(annotate(cmd, ExclusionReasonAnnotation, reason), BindingAnnotation, Excluded)
}

// ExcludeFlag declares that f is no argument of its command's tool: its
// kbase.mcp annotation is excluded, and the tool's schema does not derive it.
func ExcludeFlag(f *pflag.Flag) {
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	f.Annotations[BindingAnnotation] = []string{Excluded}
}

// ExcludedFlag is whether f declares itself no argument of its command's
// tool.
func ExcludedFlag(f *pflag.Flag) bool {
	return slices.Equal(f.Annotations[BindingAnnotation], []string{Excluded})
}

func annotate(cmd *cobra.Command, key, value string) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[key] = value
	return cmd
}

type invocationDirKey struct{}

// InvocationDir is the directory the tool call ctx belongs to names for its
// command to start from, and whether ctx is a tool call's.
func InvocationDir(ctx context.Context) (string, bool) {
	dir, ok := ctx.Value(invocationDirKey{}).(string)
	return dir, ok
}

// ResetFlags puts cmd's own flags back to their defaults, as a new process
// would find them; the global flags it inherits are the server's and stay.
func ResetFlags(cmd *cobra.Command) error {
	var err error
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		var ferr error
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			ferr = sv.Replace(nil)
		} else {
			ferr = f.Value.Set(f.DefValue)
		}
		if ferr != nil && err == nil {
			err = fmt.Errorf("resetting --%s: %w", f.Name, ferr)
		}
		f.Changed = false
	})
	return err
}

// Positional is one positional argument a command declares in its Use line:
// <Name>, or <Name>... where it repeats.
type Positional struct {
	Name     string
	Variadic bool
}

// DeclaredPositionals is the positionals cmd's Use line declares, in order.
func DeclaredPositionals(cmd *cobra.Command) ([]Positional, error) {
	words := strings.Fields(cmd.Use)[1:]
	declared := make([]Positional, 0, len(words))
	for i, w := range words {
		name, variadic := strings.CutSuffix(w, "...")
		if !strings.HasPrefix(name, "<") || !strings.HasSuffix(name, ">") || variadic && i != len(words)-1 {
			return nil, fmt.Errorf("%s: %q in its usage line is not a positional <name>", cmd.Name(), w)
		}
		declared = append(declared, Positional{strings.Trim(name, "<>"), variadic})
	}
	return declared, nil
}

// flagSchemaTypes is the JSON Schema of each flag type a tool takes.
var flagSchemaTypes = map[string]protocol.Property{
	"bool": {Type: "boolean"}, "string": {Type: "string"}, "int": {Type: "integer"}, "float64": {Type: "number"},
	"stringArray": {Type: "array", Items: &protocol.Property{Type: "string"}},
	// A duration flag's argument is a Go duration string, as the command
	// line spells it ("30s").
	"duration": {Type: "string"},
}

// Calls is what a tool call reaches beyond the command tree, supplied by the
// caller of Tools.
type Calls struct {
	// WriteOp runs the write op op over the values document values, writing
	// its result document on out.
	WriteOp func(op string, values []byte, out io.Writer) error
	// RenderError writes on out the result document for err, an error a
	// command's RunE returned without writing one, and on stderr its line.
	RenderError func(err error, out, stderr io.Writer)
	// Detached is the tool of each command it keys, in place of one that runs
	// the command's RunE.
	Detached map[*cobra.Command]func(*cobra.Command) (protocol.Tool, error)
	// Listed is the tools no command stands behind, after the derived ones.
	Listed []protocol.Tool
}

// Tools is the tool table: one tool per command under root its binding
// declares bound, each resolving its KB from dir, and after them calls.Listed.
func Tools(root *cobra.Command, dir string, stderr io.Writer, calls Calls) ([]protocol.Tool, error) {
	var tools []protocol.Tool
	for _, cmd := range root.Commands() {
		if cmd.Annotations[BindingAnnotation] != Bound {
			continue
		}
		var tool protocol.Tool
		var err error
		if fields, ok := write.OpVocabulary(cmd.Name()); ok {
			tool = writeOpTool(cmd.Name(), fields, calls.WriteOp)
		} else if detached, ok := calls.Detached[cmd]; ok {
			tool, err = detached(cmd)
		} else {
			tool, err = commandTool(cmd, dir, stderr, calls.RenderError)
		}
		if err != nil {
			return nil, err
		}
		tool.Description = cmd.Short
		tool.ReadOnly = cmd.Annotations[ReadOnlyAnnotation] == "true"
		tools = append(tools, tool)
	}
	return append(tools, calls.Listed...), nil
}

// writeOpTool is op over one entry: the arguments are the entry.
func writeOpTool(op string, fields []write.OpField, run func(op string, values []byte, out io.Writer) error) protocol.Tool {
	schema := protocol.Schema{Properties: map[string]protocol.Property{}, Required: []string{}}
	for _, f := range fields {
		schema.Properties[f.Name] = protocol.Property{}
		if f.Required {
			schema.Required = append(schema.Required, f.Name)
		}
	}
	call := func(args map[string]any) (string, map[string]any, bool, error) {
		values, err := json.Marshal(map[string]any{"entry": []any{args}})
		if err != nil {
			return "", nil, false, err
		}
		var out bytes.Buffer
		if err := run(op, values, &out); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return protocol.Tool{Name: op, InputSchema: schema, Call: call}
}

// CommandSchema is cmd's positionals and its tool's input schema: the
// positionals are the required arguments, and each of the command's own flags
// not excluded from MCP follows as a property the schema does not require.
func CommandSchema(cmd *cobra.Command) ([]Positional, protocol.Schema, error) {
	positionals, err := DeclaredPositionals(cmd)
	if err != nil {
		return nil, protocol.Schema{}, err
	}
	schema := protocol.Schema{Properties: map[string]protocol.Property{}, Required: []string{}}
	for _, p := range positionals {
		property := protocol.Property{Type: "string"}
		if p.Variadic {
			property = protocol.Property{Type: "array", Items: &protocol.Property{Type: "string"}}
		}
		schema.Properties[p.Name] = property
		schema.Required = append(schema.Required, p.Name)
	}
	cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
		if err != nil || f.Name == "help" || ExcludedFlag(f) {
			return
		}
		property, ok := flagSchemaTypes[f.Value.Type()]
		_, clash := schema.Properties[f.Name]
		switch {
		case !ok:
			err = fmt.Errorf("%s: --%s is a %s, which no tool argument carries", cmd.Name(), f.Name, f.Value.Type())
		case clash:
			err = fmt.Errorf("%s: --%s shares its name with a positional", cmd.Name(), f.Name)
		default:
			property.Description = f.Usage
			schema.Properties[f.Name] = property
		}
	})
	return positionals, schema, err
}

// commandTool is cmd run as its RunE runs it, its positionals and flags
// taken from the arguments.
func commandTool(cmd *cobra.Command, dir string, stderr io.Writer, renderError func(err error, out, stderr io.Writer)) (protocol.Tool, error) {
	positionals, schema, err := CommandSchema(cmd)
	if err != nil {
		return protocol.Tool{}, err
	}
	call := func(args map[string]any) (string, map[string]any, bool, error) {
		var out bytes.Buffer
		if err := runCommandTool(cmd, positionals, args, dir, &out, stderr, renderError); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return protocol.Tool{Name: cmd.Name(), InputSchema: schema, Call: call}, nil
}

// runCommandTool runs cmd over args from dir, its result document on out,
// and leaves cmd's streams and context as it found them and its flags at
// their defaults.
func runCommandTool(cmd *cobra.Command, positionals []Positional, args map[string]any, dir string, out, stderr io.Writer, renderError func(err error, out, stderr io.Writer)) error {
	if err := ResetFlags(cmd); err != nil {
		return err
	}
	prev := cmd.Context()
	cmd.SetContext(context.WithValue(context.Background(), invocationDirKey{}, dir))
	cmd.SetOut(out)
	cmd.SetErr(stderr)
	defer func() {
		cmd.SetContext(prev)
		cmd.SetOut(nil)
		cmd.SetErr(nil)
	}()
	argv, flags := CommandLine(positionals, args)
	err := cmd.ValidateArgs(argv)
	for _, f := range flags {
		if err != nil {
			break
		}
		err = cmd.Flags().Set(f.Name, f.Value)
	}
	if err == nil {
		err = cmd.RunE(cmd, argv)
	}
	if err != nil {
		renderError(err, out, stderr)
	}
	return ResetFlags(cmd)
}

// FlagArgument is one flag setting: --Name Value.
type FlagArgument struct{ Name, Value string }

// CommandLine is args as the command line spells them: the positionals in
// their declared order, then a setting per flag value in name order, one for
// each item of an array.
func CommandLine(positionals []Positional, args map[string]any) (argv []string, flags []FlagArgument) {
	for _, p := range positionals {
		argv = append(argv, argumentTexts(args[p.Name])...)
	}
	for _, name := range slices.Sorted(maps.Keys(args)) {
		if slices.ContainsFunc(positionals, func(p Positional) bool { return p.Name == name }) {
			continue
		}
		for _, v := range argumentTexts(args[name]) {
			flags = append(flags, FlagArgument{name, v})
		}
	}
	return argv, flags
}

// argumentTexts spells an argument the schema admitted as command-line
// words: one, or one per item of an array; none for an absent one.
func argumentTexts(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case []any:
		var out []string
		for _, item := range x {
			out = append(out, argumentTexts(item)...)
		}
		return out
	case bool:
		return []string{strconv.FormatBool(x)}
	case float64:
		return []string{strconv.FormatFloat(x, 'f', -1, 64)}
	}
	return []string{fmt.Sprint(v)}
}

// ToolResult is a result document as an MCP tool's result: the document,
// the document decoded, and whether its outcome is an error.
func ToolResult(doc []byte) (string, map[string]any, bool, error) {
	var structured map[string]any
	if err := yaml.Unmarshal(doc, &structured); err != nil {
		return "", nil, false, fmt.Errorf("decoding the result document: %w", err)
	}
	outcome, _ := structured["outcome"].(string)
	return string(doc), structured, result.IsError(outcome), nil
}
