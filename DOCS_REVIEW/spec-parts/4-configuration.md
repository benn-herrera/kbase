## 9. Configuration

Two TOML files, `providers.toml` (§9.1, the endpoint pool) and `config.toml` (§9.2, the choices), are read under the strict-load contract (§9.3) from one or two layers. `<repository>/.kbase/` is read over `~/.config/kbase`. A directory `--config-dir` names, else `$KBASE_CONFIG_DIR`, is instead the one directory read, so an explicit choice isolates a run from a repository's own `.kbase/`. An invocation outside a worktree has no repository layer. `show-config`, `models`, `configure-providers` and `configure-models` run in any directory. `configure-providers` and `configure-models` write where [The configure commands' write location](#the-configure-commands-write-location) states.

API keys are in neither file. The credentials store (`kbase auth add`) contains a key under a provider's name, and an entry may name a key file instead (§9.1).

### Merging

The two files merge by two rules.

- `providers.toml` merges by provider name, and a provider's table is replaced whole: a repository table of the same name supersedes the global one entirely, never field by field, so a repository cannot inherit the global table's `apiKeyFile` for an endpoint it chose. A stored key belongs to the provider name, whatever layer supplies the table. Provider names compare case-insensitively (§9.1), so a repository table overrides the global table of the same name in any case. A table that a fault drops (§9.1) supersedes nothing: the lower layer's table of that name remains in the pool, and the fault is reported. A fault naming a provider that a higher layer supersedes is not reported: the pool's provider is the higher one, and a diagnostic about a value nobody reads is not a fact about this run.
- `config.toml` merges field by field: a value a higher layer states wins, and one it leaves out keeps the lower layer's value.

A layer that fails the strict load refuses the whole load, naming its file (§9.3): no lower layer is read, so a broken repository file never silently falls through to the global one.

### A leading `~` expands; nothing else does

In the `--config-dir` and `$KBASE_CONFIG_DIR` values, and in an entry's `apiKeyFile`, `~` alone is the home directory and `~` followed by a path separator is a path beneath it. `~user` is not supported, a `~` anywhere else is literal, and where the home lookup fails or its answer is not an absolute path, the string remains exactly as written.

### Global flags

Every subcommand accepts:

- `--config-dir DIR`: the one configuration directory read (the introduction above). The flag outranks `$KBASE_CONFIG_DIR`, and a blank value of either counts as unset.
- `--log-level debug|info|warn|error` (default `warn`).
- `--log-file PATH`: copies diagnostics to a file; console output is never redirected away.
- `--project-dir PATH`: the project root, the git worktree root that contains `kb-root/` or will. The subcommand's search for the repository starts there instead of the working directory. A path that is not a git worktree root is refused: `check: worktree`, `key` the `project-dir` argument formatted as the `key` rule in [§7](#item) states (an MCP call's `project-dir` is refused the same way), `path` the absolute path.
- `-h`/`--help`.

The root command also accepts `-v`/`--version`, which prints exactly the version string and a newline.

A relative path a subcommand takes (a volume root, `--bibliography`, `--state-dir`, `export-latex --out`, `--values`) resolves against the project root. `--config-dir` and `--log-file` resolve against the working directory, whatever `--project-dir` names, and a printed `resume` writes `--config-dir` absolute.

### Provider commands

