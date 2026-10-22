# Phase 3 — the consumers (named, not done)

Status: blocked on Phases 1 and 2; external owners. **Nothing in this repository lands in this
phase.** It is the hand-off checklist for the two repositories that must follow the format and the
scoring command.

## Why it is a phase and not a gate

Neither consumer can block Phases 1 and 2: the compat harness is flag-gated and skips without its
flags, and the adjagent definitions are installed from another repository. Naming the work here keeps
it from disappearing, and gives each owner an acceptance they can run without reading this thread.
The kbase landings do not wait for either.

## A — the reference toolchain reads `2.0.0`

**Owner:** the `kb-tools` checkout the compat harness is pointed at (a transient tree, not this
repository).

**What must change.** Its schema and loader accept the `2.0.0` form and refuse nothing it should
read: the three field names (`rigor`, `bearing`, `strength`), the five node types (`claim`,
`analysis`, `experiment`, `work`, `axiom` — no `support`, no `invariant`), the four relations plus
`references` and `demoted`, the three-letter id prefixes (`clm-`, `ana-`, `exp-`, `wrk-`,
`axm-<label>` with its own label alphabet), the `analysis-nodes` leaf key, the `edges.yaml`
index file (`supported-by.yaml` is no longer written), and the pending literal on a `bearing`. Its
version constant stops being `1.0.0`. Whether it keeps reading `1.0.0` is its owner's
decision; kbase's converter is not a compatibility reader and does not serve it.

**The register's new names.** A converted KB's registers are `claim-rigor.md` files whose entries
carry a `### Rigor` heading, a `<!-- claim-rigor: … -->` Tier-2 marker and a `### Strength` block for
the derived value. Any reader that opens `claim-quality.md` by name, matches `### Quality`, or greps
for `<!-- claim-quality:` must follow all three, or it finds no register in any migrated KB. The old
names survive only inside `internal/migrate`'s read of `1.0.0` bytes.

**The harness side, split by landing.** At `7e0cfee` (landed) Phase 1 renamed
`internal/index/compat_test.go`'s case table and its two regexes to the `2.0.0` names; the node-pass
rework carried the node words (`ana-`, `analysis-nodes`, `analysis`) through the
same table, and the two regexes are `pendingScore` and `pendingArc`
(`internal/index/compat_test.go:176-177`). Nothing in this phase edits that file; if the comparison
finds a mismatch, the finding is fixed on whichever side is wrong, not by relaxing the case.

**The check.** `-reference.out` is a directory this run creates and names. The three source paths and
the fixture ids are the kb-tools checkout's: this repository cannot name them, so its README is the
source. The form is:

```
go test ./internal/index -run 'TestStageReferenceComparisons|TestReferenceComparisons' -args \
  -reference.stage=false \
  -reference.out <dir> \
  -reference.minikb <minikb path> \
  -reference.materialized <materialized path> \
  -reference.fixtures <fixtures dir> \
  -reference.ids "<ids the kb-tools README lists, space-separated>"
```

`-reference.minikb`, `-reference.materialized`, `-reference.fixtures` and `-reference.ids` are the
harness's own flags (`compat_test.go:28-33`), and the checkout's README names the directory each
points at and the fixture ids it holds — record both in the run's report. The stage leg
(`-reference.stage=true`) writes the inputs; the comparison leg reads what both toolchains left.

**Acceptance.** The comparison is green on the mini-KB and every fixture the README lists, with the
exact `-reference.*` values and the id list in the run's report. The reference finds a converted
KB's registers under `claim-rigor.md` with their `### Rigor` headings and `<!-- claim-rigor: … -->`
markers, and reads `wrk-`/`axm-` ids without a dead-link report. A difference that remains is
attributed in that report to the reference's reader or to one of Phase 1's three value changes,
never waived. Evidence: the command, the comparison output, and the fixture list.

**Not a gate.** CI does not run it, with or without the flags; the harness skips.

## B — adjagent retires the scorer and routes scoring to `kbase score`

**Owner:** the adjagent repository; follow `CLAIM_SCORE_BEARING.md` there.

**What must change.**

1. `definitions/kb_claim_scorer.py` is deleted. No replacement agent is installed, and the two seats
   `CLAIM_SCORER_SEATS.md` drafts are never installed; that file is deleted or reduced to a pointer
   that names `CLAIM_SCORE_BEARING.md` and `kbase score` and carries no retired definition's name, so
   the acceptance grep below can come back empty.
2. `definitions/kbase.py`: the description becomes "score the KB's rigor and bearings"; the route
   table's grading row becomes `scoring the KB's rigor and bearings → references/scoring.md`; the
   route names the installed `kbase` binary. `score` is both a CLI verb and a builtin tool: a tool
   call starts the run detached and returns its pid and state store, and `wait` reports the outcome
   when it ends.
3. `references/scoring.md` carries: what `kbase score --pass rigor|bearing` does; one quantity per
   ask and the two passes; the configured provider and text model (`show-config` reports what
   answers); the two human-scored exceptions (a work's rigor, a `founds` bearing); the ledger, the
   run id and `--resume`; that values age and a fresh run re-asks; where the ledger lives and how to
   read a run's outcome.
4. `definitions/kb_maintainer.py`: drop the scorer from its description and its "you do NOT score"
   paragraph; say a created entry's rigor is `*pending*` until a run or a person supplies it; keep
   every write instruction and the refresh→verify loop.
5. Nothing in the set instructs a model to iterate over the scoring set.

**Review.** These files are model-facing. Prompt-engineer review is required before the change lands
and before the skill is installed anywhere.

**Acceptance.** A `kbase` install writes no `kb-claim-scorer` definition into any harness;
`rg -n 'kb-claim-scorer'` over the installed set returns nothing; no file in the set instructs a
model to iterate over the scoring set (`rg -n 'scoring set|for each item|iterate'` over the
installed definitions finds no instruction); a scoring request routes to `references/scoring.md` and
names `kbase score`, the two passes, the model, the ledger and the two exceptions; `kb-maintainer`
names no scorer and keeps its write surface. Evidence: the install transcript and the greps.

## What this phase does not do

- It adds no compatibility reader to kbase: `internal/migrate` reads `1.0.0` to convert it, and no
  command reads a KB through it.
- It adds no `score` MCP tool and no scoring agent.
- It does not change the format again; `3.0.0` is a separate roadmap item (jointly established
  units).
