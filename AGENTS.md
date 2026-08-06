# AGENTS.md — kbase

Guidance for agents (automated or human) working on this codebase.
**Read ARCHITECTURE.md first.** It is the authoritative design reference and
supersedes any inference you draw from code alone.

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
just build    # debug build (cargo build)
just release  # optimized build → target/release/laterm
just test     # unit tests (cargo test)
just check    # type-check all three release targets (validates cfg flags)
just dist     # cross-build aarch64-apple-darwin, x86_64-unknown-linux-gnu,
              #   x86_64-pc-windows-gnu via cargo-zigbuild → dist/
just install  # copy this OS's dist binary to ~/bin (INSTALL_DIR overrides);
              #   rm-then-cp for a fresh inode + dequarantine on macOS
just fmt      # cargo fmt
just lint     # cargo clippy --all-targets
just setup    # rustup targets + cargo-zigbuild (needs zig: brew install zig)
```

`just dist` cross-compiles every target from one host

---

## Module Structure

```
cmd/
internal/
```

### Module responsibilities in brief

TBD

---

## Key Architecture Constraints

These are invariants from ARCHITECTURE.md. Violating any is a blocking defect.

### Isolation constraints

TBD

---

## Testing

TBD

---

## Dependency Policy

TBD

---

## Logging

TBD
