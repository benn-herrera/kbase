package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"kbase/internal/build"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/version"
	"kbase/internal/write"
)

// readOnlyAnnotation marks a command that writes nothing; its MCP tool
// carries the mark as its read-only hint.
const readOnlyAnnotation = "kbase.readOnly"

func readOnly(cmd *cobra.Command) *cobra.Command { return annotate(cmd, readOnlyAnnotation, "true") }

// mcpAnnotation is a command's MCP binding, declared where the command is
// built: mcpBound where a tool binds it, mcpExcluded where none does. A
// command declaring neither is bound by no tool.
const (
	mcpAnnotation = "kbase.mcp"
	mcpBound      = "bound"
	mcpExcluded   = "excluded"
)

// mcpBinding declares cmd's MCP binding.
func mcpBinding(cmd *cobra.Command, binding string) *cobra.Command {
	return annotate(cmd, mcpAnnotation, binding)
}

func annotate(cmd *cobra.Command, key, value string) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[key] = value
	return cmd
}

type invocationDirKey struct{}

// invocationDir is where a verb's search for the repository root starts: the
// directory an MCP tool call names, else the working directory.
func invocationDir(cmd *cobra.Command) (string, error) {
	if ctx := cmd.Context(); ctx != nil {
		if dir, ok := ctx.Value(invocationDirKey{}).(string); ok {
			return dir, nil
		}
	}
	return os.Getwd()
}

