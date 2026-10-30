package mcp

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

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
	// awaited is what a run still starting has not yet done.
	awaited string
	// started is whether the run pid has started in the store at stateDir, and
	// the fields its done document carries after the placement.
	started func(stateDir string, pid int) (bool, []result.Field, error)
	// recorded is whether the server's record of the build it launched last
	// takes each launch.
	recorded bool
}

// BuildLaunch is the build's launch: started once its progress record shows a
// stage entered, each launch recorded as the server's last build.
func BuildLaunch() Launch {
	return Launch{noun: "the build", capturePrefix: buildstore.CapturePrefix, awaited: "entered a stage",
		started: buildStarted, recorded: true}
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
// stand.
func ScoreLaunch() Launch {
	return Launch{noun: "the score run", capturePrefix: score.CapturePrefix, awaited: "opened its ledger",
		started: scoreStarted}
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
	call := func(p Project, args map[string]any) (string, map[string]any, bool, error) {
		var out bytes.Buffer
		if err := l.start(cmd.Name(), positionals, args, p, s, globals, &out); err != nil {
			return "", nil, false, err
		}
		return ToolResult(out.Bytes())
	}
	return ProjectTool{Tool: protocol.Tool{Name: cmd.Name(), InputSchema: schema}, Run: call}, nil
}

// start starts `<this executable> verb <args>` from the root of the project p
// — in its own process group, stdin closed, stdout and stderr captured under
// the state store's reports, the environment and globals inherited, and
// --project-dir where the call named p — and writes on out the start document
// once the run has started, the run's own document where it exits first, or a
// failure naming the state store where it does neither within
// buildstore.StartWait. A relative state-dir is the project root's, as the
// volume roots are in the run's working directory.
func (l Launch) start(verb string, positionals []Positional, args map[string]any, p Project, s serving, globals Globals, out io.Writer) error {
	args = maps.Clone(args)
	stateFlag, _ := args[stateDirFlag].(string)
	kbRoot, stateDir, outcome, fields := buildstore.Locate(buildstore.MonitorOptions{StateDir: stateFlag, WorkDir: p.Root, Logger: s.lg, DefaultStore: true})
	if outcome != "" {
		return result.Emit(out, outcome, fields...)
	}
	if stateFlag != "" {
		args[stateDirFlag] = stateDir
	}
	positionalArgv, flags := CommandLine(positionals, args)
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
	child.Stdout, child.Stderr = stdout, stderr
	child.SysProcAttr = detachedProcess()
	if err := child.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", l.noun, err)
	}
	pid := child.Process.Pid
	if l.recorded {
		s.last.launched(p.Root, stateDir, pid)
	}
	s.lg.Info("mcp: "+verb+" started", "pid", pid, "stdout", stdout.Name(), "stderr", stderrPath)
	// Waiting reaps the child; the goroutine ends when it exits, whether or
	// not this call is still listening.
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()

	placement := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: stateDir}, {Key: "pid", Value: pid}}
	deadline := time.NewTimer(buildstore.StartWait)
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
			return result.Emit(out, result.Failed, append(placement, result.Field{Key: result.FailuresKey,
				Value: []result.Item{{Check: buildstore.CheckError, Path: stateDir,
					Detail: fmt.Sprintf("%s (pid %d) is still starting: it has neither %s nor exited within %v; follow it with status", l.noun, pid, l.awaited, buildstore.StartWait)}}})...)
		case <-poll.C:
			started, done, err := l.started(stateDir, pid)
			if err != nil {
				return err
			}
			if started {
				wait := append(append([]string{exe, "wait"}, waitProject...), "--state-dir", stateDir, "--pid", strconv.Itoa(pid))
				fields := append(append(placement, done...), result.Field{Key: "wait", Value: buildstore.ShellJoin(wait)})
				return result.Emit(out, result.Done, fields...)
			}
		}
	}
}

// lastBuild is the build a server started last: the project it ran in, the
// state store buildstore.Locate resolved for it and the pid the launch
// recorded, so the build resource and its watcher follow the launch rather
// than a store some later argument names. The record is the process's, so a
// restarted server has launched none.
type lastBuild struct {
	located Located

	mu       sync.Mutex
	project  string
	dir      string
	pid      int
	fallback string
	resolved bool
}

// newLastBuild is the record of a server with the located project, or none,
// that has launched no build yet.
func newLastBuild(located Located) *lastBuild { return &lastBuild{located: located} }

// launched records the project a build was started in, its state store and
// the pid that runs it.
func (l *lastBuild) launched(project, dir string, pid int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.project, l.dir, l.pid = project, dir, pid
}

// store is the project and state store the resource reads and the watcher
// follows, and the pid of the build this server started there — zero where it
// started none. It is the launch's, else the located project and the store its
// build resolves, which is read once because resolving it asks git; ok is
// false where the server launched none and located no project.
func (l *lastBuild) store() (project, dir string, pid int, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.dir != "" {
		return l.project, l.dir, l.pid, true
	}
	root, ok := l.located.Root()
	if !ok {
		return "", "", 0, false
	}
	if !l.resolved {
		_, l.fallback, _, _ = buildstore.Locate(buildstore.MonitorOptions{WorkDir: root, DefaultStore: true})
		l.resolved = true
	}
	return root, l.fallback, 0, true
}
