# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC](PROJECT_DOCS/SPEC/entry-point.md) and
[ARCHITECTURE](PROJECT_DOCS/ARCHITECTURE/entry-point.md); where plans live is in
[CONVENTIONS, "Plan & Execute Process"](PROJECT_DOCS/CONVENTIONS/plan-execute-process/index.md).
Finished items are deleted, not archived.

## Items

1. Claim graph
- Strength that does not compound with chain depth, so the same points arranged in more or fewer
  steps score alike (THESIS: arrangement-invariant): see
  [PROPOSED_REVISED_STRENGTH_SCORING_SCHEME](ROADMAP_DESIGNS/PROPOSED_REVISED_STRENGTH_SCORING_SCHEME.md)
  §5, "The combination".
- A `definition` node. A definition is context today: no node, no edge, no score, and a claim's
  dependence on a definition is unrecorded.
  - Why it matters: a definition compresses a construct into a short handle that claims then use, so
    every claim built on the handle inherits whatever is wrong with the construct. A definition can be
    well or poorly posed, and a poorly posed one undermines an argument's validity however sound its
    steps.
  - A candidate rigor checklist is the standard well-posedness criteria: the defined object exists
    and, where claimed, is unique; it does not depend on arbitrary choices (independence of
    representation); it is not circular; and the term is used afterwards as defined.
  - A candidate shape: a node kind with its own narrow rigor judgment, which claims rest on through an
    edge, so a poorly posed definition caps what is built on it as a weak premise does.
  - Open: the node kind and its relations; finding definitions at all, since the build has no notion
    of one today and a definition is incidental prose; and how it composes with the existing faults
    that already touch definitions indirectly (`flt-equivocation`, `flt-referent`).

2. Reader fidelity
- Display names from `\input`-ed preambles.
- Resolved numbering: theorem, equation and section numbers as the author's counter scheme produces
  them, where within-section numbering and `\numberwithin` make pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled (`-f
  latex-latex_macros`), which may preserve author environment names.
- A LaTeX reader in place of pandoc's (`../polytexnical`), taken up only if it shows coverage pandoc's
  reader lacks and the pre-pass registry can't supply, measured on real papers. pandoc is actively
  maintained and may close those gaps itself.

3. Exporter
- see [EXPORT_LATEX_BUG](ROADMAP_DESIGNS/EXPORT_LATEX_BUG.md)

4. Configuration
- A test for whether lockstep schedule for the ask queue, beside the continuous one would be more efficient
  - N calls launch together and the next N wait for the slowest, so a local endpoint prefills a whole
    wave at once instead of admitting calls one by one. 
  - Worth it only if measured faster on local inference; a group's first ask still goes alone to warm
    the prefix cache.

5. Distribution
- A `winget` distribution for Windows, which today has no distributed binary and builds from source.

6. After kbase is proven in use
- The README's argument-graph images are snapshots of another project's graph. They are replaced by
  `refresh`'s sheets from kbase's own KB once a KB argues the scoring mechanism, so the
  pictures become kbase's own graph rather than an illustration of its capability.
