# writeapi-regression — provenance

The frozen failure corpus for the 2026-09-02 regression replay, held as KB format 3.0.0 records:
`kb-root/.index/nodes.yaml` (23 claim records) and `kb-root/.index/edges.yaml` (their 21 premise
edges, each a `supports` record at bearing `1.0`). These files are **inputs to tests only**. They
are never re-derived and never shipped: they live under `internal/write/testdata`, which no build
embeds and no install carries. `TestRegressionReplayOps` reads them through `kb.LoadAuthoredGraph`
and writes the same population through `insert-claim-entry`, requiring the graph back isomorphic
under the renaming.

## Freeze source

| Fact | Value |
|---|---|
| Repository | a transient build fixture repository, read-only here |
| Commit | `edf672ac848a5447241fa6f93326b344860f0450` |
| Subject | `kb-build: phase-3 \| distillation` |
| Date | 2026-09-02 18:16:00 -0700 |

That repository is no longer reachable. Five `claim-quality.md` registers were taken from that
commit's git objects, flattened to `<domain>-claim-rigor.md`, with one redaction (`part4`'s first
premise bullet named a LaTeX file no longer distributable) and the `1.0.0 → 2.0.0` restamp. Those
five files, in the last form they took, are
`git show 48234f9:internal/write/testdata/writeapi-regression/<file>`.

## The conversion to records

The five registers carried the defect the corpus was frozen for: every entry's `<!-- id: … -->`
marker authored above its `##` heading, so each register lost its first entry and shifted every later
title by one. The records hold the corpus as its author meant it — the registers read through the
2.0.x readers (`kb.ParseClaimEntries`) after every marker was put back below its own heading, the
repair the replay used to make in memory: per claim, its id, title, rigor, rationale and strengthen-by
items, and per premise bullet a `supports` record carrying the bullet's bearing and context. The
records were written by `kb.EncodeNodes` and `kb.EncodeEdges`, in their order. Records bind no id
to a heading, so the corpus no longer carries the defect, and the census of it retired with the
registers.

| File | Bytes | sha256 |
|---|---|---|
| `kb-root/.index/nodes.yaml` | 15470 | `9b02083eec857ed6c51e4174604fac009eb55c239c78387276a47b182ac36c38` |
| `kb-root/.index/edges.yaml` | 2583 | `41a980506dd5fb4e26d99d67926a4118517a6bc0431ca574cb45aad8bf81134a` |

## What is not here, and why

- **The phase-2.5 mint record.** It belongs to the freeze, but it was never in commit `edf672a`
  (`.claude-temp/` is gitignored in the fixture repository), so no copy survives. The values the
  replay needs are all in the records above.
- **Any analysis, experiment, work or axiom.** The registers declared claims only, and every
  premise names a claim the corpus declares.
