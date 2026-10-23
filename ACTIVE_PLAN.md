# ACTIVE PLAN — scoring ask capture and replay

**Purpose.** Scoring runs fail items on the reply's citation form, and testing a change to a scoring
template costs a fixture install, a full walk and a reset. This plan makes every scoring call leave a
replayable record, and adds a non-dist replay that re-asks recorded subjects under the binary's
embedded templates and reports what the runtime would have done — so a template variation is
`edit → just build → replay`, over exactly the items that failed.

Contract sources: THESIS.md, SPEC.md, ARCHITECTURE.md, CONVENTIONS.md.
[RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md) is normative on the rubric;
this plan changes no rubric, template, parse rule or price.

## Settled shape

1. **Every scoring call is captured.** Each attempt that reaches the `Ask` seam appends one JSON line
   to `captures.jsonl` in the run's directory (`<state-dir>/score/<run>/`), beside `ledger.jsonl`,
   whatever its outcome — a landing, a parse refusal, a call failure, a write refusal — a cancelled
   call excepted, which writes neither capture nor ledger line. An item the guard refuses makes no
   call and has no capture. The capture is written before the item's ledger line. A capture that
   cannot be written fails the run as a ledger line that cannot be written does. The ledger is
   unchanged: its failed line still carries the reason and no reply text.
2. **A capture is self-sufficient for a replay.** Its keys, in this order: `run`, `pass`, `kind`,
   `id`, `source`, `target`, `relation` (the ledger line's identity keys, omitted as the ledger omits
   them), `class` (`statement`, `method` or `bearing`), `text` and `lines` (the closed text whole:
   each line `{no, end, from, to}` as `closedText` holds it), `prompt` (the bytes sent),
   `prompt_sha256`, `reply` (what the `Ask` seam returned; `""` where the call errored), `error`
   (the call's, the parse's or the write's error text; absent on a landing), `model`, `backend`,
   `started`, `ended`.
3. **Runs are kept to the newest `keptRuns` (16) of each kind.** A real run's id is minted
   `run-<id>` as today; a dry run's is minted `dry-<id>` (`kb.MintID`'s prefix), both under
   `score/`. A directory's kind is its name's prefix; a dry run minted before this change is named
   `run-…` and counts as real. Immediately after `build.Hold` returns, a scoring invocation keeps the
   run it names (created or resumed) and the newest 15 other directories of its own kind, by
   `run.yaml`'s `started`, equal starts ordered by name with the later name newer, and removes the
   rest of that kind whole: a dry run never removes a real run, nor a real run a dry one. A directory
   whose `run.yaml` is absent or unreadable has no start to order by and is left. A prune error ends
   the invocation `failed`. An invocation refused after `build.Hold` has pruned already, which only
   ever removes directories beyond the kept count. `status`, `wait` and `RunByPID` find runs by
   directory, whatever the prefix. Captures and replays live and go with their run. A replay, at its
   start, keeps itself and the newest 15 others of its run's `replays/` by `replay.yaml`'s `started`,
   by the same ordering.
   - `--resume dry-…` is refused by the id-shape check, whose detail reads "--resume names a real
     run's id (run-<…>), not a path"; `--resume` naming a well-formed run no longer standing is
     refused as `Resume` refuses a missing `run.yaml`.
   - `--replay`'s value must match `^(run|dry)-<id>$` (the alphabet and length `runIDRE` uses), or
     the refusal is `check: usage` naming `--replay`; a well-formed run no longer standing, and a
     `dry-` run, have no `captures.jsonl` and take that refusal (Settled shape 4).
4. **`kbase score --replay RUN`** re-asks a run's captured subjects. The selection is every item
   with a capture, in the order of the run's `items.jsonl`, each by its last capture; `--failed`
   narrows it to items whose last ledger status is `failed`; `--limit N` takes the first N. Per item
   it renders the captured closed text through the binary's embedded template for the capture's
   class, sends it on the scoring role's endpoint (`--scoring-model` overrides, as for a run),
   parses it against the captured closed text with the class's table, and prices it. It lands
   nothing, opens no KB and takes no KB, operation or holder lock; a run in progress may be
   replayed, its captures read to the last complete line. Accepted beside `--replay`:
   `--failed`, `--limit`, `--scoring-model`, `--state-dir`, `--timeout`, `--config-dir`; any other
   score flag with it is a usage refusal naming `--replay`, and `--failed` without `--replay` is a
   usage refusal naming `--failed`. A run with no `captures.jsonl` is `refused`, `check: usage`,
   `key: --replay`.
