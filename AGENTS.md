# AGENTS.md — kbase

Guidance for agents (automated or human) working on this codebase.
**Read ARCHITECTURE.md first.** It is the authoritative design reference and
supersedes any inference you draw from code alone. The live task queue is
ROADMAP.md; ephemeral per-burst plans live in `TEMP_*.md` files.

---

## Project Overview
kbase is a **standalone application/appliance** that converts a human-targeted
documentation corpus (Markdown; other formats arrive as Markdown via external
conversion — see ARCHITECTURE.md §4) into an **agent-friendly knowledge
base**: a navigable Markdown tree (entry-point → domain index → subtopic
index → leaf) with verbatim leaves, progressive hierarchical summaries, and
mechanically generated bidirectional navigation links.

---

## Build and Run

```sh
just            # list recipes
just build      # host-platform build → bin/kbase
just test       # unit tests (VERBOSE=1 for per-test output)
just test-race  # unit tests under the race detector
just edit-gate  # cheap gate, run after every change: fmt-check + go vet
just checkpoint # full gate, run at checkpoints: edit-gate + test-race + build
just cover      # aggregate whole-suite coverage → cover.out
just test-integration        # omnibus: every hermetic integration test
just test-integration-survey-rojo  # pinned Rojo corpus: summary values + determinism
just test-integration-build-mechanical-rojo  # `kbase build` over that corpus under
                                  # the offline [dev] fixture; the ten verify gates
just test-integration-build-live-rojo  # the same build with the model in the loop;
                                  # LIVE (provider + network), excluded from the omnibus
just prep-test-integration-rojo  # fetch that corpus into test_data/transient/ (no-op if present)
just test-integration-survey-omlx  # pinned oMLX docs corpus: summary values + determinism
just test-integration-build-mechanical-omlx  # offline KB build over it, ten gates
just test-integration-build-live-omlx  # the same build with the model in the loop;
                                  # LIVE (provider + network), excluded from the omnibus
just prep-test-integration-omlx  # sparse+partial fetch of that corpus (no-op if present)
just test-integration-write-generic-agents  # write-generic-agents: fresh write,
                                  # full + partial overwrite refusal; joins the omnibus
just fmt        # gofmt -w over the Go source roots
just fmt-check  # read-only counterpart of fmt; fails on formatting drift
just clean      # remove the host build (bin/kbase)
just nuke       # remove every build output (bin/, dist/)
just dist       # checkpoint, then cross-builds → bin/ + a staged distro tarball
just add-dependency <module>@<version>  # pin ONE vetted module
just update-dependencies                # upgrade the WHOLE module graph
```

`just dist` cross-compiles every target from one host (pure Go, no extra
toolchain setup needed) and runs `checkpoint` first — a cross-build is what
users receive, so it ships only from a tree that passes the full gate.

**No built binaries live in the repo.** `bin/` and `dist/` are gitignored
build products; distribution is the `just dist` tarball attached to a GitHub
release by the user. Never commit build outputs or publish releases.

**Test data layout.** `test_data/fixtures/` holds version-controlled test
data we author, own, and maintain. `test_data/transient/` (gitignored) holds
cloud-sourced corpus clones, generated test data, and test output. Never
write generated or downloaded data into `fixtures/`.

---

## Module Structure

```
cmd/                kbase CLI (cobra): composition root + one file per verb
internal/assemble/  stages 8+9: index/entry-point rendering, the fixture
                    manifest, the ten verify gates, delivery; also the generic
                    agent-definition samples, which are not delivered but do
                    render the same contract statement
internal/config/    ~/.config/kbase resolution; providers.toml pool loader;
                    config.toml choices (provider, [models] heavy/light tiers)
internal/detect/    pure gemma-4 family/tier classifier over model-id lists
                    (no I/O); precision-first matching
internal/dissect/   stage 4: mechanical split, the boundary-refinement fold,
                    cut-list verification, leaf slicing
internal/distill/   stage 5: page bytes from tree plan + cut lists, the link
                    rebase map, the page grammar and shared renderers
internal/ingest/    corpus walk + immutable source custody (§4 stage 1);
                    format-neutral (document extensions are a parameter);
                    per-file sha256 + corpus content hash
internal/log/       leveled structured logging seam over log/slog; console
                    plus optional file tee, built at the composition root
internal/model/     OpenAI-compatible client: blocking + streaming (SSE),
                    ListModels, ConsultDrained (stream-and-drain), mock fabric
internal/prompt/    per-call slot-stack context builder (§7): render, budgets,
                    CRITICAL/REMINDER trailer, per-slot churn hashes
internal/summarize/ stage 6: level-sliced summary stages, the JSON summary
                    artifact and its verifier; a no-fallback seam
internal/survey/    the survey artifact (§4 stage 2) and NOTHING format-specific:
                    heading tree with byte offsets, section token sizes, link
                    graph, gists; corpus roll-up, tiling + custody checks,
                    deterministic JSON; the import-policy test lives here
internal/survey/markdown/
                    the goldmark adapter — the only package that may import
                    goldmark or yaml; produces survey.Artifact and owns the
                    Markdown extension set
internal/taxonomy/  stage 3: the container descent that designs the tree
                    plan from the survey; a no-fallback seam
internal/tokens/    single chars-per-token estimator (§8)
internal/treeplan/  the tree plan artifact (kbase.treeplan/1): planned KB
                    tree + split groups + annexes, namer, mechanical verifier
internal/version/   single-source version identity
```

