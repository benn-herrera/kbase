# CONVENTIONS — {project-name} knowledge base

How to behave when operating on this knowledge base. The formats it conforms to are the toolchain's
own — what a write op composes here was composed from values a seat supplied, and an op refuses
values it cannot render. This file is conduct, not format.

## What this tree is

Two graphs over one tree of Markdown files, beside which sit the images leaves embed, under
`assets/` directories. The **topography graph** is the navigation hierarchy: entry point, domain
indexes, subtopic indexes, leaves. The **claim graph** is a graph over results, of node kinds {node-kinds}, held as kbase's records in
`.index/` and shown for reading on generated `claim-rigor.md` pages. Its four scoring relations —
`supports`, `founds`, `lifts` and `refutes` — close no cycle: the derived value is undefined on
one, so the build and `refresh` demote one edge of any cycle they find, and a write op refuses an
edge that would close one. `references` and `demoted` edges are unscored; a `demoted` edge is a
scoring edge demoted to break a cycle, kept whole so `resolve-demoted` can restore it. A leaf is a
container — its position label says where it sits in the hierarchy — and the claims, analyses and
experiments it hosts are marked in its prose.

Corpus axioms — notation and cross-cutting definitions — are stated in `axioms.md`, one
`### Axiom <label>: <title>` heading each, its label and title those `insert-axiom-entry` recorded.
Prose may call an axiom an invariant or a framework premise; the graph has one word for it.

## Use the agents

- The `kbase` skill — how the KB is reached: it locates this tree and routes to reading, maintenance
  or building.
- `kb-maintainer` — every change: new leaves, corrections, records, claim-graph wiring, and the
  loop back to green.
- A build (`kbase build`) writes this tree where it does not already stand; it is not rerun for
  incremental work.

The skill locates this KB itself. Once it has, do not change directory while exploring: navigation
is the reading route's job, and moving the session's ground out from under it breaks the paths it
is tracking.

Working the KB without the skill is the failure this file exists to prevent. The constraints below
are not discoverable by reading the files, and an edit that looks right while violating one of them
either fails at the gate or passes and leaves the graph quietly wrong.

## The toolchain

kbase, a single binary run from the project root, maintains this KB.

| Operation | Command |
|---|---|
| A metadata write | `kbase <op>`, values as YAML on stdin or in a `--values` file |
| The op list | `kbase --help`; `kbase <op> --help` |
| Regenerate derived state | `kbase refresh` |
| The read-only gate | `kbase verify` |
| Claim-graph counts | `kbase stats` |
| Edge counts by relation and bearing | `kbase edge-census` |
| The edges of a relation | `kbase edges --relation <relation>` |

Every subcommand but `help`, `completion` and `mcp` writes one YAML document to stdout, its
`outcome` key the result: `refused` exits 1, `retry` exits 2, and `kbase --help` states the whole
exit table. kbase's prose contract stays in its source repository and is not installed into a
consuming project, so no document under this project root holds it.

Where the harness has MCP, the `kbase` MCP server (`kbase mcp`; a call names its project with
`project-dir` where the server's instructions ask) offers kbase's subcommands as tools, under the
same names and returning the same documents, so what this file says of a subcommand holds for its
tool; a write op's values are its tool's arguments, one entry per call. `rewind-build` is the one
exception: no subcommand stands behind it; read-only, it reports the build's trail and changes
nothing. A document whose exit code is not 0, a refusal among them, arrives as a tool error carrying
the same items and remedy, and the tool list implies no order of work.

## Authored versus derived

Authored: leaf content and the statement markers in it, and kbase's records — nodes, their local
`rigor`, rationale and the rest, and edges with their `bearing`. Derived and regenerated:
`strength`, build status, subtree aggregates, the `claim-rigor.md` pages, and the
`claim-graph.svg` and `claim-graph-digest.svg` sheets. **A hand-edited `claim-rigor.md` page is a
verifier failure**, not a shortcut; `refresh` overwrites every derived file.

Records and the statement markers are written only by a write op (`mark-claim-in-leaf` places or
moves a marker), never by hand: you supply values, the op composes the record, proves each marker
it placed by reading it back, and refuses rather than writing what it cannot prove. It mints an entry's id
itself and reports it, so no id is ever chosen. A values file carrying a key its op does not take is
refused with the offending key, its position, and the complete set the op does take, so the call
you would have made anyway is what answers the question.

## The gate

Refresh regenerates derived state; verify is the read-only gate. Refresh before verify, always, and
leave the gate green. A failure marked refresh-fixable means run refresh; anything else is a real
defect in what was authored, and re-running will not clear it. `verify` also lists every `demoted` edge under `findings`, which are not failures;
`resolve-demoted` removes one or restores the edge it replaced, refusing a restore that would close
a cycle. After a merge the load may be degraded; `refresh` repairs it, and its result's `warnings`
name every edge it dropped or demoted. Report them: nothing else records a drop.

`claim-graph.svg` and `claim-graph-digest.svg` beside this file and `<volume>/claim-graph.svg` in
each volume are the derived files no verifier reads; the digest and the volume sheets exist only where
the KB holds two or more volumes with nodes. Refresh redraws them on every run, so a hand
edit to one is not reported — the next refresh overwrites it. kbase draws them through Graphviz
`dot`; without `dot` on `PATH` it draws nothing, leaves existing sheets as they stand, and writes a
placeholder `claim-graph.svg` only where none exists. Only documents under `kb-root/` are
link-checked; a target resolves against the whole repository.

## Where things may be written

Create files only at targets an assignment names. Output with no assigned home is a question to
raise, never a location to choose.

`AGENTS.md` and `CONVENTIONS.md` in this tree belong to the toolchain, not to work done in the KB
afterwards: kbase stamps them where none stands — at birth through `kbase init`, at a build's
validation gate — and leaves whole whichever of them a project had already authored for itself.
Beside them the same stamp writes `CLAUDE.md` as the one line `@AGENTS.md`. No agent authors or
edits any of the three, and corpus content belongs in none — it belongs in `axioms.md`.

## Citations

In authored files — indexes, summaries, `axioms.md`, and meta-docs — a reference to a node is a
structured field, a marker, or a link, never an id or a rule name sitting loose in prose.
A cited authority carries the minimal quoted excerpt that makes the citing statement falsifiable
where it stands, linked to a durable path in this tree; one clause, not the passage, because a
citation that reproduces its source has become a second copy of it. Where a reference genuinely
warrants no graph edge, the exemption is the entry's `no-edge` reason, supplied to the op that
writes the entry and standing in no other form. `kbase render-citation` prints the sanctioned
citation string for an authority.

The verifier enforces the exact shapes, and the ops refuse values that would not survive it.
