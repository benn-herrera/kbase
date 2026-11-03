package mcp

import (
	"bytes"
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

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	"kbase/internal/mcp/protocol"
	"kbase/internal/result"
	"kbase/internal/score"
)

// stateDirFlag is the flag naming a run's state store.
const stateDirFlag = "state-dir"

// launchPoll is how often a launch reads the state store while it waits for
// the run it started.
const launchPoll = 50 * time.Millisecond

// launchStarting is how long a launch waits for the run it started to start
// before it reports the run still starting, done, for status to follow.
const launchStarting = 5 * time.Second

// Globals is the server's own global flags, which every run it launches
// inherits.
type Globals struct{ ConfigDir, LogLevel, LogFile string }

// Launch is how a command's tool starts it as a process of its own and tells
// that the run has started.
type Launch struct {
	// noun names the run in messages: "the build".
	noun string
	// capturePrefix begins the names of the files capturing the run's output.
	capturePrefix string
	// started is whether the run pid has started in the store at stateDir, and
	// the fields its done document carries after the placement.
	started func(stateDir string, pid int) (bool, []result.Field, error)
}

// BuildLaunch is the build's launch: started once its progress record shows a
// stage entered, reported still starting after 5 s.
func BuildLaunch() Launch {
	return Launch{noun: "the build", capturePrefix: buildstore.CapturePrefix, started: buildStarted}
}

// buildStarted adds the resume command where the record's command reads as a
// kbase build invocation.
func buildStarted(stateDir string, pid int) (bool, []result.Field, error) {
	entered, resume, err := buildstore.Launched(stateDir, pid)
	if err != nil || !entered {
		return false, nil, err
	}
	if resume == "" {
		return true, nil, nil
	}
	return true, []result.Field{{Key: "resume", Value: resume}}, nil
}

// ScoreLaunch is the score run's launch: started once its ledger and pid
// stand, reported still starting after 5 s.
func ScoreLaunch() Launch {
	return Launch{noun: "the score run", capturePrefix: score.CapturePrefix, started: scoreStarted}
}

// scoreStarted reads a run whose ledger or pid does not read back yet as not
// started, so the launch polls again.
func scoreStarted(stateDir string, pid int) (bool, []result.Field, error) {
	run, err := score.RunByPID(stateDir, pid)
	if err != nil {
		return false, nil, nil
	}
	return true, []result.Field{{Key: "run", Value: run}, {Key: "ledger", Value: filepath.Join(score.RunsDir(stateDir), run)}}, nil
}

