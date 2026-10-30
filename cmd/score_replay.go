//go:build kbasedev

package main

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
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

// scoreReplayFlags is score's own flags a replay takes beside --replay; any
// other of them is a run's and is refused with it.
var scoreReplayFlags = []string{"replay", "pass", "failed", "limit", "scoring-model", "state-dir", "timeout", "ask-concurrency"}

// replayRunRE is the shape of the run --replay names: a real or a dry run's id
// as kb.MintID mints it, never a path.
var replayRunRE = regexp.MustCompile(`^(` + score.RunKindReal + `|` + score.RunKindDry + `)-` + runIDBody + `$`)

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
		given := changedLocalFlags(cmd)
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
		if !slices.Contains(scoreReplayFlags, name) {
			others = append(others, "--"+name)
		}
	}
	switch {
	case !slices.Contains(opts.Given, "replay"):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--failed",
			Detail: "--failed narrows a replay and is given with --replay"})
	case !replayRunRE.MatchString(opts.Run):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: buildstore.ReplayKey,
			Detail: "--replay names a run's id (" + score.RunKindReal + "-<…> or " + score.RunKindDry + "-<…>), not a path: " + opts.Run})
	case len(others) > 0:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: buildstore.ReplayKey,
			Detail: "--replay takes " + replayTakes(scoreReplayFlags) + "; " + strings.Join(others, ", ") + " is a run's"})
	case opts.Limit < 0:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--limit", Detail: "--limit counts items and cannot be negative"})
	}
	concurrency, err := askConcurrency(opts.AskConcurrency, opts.Config.Asks.Concurrency)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	dir, err := replayStateDir(opts.StateDir, opts.WorkDir)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	fields = []toolresult.Field{{Key: "state-dir", Value: dir}, {Key: "run", Value: opts.Run}}

	rp, err := score.OpenReplay(dir, opts.Run, opts.Pass, opts.Failed, opts.Limit)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	ep, err := scoringRole(opts.scoreOptions)
	if err != nil {
		return refusedOrFailed(fields, err)
	}
	reg := modelclient.New(opts.NewClient, opts.Logger, rp.ID(), concurrency)
	defer reg.CloseAll()
	launch := scoringLaunch(opts.scoreOptions, reg, ep, concurrency)
	fields = append(fields, toolresult.Field{Key: "replay", Value: rp.ID()}, toolresult.Field{Key: "dir", Value: rp.Dir()})
	res, err := rp.Walk(ctx, launch, score.ReplayHeader{BackEnd: string(config.BackEndProvider), Provider: ep.Provider, Model: ep.Model,
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
