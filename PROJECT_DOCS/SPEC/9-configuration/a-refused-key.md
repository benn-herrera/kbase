### A refused key

Nothing checks a key before a call. A provider that answers a call 401 or 403
has refused the call's key — a missing key and a wrong one alike — and the call fails as any
provider refusal does ([§8](../8-maintenance-subcommands/index.md), `provider-refused`). Its failure item carries `remedy`
`<command-that-prints-the-key> | kbase auth add <provider>`, `<provider>` the provider the call
dialled: the user runs it, or an agent runs it with the command the user names or `show-config`'s
`upgrade` gives; its `detail` ends, before the outcome's ask, by saying so, and that no one asks
for the key, so it never enters a conversation. The call
failures that carry it are a build's — the `inference-failed` stop of a claim-graph stage and the
overview's `inference` failure — a scoring run's `failures` (from the attempt's ledger line, [§8](../8-maintenance-subcommands/index.md)),
`probe`'s failure, and `models`' and `configure-models`' failure to list a catalogue. A call ended by any
other status carries no such remedy, and neither does `configure-providers`' dial, whose failure is
its `next` ([§9](provider-commands.md), "Provider commands").

