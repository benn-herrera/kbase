### `probe`

```
kbase probe [--model <provider>/<model>] [--decision | --text | --scoring]
            [--prompt TEXT] [--timeout DURATION]
```

One ask, no KB and no state: a diagnostic of a model selection, answered by the selection itself.
A probe asks one selection, and its flags name it: `--model` names a `<provider>/<model>` reference,
resolved against the pool as a role's reference is; `--decision`, `--text` and `--scoring` each ask
the selection the configuration resolves for that role — the `models.<role>` reference — and supply
no model of their own; and with none of the four given the probe asks the `scoring` role, the
selection a scoring run would make, so `kbase probe` with no arguments asks the selection a run
would use. The four are mutually exclusive: more than one is a usage refusal naming all four as
what may be chosen once, before anything resolves or dials. A selection that does not resolve is
`refused`, naming the role's own key — `models.<role>` — or the flag, and what is missing, before
any client is opened. The document states which selection answered: the role's name where a role flag
or the default named it, and the reference where `--model` did.

A chat selection is one tool-less chat request through the registry a scoring run's asks take, so
an endpoint is reached over the wire shape its provider names: no tools, no history, an empty
system prompt, and `--prompt` in place of the default one-word check. It carries its role's sampling
settings ([§9.2](../9-configuration/9.2-config-toml-the-choices/index.md)) — the `scoring` role's where `--model` names the selection — and the document
reports the effective settings, `temperature` and `thinking`. A `--decision` selection is
one decision call on the surface its provider's `decisionsFlavor` names, with no system message, and
its `--prompt` in place of the default predicate. `--timeout` bounds the call (default `30s`); a
call that does not complete is re-issued at once up to the attempt count, so that deadline is spent
on attempts rather than on pauses ([§12](../12-interim-releases/index.md)). The document reports what a ledger cannot — the selection,
the provider, the model, the time to the first token, the call's wall time, the finish reason, the
reply, and the usage the provider reported — and its outcome follows the call: `done` where it
completed, or where a chat answer was cut off at the completion cap before any answer text
(`finish-reason` `length`, `reply` empty); `failed` otherwise, with the attempt's own outcome and the
provider's words in the failure, and its `remedy` where the provider refused the call's key ([§9](../9-configuration/index.md)). It reads no KB and reads or writes no state directory: it prints its document and exits.

`probe` is also a read-only MCP tool taking the same arguments ([page §3.7](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md); [§11](../11-the-mcp-server.md)). Each call is
a paid model call, and its MCP note is the one [page §4.2](../../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) quotes.

Result keys: [§7](../7-tool-result-contract-and-exit-codes/index.md).

