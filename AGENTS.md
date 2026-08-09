# AGENTS.md — kbase

Guidance for agents (automated or human) working on this codebase.
**Read ARCHITECTURE.md first.** It is the authoritative design reference and
supersedes any inference you draw from code alone. The live task queue is
ROADMAP.md; ephemeral per-burst plans live in `TEMP_*.md` files.

---

## Project Overview
kbase is a **standalone application/appliance** that converts a human-targeted
documentation corpus — a Markdown doc set or LaTeX source (PDF via a preprocessing
adapter, later) — into an **agent-friendly knowledge base**: a navigable Markdown tree
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

**Dist binaries are the user's to update.** The cross-built binaries under
`bin/` are LFS-tracked, and only the user refreshes them unless you are
directly instructed otherwise. Agents may run `just dist` to verify a
cross-build still succeeds; never commit its outputs.

---

## Module Structure

```
cmd/                kbase CLI (cobra): composition root + one file per verb
internal/config/    ~/.config/kbase resolution; providers.toml pool loader;
                    config.toml choices (provider, [models] heavy/light tiers)
internal/detect/    pure gemma-4 family/tier classifier over model-id lists
                    (no I/O); precision-first matching
internal/ingest/    Markdown corpus walk + immutable source custody (§4
                    stage 1); per-file sha256 + corpus content hash
internal/log/       leveled structured logging seam over log/slog; console
                    plus optional file tee, built at the composition root
internal/model/     OpenAI-compatible client: blocking + streaming (SSE),
                    ListModels, ConsultDrained (stream-and-drain), mock fabric
internal/prompt/    per-call slot-stack context builder (§7): render, budgets,
                    CRITICAL/REMINDER trailer, per-slot churn hashes
internal/survey/    per-file structural inventory (§4 stage 2): heading tree
                    with byte offsets, section token sizes, link graph, gists
internal/tokens/    single chars-per-token estimator (§8)
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

TBD

---

## Dependency Policy

TBD

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
