package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"kbase/internal/build"
	"kbase/internal/config"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
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
	// A duration flag's argument is a Go duration string, as the command
	// line spells it ("30s").
	"duration": {Type: "string"},
}

// mcpTools is the tool table: one tool per command under root its binding
// declares bound, each resolving its KB from dir, and beside them the one
// listed tool no command stands behind.
func mcpTools(root *cobra.Command, dir string, stderr io.Writer, lg log.Logger, launch *buildLaunch) ([]mcp.Tool, error) {
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
			tool, err = detachedBuildTool(cmd, dir, lg, launch)
		} else if cmd == scoreCmd {
			tool, err = detachedScoreTool(cmd, dir, lg)
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
	return append(tools, rewindBuildTool(dir)), nil
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
as structured content, but for rewind-build, which reports the build's trail
and is no subcommand; a result whose exit code is not 0 is an error result.
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
	cmd.Flags().StringVar(&kbRoot, "kb-root", "", "the KB to serve: its kb-root/ directory, or the repository root that holds it or will (default: the one found from the working directory)")
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
	launch := &buildLaunch{served: dir}
	tools, err := mcpTools(cmd.Root(), dir, cmd.ErrOrStderr(), processLog.logger, launch)
	if err != nil {
		return err
	}
	instructions, err := servedInstructions(dir)
	if err != nil {
		return err
	}
	return mcp.Serve(cmd.InOrStdin(), cmd.OutOrStdout(), version.Current, instructions, tools, servedResources(dir, launch),
		buildSubscription(launch, processLog.logger), processLog.logger)
}

// The instructions initialize carries: which of the three states the served
// directory is in, where the two forms name their path, and the two names a
// session needs to go on — the skill carrying the workflow and the tool
// reporting configuration. Plain text: no Markdown reaches the wire.
const (
	instructionsWorkflow = " The kbase skill carries the workflow; show-config reports how provider and model configuration resolves."
	instructionsServing  = "kbase serves the knowledge base at %s." + instructionsWorkflow
	instructionsBuilding = "kbase is registered for %s and can build its knowledge base; kb-root/ does not exist yet." + instructionsWorkflow
)

// servedInstructions is what the server tells a session at initialize, from
// the directory the tools find their KB from: the KB's path where its kb-root/
// stands, the repository's where that directory has yet to be made, and
// nothing where no repository holds the directory. A --kb-root naming the
// repository root and one naming its kb-root/ both reach the same answer.
func servedInstructions(dir string) (string, error) {
	repo, err := kb.RepositoryRoot(dir)
	if err != nil || repo == "" {
		return "", err
	}
	kbRoot := filepath.Join(repo, kb.KBDir)
	if info, err := os.Stat(kbRoot); err == nil && info.IsDir() {
		return fmt.Sprintf(instructionsServing, kbRoot), nil
	}
	return fmt.Sprintf(instructionsBuilding, repo), nil
}

// servedResources is what the server offers as resources: the served KB's
// claim-graph sheets, then the model guidance, which needs no KB, then the
// build trail of the repository holding the served KB, then the build's state.
func servedResources(dir string, launch *buildLaunch) resourceFamilies {
	return resourceFamilies{
		{prefix: sheetURIPrefix, source: sheetResources{dir}},
		{prefix: modelHintsURI, source: modelHintsResource{}},
		{prefix: buildTrailURI, source: buildTrailResource{dir}},
		{prefix: buildURI, source: buildResource{launch: launch}},
	}
}

// stateDirFlag is the flag naming a build's state store.
const stateDirFlag = "state-dir"

// buildStartPoll is how often the build tool reads the progress record while
// it waits for the build it started.
const buildStartPoll = 50 * time.Millisecond

// detachedBuildTool is cmd run as this executable in a process of its own,
// returning once the build has entered its first stage or exited.
func detachedBuildTool(cmd *cobra.Command, dir string, lg log.Logger, launch *buildLaunch) (mcp.Tool, error) {
	positionals, schema, err := commandSchema(cmd)
	if err != nil {
		return mcp.Tool{}, err
	}
	call := func(args map[string]any) (string, map[string]any, int, error) {
		var out bytes.Buffer
		if err := startDetachedBuild(cmd.Name(), positionals, args, dir, &out, lg, launch); err != nil {
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
func startDetachedBuild(verb string, positionals []positional, args map[string]any, dir string, out io.Writer, lg log.Logger, launch *buildLaunch) error {
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
	launch.launched(stateDir, pid)
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
				fields := placement
				if resume != "" {
					fields = append(fields, toolresult.Field{Key: "resume", Value: resume})
				}
				fields = append(fields, toolresult.Field{Key: "wait",
					Value: build.ShellJoin([]string{exe, "wait", "--state-dir", stateDir, "--pid", strconv.Itoa(pid)})})
				return toolresult.Emit(out, toolresult.Done, fields...)
			}
		}
	}
}

