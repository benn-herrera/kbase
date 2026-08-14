# AGENTS.md — kbase

Guidance for agents (automated or human) working on this codebase.
**Read ARCHITECTURE.md first.** It is the authoritative design reference and
supersedes any inference you draw from code alone. The live task queue is
ROADMAP.md; ephemeral per-burst plans live in `TEMP_*.md` files.

---

## Project Overview
kbase is a **standalone application/appliance** that converts a human-targeted
documentation corpus — a Markdown doc set; LaTeX and PDF arrive later as
markdown via external conversion (a separate tex→md converter module and a
PDF preprocessing slot — never native ingest) — into an **agent-friendly
knowledge base**: a navigable Markdown tree
(entry-point → domain index → subtopic index → leaf) with verbatim leaves, progressive
hierarchical summaries, and mechanically generated bidirectional navigation links.

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
just test-integration        # omnibus: every per-corpus integration test
just test-integration-rojo   # pinned Rojo corpus: summary values + determinism
just test-integration-rojo-build  # build a whole KB from that corpus, offline,
                                  # and assert the nine verify gates
just prep-test-integration-rojo  # fetch that corpus into test_data/transient/ (no-op if present)
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

**No built binaries live in the repo.** `bin/` and `dist/` are local build
products, gitignored entirely. The distribution artifact is the versioned
tarball `just dist` stages, published as an artifact on a GitHub releases
entry — publishing is the user's act. Agents may run `just dist` to verify
the cross-build still succeeds; never commit build outputs or publish
releases.

**Test data layout.** `test_data/fixtures/` holds version-controlled test
data we author, own, and maintain. `test_data/transient/` (gitignored) holds
cloud-sourced corpus clones, generated test data, and test output. Never
write generated or downloaded data into `fixtures/`.

---

## Module Structure

```
cmd/                kbase CLI (cobra): composition root + one file per verb
internal/assemble/  stages 8+9: the index and entry-point grammar (§4.2/§4.3),
                    the closed class-B fixture manifest shipped with every KB,
                    and the nine verify gates, each scoped to a file class
internal/config/    ~/.config/kbase resolution; providers.toml pool loader;
                    config.toml choices (provider, [models] heavy/light tiers)
internal/detect/    pure gemma-4 family/tier classifier over model-id lists
                    (no I/O); precision-first matching
internal/distill/   stage 5: a page's bytes (span from the tree plan, interior
                    boundaries from the cut list), the §4.4 rebase map, §4.1's
                    page grammar, and the up-link/relative-path/provenance
                    renderers stage 8 shares
internal/ingest/    corpus walk + immutable source custody (§4 stage 1);
                    format-neutral (document extensions are a parameter);
                    per-file sha256 + corpus content hash
internal/log/       leveled structured logging seam over log/slog; console
                    plus optional file tee, built at the composition root
internal/model/     OpenAI-compatible client: blocking + streaming (SSE),
                    ListModels, ConsultDrained (stream-and-drain), mock fabric
internal/prompt/    per-call slot-stack context builder (§7): render, budgets,
                    CRITICAL/REMINDER trailer, per-slot churn hashes
internal/survey/    the survey artifact (§4 stage 2) and NOTHING format-specific:
                    heading tree with byte offsets, section token sizes, link
                    graph, gists; corpus roll-up, tiling + custody checks,
                    deterministic JSON; the import-policy test lives here
internal/survey/markdown/
                    the goldmark adapter — the only package that may import
                    goldmark or yaml; produces survey.Artifact and owns the
                    Markdown extension set
internal/tokens/    single chars-per-token estimator (§8)
internal/treeplan/  the tree plan artifact (§4 stage 3 output, kbase.treeplan/1):
                    planned KB tree + split groups + annexes, namer/slugger, the
                    mechanical verifier (partition/caps/G-1/G-2, design-time
                    split expansion, restructuring operators), deterministic JSON
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
   gitignored; these files are never committed.

Why repo-root and not agent memory (`~/.claude/projects/<project>/memory/`):
plans there are invisible to the human and accumulate forever. A root-level
TEMP file is human-inspectable while live and dies when done. The durable
task queue is ROADMAP.md; TEMP plans are per-burst execution detail only.

## Testing

**No important verification is a one-off** (ruled 2026-08-13). When we go
to the trouble of devising a way to verify something works, that way is
preserved so it can run repeatedly — regression protection first, coverage
extension second. Concretely:

- Every verification lands as a repeatable invocation: a unit test, an
  integration recipe (`test-integration-*`), or a dev verb — never an
  ad-hoc command sequence that lives only in a conversation.
- Observational evidence (counts, measurements, composed artifacts, A/B
  numbers) is emitted to an inspectable location — `test_data/transient/
  <name>/` for test-produced artifacts, an explicit `--out` for verbs —
  not left in ephemeral agent reports or terminal scrollback.
- Unit tests live beside their packages and must pass hermetically (no
  network, no corpus). Corpus-driven tests skip when the pinned corpus is
  absent and are wired into `test-integration-rojo`, where the corpus is
  guaranteed present — a corpus property that only runs "sometimes" is
  coverage that silently isn't.

**Results are part of the test** (ruled 2026-08-13). Results that cannot
be examined are not results — they are phantasms leading to delusions of
progress and hallucinations of adequacy. Concretely:

- Every **integration test** preserves its log output under
  `test_data/transient/<test-name>/`. Where the relevant information is
  in the log, that is sufficient; where gathered stats ARE the results,
  those stats are dumped to files under the same location.
- The **artifacts** of integration test runs are preserved under
  `test_data/transient/` — nothing is ever written to an ephemeral
  location and tossed. The temp-work keep switch is ALWAYS on for
  integration tests, with one deliberate exception: the test that proves
  correct behavior in the switch's absence (guarding against logic that
  silently depends on it).
- **Unit tests** run under `go test`, which already logs expected-vs-
  actual and pass/fail to stdout/stderr; preserving that log under
  `test_data/transient/unit_tests/<test_name>_log.txt` is sufficient.

---

## Dependency Policy

TBD. One ruling stands: `golang.org/x/text/unicode/norm` is sanctioned
(2026-08-13) for the NFC pre-pass at ingest (ARCHITECTURE.md §4 stage 1, §9).
It is the Go project's own module, it brings no transitive dependency, and the
tables it applies are frozen by the Unicode normalization stability policy.
Add a module with `just add-dependency <module>@<version>` and only after the
import exists — `go mod tidy` drops a dependency nothing imports.

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
