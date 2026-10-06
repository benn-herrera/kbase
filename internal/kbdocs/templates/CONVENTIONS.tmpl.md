# CONVENTIONS — {project-name} knowledge base

How to behave when operating on this knowledge base. The formats it conforms to are the toolchain's
own — every metadata byte here was composed by a write op from values a seat supplied, and an op
refuses values it cannot render and read back. This file is conduct, not format.

## What this tree is

Two graphs over one tree of Markdown files, beside which sit the images leaves embed, under
`assets/` directories. The **topography graph** is the navigation hierarchy: entry point, domain
indexes, subtopic indexes, leaves. The **claim graph** is a graph over results, of node kinds
{node-kinds}, registered in `claim-quality.md` files — the framework kinds in `invariants.md`
instead (below) — and materialized under `.index/`. Its `depends` and `supports` edges close no
cycle: solidity is undefined on one, and verify fails it. `references` edges carry no solidity and
may cycle; so may `demoted` edges, the `depends` edges the build's cycle breaking cut. A leaf is a container — its position label says where it sits in the hierarchy and
nothing about what it hosts.

Corpus invariants — notation and cross-cutting definitions — belong in `invariants.md`, which is
also the file the toolchain parses for framework nodes, so a line in it shaped like a declaration
becomes one.

## Use the agents

- `kb-docent` — reading and navigation, through `/kb-start` and `/kb-next`. Read-only on the KB;
  writes only under `session/`.
- `kb-maintainer` — every change: new leaves, corrections, frontmatter, claim-graph wiring, and the
  loop back to green.
- The build (`kbase build`, or kb_tools' `/kb-build`) built this tree. It is not rerun for
  incremental work.

Run `/kb-start` and `/kb-next` from the project root — the directory holding this KB — or from
inside the KB root itself. Those are the two invocation points the commands support; anywhere else
they stop and ask. Once a session is running, do not change directory while exploring: navigation is
the docent's job, and moving the session's ground out from under it breaks the paths it is tracking.

Working the KB without them is the failure this file exists to prevent. The constraints below are
not discoverable by reading the files, and an edit that looks right while violating one of them
either fails at the gate or passes and leaves the graph quietly wrong.

## Two toolchains

Two toolchains maintain this KB, and each reads and writes what the other does: **kbase**, a single
binary run from the project root, and **kb_tools**, a Python package installed under the project's
`.claude/agents/`, or under `.opencode/agents/` with opencode, which then replaces `.claude/agents`
in the commands below. Pick either for a piece of work and stay
with it until the gate is green.

| Operation | kbase | kb_tools |
|---|---|---|
| A metadata write | `kbase <op>`, values as YAML on stdin or in a `--values` file | `PYTHONPATH=<project-root>/.claude/agents python3 -m kb_tools.kb_util <op> --values <file.toml>` |
| The op list | `kbase --help`; `kbase <op> --help` | `... -m kb_tools.kb_util --help`; `... -m kb_tools.kb_util <op> --help` |
| Regenerate derived state | `kbase refresh` | the runner's `kb-refresh` target |
| The read-only gate | `kbase verify` | the runner's `kb-verify` target |
| Claim-graph counts | `kbase stats` | the runner's `kb-stats` target |

The ops carry the same names on both sides. kbase writes one YAML document to stdout per command,
its `outcome` key the result — `refused` (exit 1) and `retry` (exit 2) are kb_tools' 7 and 8;
kb_tools prints report lines. kb_tools runs under the `python3` first
on `PATH`, which must be 3.11 or later, standard library only. The toolchains' prose contracts stay
in their source repositories and are not installed into a consuming project, so no document under
this project root holds them.

Where the harness has MCP, the `kbase` MCP server (`kbase mcp --kb-root <path>`) offers kbase's
subcommands as tools, one to one, under the same names and returning the same documents, so what
this file says of a subcommand holds for its tool; a write op's values are its tool's arguments,
one entry per call. A document whose exit code is not 0, a refusal among them, arrives as a tool
error carrying the same items and remedy, and the tool list implies no order of work.

## Authored versus derived

Authored: leaf content, leaf frontmatter, register entry text, dependency membership, and local
rigor (`confidence` on a claim, `quality` on a support). Derived and regenerated: solidity, build
status, the parenthetical solidity annotations, subtree aggregates, the `Leaf references:` line in
each register entry, the `claim-graph.svg` and `claim-graph-digest.svg` sheets, and everything under `.index/`. **A hand-edited derived
field is a verifier failure**, not a shortcut — the value is overwritten and the drift is reported.

Authored does not mean typed. Everything authored except leaf prose is metadata, and metadata is
written only by a write op: you supply values, the op composes the format, proves what it wrote by
reading it back, and refuses rather than writing what it cannot prove. It mints an entry's id itself
and reports it, so no id is ever chosen. A values file carrying a key its op does not take is refused
with the offending key, its position, and the complete set the op does take, so the call you would
have made anyway is what answers the question.

## The gate

Refresh regenerates derived state; verify is the read-only gate. Refresh before verify, always, and
leave the gate green. Run kb_tools' refresh and verify through this project's runner targets rather than invoking its modules
directly. A failure marked refresh-fixable means run refresh; anything else is a real defect in what
was authored, and re-running will not clear it. kbase's verify also lists every `demoted` edge under
`findings`, which are not failures; kbase's `resolve-demoted` removes one or restores it to
`depends`, refusing a restore that would close a cycle.

`claim-graph.svg` and `claim-graph-digest.svg` beside this file and `<volume>/claim-graph.svg` in
each volume are the derived files no verifier reads; the digest and the volume sheets exist only where
the KB holds two or more volumes with claims. Refresh redraws them on every run, so a hand
edit to one is not reported — the next refresh overwrites it. kbase draws them through Graphviz
`dot`; without `dot` on `PATH` it draws nothing, leaves existing sheets as they stand, and writes a
placeholder `claim-graph.svg` only where none exists. Both toolchains' link checks cover `kb-root/` only, and every verify failure is the
same failure on both sides.

## Where things may be written

Create files only at targets an assignment names. Output with no assigned home is a question to
raise, never a location to choose.

`AGENTS.md` and `CONVENTIONS.md` in this tree belong to the build, not to work done in the KB
afterwards: the build stamps them at its validation gate — `AGENTS.md` carrying this project's
scope pin — and leaves whole whichever of them a project had already authored for itself. Beside
them the same stamp writes `CLAUDE.md` as the one line `@AGENTS.md`. No agent authors or edits any
of the three, and corpus content belongs in none — it belongs in `invariants.md`.

## Citations

In authored files — registers, indexes, summaries, `invariants.md`, and meta-docs — a reference to a
node is a structured field, a marker, or a link, never an id or a rule name sitting loose in prose.
A cited authority carries the minimal quoted excerpt that makes the citing statement falsifiable
where it stands, linked to a durable path in this tree; one clause, not the passage, because a
citation that reproduces its source has become a second copy of it. Where a reference genuinely
warrants no graph edge, the exemption is the entry's `no-edge` reason, supplied to the op that
writes the entry and standing in no other form. `kbase render-citation` prints the sanctioned
citation string for an authority.

The verifier enforces the exact shapes, and the ops refuse values that would not survive it.
