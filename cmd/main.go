// Command kbase builds a knowledge base from LaTeX and maintains it.
//
// This file is the composition root: it declares the root command and the
// global flags every verb draws on, builds the one logger the process has, and
// owns process exit.
// Verb logic lives beside its command in its own file, behind a plain
// function the cobra RunE is a thin shell over — so a verb is testable
// without a process, a terminal, or a network.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/version"
)

// flagConfigDir backs the persistent --config-dir flag. See config.Layers for
// the precedence it participates in.
var flagConfigDir string

// flagProjectDir backs the persistent --project-dir flag. invocationDir
// validates it as a worktree root; build and score carry it, absolute, into
// the command that continues the run.
var flagProjectDir string

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
	Long:  rootLong,

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
		// cobra prints a command's summary and flag usages itself — shell
		// completion's descriptions among them — so the root's stdout spells
		// the text it is handed, unless the call came in over MCP and settled
		// its spelling already. The server's stdout is its protocol frames.
		if cmd.Name() != mcpCommandName {
			root := cmd.Root()
			root.SetOut(cliOut(root, root.OutOrStdout()))
		}
		return nil
	},

	// A failing verb prints its error once, from main, and exits 1: usage
	// text is noise on a runtime failure, and cobra's own "Error: ..." line
	// would render the message a second time.
	SilenceUsage:  true,
	SilenceErrors: true,
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

// rootLong is the root command's shared help body.
var rootLong = `kbase builds a knowledge base (KB) from LaTeX and maintains it. build takes
volume roots — the top .tex file of each paper — and makes a KB at kb-root/
beside the repository's .git: the Markdown document tree and its records,
under .index/; init creates one empty instead. The write ops change the KB,
each an entry of a write batch, and the queries read it. verify checks it,
update-format converts it to the current format and re-derives it, and
export-latex writes it out as LaTeX. score asks the scoring model, one item
per ask, for each rigor and bearing a model judges that is not manually scored
and not yet judged against its current text. build and score run in the foreground;
status and cancel follow and stop either kind of run, and build-trail lists
the build's stage boundaries. show-config reports how the
provider and model configuration resolves; models, configure-providers and
configure-models choose it, and probe asks one model one question.

Three commands are CLI-only, served by no MCP tool: auth stores, lists and
removes a provider's API key, which never travels through MCP; wait blocks
until a run ends, which an MCP call cannot; and mcp is the server itself.
help and completion are the CLI framework's own. Every other command is an
MCP tool or an op of the write or query tool.

` + mcp.EdgeDirection + `

Every subcommand writes its result, one YAML document with its outcome first,
to stdout and nothing else; stderr is for humans. The exit code follows the
outcome: 0 for done, unchanged or bounded; 1 refused; 2 retry; 3 failed; 4
cancelled.

` + queryHelp + `

` + listQueryHelp + `

` + valuesHelp

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfigDir, "config-dir", "",
		"the only configuration directory read (default: $"+config.EnvConfigDir+", else <repository>/"+config.RepositoryDirName+" over ~/.config/kbase)")

	rootCmd.PersistentFlags().StringVar(&flagProjectDir, "project-dir", "",
		"the project root: the git worktree root that holds kb-root/ or will (default: found from the working directory)")

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
	// cobra adds --version at Execute; adding it here puts it in the root's
	// argument list wherever that list is rendered.
	rootCmd.InitDefaultVersionFlag()

	// Every command's --help, cobra's help command included, prints
	// mcp.CLIHelp in place of cobra's template.
	rootCmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		help, err := mcp.CLIHelp(c)
		if err == nil {
			_, err = fmt.Fprint(c.OutOrStdout(), help)
		}
		if err != nil {
			c.PrintErrln(err)
		}
	})
}

func main() {
	stdout := io.Writer(os.Stdout)
	if os.Getenv(mcp.SurfaceEnv) == mcp.SurfaceMCP {
		stdout = toolresult.Spelled(stdout, mcp.Spelling(rootCmd, true))
	}
	rootCmd.SetOut(stdout)
	err := rootCmd.Execute()

	// The log file outlives Execute by exactly this much: closing it is
	// the last thing the process does, and a failure to flush it is worth
	// reporting only when nothing worse already went wrong.
	if c := processLog.closer; c != nil {
		if cerr := c.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing log file: %w", cerr)
		}
	}

	os.Exit(exitStatus(err, stdout, os.Stderr))
}

// resultOut is w as a verb's result reaches its reader: spelled as the
// command line takes an argument, unless the call w answers came in over MCP,
// which settled its spelling where it came in.
func resultOut(w io.Writer) io.Writer { return cliOut(rootCmd, w) }

// cliOut is w spelling for the command line, from the tree under root, unless
// it carries a spelling already.
func cliOut(root *cobra.Command, w io.Writer) io.Writer {
	return toolresult.Spelled(w, mcp.Spelling(root, false))
}

// exitStatus is the process exit code for what Execute returned. An error no
// verb wrote a result for — a usage error among them — is refused with its
// own result.
func exitStatus(err error, stdout, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	var code exitCode
	if errors.As(err, &code) {
		return int(code)
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
	outcome, fields := refused(nil, errorItems(err)...)
	if eerr := toolresult.Emit(resultOut(stdout), outcome, fields...); eerr != nil {
		fmt.Fprintf(stderr, "error: writing the result: %v\n", eerr)
	}
	return toolresult.ExitCode(outcome)
}

// errorItems is the items of err, an error no verb wrote a result for: a
// refusal's own, else one usage item carrying its text.
func errorItems(err error) []toolresult.Item {
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		return refusal
	}
	return []toolresult.Item{{Check: toolresult.CheckUsage, Detail: err.Error()}}
}
