// Command kbase builds a knowledge base from LaTeX and maintains it.
//
// This file is the composition root: it declares the root command, resolves
// the configuration directory every verb draws on, builds the one logger the
// process has, and owns process exit.
// Verb logic lives beside its command in its own file, behind a plain
// function the cobra RunE is a thin shell over — so a verb is testable
// without a process, a terminal, or a network.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/version"
)

// flagConfigDir backs the persistent --config-dir flag. See
// resolveConfigDir for the precedence it participates in.
var flagConfigDir string

// flagLogLevel and flagLogFile back the persistent logging flags.
var (
	flagLogLevel string
	flagLogFile  string
)

// processLog holds the logger between the point cobra has parsed the
// persistent flags and the point main tears it down.
//
// It is the one piece of composition-root state that cannot be a local:
// cobra parses flags inside Execute, so the logger cannot exist before the
// call that would otherwise receive it. Nothing outside this file reads it —
// a component that logs takes a log.Logger parameter, and the RunE shell
// hands it this one. It starts as a discarding logger so the field is never
// nil, whatever order cobra decides to run things in.
var processLog = struct {
	logger log.Logger
	closer io.Closer
}{logger: log.Discard()}

var rootCmd = &cobra.Command{
	Use:   "kbase",
	Short: "build and maintain a knowledge base from LaTeX",
	Long: `kbase builds a knowledge base (KB) from LaTeX and maintains it. Volume roots
go in — the top .tex file of each paper — and a KB comes out at kb-root/
beside the repository's .git: the Markdown document tree, its metadata layer
and .index/, as kb_tools builds it. The maintenance subcommands then operate
on the living KB: refresh and verify, the write API's ops, render-citation,
the queries and the claim-graph sheet. build runs in the foreground; status
and cancel observe and stop it. models and configure choose the provider and
the models a build's inference goes to.

Every subcommand writes one YAML document to stdout, its outcome first, and
nothing else; stderr is for humans. The exit code follows the outcome: 0 for
done, unchanged or bounded; 1 refused; 2 retry; 3 failed; 4 cancelled.`,
	// No subcommand a bare invocation could reasonably mean, so it prints
	// help rather than guessing.
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },

	// Every verb runs through here first, so the logger is built exactly
	// once per invocation, after the flags that configure it are parsed
	// and before any verb could want it. A bad --log-level or an
	// unopenable --log-file fails the invocation here rather than
	// downgrading itself silently.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		logger, closer, err := log.New(log.Options{
			Level:    log.Level(flagLogLevel),
			Console:  cmd.ErrOrStderr(),
			FilePath: flagLogFile,
		})
		if err != nil {
			return err
		}
		processLog.logger, processLog.closer = logger, closer
		return nil
	},

	// A failing verb prints its error once, from main, and exits 1: usage
	// text is noise on a runtime failure, and cobra's own "Error: ..." line
	// would render the message a second time.
	SilenceUsage:  true,
	SilenceErrors: true,
}

// resolveConfigDir returns the configuration directory this invocation
// reads: the --config-dir flag when given, otherwise config.Dir's
// resolution ($KBASE_CONFIG_DIR, else ~/.config/kbase). The flag is the
// outermost override — it beats the environment as well as the default,
// because an explicit path on the command line is the least ambiguous
// statement of intent available.
//
// It is the single place that precedence is expressed; verbs call it and
// hand the result to the config loaders, which take explicit paths.
func resolveConfigDir() (string, error) {
	if dir := strings.TrimSpace(flagConfigDir); dir != "" {
		return dir, nil
	}
	return config.Dir()
}

// requireStreams checks the writers a verb was handed before it does any
// work. Both are required rather than defaulted: a verb whose stream is nil
// was constructed wrong, and quietly substituting os.Stdout would send a
// test's captured output to the terminal instead of failing.
//
// verb prefixes the message because this names a programming mistake at a
// call site, and which call site is the first thing worth knowing.
func requireStreams(stdout, stderr io.Writer, verb string) error {
	if stdout == nil || stderr == nil {
		return fmt.Errorf("%s: Stdout and Stderr are required", verb)
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfigDir, "config-dir", "",
		"configuration directory (default: $"+config.EnvConfigDir+", else ~/.config/kbase)")

	// Diagnostics are off by default in all but name: a verb's own summary
	// and warnings are the CLI's output, and the log is the channel you
	// turn up when that is not enough. --log-file tees, it does not
	// redirect — a run whose console you were watching stays watchable.
	rootCmd.PersistentFlags().StringVar(&flagLogLevel, "log-level", string(log.DefaultLevel),
		"log level: debug, info, warn, or error")
	rootCmd.PersistentFlags().StringVar(&flagLogFile, "log-file", "",
		"also append diagnostics to this file (default: console only)")

	// `kbase --version` prints exactly version.Current. Cobra's default
	// template wraps it in its own "kbase version X" line, which would
	// render the version string twice over.
	rootCmd.Version = version.Current
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func main() {
	err := rootCmd.Execute()

	// The log file outlives Execute by exactly this much: closing it is
	// the last thing the process does, and a failure to flush it is worth
	// reporting only when nothing worse already went wrong.
	if c := processLog.closer; c != nil {
		if cerr := c.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing log file: %w", cerr)
		}
	}

	os.Exit(exitStatus(err, os.Stdout, os.Stderr))
}

// exitCodes maps a result's outcome to the process exit code.
var exitCodes = map[string]int{
	toolresult.Done: 0, toolresult.Unchanged: 0, toolresult.Bounded: 0,
	toolresult.Refused: 1, toolresult.Retry: 2, toolresult.Failed: 3, toolresult.Cancelled: 4,
}

// exitStatus is the process exit code for what Execute returned. An error no
// verb wrote a result document for — a usage error among them — is refused
// with its own document.
func exitStatus(err error, stdout, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	var code exitCode
	if errors.As(err, &code) {
		return int(code)
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
	outcome, fields := refused(nil, toolresult.Item{Check: checkUsage, Detail: err.Error()})
	if eerr := toolresult.Emit(stdout, outcome, fields...); eerr != nil {
		fmt.Fprintf(stderr, "error: writing the result: %v\n", eerr)
	}
	return exitCodes[outcome]
}
