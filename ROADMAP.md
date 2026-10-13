# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); the plan under way is `ACTIVE_PLAN.md`. Finished items are
deleted, not archived.

## Now

- The claim-graph pipeline rework (`ACTIVE_PLAN.md`): discovery; mutual references found with
  adjacency by rule and a proximity-ranked shortlist, links held in the records; the
  co-linearity cull over links; direction by the three-way ask over every link; the cycle cut
  last and once. The ask-refactor cleanup that preceded it closed 2026-10-12 (its record is
  `ACTIVE_PLAN.md` at commit `c1bae14`): the vetting-by-degree stage ran live and culled
  nothing, and the cycle cut inside attribution removed two thirds of what lenient capture
  found, 316 of 471 edges being mutual yeses.

- The instrument before the readings. The comparison tool in `tools/measure` is the only measure of
  the pipeline against the curated KB, and its threshold moves the edge table by itself
  (`ACTIVE_PLAN.md`, the resumed run's reading). Done: the architect review (the document is
  `ROADMAP_PLANS/COMPARE_TO_PRISTINE_REVIEW.md` at commit `06059d2`) and its first two rounds of
  fixes with tests pinning the cascade and every recall class, plus the cull-aware listing
  (`tools/measure/culled_matches.py`). Remaining, listed as `ACTIVE_PLAN.md` §4: the sibling
  scripts' reach read from kbase's own records, the review's open notes, and then the per-miss
  diagnostic, one row per curated edge not recovered, naming the path it was lost on (never
  shortlisted, answered no, culled, or read as mention) from the build records. Only then tune
  the shortlist's K, the cull or the classify framing. The sweep
  scripts score rankings against the curated edges only and are blind to the chain links proximity
  recovers. The positional window for the asks is `ROADMAP_PLANS/ASK_CONTEXT_WINDOW.md`.

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

## Cleanup
- Does the charter file even have meaning anymore with the shape the build process has evolved to?
  I don't think it does. Verify and eliminate.

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
- The claim-graph sheets rendered through Graphviz's library in process (cgo over `libgvc` and
  `libcgraph`, the `dot` layout and SVG plugins linked statically, the libraries vendored once per
  build platform), replacing the `dot`-on-PATH requirement; the DOT goldens judge the swap. The
  exec route stays the contract until then.
- Convenient path for MD -> LaTeX via pandoc for KB-canonical workflow
  (don't rewrite an MD -> LaTeX workflow, for pity's sake.)
- Create a workflow for a scratch start from an empty KB instead of LaTeX -> KB first

Four items are also reported upstream to kb_tools as findings: resolved numbering, display names
from `\input`-ed preambles, the `-latex_macros` reading, and records-native leaves.
