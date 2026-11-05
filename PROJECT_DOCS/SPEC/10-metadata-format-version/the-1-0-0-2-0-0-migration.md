### The `1.0.0 → 2.0.0` migration

The `1.0.0` form is standard YAML frontmatter (`---` fenced) in each document. A leaf's per-node
key groups (`exp-id`, `status`, `strengthens`; `sup-id`, `supports`) are one mapping each in a
top-level `experiment-nodes:` or `support-nodes:` list, a one-node leaf a one-element list; the build
records are YAML (`kb-build-*.yaml`); `.index/*.yaml` are YAML streams, one document per line, each
the record as a JSON object after the `--- ` marker; a writer never splits a record across lines. The
frontmatter is at the top of the file, so the up-link line ([§3](../3-compatibility-contract/index.md)) is the first line after its closing
`---`.

The `2.0.0` form keeps that serialisation and changes the scoring vocabulary: three quantities
named, five node types and one scalar on every scoring edge, with the field names, relation names
and arithmetic those fields feed. Its patch `2.0.1` changes the bottom band's words on a
derived line, and nothing else.

One converter reads a `1.0.0` KB's files as bytes and writes the `2.0.0` form's, which the
`2.0.x → 3.0.0` converter (below) then takes. A file it does not locate it does not touch, and a
file already malformed under `1.0.0` is passed through as bytes for the `2.0.x` load the next step runs to
refuse by name — the converter never makes invalid `1.0.0` bytes valid. Nor does it drop bytes: a
file holding content the converter cannot convert refuses the conversion, `check: conversion`, naming
the file, the line and the content, so the load writes nothing and the KB remains as it was. It

- renames `- confidence:` on a claim entry, `- quality:` on an analysis entry and a work's
  `- strength:` to `- rigor:`, and `- solidity:` and `(solidity …)` to `- strength:` and
  `(strength …)`;
- renames the premise list `- depends-on:` to `- rests-on:`, an analysis's `supports:` pairs to
  `lifts:` and an experiment's `strengthens:` pairs to `lifts:`, each pair's value becoming its
  bearing;
- splits every claim into its statement and the argument for it: an analysis `ana-<body>` — the
  claim's own id body, or the next free body where the KB already holds that id — takes the claim's
  title, its rigor and its rationale, the claim's premise list moves to it whole, and the claim keeps
  its rigor, its derived line and its rework items and is lifted by that analysis alone, one `lifts`
  pair at bearing `1.0` declared on the leaf that hosts it. The analysis's statement marker is on
  the claim's own marker line, after the claim's, in the leaf the claim's statement is read from; a
  claim its leaf lists and does not mark takes one on the leaf's first heading. An argument stating
  no premise gives its analysis no premise list, which strength propagation reads at the analysis's
  own rigor. A claim no leaf contains is not split: it keeps its premise list and its rationale,
  since the analysis would hold them on behalf of no host — the `2.0.x` load reads no node for it;
- gives every scoring edge its bearing: an old `(applicability X)` becomes `(bearing X)` with its
  value kept, an untokened premise bullet takes `(bearing 1.0)` — work and axiom bullets included
  — and every experiment's mapping gains `rigor: 1.0`;
- rewrites each entry's `### Quality` to `### Rigor` and `### Solidity` to `### Strength`, and
  renames the register file `claim-quality.md` to `claim-rigor.md` with its Tier-2 marker
  `<!-- claim-quality: … -->` to `<!-- claim-rigor: … -->`, and rewrites every Markdown link to it
  to name `claim-rigor.md`, its anchor kept;
- rewrites every authored `work-<citation key>` id to `wrk-<citation key>` where it appears, an
  analysis's `support-nodes:` to `analysis-nodes:`, its `sup-id:` to `ana-id:` and its `sup-` ids to
  `ana-`;
- writes `axioms.md` — one `### Axiom <label>: Title` heading per `### INVARIANT-<label>:` section
  and per `- Axiom N: **…**` bullet of `invariants.md` or a legacy `AGENTS.md`, each axiom bullet's
  body beneath its heading — rewrites every `INVARIANT-<label>` citation in the register to
  `Axiom <label>`, a legacy `Axiom N` citation already being the `2.0.0` form, gives that bullet
  the bearing an axiom premise requires, and moves a legacy bullet's trailing parenthetical into
  `[context]`; labels are preserved and nothing is renumbered;
- splits a bullet naming several scoring targets into one bullet per target; and
- removes both paths the `2.0.0` form does not have, reported under `removed`: `claim-quality.md`,
  and `invariants.md`, whose declarations move into `axioms.md`. Every other file is left byte for
  byte and no index file is converted: the `1.0.0` index files were derived.

**The migration is not value-identical, and no derived-value equality is promised.** Four causes,
and every one of them is a value a reader will see change:

1. **An analysis's lift changes station; an experiment's does not.** Under `1.0.0` an analysis's
   support contribution was maxed into the claim's local value, which then entered the minimum: a
   corroborating analysis could be dragged down by a weak premise. Under `2.0.0` a lift sits above
   the minimum. A supported claim whose lift exceeds its derivation rises to the lift; where the
   derivation is at or above the lift, the value is unchanged. An experiment's pair already sat
   above the minimum, so its station does not change.
2. **`founds` scales.** An old `applicability X` was a gate: any positive value contributed the
   work's whole rigor, and `0` skipped the edge. A `2.0.0` `bearing X` contributes `X × rigor`, so
   every work edge below `1` falls, a `0` drives its claim to zero, and `1` is unchanged.
3. **An untokened or pending work edge stops pending.** A work bullet with no applicability, or an
   explicit pending one, left its claim pending. The untokened bullet now has `1.0` and
   contributes the work's rigor; an explicit `*pending*` token is kept and the edge is excluded.
   In both cases the claim reads a number where it withheld one.
4. **A claim nobody judged reads `0`.** A `1.0.0` claim whose `confidence` is `*pending*` withheld a
   number. The split gives that unjudged value to the claim's analysis, and an analysis whose
   premises are judged reads pending — so the claim's lift from it is excluded, the claim derives
   nothing, and it reads `0.00` and falls in `unsubstantiated`. That is the rule of strength propagation
   and not a defect: an unjudged analysis is no lift, exactly as an unjudged bearing is no
   supporter.

An old applicability was authored as a gate, not a weight, so every non-`1` value needs a human
re-read before it is trusted; the converter keeps the number and does not reinterpret it, and it
invents neither a finding nor a context where a migrated work bullet's context names none.

