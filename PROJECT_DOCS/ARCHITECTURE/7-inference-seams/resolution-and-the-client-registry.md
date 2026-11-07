### Resolution and the client registry

**Pattern:** Factory registry with lazy, shared clients.

`config.Resolve` (`internal/config/resolve.go`) answers one role's `Endpoint` from the provider its
reference names. For `text` and `scoring` the endpoint has `Temperature` and `Thinking` from
`[sampling.<role>]` with the layer of each, and a temperature outside the range the protocol accepts is
refused. Resolution reads only the loaded configuration: no file, no connection, no stderr. A refusal's
remedy may name the command that fixes it (`kbase configure-models`). `config.ResolveFlag` is the same
resolution with the flag that supplies a role's model named by the caller, so `probe`'s `--model` is
named in a refusal where `--scoring-model` would be. `config.Select` is `models`' choice of a provider
to send a request to: a flag, else the only provider (SPEC §9 Provider selection is the catalogue
verbs' alone). No other site applies a selection or a resolution rule.

A configuration refusal is a `result.Refusal` whose items name their check: `config.CheckProvider`
unless an item names its own, as `result.CheckUsage` does. A decision with more than one side to state
has several items. An item's remedy is its clearing text alone. `cmd`'s `refused` composes
`config.ShowConfigRemedy` over every `provider` item a verb refuses with; `show-config` refuses with
none, so it reports its role diagnostics with the clearing text alone; `walk.launch` composes it for the
build's preflight refusal and reads the key and path of a one-item refusal.

`modelclient.Registry` is the only constructor of a `model.Client`. It keeps one client per provider
name, wire protocol and resolved sampling, so roles whose resolved sampling is equal, an unset
setting being equal to its stated default, share one client and its connection pool. A client is opened
on first use, connects with the key the provider resolved to at load, and `CloseAll` closes every client
before the verb returns on every outcome. `modelclient.NewClientFunc` is the constructor the registry
calls, `model.NewClient` in production, so a verb runs without a network. `samplingOf` maps the
configuration's `thinking` onto `model.Sampling`.

