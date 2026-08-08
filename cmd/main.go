// Command kbase converts a human-targeted documentation corpus into an
// agent-friendly knowledge base.
//
// This file is the composition root: it declares the root command, resolves
// the configuration directory every verb draws on, and owns process exit.
// Verb logic lives beside its command in its own file, behind a plain
// function the cobra RunE is a thin shell over — so a verb is testable
// without a process, a terminal, or a network.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/version"
)

// flagConfigDir backs the persistent --config-dir flag. See
// resolveConfigDir for the precedence it participates in.
var flagConfigDir string

var rootCmd = &cobra.Command{
	Use:   "kbase",
	Short: "turn a documentation corpus into an agent-friendly knowledge base",
	Long: `kbase is a standalone batch appliance. Point it at a documentation corpus
and it produces a navigable Markdown knowledge base — entry point, domain
index, subtopic index, leaf — with verbatim leaves, progressive routing
summaries, and mechanically generated navigation links. It is not
interactive and has no default action: each verb runs a fixed pipeline to
completion and exits.`,
	// No REPL, and no pipeline that a bare invocation could reasonably
	// mean — so a bare invocation prints help rather than guessing.
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },

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

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfigDir, "config-dir", "",
		"configuration directory (default: $"+config.EnvConfigDir+", else ~/.config/kbase)")

	// `kbase --version` prints exactly version.Short(). Cobra's default
	// template wraps it in its own "kbase version X" line, which would
	// render the version string twice over.
	rootCmd.Version = version.Short()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