// launchTool is cmd run by l as this executable in a process of its own,
// returning once the run has started or exited.
func launchTool(cmd *cobra.Command, l Launch, s serving, globals Globals) (ProjectTool, error) {
	positionals, schema, err := CommandSchema(cmd)
	if err != nil {
		return ProjectTool{}, err
	}
	mappings := Mappings(cmd)
	call := func(p Project, args map[string]any) (string, map[string]any, bool, error) {
		var out bytes.Buffer
		if refusal := commaInMapping(mappings, args); refusal != nil {
			if err := result.Emit(result.Spelled(&out, s.spelling), result.Refused,
				result.Field{Key: result.RefusalsKey, Value: []result.Item{*refusal}}); err != nil {
				return "", nil, false, err
			}
			return ToolResult(out.Bytes())
		}
		if err := l.start(cmd.Name(), positionals, mappings, args, p, s, globals, result.Spelled(&out, s.spelling)); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return ProjectTool{Tool: protocol.Tool{Name: cmd.Name(), InputSchema: schema}, Run: call}, nil
}

// commaInMapping is the usage refusal of the first mapping argument of args —
// one mappings names — whose value under one of its keys holds a comma, which
// the command line reads as the separator between the mapping's values; nil
// where none does. The schema has already admitted each mapping's keys.
func commaInMapping(mappings map[string][]string, args map[string]any) *result.Item {
	for _, name := range slices.Sorted(maps.Keys(mappings)) {
		m, _ := args[name].(map[string]any)
		for _, key := range mappings[name] {
			if v, _ := m[key].(string); strings.Contains(v, ",") {
				return &result.Item{Check: result.CheckUsage, Key: argmark.Flag(name),
					Detail: argmark.Flag(name) + "'s " + key + " holds a comma, and none of its values may: " + strconv.Quote(v)}
			}
		}
	}
	return nil
}

// start starts `<this executable> verb <args>` from the root of the project p
// — in its own process group, stdin closed, stdout and stderr captured under
// the state store's reports, the environment and globals inherited, SurfaceEnv
// telling it its document is read over MCP, and --project-dir where the call
// named p — and writes on out the start document once the run has started,
// the run's own document where it exits first, or, where it does neither
// within launchStarting, a done document saying it is still starting. A
// relative state-dir is the project root's, as the volume roots are in the
// run's working directory.
func (l Launch) start(verb string, positionals []Positional, mappings map[string][]string, args map[string]any, p Project, s serving, globals Globals, out io.Writer) error {
	args = maps.Clone(args)
	stateFlag, _ := args[stateDirFlag].(string)
	kbRoot, stateDir, outcome, fields := buildstore.Locate(buildstore.MonitorOptions{StateDir: stateFlag, WorkDir: p.Root, Logger: s.lg, DefaultStore: true})
	if outcome != "" {
		return result.Emit(out, outcome, fields...)
	}
	if stateFlag != "" {
		args[stateDirFlag] = stateDir
	}
	positionalArgv, flags := CommandLine(positionals, mappings, args)
	argv := []string{verb}
	for _, f := range flags {
		argv = append(argv, "--"+f.Name+"="+f.Value)
	}
	var waitProject []string
	if p.Named {
		waitProject = []string{"--project-dir", p.Root}
		argv = append(argv, "--project-dir="+p.Root)
	}
	for _, global := range []struct {
		name, value string
		path        bool
	}{{"config-dir", strings.TrimSpace(globals.ConfigDir), true}, {"log-level", globals.LogLevel, false}, {"log-file", globals.LogFile, true}} {
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
	reports := filepath.Join(stateDir, buildstore.ReportsDir)
	if err := os.MkdirAll(reports, 0o777); err != nil {
		return err
	}
	stdout, err := os.CreateTemp(reports, l.capturePrefix+time.Now().UTC().Format("20060102T150405Z")+"-*"+buildstore.CaptureStdoutSuffix)
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderrPath := strings.TrimSuffix(stdout.Name(), buildstore.CaptureStdoutSuffix) + buildstore.CaptureStderrSuffix
	stderr, err := os.OpenFile(stderrPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	defer stderr.Close()

	child := exec.Command(exe, argv...)
	child.Dir = filepath.Dir(kbRoot)
	child.Env = append(os.Environ(), SurfaceEnv+"="+SurfaceMCP)
	child.Stdout, child.Stderr = stdout, stderr
	child.SysProcAttr = detachedProcess()
	if err := child.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", l.noun, err)
	}
	pid := child.Process.Pid
	s.lg.Info("mcp: "+verb+" started", "pid", pid, "stdout", stdout.Name(), "stderr", stderrPath)
	// Waiting reaps the child; the goroutine ends when it exits, whether or
	// not this call is still listening.
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()

	placement := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: stateDir}, {Key: "pid", Value: pid}}
	wait := result.Field{Key: "wait", Value: buildstore.ShellJoin(append(append([]string{exe, "wait"}, waitProject...),
		"--state-dir", stateDir, "--pid", strconv.Itoa(pid)))}
	deadline := time.NewTimer(launchStarting)
	defer deadline.Stop()
	poll := time.NewTicker(launchPoll)
	defer poll.Stop()
	for {
		select {
		case waitErr := <-exited:
			doc, err := os.ReadFile(stdout.Name())
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(doc)) == 0 {
				return fmt.Errorf("%s (pid %d) exited (%v) with no result document; its stderr is %s", l.noun, pid, waitErr, stderrPath)
			}
			_, err = out.Write(doc)
			return err
		case <-deadline.C:
			return result.Emit(out, result.Done, result.Field{Key: "state", Value: "starting"},
				result.Field{Key: "state-dir", Value: stateDir}, result.Field{Key: "pid", Value: pid}, wait)
		case <-poll.C:
			started, done, err := l.started(stateDir, pid)
			if err != nil {
				return err
			}
			if started {
				return result.Emit(out, result.Done, append(append(placement, done...), wait)...)
			}
		}
	}
}
