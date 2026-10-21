# Proposed delivery shape: the skill, and how kbase reaches a model

**Status.** A proposal. Nothing here is built, and none of it is in 0.8.0. It records the decisions
taken about how kbase is delivered to a user and how a scoring run reaches a model, so that the work
after this release starts from them.

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
binary, which travel to whatever back end runs the ask as payload. They are not documentation, and a
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

## 2. Harness awareness lives in the Ask internals

The only place kbase may know what a harness is, is the Ask implementation for that back end. Outside
it kbase is harness-neutral.

One place breaks that today: the `harnessesTemplate` constant in `internal/config/template.go`, which
ships a profile naming one harness's flags. That constant and its use in `configure --create` go; the
file's other two templates stay.

## 3. The profile mechanism goes away

There are no user-supplied commands. The driver internalises the command lines for the back ends it
ships.

**`--harness` names the back end kbase ships.** The set is `opencode` today, and any other value is
refused naming it. The flag survives because the direct provider path still answers a run that names
no back end; collapsing the two belongs with §8's question of whether that path survives.

What goes with the mechanism:

- `harnesses.toml`, its loader, and the layer merge that let a repository replace a global profile.
- The `{model}` and `{prompt}` token contract, and the refusals that policed it — a model id beginning
  with `-`, a `--scoring-model` the command cannot carry, a `{prompt}` token in two elements.
- The records the SPEC keeps for it: `back-end: harness`, the `harness` and `harness-layer` fields,
  `harnesses` and `harnessesExists` in `show-config`, and the `check: harness` class.

**Why.** A user who assembles a command that both reaches a model correctly and answers in the exact
form the driver requires is a low-odds bet, and its failures are silent, because a wrong answer still
parses. The support cost is high and recurring. Supported therefore means shipped and tested.

## 4. One internal seam survives

The integration suite fakes a back end: an over-limit reply, a non-zero exit, a leftover file, a
megabyte of stderr. With profiles gone there is no injection point, so one override remains: an
environment variable naming a command for the duration of the run — `KBASE_SCORE_BACK_END_COMMAND`,
parsed and refused in one place. It is outside the support surface and outside the child's
environment, and it is not silent: a run that sees it announces it once on stderr and records it in
the run's header as `command_override`. A stray inherited variable must never score a KB with a
command nobody chose and nothing to say so.

## 5. opencode: v2 only

The adapter targets opencode v2. A run against a lower major is refused before the first item, and
there is no pin below the major: opencode updates often, and a build-exact pin breaks minutes after an
upload.

Mechanism: `opencode run`, plus the inline config environment variable that defines the agent for that
process — no tools, no instructions of its own — with the CLI options that go with it. Nothing is
written to the user's configuration, so there is no file to clean up, no stale definition after a
crash, and no name to collide with a user's own agent. None of it reaches the user: there is no config
to tune and no agent definition to edit; the adapter builds both.

Instruction discovery is the hole tool removal does not close. The item's directory is inside the run's
ledger, which may sit in a user's project, so the inline config has to stop the upward walk as well.

**The scoring model.** The adapter is handed the scoring role's model and passes it in whatever form
the back end's own flag or inline config takes. A model the back end cannot name is refused before the
walk, never dropped, and the run records the id it passed.

**Nothing verifies this at run time.** The digest records what kbase sent, not what the harness added:
an instruction-bearing run still returns a closed answer. What would show it is a fixture run from a
directory with an instruction file above it, whose charges differ from the same run in a clean
directory. That is a manual probe, run when the assumption is re-checked, and its command and result
belong in the commit.

## 6. What a back end must satisfy

The contract is SPEC §8's harness back end, which each adapter implements and where SPEC wins on any
disagreement. As a checklist for an adapter:

- one process per item, the prompt whole, and the reply parsed by the same rule a provider's is;
- no tools, and no instructions of its own;
- no re-issue: one attempt per item, so a retry kbase cannot see is a variance it cannot account for;
- failure loud — a non-zero exit with a reason, never a fabricated answer.

## 7. What this leaves for adjagent

adjagent retires `definitions/kb_claim_scorer.py` and the agent definition it renders. The scoring
prompt carries the whole instruction set now, so an agent definition left in place adds instructions
the run cannot account for. `PLAN_3_TOOLING.md` B.1 already requires the retirement.

Its `scoring` reference also carries two facts that are already false: the ledger records `done`,
`failed` and `pending` (not `malformed`), and under this proposal the answer comes through a back end
rather than the configured provider.

## 8. Still open

- Whether a second back end ships — claude alongside opencode — and with it the selector that names
  more than one value.
- Whether the direct provider path stays beside the adapters, or the adapters are the only way a
  scoring run reaches a model.

## 9. What this supersedes

Text that already carries an obligation this page voids or restates, and must move with it:

- `SPEC.md` §8's *harness back end* and *The model*, §9.3, and §9.7's `harnesses`/`harnessesExists`.
- `CONVENTIONS.md`'s `internal/harness` paragraph and its exec-owner list — a back end that execs a
  program needs an owner, as `internal/harness` has one today.
- `USER_CONVENTIONS.md`'s `harnesses.toml` paragraph, and `ROADMAP.md`'s *Configuration* bullet.
- `PLAN_3_TOOLING.md` B.1 and B.3.
