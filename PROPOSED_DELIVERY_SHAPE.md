# Proposed delivery shape: the skill, and how kbase reaches a model

**Status.** A proposal. The skill delivery is not built; the back end this page once described was
built, tried and dropped. It records the decisions taken about how kbase is delivered to a user and
how a scoring run reaches a model, so that the work after this release starts from them.

## 1. The skill is kbase's delivery, and its text is adjagent's

The delivery package is a skill: a routing file and one reference per verb family — building, reading,
scoring, maintenance. That is the shape an agent reads, and it is the shape kbase takes.

**adjagent owns the text.** The references are generated from adjagent's own definitions and rendered
per harness; kbase has no skill files and no skill source. What kbase supplies is the interface those
references point at: the MCP tool descriptions and the verbs' own output — refusals, `verify` findings,
`show-config`, `--help`. A reference states procedure and points at that surface; it never restates a
rule, a vocabulary or a number, because every copy is a second source that goes stale on the next
release.

**The scoring criteria are not part of that surface.** They live in the ask templates embedded in the
binary, which travel to the model as the request's payload. They are not documentation, and a
reference must not describe them.

**Harness differences are rendering, not content.** The text carries no harness nouns — no harness's
tool names, no config paths, no runtime-specific instructions. Each of those would be rendered
faithfully into every harness, where each wanted something else.

**The MCP entry is trackable.** The registered command is a path to the installed binary, relative to
the project, and `--kb-root` is omitted: the server walks up to `.git` and serves the `kb-root/` beside
it, the same resolution every verb uses. So `.mcp.json` and `opencode.jsonc` carry nothing
machine-specific. Both relativities rest on one premise — the harness starts the server inside the
project — and both fail loudly when it does not: a missing command, or a KB that is not there.

**There is no version pairing to build.** adjagent installs the kbase release it was built against
(`KBASE_VERSION`) and drives that binary; the release and the skill move in one act. kbase does not
check the pairing, and what a mismatch looks like is a reference naming a verb the installed binary
does not have.

## 2. kbase has no harness awareness

kbase reaches a model one way: the provider API it already speaks. There is no command-based back end
and no adapter, so no code outside the provider client knows what a harness is.

**The back end that was built, and dropped.** An `opencode` back end was implemented and run against a
real KB: it composed its own argv and environment, defined a tool-less agent inline for the process,
and returned a closed answer. It worked, and it is not viable. opencode's process start-up costs tens
of seconds per invocation, and a scoring walk invokes it once per item — 51 asks for a rigor pass and
96 for a bearing pass on one KB. The walk spends longer starting processes than asking models. It is
not finished, and nothing in 0.8.0 should be read as shipping it.

What would reopen the question: a back end whose per-item start-up is in the noise against the model
call — a long-lived server mode, or a batch mode answering many asks in one process — and the
guarantees in §4. Absent that, a command-based back end is not a path kbase supports.

## 3. The profile mechanism goes away

There are no user-supplied commands. With no back end shipping, the whole mechanism goes: a scoring run
reaches a model through the provider API and nothing else.

What goes with it:

- the opencode back end: its composition, its version gate, and its tests;
- `--harness`, and the records and vocabulary that exist for a back end: `back-end: harness`, the
  `harness` and `harness-layer` fields, `show-config`'s `harnesses` and `harnessesExists`, the
  `check: harness` class, and the `--scoring-model` reading that only a back end used;
- the seam `KBASE_SCORE_BACK_END_COMMAND`, its stderr announcement and its `command_override` header;
- `internal/harness`, and its exec owner;
- `harnesses.toml`, its loader and layer merge (already gone in 0.8.0), and the `harnessesTemplate`
  constant with its `configure --create` use.

**Why.** Two arguments, and either suffices. A user who assembles a command that both reaches a model
correctly and answers in the exact form the driver requires is a low-odds bet whose failures are
silent, because a wrong answer still parses. And a back end that starts a process per item has to
start it fast enough to disappear against the model call; opencode's does not. Supported means shipped,
tested and fast enough to be worth running.

## 4. What a back end would have to satisfy

Kept as the bar for reopening the question, not as a contract anything implements. SPEC §8's *harness
back end* is the contract text; this is its checklist:

- one process per item whose start-up is in the noise against the call, the prompt whole, and the reply
  parsed by the same rule a provider's is;
- no tools, and no instructions of its own;
- no re-issue: one attempt per item, so a retry kbase cannot see is a variance it cannot account for;
- failure loud — a non-zero exit with a reason, never a fabricated answer.

## 5. What this leaves for adjagent

adjagent retires `definitions/kb_claim_scorer.py` and the agent definition it renders. The scoring
prompt carries the whole instruction set now, so an agent definition left in place adds instructions
the run cannot account for. `ROADMAP_PLANS/PLAN_3_TOOLING.md` B.1 already requires the retirement.

Its `scoring` reference also carries facts that are false or about to be: the ledger records `done`,
`failed` and `pending` (not `malformed`); the answer comes through the provider API, not a back end;
and there is no `--harness` flag to mention. A reference naming a retired flag costs a user the time to
type it and read the refusal.

## 6. Still open

- Whether the integration suite's command-path legs have a provider equivalent or go with the back end;
  the commit that removes it says which.
- Whether the provider path stays the only way a scoring run reaches a model, or a future back end
  reopens it — §2's bar is the condition.

## 7. What this supersedes

Text that already carries an obligation this page voids or restates, and must move with it:

- `SPEC.md` §8's *harness back end*, *The model*, and *The built-in opencode back end*; §9.3 and §9.7's
  `harnesses`/`harnessesExists`; §11's `score` text wherever it names a back end.
- `CONVENTIONS.md`'s `internal/harness` paragraph and its exec-owner list.
- `USER_CONVENTIONS.md`'s `harnesses.toml` paragraph and its opencode bullet; `README.md`'s harness and
  opencode passages; `ROADMAP.md`'s *Configuration* bullet.
- `ROADMAP_PLANS/PLAN_3_TOOLING.md` B.1 and B.3.
