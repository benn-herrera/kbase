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
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	"kbase/internal/log"
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

// A command's help is four texts: its Short, the summary; its Long, the help
// body both surfaces share; and two optional notes, declared under these
// annotations, stating what differs over MCP and at the command line.
const (
	MCPNoteAnnotation = "kbase.help.mcp"
	CLINoteAnnotation = "kbase.help.cli"
)

// Notes declares cmd's MCP note and CLI note; an empty one declares none.
func Notes(cmd *cobra.Command, mcpNote, cliNote string) *cobra.Command {
	if mcpNote != "" {
		annotate(cmd, MCPNoteAnnotation, mcpNote)
	}
	if cliNote != "" {
		annotate(cmd, CLINoteAnnotation, cliNote)
	}
	return cmd
}

// noteEveryCallAnnotation marks a command whose MCP note every call of its
// tool needs, so the tool description carries the note as well as the help.
const noteEveryCallAnnotation = "kbase.help.mcp.everyCall"

// NoteEveryCall declares that every call of cmd's tool needs its MCP note.
func NoteEveryCall(cmd *cobra.Command) *cobra.Command {
	return annotate(cmd, noteEveryCallAnnotation, "true")
}

// briefAnnotationPrefix leads the annotation holding an argument's brief, the
// argument's name, a flag's or a positional's, following it.
const briefAnnotationPrefix = "kbase.mcp.brief."

// Brief declares the brief of cmd's argument name: the few words naming what
// its value is, which the served schema describes the argument by. The help's
// argument list keeps the argument's full description.
func Brief(cmd *cobra.Command, name, brief string) *cobra.Command {
	return annotate(cmd, briefAnnotationPrefix+name, brief)
}

// BriefOf is the brief cmd declares for its argument name, its markers
// unspelled; "" where it declares none.
func BriefOf(cmd *cobra.Command, name string) string {
	return cmd.Annotations[briefAnnotationPrefix+name]
}

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

// MappingAnnotation names a string flag a tool takes as a mapping: its keys,
// in the order the command line joins their values with commas.
const MappingAnnotation = "kbase.mcp.mapping"

// mappingEnumAnnotationPrefix leads the annotation holding the values a
// mapping key admits, the key's name following it.
const mappingEnumAnnotationPrefix = MappingAnnotation + ".enum."

// MappingKey is one key of a mapping argument: its name and, where its values
// are a fixed set, that set.
type MappingKey struct {
	Name string
	Enum []string
}

// MappingFlag declares that f's tool argument is a mapping of keys, each a
// string and each required, a key with an Enum admitting only those values,
// which the command line takes as the values joined by commas in keys' order.
func MappingFlag(f *pflag.Flag, keys ...MappingKey) {
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = k.Name
		if k.Enum != nil {
			f.Annotations[mappingEnumAnnotationPrefix+k.Name] = k.Enum
		}
	}
	f.Annotations[MappingAnnotation] = names
}

// enumAnnotation holds the values a string flag's tool argument admits.
const enumAnnotation = "kbase.mcp.enum"

// EnumFlag declares that f's tool argument admits only values, which its
// schema property carries as its enum. The command line is left to the
// command's own check.
func EnumFlag(f *pflag.Flag, values ...string) {
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	f.Annotations[enumAnnotation] = values
}

// Mappings is each of cmd's mapping flags, by name, and its keys; none for a
// nil cmd.
func Mappings(cmd *cobra.Command) map[string][]string {
	out := map[string][]string{}
	if cmd == nil {
		return out
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if keys := f.Annotations[MappingAnnotation]; keys != nil {
			out[f.Name] = keys
		}
	})
	return out
}

// requiredAnnotation marks a flag its command's tool schema requires.
const requiredAnnotation = "kbase.mcp.required"

// RequireFlag declares that f's tool argument is required. The command line
// is left to the command's own refusal of its absence.
func RequireFlag(f *pflag.Flag) {
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	f.Annotations[requiredAnnotation] = []string{"true"}
}

