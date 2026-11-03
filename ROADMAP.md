# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC](PROJECT_DOCS/SPEC/entry-point.md) and
[ARCHITECTURE](PROJECT_DOCS/ARCHITECTURE/entry-point.md); where plans live is in
[CONVENTIONS, "Plan & Execute Process"](PROJECT_DOCS/CONVENTIONS/plan-execute-process/index.md).
Finished items are deleted, not archived.

## Maintenance

- update to go go 1.27.X (edit in `go.mod`)
- update dependencies (`just update-dependencies`)
- review of `PROJECT_DOCS/CONVENTIONS/house-rules/`
  - some have rotted
  - some were never fresh in the first place 
  - `a-result-field-s-integer-is-int-never-int64.md`: a workaround for `result.Write`'s closed type
    switch (`internal/result/result.go`); giving the encoder the integer cases retires it.
  - The leaves holding the 18 bold list titles the dicer's refactor skipped as over 8 words: each
    may bundle several rules under one heading, as the file-formats leaf does.
- survey result keys across every command so one concept has one key everywhere, and rename the
  stragglers
- `update-format` converts every bare node id in prose to the naming form on every run, not only on
  upconversion; `verify`'s `bare-id` remedy becomes `update-format`. `add-citation`, the remedy
  today, refuses an id that occurs twice on one line.
- `kbase://help/write` prints the batch contract twice: the `write` tool description needs it, and
  the MCP note is the description's only text slot, which the help resource also renders.
- `remove-node` leaves dangling a plain link into the removed node's summary entry or axiom
  heading, and a bare id of it in leaf text; `verify` reports both only afterwards.
- `build`, `probe` and `score` report provider faults on stderr only, never in their result.
- `score`'s help says it "prices" faults without saying a reply is a fault list kbase converts
  to a value through the pass's table.

## Reader fidelity

- Display names from `\input`-ed preambles.
- Resolved numbering: theorem, equation and section numbers as the author's counter scheme produces
  them, where within-section numbering and `\numberwithin` make pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled (`-f
  latex-latex_macros`), which may preserve author environment names.

## Exporter

- see [EXPORT_LATEX_BUG](ROADMAP_PLANS/EXPORT_LATEX_BUG.md)

## Configuration

- A test for whether lockstep schedule for the ask queue, beside the continuous one would be more efficient
  - N calls launch together and the next N wait for the slowest, so a local endpoint prefills a whole
    batch at once instead of admitting calls one by one. 
  - Worth it only if measured faster on local inference; a group's first ask still goes alone to warm
    the prefix cache.

## Distribution

- A `winget` distribution for Windows, which today has no distributed binary and builds from source.

## After kbase is proven in use
- The README's argument-graph images are snapshots of another project's graph. They are replaced by
  `refresh`'s sheets from kbase's own KB once a KB argues the scoring mechanism, so the
  pictures become kbase's own graph rather than an illustration of its capability.
- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The next major bump of the metadata format after `3.0.0` (SPEC §10): jointly established units —
  a strongly connected component a person promotes into one node carrying the strength its members
  share, the DAG constraint over units — once the `demoted` relation has been lived with.
- A complete LaTeX reader in place of pandoc's: the `polytexnical` project (`../polytexnical`),
  taken up only after kbase has shown the pipeline produces a claim graph worth auditing.
