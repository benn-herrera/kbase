# The claim-graph ask rework: links found, co-linear claims culled, direction asked, rings cut last

**Status:** the rework landed; SPEC.md and ARCHITECTURE.md describe the landed system and govern on
any disagreement with this plan. WP13 is read: the resumed run in §3, the fresh full build in §5.
The instrument's review is closed (§4). Open: the per-miss diagnostic (end of §4), the one item
that would move the pipeline's settings. Moved here 2026-10-14, when the KB→LaTeX export plan
became the active plan. The full design text — grounding, invariants, the four
stations, contract deltas, WP1–WP12 and the filled WP12 gate — is this file at commit `06059d2`
(as `ACTIVE_PLAN.md`).

Landed: WP1–WP3 (`3259f70`, `d74e2af`), WP6 (`003f6c8`), WP7a (`a40c14b`), WP4+WP5 (`002896c`),
WP7b (`927266b`), WP8 (`83c3214`), WP9 (`c6e9a95`), WP10, WP12 (R1–R16 all as stated). WP11's
hand-off to kb_tools was ingested into adjagent's next sprint plan (`../adjagent/ACTIVE_PLAN.md`,
2026-10-13); the hand-off file itself is retired.

Owners: MS main session.

Runs are named by their launch timestamp (`20261012T182245`, …). Their trees, state and
measurement output were scratch and are not retained; the readings below are the record.

---

## 1. Baselines the reading compares against

- **G-33, the pre-rework run** (run `20261012T182245`, its tree recovered from the fixture
  repository's reflog): 276 claim nodes; live depends / demoted /
  references 315 / 471 / 26. Its reading under the old instrument, recorded in this file at commit
  `7aa4029` ("WP9 — the reading"): 44 of 51 reference claims matched at 0.30; 11 depends + 3 path +
  1 reversed of the reference's 89 edges. Re-measured under the current instrument in §3.
- **Report field shapes.** Every `*-asks` fact is `askTotals`' record (`items`, `answered`,
  `re-asked`, `defaulted`, `calls`, …). `stage-U-shortlist` carries `sources` and `planned`;
  `stage-cull-asks-1`, `stage-cull-asks-2` and `stage-cull` (`candidates`, `one-step`,
  `confirm-asked`, `culled`, `by-root`, `links`); `stage-D-candidates` (`candidates`,
  `by-harvest`); `stage-D-asks`; `stage-D-classify` (`dependency-edges`, `references`);
  `stage-D-ring` (`demoted`).
- **Ask-count bounds** (invariant I6 of the design): `stage-U-asks.items` ≤ 5 ×
  `stage-U-shortlist.sources`; `stage-cull-asks-1.items` = `stage-cull.candidates`;
  `stage-cull-asks-2.items` = `stage-cull.confirm-asked`; `stage-D-asks.items` ≤
  `stage-D-candidates.candidates`.

---

## 2. WP13 — the live run and its reading (MS)

**The build:** a fresh full build (`just prep-test-integration-fixture` then
`just test-integration-build-live-fixture …`), or the resume recipe over the owner-reset fixture.
Then `kbase refresh`, `kbase verify` and kb_tools' verify through the fixture's runner, all green.
`just test-integration-build-live-arxiv` runs the retargeted `kbtools-shortlist` check; its
evidence is under `test_data/transient/test-integration-build-live-arxiv/`.

