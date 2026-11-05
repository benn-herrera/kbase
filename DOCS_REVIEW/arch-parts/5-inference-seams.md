## 7. Inference seams

**Pattern:** Role-based model routing behind provider adapters.

`internal/config` loads the providers and the role choices, `internal/model` contains the wire clients
and the call queue, and `internal/modelclient` is the only constructor of a `model.Client`. Three roles
route calls: `decision` for the multichoice questions, `text` for the overview call and `scoring` for
`kbase score`'s calls (SPEC §9 The files are the one source of inference). Each role names its own
provider and model in `[models]` as a `<provider>/<model>` reference. No role falls back to another;
no reason is recorded for that.

Every model call is made through a `model.Slot`. `model.Queue` has N slots; `Queue.Go` waits for a free
one and starts the item with it (§2, I12). `Slot.Chat` makes the overview call and
each scoring call, and `Slot.Decision` makes each multichoice question. The `Client` interface's
inference methods are unexported, so the compiler restricts every other package to a slot, and a slot is
spent when its item returns. `build`, `score` and both replays build the queue from `--ask-concurrency`,
else `[asks] concurrency`, else `config.DefaultAskConcurrency` (§12). `askConcurrency`
in `cmd/askconcurrency.go` is the only resolver and refuses a flag under 1. The same N is the idle
connections kept per host: `modelclient.New` takes it and sets `model.Endpoint.IdleConns`, so parallel
calls reuse connections. A build shares one queue across its roles; `probe` runs a queue of one.
`walk.launch` checks that a provider resolves before the first stage when a stage left to run makes
model calls (SPEC §5 Inference preflight).

### Probe

**Pattern:** Health-check / smoke call.

`cmd/probe.go` makes one call on the path a run uses and reports it (SPEC §8 `probe`). It is a
diagnostic, the CLI command and the read-only MCP tool alike (§2, I9).

- **Selection.** `config.ResolveFlag` resolves one selection. `--model` supplies a reference and names
  itself in a refusal. A role flag (`--decision`, `--text`, `--scoring`) names the role and supplies no
  model, so the role's model comes from the configuration and a refusal names its key alone.
- **Client and call.** The client comes from the registry, as `score`'s does. A `--decision` selection
  makes one `Slot.Decision` call on the API its provider's `decisionsFlavor` names; every other
  selection makes one `Slot.Chat` call. `--prompt` replaces the default check of either path,
  `probeDecisionPrompt` or `probePrompt`. A chat request to a decision selection is the mismatch a probe
  exists to find.
- **One call.** The call has no backoff, so a repeat is immediate. `--timeout` bounds it, and the
  deadline and the `wall-ms` clock start inside the item, so neither counts a wait for a slot. The
  chat path reports `ChatAttempt.FirstToken`, measured in `chatOnce`: request start to the first
  non-empty content or reasoning delta, zero where none arrived. The decision API streams nothing, so
  its document has `first-token-ms` zero and `finish-reason` null.
- **Sampling.** A chat selection's client is the registry's for its role, so the call has that role's
  sampling (the `scoring` role's for `--model`), and the document reports `model.SamplingOf(client)`.
  Where the call ends `truncated-empty`, the chat path reports `ChatResult.Final` as the answer
  instead of failing.
- **Isolation.** `probe` opens no `kb.Source`, resolves no state directory and takes no lock.

### Wire clients

**Pattern:** Provider adapters behind one interface; connection pooling.

`internal/model` contains two implementations of the chat request. `model.OpenAIClient`
(`openai_client.go`) speaks `chat/completions`. `model.AnthropicClient` (`anthropic_client.go`) speaks
the Messages API: `POST <baseUrl>/messages`, the `x-api-key` and `anthropic-version` headers, its own
event sequence and its own catalogue shape. A provider silent on `api` gets the Messages client
(SPEC §9.1 The chat protocol). `model.NewClient` dispatches on `model.Endpoint.Protocol`, a value
`modelclient` resolves from `config.Endpoint.Protocol` (`chatProtocol`); `Endpoint.Decisions` is
resolved from `decisionsFlavor` the same way (`decisionsSurface`). `internal/model` reads no
configuration string, imports no kbase package but `secret`, and touches no file (`TestModelImportsNoOS`).
The body each client sends is its protocol's, never shaped by host or provider name.

Each client's HTTP client has no total timeout, which would cap a streamed generation. The
bounds are the call's context deadline, `responseHeaderTimeout` for the handshake and
`streamIdleTimeout` for the gap between reads (§12). A non-2xx answer is a
`model.StatusError` from both, its body truncated at `errorBodyLimit` and scrubbed of any key the server
echoed (`statusError`).

**Pattern:** Vendor parameter mapping.

`model.Sampling` is the sampling type for both protocols. Each client resolves `Endpoint.Sampling` once, at
construction, with `Sampling.Resolved()`: `DefaultTemperature` and thinking off where the role states
none. `chat()` copies it onto every request through `Client.sampling()` and shapes no field of either
protocol; each protocol's wire rules live only in its body encoder (`encodeRequest`,
`encodeAnthropicRequest`). `model.SamplingOf` returns a copy, so no caller can alter what a client
sends. The fields each protocol sends are SPEC §9.1 The chat protocol. The Messages encoder sets the
required `max_tokens` to the thinking budget (`thinkingBudgetLow`, `thinkingBudgetMedium`,
`thinkingBudgetHigh`) plus an answer allowance: the larger of `maxTokensFloor` and a quarter of the
request's characters, rounded up. The quarter has no recorded reason (§12).

**Pattern:** Provider prompt-cache affinity.

`modelclient.New` takes a prompt-cache key. A scoring run and a scoring replay pass their id, and the
registry sets it on every endpoint it builds. The OpenAI client sends it as `prompt_cache_key` on every
request, so one run's calls reach one upstream and share a cache entry. A verb that names no run passes
none, and the field is omitted rather than sent empty. The Messages client has no such field and
sends none.

### Retry and attempt outcomes

**Pattern:** Retry with backoff on retryable status.

