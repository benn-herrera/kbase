package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"kbase/internal/build"
	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	toolresult "kbase/internal/result"
)

type buildOptions struct {
	VolumeRoot     string
	Bibliographies []string
	Charter        string
	Through        string
	StateDir       string
	NoInference    bool
	// ConfigDir is the --config-dir flag, "" where it was not given.
	ConfigDir string
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	// Provider is where the build's model calls go, or what the configuration
	// lacks for them.
	Provider func() (build.Provider, error)
	Stdout   io.Writer
	Stderr   io.Writer
	Logger   log.Logger
}

// runBuild runs the build, writes its result document, and returns the exit
// code the outcome maps to.
func runBuild(ctx context.Context, opts buildOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "build"); err != nil {
		return 0, err
	}
	outcome, fields := build.Run(ctx, build.Options{
		VolumeRoot: opts.VolumeRoot, Bibliographies: opts.Bibliographies, Charter: opts.Charter, Through: opts.Through,
		StateDir: opts.StateDir, NoInference: opts.NoInference, ConfigDir: opts.ConfigDir, WorkDir: opts.WorkDir, Provider: opts.Provider,
		Logger: opts.Logger,
	})
	if err := toolresult.Emit(opts.Stdout, outcome, fields...); err != nil {
		return 0, fmt.Errorf("build: writing the result: %w", err)
	}
	return exitCodes[outcome], nil
}

// refused is a refusal of every item, after fields.
func refused(fields []toolresult.Field, items ...toolresult.Item) (string, []toolresult.Field) {
	return toolresult.Refused, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: items})
}

// checkError names a failure kbase has no narrower class for.
const checkError = "error"

// failed is a failure of err, after fields.
func failed(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	return toolresult.Failed, append(fields, toolresult.Field{Key: toolresult.FailuresKey,
		Value: []toolresult.Item{{Check: checkError, Detail: err.Error()}}})
}

// exitCode carries a non-zero exit out of a verb that has already written
// its result document.
type exitCode int

func (c exitCode) Error() string { return fmt.Sprintf("exit %d", int(c)) }

var (
	flagBibliographies []string
	flagCharter        string
	flagThrough        string
	flagStateDir       string
	flagNoInference    bool
)

// buildProvider is the provider the loaded configuration selects, its letter
// asks on the light tier and the overview passage on the heavy, or what the
// configuration lacks.
func buildProvider(p providerOptions) func() (build.Provider, error) {
	return func() (build.Provider, error) {
		name, err := selectProviderName("", p.Config.Provider, p.Providers)
		if err != nil {
			return build.Provider{}, err
		}
		entry, ok := p.Providers[name]
		if !ok {
			return build.Provider{}, unresolvedProviderError(name, p.Providers, p.Faults)
		}
		letters, okLight := p.Config.ModelFor(config.TierLight)
		overview, okHeavy := p.Config.ModelFor(config.TierHeavy)
		var unset []string
		if !okLight {
			unset = append(unset, "models."+config.TierLight)
		}
		if !okHeavy {
			unset = append(unset, "models."+config.TierHeavy)
		}
		if len(unset) > 0 {
			return build.Provider{}, fmt.Errorf("config.toml sets no %s", strings.Join(unset, " or "))
		}
		client := model.NewHTTPClient(model.Endpoint{Name: name, BaseURL: entry.BaseURL, APIKey: entry.APIKey})
		concurrency := 0
		if n := p.Config.Asks.ReaderConcurrency; n != nil {
			concurrency = *n
		}
		return build.Provider{Client: client, Letters: letters, Overview: overview, ReaderConcurrency: concurrency}, nil
	}
}

var buildCmd = &cobra.Command{
	Use:   "build <volume-root>",
	Short: "build kb_tools' KB from a LaTeX volume root into <git root>/kb-root",
	Long: `build converts one LaTeX volume root into the KB kb_tools builds, at kb-root/
beside the repository's .git, walking the build's stages in order through the
stage --through names (an id or a display name; every stage when absent).
Each stage's boundary is a commit in the repository, its subject
"kb-build: <stage> | <display>", scoped to the paths the build owns; an
invocation resumes from the last boundary the commit trail holds.
--no-inference drops every row that spends inference and walks the rest;
without it, the claim graph's letter asks go to the configured provider's
light model and the overview passage to its heavy one. With no --bibliography,
every .bib beside the volume root is offered, in sorted order.
Build state — the run lock, progress.jsonl, the document graph's records, each
stage's report under reports/, each call's capture and the answer cache under
scratch/ — lives in --state-dir, by default $XDG_STATE_HOME/kbase/<key>.
SIGINT or SIGTERM (kbase cancel) stops it resumably. Its result is one YAML
document on stdout, naming each stage walked, its boundary commit and its
report, and the command that resumes the build where it did not finish.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		_, providers, err := loadVerbContext()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		code, err := runBuild(ctx, buildOptions{
			VolumeRoot:     args[0],
			Bibliographies: flagBibliographies,
			Charter:        flagCharter,
			Through:        flagThrough,
			StateDir:       flagStateDir,
			NoInference:    flagNoInference,
			ConfigDir:      flagConfigDir,
			WorkDir:        wd,
			Provider:       buildProvider(providers),
			Stdout:         cmd.OutOrStdout(),
			Stderr:         cmd.ErrOrStderr(),
			Logger:         processLog.logger,
		})
		if err != nil {
			return err
		}
		if code != 0 {
			return exitCode(code)
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringArrayVar(&flagBibliographies, "bibliography", nil,
		"a .bib citations resolve against; repeat per file, in the order that decides a key two files define (default: every .bib beside the volume root, sorted)")
	buildCmd.Flags().StringVar(&flagCharter, "charter", "", "a file stating the build's scope, kept as kb-build-charter.md when the build opens")
	buildCmd.Flags().StringVar(&flagThrough, "through", "", "the last stage to walk, by id or display name (default: every stage)")
	buildCmd.Flags().StringVar(&flagStateDir, "state-dir", "", "the build's state store (default: $XDG_STATE_HOME/kbase/<key>)")
	buildCmd.Flags().BoolVar(&flagNoInference, "no-inference", false, "drop every row that spends inference and walk the rest")
	rootCmd.AddCommand(buildCmd)
}
