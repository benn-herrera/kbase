# ACTIVE PLAN – kbase

**Status:** no plan under way. The port of kb_tools' KB toolchain into kbase is
complete: its outcomes are the contract documents (SPEC.md, ARCHITECTURE.md,
CONVENTIONS.md), and its evidence is the integration recipes `just --list`
names and their output under `test_data/transient/`. Future intent is
ROADMAP.md; partly-planned work waits in `ROADMAP_PLANS/`.

## Premise of the next plan

Inference R&D — the claim-graph asks, their templates and fragments, the letter
rules and defaults, the stage table and its inference classification — moves
to this repository. From the first tactic change landed here, kbase is the
canonical source of those prompts and algorithms, and nothing under
`kb_tools/` in adjagent changes except as a derivation of kbase's work: one
direction of novelty, never two. kb_tools stays the zero-install
implementation shipped with the agent set and tracks kbase through the
compatibility harness. Consequences when the plan is written: I7 flips
(kbase authors the templates under prompt-engineer review; adjagent's
provenance pins to kbase's commit); SPEC §2 names kb_tools the reference for
the KB contract and the mechanical stages and kbase the reference for the
inference layer; product code in Go, measurement tooling over the state
store's captures in Python under `tools/`; kb-testing's `compare-to-pristine`
ported first and run once against the prompts as imported, so the thread
starts from a baseline number.

## Open items

**Owner.** The live docent check (SPEC §3: a KB kbase builds is navigable by the
kb-docent agent): `/kb-start` and `/kb-next` in Claude Code and in opencode on
one kbase-built fixture under `test_data/transient/test-integration-build-arxiv/`,
against a checklist — entry point reached; descent by index; up-links followed;
a query answered; `session/` written and ignored by both verifiers. Agents
cannot open the interactive session this needs.

**Upstream findings for adjagent.**
- `kb-docent` and `kb-maintainer` name kb_tools' maintenance surface only; in a
  kbase-only KB they would run `kb_util` and fail. `kb-maintainer` states that
  re-authored values write a duplicate and that a work re-insert is refused,
  where kbase adopts and reports `unchanged` (SPEC §4); it warns against
  refreshing while siblings write without accounting for a write op's own
  trailing refresh (`--no-refresh`).
- For kb_tools (ROADMAP): resolved numbering; display names from `\input`-ed
  preambles; the `-latex_macros` reading; records-native leaves.
- The coder-definition rule the owner adopted for kbase (CONVENTIONS: change
  files only with the Edit and Write tools) belongs in adjagent's shared coder
  chunk.

**Carried forward in kbase** (facts, not fixes):
- A default-limit `solidity-below` result on a scored KB can exceed personant's
  8 KB cap; the six fixtures carry no scores (ARCHITECTURE §12).
- On Windows `status` cannot see a running build (no advisory lock,
  ARCHITECTURE §8).
- A block inside the abstract is treated as not at the margin; no corpus paper
  exercises it (ARCHITECTURE §5.5).
- Python edge cases not reproduced: non-ASCII digits, final-sigma lowercasing,
  a directory named `*.md`; line splitting on the rarer Unicode separators
  affects reported line numbers only.
- `test_data/fixtures/config/` is read by the live recipe and `internal/config`'s
  tests only.
