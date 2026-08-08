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
just edit-gate  # cheap gate, run after every change: fmt-check + go vet
just checkpoint # full gate, run at checkpoints: edit-gate + test + build
just cover      # aggregate whole-suite coverage → cover.out
just fmt        # gofmt -w over the Go source roots
just dist       # cross-builds → bin/: darwin-arm64, windows-amd64, linux-amd64
just add-dependency <module>@<version>  # pin ONE vetted module
```

`just dist` cross-compiles every target from one host (pure Go, no extra
toolchain setup needed).

---

## Module Structure

```
cmd/                kbase CLI (cobra): composition root + one file per verb
internal/config/    ~/.config/kbase resolution; providers.toml pool loader;
                    config.toml choices (provider, [models] heavy/light tiers)
internal/detect/    pure gemma-4 family/tier classifier over model-id lists
                    (no I/O); precision-first matching
internal/model/     OpenAI-compatible client: blocking + streaming (SSE),
                    ListModels, ConsultDrained (stream-and-drain), mock fabric
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

## Testing

TBD

---

## Dependency Policy

TBD

---

## Logging

TBD
