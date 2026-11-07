package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
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
var probeSelections = []string{argmark.Flag("model"), argmark.Flag("role")}

// probeRoles is what --role may name: each role's name, in config.Roles'
// order.
var probeRoles = func() []string {
	names := make([]string, len(config.Roles))
	for i, r := range config.Roles {
		names[i] = string(r)
	}
	return names
}()

// probeOptions is the probe verb's input: the shared provider-reaching options,
// the reference and the prompt the flags name, and the stream its result goes
// to.
type probeOptions struct {
	providerOptions

	// Model is the --model flag: a <provider>/<model> reference to probe
	// instead of a role's selection, "" where the flag names none.
	Model string

	// Role is the --role flag, "" where it names none: it asks the selection
	// the configuration resolves for that role, and supplies no model of its
	// own. A probe asks one selection, so Role and Model never stand together.
	Role config.Role

	// Prompt is the --prompt flag: the check to send, "" taking probePrompt on
	// the chat path and probeDecisionPrompt on the decision one.
	Prompt string

	Stdout io.Writer
	Logger log.Logger
}

// runProbe asks one model one question and writes the result, and
// returns the exit code the outcome maps to. It is a diagnostic of a model
// selection: one call, no turn, no tool, no history, no KB and no state. The
// configuration's faults are the result's warnings and lines on Stderr.
func runProbe(ctx context.Context, opts probeOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "probe"); err != nil {
		return 0, err
	}
	warnings := warningsField(warnFaults(opts.Stderr, opts.Faults))
	outcome, fields := probeCall(ctx, opts)
	return emitResult("probe", opts.Stdout, outcome, toolresult.WithWarnings(fields, warnings))
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
		toolresult.Field{Key: "probed", Value: probeSelectionName(opts, ep)},
		toolresult.Field{Key: "model", Value: stringOrNull(ep.Model)})
	// The registry is scoring's client path, so the probe speaks the wire shape
	// the registry gives the selection's role.
	reg := modelclient.New(opts.NewClient, opts.Logger, "", probeAskConcurrency)
	defer reg.CloseAll()
	client := reg.Get(ep)
	warnCleartextKey(opts.Stderr, ep)
	if opts.Role != config.RoleDecision {
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
// surface answers it: --role decision names the decision role, whose asks are
// decision calls on whichever surface its entry declares, and a chat request
// to that selection is the mismatch a probe exists to find. Every other
// selection is asked the one chat turn, carrying the sampling client sends.
func probeAsk(ctx context.Context, slot *model.Slot, client model.Client, ep config.Endpoint, opts probeOptions, timeout time.Duration) ([]toolresult.Field, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	if opts.Role == config.RoleDecision {
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
// flag names one, and otherwise the role --role names — the scoring role where
// it named none, which is the selection a run would make.
func probeEndpoint(opts probeOptions) (config.Endpoint, error) {
	role, roleFlag, err := probeRole(opts)
	if err != nil {
		return config.Endpoint{}, err
	}
	// The flag a refusal names where a flag supplied the model: --model where
	// the probe asks a reference, and none where --role named the role —
	// --role supplies no model, so the role resolves from the configuration
	// alone. defaulted is the case neither names: nothing named a selection, so
	// the probe asks the scoring role because that is the selection a run would
	// make, and its refusal says so rather than naming a flag nobody passed.
	defaulted := roleFlag == "" && opts.Model == ""
	flagKey := argmark.Flag("model")
	if roleFlag != "" {
		flagKey = ""
	}
	return config.ResolveFlag(opts.inputs(), role, opts.Model, flagKey, defaulted)
}

// probeRole is the role the probe resolves and the flag that named it: --role
// where it names one, else "" and the scoring role. --role names a role and
// supplies no model, so the resolution is handed no flag for it. --role beside
// --model is a usage refusal naming the two that may be chosen once, and a
// --role outside config.Roles one keyed --role, each before anything resolves.
func probeRole(opts probeOptions) (config.Role, string, error) {
	if opts.Role == "" {
		return config.RoleScoring, "", nil
	}
	if opts.Model != "" {
		return "", "", toolresult.Refusal{{Check: toolresult.CheckUsage, Allowed: probeSelections,
			Detail: fmt.Sprintf("a probe asks one selection, and both %s were named: pass one",
				strings.Join(probeSelections, " and "))}}
	}
	if !slices.Contains(config.Roles, opts.Role) {
		return "", "", toolresult.Refusal{{Check: toolresult.CheckUsage, Key: argmark.Flag("role"), Allowed: probeRoles,
			Detail: fmt.Sprintf("%s names a role, %s, and %q is none of them; absent, a probe asks %s",
				argmark.Flag("role"), proseSeries(probeRoles, "or"), opts.Role, config.RoleScoring)}}
	}
	return opts.Role, argmark.Flag("role"), nil
}

// probeSelectionName is which selection answered, as the result states it:
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

// probeAnswerFields is the answered call's five keys, in the result's order.
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
	probeFlagModel   string
	probeFlagRole    string
	probeFlagPrompt  string
	probeFlagTimeout time.Duration
)

// probeMCPNote is probe's MCP note: a tool call spends money, so an agent makes
// one only at the user's direction.
const probeMCPNote = "Each call is a paid model call: run it only when the user directs it, one call per direction, never on your own initiative or to retry."

var probeCmd = mcp.NoteEveryCall(mcp.Notes(&cobra.Command{
	Use:   "probe",
	Short: "ask one model one question and report what came back",
	Long: `Ask one model one question and report what came back: the outcome, the
selection probed ("probed") and the model asked, the temperature and thinking a
chat call sent (its role's sampling, the scoring role's for ` + argmark.Flag("model") + `),
the time to the first token, the call's wall time, the finish reason, the
reply, and the tokens the provider reported. Each call is a paid model call. An
answer cut off at the completion cap before any text is reported as done, with
finish reason length and an empty reply. One call, one turn: no tools, no
history, no system prompt of its own, and no KB.

A probe asks one selection. ` + argmark.Flag("role") + ` names a role, decision, text or scoring,
and asks the selection the configuration resolves for it: models.<role> in
config.toml. ` + argmark.Flag("model") + ` names a <provider>/<model> reference to probe instead,
its provider part in any case, resolved against the same providers. The two
are exclusive: both together are refused (check: usage), naming them, and so
is a role outside the three. Without either, the probe asks the scoring role —
the selection a run would make — so a probe with no arguments asks the
selection a run would use.

A selection that does not resolve is refused, naming the key or flag at fault
and what is missing:
- the role's key is unset or holds no <provider>/<model> reference — check:
  provider; step: ` + argmark.Call("configure-models", map[string]any{string(config.RoleScoring): "<provider>/<model>"}) + `, with the flag of the role asked;
- the reference's provider is declared in no layer's providers.toml — check:
  provider; step: ` + argmark.Call("configure-providers", map[string]any{"provider": "<provider>", "base-url": "<url>"}) + `;
- the reference's provider failed to load — check: provider; no step, and the
  detail gives the fault's reason;
- a text or scoring selection's sampling.<role>.temperature lies outside what
  its provider's chat protocol accepts (0 to 1 for anthropic, 0 to 2
  otherwise) — check: provider, keyed by that setting; step: set it within the
  range, or remove it from the layer that states it;
- ` + argmark.Flag("model") + ` holds no reference — check: usage; no remedy.
Each check: provider remedy is its step, where there is one, then ` + argmark.Call("show-config", nil) + `.

` + argmark.Flag("prompt") + ` replaces the default check: a word to reply with on the chat path, a
predicate to answer on the decision one.

The call goes through the same registry a scoring run's asks go through, so the
endpoint is reached over the wire shape its provider names. A decision
selection is asked a decision call on the surface its provider declares, as a
run's multichoice asks are: a surface that reports no first token and no finish
reason, which the result states as zero and null. A decision selection
always asks the OpenAI-shaped decision surface on its provider's base URL,
whatever the provider's api. Nothing is read from a KB and no state directory
is read or written.

` + configWarningsHelp("the result"),
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
			Role: config.Role(probeFlagRole), Prompt: probeFlagPrompt, Stdout: cmd.OutOrStdout(), Logger: processLog.logger})
		if err != nil {
			return err
		}
		if code != 0 {
			return exitCode(code)
		}
		return nil
	},
}, probeMCPNote, ""))

func init() {
	probeCmd.Flags().StringVar(&probeFlagModel, "model", "",
		"a <provider>/<model> reference to probe instead of a role's selection")
	mcp.Brief(probeCmd, "model", "provider/model to probe instead")
	probeCmd.Flags().StringVar(&probeFlagRole, "role", "",
		"the role whose configured selection to probe: decision, text or scoring (models.<role>); with neither it nor "+argmark.Flag("model")+", scoring")
	mcp.EnumFlag(probeCmd.Flags().Lookup("role"), probeRoles...)
	mcp.Brief(probeCmd, "role", "role whose model to probe")
	probeCmd.Flags().StringVar(&probeFlagPrompt, "prompt", "", fmt.Sprintf("the check to send (default: %q, or %q for %s)", probePrompt, probeDecisionPrompt, argmark.FlagValue("role", "decision")))
	mcp.Brief(probeCmd, "prompt", "prompt text to send in place of the default")
	registerTimeoutFlag(probeCmd, &probeFlagTimeout, "deadline for the probe's call", "deadline for the call, e.g. 30s")
}