**Report fields read** (`state/reports/<stage>.yaml` under the run's directory):

| Station | Fields |
|---|---|
| 1, `references-found` | `stage-U-shortlist.sources`, `stage-U-shortlist.planned`, `stage-U-asks.items` |
| 2, `claims-culled` | `stage-cull-asks-1.items`, `stage-cull-asks-2.items`, `stage-cull.candidates`, `stage-cull.one-step`, `stage-cull.confirm-asked`, `stage-cull.culled`, `stage-cull.by-root`, `stage-cull.links` |
| 3, `depends-attributed` | `stage-D-candidates.candidates`, `stage-D-candidates.by-harvest`, `stage-D-asks.items`, `stage-D-classify.dependency-edges`, `stage-D-classify.references` |
| 4, `depends-attributed` | `stage-D-ring.demoted` |

**Measurement:** the comparison instrument at `06059d2` or later (its open items are §4).
Preserve the built tree first (`<run>/built/kb-root` and the
`kb-build-*.yaml` records), then
`just measure-kb-roots tools/measure/compare_to_pristine.py --ours <run>/built/kb-root --reference
test_data/transient/<private-fixture>/kb-root --threshold 0.30`, then `0.40`, each with
`--sources <directory of the .tex>` (§4 item 10; until it lands, `--volume-root <root .tex>`
repeated once per volume in the order the built tree's `entry-point.md` lists them). Without either
the number class does not run. A KB records nothing of its LaTeX origin, so the instrument finds the
roots in the source directory itself. Read the top-1 column beside the fan, and `demoted` as its
own class. The sibling scripts measure kb_tools' plan, not kbase's; read none of
their numbers until §4's item 1 lands.

**Signatures, each stated so a reading can contradict it:**

| Signature | Reading that would falsify it |
|---|---|
| Demoted rows fall from 471 (G-33) to genuine rings. No demoted two-cycle contains an inferred link's edge | Any output from, in `test_data/transient/<private-fixture>`: `comm -12 <(grep '"relation": "demoted"' kb-root/.index/depends-on.yaml \| sed -E 's/.*"source": "([^"]+)", "target": "([^"]+)".*/\1 \2/' \| awk '{print ($1<$2)?$1" "$2:$2" "$1}' \| sort \| uniq -d) <(awk '/^  - a:/{a=$3} /^    b:/{print a" "$2}' kb-build-cull.yaml \| sort)` |
| Ask counts linear | Any bound in §1, read from the named fields, exceeded |
| Live depends edges (`stage-D-classify.dependency-edges`) move from 315 toward the reference's 89 edges | Live depends ≥ 315 with `stage-D-ring.demoted` small: direction asks are converting links to depends wholesale. Read against the classify framing change landed in WP5 (`002896c`) first |
| Culls fall on prose claims the curated reference lacks | Reference claims matched at 0.30 fall below G-33's while `stage-cull.culled` > 0, or the cull-aware listing (`tools/measure/culled_matches.py`) names a curated claim's exact match among the culls |
| Reference edges recalled hold at or above G-33's in every view (fan and top-1, 0.30 and 0.40) | Fewer in any view, with reference claims matched unchanged |

**What the fresh run adds:** the node pass re-run under the paragraph ask's new rendering
(`4ea6960`, `7aa4029`) with an empty answer cache. Read claims minted against 276 and sources at
`references-found` against 247 first; that is the only comparison the rendering change speaks to
directly. Then the three stations against §3's values. Do not tune the shortlist's K, the cull or
the classify framing from this run alone (ROADMAP).

---

## 3. The resumed run's reading (2026-10-13; resume over the fixture reset to `18c48e3`)

Run `20261013T000916`, resuming over run `20261012T182245`'s state and reports. Every recipe
check passed: kb_tools' verify green before refresh,
kbase refresh and verify green. The node set is the previous run's (built by the old paragraph
ask): 276 claims before the cull. The answer cache served 2 of 4511 captures.

**Report fields:**

| Station | Field | Value |
|---|---|---|
| 1 | `stage-U-shortlist.sources` / `.planned` | 247 / 1235 |
| 1 | `stage-U-asks.items` / `stage-U-found.found` | 1235 / 726 |
| 2 | `stage-cull-asks-1.items` = `stage-cull.candidates` | 720 |
| 2 | `stage-cull.one-step` / `.confirm-asked` = `stage-cull-asks-2.items` | 90 / 10 |
| 2 | `stage-cull.culled` / `.links` / largest `by-root` | 53 / 541 / 4 |
| 3 | `stage-D-candidates.candidates` (by harvest) | 595 (reference 61, unmarked 317, adjacent 92, culled 125) |
| 3 | `stage-D-asks.items` | 595 (answered 595, defaulted 0) |
| 3 | classified by relation | supported-by 245, in-support-of 94, mention 256 |
| 3 | `stage-D-classify.dependency-edges` / `.references` | 337 / 256 |
| 4 | `stage-D-ring.demoted` | 5 |

Wall time: references-found 62 min, claims-culled 19 min, depends-attributed 19 min (100 min for
stations 1–4; the previous run's same stations took 63 min, its node pass 35 min). Claims after the
cull: 223 (168 prose, 26 block, 29 equation).

**Ask-count bounds:** all hold (1235 ≤ 5 × 247; 720 = 720; 10 = 10; 595 ≤ 595).

**Signatures, as first read with the old instrument** (at 0.30 and 0.40):

| Signature | Reading |
|---|---|
| Demoted rows fall from 471 to genuine rings; no demoted two-cycle holds an inferred link | **Holds.** 5 demoted (from 471). The two-cycle command returns nothing |
| Ask counts linear | **Holds.** 2560 asks over three stations, each within its bound |
| Live depends move from 315 toward the reference's 89 | **Falsified as stated.** 337 depends with 5 demoted: the direction ask turned 57% of 595 links into depends (339 edges) and 43% into mention. Read against the classify framing change first: it discounts a bare pointer for every answer, and every link reaches the ask with the source's own body as its passage. The fresh run (new node set) separates this from the node pass |
| Culls fall on prose claims the reference lacks | **Holds** as first read (46 of 51 matched at 0.30, from 44, with 53 culled); corrected below |
| Reference edges recalled at 0.30 hold at or above 11 + 3 + 1 | **Holds** (20 depends + 7 path + 2 reversed, +5 shared); magnitude corrected below |

**Instrument review (2026-10-13; the document is
`ROADMAP_PLANS/COMPARE_TO_PRISTINE_REVIEW.md` at commit `06059d2`, its findings F1–F16), which
corrects the readings above.** The comparison tool's summary claimed more than its code did: no
label class existed (F1); `demoted` rows were never read, so every cut edge read as "nothing" (F2),
which made the previous run's "nothing 47" incomparable since its 471 cuts all read so; a wrong
end match reads as "nothing", not "endpoint unmatched" (F3). Robust to the threshold and to
single-best-end matching: 9 depends + 2 path + 2 reversed. Artefacts: the 5 shared nodes (none at
0.40); "same node set" is wrong (276 before the cull, 223 after). The cull signature is
contradicted: reference claim `clm-1xvw8e` matched `clm-jhcgxm` at 0.86 by statement, that node
was culled into `clm-fi5ncu`, and the reference claim then matched five unrelated nodes.

**Re-measured (2026-10-13 03:31) with the instrument after the review's rounds one and two
(`06059d2`): `demoted` read as its own class, edge classes before `shared node`, cosine
withholding the exact classes' nodes, a top-1 view beside the fan.** The G-33 tree was recovered
from the fixture repository's reflog and measured beside this run's; two earlier trees
(`20261011T164029`, `20261012T105257`) likewise. Each tree was recovered at its
`depends-attributed` trail commit; the reflog is the only hold on those commits, and the trees
predate the recipe's refresh and verify.

| Recall: depends + path + reversed, of 89 | 0.30 fan | 0.30 top-1 | 0.40 fan | 0.40 top-1 |
|---|---|---|---|---|
| G-33 (`20261012T182245`) | 11 + 3 + 1 | 6 + 2 + 1 | 6 + 1 + 1 | 4 + 0 + 1 |
| this run (`20261013T000916`) | 21 + 7 + 2 | 15 + 5 + 2 | 16 + 5 + 2 | 9 + 3 + 2 |

- The recall gain holds in every view (15 → 30, 9 → 22, 8 → 23, 5 → 14). The "robust core" above,
  9 depends, is the 0.40 top-1 column.
- G-33's "nothing 47" held 15 cut edges. With the cut class visible, `nothing` is 33 → 33 (0.30 fan)
  and 43 → 43 (top-1): the misses did not move; the recovered edges came out of `demoted`.
- Reference claims matched: 43 → 45 at 0.30, 40 → 39 at 0.40. The cull-aware listing over G-33's
  matches and this run's cull record names one exact match the cull removed (`clm-1xvw8e`'s
  `clm-jhcgxm`, into `clm-fi5ncu`): the cull signature reads as one curated claim lost, not "holds".
- The 2026-10-11 tree (223 depends, no cycle breaking) reads 14 + 1 + 2 at 0.30 fan; its old
  comparison-column figure 15 + 4 + 1 was the old instrument's.

**At the instrument after §4's round (the commit landing §4's closure).** Only the 0.30 fan view
moves: the three `shared node` rows (`1xvw8e→g83uc5`, `1xvw8e→pcfkxg`, `yl5n5v→f58f7y`) read
`depends`, `depends reversed`, `depends`, so 0.30 fan is 23 + 7 + 3 against G-33's 12 + 3 + 1;
top-1 and 0.40 are unchanged; `shared node` is 0 everywhere. Distinct built edges at 0.30 fan: 37
over 33 recovering rows. The reach of the 32 unmarked misses read from the build's own records: 29
never planned by the shortlist, 3 planned and answered no, 0 answered yes. Read over the fan of both
ends, so a wrong match inflates "never planned". With the seven volume roots the number class matches
1 of the 13 numbered reference claims; the other 12 carry a number the walked source does not (the
reference numbers against another version of the sources), and no number/statement disagreement
is reported.

