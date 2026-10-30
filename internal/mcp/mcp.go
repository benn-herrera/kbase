// Package mcp is the MCP server over the command tree: the kbase.mcp,
// kbase.mcp.family and kbase.readOnly annotations a command and a flag
// declare, the tool table derived from the bound commands and their families,
// a call run as its command's RunE, the
// project a call runs in, the resources, the build watcher, and the detached
// build and score launches. What a call reaches beyond the command
// tree is the caller's, handed in as function values.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"kbase/internal/buildstore"
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

// FamilyAnnotation names the family tool a bound command is an operation of;
// a bound command without it is a tool of its own.
const FamilyAnnotation = "kbase.mcp.family"

// InFamily declares cmd an operation of the family tool named family.
func InFamily(cmd *cobra.Command, family string) *cobra.Command {
	return annotate(cmd, FamilyAnnotation, family)
}

// Family is one tool binding several commands, each an operation of it.
type Family struct {
	// Name is the tool's name, which its members declare with InFamily.
	Name string
	// Summary is the sentence opening the tool's description.
	Summary string
	// Help is what the family's operations share, stated once, in the
	// family's help resource.
	Help string
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
// <Name>, or <Name>... where it repeats. The last one may be optional,
// spelled [<Name>...].
type Positional struct {
	Name     string
	Variadic bool
	Optional bool
}

// DeclaredPositionals is the positionals cmd's Use line declares, in order.
func DeclaredPositionals(cmd *cobra.Command) ([]Positional, error) {
	words := strings.Fields(cmd.Use)[1:]
	declared := make([]Positional, 0, len(words))
	for i, w := range words {
		name := w
		optional := false
		if inner, ok := strings.CutSuffix(strings.TrimPrefix(w, "["), "]"); strings.HasPrefix(w, "[") && ok {
			name, optional = inner, true
		}
		name, variadic := strings.CutSuffix(name, "...")
		last := i == len(words)-1
		if !strings.HasPrefix(name, "<") || !strings.HasSuffix(name, ">") || variadic && !last || optional && !(variadic && last) {
			return nil, fmt.Errorf("%s: %q in its usage line is not a positional <name>", cmd.Name(), w)
		}
		declared = append(declared, Positional{strings.Trim(name, "<>"), variadic, optional})
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

// Project is the project a call runs in: its git worktree root, and whether
// the call named it rather than falling back to the server's.
type Project struct {
	Root  string
	Named bool
}

// ProjectTool is a tool whose Run takes the project each call runs in. Its
// Tool's Call is unset: Tools sets it.
type ProjectTool struct {
	Tool protocol.Tool
	Run  func(p Project, args map[string]any) (string, map[string]any, bool, error)
}

// Calls is what a tool call reaches beyond the command tree, supplied by the
// caller of Tools.
type Calls struct {
	// WriteOp runs the write op op over the values document values in the
	// project at dir, writing its result document on out.
	WriteOp func(dir, op string, values []byte, out io.Writer) error
	// RenderError writes on out the result document for err, an error a
	// command's RunE returned without writing one, and on stderr its line.
	RenderError func(err error, out, stderr io.Writer)
	// Detached is the tool of each command it keys, in place of one that runs
	// the command's RunE.
	Detached map[*cobra.Command]func(*cobra.Command) (ProjectTool, error)
	// Listed is the tools no command stands behind, after the derived ones.
	Listed []ProjectTool
	// Families is the family tools the bound commands' FamilyAnnotation names.
	Families []Family
}

// Tools is the tool table, each tool served by inProject: one tool per command
// under root its binding declares bound and no family claims, one per family
// in the place of its first member, and after them calls.Listed.
func Tools(root *cobra.Command, located Located, stderr io.Writer, calls Calls) ([]protocol.Tool, error) {
	var tools []protocol.Tool
	bound := boundCommands(root)
	for _, cmd := range bound {
		family := cmd.Annotations[FamilyAnnotation]
		if family == "" {
			tool, err := operationTool(cmd, stderr, calls)
			if err != nil {
				return nil, err
			}
			tool.Tool.Description = cmd.Short
			tool.Tool.ReadOnly = readOnly(cmd)
			tools = append(tools, inProject(tool, located))
			continue
		}
		if slices.ContainsFunc(tools, func(t protocol.Tool) bool { return t.Name == family }) {
			continue
		}
		i := slices.IndexFunc(calls.Families, func(f Family) bool { return f.Name == family })
		if i < 0 {
			return nil, fmt.Errorf("%s: an operation of the family %q, which no Family names", cmd.Name(), family)
		}
		tool, err := familyTool(calls.Families[i], familyMembers(bound, family), stderr, calls)
		if err != nil {
			return nil, err
		}
		tools = append(tools, inProject(tool, located))
	}
	for _, tool := range calls.Listed {
		tools = append(tools, inProject(tool, located))
	}
	return tools, nil
}

// boundCommands is the commands under root whose binding declares them bound,
// in the tree's order.
func boundCommands(root *cobra.Command) []*cobra.Command {
	return slices.DeleteFunc(slices.Clone(root.Commands()), func(c *cobra.Command) bool {
		return c.Annotations[BindingAnnotation] != Bound
	})
}

// familyMembers is the commands of bound that declare themselves operations of
// family, in bound's order.
func familyMembers(bound []*cobra.Command, family string) []*cobra.Command {
	return slices.DeleteFunc(slices.Clone(bound), func(c *cobra.Command) bool {
		return c.Annotations[FamilyAnnotation] != family
	})
}

func readOnly(cmd *cobra.Command) bool { return cmd.Annotations[ReadOnlyAnnotation] == "true" }

// operationTool is cmd run as its own tool would run it, with no description
// or read-only hint: a write op over one entry, a detached launch, or its
// RunE over positionals and flags.
func operationTool(cmd *cobra.Command, stderr io.Writer, calls Calls) (ProjectTool, error) {
	if fields, ok := write.OpVocabulary(cmd.Name()); ok {
		return writeOpTool(cmd.Name(), fields, calls.WriteOp), nil
	}
	if detached, ok := calls.Detached[cmd]; ok {
		return detached(cmd)
	}
	return commandTool(cmd, stderr, calls.RenderError)
}

// The family tool's two arguments: the operation a call runs, and the
// arguments that operation's own tool would take.
const (
	operationProperty = "operation"
	argumentsProperty = "arguments"
)

// familyTool is f over its members, in their order: operation names the member
// a call runs, and arguments, checked against that member's own schema, run it
// as its own tool would. The tool is read-only where every member declares
// itself so.
func familyTool(f Family, members []*cobra.Command, stderr io.Writer, calls Calls) (ProjectTool, error) {
	operations := make(map[string]ProjectTool, len(members))
	names := make([]string, 0, len(members))
	allReadOnly := true
	for _, cmd := range members {
		op, err := operationTool(cmd, stderr, calls)
		if err != nil {
			return ProjectTool{}, err
		}
		operations[cmd.Name()] = op
		names = append(names, cmd.Name())
		allReadOnly = allReadOnly && readOnly(cmd)
	}
	schema := protocol.Schema{
		Properties: map[string]protocol.Property{
			operationProperty: {Type: "string", Enum: names},
			argumentsProperty: {Type: "object", Description: "a positional by its Usage name, a flag by its name " +
				"without dashes; for a write op, its entry's keys"},
		},
		Required: []string{operationProperty},
	}
	// The schema admits only a member's name as the operation, so the lookup
	// always finds one.
	run := func(p Project, args map[string]any) (string, map[string]any, bool, error) {
		name, _ := args[operationProperty].(string)
		op := operations[name]
		arguments, _ := args[argumentsProperty].(map[string]any)
		if arguments == nil {
			arguments = map[string]any{}
		}
		if refusal := argumentRefusal(op.Tool, arguments); refusal != nil {
			doc, err := refusalDocument(refusal)
			if err != nil {
				return "", nil, false, err
			}
			return ToolResult([]byte(doc))
		}
		return op.Run(p, arguments)
	}
	description := f.Summary + " What its operations share: " + HelpURIPrefix + f.Name +
		". An operation's full help: " + HelpURIPrefix + "<operation>."
	return ProjectTool{Tool: protocol.Tool{Name: f.Name, Description: description, InputSchema: schema, ReadOnly: allReadOnly}, Run: run}, nil
}

// argumentRefusal is the usage refusal of each way arguments fail op's own
// schema, each item naming its argument and every argument op takes; nil where
// they fit it.
func argumentRefusal(op protocol.Tool, arguments map[string]any) result.Refusal {
	faults := op.InputSchema.Faults(arguments)
	if faults == nil {
		return nil
	}
	allowed := slices.Sorted(maps.Keys(op.InputSchema.Properties))
	refusal := make(result.Refusal, 0, len(faults))
	for _, f := range faults {
		detail := fmt.Sprintf("%s's argument %q must be of type %s", op.Name, f.Key, f.Want.Type)
		switch {
		case f.Missing:
			detail = fmt.Sprintf("%s requires the argument %q", op.Name, f.Key)
		case f.Unknown:
			detail = fmt.Sprintf("%q is no argument of %s", f.Key, op.Name)
		}
		refusal = append(refusal, result.Item{Check: buildstore.CheckUsage, Key: f.Key, Allowed: allowed, Detail: detail,
			Remedy: "its arguments are in its full help, " + HelpURIPrefix + op.Name})
	}
	return refusal
}

// projectDirProperty is the argument naming the project a call runs in, which
// every tool's schema carries.
const projectDirProperty = "project-dir"

// projectDirMeaning is what the project-dir property names, whatever the
// server's mode.
const projectDirMeaning = "Absolute path of the project this call acts on: its git worktree root, " +
	"the directory holding .git, where kb-root/ is or will be. Never kb-root/ itself or a subdirectory."

// projectDirDescription is the project-dir property's description on a
// server that may locate a project of its own.
const projectDirDescription = projectDirMeaning + " Omit it for this server's own project, the one " +
	"its instructions locate, and name it only for another project. Where the instructions locate " +
	"none, a call that omits it is refused."

// inProject is t as the server serves it: its schema carries project-dir,
// optional where the server may locate a project of its own and required where
// every call names its project, and a call runs t in the project callProject
// finds, with project-dir removed from its arguments, or answers callProject's
// refusal. Where project-dir is required the protocol's schema check stops a
// call that omits it, so only an empty one reaches that refusal.
func inProject(t ProjectTool, located Located) protocol.Tool {
	tool := t.Tool
	tool.InputSchema.Properties = maps.Clone(tool.InputSchema.Properties)
	description := projectDirDescription
	if located.Required() {
		description = projectDirMeaning
		tool.InputSchema.Required = append(slices.Clone(tool.InputSchema.Required), projectDirProperty)
	}
	tool.InputSchema.Properties[projectDirProperty] = protocol.Property{Type: "string", Description: description}
	tool.Call = func(args map[string]any) (string, map[string]any, bool, error) {
		p, err := callProject(args, located)
		var refusal result.Refusal
		if errors.As(err, &refusal) {
			doc, err := refusalDocument(refusal)
			if err != nil {
				return "", nil, false, err
			}
			return ToolResult([]byte(doc))
		}
		if err != nil {
			return "", nil, false, err
		}
		args = maps.Clone(args)
		delete(args, projectDirProperty)
		return t.Run(p, args)
	}
	return tool
}

// callProject is the project a call runs in: the one its project-dir names,
// else the server's located project. A call with neither is refused, and so is
// a project-dir that is no git worktree root.
func callProject(args map[string]any, located Located) (Project, error) {
	if named, _ := args[projectDirProperty].(string); named != "" {
		root, err := buildstore.ProjectRoot(named, projectDirProperty)
		return Project{Root: root, Named: true}, err
	}
	if root, ok := located.Root(); ok {
		return Project{Root: root}, nil
	}
	return Project{}, noProject()
}

// noProject is the refusal of a call or read on a server with no located
// project that names none.
func noProject() result.Refusal {
	return result.Refusal{{Check: buildstore.CheckUsage, Key: projectDirProperty,
		Detail: "this server has no project of its own and the call names none",
		Remedy: "name the project with project-dir: the absolute path of its git worktree root"}}
}

// refusalDocument is the refused result document carrying refusal.
func refusalDocument(refusal result.Refusal) (string, error) {
	var out bytes.Buffer
	if err := result.Emit(&out, result.Refused, result.Field{Key: result.RefusalsKey, Value: []result.Item(refusal)}); err != nil {
		return "", fmt.Errorf("rendering the refusal: %w", err)
	}
	return out.String(), nil
}

// writeOpTool is op over one entry: the arguments are the entry.
func writeOpTool(op string, fields []write.OpField, run func(dir, op string, values []byte, out io.Writer) error) ProjectTool {
	schema := protocol.Schema{Properties: map[string]protocol.Property{}, Required: []string{}}
	for _, f := range fields {
		schema.Properties[f.Name] = protocol.Property{}
		if f.Required {
			schema.Required = append(schema.Required, f.Name)
		}
	}
	call := func(p Project, args map[string]any) (string, map[string]any, bool, error) {
		values, err := json.Marshal(map[string]any{"entry": []any{args}})
		if err != nil {
			return "", nil, false, err
		}
		var out bytes.Buffer
		if err := run(p.Root, op, values, &out); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return ProjectTool{Tool: protocol.Tool{Name: op, InputSchema: schema}, Run: call}
}

// CommandSchema is cmd's positionals and its tool's input schema: the
// positionals are the required arguments save an optional variadic one, and each of the command's own flags
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
		if !p.Optional {
			schema.Required = append(schema.Required, p.Name)
		}
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
func commandTool(cmd *cobra.Command, stderr io.Writer, renderError func(err error, out, stderr io.Writer)) (ProjectTool, error) {
	positionals, schema, err := CommandSchema(cmd)
	if err != nil {
		return ProjectTool{}, err
	}
	call := func(p Project, args map[string]any) (string, map[string]any, bool, error) {
		var out bytes.Buffer
		if err := runCommandTool(cmd, positionals, args, p.Root, &out, stderr, renderError); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return ProjectTool{Tool: protocol.Tool{Name: cmd.Name(), InputSchema: schema}, Run: call}, nil
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