### Module responsibilities in brief

- **cmd/**: verb logic lives behind a plain options-struct function; the cobra
  `RunE` is a thin loader shell, so verbs unit-test without process/network.
- **internal/config**: loaders take explicit paths (tests never touch a real
  home). Per-entry provider faults never abort the pool; fault reasons carry
  no key material.
- **internal/model**: `Endpoint{Name, BaseURL, APIKey}` is the transport-level
  slice of a provider. Long pipeline calls use `ConsultDrained` (streaming
  transport, blocking semantics) for idle-timeout robustness. API keys never
  appear in logs or error strings.

---

## Key Architecture Constraints

The invariants and design principles in ARCHITECTURE.md (§2, §3) bind every code
change; violating any is a blocking defect. They are deliberately not restated
here — ARCHITECTURE.md is the single source. Working habits they impose:

- Before writing code, have ARCHITECTURE.md §2–§3 fresh in context; check your
  change against them before considering it done.
- When a requested change collides with an invariant or principle, stop and
  surface the collision — do not quietly pick a side.
- Constants live in the ARCHITECTURE.md constants table and as named constants
  in code; never introduce a magic number alongside them.

---

## Plan & Execute Process

Non-trivial work bursts follow a standard shape:

1. Write the plan to `TEMP_PLAN_<SCREAMING_SNAKE_TOPIC>.md` at the repo root
   (e.g. `TEMP_PLAN_CONTEXT_MANAGEMENT.md`), including decisions confirmed at
   kickoff, work-package boundaries, and sequencing status.
2. Execute out of the plan file — dispatched agents read it; status checkboxes
   update as work packages land.
3. Discard the file once the work is landed and reviewed. `TEMP_*.md` is
   gitignored; these files are never committed. The durable task queue is
   ROADMAP.md; TEMP plans are per-burst execution detail only.

## Coordinator Policy

For the session-level agent managing dispatched coder agents.

**Audit the diff, not the report.** After a dispatched agent reports, audit
the DIFF before accepting — the report is an index, the diff is the
evidence. Check the diff against the recurring violation classes:

- integration tests invoke the app binary; no dev-only entry points, verbs,
  or test scaffolds around the real surface
- verifications are repeatable invocations; evidence lands in inspectable
  artifacts, never only in the report
- no special-case file lifecycles: delivered classified files and temp-work
  are the only two path classes
- no gate, pin, or assertion loosened to make something pass — expected-truth
  updates must state the new truth they track
- no second home for a value, string, or mechanism that has one
- nothing beyond the dispatched scope, even improvements

Violations block acceptance: fix (redispatch or direct) before commit.
Style and judgment calls outside these classes: flag only if egregious.

**Direct coding at the session level**: permitted, but read the
topic-specific coder agent definition first (plus this file if not fresh in
context) so session-level code holds the same contract dispatched code does.

## Testing

**No verification is a one-off.** Every verification is a repeatable
invocation — a unit test, an integration recipe, or a dev verb — never an
ad-hoc command sequence that lives only in a conversation.

**Integration tests invoke the app binary.** An integration test runs
`./bin/kbase` directly — the system end-to-end as delivered. Behavior
switches (keep-temp-work, config dirs) are flags in the recipe's
invocation. In-process calls to verb functions are unit tests of verb
logic, never integration tests.

**Results are part of the test.** Results that cannot be examined are not
results.

- Integration recipes: `test-integration-<process>-<corpus>`; shared
  corpus fetch `prep-test-integration-<corpus>`; anything needing network
  or a live provider is excluded from the hermetic omnibus
  `test-integration` and says so in its `[doc]`.
- Every integration test preserves its full log, its stats, and its run
  artifacts under `test_data/transient/<test-name>/` — nothing is written
  to an ephemeral location and tossed. The temp-work keep switch is on,
  with one deliberate no-keep exception test.
- Unit tests pass hermetically (no network, no corpus) beside their
  packages; their `go test` log is preserved under
  `test_data/transient/unit_tests/`. Corpus-driven property tests skip
  when the corpus is absent and are wired into an integration recipe
  where it is guaranteed present.
- Observational evidence (counts, measurements, A/B numbers) is emitted
  to an inspectable location, never left in agent reports or scrollback.

---

## Dependency Policy

Stdlib-first; a new module requires a ruling. Sanctioned:
`golang.org/x/text/unicode/norm` (NFC pre-pass; no transitive deps; tables
frozen by Unicode stability policy). Add with
`just add-dependency <module>@<version>`, only after the import exists —
`go mod tidy` drops a dependency nothing imports.

---

## Logging

`internal/log` is the only logging seam: a four-method `Logger` interface
(`Debug`/`Info`/`Warn`/`Error`, each taking alternating key/value pairs) with
`log/slog` behind it.

- It is built **once**, in the composition root (`cmd/main.go`), from the
  persistent `--log-level` (debug|info|warn|error, default warn) and
  `--log-file` flags. `--log-file` tees; it never redirects.
- It is **threaded, not global**: a component that logs takes a `log.Logger`
  parameter, so its logging is visible in its constructor signature. There is
  no package-level logger and no setter; `log.Discard()` covers a caller with
  nothing to hand it.
- Never call `slog` directly, and never use `fmt.Fprintf(os.Stderr, ...)` for
  a diagnostic. Verb *output* — a summary line, a user-facing warning — is a
  different channel and stays on the verb's `Stderr` writer.