`chat` repeats a request identically while it does not complete, up to `model.ChatAttempts`. The
caller's `ChatRequest.Backoff` gives the pause before each repeat, its last entry repeating. The overview
call (`overviewBackoff`) and `kbase score`'s calls (`scoreBackoff`) pass a backoff; the build's
multichoice questions and `probe` pass none and repeat at once. The build's multichoice questions have
no pause and no reason is recorded for that. A pause the call's deadline cannot cover ends the call at
once, and its reason names the attempts it made (`gaveUp`). Values are in §12.

`retryableStatus`, a 429 or a 5xx, is the only non-2xx status repeated; any other ends the call on the
attempt that met it. So does an answer that reaches the completion cap before any answer text:
repeating the request identically reaches the cap the same way. `Slot.Decision` has its own loop with the same
count and the same status rule, no pause, no streaming and no `ErrChatIncomplete`.

Each attempt ends in one outcome constant: `ChatOK` (`ok`), `ChatServedNothing`,
`ChatTruncatedEmpty`, `ChatProviderRefused` (a non-2xx status) or `ChatTransportFailure` (no answer
from the provider). An attempt records whether the call reasoned, from a reasoning delta on the stream
or a usage block reporting reasoning tokens, so a call that thought without streaming the thought
counts. The error a failed call returns wraps `ErrChatIncomplete`, has the last attempt's outcome
and text, and wraps that attempt's `StatusError` where it met one. That text is what a failed item's
reason records (SPEC §8 A provider call's failure names what ended it).

### Refused keys

**Pattern:** Typed error classification at the boundary.

`StatusError.RefusedKey` (401 or 403) is the single predicate. Two sites match it: `asks/call`'s
`keyRemedy` and `modelclient.KeyRefused`, which `Registry.Catalogue`, `cmd/score.go`'s call and
`cmd/probe.go` apply. Each attaches `config.AuthAddRemedy` with the name of the provider the call went
to, and the result travels as data:

- `asks.IncompleteError.Remedy`, which `claimgraph`'s inference stop and the build's overview failure
  put on their items;
- `score.RemedyError`, whose remedy and `config.RefusedKeyNote` the attempt's ledger line records;
- `modelclient.KeyRefusedError`, which `cmd`'s `failed` puts on the failure item of `probe`, `models`
  and `configure-models`.

`asks/call` is told the provider it calls by `call.New`, from `build.Provider`'s per-role provider
names. No site recovers a status from a reason's text, and nothing checks a key before a call
(SPEC §9 A refused key).

### Layered configuration

**Pattern:** Layered configuration.

`config.Layers` resolves the directories and `config.Load` reads and merges the two files of each,
once, before a verb's own work. It returns one `Loaded`: the pool, its faults and the choices. The
layers are `<repository>/.kbase/` over `~/.config/kbase`, or the one directory `--config-dir` or
`$KBASE_CONFIG_DIR` names (`explicit` and `environment` in the source vocabulary), so an explicit choice
isolates a run from a repository's own `.kbase/`, which every test and pinned deployment needs. The
caller resolves the repository root with `kb.RepositoryRoot(workDir)`, which answers `""` outside a
worktree, so `internal/config` takes no git dependency and reads the global layer alone there.

- **Merge.** `takeProviders` replaces a provider's table whole by a higher layer's of the same name, so a
  repository entry cannot inherit the global entry's `apiKeyFile` for an endpoint it chose;
  `takeConfig` overlays `config.toml` one field at a time (SPEC §9 Merging). `parseProviders` folds each
  table name to lowercase as it reads it, after refusing a file with two names that differ only by case
  (SPEC §9.1 `providers.toml` — the endpoint pool). `config.ParseReference` and `config.Select` fold the
  names they are given the same way.
- **Faults.** `config.Load` drops a fault naming a provider a higher layer already supplies: the pool's
  provider is the higher one, so the fault describes a value nothing reads. A higher layer's own fault
  is kept, since its provider never entered the pool. A layer that fails the strict load refuses the
  whole load and no lower layer is read (SPEC §9.3 Strict-load contract; `rejectUnknownKeys`).
- **Provenance.** `Provider.Layer` and `Config.ModelLayer` record the layer each provider and each
  role's value came from. `ProviderFault.Path` names the `providers.toml` a fault came from, and a
  `Refusal`'s path names the file the lacking setting belongs in, so a layered configuration says which
  file to edit. `config.Layer` has a directory, its source (`explicit`, `environment`, `repository`,
  `global`) and, once `config.Report` has probed it, whether the directory and each file exist
  (`ConfigExists` beside `ProvidersExists`).
- **Write target.** `config.WriteTarget` is the directory `--config-dir` or `$KBASE_CONFIG_DIR` names,
  else the global one. It is where the configure commands write with no location flag
  (`configureLocation` in `cmd/provider.go`), so a refusal of a name the pool lacks, whose remedy is the
  `configure-providers` call that adds it, names the file that call writes. `configure-models` checks
  against the stack it writes into, `config.WrittenStack`, loaded by `loadWrittenStack`, so its refusal
  names the written layer's `providers.toml` and its remedy has the location flag the call had.
  `configure-providers --remove` loads the same stack to name what still refers to the provider. Both
  commands resolve the layers (`resolveVerbLayers`) without loading them, so no layer outside the stack
  written is read (SPEC §9 The configure commands' write location).
- **Writes.** `UpdateConfig`, `SetProvider` and `RemoveProvider` edit the existing file line by line.
  `verifyEdit` decodes the proposed file and requires it to equal the decoded original plus the
  intended change, else the write is refused. `installConfig` then calls `atomicfile.Write`: a temp file,
  synced, renamed into place (SPEC §9.5 The configure commands' targeted update (byte-preservation
  contract)).
- **Report.** `config.Report` produces the facts `show-config` reports: the pool's providers and faults,
  each provider's key kind and upgrade, the diagnostics, the files' presence and `config.toml`'s values.
  It reads no file of its own; the caller passes the existence probe. `cmd/show-config.go` opens no
  client (SPEC §9.6 `kbase show-config`).

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

### The decision API

**Pattern:** Constrained classification endpoint.

A multichoice question is one `POST` to the provider's decision API (SPEC §9.1 The decision API): a
`Slot.Decision` call with one `noul` or `choice` question, whose answer is read by the key it echoes, so
no reply text is parsed. `decisionsFlavor` selects between two request shapes, and a provider silent on
it gets `systemone`.

- `systemone`: `model`, `state`, and `questions` as a JSON object keyed by the question key, each
  question's `criteria` having the per-letter meanings. The `answers` object returns keyed the same way.
- `decisions`: the state as `input`, and `questions` as an array of named questions. A `noul` question
  is sent as a `predicate`, which has no choices, so its true and false guidance goes into
  `instructions` (`predicateInstructions`); a `choice` question's criteria are `choices`, sorted by
  value. The `answers` array names the question each entry answers.

Both decoders map into `model.DecisionAnswer`, so a caller cannot tell the shapes apart. Under either,
`validateDecisionAnswers` refuses an answer naming a question kbase did not send. A `refusal`, a missing
answer and an answer with no value decode to an answer with no confidence, so `asks.Decide` finds no
letter and the question takes the multichoice-ask interface's retry-once-then-default path
([next](#the-multichoice-ask-interface)).

Both shapes are OpenAI-shaped, so the registry gives the `decision` role an `OpenAIClient` whatever its
provider's `api`. A provider whose `api` is `anthropic` keeps its Messages client for chat and gains an
OpenAI client, on the same base URL and key, for decisions. A Messages-only server has no decision API
and the call fails with that server's status. `AnthropicClient.decision` refuses, and nothing in kbase
reaches it. A decision call has no system message.

### The multichoice-ask interface

**Pattern:** Dependency injection with an architecture fitness test.

Each inference pass reaches the model only through `internal/asks`' multichoice-ask interface,
injected by `build`; nothing in `claimgraph` knows a network exists. `cmd/importpolicy_test.go`
enforces it two ways: `claimgraph` may not reach `internal/model` or `internal/asks/call` through any
chain of imports, and only `build` imports `internal/asks/call` directly.

- **`internal/asks`** contains the embedded templates and fragments, the composer and `Batch`. Every
  claim-graph model call is one question answered by one letter from a fixed set the build offers.
  `asks.Decide` takes the offered letter with the highest confidence, the first offered winning a tie,
  or else a one-character reply that is an offered letter. A reply with no letter is retried once
  with the identical question, the attempt ordinal keeping the two apart in the answer cache, then the
  item takes its deterministic draft and is recorded `defaulted`. `AskGroup` launches the first item
  of a group of more than one alone and waits for it, so the server prefills and caches the group's
  shared prefix once, then launches the rest.
- **Compose.** Each kind composes its state through `MultichoiceItem.Compose`, its kind's template
  rendered from the item's template inputs. `asks.RecordedItems` builds items from a group record
  through the same constructor.
- **`internal/asks/call`** contains the caller, imported by `build` alone: `Caller.Launch` puts each
  question to the decision role's model through the item's slot, and `PassageLaunch` makes the overview
  call through `Slot.Chat`. It writes the per-call captures and the answer cache (see
  [the answer cache](#the-answer-cache)). A chat capture's request record has the effective
  `temperature` and `thinking`, the values `model.SamplingOf` reports. A decision capture's question
  has its `true-letter` where it has one. The overview capture's request has its `excerpts` and, on a
  retry, its `rejected-lines`, the inputs `asks.OverviewPrompt` renders. Each file is written once,
  after the item that made the call has released its slot (§2, I10 and I11).
- **Batch records.** `Batch.Wait` returns a record for each group. A batch that stops still returns
  its records: an item whose call never completed is recorded `failed`, one cancelled or never launched
  is absent, and so is a group with no item that made a call. The stage writes the records before it stops. A
  finished batch records every group it was given. A batch made by `asks.NewReplayBatch` records a
  failed item and continues.
- **Templates.** Templates (`*.tmpl.md`) and fragments are embedded files (§2, I7). A
  prompt is a `@!slot!@` template, and a call's system prompt is the fragment named for it:
  `overview-system`, the one system fragment, is the overview call's. A multichoice question has no
  system message. `asks.Render` builds the one user turn. Tests key on the offered letters, the slot
  set, the system fragment and the closing line's letters, and none asserts a template's prose. A
  template change is an experiment with a measured result.
- **Overview.** The overview text is written from excerpts composed deterministically from the tree,
  checked for structure and retried once through the `overview-correction` fragment. A call ending
  `truncated-empty` fails the stage without that retry.

### Credentials

**Pattern:** Local secrets file with a redacting secret type.

**Store.** `internal/auth` keeps one file per provider, `<provider>-key.enc` (`auth.FileSuffix`), in
`auth.DirName` under `statedir.Base`. The directory sits beside the per-KB state directories, whose
16-hex-digit names it cannot collide with. The name is lowercased, and `auth.CheckName` runs before any
path is built from it. `auth` imports no `config`, so `auth.NamePattern` restates
`config.ProviderNamePattern`, and `cmd/auth_test.go` pins the two equal. A key is sealed with
AES-256-GCM under a per-user key that HKDF derives from a constant in the source and the OS username
(`Store.sealer`). The constant is public on purpose: a key chosen at build time would leave each
release unable to read the last one's store, and disguising a shipped constant hides nothing. The
sealing is obfuscation, not a boundary: any process of the same user recovers the key, and separation
between users is the file's mode. `auth.FileMode` applies on creation and on every replace
(`atomicfile.WriteMode`), and `auth.DirMode` on every `Add`. The provider's name is not bound into the
file, so a file renamed by hand opens under its new name. kbase uses no OS keychain, and no reason is
recorded for that.

**Secret type.** `secret.Key` is the type every key travels in. fmt's verbs, a text, JSON or YAML
encoding and a log value all render `<redacted>`, and `Reveal` alone returns the text.
`cmd/revealpolicy_test.go` allows `Reveal` in four production files: `internal/auth/auth.go`, the two
wire clients, which set the auth header and scrub an echoed key, and `internal/model/transport.go`,
which truncates a status error's body around a key.

**Read path.** `cmd`'s `loadConfigContext` reads every file of the store through `internal/auth`
(`readStoredKeys`) before the load and gives `config.Load` a `config.StoredKeys`: each key that
opened, by the lowercase name its file is stored under, and a `config.StoreFault` per file that did
not, built from the `*auth.FileError`'s path and error apart. A store directory that exists and does
not list is one nameless `StoreFault` against the directory, with no keys, and the load goes on.
`config` imports no `auth`.

**Precedence.** `keyKind` is the single source of truth for which key an entry uses: the key stored
under the provider's name, whatever layer supplied its table; else the table's `apiKeyFile`, which
`LoadProviders` then reads, and which a provider with a stored key never reads, so it raises neither a
read fault nor a permission warning; else none (SPEC §9 The files are the one source of inference).
`keyFileExposure` warns on a key file readable beyond its owner, except on Windows (SPEC §9.4 Key-file
permission warning).

**Faults.** `config.Load` turns each store fault whose name a configured provider has into a warning
`ProviderFault` naming that provider, with `Source` `SourceStoreFile`, `Path` the store file and
`DoesNotOpen` the store fault's. A file no provider names is read by nothing and not reported. The
nameless directory fault names no provider and becomes a warning with `Source` `SourceStoreDir`,
`Path` the directory, which `warnFaults` writes as the credentials store's. A `ProviderFault`'s `Reason`
never repeats its `Path`; `ProviderFault.LocatedReason` leads a store fault's reason with its path for a
line that is complete on its own (`warnFaults`' and a provider's `fault` in `show-config`). `config.Report` reports
a store fault as its path and reason, with `config.AuthAddRemedy` as the remedy where the file does not
open and for a provider dropped because its `apiKeyFile` could not be read (`ProviderFault.KeyFileUnread`),
the stored key outranking the file. It alone computes a `file` key's `upgrade` and the warning that a
stored key leaves an `apiKeyFile` unread, so `show-config` alone emits them.

**Input and transport.** `kbase auth add` reads the key from stdin only, at most `authAddInputLimit`
bytes (§12); it reads one byte past the limit to tell more from exactly. A configured key
sent to an `http://` base URL on a host other than loopback travels in cleartext. `build`, `score` and
`probe` write one stderr line saying so through `warnCleartextKey` (a build writes one per endpoint),
which names the host and never the key, and proceed. No switch turns the warning off.

---

## 8. Ledger, state store and monitoring

**Pattern:** Checkpoint commits, a content-addressed answer cache, and a background job with a status file.

`internal/gitledger` is the only package that execs git (§2, I1). It execs the host's
git, which SPEC §2 Given interfaces requires, so the user's configuration, hooks and attributes apply,
and no git library is a dependency. `internal/build` owns the stage pipeline, the state store's writers,
resume, `--no-inference`, `--through`, progress and cancel. `internal/buildstore` owns the stage
vocabulary, the store's reading, status and the reading of checkpoint commits. Which packages and `cmd`
files import `gitledger` is in [§11](#dependency-rules).

### Checkpoint commits and resume

**Pattern:** Checkpoint/restart pipeline; stage = idempotent unit.

Whether a build is fresh or a resume is read from the checkpoint commits alone; nothing in the state
store is authoritative for position (SPEC §6 Git holds the recoverable record). The checkpoint commits
are `gitledger.Repo.Trail`: every boundary commit reachable from `HEAD` above `Repo.Removal`, the newest
non-boundary commit that took `kb-root/` out of the tree, so a removal ends the commits below it.
`buildstore.CurrentTrail` takes the newest build's part, back to its newest `start` boundary, so a fresh
build over a removed `kb-root/` starts a set of its own. The four tracked build records
(`kb-build-*.yaml`: node pass, unmarked, classification and cull) are read and written by `claimgraph`,
and `build` commits them.

- **Inputs.** Once it has found the repository, `build.Run` makes a fresh build's top `.tex` files and
  `--bibliography` files absolute against the repository root and sorts them by path (`inRepository`)
  before anything reads them: the launch checks, the document graph and the regeneration of records.
  Every boundary records them, and `start` also records `started-at`, the `HEAD` before it; on a branch
  with no commit it first makes an empty `kb-start:` commit. The line keys and their readers are
  `internal/buildstore`'s (`RecordedInputs`, `StartedAt`). `--resume` takes the inputs from the newest
  boundary in the order recorded, never re-sorted (`takeInputs`; SPEC §5 Inputs).
- **Document graph tree.** `docgraph.Build` writes it, then every leaf is rewritten with each link
  pandoc wraps around a cross-reference unwrapped to that cross-reference (`build.unwrapTree`), in the
  build and in the regeneration of records alike.
- **Interrupted stage.** A stage writes its build records and its `kb-root/` work once its model calls
  have all returned, so a stop during the calls leaves those paths as the boundary left them, and a kill
  during the write leaves them partial; either way they are uncommitted. A resume restores those paths
  to the last stage commit (`Repo.Restore`) and re-runs the stage whole. The answer cache supplies the
  answers already obtained, so the re-run makes no call it already has an answer for. The re-run mints fresh ids, and nothing outside the restored paths referenced
  the old ones. A classify prompt names claim ids, but those were minted in stages already committed
  and the restore reaches back only to the last boundary, so a resumed run's prompts are identical and
  the cache answers them; a fresh build's are not and it makes the calls again. Resume reads no build
  record for position. The progress record accounts for the dirt an interrupted stage leaves
  (`uncommittedIn`); the dirty-path refusal (SPEC §5 Starting over) applies to dirt it cannot account for.
- **Resume command.** The resume command is recorded as the argv `kbase build --resume` with every flag
  the launch had but `--through` and the inputs; `--config-dir` is written absolute. `walk.result`
  emits it only where an identical repeat continues the work: a `bounded` run gets none, a refusal that
  recorded no boundary leaves nothing to continue, and none is emitted where `kb-root/` does not exist
  when the run ends. A document renders it in the form each interface takes (see
  [per-interface rendering](#per-interface-rendering)). It is also emitted only where the recorded argv
  parses as a `kbase build` invocation (`buildstore.RecordedCommand`, SPEC §6 `kbase status`): the state
  store is a directory the caller names, so a record naming any other program is repository-supplied
  text, and no document renders it for a harness to run.
- **Progress units.** Units are leaves for the node pass, source groups for `references-found`, the
  multichoice question groups of each of `claims-culled`'s two passes (each announcing its own total) and
  question groups for classification. A defaulted item is a `fallback` event. A stage's `units` event
  is written before its model calls and its unit and `fallback` events in one append after them, so
  `status` changes between concurrent call batches. Every build record is written once per stage, after
  its calls.
- **`--no-inference`.** The overview call is the dropped step and no `README.md` is written
  (SPEC §5 Determinism; §4).

### Preconditions

**Pattern:** Precondition checks.

`run` refuses a `--resume` with nothing to resume as soon as it has the checkpoint commits, `kb-root/`
absent among the causes (`takeInputs`, SPEC §5 Starting over). `prepare` refuses a fresh build over any
`kb-root/` that exists with `buildstore.KBRootExists`, the refusal `init` gives, and drops the paths
under the absent `kb-root/` from the fresh build's dirt check, since its `start` boundary commits the
removal. `buildstore.TrailRefusal` refuses commits that record a stage past one they leave unrecorded,
in `Stages` order (`check: trail`), right after reading which stages are recorded and before the
launch checks and any restore: walking the gap would run a stage over the products of stages meant to
follow it. A `.git/index.lock` left by a kill during a boundary commit is refused by name
(`Repo.IndexLock`).

### Regenerating missing records

**Pattern:** Reproducible-build verification.

The document graph's records live in the state store. A resume that finds them absent re-runs the
document-graph stage into scratch (`walk.ensureRecords`) over the recorded inputs and requires its tree
byte-identical to the committed `kb-root/` (`firstDifference`); a difference is `failed`, naming the
first differing path (SPEC §5 Determinism).

### The answer cache

**Pattern:** Content-addressed memoization cache.

`internal/asks/call` keeps one answer per call under
`scratch/trails/<start-commit>/answers/`, written through `atomicfile`. A chat answer is keyed by the
digest of the model, the system prompt, the prompt, the attempt ordinal and the client's
sampling, which contributes key parts only where it is not kbase's default (`samplingKey`). A decision
answer is keyed by the model, the question whole, the state and the attempt ordinal. The attempt
ordinal is in the key so a retry reaches the model instead of finding the first answer.
`AskTotals.WarmCacheReadTokens` and `CallStats.CachedTokens` read 0 on every decision call: neither
decision API's usage block has a cached-token field, so the zero is the API's.

### State store layout and format

**Pattern:** Versioned on-disk schema with sequential migrations.

The state store's contents and path are SPEC §6 The state store is versioned and SPEC §6 Git holds the
recoverable record. `--state-dir` defaults to `statedir.Dir`: `statedir.Key` under `statedir.Base`
(§12). The layout:

- `format`, and `lock`, the run lock's second file, with its holder record ([Locks](#locks));
- `progress.jsonl`, and the rotated `progress-<UTC timestamp>.jsonl` files;
- `records/`: the document graph's records and the pre-pass count report;
- `reports/`: `<stage-id>.yaml`, each stage's records and findings, named by the result's
  `stages[].report`, and the captures of a build or score run the MCP server starts
  (`build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`);
- `scratch/trails/<start-commit>/`, one directory per build, holding `answers/`; `asks/`, one file per
  stage, `<kind>.yaml`, one YAML document per group (the group's shared template inputs once, each
  item's own inputs, its `prompt-sha256` from `asks.AskDigest`, its outcome and its calls), written by
  a stage whose batch stopped as well as by one that finished; `captures/`, every call's request, reply
  and usage; and `replays/<replay-id>/`;
- `scratch/regenerate/`, where a resume rebuilds the records;
- `score/<run>/`, a scoring run's directory ([§6](#the-ledger-and-the-state-directory)).

`<state-dir>/format` is one integer, `buildstore.StoreFormat`, written when a build first takes the
run lock (`readyStore`); an absent file is format 1. `build`, `status`, `wait`, `cancel`, `build-trail`
and the MCP `build` tool read it at open (`MonitorOptions.Open`, `walk.open`), and a store newer than
`StoreFormat` is refused (`buildstore.StoreVersion`; `check: state-dir`, `key` the marked `state-dir`
flag, `path` the store, SPEC §7 Item). A build converts an older store in place, in `readyStore`,
through `storeUpgrades` in `internal/build`: one step per version, each writing `format` when it
completes, so an interrupted upgrade resumes at the step it stopped at, and a gap in the chain is an
error. The chain is independent of the KB's `formats.Chain` ([§12](#format-migration-spec-10)). At
format 1 `storeUpgrades` has no steps.

### State store retention

**Pattern:** Keep-last-N retention.

Only a build or a scoring run prunes; `status` and `cancel` remove nothing. Counts are in
§12.

- **Captured builds.** After the run event, a build keeps the newest `keptBuilds`+1 captured builds
  under `reports/` and removes the rest (`pruneBuildCaptures`, `buildstore.PruneCaptures`). A capture is
  the `.stdout.yaml` and `.stderr.log` pair sharing a stem, ordered by name. It reads no progress record
  to do so.
- **Build directories.** Once its launch checks pass and before its first stage, a build prunes
  `scratch/trails/` (`pruneTrails`). It keeps the current build's directory, named by the commit of the
  newest `start` boundary, and the newest `keptBuilds` others by modification time. On a resume it
  creates the current directory if absent and sets its modification time to now; a fresh build's directory is created by its first model call, so its time is when it
  last gained a subdirectory. The rest are removed whole, with the undivided `scratch/answers/`,
  `scratch/asks/` and `scratch/captures/` directories. `status` computes `cache-entries` and
  `cache-bytes` over every build directory's `answers/`.
- **Progress.** At the same point, a fresh build whose `progress.jsonl` has more than
  `progressRotateLines` lines relocates every line before its own run event to `progress-<UTC timestamp>.jsonl`,
  leaves its run event alone in the record, and keeps the newest `keptProgressFiles` rotated files
  (`rotateProgress`). Nothing reads them. Stage reports are never pruned.
- **Scoring runs.** A scoring run prunes `score/` once its ledger has opened: of its kind (`run-` or
  `dry-`) it keeps the run it names and the newest `KeptRuns`-1 others by `run.yaml`'s start, and removes
  the rest whole. It keeps the newest `KeptRuns` captured runs under `reports/score-*`, by name
  (`buildstore.PruneByStart`; [§6](#run-retention)).

### Locks

**Pattern:** Repo-scoped mutex lockfile plus a pidfile-style holder record.

Two locks exclude concurrent writers of a KB: the run lock, held for the length of a build or a
scoring run, and the KB write lock, held for the length of a write. Both are `internal/filelock`
advisory locks. `filelock.Acquire` takes an exclusive `flock` on an existing directory or on a lock
file it creates, waiting up to a given time. The descriptor holds the lock, so the operating system
releases it when the holder exits however it exits. On a platform without `flock` (`filelock_other.go`)
nothing is locked and nothing is reported held, so `status` cannot see a running run (SPEC §6 Git holds the
recoverable record).

**The run lock.** The run lock is two lock files, taken in a fixed order (SPEC §6 Git holds the
recoverable record; SPEC §8 The run's locks):

1. `<git dir>/kbase-operation.lock` (`buildstore.OperationLockPath`, found by reading `.git` and
   following a `gitdir:` file, since a maintenance verb that checks it never requires git). The lock is
   per repository, so a second run through any `--state-dir` is refused. The file contains the state
   directory the run named. An empty file is the window between acquiring the lock and writing the state
   directory: a second run gets `retry` and a writer proceeds.
2. `<state-dir>/lock` (`buildstore.HolderPath`). Its holder record (`buildstore.Holder`) is JSON with the
   holder's pid, its start, the verb that owns the run (`Kind`, `build` or `score`) and, for a scoring
   run, the run id. `status` reads liveness from the lock and the run from the record, `wait` reads
   whether the run it waits for still holds the lock, and `cancel` signals the pid.

The liveness test is the lock, not the pid: `filelock.Held` takes and drops a shared lock, which an
exclusive holder refuses. `build.Hold` takes both files in the order above, and its release function
removes `<state-dir>/lock` first (`ReleaseRemoving`), so a clean exit, `cancel` included, leaves neither
file and a standing file is not by itself evidence of a live holder. Each wait is `buildstore.LockWait`
(§12). The record is written after the lock is taken, so a state directory named by the
first file whose record is not yet written is reported "a run is starting". `takeOperationLock` and
`runningRefusal` give the refusals (`check: lock`, `path` the state directory), naming the verb and pid
read from the store the first file names. A second run on the same store through a different repository,
where only `<state-dir>/lock` is held, is `retry` (`takeLock`). `buildstore.ReadHolder` refuses a record
naming no known `Kind` (`check: state-dir`, `path` the record): it is a store another version wrote and
is never read as a build's. A scoring run takes the lock through the same `build.Hold`, in
`cmd/score.go`, before it writes its first record or does its start rebuild.

A build's order is: refuse a store newer than this kbase at open; `<git dir>/kbase-operation.lock`;
`<state-dir>/lock`; ready the store (upgrade and write `format`); the run event; prune the captured
builds; wait out writers. The wait (`awaitWriters`) takes and releases the KB write lock, or ends `retry`
if it cannot, so `status` sees a waiting build. A build's own writes, its rebuilds of the derived views
included, take no KB write lock, so it cannot refuse itself.

Given no `--state-dir`, the monitors (`status`, `cancel`, `wait`, `build-trail`) resolve their store
through `buildstore.MonitorOptions.Open`, which reads the store `<git dir>/kbase-operation.lock` names
while that file exists and names one, else the default store. The MCP launch sets `DefaultStore`, so a
new run never reuses the named store.

**Pattern:** Advisory file lock with timeout.

**The KB write lock.** `write.LockKB` takes an exclusive lock on the directory that contains `kb-root/`,
which exists before `kb-root/` does and leaves nothing to clean up, waiting up to `write.LockWait`
(§12); a holder that keeps it past the wait gives `filelock.ErrHeld`, which
`write.HeldLockItem` turns into the `retry` item (SPEC §8 One write lock). `write.Run` takes no lock;
the caller holds `LockKB` across it, except the build, which holds the run lock instead. The callers are
`cmd`'s `holdKB` (write batches, `update-format` and `init`) and `holdWriting`, and
`score.landValues`. `holdKB` probes `build.RunningOperation` for a held run lock before it waits on the
write lock and again after taking it, so a run that started during the wait is refused at once. A scoring
run's own start rebuild (`scoreRefresh`) and `landValues` take the KB write lock and make no probe: they
write as the run that holds the run lock. A scoring run holds the KB write lock around writing its
results and around its start rebuild, never across the run ([§6](#writing-the-results)).

### Status, progress, cancel and wait

**Pattern:** Background job with a status file, signal cancel and a polling wait.

- **`status`.** The verb is `score.Status`. `buildstore.Status` reads `<state-dir>/lock` and its record,
  `progress.jsonl` and the checkpoint commits, and returns the snapshot in SPEC §6 `kbase status`. Where
  the record names `score`, `score.Status` reports the run in its own shape, from the run's ledger and
  result document, and counts the run's result files while it is in its item loop. Where no live run
  holds the run lock and the record names no score run, it compares the start `score.NewestRun` reads from
  the newest run directory's `run.yaml` with the newest build invocation's in `progress.jsonl`
  (`buildstore.ReadState`), and reports the build where neither exists or the two do not order. A
  build's `stages` lists the stages done and not done in the newest build's checkpoint commits and names
  `build-trail` for their commits. Where no build run holds the run lock and `kb-root/` is absent, or the
  commits above a removal are empty, `status` reads no commits (`buildstore.noBuild`).
- **`build-trail`.** `buildstore.BuildTrail` reads the same checkpoint commits. Each boundary's command is
  `git reset --keep` and then the build resumed with the flags its last run was launched with: the resume
  argv the store's newest run recorded, through `RecordedCommand`, which is the `build` call with
  `resume` alone where no run recorded one that parses as a `kbase build` invocation. `started-at`'s is
  the reset alone (SPEC §6 `kbase build-trail`).
- **`cancel`.** `cmd`'s `cancelScore` signals the holder the record names. For a live scoring run it
  calls `build.StopHolder` and takes the resume call from `score.ResumeOf`; otherwise `build.Cancel`
  does the same through `StopHolder` and reads the resume call from the progress record. `StopHolder`
  sends `os.Interrupt` to the pid and polls `<state-dir>/lock` until it is let go, or ends `retry` after
  `cancelWait`. The holder abandons the calls in flight with nothing written for them, logs the event,
  releases the locks and exits `cancelled` (SPEC §6 `kbase cancel`).
- **`wait`.** `wait` polls `buildstore.ReadState` through `buildstore.WaitState` every `waitPoll`, the
  store alone, until the run it names is superseded or has ended, then renders `status`'s document
  through `buildstore.Status`. It takes no lock and reads no checkpoint commits until that final
  document. A score run's pid is waited for through `score.RunByPID` and the store's record
  (`score.WaitRun`), so `wait` returns the run's own result document, re-emitted by
  `result.Reemit`. Values are in §12.

### The build's development replay

**Pattern:** Offline eval / record-replay regression harness.

`build.OpenReplay` and `Replay.Walk` (`internal/build/replay.go`, tagged `//go:build kbasedev`, as the scoring replay is,
[§6](#the-development-replay)) repeat one build's recorded questions under the
binary's embedded templates and question files, from the inputs the build directory records
(§2, I10), so a prompt, a question file or a model can be refined against one build
without re-running it. The build is the one a hex prefix of its start commit names, or, given none,
the directory with the newest modification time, listed by `listTrails`, the listing `pruneTrails`
orders by.

- **Selection.** `OpenReplay` reads each kind's `asks/<kind>.yaml` (`asks.ReadGroupRecords`) and the
  overview captures (`call.ReadOverviewCaptures`) and selects in this order: the multichoice kinds in
  `asks.Kinds` order, groups in file order and items in record order, then the overview calls by
  attempt. The selection can be narrowed to one kind or `overview`, to the items recorded `failed` or
  `defaulted` and the overview calls `failed` or `rejected`, and to the first N. `OpenReplay` refuses
  (`check: usage`, `key: --replay`) a store with no build directory, a prefix matching none or more than
  one, a build recording no group record and no overview capture, an empty selection, and a selected
  record or capture with no inputs.
- **Calls.** The replay takes no lock. `asks.RecordedItems` rebuilds the items and `asks.NewReplayBatch`
  puts them to the `decision` role; each overview call is its own item on the `text` role, through
  `call.New` pointed at the replay's directory, every call on the invocation's queue.
  `MultichoiceItem.Compose` and `asks.OverviewPrompt` remain the only composition sites. A call that
  never completes is a `failed` row and the replay continues. Progress is logged as the scoring replay
  logs it, through `buildstore.ReplayProgress`; the last line's `done` and `failed` are `ReplayResult`'s.
- **Output.** It writes only under `scratch/trails/<start-commit>/replays/<replay-id>/`, the id minted
  `replay-<id>`: `replay.yaml` (`replay`, `trail`, `decision_model`, `text_model`, `started`); its own
  `answers/` and `captures/`, in the build directory's shapes; `asks/<kind>.yaml`, its own group
  records; and `comparison.jsonl`, one row per replayed item or overview call with the original and
  replayed `outcome`, `choice` and `confidence` and `prompt_changed` (the re-rendered prompt's digest
  differs from the recorded `prompt-sha256`). It never writes the build directory's `answers/`, `asks/`
  or `captures/`, nor `kb-root/`.
- **Directory handling.** Setup creates `replays/`, the replay's directory and its subdirectories with
  `os.Mkdir`, never `MkdirAll`, so a build directory the prune removed fails setup rather than being
  recreated, and restores the build directory's modification time, which is the current-build marker and
  the prune's order. After writing `replay.yaml`, the replay keeps itself and the newest `KeptRuns`-1
  other replays of its build's `replays/` by `started` (`buildstore.PruneByStart`), which removes only
  entries of `replays/`. Once the item loop drains, a replay whose `replay.yaml` no longer exists,
  because a prune removed it, removes its directory, and its `replays/` and the build directory where
  they contain nothing else, and ends `failed`; otherwise it writes its group records and
  `comparison.jsonl`, on every exit after it made any call.
- **Outcome.** `ReplayResult` counts the selected rows (`total`), those replayed to an outcome other than
  `failed` (`done`), those `failed`, and as `changed` the rows whose outcome or choice differs from the
  original's. The outcome is `done` when every selected row was put to a model, `cancelled` when the
  context stopped the replay, and `failed` for a fault of the replay itself: a store write, a compose
  error or the replay directory removed during the replay.
- **Verb.** `kbase build --replay [<start-commit>]` is in `cmd/build_replay.go`, reached from
  `cmd/build.go` only through the `buildReplay` hook that file's `init` sets. The tag, the hook,
  `mcp.ExcludeFlag` and `test-dist-no-replay` are as for the scoring replay ([§6](#the-verb)). `--replay`,
  `--kind`, `--failed` and `--limit` are excluded from MCP. Beside `--replay` it takes `--state-dir`,
  `--ask-concurrency` and `--config-dir`; any other of build's own flags is refused, naming `--replay`, as
  are `--kind`, `--failed` or `--limit` without it, more than one positional or one not matching
  `^[0-9a-f]+$`, a `--kind` outside the five (`paragraph`, `unmarked`, `colinear`, `classify`,
  `overview`) and a negative `--limit`. Models and sampling come from the build's own `Provider` function
  (`buildModels`, `cmd/build.go`). The result document has verb `build` and `outcome`, `state-dir`,
  `trail` (the start commit), `replay`, `dir` (absolute), `counts` and `changed`, then the final record
  its outcome names.

---

## 9. Result emitter

**Pattern:** Single structured-output writer.

`internal/result` owns the YAML result document and its emitter (§2, I5). `result.Write`
builds the document as a `yaml.Node` mapping from the ordered `Field` list it is given: the key order
is the caller's, every string is double-quoted, and a float is written without an exponent. A refusal is
a list of `Item`s and the emitter writes every item it contains; collecting every offender is each
producer's work, and nothing in `internal/result` checks it. `Emit` puts `outcome` first. The result
tests parse each covered verb's stdout as one document and compare its keys with the documented order
(`documentedKeys`, `cmd/resultkeys_test.go`). Production code names `os.Stdout` once, in `main`, and no
test scans for another stdout writer. SPEC §7 Tool results is the contract.

`result.ExitCode` maps `outcome` to the exit code and `result.IsError` says whether the outcome is an
error: `cmd` exits with the first, and an MCP tool result's `isError` is the second. `Emit` ends the
`detail` of each item that ends a `retry` or `failed` document with the outcome's closing sentence. `RetryAsk` and
`FailedAsk` are the single source of both sentences, and a caller passes `Emit` its items without
them. The YAML library is the one named in CONVENTIONS "Dependency Policy".

### Per-interface rendering

**Pattern:** Per-surface presentation layer.

Text that names a tool argument is written once, in a marked form, and each interface formats it as it
takes the argument. Recorded documents and help text are read through either interface, so they contain
the neutral form and the reader's interface formats it.

- **The marker.** `internal/argmark` is the single marker by which text names a tool argument (`Flag`,
  `Positional`, `FlagValue` with its value) or contains a kbase call (`Call`, with an arguments mapping
  whose values are JSON values, lists of mappings among them; `Recorded`, a recorded argv). A marker is
  delimited by two private-use code points, with its kind in the byte after the opening one. It imports
  no kbase package, so every producer of text an MCP reader can receive reaches it; `kb`, `config`,
  `result`, `statedir` and `log` use it.
- **Formatting.** `Spelling.Spell` formats each marker as one interface takes it. At the command line
  that is `--name`, `<name>` or `--name value`; over MCP, `name` or `name: value`. A call is formatted by
  the `Spelling.Call` its caller supplies, which `mcp.Spelling(root, overMCP)` builds from the command
  tree: over MCP, the call of the tool the command is exposed as (`toolCall`); at the command line, a
  `kbase` command line, with the values document of a command that reads one piped in on stdin. A call
  with no `Spelling.Call` stays marked. A marker inside other text is backticked where its formatting
  has no delimiter of its own (every one over MCP, a call at the command line); a marker that is the
  whole text stays bare. Text outside markers is unchanged.
- **Writers.** `result.Spelled(w, s)` wraps a writer so that every string `result.Write` puts on it is
  formatted by `s`, and so is any other text written on it, such as what cobra prints from a command's
  summary and flag usages. The root's stdout is such a writer for every subcommand but `mcp`, set in
  `PersistentPreRunE`; the MCP server wraps each tool call's output the same way. A writer already
  `Spelled` keeps its formatting, settled where the call came in (`documentOut`, `cliOut`). A document
  written on any other writer keeps the marked form.
- **`result.Verbatim`.** The KB text a result passes through (a title, a rationale, a context, a leaf's
  line, a conversion list's id and text) is a `Verbatim` and is written unchanged on every
  interface. Only text kbase composes is formatted, because KB text may contain the same code points
  and is the author's. `Items.Error` formats for the command line, since an error's text is read on
  stderr and in logs, and `internal/log` formats every line it writes the same way.
- **Recorded documents.** The progress record's items and a score run's result document beside its
  ledger contain the marked form, because each was written on a writer that formats nothing. `status` and
  `cancel` emit them again through `result.Emit` on the writer of the call reading them, so a recorded
  item and the recorded `Resume` argv reach each interface in its format; the argv goes through
  `buildstore.RecordedCommand`'s `argmark.Recorded` call and is read with `build`'s own flags into an
  arguments mapping where it is formatted. `wait` re-emits a score run's document through
  `result.Reemit`. The store's layout is unchanged: a marker sits inside the string fields, and
  `Resume` stays an argv.
- **Detached runs.** A run a tool call launched is told its document is read over MCP by `mcp.SurfaceEnv`
  in its environment, set by `Launch.start`. `cmd/main.go` reads it before `Execute` and wraps the
  process's stdout, so the run's own document, which the launch hands back where the run exits first, is
  the tool's.
- **Checks.** `cmd/argspelling_test.go` enforces the layer: no MCP rendering contains command-line
  formatting, no command-line output contains a marker, every marker names an argument, and a refusal's
  argument and a remedy's call are in the form of the interface that receives them.

---

## 10. Compatibility harness

**Pattern:** Black-box integration tests with golden fixtures.

Compatibility is judged by kbase's own readers and checks, never by a byte diff of presentation
(§2, I4). The hermetic check is `just test-integration`, whose nine members each judge
what their process produced:

- `test-integration-results-fixture`, `test-integration-init`, `test-integration-opscript-results`,
  `test-integration-mcp`, `test-integration-config`, `test-integration-auth`,
  `test-integration-parity-adjagent-sheet` and `test-integration-refused-write-adjagent-sheet` run
  `./bin/kbase` over fixtures they stage. They judge the result document against the verb's keys, the
  tree it wrote, the transcript it served, or the streams and files a credentials command left. The
  parity recipe judges the frozen answers the 2.0.1 binary gave over the KB the conversion reads, each
  moved only by a rule naming a designed outcome of the conversion. Each runs the script of its own name
  under `tools/scripts/` and writes its log under `test-data/transient/<recipe>/`.
- `test-dist-no-replay` judges the default build's command tree for the compiled-out replay.

The recipes that the rule in CONVENTIONS.md "Testing" keeps out of that check run by name;
`test-integration-mcp-build`, the MCP notification probe that builds a paper, is one.

Unit tests use golden-tree conformance fixtures: `internal/formats/testdata/` contains old bytes in and
new bytes out for each converter, with decoded-values goldens for the record files and a README naming
what each pair pins, and `internal/sheet/testdata/` contains the claim-graph sheet fixture and its DOT
goldens. A conversion or decode that differs from its golden is written under
`test-data/transient/unit_tests/formats/got/` with the golden's name, and a golden is replaced only by
copying that file over it.

---