**What the numbers say.** The cycle cut is now a tail of five genuine rings instead of two thirds
of the graph: that stands on the index, not on the instrument. Recall of the curated edges
improved in every view of the instrument. Precision, 332 live depends against 89 curated, is
unjudged: the instrument measures none, and the per-miss diagnostic (ROADMAP) is what would.
The cull was conservative (12.5% one-step, pass 2 asked 10 times, largest root four claims) and
removed one curated claim's match.

**Next run:** the fresh build over the restaged fixture, with the new paragraph rendering and no
cached answers, is read in §5.

---

## 4. The comparison instrument — closed

The review of `tools/measure/` (`ROADMAP_PLANS/COMPARE_TO_PRISTINE_REVIEW.md` at `06059d2`) is
closed: findings F1–F8, F12 and F13 landed at `06059d2`; the nine items below landed, each with its
test, in the commit carrying this paragraph (80 tests under `just test-measure`), verified by the
architect against this list and re-run over the five preserved trees at 0.30 and 0.40 (the
summary-builder extraction was byte-identical before any behaviour change). Readings at the closed instrument are in §3 and §5.
The items are kept as the record of what the instrument now does.

1. **The sibling scripts measure kb_tools' plan, not kbase's (F11).** `measure_unmarked_shortlist.py`
   and `measure_pool_yield.py` rank with kb_tools' `shortlist.rank` and so read a kbase-planned pair
   outside kb_tools' top-K as `not asked`. The reach of unmarked misses must come from kbase's own
   records: `kb-build-unmarked.yaml` `planned` and its letters, and the cull record's `links`.
   Test: an `UnmarkedRecord` with `planned = [(s, t)]` and letters, plus an `edges.tsv` row whose
   matches are (s, t), is reached from the record independent of `shortlist.rank`. Until this lands
   none of the siblings' numbers is read (§2).
