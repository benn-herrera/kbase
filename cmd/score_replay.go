//go:build !kbasedist

package main

import (
	"context"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
	"kbase/internal/statedir"
)

// scoreReplayOptions is the replay's input: the score verb's options it shares,
// the run it re-asks, whether only its failed items, and which of score's own
// flags the invocation set.
type scoreReplayOptions struct {
	scoreOptions

	// Run is the --replay flag: the run whose captures are re-asked.
	Run string

	// Failed narrows the selection to the items the run's ledger last records
	// failed.
	Failed bool

	// Given is score's own flags the invocation set, by name.
	Given []string
}

// replayFlags is score's own flags a replay takes beside --replay; any other of
// them is a run's and is refused with it.
var replayFlags = []string{"replay", "pass", "failed", "limit", "scoring-model", "state-dir", "timeout"}

// replayRunRE is the shape of the run --replay names: a real or a dry run's id
// as kb.MintID mints it, never a path.
var replayRunRE = regexp.MustCompile(`^(` + score.RunKindReal + `|` + score.RunKindDry + `)-` + runIDBody + `$`)

// replayTakes is the flags a replay takes beside --replay as its refusal names
// them: replayFlags, and the persistent --config-dir.
func replayTakes() string {
	var names []string
	for _, name := range replayFlags {
		if name != "replay" {
			names = append(names, "--"+name)
		}
	}
	names = append(names, "--config-dir")
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

var (
	scoreFlagReplay string
	scoreFlagFailed bool
)

func init() {
	scoreCmd.Flags().StringVar(&scoreFlagReplay, "replay", "",
		"re-ask this run's captured calls under this binary's templates and compare; lands nothing")
	scoreCmd.Flags().BoolVar(&scoreFlagFailed, "failed", false, "with --replay, re-ask only the items the run last recorded failed")
	mcp.ExcludeFlag(scoreCmd.Flags().Lookup("replay"))
	mcp.ExcludeFlag(scoreCmd.Flags().Lookup("failed"))
	scoreReplay = func(cmd *cobra.Command) func(scoreOptions) (int, error) {
		var given []string
		cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if f.Changed {
				given = append(given, f.Name)
			}
		})
		if !slices.Contains(given, "replay") && !slices.Contains(given, "failed") {
			return nil
		}
		return func(opts scoreOptions) (int, error) {
			return runScoreReplay(scoreReplayOptions{scoreOptions: opts, Run: scoreFlagReplay, Failed: scoreFlagFailed, Given: given})
		}
	}
}

// runScoreReplay runs one replay, writes its result document, and returns the
// exit code the outcome maps to.
func runScoreReplay(opts scoreReplayOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "score"); err != nil {
		return 0, err
	}
	outcome, fields := scoreReplayRun(opts)
	return emitResult("score", opts.Stdout, outcome, fields)
}

// scoreReplayRun is the replay's body: the run's captures selected, the scoring
// role resolved and its client opened as a run's are, and the walk. It opens no
// KB and takes no lock, and writes only under the replay's own directory.
func scoreReplayRun(opts scoreReplayOptions) (outcome string, fields []toolresult.Field) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	var others []string
	for _, name := range opts.Given {
		if !slices.Contains(replayFlags, name) {
			others = append(others, "--"+name)
		}
	}
	switch {
	case !slices.Contains(opts.Given, "replay"):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--failed",
			Detail: "--failed narrows a replay and is given with --replay"})
	case !replayRunRE.MatchString(opts.Run):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--replay",
			Detail: "--replay names a run's id (" + score.RunKindReal + "-<…> or " + score.RunKindDry + "-<…>), not a path: " + opts.Run})
	case len(others) > 0:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--replay",
			Detail: "--replay takes " + replayTakes() + "; " + strings.Join(others, ", ") + " is a run's"})
	case opts.Limit < 0:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--limit", Detail: "--limit counts items and cannot be negative"})
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	fields = []toolresult.Field{{Key: "kb-root", Value: root}}
	dir, err := statedir.Dir(opts.StateDir, root)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	fields = append(fields, toolresult.Field{Key: "state-dir", Value: dir}, toolresult.Field{Key: "run", Value: opts.Run})

	rp, err := score.OpenReplay(dir, opts.Run, opts.Pass, opts.Failed, opts.Limit)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	ep, _, err := scoringRole(opts.scoreOptions, config.EnvOverrides(os.Getenv))
	if err != nil {
		return refusedOrFailed(fields, err)
	}
	reg := modelclient.New(opts.NewClient, opts.Logger, rp.ID())
	defer reg.CloseAll()
	ask, err := scoringAsk(opts.scoreOptions, reg, ep)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	fields = append(fields, toolresult.Field{Key: "replay", Value: rp.ID()}, toolresult.Field{Key: "dir", Value: rp.Dir()})
	res, err := rp.Walk(ctx, ask, score.ReplayHeader{BackEnd: backEndName(ep), Provider: ep.Provider, Model: ep.Model,
		Started: score.Stamp()})
	if err != nil {
		return failed(fields, err)
	}
	fields = append(fields,
		toolresult.Field{Key: "counts", Value: toolresult.Record{
			{Key: "total", Value: res.Counts.Total},
			{Key: "done", Value: res.Counts.Done},
			{Key: "failed", Value: res.Counts.Failed},
			{Key: "skipped", Value: res.Counts.Skipped}}},
		toolresult.Field{Key: "changed", Value: res.Changed})
	if res.Outcome == toolresult.Cancelled {
		fields = append(fields, toolresult.Field{Key: toolresult.CancelledKey,
			Value: toolresult.Record{{Key: "item", Value: res.Stopped}}})
	}
	return res.Outcome, fields
}