5. **A replay writes only under `score/<RUN>/replays/<replay-id>/`**, the id minted as a run id is:
   - `replay.yaml`: `replay`, `run`, `backend`, `provider`, `model`, `ask_sha256` (the binary's,
     computed as `Create` computes it for the run's pass), `started`.
   - `ledger.jsonl`: one line per item in the run ledger's line shape, `run` being the replay id.
   - `captures.jsonl`: the replay's own captures, Settled shape 2's keys, `run` the replay id.
   - `comparison.jsonl`: per item `item` (`ItemName`), `original` and `replay` (each `status`,
     `value`, `faults` — the fault ids in order, `[]` for `none`), and `prompt_changed` (the
     replayed prompt's digest differs from the captured `prompt_sha256`).
6. **The replay's result document** is one SPEC §7 YAML document, keys in this order: `outcome`,
   `kb-root`, `state-dir`, `run`, `replay`, `dir` (the replay directory, absolute), `counts`
   (`total`, `done`, `failed`), `changed` (an integer: items whose status or value differs from the
   original's), then the closing record the outcome names. Its outcome is `done` when every selected
   item was asked — per-item failures are counted, not the replay's outcome — and `failed` for a
   fault of the replay itself (an unreadable capture file, a store write).
7. **Fidelity is a test.** Rendering a capture's closed text through the unchanged template
   reproduces its `prompt_sha256` byte for byte, asserted over captures the runner wrote.
8. **The replay is not in dist builds.** Every replay file, code and tests, in `cmd` and
   `internal/score`, carries `//go:build !kbasedist`. `cmd/score.go` reaches the replay only through
   a package-level hook the tagged file's `init` sets, nil under `kbasedist`, and the tagged file's
   `init` declares `--replay` and `--failed`. Both flags carry a flag-level `kbase.mcp` =
   `excluded` annotation and MCP schema derivation skips such a flag, so dev and dist builds serve
   one tool table; an argument outside a tool's schema is already refused by `internal/mcp`. The
   dist build passes `-tags kbasedist`; default builds and `go test ./...` include the replay. SPEC
   states nothing of the replay; ARCHITECTURE and CONVENTIONS do.

## Established facts the packages act on

Re-run each before acting on it; the answer given is what it returned when this plan was written.

| Fact | Command | Answer |
|---|---|---|
| The runner holds closed text, prompt and raw reply at one site | `sed -n 160,199p internal/score/run.go` | `attempt` composes (`r.compose`), calls `r.Ask`, then `ParseAnswer`, then `r.Write`; a parse or write error replaces the reply before `Ledger.Append` |
| `closedText` is unexported `text` + `[]line{no, end, from, to}` | `sed -n 82,99p internal/score/text.go` | as stated |
| A class is `ask` + `table` + `edge` | `sed -n 625,645p internal/score/ledger.go` | `statementClass`, `methodClass`, `bearingClass` |
| Pruning removes build output only | `sed -n 461,465p internal/build/store.go` | `reports/` entries prefixed `CapturePrefix`, and `scratch/captures/` |
| Nothing removes under `score/` but the result document | `grep -rn 'os.Remove' --include='*.go' internal cmd \| grep -v _test` | under `score/`, only `internal/score/ledger.go:334` (`result.yaml`) |
| The build's retention precedent | `grep -n 'keptBuilds' internal/build/*.go \| grep -v _test` | `keptBuilds` (5), ARCHITECTURE §12 "Builds kept" |
| A run's start is readable per directory | `sed -n 383,416p internal/score/ledger.go` | `NewestRun` reads each `run.yaml`'s `started` and skips a directory whose `run.yaml` is absent or unreadable |
| The dist build line | `sed -n 20,37p tools/scripts/build-dist.sh` | `RELEASE_FLAGS=(-trimpath "-ldflags=-s -w")` at 27; `go build "${RELEASE_FLAGS[@]}"` at 36 |
| The dist test matches that line whole | `sed -n 189,192p tools/scripts/test-build-dist.sh` | `grep -qxF` on `… go build -trimpath -ldflags=-s -w -o … ./cmd` |
| MCP schema walks every local flag | `sed -n 211,226p cmd/mcp.go` | `commandSchema` skips `help` only |
| An argument outside a schema is refused | `sed -n 128,142p internal/mcp/mcp.go` | "argument … is not one this tool takes" |
| A test pins every score flag in its schema | `sed -n 370,385p cmd/mcp_test.go` | `TestMCPRequiredFlags`, subtest `every flag still derives` |
| No flag-level annotation exists | `grep -rn 'SetAnnotation' cmd internal` | no match |
| No build tag beyond OS tags exists | `grep -rn 'go:build' --include='*.go' cmd internal` | `unix`, `!unix`, `windows` only |
| A build's asks re-render from its boundary | SPEC §6: "A user may reset to a stage commit and resume from there"; ARCHITECTURE §8: a resume "re-runs the stage whole"; `sed -n 280,300p internal/asks/call/call.go` | the answer cache key is the digest of the model, the question and the composed state, so a changed prompt misses the cache and reaches the model |

## Work packages

| WP | Rating | Seat / model | Lands |
|---|---|---|---|
| 1 | expert | go-coder, inherits | capture |
| 2 | expert | go-coder, inherits | replay and the MCP flag exclusion |
| 3 | mechanical | shell-dsl-coder, `reaper/Qwen3.8-Flash-Next` | dist tag and recipes |
| 4 | expert | tech-writer, inherits | contract text |

One landing, WP1 → WP2 → WP3 → WP4, each package's diff audited before the next starts, committed
once after WP4. Before WP2 is dispatched the coordinator lands WP4's CONVENTIONS exception, so WP2's
diff is audited against a written rule. WP3 runs on local inference, so it runs alone.

### WP1 — capture

- `internal/score`: the runner writes a capture per call through the ledger (a method beside
  `Append`), so the runner keeps no file I/O of its own, per Settled shape 1–2.
- A capture reader returns, per item, its last capture with the closed text whole, so WP2 parses
  against it.
- Run retention per Settled shape 3: `keptRuns` a named constant in `ledger.go`'s const block
  beside `runsDir`; the per-directory start read extracted from `NewestRun` into one enumerator that
  `NewestRun` and the prune both use; the prune a function of the state directory and the kept run,
  called from `runScore` immediately after `build.Hold`; the dry run mints `dry-`; the `runIDRE`
  refusal detail and its comment (`cmd/score.go:121-123`, `365`) name a real run's id. Tests: a
  seventeenth real run removes the oldest real run; a seventeenth dry run removes the oldest dry run
  and no real run, and the reverse; with 17 real runs standing, each with a distinct `started`,
  resuming the oldest keeps it and removes the second-oldest; equal starts prune the earlier name; a
  directory without a readable `run.yaml` stands; a `--resume` of a pruned run, and of a `dry-` id,
  is refused with the details Settled shape 3 gives; `status` and `wait` report a dry run.
- Tests: a landed, a parse-refused, a call-failed and a write-refused attempt each leave one
  capture with Settled shape 2's keys; the parse-refused capture carries the reply text and its
  ledger line does not; a cancelled call leaves neither; a capture write failure fails the run;
  re-rendering a capture's closed text through its class's template reproduces its `prompt_sha256`.

### WP2 — replay and MCP flag exclusion

- `cmd` and `internal/score`, every file `!kbasedist`, per Settled shape 4–6 and 8: the scoring role
  resolved as `runScore` resolves it, the client from the registry, `model.Chat` and the
  completion-cap check as the run's `ask` does them — one ask path, not a copy; a replay walk over
  captures writing the replay directory's four files.
- `cmd/mcp.go`: a helper setting a flag's `kbase.mcp` annotation to `[]string{"excluded"}`, applied
  to `--replay` and `--failed`; `commandSchema` skips a flag carrying it.
  `TestMCPRequiredFlags`'s `every flag still derives` subtest moves to its new truth: every flag
  not annotated `excluded` derives with its usage, and `replay` and `failed` are absent from
  `score`'s properties.
- Tests: a replay over a fake endpoint answering the original replies reproduces the original
  statuses and values with `prompt_changed` false and `changed` 0; `--failed` selects only failed
  items; `--limit` cuts in `items.jsonl` order; the last capture of a re-asked item is the one
  replayed; `replays/` retention per Settled shape 3 (a seventeenth replay removes the oldest);
  `--replay` of a malformed id, of a pruned run and of a `dry-` run each refused; each refused flag
  combination and the missing-captures refusal; the result document's
  keys in order; a replay leaves `kb-root/` and the run's own files byte-identical and writes only
  under `replays/<id>/`.

### WP3 — dist tag and recipes

- `tools/scripts/build-dist.sh` line 27 becomes `RELEASE_FLAGS=(-trimpath -tags kbasedist
  "-ldflags=-s -w")`, and the comment above it (lines 20–22) names the tag as excluding the
  development replay.
- `tools/scripts/test-build-dist.sh` line 190 becomes
  `line="GOOS=${os} GOARCH=${arch} go build -trimpath -tags kbasedist -ldflags=-s -w -o ${bin}/${name} ./cmd"`.
- An in-package test file `cmd/dist_test.go` (`package main`'s, as `scoreCmd` is unexported),
  `//go:build kbasedist`, holding `TestDistBuildHasNoReplay`: `scoreCmd.Flags().Lookup("replay")` and `Lookup("failed")` are
  nil.
- `justfile`: the `test-dist` recipe gains, after its script line,
  `go test -count=1 -tags kbasedist -run '^TestDistBuildHasNoReplay$' ./cmd 2>&1 | tee -a "{{TEST_DATA_TRANSIENT_DIR}}/test-dist/log.txt"`;
  `edit-gate` gains `go vet -tags kbasedist ./...` after `go vet ./...`.
- After the justfile edit: `set -o pipefail; just --dry-run test-dist 2>&1 | bash -n`, then
  `just edit-gate` and `just test-dist`.

### WP4 — contract text

- SPEC §6, the paragraph led **The state store holds what does not belong in the repository**, and
  §8 *The ledger and resume*: a scoring run's directory holds `captures.jsonl`, every call's closed
  text, prompt and reply, failures included; the ledger line's no-reply-text rule stands. SPEC §6's
  **The state store is pruned only by a build** becomes pruned by a build or a scoring run, the run
  rule (Settled shape 3) stated beside the build's; §8's `--dry-run` paragraph states the `dry-`
  id and that a dry run counts only against dry runs; §8 *Outcomes*' refusal list reads "a
  `--resume` naming no real run kbase minted". SPEC says nothing of `--replay`.
- ARCHITECTURE §12 gains `keptRuns` beside "Builds kept"; §8 *Retention* names the scoring run's
  prune; §6 *The verb* gains the prune step after `build.Hold`.
- ARCHITECTURE §2 gains an invariant: every ask a build or a scoring run makes leaves, for as long
  as its records are retained, what re-asking it under the binary's embedded templates and
  re-reading its reply as the run read it needs — a build through its commit trail, its records and
  the answer cache's keying on the composed ask (re-running a stage from its boundary re-renders
  every ask), a scoring run through its captures. `probe` is a diagnostic and outside it.
- ARCHITECTURE §6 *Scoring*: the capture, the replay walk, its directory, the fidelity rule, the
  dist tag; §12's *MCP server (SPEC §11)* paragraph: a flag annotated `kbase.mcp` = `excluded` is
  not derived into its command's schema; §11 package map if `internal/score`'s line moves.
- CONVENTIONS: in *Coordinator Policy*'s dev-only bullet and as a house rule, the one exception — a
  diagnostic compiled only into non-dist builds (`//go:build !kbasedist`), excluded from MCP by its
  flag annotation, absent from the shipped binary; and under I9's house rule, that a flag annotated
  `kbase.mcp` = `excluded` is not part of its command's tool schema. (The coordinator lands this
  bullet before WP2.)

## Gates

Per package: `just edit-gate`, `just test`. Before the landing: `just test-integration` (which runs
`test-dist`) and `just checkpoint`.

**First use, after the landing (the owner's, live):** one `kbase score` run over the fixture KB
under the new build, so its failures are captured; then `kbase score --replay <run> --failed` under
an unchanged build, whose comparison shows `prompt_changed` false throughout — the baseline every
template experiment is read against.

## Not in this plan

- Replay of the build's letter asks and the overview call through a capture; the build re-asks by
  re-running a stage.
- Any template, rubric or parse-rule change. Those are the experiments this enables.

## Post-sprint review

Once the sprint's last landing is committed, one pass reviews the changes since the plan was
activated rather than the individual changes. The reviewers are an architect and a coder; before
they start they read THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md, and they do not read
this plan. Their subject is the changes against those standards and whether the result is fit for
its purpose — not a rehash of how it was implemented — and their brief includes the tests.

**Review**: findings against the four documents. **Remediation**: what each finding makes
necessary, landed in a follow-up commit or declined with the reason recorded. **Burndown**: what
remains, one fact to an item, below.

### Post-sprint cleanup burndown

Not started.
