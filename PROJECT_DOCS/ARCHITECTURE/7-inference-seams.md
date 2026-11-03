## 7. Inference seams

- Kept from kbase: provider management (`internal/config`) and the client (`internal/model`). The
  call path is a `model.Slot`'s `Chat` for the overview call and for each scoring call, and its
  `Decision` for the multichoice asks. A `Slot` is issued only by the invocation's one `model.Queue`, from
  `Go`, and only while one of its N tokens is free. `build` and `score` build the queue from
  `--ask-concurrency`, else `[asks] concurrency`, else 4.
  `kbase probe` takes one call on one of the two by its selection, which the next item states; the decision role's model is an id on whichever surface its provider speaks
  (`decisionsFlavor`, SPEC §9.1), and kbase's three roles apply — multichoice asks go to the `decision`
  role, the overview call to `text`, and `kbase score`'s asks to `scoring` (SPEC §9). Each role
  names its own endpoint and its own model, in `[models]` as a `<provider>/<model>` reference; no
  role falls back to another.
- **`kbase probe` is a diagnostic on the same path**, the CLI command and the read-only `probe` tool
  alike (I9). `cmd/probe.go` resolves one selection through
  `config.ResolveFlag` — `--model` supplying a reference and naming itself in a refusal, a role flag
  (`--decision`, `--text`, `--scoring`) naming the role to resolve and supplying no model, so that
  role's model comes from the configuration and a refusal names its key alone — then opens
  the client through `internal/modelclient`'s registry exactly as `score` does, so the chat path speaks
  whichever chat protocol the provider's `api` names, and the registry is the only site that builds a
  wire client. One call, no backoff, bounded by `--timeout`, and the selection's own surface is
  what answers it: a `--decision` selection makes one `Slot.Decision` call on the surface its
  provider's `decisionsFlavor` names, every other selection the one `Slot.Chat` call; `--prompt`
  replaces either path's default check — `probeDecisionPrompt` on the decision one, `probePrompt`
  on chat — and a chat request to a decision selection is the mismatch a probe exists to find.
  The chat attempt's `FirstToken` — request start to the first non-empty content
  or reasoning delta, measured in `chatOnce`, zero where none arrived — is what the result reports
  beside the call's wall time; the decision surface streams nothing, so the document reports
  `first-token-ms` zero and `finish-reason` null. It opens no `kb.Source`, resolves no state
  directory and takes no lock: the call is one item on a queue of one (`model.NewQueue(1)`, no
  flag), and the decision path is `probeEndpoint` → `Registry.Get` → the item's `Slot.Decision` →
  the result document, the chat path the same through `Slot.Chat` (SPEC §8). A chat selection's
  client is the registry's for its role, so the call carries that role's sampling — the `scoring`
  role's for `--model` — and the document reports the effective settings, `model.SamplingOf(client)`; where the call ends
  `truncated-empty`, the chat path reports `ChatResult.Final` as the answer rather than failing. The
  call's deadline and its `wall-ms` clock start inside the item, so neither counts a wait for a token.