// resetFlags puts cmd's own flags back to their defaults, as a new process
// would find them; the global flags it inherits are the server's and stay.
func resetFlags(cmd *cobra.Command) error {
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

// positional is one positional argument a command declares in its Use line:
// <name>, or <name>... where it repeats.
type positional struct {
	name     string
	variadic bool
}

func declaredPositionals(cmd *cobra.Command) ([]positional, error) {
	words := strings.Fields(cmd.Use)[1:]
	declared := make([]positional, 0, len(words))
	for i, w := range words {
		name, variadic := strings.CutSuffix(w, "...")
		if !strings.HasPrefix(name, "<") || !strings.HasSuffix(name, ">") || variadic && i != len(words)-1 {
			return nil, fmt.Errorf("%s: %q in its usage line is not a positional <name>", cmd.Name(), w)
		}
		declared = append(declared, positional{strings.Trim(name, "<>"), variadic})
	}
	return declared, nil
}

// flagSchemaTypes is the JSON Schema of each flag type a tool takes.
var flagSchemaTypes = map[string]mcp.Property{
	"bool": {Type: "boolean"}, "string": {Type: "string"}, "int": {Type: "integer"}, "float64": {Type: "number"},
	"stringArray": {Type: "array", Items: &mcp.Property{Type: "string"}},
}

// mcpTools is the tool table: one tool per command under root its binding
// declares bound, each resolving its KB from dir.
func mcpTools(root *cobra.Command, dir string, stderr io.Writer, lg log.Logger) ([]mcp.Tool, error) {
	var tools []mcp.Tool
	for _, cmd := range root.Commands() {
		if cmd.Annotations[mcpAnnotation] != mcpBound {
			continue
		}
		var tool mcp.Tool
		var err error
		if fields, create, ok := write.OpVocabulary(cmd.Name()); ok {
			tool, err = writeOpTool(cmd.Name(), fields, create, dir, stderr, lg)
		} else if cmd == buildCmd {
			tool, err = detachedBuildTool(cmd, dir, lg)
		} else {
			tool, err = commandTool(cmd, dir, stderr)
		}
		if err != nil {
			return nil, err
		}
		tool.Description = cmd.Short
		tool.ReadOnly = cmd.Annotations[readOnlyAnnotation] == "true"
		tools = append(tools, tool)
	}
	return tools, nil
}

// createArgument is the write-op argument standing for --create.
const createArgument = "create"

// writeOpTool is op over one entry: the arguments are the entry, create
// aside.
func writeOpTool(op string, fields []write.OpField, create bool, dir string, stderr io.Writer, lg log.Logger) (mcp.Tool, error) {
	schema := mcp.Schema{Properties: map[string]mcp.Property{}, Required: []string{}}
	for _, f := range fields {
		schema.Properties[f.Name] = mcp.Property{}
		if f.Required {
			schema.Required = append(schema.Required, f.Name)
		}
	}
	if create {
		if _, clash := schema.Properties[createArgument]; clash {
			return mcp.Tool{}, fmt.Errorf("%s: the values key %q stands where --create would", op, createArgument)
		}
		schema.Properties[createArgument] = mcp.Property{Type: "boolean", Description: "create the register when it does not exist yet"}
	}
	call := func(args map[string]any) (string, map[string]any, int, error) {
		entry := maps.Clone(args)
		createRegister, _ := entry[createArgument].(bool)
		if create {
			delete(entry, createArgument)
		}
		values, err := json.Marshal(map[string]any{"entry": []any{entry}})
		if err != nil {
			return "", nil, 0, err
		}
		var out bytes.Buffer
		if _, err := runWriteOp(writeOpOptions{Op: op, WorkDir: dir, Stdin: bytes.NewReader(values), Create: createRegister,
			ArgumentEntry: true, Stdout: &out, Stderr: stderr, Logger: lg}); err != nil {
			return "", nil, 0, err
		}
		return toolResult(out.Bytes())
	}
	return mcp.Tool{Name: op, InputSchema: schema, Call: call}, nil
}

// commandSchema is cmd's positionals and its tool's input schema: the
// positionals required, then its own flags.
func commandSchema(cmd *cobra.Command) ([]positional, mcp.Schema, error) {
	positionals, err := declaredPositionals(cmd)
	if err != nil {
		return nil, mcp.Schema{}, err
	}
	schema := mcp.Schema{Properties: map[string]mcp.Property{}, Required: []string{}}
	for _, p := range positionals {
		property := mcp.Property{Type: "string"}
		if p.variadic {
			property = mcp.Property{Type: "array", Items: &mcp.Property{Type: "string"}}
		}
		schema.Properties[p.name] = property
		schema.Required = append(schema.Required, p.name)
	}
	cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
		if err != nil || f.Name == "help" {
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
func commandTool(cmd *cobra.Command, dir string, stderr io.Writer) (mcp.Tool, error) {
	positionals, schema, err := commandSchema(cmd)
	if err != nil {
		return mcp.Tool{}, err
	}
	call := func(args map[string]any) (string, map[string]any, int, error) {
		var out bytes.Buffer
		if err := runCommandTool(cmd, positionals, args, dir, &out, stderr); err != nil {
			return "", nil, 0, err
		}
		return toolResult(out.Bytes())
	}
	return mcp.Tool{Name: cmd.Name(), InputSchema: schema, Call: call}, nil
}

// runCommandTool runs cmd over args from dir, its result document on out,
// and leaves cmd's streams and context as it found them and its flags at
// their defaults.
func runCommandTool(cmd *cobra.Command, positionals []positional, args map[string]any, dir string, out, stderr io.Writer) error {
	if err := resetFlags(cmd); err != nil {
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
	argv, flags := commandLine(positionals, args)
	err := cmd.ValidateArgs(argv)
	for _, f := range flags {
		if err != nil {
			break
		}
		err = cmd.Flags().Set(f.name, f.value)
	}
	if err == nil {
		err = cmd.RunE(cmd, argv)
	}
	if err != nil {
		exitStatus(err, out, stderr)
	}
	return resetFlags(cmd)
}

// flagArgument is one flag setting: --name value.
type flagArgument struct{ name, value string }

// commandLine is args as the command line spells them: the positionals in
// their declared order, then a setting per flag value in name order, one for
// each item of an array.
func commandLine(positionals []positional, args map[string]any) (argv []string, flags []flagArgument) {
	for _, p := range positionals {
		argv = append(argv, argumentTexts(args[p.name])...)
	}
	for _, name := range slices.Sorted(maps.Keys(args)) {
		if slices.ContainsFunc(positionals, func(p positional) bool { return p.name == name }) {
			continue
		}
		for _, v := range argumentTexts(args[name]) {
			flags = append(flags, flagArgument{name, v})
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

// toolResult is a result document as an MCP tool's result: the document,
// the document decoded, and its exit code.
func toolResult(doc []byte) (string, map[string]any, int, error) {
	var structured map[string]any
	if err := yaml.Unmarshal(doc, &structured); err != nil {
		return "", nil, 0, fmt.Errorf("decoding the result document: %w", err)
	}
	outcome, _ := structured["outcome"].(string)
	return string(doc), structured, exitCodes[outcome], nil
}

func init() {
	var kbRoot string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "serve the KB's subcommands as MCP tools over stdio",
		Long: `mcp serves the KB at --kb-root to an agent harness over the Model Context
Protocol: newline-delimited JSON-RPC on stdin and stdout. Every tool is one
subcommand, with the same arguments and the same result document, as text and
as structured content; a result whose exit code is not 0 is an error result.
Without --kb-root the KB is found from the working directory, as every
subcommand finds it. stdout carries protocol frames and nothing else;
diagnostics go to stderr and --log-file. EOF on stdin ends the server.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			if kbRoot != "" {
				root, refusal, err := servedKBRoot(kbRoot)
				if err != nil {
					return err
				}
				if refusal != nil {
					outcome, fields := refused(nil, *refusal)
					code, err := emitResult("mcp", cmd.OutOrStdout(), outcome, fields)
					if err != nil {
						return err
					}
					return exitCode(code)
				}
				dir = root
			}
			if err := serveMCP(cmd, dir); err != nil {
				processLog.logger.Error("mcp: the server stopped", "err", err)
				return exitCode(exitCodes[toolresult.Failed])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kbRoot, "kb-root", "", "the KB to serve: its kb-root/, or the repository root holding it (default: the one found from the working directory)")
	rootCmd.AddCommand(mcpBinding(cmd, mcpExcluded))
}

// servedKBRoot is the directory the tools find their KB from, given the path
// --kb-root names: a repository root, whether or not its kb-root/ exists yet,
// or that kb-root/ itself. Any other path is a usage refusal.
func servedKBRoot(path string) (string, *toolresult.Item, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		repo, err := kb.RepositoryRoot(abs)
		if err != nil {
			return "", nil, err
		}
		if repo != "" && (abs == repo || abs == filepath.Join(repo, kb.KBDir)) {
			return abs, nil, nil
		}
	}
	return "", &toolresult.Item{Check: checkUsage, Path: abs, Key: "--kb-root",
		Detail: fmt.Sprintf("--kb-root %s is neither a repository root nor the %s/ directory beside its .git", abs, kb.KBDir)}, nil
}

func serveMCP(cmd *cobra.Command, dir string) error {
	tools, err := mcpTools(cmd.Root(), dir, cmd.ErrOrStderr(), processLog.logger)
	if err != nil {
		return err
	}
	return mcp.Serve(cmd.InOrStdin(), cmd.OutOrStdout(), version.Current, tools, sheetResources{dir}, processLog.logger)
}

// stateDirFlag is the flag naming a build's state store.
const stateDirFlag = "state-dir"

// buildStartPoll is how often the build tool reads the progress record while
// it waits for the build it started.
const buildStartPoll = 50 * time.Millisecond

// detachedBuildTool is cmd run as this executable in a process of its own,
// returning once the build has entered its first stage or exited.
func detachedBuildTool(cmd *cobra.Command, dir string, lg log.Logger) (mcp.Tool, error) {
	positionals, schema, err := commandSchema(cmd)
	if err != nil {
		return mcp.Tool{}, err
	}
	call := func(args map[string]any) (string, map[string]any, int, error) {
		var out bytes.Buffer
		if err := startDetachedBuild(cmd.Name(), positionals, args, dir, &out, lg); err != nil {
			return "", nil, 0, err
		}
		return toolResult(out.Bytes())
	}
	return mcp.Tool{Name: cmd.Name(), InputSchema: schema, Call: call}, nil
}

// startDetachedBuild starts `<this executable> verb <args>` from the
// repository dir is in — in its own process group, stdin closed, stdout and
// stderr captured under the state store's reports, the environment,
// --config-dir, --log-level and --log-file inherited — and writes on out the start document once the
// build has entered a stage, the build's own document where it exits first,
// or a failure naming the state store where it does neither within
// build.StartWait. A relative state-dir is the repository root's, as the
// volume roots are in the build's working directory.
func startDetachedBuild(verb string, positionals []positional, args map[string]any, dir string, out io.Writer, lg log.Logger) error {
	args = maps.Clone(args)
	stateFlag, _ := args[stateDirFlag].(string)
	kbRoot, stateDir, outcome, fields := build.Locate(build.MonitorOptions{StateDir: stateFlag, WorkDir: dir, Logger: lg})
	if outcome != "" {
		return toolresult.Emit(out, outcome, fields...)
	}
	if stateFlag != "" {
		args[stateDirFlag] = stateDir
	}
	positionalArgv, flags := commandLine(positionals, args)
	argv := []string{verb}
	for _, f := range flags {
		argv = append(argv, "--"+f.name+"="+f.value)
	}
	for _, global := range []struct {
		name, value string
		path        bool
	}{{"config-dir", strings.TrimSpace(flagConfigDir), true}, {"log-level", flagLogLevel, false}, {"log-file", flagLogFile, true}} {
		if global.value == "" {
			continue
		}
		if global.path {
			abs, err := filepath.Abs(global.value)
			if err != nil {
				return err
			}
			global.value = abs
		}
		argv = append(argv, "--"+global.name+"="+global.value)
	}
	argv = append(append(argv, "--"), positionalArgv...)
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	reports := filepath.Join(stateDir, build.ReportsDir)
	if err := os.MkdirAll(reports, 0o777); err != nil {
		return err
	}
	stdout, err := os.CreateTemp(reports, build.CapturePrefix+time.Now().UTC().Format("20060102T150405Z")+"-*.stdout.yaml")
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderrPath := strings.TrimSuffix(stdout.Name(), ".stdout.yaml") + ".stderr.log"
	stderr, err := os.OpenFile(stderrPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	defer stderr.Close()

	child := exec.Command(exe, argv...)
	child.Dir = filepath.Dir(kbRoot)
	child.Stdout, child.Stderr = stdout, stderr
	child.SysProcAttr = detachedProcess()
	if err := child.Start(); err != nil {
		return fmt.Errorf("starting the build: %w", err)
	}
	pid := child.Process.Pid
	lg.Info("mcp: build started", "pid", pid, "stdout", stdout.Name(), "stderr", stderrPath)
	// Waiting reaps the child; the goroutine ends when it exits, whether or
	// not this call is still listening.
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()

	placement := []toolresult.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: stateDir}, {Key: "pid", Value: pid}}
	deadline := time.NewTimer(build.StartWait)
	defer deadline.Stop()
	poll := time.NewTicker(buildStartPoll)
	defer poll.Stop()
	for {
		select {
		case waitErr := <-exited:
			doc, err := os.ReadFile(stdout.Name())
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(doc)) == 0 {
				return fmt.Errorf("the build (pid %d) exited (%v) with no result document; its stderr is %s", pid, waitErr, stderrPath)
			}
			_, err = out.Write(doc)
			return err
		case <-deadline.C:
			return toolresult.Emit(out, toolresult.Failed, append(placement, toolresult.Field{Key: toolresult.FailuresKey,
				Value: []toolresult.Item{{Check: checkError, Path: stateDir,
					Detail: fmt.Sprintf("the build (pid %d) is still starting: it has neither entered a stage nor exited within %v; follow it with status", pid, build.StartWait)}}})...)
		case <-poll.C:
			entered, resume, err := build.Launched(stateDir, pid)
			if err != nil {
				return err
			}
			if entered {
				return toolresult.Emit(out, toolresult.Done, append(placement, toolresult.Field{Key: "resume", Value: resume})...)
			}
		}
	}
}

// sheetURIPrefix leads a claim-graph sheet's resource URI; its
// kb-root-relative slash path follows.
const sheetURIPrefix = "kbase://sheet/"

// sheetResources offers the claim-graph sheets of the KB found from dir.
type sheetResources struct{ dir string }

// source is the KB found from dir, nil where there is none yet.
func (r sheetResources) source() (*kb.Source, error) {
	root, items, err := kbRootFrom(r.dir)
	if err != nil || items != nil {
		return nil, err
	}
	src, items, err := openKB(root)
	if items != nil {
		return nil, toolresult.Refusal(items)
	}
	return src, err
}

func (r sheetResources) List() ([]mcp.Resource, error) {
	src, err := r.source()
	if err != nil || src == nil {
		return nil, err
	}
	_, present, err := index.SheetPaths(src)
	if err != nil {
		return nil, err
	}
	var sheets []mcp.Resource
	for _, rel := range present {
		sheets = append(sheets, mcp.Resource{URI: sheetURIPrefix + rel, Name: rel, MimeType: "image/svg+xml"})
	}
	return sheets, nil
}

func (r sheetResources) Read(sheet mcp.Resource) (string, error) {
	src, err := r.source()
	if err != nil {
		return "", err
	}
	if src == nil {
		return "", fmt.Errorf("no KB is found from %s", r.dir)
	}
	data, err := src.ReadFile(src.KBPath(strings.TrimPrefix(sheet.URI, sheetURIPrefix)))
	return string(data), err
}
