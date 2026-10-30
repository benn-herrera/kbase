package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

// probePrompt is what a probe asks where --prompt names nothing: a one-word
// check, so the heartbeat costs a handful of tokens and its reply is short.
const probePrompt = "Reply with the single word: ok"

// probeDecisionPrompt is what a decision probe asks where --prompt names
// nothing. The chat path's default names a word to reply with, which no
// decision surface can answer; a decision call takes one predicate, so the
// default check is one.
const probeDecisionPrompt = "This selection answers a decision ask"

// probeDecisionQuestion is the key a decision probe's answer comes back under,
// and the one question it asks.
const probeDecisionQuestion = "probe"

// probeSelections is what a probe may be told to ask — one selection per
// invocation — in the order a refusal that lists them offers them.
var probeSelections = []string{"--model", "--decision", "--text", "--scoring"}

// probeOptions is the probe verb's input: the shared provider-reaching options,
// the reference and the prompt the flags name, and the stream its result
// document goes to.
type probeOptions struct {
	providerOptions

	// Model is the --model flag: a <provider>/<model> reference to probe
	// instead of a role's selection, "" where the flag names none.
	Model string

	// Decision, Text and Scoring are the role flags: each asks the selection
	// the configuration resolves for that role, and supplies no model of its
	// own. A probe asks one selection, so at most one of the three stands with
	// Model.
	Decision, Text, Scoring bool

	// Prompt is the --prompt flag: the check to send, "" taking probePrompt on
	// the chat path and probeDecisionPrompt on the decision one.
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
		toolresult.Field{Key: "selection", Value: probeSelectionName(opts, ep)},
		toolresult.Field{Key: "model", Value: stringOrNull(ep.Model)})
	// The registry is scoring's client path, so the probe speaks the wire shape
	// the registry gives the selection's role.
	reg := modelclient.New(opts.NewClient, opts.Logger, "", probeAskConcurrency)
	defer reg.CloseAll()
	client := reg.Get(ep)
	warnCleartextKey(opts.Stderr, ep)
	if !opts.Decision {
		// The effective sampling is stated whether or not the call is
		// answered: a failure is read against the sampling that met it.
		sampling := model.SamplingOf(client)
		fields = append(fields,
			toolresult.Field{Key: "temperature", Value: *sampling.Temperature},
			toolresult.Field{Key: "thinking", Value: sampling.Thinking.String()})
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	// The one call is the one item of a queue of one. Its deadline and its
	// wall clock both start inside the item, at the call, so neither covers a
	// wait for the token.
	var (
		answer  []toolresult.Field
		callErr error
		wg      sync.WaitGroup
	)
	err = model.NewQueue(probeAskConcurrency).Go(ctx, &wg, func(slot *model.Slot) func() {
		answer, callErr = probeAsk(ctx, slot, client, ep, opts, timeout)
		return nil
	})
	if err != nil {
		return failed(fields, err)
	}
	wg.Wait()
	if callErr != nil {
		return failed(fields, modelclient.KeyRefused(callErr, ep.Provider))
	}
	return toolresult.Done, append(fields, answer...)
}

// probeAskConcurrency is the probe's queue: one call, so one token.
const probeAskConcurrency = 1

// probeAsk is the probe's one call through slot, under timeout counted from
// the call's start, and the fields its answer reports. The selection's own
// surface answers it: --decision names the decision role, whose asks are
// decision calls on whichever surface its entry declares, and a chat request
// to that selection is the mismatch a probe exists to find. Every other
// selection is asked the one chat turn, carrying the sampling client sends.
// No backoff: a heartbeat re-issues at once, so the deadline is spent on
// attempts rather than on pauses.
func probeAsk(ctx context.Context, slot *model.Slot, client model.Client, ep config.Endpoint, opts probeOptions, timeout time.Duration) ([]toolresult.Field, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	if opts.Decision {
		prompt := opts.Prompt
		if prompt == "" {
			prompt = probeDecisionPrompt
		}
		resp, err := slot.Decision(callCtx, client, probeDecisionRequest(ep.Model, prompt))
		if err != nil {
			return nil, err
		}
		return probeDecisionAnswer(resp, time.Since(started)), nil
	}
	prompt := opts.Prompt
	if prompt == "" {
		prompt = probePrompt
	}
	res, err := slot.Chat(callCtx, client, model.ChatRequest{Model: ep.Model, Prompt: prompt})
	// An answer cut off at the completion cap before any text is still the
	// selection answering, so it is reported rather than failed.
	if err != nil && res.Final.Outcome != model.ChatTruncatedEmpty {
		return nil, err
	}
	return probeAnswer(res.Final, time.Since(started)), nil
}

