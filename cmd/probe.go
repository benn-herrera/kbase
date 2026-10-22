package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	toolresult "kbase/internal/result"
)

// probePrompt is what a probe asks where --prompt names nothing: a one-word
// check, so the heartbeat costs a handful of tokens and its reply is short.
const probePrompt = "Reply with the single word: ok"

// probeOptions is the probe verb's input: the shared provider-reaching options,
// the reference and the prompt the flags name, and the stream its result
// document goes to.
type probeOptions struct {
	providerOptions

	// Model is the --model flag: a <provider>/<model> reference to probe
	// instead of the resolved scoring role, "" where the flag names none.
	Model string

	// Prompt is the --prompt flag: the check to send, "" taking probePrompt.
	Prompt string

	Stdout io.Writer
	Logger log.Logger
}

// runProbe asks one model one question and writes the result document, and
// returns the exit code the outcome maps to. It is a diagnostic of a model
// selection: one call, no turn, no tool, no history, no KB and no state.
func runProbe(ctx context.Context, opts probeOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "probe"); err != nil {
		return 0, err
	}
	outcome, fields := probeCall(ctx, opts)
	return emitResult("probe", opts.Stdout, outcome, fields)
}

// probeCall is the verb's body: the selection, the client the registry opens
// for it, and what the one call answered.
func probeCall(ctx context.Context, opts probeOptions) (string, []toolresult.Field) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	ep, err := probeEndpoint(opts)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	fields := append(providerFields(ep.Provider, ep.Rule, ep.Layer),
		toolresult.Field{Key: "model", Value: stringOrNull(ep.Model)})
	// The registry is scoring's client path, so the probe speaks whatever wire
	// shape the endpoint's entry names.
	reg := newClients(opts.NewClient, opts.Logger, "")
	defer reg.closeAll()
	client, err := reg.get(ep)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	if key, err := reg.key(ep); err == nil {
		if line := config.CleartextWarning(ep.BaseURL, key); line != "" {
			fmt.Fprintln(opts.Stderr, line)
		}
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	prompt := opts.Prompt
	if prompt == "" {
		prompt = probePrompt
	}
	// No backoff: a heartbeat re-issues at once, so the deadline is spent on
	// attempts rather than on pauses.
	started := time.Now()
	attempt, err := model.Chat(callCtx, client, ep.Model, "", prompt, nil, nil)
	if err != nil {
		return failed(fields, err)
	}
	return toolresult.Done, append(fields, probeAnswer(attempt, time.Since(started))...)
}

// probeEndpoint is the selection the probe asks: --model's reference where the
// flag names one, and otherwise the scoring role — the selection a run would
// make, in either mode. An explicit reference is a pool reference, so it is
// resolved in file mode whatever the environment supplies; absent one, the modes
// are exclusive and the mode decides.
func probeEndpoint(opts probeOptions) (config.Endpoint, error) {
	env := config.EnvOverrides(os.Getenv)
	s := opts.inputs(env)
	mode, err := config.ModeOf(s)
	if err != nil {
		return config.Endpoint{}, err
	}
	resolution := mode
	if opts.Model != "" {
		resolution = config.ModeFile
	}
	ep, err := config.ResolveFlag(s, resolution, config.RoleScoring, opts.Model, "--model")
	if err != nil {
		return config.Endpoint{}, err
	}
	if resolution == config.ModeFile {
		// This resolution read the pool, and a faulted entry is what a probe
		// is asking about.
		supersededProvider(opts.Stderr, opts.Config)
		warnFaults(opts.Stderr, opts.Faults)
	}
	return ep, nil
}

// probeAnswer is what one answered call reports after provider and model: when
// the first delta arrived, how long the call took end to end, how the reply
// ended and what it said, and the tokens the provider reported.
func probeAnswer(a model.ChatAttempt, wall time.Duration) []toolresult.Field {
	return []toolresult.Field{
		{Key: "first-token-ms", Value: int(a.FirstToken.Milliseconds())},
		{Key: "wall-ms", Value: int(wall.Milliseconds())},
		{Key: "finish-reason", Value: stringOrNull(a.FinishReason)},
		{Key: "reply", Value: a.Reply},
		{Key: "usage", Value: probeUsage(a)},
	}
}

// probeUsage is the call's token accounting as the provider reported it.
// `reported` says whether a usage block arrived at all, so a zero count is a
// measurement only where it is true; the reasoning and cached shares are the
// two a provider leaves out of the block most often, and neither is ever
// inferred from the other.
func probeUsage(a model.ChatAttempt) toolresult.Record {
	return toolresult.Record{
		{Key: "reported", Value: a.UsageReported},
		{Key: "prompt-tokens", Value: a.Usage.PromptTokens},
		{Key: "completion-tokens", Value: a.Usage.CompletionTokens},
		{Key: "reasoning-tokens", Value: a.Usage.ReasoningTokens},
		{Key: "cached-tokens", Value: a.Usage.CachedPromptTokens},
	}
}

var (
	probeFlagModel   string
	probeFlagPrompt  string
	probeFlagTimeout time.Duration
)

var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "ask one model one question, as a heartbeat of a model selection",
	Long: `Ask one model one question and report what came back: the outcome, the
provider and model asked, the time to the first token, the call's wall time,
the finish reason, the reply, and the tokens the provider reported. One call,
one turn: no tools, no history, no system prompt of its own, and no KB.

Without --model the probe asks the configured scoring role — the selection a
run would make, in file mode or in the environment's — so a probe with no
arguments is the heartbeat. --model names a <provider>/<model> reference to
probe instead, resolved against the same pool. A model that does not resolve is
refused, naming what is missing. --prompt replaces the default one-word check.

The call goes through the same registry a scoring run's asks go through, so the
endpoint is reached over the wire shape its pool entry names. Nothing is read
from a KB and no state directory is read or written: the probe prints its
document and exits. Its result is one YAML document on stdout.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := invocationDir(cmd)
		if err != nil {
			return err
		}
		opts, err := loadVerbContext(wd)
		if err != nil {
			return err
		}
		opts.Timeout = probeFlagTimeout
		code, err := runProbe(cmd.Context(), probeOptions{providerOptions: opts, Model: probeFlagModel,
			Prompt: probeFlagPrompt, Stdout: cmd.OutOrStdout(), Logger: processLog.logger})
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
	probeCmd.Flags().StringVar(&probeFlagModel, "model", "",
		"a <provider>/<model> reference to probe instead of the configured scoring role")
	probeCmd.Flags().StringVar(&probeFlagPrompt, "prompt", "", "the check to send (default: "+probePrompt+")")
	registerTimeoutFlag(probeCmd, &probeFlagTimeout, "deadline for the probe's call")
	// No MCP tool binds it: a tool would let an agent poll a model.
	rootCmd.AddCommand(mcpBinding(probeCmd, mcpExcluded))
}
