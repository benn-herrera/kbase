# Proposed delivery shape: the skill, and how kbase reaches a model

**Status.** A proposal. Nothing here is built, and none of it is in 0.8.0. It records the decisions
taken about how kbase is delivered to a user and how a scoring run reaches a model, so that the work
after this release starts from them rather than rediscovering them.

## 1. kbase ships as a skill

The delivery package is a skill: an entry point and one reference per verb family — building, reading,
scoring, maintenance. That is the shape an agent reads today, and it is the shape kbase takes.

**kbase owns the content; adjagent owns the rendering.** kbase's text is harness-neutral, and adjagent
takes it and produces each runtime's tree, handling the harness differences that matter. kbase does not
grow a renderer: owning the whole skill would mean rebuilding adjagent's rendering and agent-set
handling inside kbase, which is the one thing the split exists to avoid.

The test of the boundary: **if a change to the skill would need kbase to know what a harness is, it
isn't kbase's change.** In practice that means no tool names, no config paths and no runtime-specific
instructions in kbase's references — each of those would be rendered faithfully into every harness,
where each wanted something else.

Both projects are maintained and released in lockstep by the same person, so a rendered skill and the
binary it drives share one identity. No version-pairing mechanism is built: there is nothing to pair.

**The limit.** A skill is prose, and kbase's rule is that no test asserts a prompt's prose. "The
package works" is therefore checked by walking the documented procedure against a fixture — an
acceptance run, not a diff.

## 2. Harness awareness lives in the Ask internals

The only place kbase may know what a harness is, is the Ask implementation for that back end. Outside
it, kbase is harness-neutral.

One place breaks that today: `internal/config/template.go` ships a profile naming one harness's flags,
and documents another's missing ones. Under this proposal the file goes entirely (§3).

## 3. The harness profile mechanism goes away

There are no user-supplied commands. The driver internalises the command lines for the back ends it
ships, and a run selects among those.

What goes with it:

- `harnesses.toml`, its loader, and the layer merge that let a repository replace a global profile.
- The `{model}` and `{prompt}` token contract, and the refusals that policed it — a profile naming no
  model, a model id beginning with `-`, a token in two elements.
- The shipped profile template, and with it the one place kbase named a harness outside the Ask
  internals.

**Why.** A user who assembles a command that both reaches a model correctly and answers in the exact
form the driver requires is a low-odds bet; the failures are silent, because a wrong answer still
parses. The support cost is high and recurring. Supported therefore means shipped and tested: the back
ends kbase names are the ones it tests, and anything else is out of scope rather than "you might be
able to make it work".

## 4. One internal seam survives

The integration suite fakes a back end: an over-limit reply, a non-zero exit, a leftover file, a
megabyte of stderr. With profiles gone there is no injection point, so one override remains inside the
driver — an environment variable naming a command for the duration of the run. It is undocumented, not
part of the support surface, and the same shape as §2: harness knowledge reachable only from inside.

Without it the hermetic integration targets regress to asserting against whatever harness happens to be
installed.

## 5. opencode: v2 only

The opencode adapter targets v2, the version in use and the only one supported. There is no pinning
below the major: opencode updates often, and a build-exact pin fails minutes after an upload.

Mechanism: `opencode run`, plus the inline config environment variable that defines the agent for that
process — no tools, no instructions of its own — together with the CLI options that go with it. Nothing
is written to the user's configuration, so there is no file to clean up, no stale definition after a
crash, and no name to collide with a user's own agent.

None of this reaches the user. There is no config to tune and no agent definition to edit; the adapter
builds it.

The working directory is inside the run's ledger, which may sit within a user's project, so
instruction discovery upward has to be suppressed by that inline config too — tool removal alone
leaves the instructions channel open.

**No probe layer.** The first run against a fixture is the experiment. What the adapter sends is in the
run's digest, so a wrong assumption about the mechanism is visible afterwards rather than inferred
forward.

## 6. What a back end must satisfy

The requirements stated in the scoring handoff become the contract each adapter implements:

- one process per item, the prompt whole, and a closed answer on stdout;
- no tools, and no instructions of its own;
- one attempt per item, so a retry kbase cannot see is a variance it cannot account for;
- failure loud: a non-zero exit with a reason, never a fabricated answer.

The adapter is where these are met, which is why the adapter is the only harness-aware code.

## 7. What this leaves for adjagent

- Render the skill (§1).
- If it ever acts as a scoring back end, implement §6 itself. That is a choice adjagent holds, not a
  need kbase has.
- Retire its own scoring agent definition, and the references that name it: the scoring prompt now
  carries the whole instruction set, so an agent definition left in place adds instructions the run
  cannot account for.

## 8. Still open

- Which back ends ship in the first cut — opencode alone, or claude alongside it.
- What, if anything, replaces `--harness` on the command line.
- Whether the direct provider path stays beside the adapters, or the adapters are the only way a
  scoring run reaches a model.
- The name and home of the override in §4.
- Sequencing against the revised scoring scheme: the `scoring` reference is the only document that
  changes when that scheme lands.
