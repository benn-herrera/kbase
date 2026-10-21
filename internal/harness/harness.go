// Package harness runs one command per item: the argv the back end states, the
// prompt an ask is handed, and the reply the command answers with.
//
// It knows nothing of kbase — no provider, no endpoint, no KB path, no
// configuration type — and composes no prompt: it receives one and hands it
// over whole. A command is an agent, so one item is one process, created for it
// and reaped before the next; nothing crosses from one item to the next but the
// reply.
package harness

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"kbase/internal/log"
)

// The bounds one item's process is held to. A reply is one value, and a
// command's stderr is its progress and its own errors; a command that writes
// past either bound fails its item rather than exhausting the run. PromptLimit
// keeps one argv element inside what the host can carry.
const (
	// ReplyLimit bounds the reply read from the command's stdout.
	ReplyLimit = 64 << 10
	// StderrLimit bounds the stderr captured for a failure's reason.
	StderrLimit = 64 << 10
	// PromptLimit bounds one item's prompt.
	PromptLimit = 64 << 10
)

// tokenPrompt is the argv token a command carries where the prompt goes. An
// element naming it receives the prompt and stdin is closed empty; an argv
// naming it nowhere receives the prompt on stdin.
const tokenPrompt = "{prompt}"

// waitDelay bounds how long Wait waits for the child's output pipes after the
// child — or its process group — is gone. A command that starts a daemon of its
// own leaves a writer behind, and without this the item would wait on a pipe no
// kbase-owned process will close.
const waitDelay = 5 * time.Second

// Command is one harness command: the argv the back end states, the program
// that argv names, the environment it runs in, and — once the item is known —
// the prompt, held here when no argv element names it.
//
// The prompt lives on the command rather than being handed to Ask, so Ask
// cannot be called with a prompt the argv already carried, and cannot leave one
// unsent.
type Command struct {
	program string // argv[0] resolved at New: the program every item starts
	argv    []string
	env     []string // the child's environment; nil inherits kbase's
	prompt  string
	lg      log.Logger
}

// New is the setup probe: argv[0] resolved on the command's PATH, and no
// execution. A command that cannot start is refused here, before any item is
// asked, because a run that asked nothing has nothing to report per item.
//
// The program is resolved once and absolutely, and that path is what an item
// starts: an item runs in the run's own directory, where a path relative to
// kbase's would name another program or none. A relative argv[0] carrying a
// separator is kept rather than refused, resolved against the directory kbase
// was launched from — that is the only place a command naming ./bin/agent can
// mean, and refusing it would forbid a local command standing exactly where it
// says. An element naming {prompt} twice is refused here too: that is a command
// defect, and it fails setup rather than every item.
func New(argv []string, lg log.Logger) (*Command, error) {
	if len(argv) == 0 {
		return nil, errors.New("the command is empty")
	}
	program, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, fmt.Errorf("%s cannot be started: %w", argv[0], err)
	}
	if !filepath.IsAbs(program) {
		if program, err = filepath.Abs(program); err != nil {
			return nil, fmt.Errorf("%s cannot be started: %w", argv[0], err)
		}
	}
	for _, element := range argv {
		if strings.Count(element, tokenPrompt) > 1 {
			return nil, fmt.Errorf("the command names %s more than once in one element (%q): the prompt reaches a command once",
				tokenPrompt, element)
		}
	}
	if lg == nil {
		lg = log.Discard()
	}
	return &Command{program: program, argv: slices.Clone(argv), lg: lg}, nil
}

// Argv is the command as it stands, argv[0] as the back end wrote it: on the
// probed command — the one a run records what invoked it — the token naming the
// prompt is still the token, so the record names how the run was invoked and
// never the text of one item. The program that invocation starts is the one New
// resolved; the back end's own command is not aliased, so filling an item's
// prompt cannot reach it.
func (c *Command) Argv() []string { return slices.Clone(c.argv) }

// WithEnv is the command with the environment its process starts in: env is
// the whole of it, so a caller that composes one composes the base too, and nil
// leaves the child kbase's own environment. The environment is the caller's to
// shape — this package reads nothing out of it.
func (c Command) WithEnv(env []string) Command {
	c.env = env
	return c
}