`models`, `configure-providers`, `configure-models` and `auth` read and write the configuration. `show-config` is defined in §9.6 and `probe` in [§8](#probe). Each writes its result under [§7](#item). The inputs and outputs of all of them name a provider in lowercase (§9.1).

**`kbase models [--provider NAME] [--timeout DURATION]`** lists the selected provider's model identifiers, sorted ascending, as `models`. The provider is selected as [Provider selection](#provider-selection-is-the-catalogue-verbs-alone) states. `--timeout` bounds one request (default 30 s). `models` reports each configuration fault of the configuration it read as an item of `warnings` and as a line on stderr: `path` the file or directory the fault is against, `detail` what is wrong. A fault is a `providers.toml` entry dropped from the pool, a key file that is readable beyond its owner (§9.4) or cannot be read, a stored key file that does not open, or a store directory that exists and does not list ([The files are the one source of inference](#the-files-are-the-one-source-of-inference)). `warnings` is `[]` where there are none, and is present on every outcome, refusals included. A provider that refuses the call's key is a failure with the remedy [A refused key](#a-refused-key) states.

**`kbase configure-providers <provider> --base-url URL [--api anthropic|openai] [--decision-api systemone|decisions]`** writes a provider's table in `providers.toml`, creating the file where it is absent: it creates the table or sets the fields given. A run whose fields already have the values given writes nothing and is `unchanged`. **`kbase configure-providers <provider> --remove`** removes the provider's table. `--global` or `--local` chooses the layer written ([The configure commands' write location](#the-configure-commands-write-location)). Keys never pass through the command, and no command takes a key file's path: an entry's `apiKeyFile` is read from `providers.toml` and written by no command. Its result is `provider`, `layer`, `written`, `models`, `next` and `warnings`.

- Refusals, `check: usage`:
  - a write with no `--base-url`;
  - a `--base-url` containing credentials (`user:key@` or `key@`), with the `kbase auth add` pipe as the remedy and a detail that ends as a refused key's does ([A refused key](#a-refused-key)); this is checked before any other fault of the URL;
  - a `--base-url` that is no absolute `http` or `https` URL;
  - an `--api` other than `anthropic` or `openai`, or a `--decision-api` other than `systemone` or `decisions`, each with those values as `allowed`;
  - `--remove` beside an entry field;
  - a provider name that breaks the provider-name rule (§9.1), keyed `<provider>`.

  No refusal of `--base-url` names any part of it.
- `--remove` of a provider whose `providers.toml` has no entry is refused: `check: provider`, `path` that `providers.toml`, detail ``no provider `<name>` in <path>``, and `kbase show-config` as the remedy, which lists the entries that exist ([Idempotence](#idempotence)).
- Where the written entry has a key, stored or read from the file its `apiKeyFile` names, `configure-providers` sends a request to the provider under a 30 s deadline and gives the catalogue, sorted ascending, as `models`; this vets the connection. A failed request leaves the provider written, with the outcome `done`.
- `next` lists the steps that follow, as commands formatted as the `remedy` rule in [§7](#item) states. It is absent where there are none.
  - With no key, `next` is two commands: the `kbase auth add` pipe with the provider's name filled in, then `kbase models --provider <name>`.
  - Where the request fails, `next` is one entry naming the failure, then the `configure-providers` call for a wrong URL, the `kbase auth add` pipe for a wrong key, and `kbase models --provider <name>`.
  - Where the stack written into does not reload, `next` is one entry naming that and `kbase show-config`. Where the written entry does not load, it is one entry naming the fault's reason and `kbase show-config`. The written entry's own fault is both an item of `warnings` and, with its remedy, a step of `next`.
  - After `--remove`, where neither the layer written nor a layer beneath it still contains the provider, `next` has one entry per role whose reference names it, giving the role, the reference and the `configure-models` call that replaces it, including the removal's location flag; then, where a key file is stored under the name, one entry giving `kbase auth remove <name>`, saying where the file does not open.
- It reads only the stack it writes into (the stack is defined under [The configure commands' write location](#the-configure-commands-write-location)): a layer outside that stack that fails the strict load does not stop it. `warnings` lists every fault of the stack, as `models` reports them, on every outcome from its write or removal on; a refusal, or a write that fails, has none. `warnings` is `[]` where the stack does not reload after the write.

**`kbase configure-models [--decision REF] [--text REF] [--scoring REF] [--timeout DURATION]`** assigns a model reference (`<provider>/<model>`, §9.2) to each role given in `config.toml`, creating the file where it is absent. At least one role is given: a run naming none is a usage refusal. The roles not given are unchanged. `--global` or `--local` chooses the layer written ([The configure commands' write location](#the-configure-commands-write-location)). Its result is `models`, `written` and `warnings`.

- It sends one request to each provider named, `--timeout` (default 30 s) bounding each, and checks each reference against that provider's catalogue. A catalogue that cannot be listed is `failed`, with nothing written. A model missing from its catalogue is a warning, not a failure, since catalogues rotate: an item of `warnings` with `key` the role's flag and `detail` naming the role, the model and the provider, and a line on stderr.
- A reference's provider part is case-folded to lowercase and written so (§9.1).
- It refuses, before any request, a value that is no reference, else a provider that the stack written into does not hold. Each item is keyed by its role's flag, and every offending role is listed at once. A provider refusal has `path` the written layer's `providers.toml`, and as its remedy the `configure-providers <provider> --base-url <url>` call including the location flag this call was given, then `kbase show-config`; a provider that a fault dropped from the stack keeps the fault's reason ([Provider selection](#provider-selection-is-the-catalogue-verbs-alone)). A refusal writes nothing.
- `models` is the three roles the written file contains, null for a role it does not set.
- It reads only the stack it writes into: a layer outside it that fails the strict load does not stop it, and a stack that fails the strict load is `failed`, with no `warnings`. Once the stack loads, `warnings` lists every fault of the stack, as `models` reports them, then one item per catalogue miss, on every outcome; the list is `[]` where there are none.

**`kbase auth add <provider>`** stores the API key piped on its stdin for `<provider>`, a `providers.toml` entry name, replacing any key stored for it. The store is one file per provider, `$XDG_STATE_HOME/kbase/auth/<provider>-key.enc` (`~/.local/state` where the variable is unset or not an absolute path), named for the provider in lowercase, encrypted under a key derived for the OS user (obfuscation against a reader of the file, not a boundary against that user), and readable by its owner alone. `auth` is CLI-only: no MCP tool accepts a key ([§11](#served-text)).

- `add` reads the configuration as the provider commands do, and a configuration that does not load refuses it as it does for them. A name, case-folded to lowercase, that no `providers.toml` entry has, in the pool or dropped from it by a fault, is refused: `check: usage`, `key` the folded name, detail ``no provider `<name>` in providers.toml (names match in any case)``, `remedy: kbase show-config`.
- The key arrives on stdin and no other way. Each of the following is refused, `check: usage`, naming no part of the input, with the piped form (the provider's name filled in) as its `remedy`: stdin that is a terminal; input of more than 65536 bytes; input that is empty once surrounding whitespace is trimmed; a key that, so trimmed, still contains a whitespace or control character. The name refusals come before stdin is read, and a refusal writes nothing.
- A name that breaks the provider-name rule (§9.1) is a usage refusal, `key` the name as given and the detail the rule, never the name; its `remedy` is `kbase show-config` on `add` and `kbase auth list` on `remove`. A refusal of the argument count names no argument given.

**`kbase auth remove <provider>`** clears the stored key. Where none is stored it is refused ([Idempotence](#idempotence)): `check: usage`, `key` the folded name, detail ``no key stored for provider `<name>` ``, `remedy: kbase auth list`. **`kbase auth list`** names the providers with a stored key, in lowercase, as `providers`. `remove` and `list` do not read the configuration, so a key file whose entry is gone is still removed. No result names a stored key.

### A refused key

Nothing checks a key before a call. A provider that answers a call 401 or 403 has refused the call's key, a missing key and a wrong one alike, and the call fails as any provider refusal does ([§8](#a-provider-calls-failure-names-what-ended-it), `provider-refused`). Its failure item has `remedy` `<command-that-prints-the-key> | kbase auth add <provider>`, `<provider>` the provider the call sent its request to. The user runs the remedy, or an agent runs it with the command the user names or the `upgrade` that `show-config` gives. The item's `detail` ends, before the outcome's ask, by saying so, and that no one asks for the key, so it never enters a conversation.

The call failures that have it are a build's (the `inference-failed` stop of a claim-graph stage and the overview's `inference` failure), a scoring run's `failures` (from the attempt's ledger line, [§8](#the-ledger-and-resume)), `probe`'s failure, and the failure of `models` and `configure-models` to list a catalogue. A call ended by any other status has no such remedy. Neither does `configure-providers`' request, whose failure is its `next` ([Provider commands](#provider-commands)).

### Provider selection is the catalogue verbs' alone

`models` selects a provider to send a request to, not a role: a role is never chosen this way, and `config.toml` names no provider. The rule is `--provider`, else the sole provider. `models` refuses (`check: provider`) where it can select none: zero usable providers, two or more with none selected, or a `--provider` the pool does not hold.

- An empty pool, and a name the pool does not hold (in `--provider` or in a role's reference), point at the `providers.toml` that `configure-providers` writes into with no location flag ([The configure commands' write location](#the-configure-commands-write-location)), the file a provider's table belongs in. `configure-models`' value points instead at the `providers.toml` of the layer it writes, checked against that layer's stack ([Provider commands](#provider-commands)).
- A name the pool does not hold also has `key` the argument or `models.<role>` that named it, `allowed` the pool's provider names, a `detail` naming the table to add there (`configure-models`' says instead that its flag names a provider no `providers.toml` this call reads has), and a `remedy` naming `kbase configure-providers <name> --base-url <url>` (`configure-models`' including its location flag), then `kbase show-config`.
- Where that provider was dropped from the pool by a fault, the fault's own reason is the `detail`, with `kbase show-config` alone as the remedy.
- Two or more providers with none chosen list every provider name as `allowed`, with `key` the `provider` argument.

The argument, and each command a remedy names, are formatted as the `key` and `remedy` rules in [§7](#item) state. `show-config` reports any of these in its document and still exits 0.

### The configure commands' write location

`configure-providers` and `configure-models` write one layer's directory:

- With neither `--global` nor `--local`, a command writes the directory `--config-dir` or `$KBASE_CONFIG_DIR` names, whether or not it exists, the flag before the variable; else `~/.config/kbase`, even where a repository's `.kbase/` exists. `--global` writes `~/.config/kbase` and `--local` writes `<repository>/.kbase`; an explicit `--global` or `--local` outranks the variable.
- `--global` and `--local` together are a usage refusal keyed `--local`. Either beside `--config-dir` is a usage refusal keyed by that flag. `--local` outside a worktree is refused with `check: worktree`, `path` the directory the search started from, and `git init` as the remedy.
- `configure-providers`' `layer` names the layer written: `global`, `repository` (`--local`), `explicit` or `environment`.
- The stack a command writes into is the layer it writes and the layers beneath it: the global layer is beneath the repository layer, and nothing is beneath any other layer. The command reads that stack only.
- `configure-providers` creates `providers.toml` where absent, containing its comment header, an example entry commented out, and the entry written, and no other entry. `configure-models` creates `config.toml` where absent from its commented template: comments that explain the file and guide the `[models]` ids (the guidance `kbase://model-hints` serves, [§11](#resources)), an empty value for each role it is not given (which parses as unset), and `[asks]` and `[sampling]` commented out. Each otherwise edits the file under §9.5. A refusal writes nothing, and so does a `configure-models` catalogue failure.

### The files are the one source of inference

The endpoint and the model each role uses come from `providers.toml` and `config.toml` alone, and so do the settings a role's calls are made with: `[asks]` and `[sampling]` (§9.2). An entry's key is the one the store contains under its name, whatever layer supplies the entry; else the one in the file its `apiKeyFile` names, which an entry with a stored key never reads; else none. Every verb that reads the configuration reads the store.

- A stored key file that does not open is treated as no stored key (the entry's `apiKeyFile`, else none) and is reported by its path and what is wrong with it, never its content: as a diagnostic in `show-config` (§9.6), as an item of `warnings` and a line on stderr in `models`, `configure-models` and `configure-providers`, and as a warning line on stderr in `build`, `score` and `probe`. A stored key file that no entry's name matches is read by nothing and is not reported.
- A store directory that exists and does not list yields no stored key for any entry, and is reported the same way, by its path and what went wrong. It does not stop the configuration loading.
- No environment variable supplies an endpoint, a model or a key. `$XDG_STATE_HOME` chooses where the store is, never a key. A run isolated from the user's layers names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`; that does not isolate it from the user's store, but `$XDG_STATE_HOME` does.
- The inference preflight ([§5](#inference-preflight)) names the lacking setting as the refusal's `key`: `models.decision`, `models.text` or `models.scoring`. `show-config` reports that refusal as a diagnostic rather than stopping (§9.6).

There is no plaintext-transport switch. When a key is configured and the base URL is `http://` to a host other than loopback, `build`, `score` and `probe` write one line on stderr saying the key travels in cleartext, and proceed (as §9.4's permission warning does).

### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses. A provider name is one or more ASCII letters, digits, `_` or `-`, not beginning with `-` (`^[A-Za-z0-9_][A-Za-z0-9_-]*$`). A table whose name breaks the rule fails the file's load, naming the file and the entry.

**A provider name is case-insensitive everywhere.** Every input naming a provider is case-folded to lowercase without complaint: each table name read from `providers.toml`, `--provider`, the provider part of a model reference, the provider that `configure-providers` or `configure-models` is given, and the provider that `auth add` or `auth remove` is given. Every output names a provider in lowercase. Two tables of one file whose names differ only by case refuse the load, naming both names as written and the file.

An entry:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; an entry's only key field
type = "inference"     # optional; the only accepted value, also the default
api = "anthropic"      # optional; "anthropic" (also the default) or "openai" —
                       # the chat protocol this server speaks
decisionApi = "systemone"  # optional; "systemone" (also the default) or
                               # "decisions" — the decision API this
                               # server speaks
```

A key inside an entry that this list does not name fails the file's load (§9.3). An entry that fails validation is **dropped** with a warning naming the entry and the reason (never key material), and the rest of the pool still loads. An entry fails validation when:

- its `baseUrl` is empty;
- its `baseUrl` contains credentials (`user:key@`), or is no absolute `http` or `https` URL with a host (the reason names no part of the URL);
- `type`, `api` or `decisionApi` is set to anything but its accepted values.

A failure to read the key file faults just that entry the same way. An absent `providers.toml` is an empty pool, not a load error.

#### The chat protocol

`api` names the wire protocol chat uses, chosen per server; it has two values. `anthropic`, what an entry silent on `api` gets, is the Anthropic Messages API, `POST <baseUrl>/messages`. `openai` is the OpenAI-compatible `chat/completions` API. The two differ in more than a path. Over the OpenAI-compatible protocol a thinking level is an effort hint that the server may exceed without bound. A Messages request has `budget_tokens` and a `max_tokens` ceiling, and the ceiling bounds thinking even where a server overruns the budget.

Every chat call and catalogue listing (`models`, `configure-providers` and `configure-models`, `probe`'s chat selections, a build's overview call, a scoring run's calls) uses its entry's `api`. The `decision` role's calls never do (below).

A Messages request:

- Requires a `max_tokens` cap. The allowance for the answer is `max(256, ceil(characters / 4))` over the characters of the request's messages and system text. A role whose `thinking` is not `off` (§9.2) adds its thinking budget: 1,024 tokens for `low`, 4,096 for `medium`, 16,384 for `high`, sent as `thinking: {type: enabled, budget_tokens: <budget>}`. Thinking `off` is sent as `thinking: {type: disabled}`; the field's absence would leave it at the server's default.
- Takes the system prompt as a top-level `system` string rather than a turn, so a system message is lifted out of `messages`.
- Sends `temperature`, the role's, 0 where unset, whatever the thinking level.
- Sends no `prompt_cache_key`, `chat_template_kwargs`, `reasoning` or `reasoning_effort` field.

This mapping targets servers that implement the Messages protocol: oMLX, LiteLLM-style proxies, OpenRouter. Anthropic's own endpoint rejects a temperature beside thinking, and its newer models may reject `budget_tokens`, `temperature` or `thinking: {type: disabled}`. A Messages entry's catalogue is `GET <baseUrl>/models`, whose `created_at` is RFC 3339.

An `openai` request contains the role's `temperature`, 0 where unset. Thinking `off` sends `chat_template_kwargs: {enable_thinking: false}` and `reasoning: {enabled: false}`. A level sends it three ways at once, since a server reads whichever it speaks: `reasoning_effort: <level>`, `reasoning: {effort: <level>}` and `chat_template_kwargs: {enable_thinking: true}`.

`decisionApi` is unaffected by `api`, and both decision APIs are OpenAI-shaped: the `decision` role's multichoice questions always go to the decision API's endpoint on its entry's base URL, authenticated as an `openai` entry is, whatever the entry's `api`. A server that speaks only Messages has no such endpoint, and the decision call fails with that server's HTTP status.

#### The decision API

`decisionApi` names which of two decision APIs the server speaks; each has its own endpoint. It changes nothing about chat, which `api` alone decides, and it names no model id: the decision role's model id stays in `config.toml` (§9.2). An entry silent on it speaks `systemone`.

- `systemone`: `POST <baseUrl>/systemone`, body `model`, `state` and `questions`, an object keyed by each question's own key. The response is an `answers` object keyed the same way.
- `decisions`: `POST <baseUrl>/decisions`, body `model`, `input` (the same composed state) and `questions`, an array of `type`, `name`, `instructions` and `choices`. A `noul` question is asked as a `predicate`. A `choice` question's criteria are its `choices`, each option a `value` with its guidance as the `description`, sorted by value. The response is an `answers` array, each entry having the `name` it answers: a `predicate` (a probability, parsed as P(true)), a `choice` (its choice, confidence and per-value probabilities), a `score`, or a `refusal` (a declined answer, with no value, which leaves the question unanswered).

An answer naming a question the request did not ask is refused under either shape. A question left unanswered is retried once, then takes its deterministic default ([§5](#co-linearity-cull-claims-culled)).

### 9.2 `config.toml` — the choices

```toml
[models]
decision = "<entry>/<id>"      # the claim graph's multichoice questions: a providers.toml
                               # entry name, then the model that entry's
                               # decision API serves
text = "<entry>/<id>"          # the overview passage
scoring = "<entry>/<id>"       # kbase score's model calls; required for score

[asks]
concurrency = 4                # optional; model calls one build or score has in flight at once, across every role

[sampling.text]                # optional; the overview passage's sampling
temperature = 0.0              # optional; a number
thinking = "off"               # optional; off | low | medium | high
[sampling.scoring]             # optional; kbase score's sampling, the same two keys
```

**A `[models]` value is a reference**, `<provider>/<model>`, split at the **first** `/`: a provider name is the name of a `providers.toml` table and contains none, while a model id may (`openrouter/anthropic/…`). Either part empty, or no `/` at all, is not a reference and is refused at resolution, naming `models.<role>` and the form. Nothing is normalised but surrounding whitespace and the provider part, case-folded to lowercase (§9.1): a model id keeps the provider's formatting, not kbase's.

A role left unset resolves to "not configured": no defaulting, no guessing, and no role falls back to another.

`concurrency` is how many model calls one invocation of `build` or `score` has in flight at once, across every role. Absent, it is 4. `--ask-concurrency` overrides it for that invocation, and a flag value under 1 is a usage refusal keyed by the flag. A `concurrency` under 1 fails the load, naming `asks.concurrency`. Calls start in the order the invocation submits them.

#### Sampling is per chat role

`[sampling.text]` and `[sampling.scoring]` each take `temperature` and `thinking`, both optional. A key left unset is kbase's default, temperature 0 and thinking off, never the provider's. They merge field by field like every `config.toml` value, each keeping the layer that supplied it. There is no `[sampling.decision]`: the decision role's fixed-set probabilities take no sampling, and a `sampling.decision` key fails the strict load as unmodeled (§9.3). A `thinking` value other than the four fails the load, naming `sampling.<role>.thinking` and the four. Resolving a role refuses, naming `sampling.<role>.temperature`, a temperature outside the range its entry's chat protocol accepts: 0 to 2 for `openai`, 0 to 1 for `anthropic`.

### 9.3 Strict-load contract

Both files are decoded strictly: any key or table the schemas above do not model fails the **whole file's** load. The message names the file path and every offending key by its full dotted path (for example `models.typo_key`), as "unknown key" or "unknown keys" when more than one. A malformed-TOML file fails with a parser-detail message naming the file and line. Only key **names** appear in either message, never values: a file may still contain a secret a hand edit left behind, and the refusal that names its key is exactly where that value would be echoed.

A verb that reads the configuration and meets a file that fails the strict load is refused before it does anything else: one `check: usage` item whose `detail` is the message. Neither configure command repairs a file that fails the strict load; the remedy is a hand edit.

A layer that supplies a provider supplies its `apiKeyFile` path too, and kbase confines no key file's location; and a stored key goes to the endpoint of whichever table has its name, in whatever layer. Trusting a cloned configuration means trusting the key file it names, and the endpoint it would send a stored key to, as much as the endpoint it names.

### 9.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile`, the store contains no key under its name, and the host is not Windows, kbase checks the resolved key file's permission bits. If they grant group-read or other-read, it reports a warning (as a fault, [Provider commands](#provider-commands)) naming the key file by its path and as the one `providers.toml` names, never by the field that states it, with the offending mode and recommending `chmod 600`. The entry stays usable. A key file that cannot be read (§9.1) is named the same way, with the refused-key remedy ([A refused key](#a-refused-key)). This is the only file-permission behaviour kbase applies to a file it did not create.

### 9.5 The configure commands' targeted update (byte-preservation contract)

`configure-providers` and `configure-models` never re-serialize the file they write; each edits the existing bytes in place, keeping every byte it was not asked to change.

- An absent or blank file gets its template ([The configure commands' write location](#the-configure-commands-write-location)), then the edit below.
- `configure-models` changes **only** the `[models]` keys of the roles given. `configure-providers` changes **only** the keys it is given (`baseUrl`, and `api` and `decisionApi` where given) in the table its provider names, matched whatever its case; an entry's other keys, `apiKeyFile` included, are unchanged.
- A key already set to the value is not rewritten, so a run that changes nothing writes nothing and is `unchanged`. Every other byte (comments, unrelated keys and tables, spacing, key order) is preserved exactly. A rewritten line's own trailing `# comment` is kept; its column position is not guaranteed.
- A missing key is added after the last key of its table; a missing table is appended at the end of the file.
- `configure-providers --remove` removes the provider's table: its header, through its last key and the lines directly below that key with no blank line between, and one blank line that would otherwise be left doubled. A comment set off by a blank line before the next table is left for that table.
- Before writing, kbase decodes both the original and the proposed file as TOML. Unless the only difference between the two decoded structures is exactly the values it intended to change, the write fails (`failed`) and the file is untouched. A file that is not valid TOML fails the same way.
- The write is atomic. A file that is a symlink has its target rewritten; the symlink remains.
- Any failure along this path leaves the file completely unwritten.

### 9.6 `kbase show-config`

`kbase show-config [--provider NAME]` reports how this invocation's configuration resolves, and changes nothing: it reads no KB, writes no file and makes no network request. `--provider` names the provider the selection reports. Its result keys are the `show-config` row of [§7](#item). A configuration that fails the strict load is refused before the verb runs (§9.3). Otherwise the outcome is `done` whatever the report finds: a fault in the pool and an unresolved role are diagnostics, not refusals, so one call shows every gap at once.

The report states:

- the layers this invocation read, in precedence order, each with its path, the source that named it (`explicit` for a flag, `environment` for `$KBASE_CONFIG_DIR`, `repository` or `global`), and whether the directory and each of its two files exist;
- `config.toml`'s merged `[models]` values, null where a role is unset, and its `[sampling]` values, null where unset;
- every provider in name order, with the layer that supplied it and the chat protocol and decision API it speaks, each default filled in (so a provider silent on `decisionApi` is reported as `systemone`), including a provider that a dropping fault removed (unusable, with the fault's reason);
- the provider the selection names and the rule that named it, or why none was named;
- one record per role: the endpoint it resolves to, the layer that supplied each field, a chat role's sampling settings, or why it does not resolve.

The sources vocabulary is the layers': `repository` or `global` for a value a file supplies, `explicit` or `environment` for one read from the one directory a flag or `$KBASE_CONFIG_DIR` named.

Its diagnostics are, in order:

- one `provider` item per fault, with `path` the `providers.toml` that has the fault (which need not be the file a selection or `configure-providers` names). For a stored key file that does not open, `path` is that file, `detail` is what is wrong with it and, where it is truncated or does not decrypt, `remedy` is the refused-key remedy ([A refused key](#a-refused-key)) with the provider's name filled in. For a store directory that exists and does not list, `path` is that directory and `detail` is what went wrong, with no `remedy`;
- one `provider` item per provider whose stored key leaves the `apiKeyFile` it states unread, `path` the `providers.toml` that states it and `remedy` removing that line from the file, or `kbase auth remove <provider>` to use the file;
- one item per role that does not resolve, its `path` naming the file the setting belongs in: the write target's `providers.toml` or `config.toml`.

The upgrade (below) and the unread-`apiKeyFile` warning appear in this report only: no other verb's output has them.

#### Names and paths, never secrets

A key is reported by the kind it is (`store`, `file` or `none`) and a key file by the path its entry states, never by its content. A `file` key's entry has `upgrade`, the command that relocates the key into the store, with its values filled in: `cat <key-path> | kbase auth add <provider>`. The stored key then outranks the file. An agent runs the command as printed ([§11](#served-text)). `kbase auth add` runs from a shell with the key piped on stdin; no MCP tool accepts a key (§11).

#### It describes the process that ran it

The layers, the environment and the selection are this invocation's, so a call through the MCP server (§11) reports the configuration of the server process. `--config-dir` and `$KBASE_CONFIG_DIR` are set where the server is launched; no tool takes either.