// probeDecisionRequest is the one ask a decision selection receives: the check
// as the instructions of one predicate, which both decision surfaces carry
// without a criterion set the probe would have to invent, under the key the
// answer comes back keyed by.
func probeDecisionRequest(modelID, prompt string) model.DecisionRequest {
	return model.DecisionRequest{
		Model: modelID,
		Questions: []model.DecisionQuestion{{
			Key:          probeDecisionQuestion,
			Type:         "noul",
			Instructions: prompt,
		}},
	}
}

// probeEndpoint is the selection the probe asks: --model's reference where the
// flag names one, and otherwise the role a role flag names — the scoring role
// where none did, which is the selection a run would make.
func probeEndpoint(opts probeOptions) (config.Endpoint, error) {
	role, roleFlag, err := probeRole(opts)
	if err != nil {
		return config.Endpoint{}, err
	}
	// The flag a refusal names where a flag supplied the model: --model where
	// the probe asks a reference, and none where a role flag named the role — a
	// role flag supplies no model, so the role resolves from the configuration
	// alone. defaulted is the case neither names: nothing named a selection, so
	// the probe asks the scoring role because that is the selection a run would
	// make, and its refusal says so rather than naming a flag nobody passed.
	defaulted := roleFlag == "" && opts.Model == ""
	flagKey := "--model"
	if roleFlag != "" {
		flagKey = ""
	}
	ep, err := config.ResolveFlag(opts.inputs(), role, opts.Model, flagKey, defaulted)
	if err != nil {
		return config.Endpoint{}, err
	}
	// A faulted entry is what a probe is asking about.
	warnFaults(opts.Stderr, opts.Faults)
	return ep, nil
}

// probeRole is the role the probe resolves and the role flag that named it —
// "" where none did, and the scoring role where nothing named one. A role flag
// names a role and supplies no model, so the resolution is handed no flag for
// it. More than one selection — two role flags, or a role flag and --model — is
// a usage refusal naming the four that may be chosen once, before anything
// resolves.
func probeRole(opts probeOptions) (config.Role, string, error) {
	var named []string
	if opts.Model != "" {
		named = append(named, "--model")
	}
	role, roleFlag := config.RoleScoring, ""
	for _, f := range []struct {
		flag  string
		role  config.Role
		given bool
	}{
		{"--decision", config.RoleDecision, opts.Decision},
		{"--text", config.RoleText, opts.Text},
		{"--scoring", config.RoleScoring, opts.Scoring},
	} {
		if !f.given {
			continue
		}
		named = append(named, f.flag)
		role, roleFlag = f.role, f.flag
	}
	if len(named) > 1 {
		return "", "", toolresult.Refusal{{Check: toolresult.CheckUsage, Allowed: probeSelections,
			Detail: fmt.Sprintf("a probe asks one selection, and more than one was named (%s): pass one of %s",
				strings.Join(named, ", "), strings.Join(probeSelections, ", "))}}
	}
	return role, roleFlag, nil
}

// probeSelectionName is which selection answered, as the document states it:
// the reference --model named, else the name of the role that named it, so a
// reader never infers the selection from the model id.
func probeSelectionName(opts probeOptions, ep config.Endpoint) string {
	if opts.Model != "" {
		return config.Reference{Provider: ep.Provider, Model: ep.Model}.String()
	}
	return string(ep.Role)
}

// probeAnswer is what one answered call reports after provider and model: when
// the first delta arrived, how long the call took end to end, how the reply
// ended and what it said, and the tokens the provider reported. A decision call
// is a blocking surface — no delta and no finish reason — so probeDecisionAnswer
// reads this with both at their zero.
func probeAnswer(a model.ChatAttempt, wall time.Duration) []toolresult.Field {
	return probeAnswerFields(a.FirstToken, wall, a.FinishReason, a.Reply, a.Usage, a.UsageReported)
}

// probeDecisionAnswer is what one answered decision call reports: the surface
// streams nothing, so no first token arrived and no finish reason came back,
// and the reply is the answer itself. A response carrying no answer reports an
// empty reply: what arrived is what is stated, and nothing is invented for a
// surface that said nothing.
func probeDecisionAnswer(resp model.DecisionResponse, wall time.Duration) []toolresult.Field {
	var reply string
	if len(resp.Answers) > 0 {
		reply = probeDecisionReply(resp.Answers[0])
	}
	return probeAnswerFields(0, wall, "", reply, resp.Usage, resp.Usage.PromptTokens > 0)
}