func requiredFlag(f *pflag.Flag) bool {
	return slices.Equal(f.Annotations[requiredAnnotation], []string{"true"})
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

// positionalAnnotationPrefix leads the annotation holding a positional's
// description, the positional's name following it.
const positionalAnnotationPrefix = "kbase.positional."

// DescribePositional declares the description of cmd's positional name, which
// its tool schema's property carries as a flag's carries its usage.
func DescribePositional(cmd *cobra.Command, name, description string) *cobra.Command {
	return annotate(cmd, positionalAnnotationPrefix+name, description)
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

// flagDescription is f's description, its markers unspelled: its usage, then,
// in one parenthetical, that a duration flag's argument is a Go duration and
// the default pflag's flags block would print, as "default: X" — the form the
// hand-written usages take — quoted only where X is no bare token.
func flagDescription(f *pflag.Flag) string {
	var notes []string
	if f.Value.Type() == "duration" {
		notes = append(notes, "a Go duration")
	}
	if def, ok := helpDefault(f); ok {
		if !bareToken.MatchString(def) {
			def = strconv.Quote(def)
		}
		notes = append(notes, "default: "+def)
	}
	description := f.Usage
	if len(notes) > 0 {
		description += " (" + strings.Join(notes, "; ") + ")"
	}
	return description
}

// bareToken is a default a description states unquoted.
var bareToken = regexp.MustCompile(`^[A-Za-z0-9._/+-]+$`)

// helpDefault is f's default, unquoted, where pflag's flags block would print
// one: pflag leaves out a type's zero value. f is of a type flagSchemaTypes
// carries.
func helpDefault(f *pflag.Flag) (string, bool) {
	var zero []string
	switch f.Value.Type() {
	case "bool":
		zero = []string{"false", ""}
	case "duration":
		zero = []string{"0", "0s"}
	case "int", "float64":
		zero = []string{"0"}
	case "string":
		zero = []string{""}
	case "stringArray":
		zero = []string{"[]"}
	}
	if slices.Contains(zero, f.DefValue) {
		return "", false
	}
	return f.DefValue, true
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
	// WriteOp runs op over the batch values in the project at dir, writing its
	// result on out: write.Batch's batch.
	WriteOp func(dir, op string, values []byte, out io.Writer) error
	// RenderError writes on out the result for err, an error a
	// command's RunE returned without writing one, and on stderr its line.
	RenderError func(err error, out, stderr io.Writer)
	// Detached is the tool of each command it keys, in place of one that runs
	// the command's RunE.
	Detached map[*cobra.Command]func(*cobra.Command) (ProjectTool, error)
	// Families is the family tools the bound commands' FamilyAnnotation names.
	Families []Family
	// Logger takes the warning of a result whose item names an argument as
	// the command line spells it; nil discards it.
	Logger log.Logger

	// spelling is how a tool's result is spelled over MCP: Tools sets it from
	// the tree.
	spelling argmark.Spelling
}

// Tools is the tool table, each tool served by inProject: one tool per command
// under root its binding declares bound and no family claims, one per family
// in the place of its first member. A family a
// bound command is named for is that command's: its members are served by
// that command's tool, and have none of their own. A member naming a family
// neither calls.Families nor a bound command holds, or a family no member
// names, is an error. The schema of a tool a command alone binds, write's
// aside, describes each argument by its brief (Brief); an argument without one,
// or a brief naming no argument of the tool, is an error.
func Tools(root *cobra.Command, located Located, stderr io.Writer, calls Calls) ([]protocol.Tool, error) {
	var tools []protocol.Tool
	calls.spelling = Spelling(root, true)
	if calls.Logger == nil {
		calls.Logger = log.Discard()
	}
	bound := boundCommands(root)
	for _, cmd := range bound {
		family := cmd.Annotations[FamilyAnnotation]
		if family != "" && slices.ContainsFunc(bound, func(c *cobra.Command) bool { return c.Name() == family }) {
			continue
		}
		if family == "" {
			tool, err := operationTool(cmd, stderr, calls)
			if err != nil {
				return nil, err
			}
			if cmd.Name() != write.Batch {
				if tool.Tool.InputSchema, err = briefedSchema(cmd, tool.Tool.InputSchema, calls.spelling); err != nil {
					return nil, err
				}
			}
			tool.Tool.Description = toolDescription(cmd)
			tool.Tool.ReadOnly = readOnly(cmd)
			tools = append(tools, inProject(tool, located, calls))
			continue
		}
		if slices.ContainsFunc(tools, func(t protocol.Tool) bool { return t.Name == family }) {
			continue
		}
		i := slices.IndexFunc(calls.Families, func(f Family) bool { return f.Name == family })
		if i < 0 {
			return nil, fmt.Errorf("%s: an operation of the family %q, which no Family names", cmd.Name(), family)
		}
		tool, err := familyTool(root, calls.Families[i], familyMembers(bound, family), stderr, calls)
		if err != nil {
			return nil, err
		}
		tools = append(tools, inProject(tool, located, calls))
	}
	// The help resources list calls.Families, so a family no bound command
	// declares would serve help for a tool the table does not hold.
	for _, f := range calls.Families {
		if len(familyMembers(bound, f.Name)) == 0 {
			return nil, fmt.Errorf("family %q: no bound command declares it", f.Name)
		}
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

// briefedSchema is schema with each property described by the brief cmd
// declares for it (Brief), spelled by sp. A property cmd declares no brief for,
// or a brief cmd declares for no property, is an error naming it.
func briefedSchema(cmd *cobra.Command, schema protocol.Schema, sp argmark.Spelling) (protocol.Schema, error) {
	for _, key := range slices.Sorted(maps.Keys(cmd.Annotations)) {
		if name, ok := strings.CutPrefix(key, briefAnnotationPrefix); ok {
			if _, taken := schema.Properties[name]; !taken {
				return protocol.Schema{}, fmt.Errorf("%s: a brief for %s, which its tool does not take", cmd.Name(), name)
			}
		}
	}
	properties := make(map[string]protocol.Property, len(schema.Properties))
	for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
		brief := BriefOf(cmd, name)
		if brief == "" {
			return protocol.Schema{}, fmt.Errorf("%s: its argument %s declares no brief", cmd.Name(), name)
		}
		property := schema.Properties[name]
		property.Description = sp.Spell(brief)
		properties[name] = property
	}
	schema.Properties = properties
	return schema, nil
}

// operationTool is cmd run as its own tool would run it, with no description
// or read-only hint: the write batch, a detached launch, or its RunE over
// positionals and flags.
func operationTool(cmd *cobra.Command, stderr io.Writer, calls Calls) (ProjectTool, error) {
	if cmd.Name() == write.Batch {
		return batchTool(calls.WriteOp, calls.spelling), nil
	}
	if detached, ok := calls.Detached[cmd]; ok {
		return detached(cmd)
	}
	return commandTool(cmd, stderr, calls.RenderError, calls.spelling)
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
// itself so. A member's result is checked against the member's own arguments
// (warnCLIKeys).
func familyTool(root *cobra.Command, f Family, members []*cobra.Command, stderr io.Writer, calls Calls) (ProjectTool, error) {
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
			operationProperty: {Type: "string", Enum: names, Description: "the query to run; each one's help is " + helpURI("<operation>")},
			argumentsProperty: {Type: "object", Description: "the operation's arguments by name, as its help's argument list gives them"},
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
			doc, err := refusalResult(refusal, calls.spelling)
			if err != nil {
				return "", nil, false, err
			}
			return ToolResult([]byte(doc))
		}
		text, structured, isError, err := op.Run(p, arguments)
		warnCLIKeys(calls.Logger, op.Tool, structured)
		return text, structured, isError, err
	}
	description := Spelling(root, true).Spell(f.Summary + " What its operations share: " + helpURI(f.Name) +
		". An operation's full help: " + helpURI("<operation>") + ".")
	return ProjectTool{Tool: protocol.Tool{Name: f.Name, Description: description, InputSchema: schema, ReadOnly: allReadOnly}, Run: run}, nil
}

// argumentRefusal is the usage refusal of each way arguments fail op's own
// schema, each item naming its argument at its dotted path and every key of
// the object it lies in — op's arguments, or a mapping argument's keys — and
// its remedy pointing at op's full help for the rest; nil where they fit it.
func argumentRefusal(op protocol.Tool, arguments map[string]any) result.Refusal {
	faults := op.InputSchema.Faults(arguments)
	if faults == nil {
		return nil
	}
	refusal := make(result.Refusal, 0, len(faults))
	for _, f := range faults {
		refusal = append(refusal, result.Item{Check: result.CheckUsage, Key: f.Key, Allowed: f.Allowed, Detail: op.Name + ": " + f.Detail(),
			Remedy: "allowed names every argument it takes; its full help, " + helpURI(op.Name) +
				", says what it does"})
	}
	return refusal
}

// projectDirProperty is the argument naming the project a call runs in, which
// every tool's schema carries.
const projectDirProperty = "project-dir"

// projectDirMeaning is what the project-dir property names, whatever the
// server's mode.
const projectDirMeaning = "Absolute path of the project's git worktree root, not kb-root/"

// projectDirDescription is the project-dir property's description on a
// server that located a project of its own.
const projectDirDescription = projectDirMeaning + "; omit it for this server's own project"

// inProject is t as the server serves it: its schema carries project-dir,
// optional where the server located a project of its own and required where
// every call names its project, and a call runs t in the project callProject
// finds, with project-dir removed from its arguments, or answers callProject's
// refusal. Where project-dir is required the protocol's schema check stops a
// call that omits it, so only an empty one reaches that refusal. Each result
// is checked against the tool's arguments (warnCLIKeys).
func inProject(t ProjectTool, located Located, calls Calls) protocol.Tool {
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
			doc, err := refusalResult(refusal, calls.spelling)
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
		text, structured, isError, err := t.Run(p, args)
		warnCLIKeys(calls.Logger, tool, structured)
		return text, structured, isError, err
	}
	return tool
}

// warnCLIKeys logs, at warn, each item closing a tool's result whose key names
// one of the tool's arguments as the command line spells it, --name or <name>:
// a producer that named the argument without the marker. It rewrites nothing.
func warnCLIKeys(lg log.Logger, tool protocol.Tool, structured map[string]any) {
	for _, closing := range []string{result.RefusalsKey, result.FailuresKey} {
		items, _ := structured[closing].([]any)
		for _, it := range items {
			item, _ := it.(map[string]any)
			key, _ := item["key"].(string)
			var name string
			switch {
			case strings.HasPrefix(key, "--"):
				name = key[len("--"):]
			case strings.HasPrefix(key, "<") && strings.HasSuffix(key, ">"):
				name = key[1 : len(key)-1]
			default:
				continue
			}
			if _, ok := tool.InputSchema.Properties[name]; ok {
				lg.Warn("mcp: a result names an argument as the command line spells it", "tool", tool.Name, "key", key)
			}
		}
	}
}

// callProject is the project a call runs in: the one its project-dir names,
// else the server's located project. A call with neither is refused, and so is
// a project-dir that is no git worktree root.
func callProject(args map[string]any, located Located) (Project, error) {
	if named, _ := args[projectDirProperty].(string); named != "" {
		root, err := buildstore.ProjectRoot(named)
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
	return result.Refusal{{Check: result.CheckUsage, Key: projectDirProperty,
		Detail: "this server has no project of its own and the call names none",
		Remedy: "name the project with project-dir: the absolute path of its git worktree root"}}
}

// refusalResult is the refused result carrying refusal, spelled by sp.
func refusalResult(refusal result.Refusal, sp argmark.Spelling) (string, error) {
	var out bytes.Buffer
	if err := result.Emit(result.Spelled(&out, sp), result.Refused, result.Field{Key: result.RefusalsKey, Value: []result.Item(refusal)}); err != nil {
		return "", fmt.Errorf("rendering the refusal: %w", err)
	}
	return out.String(), nil
}

// The write batch tool's arguments: the entries, and the dry-run and commit
// options.
const (
	entryProperty  = "entry"
	dryRunProperty = "dry-run"
	commitProperty = "commit"
)

// batchSchema is the write batch tool's input schema: the batch.
func batchSchema() protocol.Schema {
	return protocol.Schema{
		Properties: map[string]protocol.Property{
			entryProperty: {Type: "array", Items: &protocol.Property{Type: "object"},
				Description: "the entries, in the order they apply: each names its op under \"operation\", beside that op's keys"},
			dryRunProperty: {Type: "boolean", Description: "do everything but write"},
			commitProperty: {Type: "boolean", Description: "commit kb-root/'s prior changes, then the batch, so git can undo it"},
		},
		Required: []string{entryProperty},
	}
}

// batchTool is the write batch: the arguments are the batch, run through run
// as write.Batch's batch, its result spelled by sp.
func batchTool(run func(dir, op string, values []byte, out io.Writer) error, sp argmark.Spelling) ProjectTool {
	schema := batchSchema()
	call := func(p Project, args map[string]any) (string, map[string]any, bool, error) {
		values, err := json.Marshal(args)
		if err != nil {
			return "", nil, false, err
		}
		var out bytes.Buffer
		if err := run(p.Root, write.Batch, values, result.Spelled(&out, sp)); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return ProjectTool{Tool: protocol.Tool{Name: write.Batch, InputSchema: schema}, Run: call}
}

// CommandSchema is commandArguments' positionals and schema, each description
// spelled over MCP.
func CommandSchema(cmd *cobra.Command) ([]Positional, protocol.Schema, error) {
	positionals, schema, err := commandArguments(cmd)
	if err != nil {
		return nil, protocol.Schema{}, err
	}
	sp := Spelling(cmd.Root(), true)
	for name, property := range schema.Properties {
		property.Description = sp.Spell(property.Description)
		schema.Properties[name] = property
	}
	return positionals, schema, nil
}

// commandArguments is cmd's positionals and its tool's input schema, each
// property described with its markers unspelled: the positionals are the
// required arguments save an optional variadic one, each described as
// DescribePositional declares, and each of the command's own flags not
// excluded from MCP is a property (flagProperties), which the schema requires
// only where RequireFlag declares it.
func commandArguments(cmd *cobra.Command) ([]Positional, protocol.Schema, error) {
	positionals, err := DeclaredPositionals(cmd)
	if err != nil {
		return nil, protocol.Schema{}, err
	}
	properties, err := flagProperties(cmd.Name(), cmd.LocalNonPersistentFlags(), false)
	if err != nil {
		return nil, protocol.Schema{}, err
	}
	schema := protocol.Schema{Properties: properties, Required: []string{}}
	for _, p := range positionals {
		if _, clash := properties[p.Name]; clash {
			return nil, protocol.Schema{}, fmt.Errorf("%s: --%s shares its name with a positional", cmd.Name(), p.Name)
		}
		property := protocol.Property{Type: "string"}
		if p.Variadic {
			property = protocol.Property{Type: "array", Items: &protocol.Property{Type: "string"}}
		}
		property.Description = cmd.Annotations[positionalAnnotationPrefix+p.Name]
		schema.Properties[p.Name] = property
		if !p.Optional {
			schema.Required = append(schema.Required, p.Name)
		}
	}
	cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
		if _, ok := properties[f.Name]; ok && requiredFlag(f) {
			schema.Required = append(schema.Required, f.Name)
		}
	})
	return positionals, schema, nil
}

// flagProperties is a property for each flag of flags, command's, but help:
// those not excluded from MCP, or where excluded is true those excluded. Each
// is typed by flagSchemaTypes, a mapping flag as an object of its keys, and
// described by flagDescription; a flag and a mapping key declaring a fixed set
// (EnumFlag, MappingFlag) carry it as their enum. A flag of a type no tool
// argument carries is an error.
func flagProperties(command string, flags *pflag.FlagSet, excluded bool) (map[string]protocol.Property, error) {
	properties := map[string]protocol.Property{}
	var err error
	flags.VisitAll(func(f *pflag.Flag) {
		if err != nil || f.Name == "help" || ExcludedFlag(f) != excluded {
			return
		}
		property, ok := flagSchemaTypes[f.Value.Type()]
		if !ok {
			err = fmt.Errorf("%s: --%s is a %s, which no tool argument carries", command, f.Name, f.Value.Type())
			return
		}
		if keys := f.Annotations[MappingAnnotation]; keys != nil && f.Value.Type() == "string" {
			closed := false
			property = protocol.Property{Type: "object", Properties: map[string]protocol.Property{}, Required: keys, AdditionalProperties: &closed}
			for _, key := range keys {
				property.Properties[key] = protocol.Property{Type: "string", Enum: f.Annotations[mappingEnumAnnotationPrefix+key]}
			}
		}
		property.Enum = f.Annotations[enumAnnotation]
		property.Description = flagDescription(f)
		properties[f.Name] = property
	})
	return properties, err
}

// commandTool is cmd run as its RunE runs it, its positionals and flags
// taken from the arguments, its result spelled by sp.
func commandTool(cmd *cobra.Command, stderr io.Writer, renderError func(err error, out, stderr io.Writer), sp argmark.Spelling) (ProjectTool, error) {
	positionals, schema, err := CommandSchema(cmd)
	if err != nil {
		return ProjectTool{}, err
	}
	call := func(p Project, args map[string]any) (string, map[string]any, bool, error) {
		var out bytes.Buffer
		if err := runCommandTool(cmd, positionals, args, p.Root, result.Spelled(&out, sp), stderr, renderError); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return ProjectTool{Tool: protocol.Tool{Name: cmd.Name(), InputSchema: schema}, Run: call}, nil
}

// runCommandTool runs cmd over args from dir, its result on out,
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
	argv, flags := CommandLine(positionals, Mappings(cmd), args)
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
// each item of an array, a mapping argument — one mappings names, with its
// keys (Mappings) — as its values joined by commas in its keys' order.
func CommandLine(positionals []Positional, mappings map[string][]string, args map[string]any) (argv []string, flags []FlagArgument) {
	for _, p := range positionals {
		argv = append(argv, argumentTexts(args[p.Name])...)
	}
	for _, name := range slices.Sorted(maps.Keys(args)) {
		if slices.ContainsFunc(positionals, func(p Positional) bool { return p.Name == name }) {
			continue
		}
		if m, isMapping := args[name].(map[string]any); isMapping && mappings[name] != nil {
			values := make([]string, len(mappings[name]))
			for i, key := range mappings[name] {
				values[i] = fmt.Sprint(m[key])
			}
			flags = append(flags, FlagArgument{name, strings.Join(values, ",")})
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

// ToolResult is a kbase result as an MCP tool's result: its text, its decoded
// mapping, and whether its outcome is an error.
func ToolResult(doc []byte) (string, map[string]any, bool, error) {
	var structured map[string]any
	if err := yaml.Unmarshal(doc, &structured); err != nil {
		return "", nil, false, fmt.Errorf("decoding the result: %w", err)
	}
	outcome, _ := structured["outcome"].(string)
	return string(doc), structured, result.IsError(outcome), nil
}
