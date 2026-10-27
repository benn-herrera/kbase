# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); the plan under way is `ACTIVE_PLAN.md`. Finished items are
deleted, not archived.

## Next
- **CONVENTIONS describes a command flag annotation `kbase.required`; no code sets it.**
- **The MCP and results fixtures carry no edges**, so the binary-level runs of `edge-census` and
  `edges` answer empty; grouping and ordering are pinned by the `internal/query` unit tests alone.

## Reader fidelity

- Display names from `\input`-ed preambles.
- Resolved numbering: theorem, equation and section numbers as the author's counter scheme produces
  them, where within-section numbering and `\numberwithin` make pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled (`-f
  latex-latex_macros`), which may preserve author environment names.

## KB format changes
The `3.0.0` format is in force: the nodes and edges are records in two authored files and
everything else is derived at load, over the `2.0` model — five node types — `claim`, `analysis`,
`experiment`, `work` and `axiom`; four scoring relations — `supports`, `founds`, `lifts` and
`refutes` — beside the two unscored ones, `references` and `demoted`; and three quantities —
`rigor`, a node's local value, `bearing`, a scoring edge's scalar, and `strength`, the derived value.
The next major is under "After kbase is proven in use".

## Configuration

- **Remove the inert `provider` key from `config.toml` at the next major.** It loads and chooses
  nothing today, and a file-mode run reports it superseded (SPEC §9). It leaves the schema once the
  reader-facing examples, `USER_CONVENTIONS.md` and adjagent's `references/scoring.md` have moved off
  it; until then strict load keeps modelling the key so the file a user has stays loadable and
  `kbase configure` can rewrite it.
- **Remove the inert `readerConcurrency` key from `config.toml`'s `[asks]` at the next major.**

## After kbase is proven in use
- The README's argument-graph images are snapshots of another project's graph. They are replaced by
  `render-claim-graph` output from kbase's own KB once a KB argues the scoring mechanism, so the
  pictures become kbase's own graph rather than an illustration of its capability.
- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The next major bump of the metadata format after `3.0.0` (SPEC §10): jointly established units —
  a strongly connected component a person promotes into one node carrying the strength its members
  share, the DAG constraint over units — once the `demoted` relation has been lived with.
- A complete LaTeX reader in place of pandoc's: the `polytexnical` project (`../polytexnical`),
  taken up only after kbase has shown the pipeline produces a claim graph worth auditing.
