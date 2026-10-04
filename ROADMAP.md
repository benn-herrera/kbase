# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); the plan under way is `ACTIVE_PLAN.md`. Finished
items are deleted, not archived.

## Now

- Nothing under way; `ACTIVE_PLAN.md` holds the open items the port left.

## Reader fidelity

- Display names from `\input`-ed preambles.
- Resolved numbering: theorem, equation and section numbers as the author's counter
  scheme produces them, where within-section numbering and `\numberwithin` make
  pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled
  (`-f latex-latex_macros`), which may preserve author environment names.

## After kbase is proven in use

- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The real claim-graph sheet algorithm, replacing the body of `internal/sheet`.
- Breaking changes to the KB's metadata formats (SPEC §3).

Four items are also reported upstream to kb_tools as findings: resolved numbering,
display names from `\input`-ed preambles, the `-latex_macros` reading, and
records-native leaves.
