### Probe

**Pattern:** Health-check / smoke call.

`cmd/probe.go` makes one call on the path a run uses and reports it (SPEC §8 `probe`). It is a
diagnostic, the CLI command and the read-only MCP tool alike ([§2](../2-invariants.md), I9).

- **Selection.** `config.ResolveFlag` resolves one selection. `--model` supplies a reference and names
  itself in a refusal. `--role` names the role and supplies no model, so the role's model comes
  from the configuration and a refusal names its key alone.
- **Client and call.** The client comes from the registry, as `score`'s does. A `--role decision`
  selection makes one `Slot.Decision` call on the API its provider's `decisionApi` names; every other
  selection makes one `Slot.Chat` call. `--prompt` replaces the default check of either path,
  `probeDecisionPrompt` or `probePrompt`. A chat request to a decision selection is the mismatch a probe
  exists to find.
- **One call.** The call repeats and pauses as every model call does ([Retry and attempt
  outcomes](retry-and-attempt-outcomes.md)). `--timeout` bounds it, and the
  deadline and the `wall-ms` clock start inside the item, so neither counts a wait for a permit. The
  chat path reports `ChatAttempt.FirstToken`, measured in `chatOnce`: request start to the first
  non-empty content or reasoning delta, zero where none arrived. The decision API streams nothing, so
  its result has `first-token-ms` zero and `finish-reason` null.
- **Sampling.** A chat selection's client is the registry's for its role, so the call has that role's
  sampling (the `scoring` role's for `--model`), and the result reports `model.SamplingOf(client)`.
  Where the call ends `truncated-empty`, the chat path reports `ChatResult.Final` as the answer
  instead of failing.
- **Isolation.** `probe` opens no `kb.Source`, resolves no state directory and takes no lock.