// Item is one item's command built from the probed argv: {prompt} filled in the
// one element that names it, or held for stdin where none does. What the token
// decides is a property of the command, so it is decided once here and never
// re-examined per item.
//
// A prompt carrying a NUL and a prompt past PromptLimit fail the item before a
// process starts: an argv cannot carry the first, and an element that grows
// without bound is how one item takes the run's memory. An argv naming the
// token in two elements is refused for the command's reason — the prompt would
// be sent twice — and an unknown brace token is passed through as written. Both
// copies inside one element are refused earlier still, by New.
func (c *Command) Item(prompt string) (Command, error) {
	if strings.IndexByte(prompt, 0) >= 0 {
		return Command{}, errors.New("the prompt carries a NUL byte, which an argv cannot")
	}
	if len(prompt) > PromptLimit {
		return Command{}, fmt.Errorf("the prompt is %d bytes, over the %d-byte bound", len(prompt), PromptLimit)
	}
	item := *c
	item.argv = slices.Clone(c.argv)
	var named []int
	for i, element := range item.argv {
		if strings.Contains(element, tokenPrompt) {
			named = append(named, i)
		}
	}
	switch len(named) {
	case 0:
		item.prompt = prompt
		return item, nil
	case 1:
		item.argv[named[0]] = strings.ReplaceAll(item.argv[named[0]], tokenPrompt, prompt)
		return item, nil
	}
	return Command{}, fmt.Errorf("the command names %s in %d elements: the prompt reaches a command once", tokenPrompt, len(named))
}

// Ask runs the item's command with dir as its working directory and WithEnv's
// environment, and returns its reply: stdout read to EOF, trailing whitespace
// trimmed. Stdin is the prompt, or empty where the argv carried it.
//
// A non-zero exit, a deadline, a cancellation or a bound passed is the call's
// failure, and its reason carries the exit code and the captured stderr — the
// only place a command's stderr is ever written.
//
// The child runs in its own process group, and that group is what the deadline
// and the cancellation kill. On a unix host the group is the guarantee: what the
// command started inside it ends with it, and a child that leaves the group
// (setsid) is a process of its own that outlives the kill. On Windows the group
// scopes a console signal and nothing more, so only the direct process is
// killed and the command's own children are the command's to end.
func (c Command) Ask(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, c.program, c.argv[1:]...)
	cmd.Dir = dir
	cmd.Env = c.env
	cmd.Stdin = strings.NewReader(c.prompt)
	reply, noise := &capture{limit: ReplyLimit}, &capture{limit: StderrLimit}
	cmd.Stdout, cmd.Stderr = reply, noise
	cmd.SysProcAttr = processGroup()
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = waitDelay
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%s could not start: %w", c.argv[0], err)
	}
	err := cmd.Wait()
	if ctx.Err() != nil {
		c.lg.Warn("harness: the command's process group was killed", "command", c.argv[0], "dir", dir, "reason", ctx.Err())
	}
	switch {
	case reply.over:
		return "", fmt.Errorf("the reply is over the %d-byte bound", ReplyLimit)
	case noise.over:
		return "", fmt.Errorf("the command wrote over the %d-byte stderr bound", StderrLimit)
	case err != nil:
		if tail := strings.TrimSpace(noise.String()); tail != "" {
			return "", fmt.Errorf("the command failed (%v): %s", err, tail)
		}
		return "", fmt.Errorf("the command failed (%v)", err)
	}
	return strings.TrimRight(reply.String(), " \t\r\n"), nil
}

// capture is a bounded sink: it holds the first limit bytes written to it and
// notes that more arrived, without failing the write. Failing it would close
// the child's pipe mid-write, turning a bound kbase enforces into a signal the
// command never asked for; draining what is past the bound costs nothing and
// lets the item fail with the bound's own reason.
type capture struct {
	buf   []byte
	limit int
	over  bool
}

func (c *capture) Write(p []byte) (int, error) {
	if room := c.limit - len(c.buf); room < len(p) {
		if room > 0 {
			c.buf = append(c.buf, p[:room]...)
		}
		c.over = true
	} else {
		c.buf = append(c.buf, p...)
	}
	return len(p), nil
}

// String is what the sink holds: the bound's worth of what the command wrote.
func (c *capture) String() string { return string(c.buf) }
