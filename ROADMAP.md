# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); the plan under way is `ACTIVE_PLAN.md`. Finished items are
deleted, not archived.

## Now

- The node-matching gap (`ACTIVE_PLAN.md`): the author's result numbers recovered in the
  measurement, the matcher made exact for named results, recall against the author's graph read
  for the first time, then the next inference change chosen from that reading.

## Reader fidelity

- Display names from `\input`-ed preambles.
- Resolved numbering: theorem, equation and section numbers as the author's counter scheme produces
  them, where within-section numbering and `\numberwithin` make pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled (`-f
  latex-latex_macros`), which may preserve author environment names.

## After kbase is proven in use

- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The next major bump of the metadata format (SPEC §10): jointly established units — a strongly
  connected component a person promotes into one node carrying the solidity its members share, the
  DAG constraint over units — once the `demoted` relation has been lived with.
- A complete LaTeX reader in place of pandoc's: the `polytexnical` project (`../polytexnical`),
  taken up only after kbase has shown the pipeline produces a claim graph worth auditing.

Four items are also reported upstream to kb_tools as findings: resolved numbering, display names
from `\input`-ed preambles, the `-latex_macros` reading, and records-native leaves.