2. **`shared node` when one end's matches contain the other's.** The edge classes test A∖B × B∖A,
   so with B ⊆ A an edge from A∖B into the shared node is unread and the row reads `shared node`.
   Fix: test (A∖B × B) ∪ (A × B∖A), excluding only pairs inside A∩B. Test: A = {x, s}, B = {s},
   depends = {(x, s)} reads `depends`; A = B = {s} still reads `shared node`. Three rows of the
   resumed run at 0.30 change class (`1xvw8e→g83uc5`, `1xvw8e→pcfkxg`, `yl5n5v→f58f7y`).
3. **Distinct witnesses per built edge.** The count is per class and per whole witness string, so
   `clm-66wgwi->clm-4mx3tt` as a `depends` witness and as the last edge of a path witness counts
   twice. Fix: decompose path witnesses into edges and count distinct built edges across all
   recovering rows, beside the per-class line. Test: a `depends` row and a path row sharing one
   edge give one distinct edge.
4. **The statement-less reference claims as their own row group (F10).** The 13 reference claims
   no marker reaches are whole-leaf claims only cosine can match, and they carry most of the
   0.30 → 0.40 swing in `endpoint unmatched`. List them as a row group in `summary.md` with best
   cosine and fan. Offering their leaf's opening section to the statement class is a separate
   choice, not taken here.
5. **Two marker readers (F14).** `MARKER_RE` with an exact-string search and
   `kb_index_lib.tier2_markers` read a multi-id marker differently; every reference marker is
   single-id today. Read markers through `tier2_markers` alone, with a test on a two-id marker.
