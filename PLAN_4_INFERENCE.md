# Phase 4 — the inference asking rework

Status: **design**, the first work item of Phase 4 (`ACTIVE_PLAN.md`). It went to the
prompt-engineer three times, not to an intersprint review; the findings of the first two passes and
the live rulings since `98ebf54` — R1, R2, R4, R5, R6, R7, and §14.7's, with R3 superseded by R6 —
are applied here, the rubric landing's ruling among them: an update applies its behaviour, so a run
resumed across a changed ask or weight continues, lands what the running table moved and asks nothing
(§12's WP4, criteria 44 and 50). §12 is the dispatch plan, its third pass closed with its findings
applied; the rubric landing (WP4) is later than those passes, and carries its own prompt-engineer
dispatch and review. It
settles the endpoint resolution, the client lifecycle, the per-role model references, the two
inference modes, the `scoring` role, the two scoring-time flags, the harness back end and its seeded
profile, and the rigour-and-bearing rubric's landing — §12's WP4, judged by §11's second list rather
than by an outcome — and states the acceptance each of the plan's six outcomes is judged by.

It changes no relation, no new derived quantity, no MCP tool and nothing of the walk's arithmetic
(`STRENGTH_SCORING_SCHEME.md`, I10). Its one scoring change is the two passes' **answer**: the
decimal they ask today becomes a list of charged faults, priced by a table in kbase (I14, §12's
WP4), and the ladder's bottom rung takes the words `unsubstantiated`. `PLAN_2_SCORE.md` is retired
and is not its basis; where this document needs a design decision that landing made, it cites the
contract documents instead.

**How to read it.** A coder reads §2 (the invariants) and §3 (the failure modes they prevent) first,
then §4.5 (the two modes), §5, §6 and §7 for the rework's spine, the section their work package
touches, and §11's criteria for their outcome — or, for WP4, §11's second list, and the decisions its
block names before dispatch. §0's grounds are commands to re-run, not facts to trust.

---

## 0. What this replaces, and the grounds it stands on

| Site | Today | After |
|---|---|---|
| `cmd/provider.go:90` `providerOptions.dial` | seven returns: name, rule, layer, client, call deadline, release, error — one call that selects, dials, bounds and reports | deleted; selection is `config.Select`, a role's endpoint is `config.Resolve`, the mode is `config.ModeOf`, the client is the registry's (§6), the deadline is the caller's (§8.5) |
| `cmd/provider.go:232` `resolveProvider` | the one resolution, reading the pool, the merged config and the environment at once, per verb | deleted; `config.Resolve` per role, pure (§5) |
| `cmd/provider.go:172` `selectProviderName` | three rules: flag, `config.toml`'s `provider`, sole entry | moves to `internal/config.Select`; `config` is no longer a rule reachable there — the key it read is inert (§4.3) |
| `internal/config/config.go:32` `Config.Provider` | the one global pool entry | inert and unused from the first landing, deleted at the next major (§4.3); each role's reference names its own (§4.1) |
| `internal/config/config.go:61` `ModelMap` | `decision`, `text`, bare ids | `decision`, `text`, `scoring`, each a reference in file mode (§4.2) |
| `internal/config/inference.go:87` `ResolveInference` | merges one environment layer over the files, field by field, and reads `KBASE_API_KEY_FILE` (`:126`) | replaced by `ModeOf` + `Resolve`, which read no file; the two modes are exclusive (R2, §4.5), and the key file is read at dial (§5.3) |
| `internal/score/run.go:24` `Ask`, `:29` `Write` | the runner's two seams | unchanged in shape; the harness is a second `Ask` (§9) |
| `cmd/score.go:205-213` the `Ask` adapter | `model.Chat` on the text category's model | one adapter per back end, both filling the same seam (§9.1) |
| `cmd/build.go:87` `buildProvider` | one client, two model ids (`internal/build/build.go:192` `Provider{Client, Letters, Overview, …}`) | one client, one model id and one provenance set per role; the build holds as many clients as its roles name (§6) |

**Grounds.** Each fact below was re-run at `482a7ce` (the tip after `98ebf54`, where `ACTIVE_PLAN`
moved and the code did not); a coder who doubts one re-runs it.

- `grep -n 'func (o providerOptions) dial' cmd/provider.go` → `:90`, signature
  `(name string, rule providerRule, layer string, client model.Client, callCtx context.Context, release func(), err error)`
  — seven returns, four of them a decision, a resource, a lifecycle and provenance.
- `grep -n 'func resolveProvider' cmd/provider.go` → `:232`; its consumers are `cmd/build.go:89`,
  `cmd/show-config.go:55` and `cmd/score.go:183` — three verbs, one resolution, and each of them
  reads the same seven-value dial beside it.
- `grep -n 'os.ReadFile' internal/config/inference.go` → `:126`: resolution reads the key file, so
  "resolution does no I/O" is not true today.
- `sed -n 253,262p internal/config/providers.go` → an entry naming no `apiKeyFile` resolves to no key,
  nil error; `sed -n 154,157p` → `Validate` requires `baseUrl` alone; `sed -n 8,18p
  internal/config/report.go` → `KeyKindNone` is "an endpoint that needs no key";
  `sed -n 122,124p internal/config/inference_test.go` → the `"no key"` case expects no warning. A key
  is optional at both ends (§4.5).
- `grep -n 'func NewHTTPClient' internal/model/http_client.go` → `:93`; the type has no `Close`, and
  `Client` (`internal/model/client.go:30`) has none either — "close at exit" is new lifecycle, not an
  existing one.
- `grep -n 'execOwners' -A 5 cmd/execpolicy_test.go` → `:22`, four owners, each one statically named
  binary (`:73` fails a call whose binary "resolves to no binary"), so a user-named command needs a
  stated exception (§9.4).
- `grep -n 'documentedKeys' -A 2 cmd/resultkeys_test.go` → `:20` (build), `:36` (score): the key
  order is pinned by a test, so a document change is a test change in the same work package.
  `sed -n 130,145p cmd/resultkeys_test.go` → `inOrder` checks each documented key's order against the
  emitted one and permits a subset, which is why a key that is conditional needs no new rule.
- `grep -n 'Provider string' internal/score/ledger.go` → `:57`, and `Model string` → `:58`: the run
  header already carries one endpoint and one model. `sed -n 133,158p` shows `Resume` reads
  `run.yaml` and never writes it — §9.2's first answer depends on that.
- `sed -n 253,262p internal/score/text.go` → `assemble` returns the guarded *text*, not a prompt;
  `sed -n 189,197p internal/score/run.go` → `compose` renders it through `asks.ScoreRigor`/`ScoreBearing`
  into the template; `sed -n 205,209p cmd/score.go` → `Ask` is handed that render, which is what
  `model.Chat` sends as its user turn. The harness receives that, not the bare text (§9.1).
- `sed -n 264,270p internal/model/http_stream.go` → the stream response decodes `choices` and `usage`
  and no `model` field, so no observed model id exists anywhere in kbase's transport (§9.2, #4).
- `git log -S 'heavy' --oneline --all` names `42ebe68 cmd: show-config replaces config; model
  categories replace tiers`, and `git show 42ebe68:internal/config/config.go` shows
  `Provider string` with the tag `toml:"provider"` beside `Models ModelMap` — the global provider
  key has stood since the categories replaced tiers, and no revision ever carried a provider per
  role. There is no prior art for this grammar, so §4.3's compatibility path is new text, not a
  restoration.
- `sed -n 50,64p internal/config/layer.go` → `Layers` returns one named directory, or repository over
  global — the order every configuration file of a layer follows, `harnesses.toml` included from R6.
- `grep -n 'test-integration-build-live-arxiv' justfile` → `:515`, whose doc says "a second build …
  configured by KBASE_API_BASE_URL, KBASE_DECISION, KBASE_TEXT and KBASE_API_KEY_FILE alone, no
  configuration directory". With `SPEC.md:866-873`'s promise, this is the live leg that makes the
  environment mode a capability to keep, not retire (R2).
- `sed -n 47,56p internal/config/write.go` → `CreateProvidersFile` is `providers.toml`'s only writer,
  the pattern `harnesses.toml`'s creator follows, and `sed -n 44,64p internal/config/template.go` →
  the template it writes, whose header says "the names are arbitrary" (`:48-50`) — the open-set
  grammar `harnesses.toml` reuses (R5, §4.6).
- Both seed commands, run on this host at `b904ff5`; the outputs are in §4.6's table. `claude` ships
  live and `opencode` ships commented with its limitation (R7):
  `printf '…0.5\n' | claude -p --tools "" --strict-mcp-config --safe-mode` → `0.5`;
  `opencode run --format default '…0.5'` → `0.5`;
  `printf '…0.5\n' | opencode run -m anthropic/claude-haiku-4-5` → no stdout (stdin is ignored);
  `claude -p --model d3 …` → exit 1, "There's an issue with the selected model (d3)".
- `SPEC.md` §7 `:396` closed `check` vocabulary; `:405-408` "a refusal lists every offending item"
  and the category refusal naming two categories; `:414` build's row; `:426`–`:430` the catalogue
  verbs' and `score`'s rows. §8 `:683` the synopsis, `:705-706` the guard's four tokens, `:719-721`
  the model category, `:723-731` the ledger. §9 `:866-873` the environment's promise, `:870-871` the
  field-by-field override sentence R2 supersedes, `:875-877` the inference preflight (which gains
  `KBASE_SCORING` and the two modes, §13), `:921-926` the grammar, `:939-949` strict load,
  `:958-975` the targeted update. §11 `:1406-1412` what the server never does.
- `ARCHITECTURE.md` §6 `:396` and `:518-607` (scoring), §7 `:643-654` (resolution), §11 `:902`
  (`kept: config, model`), `:963-966` (who imports `model`), §12 `:993` (model categories) and the
  transport-attempt count (`3 per call`).
- `CONVENTIONS.md:47-60` (`Endpoint`, the flavour mapping living in `cmd`), `:50-51` (the overview
  ask's re-issue), `:81-93` (the exec monopoly), `:310-320` (dependencies).

---

## 1. The sketch, kept or rejected

| Hypothesis from the brief | Verdict | Why |
|---|---|---|
| Split resolution from connection; `Resolve(role) → Endpoint{provider, model, rule, layer}`; a registry dials by name on first use and closes at exit | **kept** | It is the only split that lets a build hold two endpoints of two providers, and it removes the seven-return call. `Endpoint` carries more than the four fields — see §5.1 — because the dial needs a base URL, a flavour and a key. |
| Provider names carry no `/`; a reference splits on the first one | **kept** | The split is the grammar (§4.1). A model id may contain `/` (`openrouter/anthropic/…`), which is why it is the *first* one. |
| `[models] scoring` holds the pool's reference | **kept**, with one refinement | Under a harness the reference's **model part** is what the harness is asked for; the provider part is not contacted. That keeps the role meaningful in both modes, which is what makes "the scoring role must be configured, in every mode, or the run refuses" a rule rather than a hurdle (§8.1). |
| `providerOptions.dial` retired by the split | **kept** | §6. |
| The document says whether a model is requested or observed; a harness that cannot report | **kept, resolved to "requested only"** | Neither back end observes the serving model — the transport decodes none — so the document carries the requested id and nothing else, and `null` is how a reader sees that nobody named one (§9.2, I7). |
| `--timeout` bounds one item's call for scoring; the catalogue's timeout is the catalogue's | **kept** | The registry derives no deadline; each caller bounds its own call (§8.5). |
| **Rejected:** `--harness` as a bare switch | rejected | A switch cannot say which harness, and the result document must (`roles[].harness`). It names a profile (§4.6). |
| **Rejected:** the harness reached as an MCP client | rejected | Harnesses are MCP *clients*; there is no server to dial, and `SPEC.md:1406` keeps the kbase server out of inference. |
| **Rejected:** the harness's argv written inline in the flag (`--harness "claude -p"`) | rejected | It needs a shell-word splitter, which the repo has in one direction only (`build.ShellJoin`, `internal/build/status.go:347`) and no parser for. An argv array in TOML needs no parser at all. |
| **Rejected:** a `model-reported` key "for completeness" | rejected | No producer fills it. That is the "just in case" defect, and it invites a reader to treat its absence as agreement (§9.2). |

**Rulings since the first draft**, applied in the sections named: **R1** the scoring variable is
`KBASE_SCORING` (§4.5); **R2** the environment is a second, all-or-nothing inference mode, kept
(§4.5, §5, §11); **R4** harness profiles live in a configuration file that `configure --create` seeds
with `claude` and `opencode` — its placement in `providers.toml` is **superseded by R5**; **R5** the
profiles have their own file, `harnesses.toml` (§4.6, §4.7, §11, §13); **R6** the repository layer's
`harnesses.toml` **is read**, exactly as its `providers.toml` is, and supersedes R3 (§4.6, §7.1, §9.4,
§14). One decision outside that series is ruled too — §14.7's base-URL-alone refusal — and so is
**R7**: `claude` ships live and `opencode` ships commented with its limitation stated (§4.6, §11, §13,
§14.8). Those are the live rulings; §14 records only the points they leave to this design, marked as
such, and R3 is history: R6 replaced it, and no section states it.

---

## 2. Invariants

Numbered; each is a review check, not aspiration.

**I1 — Resolution is pure.** A role's endpoint is a function of the loaded configuration, the pool
and the environment's strings. It reads no file, opens no connection, writes no stderr and names no
verb. The key file `KBASE_API_KEY_FILE` names is opened by the registry, never by resolution.

**I2 — One grammar per mode.** In file mode a model is named `provider/model`, split at the first
`/`; in environment mode a model is a bare id of the environment's endpoint. One `Reference` type
builds both — the bare id is an `Environment` reference's model, never a string split by hand — and no
site reads a bare id as a fallback to a provider.

**I3 — No role falls back.** No role's endpoint, model or provider is taken from another role. A
role that does not resolve is refused by name — `models.<role>`, `KBASE_<ROLE>` or the flag — and
nothing is asked.

**I4 — One step decides, another connects.** A client is opened only when a verb is about to ask
something, and every client a verb opens is closed before the verb returns, on every outcome.

**I5 — Endpoint count is not fixed.** No type in the design assumes one endpoint per verb. The build
resolves two, `score` one, the catalogue verbs one pool entry (which is not a role).

**I6 — One fact, one key.** A verb resolving one endpoint states provider provenance at the
document's top level. A verb whose endpoints are per role states them in `roles`, one record per
role, in role order. No document states the same fact twice.

**I7 — The model a document names is the one requested.** Never an observed one. It is null where no
source named one, **or where the profile cannot carry one** — a harness profile whose command has no
`{model}` asks its own model and kbase cannot name it — and a reader tells a harness's own default
from a named model by that null and nothing else.

**I8 — The back end is below the seam.** The harness is one implementation of `score.Ask`. The
runner, the closed-text guard, the write seam, the append-after-write rule and the refresh cadence
learn nothing about back ends; only the ledger's line gains the facts that distinguish them.

**I9 — A harness command is the user's code, and its source is never silent.** A profile layers as
every configuration file does — the repository's over the global one, or the one directory
`--config-dir`/`$KBASE_CONFIG_DIR` names — and a profile the repository supplied announces itself on
stderr once per run, in the role record's `harness-layer` and in the ledger's `harness_layer`. It
receives the prompt as its whole input — on stdin or through `{prompt}`, never both.

**I10 — The scoring model does not move.** No fifth relation, no new derived quantity, no change to
`STRENGTH_SCORING_SCHEME.md`, no new dependency, no new MCP tool. It bounds the walk: a pass's
*answer* is the one scoring fact this design moves, and I14 is what bounds that.

**I11 — No endpoint is inferred.** A file-mode reference must name its provider; a bare id there is
refused rather than resolved through the superseded `provider` key, which is inert and never a
fallback.

**I12 — A run's record is readable without its invocation.** Every ledger line carries the back end,
the requested model and, under a harness, the profile's name and `harness_layer`, so a mixed or
resumed run's lines say what answered each item and which layer's file named it.

**I13 — Two modes, never a merge.** Under the provider back end, inference is supplied by the files or
by the environment and by one of them only: an environment model name with a file-supplied inference
is a refusal, and the environment mode is complete or refuses naming the variable it lacks. The key
file is not a mode. A harness reads no endpoint, so it consults neither mode (§4.5).

**I14 — A pass's value is a projection of its recorded charges.** The value is the table's output for
the ids its ledger line records, and nothing else is an input: a citation is the audit of a charge and
never a term in the sum, and the counts are integers. The two tables, their maxima and the floor at
zero stand in one place (`kb`, §10), no template spells a number, and the sum is exact in integer
hundredths — `0.95 − 0.30 − 0.30` is `35/100`, not a float. A landing is checkable against its own
line without a model, and a re-derivation makes no call.

**I15 — A run's judgements are identified by the ask, not by the table.** The run's header records
`ask_sha256`, the digest of its pass's prompt rendered for an empty subject, and no template carries a
size-dependent element, so that digest is a function of the template alone and one digest certifies one
instruction. It is a record and gates nothing: it says which instruction answered which judgement, so a
corpus that compares models and versions can be read honestly across an update, and it decides nothing
at run time. A changed weight re-prices the recorded ids under the running table, lands the values that
moved through the write path — the register is not left stale behind the ledger — and the run records
that it was re-priced; a resume under a changed ask continues under the current ask. Nothing refuses,
and nothing asks the user to choose between versions.

---

## 3. The ways a coder gets this wrong

The skeleton exists to prevent these; each names the invariant that stops it.

| Failure | Prevented by |
|---|---|
| `scoring` falls back to `text` when absent — the obvious code, since `score` used `text` | I3; §8.1's refusal is the only branch |
| The old field-by-field merge is kept "because it still compiles": an environment variable quietly overriding one file field | I13; `ModeOf` decides once and refuses the mix (§5.1) |
| A second resolution appears beside `resolveProvider` (e.g. "the harness path needs its own look at the config") | I1/I4; `resolveProvider` is deleted and `config.Resolve` is the only function that reads a role |
| `Resolve` returns a client, or dials to validate an endpoint | I1/I4; `Endpoint` is a value type and the registry is the only constructor of `model.Client` |
| The harness is handed the whole reference (`openrouter/sonnet`) as its model | §8.1: the harness is asked for the model part |
| A repository's profile runs as if the user had chosen it: no line, no layer in the record | I9; §4.6's one stderr line and §7.1's `harness-layer` make the source visible |
| A profile's `command` is read from `providers.toml`, or a pool entry from `harnesses.toml` | §4.6: the two files are separate namespaces, and neither schema models the other's tables |
| `model-reported` is added "so the harness's answer is complete" | I7 |
| `build` keeps a top-level `provider` "because consumers read it" | I6; a build with two providers cannot state one |
| The template still asks for a decimal while the parser reads charge lines, or the reverse: every item of every run is `malformed` | §12's WP4 lands the ask, the parse and the value in one boundary; no intermediate commit is shipped and no live leg runs between them |
| The weights are subtracted in float64, so three gap charges land `0.3499999999999999` and the register's four-decimal rule breaks | I14; the table and the sum are integer hundredths |
| The parse finds a span in the assembled text but cannot say which passage it came from, or offsets it in the prompt rather than the closed text | `internal/score/text.go:255`'s `assemble` hands the passage extents beside the text; a ledger `offset` is the closed text's |
| The fault ids are spelled in the templates and again in the parser, and the two drift | §10 and I14; the vocabulary and both tables are `kb`'s, and `asks` asserts each template's `flt-…` tokens against its pass's set |
| `ask_sha256` is made a gate: a resume across a changed ask refuses, or the user is asked to pick a version — or `Resume` restamps the header, so the run stops saying which instruction it began under | I15; the digest is a record and `Resume` reads it and writes nothing (criterion 43), while criterion 50 is that a resume after an update continues under the new ask |
| A re-price rewrites the ledger's `value` and leaves the register's landed value as it was: one number in the KB and another in the audit line behind it | I15; the moved values land through the write path, and criterion 44 fails on either side disagreeing |
| The ladder's slug is changed and treated as a display edit: a stale index renders its bottom-rung claims as pending and counts them under no `stats` row | §12's WP4 states the pre-refresh state and makes `refresh` the repair; criterion 45 is the check |

---

## 4. The configuration surface

### 4.1 The reference

```
<provider>/<model>          # provider: a providers.toml entry name; no `/` in it
                            # model:    the entry's model id; may itself contain `/`
```

Split at the **first** `/`. Either part empty, or no `/`, is not a reference. Trimming is whitespace
only; nothing else is normalised (a model id is the provider's spelling, never ours).

The type serves both modes (§4.5): a file-mode value parses as a reference, an environment-mode value
is the reference's model under a pseudo-provider named `environment`. That name is not a pool entry
and nothing resolves through it — it exists so the formatter produces one shape from either mode, and
a pool entry that happens to be called `environment` is unrelated to it. One constructor builds each
(`config.FileReference(id)`, `config.EnvironmentReference(id)`), so no site concatenates or splits by
hand.

### 4.2 `config.toml`'s grammar

```toml
[models]
decision = "lab/d3"            # the claim graph's letter asks
text     = "lab/Qwen3.5-9B"    # the overview passage
scoring  = "cloud/clef"        # kbase score's asks; required for score, in every mode

[asks]
readerConcurrency = 4          # unchanged
```

Harness profiles are not here: they have their own file, `harnesses.toml` (§4.6).

- The top-level `provider` key no longer chooses anything: it loads inert and is removed at the next
  major (§4.3). `models.<role>` is where an endpoint is named.
- A `[models]` value is a reference, and it is a **file-mode** value: one alongside an environment
  model variable is a refusal, not a merge (§4.5, I13).
- Under a harness the reference's model part is what is asked and its provider part is not contacted
  (§8.1) — the grammar is unchanged, the reading is not.
- `[models] heavy`/`[models] light` keep failing the strict load exactly as today (`SPEC.md:936-937`).
- `scoring` is a third category, which is **three** edits, not one: `ModelMap` and `modelCategories`
  (`cmd/configure.go:31`) gain it; `freshConfig` (`internal/config/write.go:110-114`) gains its
  `assign(...)` slot and `configTemplate` (`internal/config/template.go:21-24`) a matching `%s`; and
  `modelHints` (`internal/config/hints.go:23`) gains a row, which renders the **comment block** only.
  The hints row reaches both the template's comment and the `kbase://model-hints` resource; it does
  not render the live key. A coder who edits only the hints table gets a commented example and no
  writable line.

### 4.3 The compatibility path

A configuration that names a provider globally is **stated superseded and never used**, and the
values that depended on it are **refused with the replacement named**. Three parts, and the middle one
is what keeps the path runnable:

1. **`provider` stays modelled and is inert.** It loads, it merges as it does today, and nothing
   resolves from it. Every verb that resolves a role or selects an entry in **file mode** writes one
   line on stderr: `config.toml states the superseded provider key; kbase does not use it — run kbase
   configure to rewrite the file`. A query verb, which resolves nothing, says nothing; an environment
   mode invocation, in which no file value is read, says nothing.
2. **A bare model id in a `[models]` value is refused at resolution**, naming `models.<role>`,
   stating the `provider/model` form, and carrying the remedy
   `kbase configure --provider <entry> --model-map decision=<entry>/<id>,text=<entry>/<id>,scoring=<entry>/<id>`.
   It is refused **even where `provider` names the entry that would make it work**: resolving it
   through that key is exactly the silent reinterpretation the outcome forbids, and a refusal is what
   tells the user the file is old.
3. **`configure` performs the rewrite.** It loads (1 keeps the file loadable — `SPEC.md:948-949`'s
   "configure cannot repair a config file it cannot load" would otherwise block the remedy the
   refusal names), writes the three references and deletes the `provider` line (§4.7). After it, the
   file is in the new grammar and (1)'s line stops appearing.

The allowance is time-boxed, not permanent: `provider` leaves the schema at the next major, when the
README's examples, `USER_CONVENTIONS.md` and Phase 3's `references/scoring.md` have moved. That
removal is recorded in `ROADMAP.md` by the contract package (§13, WP5), because it is future intent
and does not belong in the contract documents.

Nothing is reinterpreted: no bare id is ever resolved, and the key that used to name the endpoint
never silently becomes the provider of a reference.

### 4.4 Strict load

The strict-load contract (`SPEC.md` §9.3 today, §9.4 after §13's insertion moves it) stands
unchanged in kind — any key not modeled fails the whole file's load, naming every offending key by
dotted path, names only, never values. **Two of the three files' schemas move, and the third is
untouched:** `config.toml` gains `models.scoring`; `providers.toml` is unchanged by this design; and
`harnesses.toml` is new, with a closed per-profile schema — `command`, an array of strings that must
be **non-empty**, where an **element may be empty**: the claude seed's `--tools ""` is one, and a
schema that refused it would refuse the seed the template prints — and an open set of names.
`provider` stays in `config.toml`'s schema, inert, for §4.3's window: it is the one key modelled and
read by nothing but the diagnostic that reports it superseded.

### 4.5 The two modes (R2)

Inference comes from exactly one of two sources, and never from both.

| | **File mode** | **Environment mode** |
|---|---|---|
| Supplied by | `providers.toml` + `config.toml` | `KBASE_API_BASE_URL` + the role variables |
| A model is named | `provider/model` in `[models]` | a bare id in `KBASE_DECISION`, `KBASE_TEXT`, `KBASE_SCORING` |
| The endpoint | the named pool entry | `KBASE_API_BASE_URL`, and no pool entry |
| The pool and `[models]` | in force | supplied by nothing, and **a `[models]` value that stands is a refusal, not an ignored line** (below) |
| Entered when | no environment inference variable is set | `KBASE_API_BASE_URL` or any role variable is set |
| Complete when | every role the verb needs has a reference | the base URL and every role the verb needs are set |

**The rules.**

- **R1 — the variables are role names.** `KBASE_DECISION`, `KBASE_TEXT` and `KBASE_SCORING` are the
  three roles, spelled as `[models]` spells them; there is no `KBASE_*_MODEL`. `show-config`'s
  environment list (`internal/config/report.go:83`) gains `KBASE_SCORING`.
- **All or nothing.** The environment naming a model requires `KBASE_API_BASE_URL`: without it the
  refusal names the variable that is set *and* the one that is missing. It works the other way too —
  a base URL alone, with the files naming the models, is the same refusal, because a base URL is an
  endpoint and an endpoint is the whole of what the environment mode supplies.
- **No merge.** Any environment inference variable alongside a `[models]` value is refused, one item
  per side (`KBASE_TEXT` and `models.text`), with the detail saying the two sources are exclusive and
  naming which of the two to unset. `SPEC.md:870-871`'s "a variable given overrides the corresponding
  field and nothing else" is superseded by this rule, and §9 carries the replacement sentence. The
  cost, stated: a user whose global `config.toml` names all three roles and who exports
  `KBASE_SCORING` to try another model is refused, not overridden — `--scoring-model` is the
  scoring-time override, and an environment *mode* is a whole configuration, not a patch.
- **The environment names the whole mode.** When it is in force, no pool entry is selected and
  `providers.toml` and `config.toml` contribute nothing to inference — a faulty pool entry is not
  reported, because nothing reads it.
- **The key file is not a mode.** `KBASE_API_KEY_FILE` names a file, not a provider and not a model:
  it overrides the key of whichever endpoint is in force — the selected entry's in file mode, the
  environment's in environment mode; it is optional in both, and absent everywhere means no key is
  sent (`internal/config/providers.go:154-157` requires only `baseUrl`, `:258-261` resolves an entry
  naming no file to no key, `internal/config/report.go:14`'s `KeyKindNone` is "an endpoint that needs
  no key", and the `"no key"` case at `internal/config/inference_test.go:123` expects no warning). So
  `KBASE_API_KEY_FILE` alone never flips the mode and never makes a mixed source.
- **The scoring role must be configured.** `[models] scoring` in file mode, `KBASE_SCORING` in
  environment mode; absent in the mode's source, the refusal names that key and asks nothing, and
  nothing falls back to `text`. `--scoring-model` **overrides** a configured role and never
  substitutes for one: the role is the KB's declaration of which model judges it, and a flag is a
  choice made at the shell about a role that stands.
- **The mode is a fact about the provider back end.** `--harness` does not consult it: a harness run
  uses no endpoint, so a set `KBASE_API_BASE_URL` or role variable is not a mixed mode and not a
  refusal. The role's model under a harness follows §8.1's precedence, which reads the environment
  only for that value — and only where the profile's command names `{model}`, since a profile with no
  token cannot carry a model at all.

**Why the mode is kept** (the ground the review's escalation asked for): `SPEC.md:866-873` promises
"an integrator configures a build from the invoking environment with no configuration directory at
all", and `justfile:515`'s `test-integration-build-live-arxiv` runs exactly that leg, LIVE, with
`KBASE_API_BASE_URL`, `KBASE_DECISION`, `KBASE_TEXT` and `KBASE_API_KEY_FILE` alone. Retiring the
mode would retire a contract capability and a live recipe; keeping it needs a writable form, which
the all-or-nothing rule supplies.

### 4.6 Harness profiles — `harnesses.toml`, a file of their own (R5)

```toml
# harnesses.toml — an excerpt: the file `configure --create` writes carries a
# header, and above each profile the comment it ships with
[claude]
command = ["claude", "-p", "--tools", "", "--strict-mcp-config", "--safe-mode"]

# [opencode]
# command = ["opencode", "run", "--format", "default", "{prompt}"]
# Off by default: `opencode run` (v2.0.32) has no flag that removes tools or
# suppresses instructions, so a profile runs the default `build` agent with its
# tools, its own system prompt and the instruction files opencode loads — and an
# agent with tools can read the KB the ask is about. Run an agent that cannot
# instead, by naming it with `--agent`: that agent comes from the agent set you
# install, not from kbase.
```

**The third file.** `<config dir>/harnesses.toml`: top-level tables are profiles, keyed by name, an
open set — the grammar `providers.toml` uses for pool entries, including the merge rule, an entry
replaced whole by a higher layer's entry of the same name. The table is **not** nested: `<name>` is
top-level, and there is no `[harness.<name>]` anywhere.

**R4's placement is undone, and with it its collision.** `providers.toml` is unchanged by this
ruling: no table of this design's lives in it, and no name is reserved there — a pool entry called
`harness` is an ordinary provider entry again, as it was before R4. The two files do not share a
namespace, which is the whole of what the third file buys.

**Layers (I9, R6).** `harnesses.toml` layers exactly as `providers.toml` does: the repository's over
the global one, or the single directory `--config-dir`/`$KBASE_CONFIG_DIR` names, and an entry
replaced whole by a higher layer's entry of the same name. There is no exclusion and no drop.

**The source is never silent.** A profile resolved from the **repository** layer writes one line on
stderr naming the file and the layer, once per invocation, on the same footing as §4.3's
superseded-key line — a warning, never a refusal, and it does not stop the run. A profile from the
global layer, or from the one directory a flag or the variable names, says nothing: the user put it
there. `show-config` says nothing on stderr either; it reports the layer in
`harnesses[].layer` (§7.2). The role record carries the same fact as `harness-layer` (§7.1), so a
document written by a run says which layer's file named the command even after the terminal is gone.

A profile takes the name its highest layer states — the repository's file over the global one, or the
single directory in place of both. A malformed `harnesses.toml` in a layer that is read refuses the
whole load, as a malformed file of either other kind does (`SPEC.md:939-949`).

**The verbs that load configuration parse it, and one verb uses it.** Configuration is loaded before
such a verb does anything (`SPEC.md:948-949`), so `models`, `configure` and `show-config` parse the
profiles file and use none of it, while the MCP server loads no configuration and no tool asks a
harness; `score` alone resolves a profile, and it
is not an MCP tool. That is not a behaviour a verb carries silently: `SPEC.md:1412`'s "It reads no
harness configuration and names no harness" is reworded to say exactly this (§13, §11), rather than
the server reading a file it loads none of.

**The seeds (R5, R7).** `configure --create` writes `harnesses.toml` where none stands, from a
template `internal/config` owns — the same creation-only treatment `providers.toml` gets
(`CreateProvidersFile`, `internal/config/write.go:47-56`; a `CreateHarnessesFile` beside it),
reached from `cmd/configure.go:230-241`. **Nothing else seeds**: an existing `harnesses.toml` is
never rewritten to gain an entry (§4.7). Two ship — `claude` **live**, `opencode` **commented with
its limitation stated** — and that is the whole catalogue; any other harness is the user's to write,
in the argv its own CLI documents. The pool template's "one live entry" rule
(`internal/config/template.go:35-37`) does **not** apply: a profile is only ever named by
`--harness`, so a live profile is not a choice the configuration makes behind the user's back.

**The argvs, verified on this host** (every row run at `b904ff5`; neither command is changed by R5 or
R7 — R7 only comments the second one out):

| Command | Output | What it settles |
|---|---|---|
| `printf 'Reply with exactly one line: 0.5\n' \| claude -p --tools "" --strict-mcp-config --safe-mode` | `0.5`, exit 0, stderr empty | `claude -p` takes the prompt from **stdin**; `--tools ""` removes every built-in tool; `--strict-mcp-config` with no `--mcp-config` ignores other MCP servers; `--safe-mode` drops CLAUDE.md, skills, plugins and hooks; stdout is the answer alone |
| `opencode run --format default 'Reply with exactly one line: 0.5'` | `0.5`, exit 0, a status line on stderr | `opencode run` takes the prompt as a **positional argument**, writes the answer to stdout and its progress to stderr |
| `printf 'Reply with exactly one line: 0.5\n' \| opencode run -m anthropic/claude-haiku-4-5` | **no stdout**, exit 0, stderr `> build · claude-haiku-4-5` | piping a prompt to `opencode run` with no message argument asks **nothing** — the reason the contract has `{prompt}`, not a quirk of one seed |
| `opencode run --format default "$(<multi-line prompt>)"` | `0.78`, exit 0 | a multi-line prompt survives as one argv element |
| `claude -p --model d3 --tools "" --safe-mode 'say hi'` | exit 1, stdout `There's an issue with the selected model (d3)…` | a pool model id is not a claude id — why the seeds name no `{model}`, and what a profile naming `{model}` does with a wrong id |
| `opencode run --help` (v2.0.32) | offers `--agent`, `--format`, `--file`, `--title`, `--thinking`, `--auto`, and **no flag that removes tools or suppresses instructions** | a profile runs the default `build` agent, with its tools, its own system prompt and whatever instruction files that CLI loads — settled, and why opencode ships commented (R7) |

**The ruling (R7), stated once.** `claude` ships live and `opencode` ships commented with its
limitation in the file's own comment. The reason is not argued twice: **a seed cannot harden a CLI that
offers no way to be hardened, and a seed pretending to would be a claim kbase cannot keep.** The
tool-less agent is not kbase's to write — `ACTIVE_PLAN.md:73-75` records it as adjagent's, for **both**
harnesses, selected through a harness's own agent configuration, unowned and Phase 3's — so the design
points there and leaves the entry commented until it lands or a user uncomments it.

**Not verified, and said rather than guessed:** whether `claude --system-prompt ""` is accepted (the
seed does not pass it, so only the disabled-tools state matters). And `--safe-mode`'s reach is
narrower than "no harness baggage": it disables CLAUDE.md, skills, plugins, hooks and MCP
configuration, but **admin-managed policy settings still apply**, and it says nothing about Claude
Code's own system prompt — which is the `--system-prompt ""` question, not a guarantee.

**Model.** Neither seed names a `{model}` — the live `claude` and the commented `opencode` alike: each
runs on the harness's own configured model, which is the only thing a fresh install can rely on — the
alternative, `--model {model}` with the KB's pool id, fails every item (`d3` above). Adding
`--model {model}` (claude) or `-m {model}` (opencode) is how a user opts a profile into
`--scoring-model`; §4.6's rule then stands unchanged — a profile naming no `{model}` refuses a
`--scoring-model`, because a model that would reach nowhere is a silent wrong answer. The seed
comments say so, in the file the user edits.

**Two tokens, one grammar.** `{model}` (§8.2's value) and `{prompt}`. `config.HarnessArgv` fills
`{model}` at setup, once per run; `(*harness.Command).Item` fills `{prompt}` per item, in the one
element that names it (§9.1). An argv element carrying `{prompt}` receives the prompt and stdin is
closed empty; an argv with no `{prompt}` receives the prompt on stdin. That is how `claude` (stdin)
and `opencode` (argument) share one contract without kbase knowing either CLI. Substitution is one
pass over the profile's elements, `{model}` then `{prompt}`, never re-scanned — a document containing
the text `{model}` is not rewritten — and a prompt carrying a NUL, or longer than
`harnessPromptLimit`, fails that item before a process starts, the error in the ledger's `reply`
(§9.1). The ledger's `command` records the argv with `{prompt}` **unsubstituted** and `{model}`
substituted: the run's header names how it was invoked, not the text of one item (§9.2).

**What a fresh install still needs to know.** Run `configure --create`; then
`kbase score --pass rigor --harness claude` runs with no further flag, on claude's own model, tools
off. `--harness opencode` refuses until someone enables it — naming `--harness` with the loaded
profile names as `allowed`, the ordinary no-such-profile refusal, not a shape of its own — and
enabling it is uncommenting the entry or naming an `--agent` that cannot read the KB. What kbase
cannot supply is what the user still brings: `claude` or `opencode` installed and authenticated, and a
scoring role configured (the KB's declaration, §8.1). Only a user who wants a particular model edits a
profile to add `{model}`. Nothing about `-p`, stdin, `--tools` or `--format` is known, typed or
documented by them.

### 4.7 `configure`'s edit contract

`SPEC.md:958-975` keeps its shape and changes its key set: the editor rewrites the `[models]` table's
`decision`, `text` and `scoring` lines, **removes** a top-level `provider` line (the one deletion the
editor performs, and the decoded-structure check at `:970-972` covers it like any other change), and
writes an absent `scoring` key into the table. Every other byte is preserved as today. The template's
header comment ("updates only the `provider` and `[models]` values", `internal/config/template.go:16-18`)
changes with it, since after this landing it also removes `provider` and writes a third key.

`--model-map` values are references. The dialed path requires every category — `decision`, `text`,
`scoring` — because a `config.toml` that makes `kbase score` refuse is an incomplete file, and
`configure` is the verb that writes one. **The rule is that the file must not be left incomplete, not
that every run must restate every key:** a dialed run that leaves `scoring` unassigned refuses naming
`models.scoring` where the file has none to keep, and leaves the file's own value untouched where it
has one — so `configure --provider P` over a complete file writes nothing for `scoring`, and is
`unchanged` where it changes nothing else. This moves `SPEC.md:405-408`'s enumeration of the two
categories; §13 lists it. `configure` dials each **distinct provider** its references name (one, two
or three catalogues, through the registry) and warns per id against its own provider's catalogue.
`--create` with no `--provider` stays offline and unchecked (`SPEC.md:862-865`).

**The one thing `--create` writes that is not the user's values:** the `harnesses.toml` template it
creates carries one live harness profile and one commented entry with its limitation (§4.6).
`--create` writes **three** files where they are absent — `providers.toml`, `harnesses.toml`, then
`config.toml` — each created only where none stands, so `configure` never adds a profile to a file the
user wrote, and `--create` over an existing directory remains an editor of whichever files are missing
plus `config.toml`. A user who deleted a seed keeps it deleted. `harnesses.toml` is created under
`--create` alone, exactly as `providers.toml` is: a plain `configure` on an existing directory creates
neither.

---

## 5. Resolution

### 5.1 Types (`internal/config`)

```go
type Role string                    // RoleDecision, RoleText, RoleScoring
var Roles = []Role{RoleDecision, RoleText, RoleScoring}   // the order documents report

type Mode string                    // ModeFile, ModeEnvironment

type Reference struct{ Provider, Model string }
func ParseReference(s string) (Reference, error)          // I2's one parser, file mode
func FileReference(id string) (Reference, error)          // = ParseReference
func EnvironmentReference(id string) Reference            // the environment endpoint's model
func (r Reference) String() string

// BackEnd is which seam fills the ask; a role under a harness resolves with no
// provider and no endpoint, its model part alone.
type BackEnd string
const (BackEndProvider BackEnd = "provider"; BackEndHarness BackEnd = "harness")

// Inputs is everything a decision reads: the loaded files and the environment's
// strings. One struct so ModeOf, Select and Resolve share one parameter list.
type Inputs struct {
    Cfg       Config
    Providers Providers
    Faults    []ProviderFault
    WriteDir  string      // the file a lacking setting belongs in
    Env       Overrides
}

type Endpoint struct {
    Role      Role
    Mode      Mode      // ModeFile, or ModeEnvironment where the environment supplied it
    Provider  string    // pool entry name; "" in environment mode
    Model     string
    Rule      string    // "" | RuleFlag | RuleConfig: what named the provider
    Layer     string    // the layer that supplied the pool entry; "" with Provider
    BaseURL   string
    APIKeyFile string   // KBASE_API_KEY_FILE, expanded, unread (the dial reads it)
    Flavor    string    // the entry's decision surface; SystemOne where none
    APIKey    string    // from the pool entry, which the loader already read
}

// ModeOf is pure (I1, I13): which source supplies inference, once per
// invocation, and the refusal where the sources are mixed or the environment is
// partial. (The type is Mode; the function cannot share its name.)
func ModeOf(in Inputs) (Mode, error)

// Resolve is pure (I1): one role's endpoint in a mode.
func Resolve(in Inputs, mode Mode, role Role, backEnd BackEnd, flagModel string) (Endpoint, error)

// Select is the catalogue verbs' choice of a pool entry — not a role, file mode
// only, and still flag-then-sole-entry: a role is never chosen that way.
func Select(in Inputs, flag string) (name, rule string, err error)

// HarnessArgv fills {model} in a profile's argv — and only {model}: {prompt} is
// the item's, and (*harness.Command).Item fills it (§9.1). It refuses a model the
// command would never carry. Pure; the one place the profile meets a model.
func HarnessArgv(p Profile, model string) ([]string, error)
```

`Refusal` (in `config`) replaces `LackError` as the one typed refusal: `Key, Path, Allowed, Remedy,
Detail`, with `Named() (key, path string)` kept so `internal/build`'s `namedSetting` mapping
(`internal/build/build.go:570-573`) is untouched. `Refusals []Refusal` is the aggregate a verb maps
to items, which is how one document enumerates every gap (`SPEC.md:405`). `config` still imports no
`result`, no `model` and no `log`.

**The load returns the profiles beside the pool** (R5): `harnesses.toml` is a third file of every
layer, so one load produces all three sources and each profile carries the layer that supplied it.
`Load`'s four returns become one value — otherwise every caller grows a fifth return and one of them
will drop it:

```go
type Profile struct{ Name string; Command []string; Layer string }
type Harnesses map[string]Profile

type Loaded struct {
    Providers Providers
    Faults    []ProviderFault
    Harnesses Harnesses      // one per profile, the highest layer's entry whole
    Cfg       Config
}
func Load(layers []Layer) (Loaded, error)
```

`Layer` gains `HarnessesExists` beside `ProvidersExists` and `ConfigExists`, so `show-config` reports
the third file's presence with the other two (its probe is the same `exists` seam).

`HarnessArgv(profile, model)` is the only function that pairs a profile with a model; the verb looks
the profile up in `Loaded.Harnesses` by the `--harness` value and passes the pair on. `Resolve` never
sees a profile.

### 5.2 What a refusal names

`check` is `provider` except where the row says otherwise; `path` is named where it differs from the
write target.

| State | `key` | `allowed` | Notes |
|---|---|---|---|
| **Mode**: an environment role variable is set without `KBASE_API_BASE_URL` | the variable set, and `KBASE_API_BASE_URL` | — | two items, one per side |
| **Mode**: an environment inference variable — the base URL or a role — is set while a `[models]` value stands | one item per side: the variable, and the `models.<role>` key | — | detail says which side to unset (`SPEC.md:870-871` superseded) |
| **Mode**: environment mode, and a role the verb needs is unset | `KBASE_<ROLE>` | — | `KBASE_SCORING` for `score`; only where no `[models]` value stands, else the row above |
| **File mode**: `models.<role>` absent, or a bare id | `models.<role>` | — | a bare id's detail gives the `provider/model` form and the `configure` remedy |
| **File mode, provider back end**: the reference's provider is not in the pool | `models.<role>` | every pool entry name | not checked under a harness, where the provider part is not contacted |
| **File mode, provider back end**: the provider faulted out of the pool | `models.<role>` | — | the fault's reason is the detail |
| `--scoring-model` unparseable in file mode | `--scoring-model` | — | `check: usage` |
| `--scoring-model` naming a provider the pool lacks, in file mode | `--scoring-model` | every pool entry name | criterion 19 |
| `KBASE_API_KEY_FILE` unreadable | the variable | — | refused at dial, §5.3 |
| Catalogue verbs: no usable provider in the pool | — | — | `path` the write target's `providers.toml` |
| `--harness` naming no loaded profile | `--harness` | profile names | `check: harness` |
| `--scoring-model` with a profile whose `command` names no `{model}` | `--scoring-model` | — | `check: harness`; raised by `HarnessArgv` (§4.6) |

`harness` is a new member of `SPEC.md:396`'s closed `check` vocabulary. Nothing else in that list
moves.

**Where the mode's rows overlap, every item is listed** (`SPEC.md:405` asks for that, and a user
facing two sides needs both named). `KBASE_TEXT` set with no `KBASE_API_BASE_URL` and a `models.text`
standing yields three items: the variable, the missing base URL, and `models.text`. A base URL alone
beside a standing `[models]` yields `KBASE_API_BASE_URL` and one item per `[models]` key that stands —
that is the whole set the user must decide between, not one representative of it.

A **load** error is not a refusal document (`SPEC.md:939-949`), and this design adds no third kind: a
`harnesses.toml` whose profile carries a key the schema does not model fails the load naming the file
and the key, exactly as a `providers.toml` entry does. A repository profile is not that: it loads, and
its source is announced rather than refused (§4.6, I9).

### 5.3 The key file moves to the dial

`ResolveInference` reads `KBASE_API_KEY_FILE` (`internal/config/inference.go:126`). Under I1 the
read moves to the registry, which still returns the same `Refusal` (`key: KBASE_API_KEY_FILE`), so
the user-visible outcome is unchanged: a run refuses before asking, with the variable's name. The
invariant is what moves, not the behaviour — and a `Refusal` is a refusal wherever it arises, which is
the mapping rule `cmd` applies (`refusedOrFailed`, `cmd/provider.go:276`).

`show-config` makes no request and opens no client, so it cannot learn a key file is unreadable by
dialing. It calls the registry's **preflight** for each resolved role — the registry's one key-file
read, with no client opened — and maps a failure to a diagnostic. That keeps `SPEC.md:981-983`'s "one
call shows every gap at once" without a second reader of the key file and without I/O in `Resolve`.

### 5.4 What resolution deliberately does not decide

- It does not decide whether the run may ask at all: `--dry-run`, `--no-inference` and `--pass` are
  the verb's.
- It does not read the KB, the state directory or the ledger.
- It does not choose a back end, and `Resolve` does not choose a mode: `ModeOf` decides once per
  invocation, and the verb passes both in.
- It does not warn. Fault warnings are the verb's stderr (`cmd/provider.go:95-104` keeps its home,
  moved to the registry's caller), and they are file mode's alone — in environment mode no entry is
  read, so none is reported.

**Who consults the mode.** The mode describes a provider endpoint, so exactly the verbs that resolve
one consult it: `build` and `score` refuse a mixed or partial environment; `show-config` runs
`ModeOf` and reports the refusal as a diagnostic, never stopping (`SPEC.md:981-983`). The catalogue
verbs — `models` and `configure` — select a pool entry to dial and **never consult it**: they ask for
a catalogue, not a role, and a user who has exported `KBASE_TEXT` must still be able to run the verb
that repairs `config.toml`. Under `--harness`, `score` consults neither the mode nor a role's endpoint
(§4.5's last bullet).

---

## 6. The registry

One type, in `cmd` (`cmd/provider.go`), because `cmd` is where `model` is imported for verbs
(`ARCHITECTURE.md:963-966`) and no other package may construct a client:

```go
type clients struct {
    newClient func(model.Endpoint) model.Client   // the seam, as today (`cmd/provider.go:29`)
    opened    map[string]model.Client             // key: entry name, else "base:"+BaseURL
    log       log.Logger
}
func (c *clients) get(ep config.Endpoint) (model.Client, error)  // opens on first use
func (c *clients) preflight(ep config.Endpoint) error            // the key-file read alone
func (c *clients) closeAll()                                     // at the verb's exit
```

- **Open on first use**, keyed by the pool entry's name — two roles on one provider share a client and
  its connection pool; an environment-mode endpoint keys by base URL, which is all it has.
- **Close together.** `*model.HTTPClient` gains `Close() error` (`CloseIdleConnections` on its
  cloned transport). `closeAll` closes every client it opened that implements `io.Closer`; a close
  error is logged, never raised — the verb has already written its result.
- **No deadlines.** `--timeout` is the caller's (§8.5).
- **The seam stays.** `newClientFunc` is unchanged in shape, so every verb unit test keeps its
  `model.MockClient` factory.
- **Lifetime.** Each verb defers `closeAll` before it writes its result; a refusal before any client
  is opened closes nothing.

`build.Provider` (`internal/build/build.go:192-201`) becomes one client, one model id **and one
provenance set** per role — the split today's struct cannot carry, since `Name`/`Rule`/`Layer` stand
once (`:192-201`, consumed at `:576-581`):

```go
type Provider struct {
    LettersClient  model.Client; Letters  string; LettersEndpoint  EndpointProvenance
    OverviewClient model.Client; Overview string; OverviewEndpoint EndpointProvenance
    ReaderConcurrency int
}
type EndpointProvenance struct{ Name, Rule, Layer string }   // the role record's three keys
```

`build`'s walk passes each client to its own `call.New` (`internal/build/build.go:577-581`), records
each role's provenance for the result's `roles`, and a fail path names the role and the provider that
failed rather than "the provider".

---

## 7. Result documents

### 7.1 The role record

One record per role, key order fixed, a key omitted where it does not apply:

| Key | Holds |
|---|---|
| `role` | `decision`, `text` or `scoring` |
| `back-end` | what answered: `provider` (a pool entry), `environment` (the environment's endpoint) or `harness` (a profile) |
| `provider` | the pool entry's name |
| `provider-rule` | the rule that named it: `flag` or `config` |
| `provider-layer` | the layer that supplied the entry: `explicit`, `environment`, `repository`, `global` |
| `model` | the model id this invocation asked for; null where none was named (§9.2) |
| `harness` | the profile's name, under a harness only |
| `harness-layer` | the layer that supplied the profile: `explicit`, `environment`, `repository` or `global` (R6) |
| `base-url`, `sources` | `show-config` only: the endpoint, and the layer that supplied each field |
| `detail` | `show-config` only: why the role does not resolve, where it does not |

`provider`, `provider-rule` and `provider-layer` remain absent together wherever no pool entry
supplied the endpoint, exactly as `SPEC.md:414` states them. `back-end` is what tells the three cases
apart — a named entry, the environment's endpoint, or a harness — so a reader never infers the mode
from an absence. `harness-layer` sits immediately after `harness` in every record that carries both:
`cmd/resultkeys_test.go:130-145`'s `inOrder` permits a record to omit a documented key and not to
reorder one, so the position is the test's, not a per-verb choice.

### 7.2 Per verb

| Verb | Change |
|---|---|
| `build` | `roles` replaces the top-level `provider`, `provider-rule`, `provider-layer`; one record per role the build resolved (`decision`, `text`), in that order; absent where none resolved (`--no-inference`) |
| `score` | `roles` replaces the top-level `provider`, `provider-rule`, `provider-layer` **and** `model`; one record, `scoring`; absent under `--dry-run` as the four were |
| `models`, `configure` | unchanged at the top level: they select a pool entry to dial, which is not a role. `configure`'s `models` sub-record gains `scoring`, and its `written` list may name three files (`providers.toml`, `harnesses.toml`, `config.toml`, §4.7) |
| `show-config` | `inference` retires; `roles` takes its place and its position (after `selection`), one record per role in `Roles` order, each with `base-url`, `sources` and — where the role does not resolve — `detail`; `config` loses `provider`, and its `models` holds `decision`, `text` and `scoring` as the values written — a reference verbatim, null where unset; `selection` loses its `config` branch (§4.3 leaves no such rule) and keeps `flag`, `sole-entry` and `detail`; `mode` names the mode (§4.5), or is null with `detail` where the sources are mixed — and says nothing about harnesses, which supply no endpoint; `layers` gains `harnessesExists` beside `providersExists` and `configExists` (R5); `harnesses` lists the loaded profiles in name order — `name`, `layer`, `command` (the argv as written, tokens in place) — and is `[]` where none loads, because `--harness`'s choices are configuration a reader must be able to see. The document's own key order is `layers`, `providers`, `config`, `environment`, `selection`, `mode`, `roles`, `harnesses`, `diagnostics`, and each nested mapping has its order in `documentedSubKeys` — `show-config.harnesses` is `name`, `layer`, `command`; `show-config.roles` is §7.1's. **A mode refusal and an unresolved role are diagnostics here, not a refusal**: `show-config` exits `done` whatever it finds (`SPEC.md:981-983`), so its `mode` and `roles` carry what `ModeOf` and `Resolve` returned rather than stopping the verb. A profile's `layer` is its source (§4.6, R6), so the repository's file is visible here even though nothing on stderr is said |

### 7.3 Why not the top-level keys with a role suffix

`SPEC.md:385` requires one key per fact. A build reaching `lab` for its letter asks and `cloud` for
its overview has no single `provider`, and the two alternatives are both worse: a suffixed pair
(`provider-decision`, `provider-text`) duplicates three keys per role and grows with every role
added, and a top-level key naming "the first role" states a fact that is not the fact a reader
wants. One record per role, one shape for all three verbs, is the minimum that states it once.

**Cost, stated.** `build`'s and `score`'s documents break for a reader that keyed on `provider`, and
`show-config`'s `inference` block retires. Phase 3's consumers are the two named in
`PLAN_3_TOOLING.md`: `kb-tools`' harness reads the KB, not result documents; adjagent's
`references/scoring.md` reads `show-config` (`PLAN_3_TOOLING.md:83-84` says the agent is told the
configured provider and text model and that `show-config` "reports what answers"), so its `inference`
sentences move to `roles` in Phase 3's own landing. The break is a documentation change for them,
not a code change — and the alternative is a document that lies about a build with two providers.
`cmd/resultkeys_test.go:20`, `:36` and `:52` change in the same work package that changes the emitter.

---

## 8. The scoring role and the two flags

### 8.1 The `scoring` role

- `[models] scoring` names the role in file mode, `KBASE_SCORING` in environment mode. **Absent in the
  mode's source is a refusal** (I3), in every mode and under a harness, even where `--scoring-model`
  names a model: `check: provider`, the mode's key, the remedy the form it should take, before the KB
  is opened — and before the run's start refresh, as every setup refusal is (`SPEC.md:752-753`).
  Nothing falls back to `text`, and the flag overrides the configured role rather than replacing it.
  A `--dry-run` resolves no role and refuses nothing for one: it is the listing, and the refusal
  belongs to the run that asks.
- **File mode:** the reference is `provider/model`; the provider's entry supplies the endpoint and
  the model part is asked (`model.Chat`, as today).
- **Environment mode:** `KBASE_SCORING` is the model; the endpoint is `KBASE_API_BASE_URL`. No pool
  entry is involved.
- **Harness mode:** where the profile's command names `{model}`, the role's **model part** is what the
  harness is asked for — `KBASE_SCORING` if the environment names one, else `[models] scoring`'s model
  part — and `--scoring-model` overrides whichever stands. Where it names none, the profile asks its
  own model and kbase sends nothing: neither seed names `{model}` (`--model` absent, §4.6), and the
  live `claude` profile is this case, so the document's `model` is null and I7 says why. The role must
  still be configured in one of those two places (§4.5). No pool entry is opened and no endpoint is
  chosen, which is why the mode's
  exclusivity does not apply here. This is the refinement over the brief's sketch: it is what makes
  the role do work in harness mode, and it is honest under the sketch's own sentence — a *pool
  reference* cannot mean a harness model, but the model part of one can, and is.
- **`--dry-run` consults neither.** It asks no model, so it resolves no role and runs no `ModeOf`: a
  listing over a configuration that would refuse a real run still lists the set (§8.1's first bullet,
  criterion 14).

### 8.2 `--scoring-model`

The flag is read by the mode that reads the role's model (I2):

| Where | Value read as | The run asks | `roles[0].model` |
|---|---|---|---|
| file mode | a reference `provider/model` | that provider, that model | the model part |
| environment mode | a bare id, the environment endpoint's model | that endpoint, that model | that id |
| **any** back end under `--harness` | the harness's model id, **verbatim, never split** (it may contain `/`) | the profile, with `{model}` filled by it | that id |

Precedence: the flag outranks the role's configured value in every mode. In file mode an unparseable
value is `check: usage`, `key: --scoring-model`; an id naming a provider the pool lacks is a
`provider` refusal naming the flag (§5.2). Under environment mode and under a harness it is accepted
as written — that is the point of a harness-accessible id, and the environment names no provider to
parse against. So an environment-mode value bearing `/` is **not** refused at resolution: it is the
environment endpoint's model, sent verbatim, and a server that does not know it answers with its
own error — the item's `failed`, its reason in the ledger's `reply`. Guessing that a `/` meant a
provider would be the inference I11 forbids.

### 8.3 `--harness`

`--harness NAME` selects a profile (§4.6) and the back end with it. It is a `score` flag alone: no
other verb asks a model through a harness, and `build`'s asks stay provider-only in this phase. It
composes with `--scoring-model` in every mode, and it is accepted on every `score` invocation,
`--dry-run` included (a dry run resolves nothing, so it simply makes no difference there — stated in
`--help` rather than refused, since refusing would second-guess a command the user will re-issue).

### 8.4 What each back end records

| | file mode | environment mode | harness |
|---|---|---|---|
| `roles[0].back-end` | `provider` | `environment` | `harness` |
| provider/rule/layer | present (rule `config` or `flag`) | absent together | absent together |
| `roles[0].harness` | absent | absent | the profile name |
| `roles[0].harness-layer` | absent | absent | `explicit`, `environment`, `repository` or `global` |
| `roles[0].model` | requested | requested | requested, or null where the profile names no `{model}` |
| ledger `run.yaml` | `backend`, `provider`, `model` | `backend`, `model` | `backend`, `harness`, `harness_layer`, `model`, `command` |
| ledger line | `backend`, `model` | `backend`, `model` | `backend`, `model`, `harness`, `harness_layer` |

### 8.5 `--timeout`

`--timeout` bounds **one request**: one item's call under `score` (provider or harness), one
catalogue request under `models`/`configure`. It is not a resolution bound and not a run bound. The
shared flag text becomes "deadline for one model request (30s)"; `score` keeps its own override
(`cmd/score.go:511`).

`--provider` leaves **`score`**: a scoring-time provider choice is `--scoring-model
provider/model`, which states the endpoint and the model in one value, so a second flag for the
provider alone would be a second way to say half of it. `build` never carried the flag
(`cmd/build.go:173-179`), and now needs none. `models`, `configure` and `show-config` keep it: they
select the pool entry to dial or report, which is not a role. This closes the carried item in
`ACTIVE_PLAN.md:71-72`.

### 8.6 The resume command

`scoreResume` (`cmd/score.go:358-364`) appends `--provider` today. It appends **`--scoring-model` and
`--harness`** whenever the invocation carried them, and drops `--provider`. `SPEC.md:430` defines
`resume` as "the command that continues the run": one issued without its harness would continue on the
provider back end against a different model under the same run id — a different judgement wearing the
same name.

**The mode is part of the command too.** An environment-mode run has no flags that name its source: a
resumed shell without the variables falls to file mode, or refuses, and that is the same different
judgement. So where the invocation was environment mode, the printed command carries the mode as
assignments before the program —
`KBASE_API_BASE_URL=… KBASE_DECISION=… KBASE_TEXT=… KBASE_SCORING=… kbase score …` — rebuilt from the
variables this invocation read (and `KBASE_API_KEY_FILE` where it was set), shell-joined by
`build.ShellJoin` so a value with spaces or quotes survives. That prints a base URL and model ids, no
key material: the key is a file, by path, as `show-config` already reports it (`SPEC.md:1002-1004`).
A harness run needs none of this — its profile comes from a file the resumed shell reads. The rule
stays the one `build` already follows: the resume command carries everything the invocation carried
but the bound (`--limit`).

---

## 9. The harness back end

### 9.1 The command contract

One item, one process:

```
<argv[0]> <argv[1:]…>          # the profile's command, {model} and {prompt} substituted
  stdin   the prompt Ask is handed, then EOF — unless the argv carried {prompt},
          in which case the prompt went there and stdin is closed empty
  stdout  the reply: read to EOF, trailing whitespace trimmed
  stderr  captured, bounded, written to the ledger line's reply on a failure and
          nowhere else — never to kbase's stderr, never to the result document
  cwd     <state-dir>/score/<run>/harness/, emptied before each item
  env     the caller's, unmodified
```

- **Who builds the argv, and when.** One probe and two substitutions, each exactly once. At setup,
  `config.HarnessArgv(profile, model)` fills `{model}` and leaves `{prompt}` intact, and
  `harness.New` runs `LookPath` on `argv[0]` — the probe that makes a command that cannot start a
  refusal before the walk. Per item, `(*Command).Item(prompt)` fills `{prompt}` in the one element
  that names it, or keeps the prompt for stdin where none does, and the item's command says which
  (§9.4). The token is gone after substitution, so **stdin-versus-argv is a property of the
  profile**, decided once and never re-examined per item.
- **Substitution is one pass, not a template engine.** `{model}` first, then `{prompt}`, and the
  result is never scanned again: a document that happens to contain the text `{model}` is not
  rewritten, because the prompt is substituted last and into one element. `{prompt}` in two elements
  is refused (the prompt would be sent twice); an unknown brace token is passed through as written.
- **The prompt's edges under `{prompt}`.** An argv cannot carry a NUL: a prompt containing one is that
  item's `failed`, with the exec error in the ledger's `reply`, before any process starts. A prompt
  over `harnessPromptLimit` is refused the same way, so an argv element cannot grow without bound.
  And an argv-borne prompt is visible in the process table for the life of the child — a fact §9.3
  states, since the ledger's token policy covers the prompt's *record*, not a child's argv.
- **The bytes.** `assemble` returns guarded *text* (`internal/score/text.go:255`), `compose` renders
  it through `asks.ScoreRigor`/`ScoreBearing` (`internal/score/run.go:189-197`), and `Ask` receives
  that render (`cmd/score.go:205-209`) — the bytes the ledger's `prompt_sha256` digests and the user
  turn a provider gets. The harness receives exactly that, on stdin or through `{prompt}` and never
  both (§4.6), never the bare text and never a system message of kbase's.
- The reply is the whole of stdout; `score.ParseAnswer` reads it as it reads a provider's reply, so a
  reply carrying prose is `malformed` by the rule that already exists (`SPEC.md:707-710`).
- **No re-issue.** A provider call is re-issued up to the transport attempt count
  (`ARCHITECTURE.md` §12's "3 per call"); a harness call is not. One item is one process and one
  judgement: a non-zero exit or a deadline is that item's failure, and `--resume` is how a user asks
  again. A harness is an agent, not a transport, and three identical prompts would be three answers
  paid for.
- Two bounds, both new `ARCHITECTURE.md` §12 constants: a reply over `harnessReplyLimit` and a stderr
  capture over `harnessStderrLimit`, each failing the item rather than exhausting the run.
- The child runs in its own process group and the group is killed on the deadline or on cancellation
  (the `Setpgid`/`SysProcAttr` split already has the precedent of `cmd/mcp_detach_unix.go` /
  `_windows.go`).
- The ctx handed to the child is the per-item deadline derived from the run's ctx
  (`cmd/score.go:206`), so a SIGINT cancels the child and the runner's existing "cancelled during the
  call records nothing" path (`internal/score/run.go:146-151`) holds unchanged.

### 9.2 The four questions

**1. What the ledger records when a harness answers.** `run.yaml` gains `backend` (either value, so a
provider run's record says so too) and, under a harness, `harness: <profile>`, `harness_layer` and
`command: [<argv>]` — the profile's command with `{model}` substituted and `{prompt}` left as the
token, so the header names how the run was invoked and never the text of one item — beside the
existing `provider` and `model`; `model` is the requested id, empty where none was named. Each
`ledger.jsonl` line gains `backend` and, under a harness, `harness` and `harness_layer`, beside the
existing `model`, `prompt_sha256` and timings, so every line says what answered it (I12) and which
layer's file named it. `ledgerFormat` stays `1`: the added keys are additive, an older reader ignores
them and reads every status correctly.

`run.yaml` is written once, at `Create`; `Resume` reads it and never rewrites it
(`internal/score/ledger.go:133-158`, and its comparison at `:148` is unchanged). So a resumed run's
header names the **first** invocation's back end and model, and a mixed run's header does the same:
the header is the run's opening, and each line is the truth per attempt. That is stated in
`SPEC.md` §8 rather than repaired, because a header rewritten mid-run would record two states of one
field.

**2. How items stay isolated.** *One process per item, created for it and reaped before the next.* No
session, no conversation, no inherited state: the process is the isolation unit, and the ledger's line
is the only thing that crosses from one item to the next. The working directory is one directory the
run owns, **emptied before each item** so nothing a command wrote is visible to the next — the same
directory, not a new tree per item, so the run's footprint does not grow with the set. What kbase
cannot prevent is stated in §9.3.

**3. Item's failure or the run's.** An item's, with one exception:
- the command cannot be started at all — `LookPath` fails at setup, before the first ask — is a
  **refusal** (`check: harness`, `key: --harness`), because nothing was asked;
- a non-zero exit, a deadline, an over-limit reply or a start failure after the walk began is **that
  item's `failed`**, with the exit code and the bounded stderr tail in the line's `reply`; the run
  goes on and the next item is asked. `counts.failed` and `failures` (`SPEC.md:755-759`) carry them as
  they carry a provider's.

There is no consecutive-failure stop: one rule for one behaviour, and a misconfigured command is
found by the `LookPath` probe or by the first line of the ledger, not by a heuristic.

**4. Requested or observed model.** **Requested only** (I7). No observed id exists to name: kbase's
transport decodes `choices` and `usage` and no `model` field (`internal/model/http_stream.go:264-270`),
and the harness protocol has one channel, stdout, which carries the answer alone. The document's
`model` is therefore the id this invocation asked for; `null` under a harness with no
`--scoring-model` and a profile naming no `{model}`, which is the one case where a reader can see that
the model was the harness's own choice. No `model-reported` key exists, because nothing would fill it.

### 9.3 What the harness back end cannot guarantee

A harness is an agent: a command with tools can read the filesystem, and a command run for `score`
can read the register it is being asked about — the closed text's four tokens (`- rigor:`,
`- strength:`, `(bearing …)`, `(strength …)`, `SPEC.md:705-706`) are what the guard keeps out of the
prompt, and a command that reads the KB can put them back. kbase cannot sandbox a child on either
supported host, so this is stated rather than enforced:

- A harness also brings its own system prompt and its own tools — the commented `opencode` entry does,
  and the seed comment says so; that is why it ships commented (R7) — and kbase can remove neither.
  Two different things follow, and they are not the same defence: the **input guard** refuses a
  prompt that would carry the register's score or strength tokens (`SPEC.md:705-706`), and
  `ParseAnswer` refuses a reply that carries no single value as `malformed`; neither says anything
  about what a profile's own instructions ask for, nor about what a tool-enabled answer narrates
  around the value. A profile is expected to be a one-shot ask; the ledger's `harness`,
  `harness_layer` and `command` are how a reader knows which command answered.
- An argv-borne prompt (`{prompt}`) is visible in the process table while the child runs, and the
  child's own environment is the caller's. The ledger's token policy covers what kbase *records*
  (`SPEC.md` §10's guard is about the prompt's text), not what a sibling process can read.
- `SPEC.md` §8 gains the isolation-contract and system-prompt sentences, and no pass reads the other
  pass's answer (`ACTIVE_PLAN.md:96`) remains a rule of the runner, which is unchanged — the
  harness sees one item's text and nothing else *from kbase*.

### 9.4 The exec monopoly, and the trust it rests on

**The trust this rests on, now that a repository may ship a profile (R6).** The exec surface is
reachable by repository content, and the design does not pretend otherwise. What follows from it, and
what does not:

- **It is the trust the pool already asks for.** A repository's `providers.toml` already chooses where
  the text of every letter ask travels and from where the answer comes; a program chosen by the same
  layer is not a new class of trust, it is the same one, which is why R3's exclusion was the
  asymmetry R6 removes. A user who clones a repository and runs `kbase build` has already accepted it.
- **No verb runs a profile it was not told to run.** `build`, `models`, `configure`, `show-config` and
  every query name a harness nowhere; only `score --harness <name>` executes one, and the name is the
  user's shell argument. A repository cannot make a run happen; it can only supply the command that
  the user's own `--harness` names.
- **The source is announced, never silent** (I9): the stderr line, the record's `harness-layer` and
  the ledger's `harness_layer` mean the user, the result document and the run's record all say which
  file named the command. A profile is not a default and never becomes one.
- **What is not claimed:** that a repository profile is safe. It is executable code chosen by whoever
  wrote the clone; the design's promise is disclosure and explicit selection, not inspection. A user
  who does not want that sets `--config-dir`, which reads one directory and no repository.

`internal/harness` is a new package and the **fifth exec owner**, of a program the user's
configuration names. `cmd/execpolicy_test.go:22`'s `execOwners` gains it with a sentinel for that
binary, and the per-site check (`:68-77`) exempts it, because the binary is not statically knowable —
which is the price of the capability, named rather than hidden. What remains enforced: only
`internal/harness` imports `os/exec` for this purpose, and a unit test pins that the argv `Item`
builds is the profile's with `{prompt}` replaced in the one element that names it and untouched
elsewhere — and that `New` probes the program the profile names, once, before any item.
`CONVENTIONS.md:81-84` gains one sentence naming the fifth owner and why its binary is dynamic.

`internal/harness`'s surface is deliberately tiny and knows nothing of kbase:

```go
type Command struct { /* the argv, and the prompt if it travels on stdin */ }
// New is the setup probe: LookPath on argv[0], and no execution.
func New(argv []string) (*Command, error)
// Item is one item's command built from the probed argv: {prompt} filled in the
// element that names it, or held for stdin where none does.
func (c *Command) Item(prompt string) (Command, error)
// Ask runs the item's command with dir as its working directory, returns stdout.
func (c Command) Ask(ctx context.Context, dir string) (string, error)
```

`Item` is per item and `New` is not: the profile is probed once, because a command that cannot start
is a setup refusal (§9.2 #3), while `{prompt}` exists only once the item's text is composed. The
`Command` `Item` returns carries the prompt itself, so `Ask` cannot be called with the wrong one and
cannot write a prompt the argv already carried.

It takes no `config` type, no endpoint, no KB path. It takes a `log.Logger` for one thing — reporting
a killed process group — because a kill that leaves no record is not debuggable, and `log` is
kbase's one logging seam.

---

## 10. Module and type skeleton

**Packages and responsibilities.**

| Package | Owns | May import |
|---|---|---|
| `internal/config` | the grammar (`Reference`), `Role`, `Mode`, `BackEnd`, `Endpoint`, `ModeOf`, `Resolve`, `Select`, `HarnessArgv`, `Refusal`, the harness profiles, the templates and hints | `atomicfile` (as today), the TOML and YAML packages |
| `internal/harness` | one command, one prompt, one reply, one process group | stdlib, and `log` alone of kbase |
| `internal/model` | the transport; `Endpoint`, `Client`, `*HTTPClient` + its new `Close` | unchanged |
| `internal/kb` | the register and its vocabulary: the fault ids, the two weight tables and a pass's value from a charge list (I14); the band ladder | unchanged — no kbase package |
| `internal/score` | the runner, the closed text **and the passages it is made of**, the parse (a reply read against that text), the ledger | unchanged, plus the ledger's charges, the header's ask digest, the re-pricing's record and the passage extents `assemble` returns |
| `cmd` | the registry, the flags, the back-end adapters, the documents | as today |
| `internal/build` | the walk | `model` through its `Provider`, as today |

**Dependency directions.** `cmd` → `config`, `harness`, `model`, `score`, `build`. `harness` → stdlib
and `log`. `config` → no kbase package but `atomicfile` (`internal/config/write.go:16`); it never
imports `model`, `result` or `log`. `score` → neither `model` nor `harness` (I8). `build` → `model`
and `asks/call`, never `harness`.

**Negative constraints — what must not appear.**

- In `internal/config`: a client, a dial, an `os/exec`, a `result` type, a verb's name, or a function
  that resolves more than one role into one answer.
- In `internal/harness`: a provider, an endpoint, a KB path, a config type, a prompt template. It
  receives the prompt; it never composes one (`CONVENTIONS.md:162-166`).
- In `cmd`: a `model.NewHTTPClient` call outside the registry; a second resolution of any role; a
  `--provider` flag on `score`.
- In `internal/score`: any knowledge that a harness exists. It gains the ledger's new fields, no
  branch. Nor a second spelling of a fault id or a weight, a parse that does not read the closed
  text, or a float subtraction of weights (I14).
- In `internal/kb`: a parser, a prompt, a ledger or a verb. The vocabulary and the table, and no
  mechanism that reads a reply.
- In a scoring template: a weight, a maximum, a band threshold, a decimal, or an id its pass does
  not accept. §10's `asks` assertion is the check on the last of those.
- In any document: a top-level `provider` for a verb whose endpoints are per role (I6).

**Deliberately left to the coder** (genuine implementation freedom, not a buried assumption): the
registry's internal keying beyond "entry name, else base URL"; the exact wording of the refusals
beyond the key, allowed set and remedy; whether `clients` is a struct or a closure-returning
constructor; the stub command's shape in the integration fixture; whether `harness.Command.Ask` reads
stdout and stderr concurrently with a single `cmd.Wait` or through an `io.Pipe` pair; how the profile
map is held after the load.

---

## 11. Acceptance criteria

Per `ACTIVE_PLAN.md` outcome. Each is a behavioural outcome; how it is verified is the work
package's to choose.

**Outcome 1 — resolution is not connection.**
1. A `scoring` role naming a provider the pool does not hold refuses before any item is asked, in
   file mode and on the provider back end, with `check: provider`, `key: models.scoring`, and the
   pool's entry names as `allowed`.
2. `show-config` reports, for every role, whether it resolves and what it lacks — with `detail` where
   it does not; it makes no network request, and an unreadable key file is a diagnostic rather than
   something it discovers by dialing.
3. A build whose `decision` and `text` roles name two different providers asks each on its own
   provider, reports both in `roles`, and a fault on either names that provider alone.
4. Every client a verb opens is closed before the verb returns, on `done`, `refused`, `failed` and
   `cancelled` alike.
5. `kbase score --help` lists `--scoring-model` and `--harness` and no `--provider`; passing
   `--provider` is refused — `check: usage`, exit 1, cobra's unknown-flag message in `detail`, since
   cobra's error reaches `exitStatus`, which writes the document (`cmd/main.go:149-165`). No
   `FlagErrorFunc` is added, and `--provider` stays on `models`, `configure` and `show-config`.

**Outcome 2 — per-role references, and the two modes.**
6. A `config.toml` holding top-level `provider` loads, and a file-mode run says once on stderr that
   the key is superseded; a bare `[models]` value is then refused, naming `models.<role>`, stating
   the `provider/model` form and carrying the `configure` remedy — and the key is never used to
   resolve it, even when it names the entry that would work.
7. No `build` or `score` document carries a top-level `provider`, `provider-rule` or `provider-layer`;
   both carry `roles`, one record per role in role order, key order as §7.1.
8. A model id containing `/` (`openrouter/anthropic/x`) resolves to provider `openrouter` and model
   `anthropic/x`.
9. `kbase configure --provider … --model-map …` rewrites an old-grammar file in one call: `provider`
   is gone, all three references are written, and every byte it was not asked to change is byte-
   identical.
10. **Environment mode works with no configuration directory at all**: `KBASE_API_BASE_URL`,
    `KBASE_DECISION`, `KBASE_TEXT` and `KBASE_SCORING` alone configure a `score` run and a `build`,
    and `show-config` reports every role's `back-end` as `environment`. Both run legs are LIVE (a
    provider for `score`, the local inference endpoint for the build) and are named as such in their
    recipe's doc; the hermetic leg is `show-config`'s document.
11. An environment role variable without `KBASE_API_BASE_URL` refuses, naming the variable set and
    the one missing; `KBASE_API_BASE_URL` alone against a `[models]` file refuses with one item per
    side.
12. An environment role variable alongside a `[models]` value refuses with one item per side, and
    nothing is asked.
13. `KBASE_API_KEY_FILE` alone changes no mode: with a file-mode configuration the run proceeds and
    the selected entry's key is replaced; in environment mode it replaces the environment endpoint's
    key; with none set in either mode and no entry key, no key is sent and no warning is printed.

**Outcome 3 — the `scoring` role.**
14. With the scoring role absent — `[models] scoring` in file mode, `KBASE_SCORING` in environment
    mode — `kbase score` refuses naming that key, in every mode and with `--harness`, even where
    `--scoring-model` names a model: the flag overrides a configured role and never substitutes for
    one. `--dry-run` without the role still lists the set and refuses nothing.
15. A scoring model different from `text`'s is asked: the ledger's lines and `roles[0].model` both
    name it, and `text`'s model appears nowhere in what the run asked — not in the role records, the
    ledger lines, or the harness's command. An environment-mode resume states the variables it read
    (§8.6), so a `KBASE_TEXT` the invocation carried may name that model there; that is the
    invocation recorded, not a role asked.

**Outcome 4 — the two flags.**
16. `--scoring-model provider/model` in file mode asks that provider and model, and `roles[0]` states
    `provider-rule: flag`.
17. `--scoring-model` under `--harness` is taken verbatim: a value containing `/` is not split, and
    `roles[0].model` is the value as written; in environment mode it is a bare id.
18. `--harness` with a profile whose command names `{model}` and no `--scoring-model`, where the role
    names a model, asks the role's model part; with the role unset it refuses rather than asking.
19. `--scoring-model` naming a provider the pool lacks, in file mode, refuses before the first item,
    naming `--scoring-model` and listing the pool.
20. `--harness NAME` naming a profile the repository's `harnesses.toml` declares loads and runs it;
    the run writes one line on stderr naming that file and the layer, `roles[0]` carries
    `harness-layer: repository` and the ledger's header and lines carry `harness_layer`, and a profile
    from the global layer or a named directory runs with no such line. A name no layer holds refuses
    before the KB is opened, listing the loaded names.
21. A harness run with `KBASE_API_BASE_URL` and role variables set in the environment succeeds: the
    mode is a fact about the provider back end and is not consulted.

**Outcome 5 — the harness back end.**
22. Every line of a harness run records the back end, the profile and the requested model; `run.yaml`
    records the argv with `{model}` substituted and `{prompt}` left as the token.
23. Two items of a harness run are answered by two processes, and a file a command leaves in the
    working directory is not visible to the next item.
24. A harness exiting non-zero fails that item only: the line carries the exit code and the stderr
    tail, the next item is asked, and the run's outcome is `failed` with that item in `failures`.
25. A command that cannot be started refuses before the walk, `check: harness`.
26. A harness run's document carries no observed-model key and `model` is the requested id, `null`
    only where §9.2's fourth answer says so.
27. `--dry-run --harness NAME` lists the set, asks nothing, records nothing and succeeds.
28. A run stopped by `--limit` or cancelled, on either back end, carries a `resume` command that
    continues on the same back end and model; an environment-mode run's command carries the mode's
    variables, so re-issuing it in a shell with none of them set asks the same endpoint with the same
    models rather than refusing or falling to file mode (§8.6).
29. A profile whose command carries `{prompt}` receives the prompt there and nothing on stdin; one
    that does not carries it on stdin — asserted with two stub profiles in one hermetic run, each
    leaving its own artifact, so the contract is tested without a model call. The live seed's own
    answer to that item is criterion 31's.

**Seeding (R4, R5, R7).**
30. A fresh `configure --create` in an empty configuration directory writes three files —
    `providers.toml`, `harnesses.toml`, `config.toml` — in that order; `harnesses.toml` holds a live
    `[claude]` with its `command`, and a commented `[opencode]` whose text is the verified command of
    §4.6 and whose comment states the limitation (R7); `show-config` lists `claude` under `harnesses`
    and `opencode` under neither `harnesses` nor `providers`, and the layer record's
    `harnessesExists` is true; `providers.toml` still holds one live pool entry, so `models` resolves
    a provider without a choice; `kbase score --harness opencode` refuses, `check: harness`,
    `key: --harness`, `allowed` `[claude]`.
31. Against that file and a configured scoring role, `kbase score --pass rigor --limit 1 --harness
    claude` completes the item on claude's own model, recording `backend: harness`, `harness: claude`
    and the `command` of criterion 22. (LIVE — needs `claude` installed and authenticated; the
    hermetic leg checks the file and the refusal, and says in its doc that the run leg is live.)
32. `configure --create` over an existing `harnesses.toml` writes no entry into it and reports the file
    unwritten; a seed a user deleted stays deleted.

**Outcome 6 — the contract moves.** §13's documents state the behaviour above, and every sentence
that changed is backed by the command or run that produced it, recorded in the commit body. The
census, each returning only historical mentions where it returns anything:

```
# The reader-facing examples carry no live `provider =` line; SPEC's grammar block is the one
# place the key is spelled, and it must carry the superseded annotation beside it.
rg -n '^\s*provider\s*=' README.md USER_CONVENTIONS.md INTEGRATING_KBASE_VIA_MCP.md   # none
rg -n -A2 '^\s*provider\s*=' SPEC.md                                                 # the grammar, annotated
rg -n 'KBASE_DECISION.*override|overrides the corresponding field' --glob '*.md'      # none
rg -n '"show-config.inference"' cmd/resultkeys_test.go                                # no match (retired block)
rg -n 'KBASE_(DECISION|TEXT|SCORING)' --glob '*.md'                                  # every hit says environment mode
# Profiles are top-level tables; no reader-facing document spells the nested form.
rg -n '\[harness\.' SPEC.md ARCHITECTURE.md README.md USER_CONVENTIONS.md INTEGRATING_KBASE_VIA_MCP.md  # none
```

**Non-functional.**
33. The two bounds hold, and are observable at the item: a stub writing more than
    `harnessReplyLimit` to stdout, or more than `harnessStderrLimit` to stderr, fails that item with
    the matching over-limit reason and the run continues; kbase reads no further than each bound.
34. No new module appears in `go.mod`, and no new MCP tool appears in `tools/list`.

**The rubric landing — its own list, keyed to
[`RIGOR_AND_BEARING_SCORING_SCHEME.md`](RIGOR_AND_BEARING_SCORING_SCHEME.md) rather than to an
outcome.** Each names the command or test that can fail on it — or, where no command can, the pass
that reads for it; the package chooses the test, not the claim.

35. A charge line is an answer and a decimal is not: `flt-overreach: <a span of the closed text>` is
    read as one charge carrying the id, the passage it was found in (`title`/`statement` for
    `rigor`, `source`/`target` for `bearing`, and `both` where a span lies at both ends of its pair
    and no longer span is unique to one) and the offset of its first match; `0.75`, `*pending*`
    written as a value, `75%` and `1` are `malformed`. — `just test-pkg ./internal/score` (the parse
    table replacing `TestParseAnswer`).
36. `none`, alone, is the empty charge list and lands the pass's maximum — `0.95` for `rigor`, `1.00`
    for `bearing`; `none` beside a charge line is `malformed`. — `just test-pkg ./internal/score`.
37. `flt-pending`, alone, is one record carrying the id and no citation, and lands `*pending*`; the
    reply beside charge lines reads as §12's D1 ruling. — `just test-pkg ./internal/score`.
38. An id outside the pass's set is `malformed`, and the refusal names the id and the pass's ids as
    `allowed`; an empty or whitespace-only reply is `malformed`, writes nothing and records the
    reply. — `just test-pkg ./internal/score`.
39. A citation not found in the closed text — including one crossing a passage boundary and one in an
    edge's `## Context` — is `malformed`, naming the id and the span. — `just test-pkg
    ./internal/score`.
40. Duplicates, overlaps and shared spans read as the scheme states: one id with one span twice
    collapses to one charge; two spans of one id that overlap without coinciding is `malformed`,
    naming both lines; two ids on one span are both charged with the offset shared. — `just test-pkg
    ./internal/score`.
41. The value equals the table for the recorded ids: every `done` line's landed value re-derives
    exactly from `faults[].id` and the pass's table, for both passes. — `just test-pkg
    ./internal/score`.
42. The reachable values are the scheme's — eleven for `rigor` over `[0, 0.95]`, six for `bearing` —
    enumerated from the table over charge lists up to the count that saturates it; no other value is
    produced. — `just test-pkg ./internal/kb`.
43. The ledger's line carries `faults` (id, passage, citation, offset) in the order written beside
    `reply`, with `[]` only for `none`; `run.yaml` carries `ask_sha256` — the digest of its pass's
    prompt rendered for an empty subject — written at `Create` from the header's `Pass`, so every run
    this build writes has one, and read and never rewritten by `Resume`. — `just test-pkg
    ./internal/score`; `just test-integration-results-fixture`.
44. A changed weight re-prices and asks no model: from a recorded ledger, every value re-derives from
    `faults[].id` under the running table with the `Ask` seam never called, and a resume whose landed
    values move lands them through the write path — the ledger line's `value` and the register's
    value for that claim both carry the running table's output, so criterion 41 holds of every line
    after it and no claim reads a figure its own line contradicts — and the run's directory gains a
    record saying the run was re-priced, naming each line that moved with the value it held and the
    value it took, while `run.yaml` stays exactly as `Create` wrote it and `ask_sha256` does not move.
    A resume whose re-derivation moves nothing records nothing and lands nothing. — `just test-pkg
    ./internal/score`.
45. The ladder's bottom rung reads `unsubstantiated` — slug, status phrase and label — at the
    standing cut; `show`'s `build_band`/`build_status`, `stats`' band record, and the graph's legend
    and tooltip carry it; and after `kbase refresh` over a KB written before the change, `verify`
    reports no build-status drift and `stats` counts its bottom-rung claims under the new row. —
    `just test-pkg ./internal/kb ./internal/index ./internal/sheet ./internal/query`;
    `just test-integration-mcp` (the `stats` transcript).
46. `--dry-run` still asks nothing: no call, no charge record, `run.yaml` and `items.jsonl`'s pending
    lines alone. — `just test-integration-results-fixture` (its `score --dry-run` leg).
47. Each template names its pass's six ids and `flt-pending`, the line form, `none` and the citation
    rule, and carries no number, weight, maximum or band; a test asserts the `flt-[a-z0-9-]+` tokens
    each template names are exactly its pass's set, and that neither template carries a decimal. —
    `just test-pkg ./internal/asks`.
48. `just build`, `just test` and `just edit-gate` are green, and no module is added to `go.mod`.
49. **LIVE**, after the templates' review: one item asked by a real model through the score leg lands
    a value whose ledger line's charges re-derive to it. — `just
    test-integration-build-live-arxiv` (its `score-env` capture).
50. A resume after an update continues and answers no question about versions: a `run.yaml` whose
    `ask_sha256` differs from the digest the running pass's template renders — or which carries none,
    as every run written before the field does — is resumed, every item still pending is asked under
    the running template's ask, and any value the update's table moves lands as criterion 44 states.
    `run.yaml` is not rewritten, so it still records the ask the run began under and a reader can see
    a run span an update; nothing refuses and nothing asks the user to choose. — `just test-pkg
    ./internal/score`.
51. The render path has no size-dependent element: a rendered prompt is a function of its template
    alone — no gate, no band keyed on the item's length or shape — so one `ask_sha256` certifies one
    instruction. No command can fail on this and none is named: it is a property of the templates'
    text, read in the prompt-engineer's pass that criterion 47's gate already requires, and the spec
    states the rule with this reason so that a later edit cannot undo it unseen.

---

## 12. The work packages

**Settled by three prompt-engineer passes; the table is the dispatch plan.** The order is
the dispatch order, `WP1 → WP2 → WP3 → WP4 → WP5 → WP6`, and the tree is green at each boundary.
`WP1`–`WP3` are the `go-coder`'s; `WP4` is the prompt-engineer's and then the `go-coder`'s; `WP5` the
`tech-writer`'s — it owns the contract documents (`ACTIVE_PLAN.md:88-93`) — and `WP6` the
coordinator's, not dispatched. **The rubric landing is inserted as WP4 and the last two packages
renumber with it**: the contract text is WP5 and the closing records WP6, and every "hands to WP4" in
the blocks below names WP5.

**Two clauses stand for every row.** A package's files are the sources it changes, **and the tests in
the packages it names**; no row lists test files one by one. And each package's commit body carries
**one line per §13 row it hands over** — `section — the as-built fact — backed by <command or run>` —
which is `WP5`'s only input (`AGENTS.md`, "the contract moves with the work").

**Why six.** The package that was to own "the documents" cannot run before the resolution it
documents: `build`'s and `score`'s `roles` records and `show-config`'s `roles`/`mode` are the
resolution's own output, and a package that first kept the old single-endpoint keys would state a
fact the design says cannot be stated ("a build with two providers cannot state one provider", §7.3).
So the document work landed with the packages that force it — WP1 (§7.1/§7.2's role records and mode),
WP2 (`configure`'s third category), WP3 (`harnesses`, `layers.harnessesExists`, the ledger's
`harness_layer`) — and the table has no mechanical coder package: every dispatch here decides
something, and the closing bookkeeping is the coordinator's. The sixth, WP4, is the rubric landing: its
ask, its parse, its ledger record and the ladder's words are one contract and no document, so it is a
code package of its own — placed after WP3, whose harness feeds the same parser and ledger, and before
WP5, so the writer changes §8 and the ladder once rather than twice. That insertion is the only
renumbering; the table's rows and the blocks below stay in dispatch order, and each §13 row is handed
to whichever package now owns it.

| WP | outcome(s) | seat | rating | model | files | criteria (§11) | gates | hands to WP5 |
|---|---|---|---|---|---|---|---|---|
| 1 | 1, 2 | `go-coder` | expert | `inherit` | `internal/config` (`inference.go`, `config.go`, `layer.go`, `report.go`, `strict.go`, `paths.go`), `cmd/provider.go`, `cmd/{build,score,models,configure,show-config}.go`, `cmd/build_test.go` (new), `internal/model/http_client.go`, `internal/build/build.go`, `cmd/resultkeys_test.go`, `justfile` (`_test-integration-config`'s `inference()` helper and the legs `show-config`'s document change invalidates) | 1 scoring-refusal (with `ModelMap.Scoring` modelled), 2 show-config, 3 two providers, 4 clients closed, 5 the `--provider` removal and its `check: usage` refusal, 6 superseded key, 7 role records, 8 slash id, 10 the build and `show-config` half, 11 partial env, 12 mixed, 13 key file | `just test-pkg ./internal/config`; `just test-pkg ./cmd`; `just test-pkg ./internal/build`; `just test-integration-config`; `just test-integration-results-fixture` (regression only); `just edit-gate` | §13's §7 rows; §13's §9 rows for the grammar, the two modes, the preflight and the `provider`-superseded clause; §13's §8 synopsis (its `--provider` removal); the README/USER_CONVENTIONS/INTEGRATING row for the modes, the flags and the documents' shape; ARCHITECTURE §6, §7, §11 |
| 2 | 3, 4 | `go-coder` | expert | `inherit` | `internal/config` (`inference.go`, `write.go`, `template.go`, `hints.go`, `report.go`), `cmd/{configure,score}.go`, `cmd/show-config_test.go` (its scrub helper), `cmd/resultkeys_test.go`, `justfile` (`_test-integration-config`'s scrub list gains the fifth variable) | 9 configure's three references, 10 the score half (`KBASE_SCORING`), 5 `--scoring-model`'s `--help` line, 14 the file and environment modes, 15 scoring ≠ text, 16 `--scoring-model` in file mode, 19 `--scoring-model` naming a missing provider | `just test-pkg ./internal/config`; `just test-pkg ./cmd`; `just test-integration-config`; `just test-integration-results-fixture`; `just edit-gate` | §13's §8 row (the category); §13's §9 rows for `models.scoring`, the environment's role names, `configure`'s key set and `SPEC.md:405-408`'s enumeration; the README/USER_CONVENTIONS/INTEGRATING row for the scoring role |
| 3 | 5 | `go-coder` | expert | `inherit` | `internal/harness/*`, `internal/config` (`harnesses.go`, `template.go`, `write.go`, `layer.go`, `paths.go`, `report.go`), `internal/score/ledger.go`, `cmd/{score,configure,show-config}.go`, `cmd/resultkeys_test.go`, `cmd/execpolicy_test.go`, `test-data/fixtures/` (the stub profiles), `justfile` (the seeding, harness and bounds legs) | 5 `--harness`'s `--help` line, 14 the harness clause, 17–18 the flag under `--harness`, 20 repository profile, 21 harness with env set, 22 ledger, 23 isolation, 24 non-zero exit, 25 cannot start, 26 no observed model, 27 dry run, 28 resume, 29 `{prompt}`/stdin, 30 seeding, 31 live `claude`, 32 create over existing, 33 bounds, 34 no new module or tool | `just test-pkg ./internal/harness`; `just test-pkg ./internal/config`; `just test-pkg ./cmd`; `just test-integration-results-fixture`; `just test-integration-config`; `just edit-gate` | §13's §8 harness sentences, ledger and `written` order; §13's new §9.3 row; §13's §9.7 row (`harnesses[].layer`); §13's §11 row; §13's §7 rows for `layers`, `harnesses` and `harness-layer`; ARCHITECTURE §11, §12; CONVENTIONS' exec sentence |
| 4 | — (the rubric landing) | `prompt-engineer`, then `go-coder` | expert, both | `inherit` | `internal/asks/prompt-templates/` (`score-rigor.tmpl.md`, `score-bearing.tmpl.md`, `fragments/score-system.tmpl.md`); `internal/kb` (`faults.go` new, `schema.go`'s last rung); `internal/score` (`answer.go`, `run.go`, `text.go`, `ledger.go`), `cmd/score.go` (the write seam `Resume` takes, and the refresh a re-price owns); `internal/asks/score.go` (the constants a template stops using); `internal/sheet/testdata/adjagent-sheet/*`, `test-data/fixtures/mcp/stats.jsonl` | 35–51 | `just test-pkg ./internal/kb`; `just test-pkg ./internal/score`; `just test-pkg ./internal/asks`; `just test-pkg ./internal/index`; `just test-pkg ./internal/sheet`; `just test-pkg ./internal/query`; `just test-pkg ./cmd`; `just test-integration-results-fixture`; `just test-integration-mcp`; `just edit-gate`; LIVE `just test-integration-build-live-arxiv` (its `score-env` leg, after the templates' review) | §13's §8 row, §13's §10 row, the ladder's words, the README/USER_CONVENTIONS row, ARCHITECTURE §6 and §11, CONVENTIONS' scoring sentence |
| 5 | 6 | `tech-writer` | expert | `inherit` | `SPEC.md`, `ARCHITECTURE.md`, `CONVENTIONS.md`, `README.md`, `USER_CONVENTIONS.md`, `INTEGRATING_KBASE_VIA_MCP.md`, `ROADMAP.md` | Outcome 6's census, plus WP4's §13 rows landed | `just edit-gate` (fmt and vet — it cannot fail on content); its content gate is **a report line per §13 row**, `section — landed — backed by <command>`, plus `git diff --stat` over the six contract files | — it is the hand-off |
| 6 | — | coordinator | not dispatched | — | `ACTIVE_PLAN.md` only | — | `just checkpoint`; `just test-integration` | — |

**WP1 — resolution, the registry, and the documents resolution forces.** Implements §4.1–§4.5, §5,
§6, §7.1–§7.3, §8.5's `--provider` removal, §8.6's flag half, and §4.2's `models.scoring` **modelled**
(`ModelMap`'s field and its merge) without resolving or writing it — the field is what makes criterion
1's refusal a `provider` item rather than a strict-load failure on an unknown key.
*Boundary.* Nothing resolves but `config.ModeOf`, `config.Resolve` and `config.Select`; `dial`,
`resolveProvider`, `resolution`, `selectProviderName` and `unresolvedProviderError` are gone, and
`rg -n 'resolveProvider|providerOptions\.dial' cmd internal` returns nothing; the five verbs answer
with `roles`, `show-config` with `roles` and `mode`; `Loaded` exists without its `Harnesses` field,
which comes with the file (WP3); `_test-integration-config` reads `roles`, its `inference()` helper
goes, and its `KBASE_TEXT`-over-files leg becomes the mixed-mode refusal (R2), so **this package
changes the recipe**. Criterion 7's build half is proved by a new
`cmd/build_test.go` — nothing today asserts a build's document, `resultdocs_test.go`'s build legs being
`--no-inference` and a refusal. `just test-integration-config` is the gate that can fail here;
`test-integration-results-fixture` is regression only (no `build` and no `show-config` leg, and its one
`score` leg is `--dry-run`, where §7.2 says `roles` is absent).
*Must not.* Resolve a role twice, or keep a second selection rule; resolve or write `scoring` — WP2
does; add a harness command or profile; touch a contract document; leave `--provider` on `score`;
declare `Profile`, `Harnesses` or `HarnessesExists`.
*Hands to WP5.* As the row lists, plus `ARCHITECTURE.md` §7's resolution paragraph.

**WP2 — the `scoring` role and the two flags' provider half.** Implements §4.2's third key in use,
§4.7, §8.1, §8.2 (file and environment readings), §8.5.
*Boundary.* `[models] scoring` resolves in both modes and `KBASE_SCORING` in environment mode;
`configure` requires three categories and writes three references, and an unassigned `scoring` refuses
only where the file has none to keep (§4.7); `score`'s `roles[0].role` is `scoring`; absence refuses in
every mode; environment mode configures a score run (criterion 10, its run leg LIVE and named as
such); `_test-integration-config` scrubs the fifth variable wherever it exercises the environment;
`just build` is clean.
*Must not.* Add a harness profile or `--harness`; change `show-config`'s `harnesses`; alter the
ledger; touch a contract document; invent a `KBASE_SCORING` path outside §4.5's mode rule.
*Hands to WP5.* As the row lists, plus `SPEC.md:405-408`'s enumeration and the scoring role's
`README` sentences.

**WP3 — the harness back end.** Implements §4.6, §8.1's harness bullet, §8.2's harness reading, §8.6's
mode assignments, §9 in full, and R7's seed.
*Boundary.* A run through `--harness <name>` asks one item per process and lands it; `harnesses.toml`
loads, layers over the repository and global files, and is seeded by `configure --create` — wired in
`writeConfiguration`, which is why `cmd/configure.go` is this package's file; the exec policy names the
fifth owner; `show-config` lists `harnesses` and its layer key; `--harness opencode` on a fresh file
refuses with `allowed: [claude]`; 29's two stub profiles, 30's seeding and 32's create-over-existing
legs are hermetic. Proved by its gates; 31's run leg is LIVE and excluded, as the recipe's doc says.
*Must not.* Change a seed's command — `claude` live, `opencode` commented with its limitation (R7);
add a prompt template (none is added; if one is, `ACTIVE_PLAN.md:91-92` applies); teach
`internal/score` that a harness exists; write a contract document; declare a type WP1 owns — it adds
`Profile`, `Harnesses` and `Loaded`'s field, and nothing else to WP1's types.
*Hands to WP5.* As the row lists, plus the `written` order (`providers.toml`, `harnesses.toml`,
`config.toml`) and `CONVENTIONS.md`'s exec sentence.

**Decisions WP4 needed, all now settled.** The four that stopped a coder were answered by the owner
and are written into the scheme, which is the normative source; what follows records each answer, and
the remaining `## Not settled` entries stay carried below, the landing shaped so each can move later
without re-asking a model.

- **`flt-pending` beside charge lines is read.** It is the judgement; the charge lines beneath it are
  parsed into `moot`, priced nowhere, and kept, so a hedging model is visible in the record without
  becoming a judgement (scheme, `## flt-pending`, `## The ledger`).
- **The span floor is one character of text.** An empty or whitespace-only citation is `malformed`;
  anything else is accepted, because a good citation is often tiny (scheme, `### The citation`).
- **A failed attempt records that it failed, not what it said.** `status: failed` and the reason, no
  charges and no text; a parsed reply keeps its text (scheme, `### Malformed replies`, `## The ledger`).
- **The change of words moves `kb-format` to `2.0.1`.** `SPEC.md:1019` defines a patch as the bytes a
  writer produces changed with their meaning not: the strength parenthetical is display text and the
  value is unchanged. The alternatives below are the record of what was weighed.

**Settled, and so not a decision here: what the run records.** The scheme's `## The ledger` rules it and
its `## Not settled` no longer asks it. The run records the **ask**, not the table's identity:
`ask_sha256` in `run.yaml` is the digest of the pass's prompt rendered for an empty subject, and the
empty input is what makes it cover every fixed byte of instruction and nothing that varies. Two
consequences follow, and §11's criteria 43, 44, 50 and 51 are where they land on the code:

- **A changed weight is not a changed judgement.** The model never sees weights, so a weight change
  cannot change a charge set: values are re-derived from the recorded ids under the running table, and
  the run records that it was re-priced.
- **A changed ask is not a decision to put to the user.** Whoever updated gets the new behaviour;
  refusing would make them adjudicate a version boundary they already crossed. A resume whose
  `ask_sha256` differs continues under the running template's ask, and the header keeps the digest it
  began under, so the record says which instruction answered which judgement and the corpus that
  compares models and versions can read a run that spans an update. Re-pricing is the same answer for
  the table: it lands what moved (scheme, `## The ledger`, "an update applies its behaviour").

**D1 — `flt-pending` beside charge lines: read, or refused.** The scheme says the extra lines are
surplus and, in the same breath, that the case is unsettled.
*Reading A — recorded as `[flt-pending]`, the rest surplus.* Costs no retry; a hedge leaves a value
unknown and visible, and penalising the hedge pushes a hedging reader towards charging faults instead —
the worse error.
*Reading B — `malformed`.* `flt-pending` and a charge are contradictory claims about the same text,
which is exactly why `none` beside charges is refused; the item is re-asked on `--resume`.
*Cost.* A writes `*pending*`, and a pending bearing excludes its edge while a pending rigor leaves its
derivation unknown. B spends a second call on a reply that was already an answer, and the value arrives
a retry later.

**D2 — the span floor: three words, or none.** The design recommends a floor of three
whitespace-separated words after normalisation; the scheme's parse contract does not carry it.
*Reading A — the floor.* A span shorter than three words after normalisation is `malformed`.
*Reading B — no floor.* Any span the lookup finds is a citation, however short.
*Cost.* A refuses a legitimate two-word citation — a title-only claim's title can be that short, and
`rigor` does judge title-only claims — and costs that item a retry. B accepts a one-token anchor
(`the`) that appears in any text, so a charge can be anchored to nothing in particular; the citation
rule's one claim is that the span is findable, and one word is findable everywhere.

**D3 — what a `malformed` line records.** The scheme writes `[]` for `none` alone and says a refused
citation is "recorded with it", without saying in which field.
*Reading A — no `faults` on a `malformed` line.* The reply and the line's refusal detail carry the
offending text, and `faults: []` keeps its one meaning: the model said `none`.
*Reading B — the refused record in `faults`.* The id and the span it named, so a failed citation is
legible without reading the reply.
*Cost.* A keeps the field's meaning clean and leaves the rejection legible only as prose. B makes one
field carry accepted and refused charges alike, which "one record per charge" does not describe, and a
later re-derivation then needs a second rule to skip them.

**D4 — does the change of words move `kb-format`'s patch?** `SPEC.md:1019` makes a patch "the bytes a
writer produces changed, their meaning did not"; this landing changes the bytes of every derived
`- strength:` line at the bottom rung, and the phrase's words with them.
*Reading A — `2.0.1`.* An old-patch KB is read as current and restamped on its next save
(`SPEC.md:1024`); `FormatVersion` (`internal/kb/format.go:10`) moves and the fixtures follow on their
next write.
*Reading B — stays `2.0.0`.* The parenthetical is derived display text whose value is unchanged, and a
stamp bump for a label would make the stamp noise.
*Cost.* A makes "which kbase wrote this phrase" answerable and costs a version move across the
fixtures. B leaves two builds writing different bytes under one stamp, and an older kbase reading a
newer KB reports build-status drift (`internal/index/verify.go:905-910`) with nothing recording why.

**Carried, not decided here.** What `counts` reports for a run whose values were re-priced mid-run —
the run records that it was re-priced, and the result document's `counts` says nothing about it, so no
key of it is invented here; the bottom cut's height — the words land at the standing cut, and moving it
moves published strengths, so it is a change of its own; the occurrence units — the parse enforces no
count and the ask states the scheme's units, so segmentation stays the reader's; whether the twelve ids
freeze — the landing writes the twelve names and `flt-pending`, and a later rename moves the ledger's
record and the templates' text with it, so it moves `ask_sha256`: a run resumed after it answers under
the new names while its header still records the old ones, and what a re-derivation does with a
recorded id the running table lacks stays open; whether a bare
assertion keeps `0.95` — the landing implements the rubric as written, and making the absence of
displayed reasoning a criterion adds a seventh `rigor` id and a weight, which is a scheme revision; and
whether `0.95` is discipline or enforcement — a pass cannot produce `1.00` under the table, and
`set-rigor` and `verify` are outside this landing.

**WP4 — the rigour-and-bearing rubric's landing.** Implements
[`RIGOR_AND_BEARING_SCORING_SCHEME.md`](RIGOR_AND_BEARING_SCORING_SCHEME.md) in full: the two asks
rewritten from its criteria, its line parse with its malformed cases, its value from its two tables,
its `faults` on the ledger's line and the ask's digest in the run's header, and the ladder's bottom
rung's words. It serves none of `ACTIVE_PLAN.md`'s six outcomes — it is the rubric's own landing, and
§11's second list is its acceptance. [`RIGOR_AND_BEARING_SCORING_DESIGN.md`](RIGOR_AND_BEARING_SCORING_DESIGN.md)
is the reasoning behind it and is not restated here; where the two disagree, the scheme is normative.
**`run.yaml` records no table.** The design's §8.9 and §10.5, and one sentence of the scheme's
`## The record format is a contract`, still speak of the table's identity; the scheme's `## The ledger`
supersedes them, and the reason is the ruling: the table is what a re-derivation applies, while the ask
is what a judgement answered, so the header carries `ask_sha256` and nothing of the table.
*Two dispatches, in order, both expert.* **First the `prompt-engineer`'s**: the two templates and the
shared fragment, from the scheme's criteria and the design's §7.1/§7.2 — the pending test first, the
assertion rule, the criteria with their near-miss clauses and occurrence units, the ids, `none`, the
line form, the citation rule, the non-faults, and no number anywhere — written to the shelf's
one-`dyn.`-slot grammar and its fixed constant names, so the coder renames no slot. **That pass also
checks the render path against the no-size-dependence rule**, because it is a property of the text they
are rewriting: a rendered prompt carries no element keyed on the item's length or shape — no gate
opening an instruction for a long statement, no band over a short one — so the ask is a function of the
template alone and one digest certifies one instruction. A size gate would make one `ask_sha256` stand
for several different asks, and no test can fail on it: the reviewer reads the text for it, and
criterion 51 states the rule rather than a command. **Then the `go-coder`'s**: the mechanism,
implemented to that fixed text. `ACTIVE_PLAN.md:91-92` makes the review a gate, not a courtesy: no run
may ask a model through these templates before it, and the live score
leg is not exercised until it has. The gate is on the templates; the parser's agreement with the rules
they state is machine-checked for the ids (criterion 47) and read by the reviewer for the prose, and
that limit is stated rather than papered over.
*Boundary.* The ask, the parse and the value are one contract and land together: the templates ask
charge lines, `ParseAnswer` (`internal/score/answer.go:13`) reads them against the closed text, `kb`'s
table prices them, and `set-rigor`/`set-bearing` take the derived value exactly as today. No
intermediate commit is a run that works, so none is shipped and no live leg runs between the two
dispatches. The closed text reaches the parse as passages: `assemble` (`internal/score/text.go:255`)
returns the passage extents beside the text, because a charge's `passage` and the ledger's `offset` are
facts about it; the guard keeps its four scoring tokens (`scoreTokenRE`, `:16`) and refuses now,
beside them, the frame's own structure — a run of twelve backticks and the `## Source:`/`## Target:`/
`## Context` labels (`frameRE`, `:27`) — and the context
passage stays uncitable. The value is exact in integer hundredths (I14). `RunHeader` gains
`ask_sha256` (I15), and `Create` (`internal/score/ledger.go:109`) stamps it from the header's `Pass`
as it stamps `Format` — one site, so no caller can write a run without it, and neither of
`cmd/score.go`'s header constructions (`:217` the dry run's, `:284` the asking one) changes for it.
The digest is the pass's prompt rendered for an empty subject — `asks.ScoreRigor("")` /
`asks.ScoreBearing("")`, whose render substitutes and never inspects the text, so the empty input is
every fixed byte of instruction and nothing that varies — which is why no template may carry a
size-dependent element (criterion 51). The templates are embedded, so a render that fails is a defect
of the build and never a user's refusal: `Create` returns its error rather than writing a run with an
empty digest. `Resume` (`:133`) makes the three comparisons it makes (`:148`) and adds none: the
header's digest is read and never compared, so a run whose `ask_sha256` differs — or whose header
carries none, as every run written before the landing does, its judgements answering the decimal ask —
is resumed, and every item still pending is asked under the running template. Nothing refuses and
nothing asks the user to choose a version. `run.yaml` is not rewritten, so the digest stays the one the
run began under and the corpus can see a run span an update. `ledgerFormat` stays `1` (§9.2): the field
is additive, and an absent one is a fact about when the run was written rather than a format a reader
must reject. The last rung becomes
`{"unsubstantiated", 0.00, "unsubstantiated", "unsubstantiated"}` (`internal/kb/schema.go:37`);
because the slug is also the index's key — `stats` counts by it (`internal/query/payload.go:303-310`)
and a claim's band in the graph is the record's, an unknown slug rendering as pending
(`internal/sheet/graph.go:138-141`, `internal/sheet/dot.go:84-95`) — a KB whose index predates the
change reads its bottom-rung claims as pending and counts them under no row of `stats` until `refresh`
re-renders it. That is stated, not aliased: the index is derived, `refresh` is the write that repairs
it, and `verify` names the register lines' phrase drift before that (`internal/index/verify.go:905-910`).
The digest changes nothing in `cmd/score.go` — the header the verb already builds carries no new field,
because the ledger stamps it — and no result key moves (`cmd/resultkeys_test.go:36` as it stands). The
re-price does change it, in the two places the landing takes it through: `Resume` is opened with the
write seam the runner is opened with (`score.Write`, whose implementation is `cmd/score.go`'s
`landScore`, running `set-rigor`/`set-bearing` through `write.Run`), and the post-run refresh runs
where the resume re-priced even though the walk landed nothing (`Repriced()` beside `Written`), so the
index is republished from the KB as it now stands rather than left behind the register it re-priced. A
resume whose landed values re-derive differently under the running table — a weight changed between
invocations — lands them through the write path, the same one a landing uses and not a second: the
ledger line's `value` and the register's value for that claim both take the running table's output from
the recorded ids alone, so criterion 41's check holds of every line and 44 of both sides, and the
register is never left stale behind the ledger. The write comes first: a moved value lands before its
line records it, so the record never claims a value the KB did not take — a refused write fails the
resume with the ledger as it stood, and the next resume re-derives the same move, the write being
idempotent. The re-price leaves a record of its own in the run's directory saying the run was
re-priced, with the lines that moved; `run.yaml` is written once and is not the place for it (§9.2),
and the result document's `counts` says nothing of it, which is the scheme's carried question. No call
is made: the ids are the input, and the model never saw the weights.
*Must not.* Keep a decimal in the ask or accept one; compute the value in float64; spell a fault id
outside `kb` or a weight inside a template; record a table's identity in place of the ask's digest, or
gate a resume on that digest, or ask the user which version to run under, or re-price the ledger's
`value` while the register keeps the old one; add a size-dependent element to a template so one digest
covers two instructions; change the guard's tokens, the walk's arithmetic, a relation,
`set-rigor`/`set-bearing` or a result key; add a verb, a flag or an MCP tool; touch a contract document;
or settle one of the decisions above by coding one of its readings.
*Hands to WP5.* As the row lists.

**WP5 — the contract text and the reader's side.** Implements §13 (including its ROADMAP row), the
`detect` check name's absence from `SPEC.md:396` (`cmd/configure.go:304`, `:309`), the `configure`
category enumeration (`SPEC.md:405-408`), the line-anchor rule (§13's closing note), and WP4's rows —
**one pass over §8 and the ladder, not two**.
*Boundary.* Every §13 row has its report line — `section — landed — backed by <command>` — and the
six contract files show the change in `git diff --stat`; §11's Outcome 6 census returns only
historical mentions; §9.3's renumbering and every in-document reference move together; the ladder's
words are stated in one place in each document that shows a band, and §10's patch is the one D4 rules,
stated beside the `FormatVersion` the code carries.
*Must not.* Change behaviour; paraphrase §4–§9 instead of citing them; land a §13 row without its
report line; touch code or tests; restate the scheme's criteria instead of citing it.
*Hands to.* Nothing — it is the hand-off.

**WP6 — the closing records.** `ACTIVE_PLAN.md`'s carried items this phase closes — `:61` (the harness
back end's four questions) and `:71-72` (the shared flags' help text) — struck with the commit that
closed each; `:62-63`, `:64-67` and `:68-70` are **not** this phase's and stay listed, and **no carried
item closes with WP4** — it closes none of them. The phase's record gains the rubric
landing, which serves none of its six outcomes. The prompt-engineer's findings arrive in the dispatch
itself, not from a file: there is no findings document to read, and `REVIEW_BURNDOWN.md` belongs to
the `2.0.0` intersprint.
*Boundary.* Each item the phase closed is struck; each finding in the dispatch is answered or carried
by name. The coordinator takes this package itself.
*Must not.* Reopen a ruling, or close an item the phase did not land.

The landing is one commit series merged once (`ACTIVE_PLAN.md:19-22`).

---

## 13. Contract documents this moves

| Document | Section | What it gains |
|---|---|---|
| `SPEC.md` | §7 | `roles` on `build`, `score`, `show-config` and the record's `harness-layer`; `harness` in the `check` list; the model-is-requested sentence; `show-config`'s `harnesses` and `mode`; `layers` gaining `harnessesExists`; `configure`'s `written` order gaining `harnesses.toml` (`providers.toml`, `harnesses.toml`, `config.toml`); `detect` added to the list it is missing from (`:396`) |
| `SPEC.md` | §8 | the `score` synopsis (no `--provider`, two new flags), the `scoring` category, the harness sentences, the ledger's fields and the header-written-once rule, `harness/` in the run's directory |
| `SPEC.md` | §9 | the reference grammar; `models.scoring`; the `provider` key's superseded status; **the two modes, replacing `:870-871`'s field-by-field override sentence** — the environment supplies the whole inference configuration or nothing, and a key file is not a mode; **the preflight paragraph (`:875-877`) rewritten for both**: it names each lacking setting in the mode's own spelling, `KBASE_SCORING` among the variables, and never `models.<role>` for a mode the files do not supply; `KBASE_SCORING`; `configure`'s three categories (`:405-408`), its editor's key set and its third created file; `show-config`'s `roles`, `mode` and `harnesses` |
| `SPEC.md` | §9.1 | unchanged by this design — the pool is a pool, and a pool entry named `harness` is ordinary (R5 undoes R4's reservation) |
| `SPEC.md` | §9.2 | `models.scoring` only; `config.toml` gains no table of this design's (R5) |
| `SPEC.md` | **a new §9.3, `harnesses.toml`** — the profiles' own subsection, with §9.3–§9.6 moving down to §9.4–§9.7 | the file's name and grammar (top-level tables, an open set, entry-whole replacement over the repository and global layers alike), `command` as its closed schema, `{model}` and `{prompt}`, the once-per-run stderr line for a repository-supplied profile, `configure --create`'s **one live profile and one commented entry** (`claude` live, `opencode` commented with its limitation — R7), and that the pool template's one-live-entry rule does not apply. **Three sentences stop counting files**: the opening's "Two TOML files" (`SPEC.md:804-805`), "Both files are decoded strictly" (`:941`), and `show-config`'s "each of its two files" (`:987`) say three. **The renumber moves every in-document reference**: `SPEC.md:190`, `:805`, `:838`, `:839`, `:861`, `:872`, `:879`, `:936-937`, `:981`, and `INTEGRATING_KBASE_VIA_MCP.md:45`, `:217` |
| `SPEC.md` | §9.7 (was §9.6) | `show-config`'s `harnesses` records carry `layer`, so the report states a profile's source without a line on stderr |
| `SPEC.md` | §11 | `:1412`'s "It reads no harness configuration and names no harness" is **reworded, not bent**: the server loads no configuration itself — the verbs it exposes load their own on each call (SPEC §9), the profiles file included — and no tool asks a harness, and `score` is not a tool. The sentence becomes: *it loads no configuration itself, the verbs it exposes load their own on each call, harness profiles included, and uses none of it for a harness; the `build` child it starts inherits the server's environment and `--config-dir`; it names no harness.* |
| `SPEC.md` | §8, the answer and the ledger | **WP4's.** A reply is a list of lines, each a fault id and a citation from the call's closed set — or `none`, alone, or `flt-pending`; the pass's six ids with `flt-pending`, the citation looked up in the closed text the call carried (one passage; an edge's `## Context` is not citable), and the cases refused as `malformed` (a line that is not a charge line, an id outside the set, a citation not found, a bare number, `none` beside charges, an empty reply). The value is kbase's: the pass's maximum and weights, summed per occurrence and floored at zero, exact at two decimals; `set-rigor`/`set-bearing` and the walk are unchanged. The ledger's line carries `faults` (id, passage, citation, offset) beside the reply, `[]` only for `none`; the run's `run.yaml` carries `ask_sha256` — the digest of its pass's prompt rendered for an empty subject, which is every fixed byte of the instructions and nothing that varies — written once and rewritten by no resume. **The run records the ask, not the table, and the digest is a record and not a gate**: a weight change re-prices every value from the recorded ids under the running table, lands the values that moved through the write path — the register with them — and leaves a record saying the run was re-priced; a resume whose `ask_sha256` differs continues under the running template's ask, with the header still naming the one the run began under, so a corpus comparing models and versions reads a run that spans an update; nothing refuses and nothing asks the user to choose. **And a rule with its reason and no test**: a rendered prompt carries no size-dependent element — no gate, no band keyed on the item's length or shape — because the digest must be a function of the template alone, and a size gate would make one digest certify several different instructions. `--dry-run` writes no charge record. The sentence to replace is `:707-710`'s and the ledger paragraph `:723-731`'s |
| `SPEC.md` | §10, the register's spelling | **WP4's.** `:1133`'s "The interior of the scale is the judge's: kbase validates the range and composes" is superseded: a pass derives its value from the charges it records and kbase prices them. The record's `- strength:` parenthetical, `build_status`/`build_band` and `stats`' band record read `unsubstantiated` at the ladder's bottom rung, whose cut does not move; a KB written before the change re-renders its derived lines and its index on the next `refresh`; and `kb-format`'s patch moves, or does not, as D4 rules — the document states the same answer the code carries |
| `ARCHITECTURE.md` | §6, §7 | resolution/binding split, the two modes, the registry, the build's per-role clients and provenance, the harness seam |
| `ARCHITECTURE.md` | §6, §11 | **WP4's.** §6's Scoring subsection: the parse reads a reply against the closed text's passages, the value comes from `kb`'s table, and the shelf's templates carry the criteria and no number; the ledger's line and header gain what `SPEC.md` §8 states, and a resume re-prices through the write seam the runner takes, so a moved value lands as a landing does. §11: `kb` holds the fault vocabulary and the two tables, `internal/score` parses and derives, `internal/asks` asserts the templates' ids against `kb`'s sets |
| `ARCHITECTURE.md` | §11 | `internal/config`'s new surface, `internal/harness`, the dependency rules, the fifth exec owner |
| `ARCHITECTURE.md` | §12 | the role list, `harnessReplyLimit`, `harnessStderrLimit`, `harnessPromptLimit` |
| `CONVENTIONS.md` | Module Structure, House Rules | `config`'s purity, `harness`'s ignorance, the dynamic exec owner |
| `CONVENTIONS.md` | Module Structure, House Rules | **WP4's.** A scoring-model fact lives once, in `kb` — ids, weights, maxima; a template names only the ids its pass accepts, and a test asserts it; a citation is looked up in the closed text the call carried, never in the register; a scoring value is derived in integer hundredths |
| `README.md`, `USER_CONVENTIONS.md`, `INTEGRATING_KBASE_VIA_MCP.md` | config sections, `show-config` | the grammar, the two modes, the flags, the documents' shape |
| `README.md`, `USER_CONVENTIONS.md` | the scoring sections (`README.md:272-278`, `USER_CONVENTIONS.md:159-165`) | **WP4's.** A pass asks for the faults the text commits, each with the span that shows it — or `none`, or `flt-pending`; kbase prices the list into the value the register holds; the ledger keeps the charges beside the reply and the header records the ask's digest, which says which instruction answered a judgement and refuses nothing, so a resume across a changed ask continues under the new one; wherever a band is shown, the bottom rung reads `unsubstantiated` |
| `ACTIVE_PLAN.md` | — | not edited by this design; the landing closes the carried items it closes. **No carried item closes with WP4** — it closes none of them — and WP6 records the rubric landing, which serves none of the phase's six outcomes |
| `ROADMAP.md` | two items | removing the inert `provider` key at the next major (§4.3); and **WP4's carried decisions** — the bottom cut's height, whether a bare assertion becomes a criterion, whether the twelve ids freeze, whether `0.95` is enforced — future intent, so they live here and not in the contract |

`THESIS.md` and `STRENGTH_SCORING_SCHEME.md` do not move (I10; the rubric's landing is I14's, and its
normative page, [`RIGOR_AND_BEARING_SCORING_SCHEME.md`](RIGOR_AND_BEARING_SCORING_SCHEME.md), is not a
contract document). That page is the owner's, and no seat in this table edits it.

**Line anchors.** Every `SPEC.md:<line>` in this design is at `9d83e90`; WP5's insertion of §9.3
moves every line after §9.2, and its renumber moves the section references above. WP4's §8 and §10 rows
name sentences, not lines, for the same reason: the writer moves them. The sentence is the
contract, the line is the citation — a coder who finds a line off by a paragraph looks for the
sentence §13 names, and does not "correct" the design back to the old numbering.

---

## 14. Decisions taken against alternatives

3 and 7 are ruled by the user; the rest are for the review to challenge.

1. **`roles` replaces the top-level provider keys on `build` and `score`** rather than being added
   beside them, and `show-config`'s `inference` retires. §7.3 argues it; the cost is a document
   break the Phase 3 owners must follow.
2. **The harness is a user-configured argv, executed once per item**, with `{model}` and `{prompt}` as
   its only substitutions. The alternatives (MCP client, inline command string, file mailbox) are
   rejected in §1. Nothing written settles it, and the `Ask` seam makes it cheap to reverse — but it
   is the decision most worth challenging, because it decides that kbase executes a program it did
   not write; R6 puts a repository's program within the same reach, and §9.4 prices that as
   disclosure and explicit selection rather than prevention.
3. **The repository layer's `harnesses.toml` is read (R6).** The alternative — not reading it, R3's
   rule — is rejected: it made the profiles file the one configuration a repository could not supply,
   while its `providers.toml` already chose where the text went, and the exclusion bought a second
   `--harness` failure shape rather than a trust boundary. What replaces it is disclosure, not
   prevention: the stderr line, `harness-layer` and the ledger's `harness_layer` (§9.4).
4. **Under a harness, the scoring role's model part is the requested id.** It keeps "absent is a
   refusal in every mode" from being an empty rule (§8.1).
5. **`configure`'s dialed path requires all three roles.** It writes a file that makes `score` work,
   or it does not write one; §13 carries the `SPEC.md:405-408` enumeration it moves.
6. **The global `provider` key is inert rather than refused.** Refusing it would block the
   `configure` run the refusal recommends (`SPEC.md:948-949`). The cost is one key the schema models
   and nothing but its own diagnostic reads, until the next major.
7. **A `KBASE_API_BASE_URL` alone, beside a file-supplied inference, is refused rather than kept as a
   base-URL override.** Ruled, not designed: every setting from files but the URL from the environment
   is withdrawn, because a base URL is an endpoint and an environment endpoint with file models is a
   third, mixed mode. `ModeOf` carries the refusal and §5.2 gives it its rows; `SPEC.md:870-871`'s
   override sentence is withdrawn for the URL as well as for the models (§13's §9 row).
8. **The seeded profile names no `{model}`, and runs on the harness's own model.** R4 and R5 ask for
   seeds that work from a fresh install — R7 makes that one live profile and one commented entry —
   and `--model {model}` with the KB's pool id fails every item — verified with `d3` (§4.6). The cost,
   stated: a fresh `--harness claude` cannot take `--scoring-model` until the user adds
   `--model {model}` to the profile — one edit, in the file the seed itself annotates. The
   alternative, a profile-level default model, was rejected because it would put a second source of
   truth for the scoring model beside the role the KB declares.
9. **A third configuration file, rather than a table in an existing one.** R5 rules the file; this
   records what it costs, so the review sees it priced: a fourth artifact in a configuration
   directory an agent must learn, a third probe on every layer record, a new SPEC subsection that
   renumbers §9.3–§9.6 and every in-document reference to them (§13), and one more file for
   `configure --create` to create. What it buys is that `providers.toml` stays exactly what it was
   and no name in it is reserved (§4.6).
10. **The rubric lands as one package with two dispatches, not as two packages.** No boundary between
    the ask and the parse is green: a template asking charge lines behind a decimal parser, or a
    parser reading charge lines behind a decimal ask, makes every item of every run `malformed`. So
    they land together, and the templates come first as the prompt-engineer's own dispatch because
    they are model-facing text with a named review gate (`ACTIVE_PLAN.md:91-92`) and not a coder's to
    write. The cost, stated: the coder is dispatched serially behind the templates, so the package is
    two dispatches long, and a wrong rule *stated* in a template is caught by the reviewer rather than
    by a test — the ids are machine-checked, the prose is not.
11. **The fault vocabulary and both weight tables live in `kb`.** The alternative, `internal/score`,
    would leave the templates' own package unable to assert the ids they name: `internal/asks` imports
    `kb` and cannot import `score` (which imports `asks`), so the drift check §10 asks for would need a
    cycle or a duplicated list. The cost is that the register's package grows a vocabulary no register
    field carries — the shape `PendingLiteral` and `BuildBandLadder` already stand in.
