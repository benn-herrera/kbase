# kbase

**Turn a human-targeted documentation corpus into an agent-friendly knowledge
base.**

kbase is a standalone batch appliance: point it at a Markdown doc set (LaTeX
later; PDF only via external preprocessing tools), and it produces a navigable
Markdown tree — entry point → domain index → subtopic index → leaf — with:

- **Verbatim leaves.** Leaf pages are faithful translations of the source, not
  paraphrases. Summaries exist only to route navigation; the generated KB's own
  contract states it plainly: *summaries route, leaves answer*.
- **Progressive hierarchical summaries** for navigation, built bottom-up.
- **Mechanically generated links.** Bidirectional tree navigation is emitted
  deterministically from the KB skeleton — dead links are impossible by
  construction. Cross-references come only from the source's own links; no
  inferred "related topics."
- **Self-description.** Every generated KB ships a `.agents/` directory with
  docent and maintainer agent definitions, a routing eval, and an entry-point
  contract — any agent that picks up the artifact finds its operating manual
  inside.
- **Provenance.** Each KB records app version, resolved model IDs, and source
  identity (commit/hash); the artifact is reproducible from that tuple.

It is a doc appliance, not a coding agent: non-interactive, fixed pipeline,
runs to completion and exits.

## Why

Large documentation sets (the first real target is Roblox's `creator-docs`,
millions of tokens) are written for humans reading in a tech writer's order.
Agents consume them badly: retrieval finds fragments without context, and
whole-corpus reading doesn't fit a context window. A curated topography — small
routing summaries over verbatim leaves — lets any agent navigate to the right
primary text and answer from it.

## How it works

A fixed multi-stage pipeline, deterministic wherever possible:

1. **Ingest + survey** (deterministic) — structural inventory: heading trees,
   section sizes, link graph.
2. **Taxonomy design** (model) — the KB skeleton, designed from the survey,
   never from raw source.
3. **Dissection** (mechanical cuts, model-refined, mechanically verified) —
   the source is sliced by verified byte offsets, never retyped.
4. **Distillation** (model, parallel fan-out) — verbatim source→leaf
   translation.
5. **Summaries + review** (model) — bottom-up index summaries; a
   find-the-discrepancy review pass flags problems for regeneration.
6. **Links + verification gates** (deterministic) — navigation emitted by
   template; integrity and drift gates run last.

Every model step emits data with a mechanically checkable post-condition, and
every model step has a deterministic fallback — model failure degrades
quality, never correctness.

## Requirements

- **Build:** Go (single static binary; prompts embedded).
- **Runtime:** an OpenAI-compatible API URL + key serving **gemma-4-family
  models** (local vLLM/llama.cpp/Ollama or any cheap cloud host). The appliance
  auto-detects gemma-4 models via `/v1/models` and refuses loudly on
  ambiguity. Creation is gemma-4-only by design; the *generated KB* is plain
  Markdown and works with whatever model you bring.

## Status

Early. The design is settled ([ARCHITECTURE.md](ARCHITECTURE.md)); the code is at
hello-world scaffolding. Validation plan: shakedown on the small Rojo v7 docs,
then the Roblox `creator-docs` prose domains.

## Development

Task recipes use [`just`](https://github.com/casey/just):

```
just            # list recipes
just edit-gate  # cheap gate: run after every change (fmt-check + vet)
just checkpoint # full gate: edit-gate + tests + build
just build      # host-platform binary → bin/kbase
just dist       # cross-builds: darwin-arm64, windows-amd64, linux-amd64
just cover      # aggregate test coverage
```

Design reference: [ARCHITECTURE.md](ARCHITECTURE.md). Implementation
specifics and constants: [SPEC.md](SPEC.md).

## License

[MIT](LICENSE). Note that generated KBs inherit the license of their source
corpus — e.g. a KB built from CC-BY-4.0 docs is itself a derivative of
CC-BY-4.0 material.
