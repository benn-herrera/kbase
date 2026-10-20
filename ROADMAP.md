# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); the plan under way is `ACTIVE_PLAN.md`. Finished items are
deleted, not archived.

## Now

- The uncertainty pool and hit list, parked: the declared categories yield nothing useful and the
  hit list returns when a scoring pass has run on the fixture and a ranking signal puts the
  author's premises near the top (`ROADMAP_PLANS/UNCERTAINTY_POOL_AND_HIT_LIST.md`; the closed
  plan's record is `ACTIVE_PLAN.md` at commit `34b956e`). Never an all-pairs check.

## Reader fidelity

- Display names from `\input`-ed preambles.
- Resolved numbering: theorem, equation and section numbers as the author's counter scheme produces
  them, where within-section numbering and `\numberwithin` make pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled (`-f
  latex-latex_macros`), which may preserve author environment names.

## KB format changes
The `2.0.0` model is in force: five node types — `claim`, `analysis`, `experiment`, `work` and
`axiom`; four scoring relations — `supports`, `founds`, `lifts` and `refutes` — beside the two
unscored ones, `references` and `demoted`; and three quantities — `rigor`, a node's local value,
`bearing`, a scoring edge's scalar, and `strength`, the derived value.

- `3.0.0`: the derived index as **one file** instead of `.index/`'s five tables, each record
  carrying its table's name. A new table becomes a record type rather than a new file, so a feature
  does not grow the file set and needs no migration for it; the save commits one rename instead of
  five, and the tables no longer have a partial state between them. The line-per-record shape makes
  the merge mechanical; the cost is that every refresh rewrites the file and one malformed line
  takes out every consumer at once.

## Deferred from `2.0.0`
Two questions the model answers no to. Each names the move that would settle it.

- **May a work refute a claim directly?** `2.0.0` says no: a work carries no authored role list, so a
  work's counter-evidence reaches a claim as an analysis the work founds, and that analysis refutes
  the claim. The move is a `refutes:` list on the work's register entry; the arithmetic is already
  defined — a cap at `1 − bearing × the work's rigor` — so the change is the list and its parse, not
  the walk.
- **Does the sheet want a bearing axis?** `2.0.0` draws the derived value alone: a bearing is an edge
  judgement, and a second visual axis over it is a ruling of its own. The move is a stroke attribute
  per bearing drawn from the index column every scoring row already carries, with the DOT goldens
  regenerated.

## After kbase is proven in use
- The README's argument-graph images are snapshots of another project's graph. They are replaced by
  `render-claim-graph` output from kbase's own KB once a KB argues the scoring mechanism, so the
  pictures become kbase's own graph rather than an illustration of its capability.
- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The next major bump of the metadata format, `3.0.0` (SPEC §10): jointly established units — a
  strongly connected component a person promotes into one node carrying the strength its members
  share, the DAG constraint over units — once the `demoted` relation has been lived with.
- A complete LaTeX reader in place of pandoc's: the `polytexnical` project (`../polytexnical`),
  taken up only after kbase has shown the pipeline produces a claim graph worth auditing.