// probeDecisionReply is one decision answer as the probe's reply: the figure a
// predicate carried, the letter a choice picked, and the answer's type where it
// carried neither — a refusal is an answer that decided nothing.
func probeDecisionReply(a model.DecisionAnswer) string {
	switch {
	case a.Type == "noul":
		return strconv.FormatFloat(a.Noul, 'f', -1, 64)
	case a.Choice != "":
		return a.Choice
	default:
		return a.Type
	}
}

// probeAnswerFields is the answered call's five keys, in the document's order.
func probeAnswerFields(firstToken, wall time.Duration, finishReason, reply string, usage model.Usage, reported bool) []toolresult.Field {
	return []toolresult.Field{
		{Key: "first-token-ms", Value: int(firstToken.Milliseconds())},
		{Key: "wall-ms", Value: int(wall.Milliseconds())},
		{Key: "finish-reason", Value: stringOrNull(finishReason)},
		{Key: "reply", Value: reply},
		{Key: "usage", Value: probeUsage(usage, reported)},
	}
}

// probeUsage is the call's token accounting as the provider reported it.
// `reported` says whether a usage block arrived at all, so a zero count is a
// measurement only where it is true; the reasoning and cached shares are the
// two a provider leaves out of the block most often, and neither is ever
// inferred from the other.
func probeUsage(u model.Usage, reported bool) toolresult.Record {
	return toolresult.Record{
		{Key: "reported", Value: reported},
		{Key: "prompt-tokens", Value: u.PromptTokens},
		{Key: "completion-tokens", Value: u.CompletionTokens},
		{Key: "reasoning-tokens", Value: u.ReasoningTokens},
		{Key: "cached-tokens", Value: u.CachedPromptTokens},
	}
}

var (
	probeFlagModel    string
	probeFlagDecision bool
	probeFlagText     bool
	probeFlagScoring  bool
	probeFlagPrompt   string
	probeFlagTimeout  time.Duration
)

var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "ask one model one question, as a heartbeat of a model selection",
	Long: `Ask one model one question and report what came back: the outcome, the
selection and model asked, the temperature and thinking a chat call sent (its
role's sampling, the scoring role's for --model), the time to the first
token, the call's wall time, the finish reason, the reply, and the tokens the
provider reported. An answer cut off at the completion cap before any text is
reported as done, with finish reason length and an empty reply. One call, one
turn: no tools, no history, no system prompt of its own, and no KB.

A probe asks one selection. Without any of --model, --decision, --text or
--scoring it asks the configured scoring role — the selection a run would
make — so a probe with no arguments is the heartbeat. --decision, --text and
--scoring each ask the selection the configuration resolves for that role:
models.<role> in config.toml. --model names a <provider>/<model> reference to
probe instead, resolved against the same pool. The four are
exclusive, and more than one is a usage refusal naming them. A selection that
does not resolve is refused, naming the role's own key or the flag, and what is
missing. --prompt replaces the default check: a word to reply with on the chat
path, a predicate to answer on the decision one.

The call goes through the same registry a scoring run's asks go through, so the
endpoint is reached over the wire shape its pool entry names. A decision
selection is asked a decision call on the surface its entry declares, as a
run's multichoice asks are: a surface that reports no first token and no finish
reason, which the document states as zero and null. A decision selection
always asks the OpenAI-shaped decision surface on its entry's base URL,
whatever the entry's api. Nothing is read from a KB and no state directory is
read or written: the probe prints its document and exits. Its result is one YAML document on
stdout.`,
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
			Decision: probeFlagDecision, Text: probeFlagText, Scoring: probeFlagScoring,
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
		"a <provider>/<model> reference to probe instead of a role's selection")
	probeCmd.Flags().BoolVar(&probeFlagDecision, "decision", false,
		"probe the selection the configuration resolves for the decision role (models.decision)")
	probeCmd.Flags().BoolVar(&probeFlagText, "text", false,
		"probe the selection the configuration resolves for the text role (models.text)")
	probeCmd.Flags().BoolVar(&probeFlagScoring, "scoring", false,
		"probe the selection the configuration resolves for the scoring role, the default (models.scoring)")
	probeCmd.Flags().StringVar(&probeFlagPrompt, "prompt", "", "the check to send (default: "+probePrompt+", or "+probeDecisionPrompt+" for --decision)")
	registerTimeoutFlag(probeCmd, &probeFlagTimeout, "deadline for the probe's call")
}
