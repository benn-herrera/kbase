# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); where plans live is in CONVENTIONS.md, "Plan & Execute Process".
Finished items are deleted, not archived.

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

## Configuration

- **Remove the inert `provider` key from `config.toml` at the next major.** It loads and chooses
  nothing today, and a file-mode run reports it superseded (SPEC §9). It leaves the schema once the
  reader-facing examples, `USER_CONVENTIONS.md` and adjagent's `references/scoring.md` have moved off
  it; until then strict load keeps modelling the key so the file a user has stays loadable and
  `kbase configure` can rewrite it.
- **Remove the inert `readerConcurrency` key from `config.toml`'s `[asks]` at the next major.**
- **Remove the environment inference mode** (`KBASE_API_BASE_URL`, `KBASE_API_KEY_FILE`,
  `KBASE_DECISION`, `KBASE_TEXT`, `KBASE_SCORING`). Nothing outside this repository uses it; the
  tests and integration scripts that do move to a scratch `--config-dir`, leaving file mode the one
  configuration path.
- **A lockstep schedule for the ask queue, beside the continuous one.** N calls launch together and
  the next N wait for the slowest, so a local endpoint prefills a whole batch at once instead of
  admitting calls one by one. Worth it only if measured faster on local inference; a group's first
  ask still goes alone to warm the prefix cache.

## Distribution

- A `winget` distribution for Windows, which today has no distributed binary and builds from source.

## Tooling

- Revise every justfile `[doc]` string for brevity.
- Review every script under `tools/scripts/` for concision, removing guards and checks against
  states that cannot arise.
- **Progress for `build --replay` and `score --replay`.** Both hold every row in memory until the
  walk drains and write nothing before, so a long replay shows no count of items done or left.

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
