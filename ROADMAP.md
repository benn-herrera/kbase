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
- Add a 'bearing' field for internal edges as exist for intervolume and external work edges
  - The same principle applies - a derivation may be tight and rigorous, but not be dead on point
  - Separating local rigor from applicability has 2 significant benefits
    - More accurate representation of state of the argument
    - Allows for inference or human judgment asks during claim-scoring to be cleanly split on
      easier-to-focus-on local conditions. For smaller models in particular 2 clean asks is just
      going to get better results than one mixed ask.

## After kbase is proven in use
- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The next major bump of the metadata format (SPEC §10): jointly established units — a strongly
  connected component a person promotes into one node carrying the solidity its members share, the
  DAG constraint over units — once the `demoted` relation has been lived with.
- A complete LaTeX reader in place of pandoc's: the `polytexnical` project (`../polytexnical`),
  taken up only after kbase has shown the pipeline produces a claim graph worth auditing.