6. **A summary heading over-claims (F15).** "the reference claims the number class left (no
   statement: falls through to cosine)" lists every claim the number class left, matched ones
   included. The heading says what the list holds, or the list holds only the claims that fell to
   cosine.
7. **Number beats statement unconditionally (F16).** A misaligned `--volume-root` order wins over
   a 1.0 statement match and only `unwalked_blocks` warns. Report every reference claim where the
   number class and the statement class name different nodes, with both nodes and the overlap.
8. **The summary still assembles inline (F9 residue).** `main` builds every `summary.md` section,
   the new recall and witness tables included; the cascade and edge sets are extracted and
   tested, the sections are not. Extract section builders as the dispatch finds them in reach;
   no behaviour change, byte-identical summaries over the preserved trees.
9. **Tests the burn-down still lacks.** `read_culls` over a record string in the cull record's
   shape (`culls: [{id: n, into: m}]`) and its refusal path; a top-1 sweep row equal to a top-1
   run at the same threshold.

10. **Root discovery (open).** The instrument takes `--sources <dir>` in place of the per-root
    `--volume-root` list and finds the volume roots itself: a `.tex` under the directory holding
    `\documentclass` that no other file there names in `\input{…}` or `\include{…}` (with or
    without the extension). Each root's volume directory is the slug of its `\title` rendered to
    plain text, by kbase's rule (`internal/docgraph/text.go` `slug`: lower-case, every run of
    characters outside `[a-z0-9]` to one hyphen, trimmed, cut at 48 characters back to the last
    hyphen, `untitled` when empty), matched against the built tree's `entry-point.md` volumes. The
    summary lists the mapping, every root no volume took and every volume no root reached; the
    number class runs over the mapped volumes. The page-block tell still guards a wrong mapping.
    Tests: a child file `\input` by a root is not a root; a multi-line `\title` with `\\` and
    braces slugs as kbase does; a title longer than 48 characters cuts back to the last hyphen; an
    unmatched root and an unmatched volume are both listed. Acceptance: over the fresh tree,
    `--sources` reproduces, to the line, the number section of an explicit seven-root
    `--volume-root` run over the same tree.

**Next, not part of the review:** the per-miss diagnostic (ROADMAP, "The instrument before the
readings"), a new `tools/measure` script with its own tests. Its first half exists: the record-based
reach in `measure_unmarked_shortlist.py` (`reach.tsv`) says whether a miss was ever planned and how
it was answered. The second half joins that to the cull record and the classification record: one row per curated edge not
recovered, naming where it was lost — never shortlisted, answered no at `references-found`,
culled, or read as mention at `depends-attributed` — joined from the four build records and
`claim-matches.tsv`. It is the reading that decides whether the shortlist's K, the cull or the
classify framing moves.

---

## 5. The fresh run's reading (2026-10-13; full build over the restaged fixture)

Run `20261013T015028`, launched 01:50 after `just prep-test-integration-fixture` restaged the
fixture. Every recipe check passed: kb_tools' verify green before refresh, kbase
refresh and verify green. The node pass ran under the paragraph ask's new rendering (`4ea6960`,
`7aa4029`) with an empty answer cache. Measured with the instrument at `06059d2`, at 0.30 and
0.40.

**The node pass against the previous run's** (run `20261012T182245`), the only comparison
the rendering change speaks to directly:

| | previous | fresh |
|---|---|---|
| leaves / paragraphs asked | 162 / 765 | 162 / 765 |
| prose claims minted (leaves minted / no-claim) | 221 (106 / 56) | 223 (110 / 52) |
| claim nodes before the cull | 276 | 278 |
| defaults by cause (`no-letter`, `unplaceable`) | 0, 0 | 0, 0 |
| node-pass wall time | 35 min | 33 min |

Two more paragraphs read as claims and four more leaves mint; nothing defaulted in either run. Every
node id is new, so no pre-cull comparison exists for this tree and the cull-aware listing cannot be
run over it.

**Report fields, beside §3's:**