// detachedScoreTool is cmd run as this executable in a process of its own,
// returning once the score run's ledger and pid stand or it has exited.
func detachedScoreTool(cmd *cobra.Command, dir string, lg log.Logger) (mcp.Tool, error) {
	positionals, schema, err := commandSchema(cmd)
	if err != nil {
		return mcp.Tool{}, err
	}
	call := func(args map[string]any) (string, map[string]any, int, error) {
		var out bytes.Buffer
		if err := startDetachedScore(cmd.Name(), positionals, args, dir, &out, lg); err != nil {
			return "", nil, 0, err
		}
		return toolResult(out.Bytes())
	}
	return mcp.Tool{Name: cmd.Name(), InputSchema: schema, Call: call}, nil
}

// startDetachedScore starts `<this executable> score <args>` from the repository
// dir is in — in its own process group, stdin closed, stdout and stderr captured
// under the state store's reports, the environment, --config-dir, --log-level and
// --log-file inherited — and writes on out the start document once the run's
// ledger and pid stand, the run's own document where it exits first, or a
// failure naming the state store where it does neither within build.StartWait.
func startDetachedScore(verb string, positionals []positional, args map[string]any, dir string, out io.Writer, lg log.Logger) error {
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
	stdout, err := os.CreateTemp(reports, "score-"+time.Now().UTC().Format("20060102T150405Z")+"-*.stdout.yaml")
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
		return fmt.Errorf("starting the score run: %w", err)
	}
	pid := child.Process.Pid
	lg.Info("mcp: score started", "pid", pid, "stdout", stdout.Name(), "stderr", stderrPath)
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
				return fmt.Errorf("the score run (pid %d) exited (%v) with no result document; its stderr is %s", pid, waitErr, stderrPath)
			}
			_, err = out.Write(doc)
			return err
		case <-deadline.C:
			return toolresult.Emit(out, toolresult.Failed, append(placement, toolresult.Field{Key: toolresult.FailuresKey,
				Value: []toolresult.Item{{Check: checkError, Path: stateDir,
					Detail: fmt.Sprintf("the score run (pid %d) is still starting: it has neither taken the runs directory nor exited within %v; follow it with status", pid, build.StartWait)}}})...)
		case <-poll.C:
			run, err := score.RunByPID(stateDir, pid)
			if err != nil {
				continue
			}
			fields := append(placement, toolresult.Field{Key: "run", Value: run},
				toolresult.Field{Key: "ledger", Value: filepath.Join(score.RunsDir(stateDir), run)})
			fields = append(fields, toolresult.Field{Key: "wait",
				Value: build.ShellJoin([]string{exe, "wait", "--state-dir", stateDir, "--pid", strconv.Itoa(pid)})})
			return toolresult.Emit(out, toolresult.Done, fields...)
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

// modelHintsURI is the model guidance's resource URI: the rows the config.toml
// template is written with, as text that needs no KB.
const modelHintsURI = "kbase://model-hints"

// modelHintsResource serves the [models] guidance the config template carries.
type modelHintsResource struct{}

func (modelHintsResource) List() ([]mcp.Resource, error) {
	return []mcp.Resource{{URI: modelHintsURI, Name: "Model hints", MimeType: "text/markdown"}}, nil
}

// Read returns the whole guidance: it is one document, and List offers no
// other resource of this family.
func (modelHintsResource) Read(mcp.Resource) (string, error) { return config.ModelHintsMarkdown(), nil }