- Every chat call is one tool-less chat request answered from its prompt alone. The transport keeps
  at least N idle connections per host, N being the invocation's concurrency, so its connections are
  reused rather than redialled (`model.Endpoint.IdleConns`, set by `modelclient`'s registry). `internal/model`
  holds two implementations of that request, and the registry builds the one the endpoint provider's
  `api` selects — for the `decision` role always the OpenAI one, which alone carries the decision
  surfaces: `model.OpenAIClient` (`openai_client.go`) speaks the OpenAI-compatible
  `chat/completions` surface, and `model.AnthropicClient` (`anthropic_client.go`) speaks the
  Anthropic Messages surface — `POST <baseUrl>/messages`, with its own event sequence, its
  `x-api-key`/`anthropic-version` headers and its catalogue shape — and is what a provider silent on
  `api` gets.
  `model.NewClient` is the dispatch, and the protocol reaches it as a value the caller resolved onto
  `model.Endpoint` from `config.Endpoint.Protocol`, which the configuration fills from the provider.
  The body each client sends is its protocol's, never one shaped by
  host or provider name. `model.Sampling` is the one sampling type: each client resolves
  `model.Endpoint.Sampling` once, at construction, with `Sampling.Resolved()` — temperature 0 and
  thinking off where the role states none — and `chat()` copies it onto every request, shaping no
  field of either protocol. Each protocol's wire rules live only in its body encoder
  (`encodeRequest`, `encodeAnthropicRequest`). `model.SamplingOf` returns a copy of the resolved
  sampling — the effective settings — so no caller can alter what a client sends.
  On the OpenAI surface: the role's temperature; thinking off as the provider's own
  `reasoning: {enabled: false}` control with `chat_template_kwargs.enable_thinking: false` beside
  it, and a level as `reasoning_effort: <level>`, `reasoning: {effort: <level>}` and
  `chat_template_kwargs.enable_thinking: true` at once, since a server reads whichever control it
  speaks; the usage chunk requested; and no completion cap, the server's own applying. On the
  Messages surface: no `reasoning`, `reasoning_effort`, `chat_template_kwargs` or
  `prompt_cache_key` field exists, so a run's cache key has no home; thinking off is sent as the
  protocol's own `thinking: {type: disabled}`, the field's absence leaving it at the server's
  default, and a level as `thinking: {type: enabled, budget_tokens: <budget>}` ([§12](12-constants/index.md)); the role's
  temperature is sent whatever the thinking level, a server that rejects the pair failing the call
  like any request it refuses; the system turn is lifted to the top-level `system` string; and the
  protocol's required `max_tokens` is the thinking budget, where thinking is on, plus the
  answer's allowance, `max(256, ceil(characters / 4))` over the ask's messages and system text.
  A call that does not complete is re-issued identically up to the attempt count, pausing where its
  caller's backoff says ([§12](12-constants/index.md)), and a pause the call's context cannot cover is not waited out — the
  call gives up at once, its reason naming the attempts it made; a provider's non-2xx status ends it
  on the attempt that met it unless
  the status is a 429 or a 5xx ([§12](12-constants/index.md)); an attempt cut off at the completion cap before any answer
  text ends the call on that attempt; both wire clients report a non-2xx as the same
  `model.StatusError`, so that classification is shared rather than re-decided per protocol. A
  scoring run's clients are built with the run's id, which the OpenAI client stamps on every
  request it sends as
  `prompt_cache_key` — one key per run, so its asks reach one upstream and share one cache entry; a
  verb that names no run builds its clients with none, and
  the field is then omitted from the body rather than sent empty. Each
  attempt's outcome names what ended it — `ok`, `served-nothing`, `truncated-empty` where the answer reached the completion cap before any
  answer text, `provider-refused` where the
  provider answered with a status that is not success, `transport-failure` where no answer of the
  provider's was had — and records whether the call reasoned, from a reasoning delta on the stream
  or from a usage block reporting reasoning tokens spent, so a call that thought without streaming
  the thought is recorded as having reasoned; the error a call returns carries the last attempt's
  outcome and its text, which is what a failed item's reason records (SPEC §8), and wraps that
  attempt's `model.StatusError` where it met one.
  `model` makes it, with a system message. This holds for the overview call; the multichoice asks take
  the decision surface instead (next item). A build with a calling row left to walk checks a
  provider is configured before its first stage (SPEC §5).
- **A refused key is classified where `model` is imported, and travels outward as data.**
  `StatusError.RefusedKey` (401 or 403) is the one predicate. `asks/call`, `cmd/score.go`'s ask,
  `cmd/probe.go` and `modelclient` (`KeyRefused`, which `Catalogue` applies) match the call's
  `StatusError` with `errors.As` and attach `config.AuthAddRemedy`, the name the dialled provider
  carries filled in, to what travels out: `asks.IncompleteError.Remedy`, which `claimgraph`'s
  inference stop and the build's overview failure carry onto their items; `score.RemedyError`,
  whose remedy and `config.RefusedKeyNote` the attempt's ledger line records; and `modelclient.KeyRefusedError`, which
  `cmd`'s `failed` reads onto the failure item of `probe`, `models` and `configure-models`. `asks/call`
  is told the provider it dials by `call.New`, from `build.Provider`'s per-role provider names. No site
  recovers a status from a reason's text, and nothing checks a key before the call (SPEC §9).
- Configuration is read from layers and merged once, before a verb's own work: `config.Layers`
  resolves the directories and `config.Load` reads and merges the two files of each, returning one
  `Loaded` — the pool and its faults, and the choices. They are
  `<repository>/.kbase/` over `~/.config/kbase`, or the one directory `--config-dir` /
  `$KBASE_CONFIG_DIR` names — `explicit` and `environment` in the source vocabulary — so an
  explicit choice isolates a run from a repository's own `.kbase/`, which every test and pinned
  deployment needs. The caller resolves the repository root with `kb.RepositoryRoot(workDir)`, which
  answers `""` outside a worktree, so `internal/config` takes no git dependency and reads the global
  layer alone there. `parseProviders` folds each table name to lowercase as it reads it, after
  refusing a file two of whose names differ only by case (SPEC §9.1), so the pool, its faults and
  every output name a provider in lowercase, and `config.ParseReference` and `config.Select` fold
  the names they are given the same way. A provider's table is replaced whole by a higher
  layer's of the same name; a `config.toml` field is overlaid one at a time. `config.Load` drops a
  fault naming a provider a higher layer already supplied — the pool's provider is the higher one, so
  the fault describes a value nothing reads — and keeps a higher layer's own fault, whose provider
  never entered the pool. `config.Layer` carries a directory, its source (`explicit`, `environment`,
  `repository`, `global`) and — once `config.Report` has probed it — whether the directory and each
  of the two files stand, `ConfigExists` beside `ProvidersExists` (`show-config`'s
  `config-exists` and `providers-exists`).
  `Provider.Layer` and `Config.ModelLayer` record the layer the merge took each provider and each
  role's value from; `ProviderFault.Path` names the `providers.toml` a fault came from, and a
  `Refusal`'s path names the file a lacking setting belongs in — the write target's
  `providers.toml` or `config.toml` — so
  a layered configuration says which file to edit. The write target is `config.WriteTarget`'s —
  the directory `--config-dir` or `$KBASE_CONFIG_DIR` names, else the global one — which is also
  where the configure commands write with no location flag (`cmd`'s `configureLocation`), so a
  refusal of a name the pool lacks, whose remedy is the `kbase configure-providers` call that adds
  it, names the file that call writes. `configure-models` checks against the stack it writes into
  instead — the layer written and the layers beneath it, `config.WrittenStack`, loaded by `cmd`'s
  `loadWrittenStack`, which `configure-providers --remove` also loads to name what still refers to
  the provider — so its refusal names the written layer's `providers.toml`, and its remedy carries
  the location flag the call carried. Both configure commands' `RunE` resolve the layers
  (`resolveVerbLayers`) without loading them, so no layer outside the stack written is read.
- **Keys: the store is read in `cmd`, the precedence lives in `config`.** `cmd`'s
  `loadConfigContext` reads every file of the credentials store through `internal/auth`
  (`readStoredKeys`) before the load, and hands `config.Load` a `config.StoredKeys` as data — each
  key that opened, by the lowercase name its file is stored under, and a `config.StoreFault` per
  file that did not, built from the `*auth.FileError`'s path and its error apart; a store directory
  that stands and does not list is one nameless `StoreFault` against the directory, with no keys,
  and the load goes on. `config` imports
  no `auth`. `keyKind` is the precedence's one home: the key stored under the provider's name, whatever
  layer supplied its table; else the table's `apiKeyFile`, which `LoadProviders` then reads, and
  which a provider with a stored key never reads, so it raises neither a read fault nor a permission
  warning; else none. `config.Load` turns each store fault whose name a configured provider carries
  into a warning `ProviderFault` naming that provider, `Source` `SourceStoreFile`, its
  `Path` the store file and its `DoesNotOpen` the store fault's, so every configuration-loading
  verb's `warnFaults` writes it; a file no provider names is read by nothing and not reported. The
  nameless directory fault names no provider and becomes a warning `ProviderFault` with `Source`
  `SourceStoreDir`, its `Path` the directory, which `warnFaults` writes as the credentials store's.
  A `ProviderFault`'s `Reason` never repeats its `Path`; `LocatedReason` leads a store fault's
  reason with its path, for a line that stands on its own — `warnFaults`' and a provider's `fault`
  in `show-config`. `config.Report` reads the faults alone, and reports a store fault's
  diagnostic as its path and reason with
  `config.AuthAddRemedy` — the piped form, also `kbase auth add`'s own input-refusal remedy and a
  refused key's failure remedy — as the
  remedy where the file does not open, and for a provider dropped because its `apiKeyFile` could not
  be read (`ProviderFault.KeyFileUnread`), the stored key outranking the file; it alone computes a
  `file` key's `upgrade` and the warning that a stored key leaves an `apiKeyFile` unread, so
  `show-config` alone emits them.
- **Resolution is pure, and connection is the registry's.** `config.Resolve`
  (`internal/config/resolve.go`) answers one role's `Endpoint` from the provider its reference
  names; for `text` and `scoring` it carries `Temperature` and `Thinking` from `[sampling.<role>]`
  with the layer of each, and refuses a temperature outside the protocol's range; it reads only the
  loaded configuration — no file, no connection, no stderr — though a refusal's remedy may name the
  command that fixes it (`kbase configure-models`). `config.ResolveFlag` is the same resolution with the flag that supplies a
  role's model spelled by the caller, so `probe`'s `--model` is named in a refusal where
  `--scoring-model` would be; the caller names no flag where none supplies a model, as `probe`'s role
  flags supply none, and a role the files leave unnamed then names only its key. `config.Select` is
  `models`' choice of
  a provider to dial — a
  flag, else the only provider — and no role is chosen that way. A configuration
  refusal is a `result.Refusal` whose items carry their check — `provider` (`config.CheckProvider`)
  unless it names its own, as `config.CheckUsage` — with several items for a decision that has more
  than one side to state; `internal/build`'s launch refusal reads the key and path of a one-item
  refusal. An item's remedy is its clearing text alone; `cmd`'s `refused` composes
  `config.ShowConfigRemedy` over every `provider` item a verb refuses with, so
  `show-config`, which refuses with none, reports its role diagnostics with the clearing text
  alone; the build's preflight refusal, which leaves through `buildstore`, composes it where it is
  built. No other site applies a selection or a
  resolution rule, and `internal/modelclient`'s `Registry` is the only constructor of a
  `model.Client`: one client per endpoint, wire protocol and resolved sampling — roles whose
  resolved sampling is equal, an unset setting equal to its stated default, share one client and its
  connection pool — opened on first use, keyed by the provider's name, by the protocol and by the
  resolved sampling, dialled with the key the provider resolved to at load, and closed together
  before the verb returns on every outcome. `samplingOf` maps the configuration's `thinking` onto
  `model.Sampling`.
  The report's other facts — the pool's providers and faults, each provider's key kind and upgrade, the
  diagnostics, the files' presence, `config.toml`'s values — come from `internal/config.Report`,
  which reads no file of its own: the caller passes the existence probe.
- A multichoice ask is one `POST` to the provider's decision surface (a `Slot`'s `Decision`; `decisionsFlavor`,
  SPEC §9.1), one `noul` or `choice` question asked and the answer read by the key it echoes — no
  reply text parsed. Two shapes stand behind that choice. `systemone`, what a provider silent on the
  flavour gets: `model`, `state`, and `questions` a JSON object keyed by the question key, each
  question's `criteria` carrying the per-letter meanings the templates' bullet lists used to carry;
  the `answers` object comes back keyed the same way. `decisions`: the state as `input`, `questions`
  an array of named questions — a `noul` question asked as a `predicate`, carrying no choices, its
  true/false guidance in `instructions`; a `choice` question carrying its criteria as `choices` —
  answered by an `answers` array, each entry naming the question it answers; a `refusal` carries no
  value and so takes the seam's re-ask-then-default path. Under either surface an answer naming a
  question kbase did not ask is refused. The request is re-issued identically up to the chat attempt
  count on a transport error or retry-class 5xx, with no streaming and no incomplete-stream path.
  Both surfaces are OpenAI-shaped, so the registry gives the `decision` role an `OpenAIClient`
  whatever its provider's `api`: a provider whose `api` is `anthropic` keeps its Messages client for chat
  and gains an OpenAI one, on the same base URL and key, for decisions. A Messages-only server has no
  decision surface, and the call fails with that server's status. The system-prompt sentence above
  is scoped to the overview call; a decision call has no system message.
- Each inference pass is built as plumbing reaching the model only through `internal/asks`' multichoice-ask
  seam, injected — nothing in `claimgraph` knows a network exists. `internal/asks` holds the embedded
  templates and fragments, the composer, the caller (system fragment, per-call captures — a chat
  capture's request record carrying the effective settings, `temperature` and `thinking`, the
  values `model.SamplingOf` reports; a decision capture's question carrying its `true-letter` where it has one, and the overview
  capture's request its `excerpts` and, on a retry, its `rejected-lines` — the inputs
  `asks.OverviewPrompt` renders — and the answer cache under the state store's `scratch/`, each written once, after the item that made the
  call has released its token, the answers into the current build trail's directory), and the multichoice-ask seam: every claim-graph
  inference is one decision answered by one letter from a closed set the build offers, an unreadable
  reply asked once more and then defaulted to the item's mechanical draft, recorded as defaulted.
  Each kind composes its state through `MultichoiceItem.Compose`, its kind's template rendered from
  the item's template inputs, and `asks.RecordedItems` builds items from a group record through the
  same constructor. A batch that stops still returns its records — an item whose call never
  completed recorded `failed`, one cancelled or never launched absent — as, after a stop, is a
  group with no asked item; a finished batch records every group it was given — and the stage writes them
  before it stops; a batch made by `asks.NewReplayBatch` records such an item and carries on.
  Templates (`*.tmpl.md`) and fragments are embedded files (I7). A template change is an experiment
  with a measured result.
- Prompts are `@!slot!@` templates; a call's system prompt is the fragment named for it
  (`overview-system` — the one surviving system fragment, the overview call's), never an agent
  definition; a multichoice ask carries no system message. `asks.Render` builds the one user turn. No test
  asserts a template's prose; tests key on the offered letters, the slot set, the system fragment and
  the closing line's letters.
- `claimgraph` reaches a model only through the multichoice-ask seam injected by `build`; the caller that
  holds `model` lives in `internal/asks/call`, imported by `build` alone, and the import-policy test
  checks the transitive graph so `claimgraph` never reaches `model`.
- No prompt originates in an in-code string.
- The overview passage is written from excerpts composed mechanically from the tree, checked for
  structure and re-asked once through the
  `overview-correction` fragment; a call ending `truncated-empty` fails the stage without that re-ask.

---

