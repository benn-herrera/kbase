//go:build kbasedev

package main

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/asks"
	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
)

// buildReplayOptions is the build's replay input: the build verb's options it
// shares, its positionals, the narrowing flags, and which of build's own
// flags the invocation set.
type buildReplayOptions struct {
	buildOptions

	// Args is the positionals: at most one, the start commit's hex prefix.
	Args []string

	Replay bool
	Kind   string
	Failed bool
	Limit  int

	// Given is build's own flags the invocation set, by name.
	Given []string
}

// buildReplayFlags is build's own flags a replay takes beside --replay; any
// other of them is a build's and is refused with it.
var buildReplayFlags = []string{"replay", "kind", "failed", "limit", "state-dir", "ask-concurrency"}

// buildReplayNarrowing is the flags that narrow a replay, refused without
// --replay.
var buildReplayNarrowing = []string{"kind", "failed", "limit"}

// startCommitRE is the shape of the start commit a replay names: a hex
// prefix, never a path.
var startCommitRE = regexp.MustCompile(`^[0-9a-f]+$`)

var (
	buildFlagReplay bool
	buildFlagKind   string
	buildFlagFailed bool
	buildFlagLimit  int
)

func init() {
	buildCmd.Flags().BoolVar(&buildFlagReplay, "replay", false,
		"re-ask a build trail's recorded asks under this binary's templates and compare; the positional names the trail's start commit (default: the current trail)")
	buildCmd.Flags().StringVar(&buildFlagKind, "kind", "", "with --replay, re-ask only this kind's asks")
	buildCmd.Flags().BoolVar(&buildFlagFailed, "failed", false, "with --replay, re-ask only the asks the trail recorded failed, defaulted or rejected")
	buildCmd.Flags().IntVar(&buildFlagLimit, "limit", 0, "with --replay, re-ask only the first N asks")
	for _, name := range append([]string{"replay"}, buildReplayNarrowing...) {
		mcp.ExcludeFlag(buildCmd.Flags().Lookup(name))
	}
	buildReplay = func(cmd *cobra.Command, args []string) func(context.Context, buildOptions) (int, error) {
		given := changedLocalFlags(cmd)
		if !buildFlagReplay && !slices.ContainsFunc(given, func(name string) bool { return slices.Contains(buildReplayNarrowing, name) }) {
			return nil
		}
		return func(ctx context.Context, opts buildOptions) (int, error) {
			return runBuildReplay(ctx, buildReplayOptions{buildOptions: opts, Args: args, Replay: buildFlagReplay,
				Kind: buildFlagKind, Failed: buildFlagFailed, Limit: buildFlagLimit, Given: given})
		}
	}
}

// runBuildReplay runs one replay, writes its result document, and returns the
// exit code the outcome maps to.
func runBuildReplay(ctx context.Context, opts buildReplayOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "build"); err != nil {
		return 0, err
	}
	outcome, fields := buildReplayRun(ctx, opts)
	return emitResult("build", opts.Stdout, outcome, fields)
}

// replayKinds is what --kind takes: each multichoice kind, then the overview.
func replayKinds() []string {
	var kinds []string
	for _, k := range asks.Kinds {
		kinds = append(kinds, string(k))
	}
	return append(kinds, build.ReplayOverview)
}

// buildReplayRun is the replay's body: the trail's records selected, the
// roles resolved from configuration as a build's are, and the walk. It needs
// no source tree, takes no lock, and writes only under the replay's own
// directory.
func buildReplayRun(ctx context.Context, opts buildReplayOptions) (outcome string, fields []toolresult.Field) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	var narrowing, others []string
	for _, name := range opts.Given {
		if slices.Contains(buildReplayNarrowing, name) {
			narrowing = append(narrowing, name)
		}
		if !slices.Contains(buildReplayFlags, name) {
			others = append(others, "--"+name)
		}
	}
	switch {
	case !opts.Replay:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--" + narrowing[0],
			Detail: "--" + narrowing[0] + " narrows a replay and is given with " + buildstore.ReplayKey})
	case len(others) > 0:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: buildstore.ReplayKey,
			Detail: buildstore.ReplayKey + " takes " + replayTakes(buildReplayFlags) + "; " + strings.Join(others, ", ") + " is a build's"})
	case len(opts.Args) > 1 || len(opts.Args) == 1 && !startCommitRE.MatchString(opts.Args[0]):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: buildstore.ReplayKey,
			Detail: buildstore.ReplayKey + " takes one positional, a hex prefix of a trail's start commit, not a path: " + strings.Join(opts.Args, " ")})
	case opts.Kind != "" && !slices.Contains(replayKinds(), opts.Kind):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--kind",
			Detail: "--kind is one of " + strings.Join(replayKinds(), ", ") + ", not " + opts.Kind})
	case opts.Limit < 0:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--limit", Detail: "--limit counts asks and cannot be negative"})
	}
	reg, provider, err := buildModels(opts.buildOptions)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	defer reg.CloseAll()
	dir, err := replayStateDir(opts.StateDir, opts.WorkDir)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	fields = []toolresult.Field{{Key: "state-dir", Value: dir}}

	start := ""
	if len(opts.Args) == 1 {
		start = opts.Args[0]
	}
	rp, err := build.OpenReplay(dir, start, opts.Kind, opts.Failed, opts.Limit)
	if err != nil {
		return refusedOrFailed(fields, err)
	}
	fields = append(fields, toolresult.Field{Key: "trail", Value: rp.Trail()},
		toolresult.Field{Key: "replay", Value: rp.ID()}, toolresult.Field{Key: "dir", Value: rp.Dir()})
	p, err := provider()
	if err != nil {
		return refusedOrFailed(fields, err)
	}
	res, err := rp.Walk(ctx, p, opts.Logger)
	if err != nil {
		return failed(fields, err)
	}
	fields = append(fields,
		toolresult.Field{Key: "counts", Value: toolresult.Record{
			{Key: "total", Value: res.Counts.Total},
			{Key: "done", Value: res.Counts.Done},
			{Key: "failed", Value: res.Counts.Failed}}},
		toolresult.Field{Key: "changed", Value: res.Changed})
	if res.Outcome == toolresult.Cancelled {
		fields = append(fields, toolresult.Field{Key: toolresult.CancelledKey, Value: toolresult.Record{
			{Key: "kind", Value: res.Cancelled.Kind}, {Key: "group", Value: res.Cancelled.Group}, {Key: "item", Value: res.Cancelled.Item}}})
	}
	return res.Outcome, fields
}