// rewindBuildTool is the one tool no subcommand stands behind: the build's
// commit trail, and for a target boundary the commands that reset the
// repository to it and re-issue the build. It reads the tree, the state store
// and history, and writes none of them.
func rewindBuildTool(dir string) mcp.Tool {
	return mcp.Tool{
		Name: "rewind-build",
		Description: "Every stage boundary of a build is a commit in the repository; this tool lists " +
			"them itself, no separate query, and for a target boundary, commands that reset the " +
			"repository to it and re-issue the build. A build rewound to one resumes by re-issuing " +
			"`build` with the newest boundary's inputs — the same volume roots and `--bibliography` " +
			"files, same order; other inputs refused (`check: inputs`); read-only, it changes nothing.",
		InputSchema: mcp.Schema{
			Properties: map[string]mcp.Property{
				"target":     {Type: "string", Description: "the boundary to reset to, by stage id or display name"},
				stateDirFlag: {Type: "string", Description: "the build's state store (default: $XDG_STATE_HOME/kbase/<key>)"},
			},
			Required: []string{},
		},
		ReadOnly: true,
		Call: func(args map[string]any) (string, map[string]any, int, error) {
			target, _ := args["target"].(string)
			stateDir, _ := args[stateDirFlag].(string)
			doc, err := rewindDocument(build.RewindOptions{RepoRoot: dir, StateDir: stateDir, Target: target})
			if err != nil {
				return "", nil, 0, err
			}
			return toolResult([]byte(doc))
		},
	}
}

// rewindDocument is build.Rewind's document rendered: the one document the
// rewind-build tool answers with and the build-trail resource reads.
func rewindDocument(opts build.RewindOptions) (string, error) {
	outcome, fields := build.Rewind(opts)
	var out bytes.Buffer
	if err := toolresult.Emit(&out, outcome, fields...); err != nil {
		return "", fmt.Errorf("rendering the build trail: %w", err)
	}
	return out.String(), nil
}

// buildTrailURI is the build's commit trail as one resource: the boundaries a
// build recorded and the commands that resume from one.
const buildTrailURI = "kbase://build-trail"

// buildTrailResource serves the build trail of the repository the KB sits in,
// as the rewind-build tool reports it.
type buildTrailResource struct{ dir string }

// List offers the trail whether or not a state store or a boundary exists: the
// document says what the repository holds, and reading it is how a caller
// learns the repository holds nothing.
func (buildTrailResource) List() ([]mcp.Resource, error) {
	return []mcp.Resource{{URI: buildTrailURI, Name: "Build trail", MimeType: "application/yaml"}}, nil
}

func (r buildTrailResource) Read(mcp.Resource) (string, error) {
	return rewindDocument(build.RewindOptions{RepoRoot: r.dir})
}

// buildURI is the build's state as one resource: the document the status tool
// answers with, for the store the build tool last launched from this server.
const buildURI = "kbase://build"

// buildLaunch is the build this server started last: the state store
// build.Locate resolved for it and the pid the launch recorded, so the
// resource and the watcher follow the launch rather than a store some later
// argument names. The record is the process's, so a restarted server has
// launched none.
type buildLaunch struct {
	served string

	mu       sync.Mutex
	dir      string
	pid      int
	fallback string
	resolved bool
}

// launched records the store a build was started in and the pid that runs it.
func (l *buildLaunch) launched(dir string, pid int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dir, l.pid = dir, pid
}

// store is the state store the resource reads and the watcher follows, and the
// pid of the build this server started there — zero where it started none. It
// is the launched store, else the one the served repository's build resolves,
// which is read once because resolving it asks git.
func (l *buildLaunch) store() (dir string, pid int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.dir != "" {
		return l.dir, l.pid
	}
	if !l.resolved {
		_, l.fallback, _, _ = build.Locate(build.MonitorOptions{WorkDir: l.served})
		l.resolved = true
	}
	return l.fallback, 0
}

// buildResource serves the build's state as the status tool reports it, for the
// store the build tool last launched from this server: the served repository's
// default store where it launched none.
type buildResource struct{ launch *buildLaunch }

// List offers the state whether or not a build ever ran: the document says so,
// and a read is how a caller learns it.
func (buildResource) List() ([]mcp.Resource, error) {
	return []mcp.Resource{{URI: buildURI, Name: "Build state", MimeType: "application/yaml"}}, nil
}

// Read is the status document for the store, and it never fails: a store no
// build wrote holds no run and reads state none, and a served directory no
// worktree holds reads the refusal status answers with.
func (r buildResource) Read(mcp.Resource) (string, error) {
	dir, _ := r.launch.store()
	return statusDocument(build.MonitorOptions{StateDir: dir, WorkDir: r.launch.served})
}

// statusDocument is build.Status's document rendered: the one document the
// status tool answers with and the build resource reads.
func statusDocument(opts build.MonitorOptions) (string, error) {
	outcome, fields := build.Status(opts)
	var out bytes.Buffer
	if err := toolresult.Emit(&out, outcome, fields...); err != nil {
		return "", fmt.Errorf("rendering the build's state: %w", err)
	}
	return out.String(), nil
}

