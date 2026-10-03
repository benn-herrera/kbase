# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); the plan under way is `ACTIVE_PLAN.md`. Finished
items are deleted, not archived.

## Now

- The plan under way: `ACTIVE_PLAN.md`.

## After the first production docgraph lands

- Display names from `\input`-ed preambles.
- Typeset-number fidelity: theorem numbers that match the typeset document, where
  within-section numbering and `\numberwithin` make pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled
  (`-f latex-latex_macros`), which may preserve author environment names.

## After kbase is proven

- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The real claim-graph sheet algorithm, replacing the body of `internal/sheet`.
- kb_tools' opt-in document audit (its `phase-5` stage).
- Breaking changes to the KB's metadata formats (SPEC §3).

Four items are also reported upstream to kb_tools as findings: typeset-number fidelity,
display names from `\input`-ed preambles, the `-latex_macros` reading, and
records-native leaves.
