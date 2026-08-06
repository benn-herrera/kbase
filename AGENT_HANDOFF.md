# AGENT_HANDOFF — orienting a fresh context

All design decisions live in [PROJECT.md](PROJECT.md). Read it first, in full. This
file carries only what a fresh context needs that PROJECT.md deliberately doesn't
hold: provenance, sibling-repo geography, operational quirks, and next actions.

## Where this project came from

kbase emerged from an extended strategy conversation (2026-08-05/06) held in
`/Users/benn/projects/insurance-gap-analysis`. Several commercial incarnations of
"KB-ification" were evaluated there and **parked** — insurance coverage-gap analysis
(shelved on founder opportunity-cost math), a horizontal KB-ification API (window
judged closed: Context7, Mintlify, first-party vendor docs-AI). Do not re-litigate or
resurrect the business framings; kbase is deliberately the personal-tool residue of
that analysis. If the business history matters, see
`insurance-gap-analysis/kb-personant-tech-monetization.md` (predates the parking
decisions — the shelving happened in conversation, recorded in that project's agent
memory, which does **not** auto-load in this directory's sessions).

Immediate personal motivation: Benn is pursuing a Roblox opportunity; a KB of the
Roblox `creator-docs` prose domains is near-term useful for learning the platform
(Rojo was assessed too: ~13k tokens total, below the complexity gate — shakedown
corpus only).

## Sibling-repo geography

| Path | Role for kbase |
|---|---|
| `../insurance-gap-analysis/kb_tools/` | **Code ancestor.** Stdlib-only Python deterministic spine (refresh/verify/link gates, id/schema discipline). Its `AGENTS.md` documents the KB concepts (topography graph, verbatim leaves, derived-vs-authored) that kbase inherits — minus the claim graph, which kbase drops. |
| `../personant/` | **Philosophy donor, explicitly not a code donor** (PROJECT.md records why). `ARCHITECTURE.md` there is the canonical statement of the house thesis, model-family-as-platform, and the gemma-4 tier table. Personant targets gemma-4 served locally on `reaper` (M5 Max) — kbase should also work against that endpoint, or any cheap cloud gemma-4 provider. |

## External facts already established (verified 2026-08-06, may age)

- **Rojo docs:** current version lives in `rojo-rbx/rojo.space` repo `docs/` (Docusaurus
  convention — `versioned_docs/` holds only frozen v0.5/v6 snapshots; v7 *is*
  `docs/`). MIT.
- **creator-docs:** `Roblox/creator-docs`, content under `content/en-us/`, CC-BY-4.0,
  actively updated (daily-ish). `reference/` is auto-generated engine API material —
  annex territory per PROJECT.md.
- **Roblox first-party AI-docs surface exists** (llms.txt at
  `create.roblox.com/docs/llms.txt`, `.md` per-page endpoints, official Studio MCP,
  community docs-MCPs like `mcp-roblox-docs`). These are retrieval tools; kbase's
  value claim is the curated-topography/docent experience, not retrieval. Worth
  re-checking state before investing in overlap areas.

## Working conventions (Benn)

- **Never proceed with a plan in response to a question** — answer first, act only on
  explicit go. This is a hard invariant in Benn's project contracts.
- Dispatch heavy work to sub-agents; keep the main loop free for discussion.
- Concise replies (≤200 words unless detail is requested). No unearned praise.
- DRY as named constants; single-source-of-truth; two-gate build discipline (cheap
  edit gate per change, full suite once at checkpoints) — see sibling repos'
  CLAUDE.md/AGENTS.md for the pattern kbase's own justfile should eventually follow
  (decided 2026-08-06: `just` over make — task-runner use only, no timestamp graphs
  needed with Go's own incrementality; siblings' Makefiles are the pattern source,
  not the tool choice).
- Benn thinks by adversarial riffing — expect design to evolve through pushback, and
  push back honestly when a proposal collides with a recorded principle.

## Operational quirks

- Agent processes run sandboxed as `agent-user` (group `agent-group`). Project dirs
  created by Benn default to `drwxr-xr-x` — **no group write** → `EACCES` on first
  write. Fix is Benn running `chmod g+w <dir>` (suggest the `!`-prefix shell escape).
  Surface and stop on sandbox obstructions; never work around them.
- This directory currently has no git repo, no justfile, no CLAUDE.md/AGENTS.md of its
  own. Those are early scaffolding tasks.

## State and next actions

Nothing is built; PROJECT.md is the only artifact. Open items, roughly in order:

1. ~~Resolve the open decision~~ **Resolved 2026-08-06: pure Go, single static
   binary** — no shell-out, no ad-hoc Python execution; kb_tools gates get ported.
   Token counting also decided: calibrated chars-per-token heuristic, no cgo
   tokenizer, no `/tokenize` dependency (PROJECT.md §3/§4).
2. Repo scaffolding — **partially done 2026-08-06**: git init ✓ (nothing committed
   yet), hello-world Go skeleton ✓ (`cmd/` + `internal/version/`, personant layout,
   `bin/` output), justfile ✓ (build/test/edit-gate/checkpoint/clean/dist/nuke; dist
   targets darwin-arm64, windows-amd64, linux-amd64). Remaining: CLAUDE.md/AGENTS.md.
   Note: agent-user needed `git config --global --add safe.directory` for this repo
   (`.git` is agent-user-owned inside a benn-owned dir); Benn's own git may want the
   same exception.
3. Build order that exploits determinism-first: ingest adapter (Markdown) + survey +
   mechanical splitter need **no model at all** — they can be built and tested against
   the Rojo clone immediately. Model-dependent stages follow.
4. gemma-4 prompt/agent definitions + their eval harness (the family-tuning
   discipline) — the long pole among the model-facing pieces.
5. Rojo shakedown end-to-end, then creator-docs prose domains.