| Station | Field | §3 (resumed) | fresh |
|---|---|---|---|
| 1 | `stage-U-shortlist.sources` / `.adjacent` / `.planned` | 247 / — / 1235 | 249 / 135 / 1245 |
| 1 | `stage-U-asks.items` / `stage-U-found.found` | 1235 / 726 | 1245 / 730 |
| 2 | `stage-cull.candidates` = `stage-cull-asks-1.items` | 720 | 726 |
| 2 | `stage-cull.one-step` / `.confirm-asked` | 90 / 10 | 97 / 10 |
| 2 | `stage-cull.culled` / `.links` / roots, largest | 53 / 541 / 4 | 58 / 538 / 42 roots, largest 5 |
| 3 | `stage-D-candidates.candidates` (reference, unmarked, adjacent, culled) | 595 (61, 317, 92, 125) | 592 (61, 312, 84, 135) |
| 3 | `stage-D-asks.items` (answered, defaulted) | 595 (595, 0) | 592 (592, 0) |
| 3 | classified: supported-by / in-support-of / mention | 245 / 94 / 256 | 236 / 100 / 256 |
| 3 | `stage-D-classify.dependency-edges` / `.references` | 337 / 256 | 334 / 256 |
| 4 | `stage-D-ring.demoted` | 5 | 10 |
| index | depends / references / demoted rows | 332 / 256 / 5 | 324 / 256 / 10 |

Claims after the cull: 220 (165 prose, 26 block, 29 equation). Wall time: references-found 62 min,
claims-culled 20 min, depends-attributed 19 min.

**Ask-count bounds:** all hold, station 1 exactly at its bound (1245 = 5 × 249; 726 = 726; 10 = 10;
592 ≤ 592).

**Signatures:**

| Signature | Reading |
|---|---|
| Demoted rows fall from 471 to genuine rings; no demoted two-cycle holds an inferred link | **Holds.** 10 demoted, all among eight claims in two rings; the two-cycle command returns nothing |
| Ask counts linear | **Holds.** 2573 asks over three stations |
| Live depends move from 315 toward the reference's 89 | **Falsified as stated, and now independent of the node pass.** 334 depends from 592 links: 57% depends and 43% mention, the resumed run's split to the point. The split is the direction ask's, not the old node set's |
| Culls fall on prose claims the reference lacks | **Holds on the match count** (45 of 51 at 0.30, as §3; 40 at 0.40). The direct test (the cull-aware listing) has no pre-cull matches for this tree |
| Reference edges recalled hold at or above G-33's in every view | **Holds.** See the table |

| Recall: depends + path + reversed, of 89 | 0.30 fan | 0.30 top-1 | 0.40 fan | 0.40 top-1 |
|---|---|---|---|---|
| G-33 (`20261012T182245`) | 11 + 3 + 1 | 6 + 2 + 1 | 6 + 1 + 1 | 4 + 0 + 1 |
| resumed (`20261013T000916`) | 21 + 7 + 2 | 15 + 5 + 2 | 16 + 5 + 2 | 9 + 3 + 2 |
| fresh (`20261013T015028`) | 22 + 6 + 1 | 16 + 6 + 1 | 16 + 4 + 1 | 10 + 4 + 1 |

Misses at 0.30 fan (demoted / references only / shared node / nothing / endpoint unmatched):
0 / 7 / 3 / 30 / 20, against the resumed run's 0 / 4 / 3 / 33 / 19. Path rows of length ≤ 2: 3 of 6
(fan). Distinct witnesses: 21 for 22 `depends` rows at 0.30 fan.

**At the instrument after §4's round.** As for §3: the same three rows leave `shared node`, so 0.30
fan is 24 + 6 + 2; top-1 and 0.40 unchanged. Reach of the 33 unmarked misses from the build's own
records: 26 never planned, 4 planned and answered no, 3 planned, answered yes and the link kept
through the cull; those three read `references only` in `edges.tsv`, so they were lost at the
direction ask. Number class with the seven volume roots: 1 of 13 matched, 12 with a number the
walked source does not carry; no number/statement disagreement.

**What the numbers say.** The rendering change did not move the node set in any way the stations
feel: every downstream figure sits within a few percent of the resumed run's, and recall of the
curated edges is the same in every view to within two edges. The direction ask's 57 / 43
depends-to-mention split reproduces on a fresh node set, so it is the ask's behaviour; whether it is
right is the per-miss diagnostic's question (§4), not this run's. The ring cut doubled to ten edges
and stayed genuine. Station 1 now plans exactly at its bound: every source fills all five slots.