// buildWatchInterval is how often the watcher reads the store after the reading
// the subscription itself makes. ARCHITECTURE §12 carries it.
const buildWatchInterval = time.Second

// buildSubscription is the subscription to the build's state: the watcher that
// reads the store the launch names and announces each run that stops
// progressing. A subscription taken again after a drop starts a new watcher,
// whose first reading is the store as it stands then.
func buildSubscription(launch *buildLaunch, lg log.Logger) *mcp.Subscription {
	return &mcp.Subscription{URI: buildURI, Watch: func(ctx context.Context, updated func()) {
		w := &buildWatch{launch: launch, lg: lg, atStart: true}
		w.watch(ctx, updated)
	}}
}

// buildWatch is one subscription's following of the build's state: the run it
// last read, and whether it has announced that run's end.
type buildWatch struct {
	launch  *buildLaunch
	lg      log.Logger
	atStart bool
	seen    runID
	done    bool
}

// runID is one run of a build: the pid that ran it and its start stamp, which
// together name no other run — a resumed build is a new pid, a new stamp.
type runID struct {
	pid     int
	started string
}

// watch reads the store at the subscription and then every
// buildWatchInterval, announcing each run that is no longer progressing, until
// the subscription ends.
func (w *buildWatch) watch(ctx context.Context, updated func()) {
	tick := time.NewTicker(buildWatchInterval)
	defer tick.Stop()
	for {
		w.read(updated)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// read is one reading of the store, announcing a run that ended since the last
// one announced.
func (w *buildWatch) read(updated func()) {
	dir, launched := w.launch.store()
	if dir == "" {
		w.atStart = false
		return
	}
	state, err := build.ReadState(dir)
	if err != nil {
		w.lg.Error("mcp: reading the build's state failed", "state-dir", dir, "err", err)
		w.atStart = false
		return
	}
	if w.announce(state, launched) {
		updated()
	}
}

// announce is whether the reading is a run's end to tell the client about: a
// run that is over and has not been announced. A run already over at the
// subscription's first reading is announced there — the event is the only
// channel — but only for a run this server launched, or for any run where it
// launched none: a launch is not atomic, and until the child takes the store's
// lock the run before it reads interrupted, which is no new build's end.
func (w *buildWatch) announce(state build.State, launched int) bool {
	atStart := w.atStart
	w.atStart = false
	id := runID{pid: state.PID, started: state.Started}
	if id != w.seen {
		w.seen, w.done = id, false
	}
	switch {
	case w.done, !build.Ended(state.State):
		return false
	case atStart && launched != 0 && launched != state.PID:
		// Consume it: this reading is dropped, and a later one of the same run
		// must not take it for the new build's end.
		w.done = true
		return false
	}
	w.done = true
	return true
}

// resourceFamily is one resource source and the URI prefix it answers.
type resourceFamily struct {
	prefix string
	source mcp.Resources
}

// resourceFamilies serves the families as one Resources: List is each family's
// in order, and Read goes to the family whose prefix the URI carries.
type resourceFamilies []resourceFamily

// List is every family that lists, in order. A family refusing what it lists
// from — a KB that exists but will not open — contributes no entries and the
// rest still list: the refusal is a fact about the KB, which status and verify
// report when asked, not a fault in the list. Any other error is a fault, and
// is returned as the family raised it, so joining families changes what is
// listed, not what any one of them answers.
func (f resourceFamilies) List() ([]mcp.Resource, error) {
	var listed []mcp.Resource
	for _, family := range f {
		one, err := family.source.List()
		var refusal toolresult.Refusal
		if errors.As(err, &refusal) {
			continue
		}
		if err != nil {
			return nil, err
		}
		listed = append(listed, one...)
	}
	return listed, nil
}

// Read goes to the family serving the URI: the one whose prefix is longest,
// since one resource's URI may lead another's, as kbase://build leads
// kbase://build-trail.
func (f resourceFamilies) Read(res mcp.Resource) (string, error) {
	i := -1
	for j, family := range f {
		if strings.HasPrefix(res.URI, family.prefix) && (i < 0 || len(family.prefix) > len(f[i].prefix)) {
			i = j
		}
	}
	if i < 0 {
		return "", fmt.Errorf("no source serves %q", res.URI)
	}
	return f[i].source.Read(res)
}
