# Survey: kbase-coined vocabulary for standard concepts

Scope: the six documents under `DOCS_REVIEW/` at commit `0eebf4c`. `reference/` was not read.
Standard-vs-novel classification and SPEC's dead links into the retired design are covered by
other surveys and are left out here.

## Summary

- **Most frequent:** `carries` (~407), `asks` (~288), `holds` (~212), `naming` (~211), `stands` (~202) and `the walk` (~196).
  The counts include every sense of each word. `carries`, `stands`, `holds` and `walk` each have three to six senses.
- **Strongest signal of a bespoke mechanism:** derived-view regeneration goes by three names: "the derivation",
  "refresh" and "re-derivation". On top of that, "derivation" also names D(n) in the strength formula.
  Other strong signals:
  - write-and-apply under the name "landing" / "the one-op landing"
  - existence checks and idempotence written as "what stands" / "re-issue" / "the existence rule"
  - the holder lock and the operation lock
  - "the ring row" / "cut" / "the walk" for DFS cycle breaking
  - "seam", "closed text" + "guard", and "price" / "re-price"
  - "register page" and "summary page" as two names for one generated page
- **Collisions the rewrite has to split by site, not by find-and-replace:**
  - "index" means four different things
  - "surface", "bound", "digest", "token", "frame", "passage", "land", "cut" and "spine" each mean two or more things
- **One accuracy defect:** SPEC's `### Idempotence` (SPEC:701) mostly states operations that are *not* idempotent.

## How counted

Each count is the number of occurrences that `grep -oiE '<pattern>' <file> | wc -l` returns, run in
`DOCS_REVIEW/`. Counts are listed per file in this order:
THESIS / SPEC / ARCHITECTURE / CONVENTIONS / RIGOR_AND_BEARING / STRENGTH, followed by the total.
The counts are approximate. They cover every sense of the pattern, including senses that are not
the coined one. The note column says where that inflates a count.

Citation shorthand: `TH` THESIS.md, `SP` SPEC.md, `AR` ARCHITECTURE.md, `CV` CONVENTIONS.md, `RB`
RIGOR_AND_BEARING_SCORING_SCHEME.md, `ST` STRENGTH_SCORING_SCHEME.md.

## Table

### Persistence, writes and derived data

| kbase term / pattern | standard term | where (representative) | count (pattern; TH/SP/AR/CV/RB/ST = total) | exact or differs |
|---|---|---|---|---|
| "stands" / "what stands" / "where X stands" / "as it stands" | exists; is unchanged; is kept | SP:704-705, SP:178, SP:865, SP:848, AR:29 | `\bstand(s\|ing)?\b\|\bstood\b` 0/110/62/4/10/16 = 202 | **Differs, polysemous.** Senses: exists (SP:704, SP:865); is left unchanged (SP:848, SP:854, SP:925); is positioned ("it stands before `state-dir`", SP:636, SP:1411); persists (SP:541, "an answer already returned stands"); remains valid (SP:1208); a person's value that is not overwritten (SP:1143). Each site needs its own word. |
| "re-issue" / "identical re-issue" / "a re-issued add … is refused" | repeated request (idempotence test); client retry; HTTP retry | SP:708, SP:501, SP:607, SP:1287, AR:29, CV:83 | `re-issu` 0/22/13/2/0/0 = 37 | **Differs.** Senses: the same write repeated, as an idempotence test (SP:708, AR:29 I6); the caller's retry instruction (SP:607, SP:677); transport retry (SP:1287, CV:83). In SP:1261 the batch is "re-issued without" one entry, so that re-issue is not identical. |
| "the existence rule": "an add of what stands refused, a removal of what does not refused, an edit to a value that stands `unchanged`, and nothing adopted" | create fails if it exists (no upsert); delete fails if absent; an update to the same value is a no-op | SP:703-705, SP:2884, AR:29 | `existence rule\|node exists\|edge exists` 0/10/0/0/0/0 = 10 | Exact. However, it sits under `### Idempotence` (SP:701), and add, remove, restore and add-citation are *not* idempotent: they refuse on repeat. Only edit and move are idempotent. |
| "adopt" / "nothing adopted" | upsert, or match to an existing record | SP:705, AR:452, AR:519 | `adopt` 0/6/3/0/0/0 = 9 | Exact. The build-only `split-claim` upserts ("adopts instead the analysis already lifting the claim", AR:519) while the public ops do not. At SP:2095, "adoption" means normalized string comparison. |
| "the derivation" / "the batch's derivation" / "the derivation after every write" / "Writes leave nothing stale" | regenerate the derived views (in-memory index, generated pages, SVGs) inside the write transaction | SP:181, SP:713-717, SP:734, SP:1260, AR:456, AR:473-475, AR:512 | `derivation` 0/43/26/0/9/11 = 89 | **Differs: three senses.** (1) regenerating derived files, as in the SP and AR write path; (2) D(n), the min-term of the strength formula (SP:2231, ST:303, AR:575); (3) a method's displayed argument (RB:24, ST:40). Rewrite sense 1 as "rebuild derived views" so that "derivation" is left to the formula. |
| "refresh" / "re-derive" / "re-derivation" / "start refresh" | same as above: regenerate derived files | SP:619, SP:1381, SP:2420, AR:632, AR:2420 | `refresh` 0/28/26/1/0/0 = 55; `re-deriv` 0/8/7/0/1/0 = 16 | Exact. This is the same mechanism as "the derivation": AR:632 lists `UpdateFormat`, `index.Refresh` and `PlanDerivation` as callers of one seam. `refresh` was also a verb that has been retired (SP:2855). "re-derive" in the score resume (SP:1359) means *recompute values under new weights*, which is a different operation. |
| "derived layer" / "derived rows" / "derived files" / "the index" | derived (materialized) views; in-memory index | AR:558, AR:433, SP:182, SP:2045 | `derived` 0/36/22/6/1/4 = 69; `\bthe index\b` 0/11/10/0/0/0 = 21 | **Collision on "index", four senses:** `.index/` holds the *authored* source-of-truth records (SP:24, SP:2042); "the index" means the *derived* rows (SP:2045, AR:590); `index.md` is a directory's document (SP:61); `internal/index` is the package. |
| "register page" / "summary page" / `kb-summary.md` / "the register" | generated report page (derived view) | SP:69, SP:2144, AR:431, RB:5, ST:482 | `register` 0/52/23/3/4/2 = 84; `summary page` 0/19/7/0/0/0 = 26 | **Two names for one thing**: SP:2144 says "Register pages are … summary pages". In RB:5 and RB:244, "the register" means the stored value. ST:482 "The register is claim-centric" describes the retired authored form. |
| "land" / "landing" / "lands" / "the one-op landing" | apply (write) results in one batch transaction | SP:1246-1264, SP:723, AR:903-944 | `\bland(s\|ed\|ing\|ings)?\b` 0/38/59/14/12/3 = 126 | **Differs, polysemous.** Senses: a score value written through the edit ops (SP:1259); work committed to the repository (CV:185 "the landing commits once", CV:131, AR:8 "decided and landed", and AR §6's "Landed" status column); a value falling in a band (ST:150 "lands `0`", SP:2239). The "one-op landing" (AR:903) is just one write batch. |
| "birth" / "birth-once" / "at birth" / "birth commit" / "scratch creation" / "the zero case of the tree" | create; create-once (fails if it exists); initial commit; `init` | SP:709, SP:749, SP:763, AR:29, AR:437, AR:638, SP:18 | `birth\|creation` 0/12/11/0/1/0 = 24 | Exact. |
| "spine" / "spine seed" / "skeleton" | scaffold (empty initial structure); navigation tree | TH:26, TH:67, SP:619, SP:1081, AR:75, AR:1965, CV:53 | `\bspine\b` 2/5/6/0/0/0 = 13; `skeleton` 0/3/2/2/0/0 = 7 | **Differs, polysemous.** Senses: the argument graph's backbone (TH:26); the mechanical draft of both hierarchies (TH:67); `init`'s empty stamp and record files (SP:619, AR:75); the index child-list tree (SP:1081 "the index child-list spine", AR:1965 "spine edge"). |
| "the mark" (`manually-scored`) / "with the mark `false`" | provenance flag: written by a person or by a machine | SP:807, SP:834, SP:1248 | `\bmark\b` 0/12/2/1/1/0 = 16 | Exact as a flag. It collides with the marker verb: SP:299 "A build marks every claim" means *places statement markers*. |
| "judged hash" / "dirty" / "dirt" | content hash for change detection; stale | SP:1110-1112, SP:2997, AR:783 | `judged.hash` 0/36/12/1/0/0 = 49; `\bdirty\b\|\bdirt\b` 0/24/11/1/0/0 = 36 | Exact. This is input-hash invalidation, the same as dirty tracking in a build system. "dirty" also means git-uncommitted (SP:186, `dirty-paths` SP:2857), which is a second sense. |
| "pre-flight commit(s)" / "makes git the batch's undo" | snapshot commit before a write; one commit per transaction as an undo log | SP:777, SP:2820-2825, AR:462-471 | `pre-?flight` 0/6/15/0/0/0 = 21 | Exact. "preflight" also means precondition checks (SP:48, AR:73, AR:202), which inflates the count. |
| "the disk still holds what was read" / "checked again against what was read" / "read-then-compare check" | optimistic-concurrency check (compare before write) | AR:458, AR:464, AR:432 | `still holds what was read\|read-then-compare\|checked again against what was read` 0/0/2/0/0/0 = 2 | Exact. |
| "save" (of a migration) / "conversion pending" | persist the migration; migrate-on-read plus an explicit migrate command | SP:2000-2015, AR:1883, AR:2117 | `\bsaves?\b\|\bsaved\b` 0/17/22/2/0/0 = 41 | Exact. |
| "maintained block" / "fence lines" / "unfenced" | managed (generated) region between marker comments | SP:85, SP:2866, AR:436, CV:281 | `maintained.block\|unfenced\|fence lines` 0/16/5/4/0/0 = 25 | Exact. |
| "stamp" / "format stamp" / "stamped documents" / "readiness stamp" | format version header; scaffold files written if absent | SP:1963, SP:76-86, AR:83, AR:96, AR:839, CV:159 | `\bstamp(s\|ed\|ing)?\b` 0/38/57/8/0/0 = 103 | **Differs, polysemous.** Senses: the version marker (SP:1963); writing template documents only where they are absent (AR:96); recording a field (AR:839 "stamps `ask_sha256`"); the state store's format integer (AR:1408). |
| "host" / "hosted" / "hosts" | containing document (location of a node's statement) | SP:2129, SP:301, AR:25, AR:551 | `\bhost(s\|ed\|ing)?\b` 1/47/31/2/0/0 = 81 | Exact. It collides with the machine sense: SP:39-41 "Required on the host", AR:1118 "idle connections per host", and TH:73 "hosted" for model serving. |
| "mint" / "minted" | generate a new unique ID | SP:239, AR:542, CV:245 | `\bmint` 0/21/29/2/0/1 = 53 | Exact. |

### Background jobs, locks, resume

| kbase term / pattern | standard term | where (representative) | count (pattern; TH/SP/AR/CV/RB/ST = total) | exact or differs |
|---|---|---|---|---|
| "commit trail" / "the trail" / "build trail" / "stage boundary" / "boundary commit" | checkpoint log (one commit per completed pipeline stage) | SP:177, SP:399, SP:205, AR:69, AR:100, AR:1338 | `\btrails?\b` 0/57/68/2/0/0 = 127; `boundar(y\|ies)` 0/23/20/3/2/0 = 48 | Exact. A second sense: a "trail" directory in the state store is a cache partition for one build (AR:1420). |
| "holder lock" / "holder record" / "holds the state store" / "operation lock" | PID lock file with owner metadata; repository-wide job mutex | SP:427-449, AR:1375-1407, AR:879 | `holder` 0/22/37/4/1/0 = 64 | Exact. AR:1405 itself notes the user-facing text says "holds the state store" while "operation lock" is the internal name. There are three lock names in total: KB write lock, operation lock and holder lock. |
| "one hold of the KB write lock" / "wait out a write" | a single lock acquisition; block until the lock is released | SP:1260, AR:906, SP:727, AR:1391 | `one hold\|the hold` 0/2/1/0/0/0 = 3; `waits? out\|wait-out` 0/1/4/0/0/0 = 5 | Exact. |
| "ledger" | append-only run log (journal) | SP:1293-1342, AR:808, RB:546 | `\bledger` 0/44/51/5/14/0 = 114 | Exact for `ledger.jsonl`. "Ledger" also names the git commit trail (`gitledger`, AR:1323; the "**Ledger.**" bullet, AR:1538), which is a second log under the same word. |
| "the walk" / "walks" / "walk order" | iteration; pipeline; traversal; propagation; DFS | AR:69, SP:1259, ST:95, SP:2552, AR:203, CV:379 | `\bwalk(s\|ed\|ing)?\b` 0/74/81/11/15/15 = 196 | **Differs, six referents.** The build's stage pipeline (AR:69 "walks these stages"); the score run's item loop (SP:1259 "when its walk ends"); strength propagation (ST:95); cycle-detection DFS (SP:2552); the export's document traversal (AR:203); a pandoc Lua AST walk (CV:379). |
| "the ring row" / "ring cut" / "the cycle walk" / "cut" | cycle breaking: remove a back edge found by DFS (feedback-edge removal) | SP:2540-2564, AR:81, AR:478, AR:525 | `\bring\b` 0/1/5/0/0/0 = 6; `\bcuts?\b` 0/30/18/1/10/1 = 60 | Exact mechanically. "cut" is also a band threshold (RB:274 "cuts at `0.85`", RB:291 "exactly at the cut"). |
| "row" of the stage table: "its row spends its own inference", "the dropped row", "calling row" | pipeline step | AR:80, AR:90-95, AR:1170, SP:286 | `dropped row\|its row\|calling row\|row ids\|inference rows?\|the row\b\|a row marked` 0/9/8/0/1/0 = 18 | Exact. "row" also means a derived index row (AR:433) and a query result row (SP:625). |
| "spends inference" / "paid-for inference" / "spending no inference" | makes billable LLM calls; cached model answers | SP:245, SP:250, SP:261, SP:438, AR:91, AR:1353 | `spend\|spent\|paid` 0/8/7/0/0/0 = 15 | Exact. |
| "dial" / "dials" / "dialled" | connect to; send a request to | SP:630, SP:1424, SP:1604, AR:993, AR:1119 | `\bdial` 0/13/6/0/0/0 = 19 | Exact. |
| "census" | count / summary report | AR:26, AR:124, CV:242 | `census` 0/2/5/1/0/0 = 8 | Exact. The SP hits are the retired `edge-census` command (SP:2782). |
| "re-ask-then-default" / "defaulted" / "fallback" | retry once, then use the deterministic default | SP:327-332, SP:350, SP:1811, AR:1296 | `re-ask` 0/6/9/1/2/0 = 18 | Exact. Other senses: retry on `--resume` (RB:520), and AR:814 "re-asks a write". |
| "closes on" / "the closing record" | the terminal key of the result document | SP:579-581, SP:1398-1408, AR:1075 | `clos(es\|e) on\|closing record` 0/6/2/0/0/0 = 8 | Exact. |

### Model calls

| kbase term / pattern | standard term | where (representative) | count (pattern; TH/SP/AR/CV/RB/ST = total) | exact or differs |
|---|---|---|---|---|
| "ask" / "asks" / "ask batch" / "multichoice ask" / `[asks]` | model call (LLM request); prompt; concurrent request batch | SP:262, SP:324, SP:1278, AR:34, AR:1271, CV:297, RB:192 | `\basks?\b` 0/100/151/12/25/0 = 288 | **Differs, polysemous.** Senses: the request (SP:1278 "The ask is one tool-less chat call"); the prompt template (RB:192 "The ask lists them…", SP:1221 "the criteria are the ask's"); the question (SP:324 "co-linearity ask"); a stage's set of concurrent calls (AR:34 "ask batch"). |
| "seam" / "multichoice-ask seam" / "`Launch` seam" / "pandoc seam" / "logging seam" | interface; injected dependency (port) | AR:107, AR:669, AR:685, AR:1288, AR:1311, CV:690 | `\bseams?\b` 0/2/25/2/0/0 = 29 | Exact. It also leaks into SPEC (SP:1811 "the seam's own re-ask-then-default path"). |
| "token(s)" of the queue / "releases its token" / "N tokens" | semaphore permit (concurrency slot) | AR:34-35, AR:1087, CV:293 | `\btokens?\b` 0/13/16/3/1/3 = 36 (all senses) | **Collision:** LLM tokens (SP:1284 "four characters per token", SP:1774); scoring tokens in text (SP:1166, CV:358). |
| "closed text" / "the guard" | the rendered prompt input for one item; input sanitizer (prompt-injection filter) | SP:1164-1167, AR:717-744, RB:388 | `closed text` 0/2/15/1/5/0 = 23 | Exact. "closed" here means *bounded to the item*, which is unrelated to "closed set" (an enum). The two phrases sit close together. |
| "closed set" / "closed vocabulary" | enumeration (allowed values) | SP:263, SP:605, SP:695, AR:445 | `closed (set\|vocabulary)` 0/4/3/0/2/0 = 9 | Exact. |
| "the prompt shelf" | prompt template collection | AR:697-715 | `shelf` 0/0/3/0/0/0 = 3 | Exact. |
| "frame" / "frame label" / "the frame's gutter" | fixed prompt-template scaffolding around the item | SP:1176, AR:737-742, RB:410 | `\bframes?\b` 0/7/9/0/1/0 = 17 | **Collision:** JSON-RPC frames (SP:559, SP:2729, AR:2264). |
| "passage" | a section of the prompt input (title, statement, source, target) | SP:1175, AR:730, RB:448 | `\bpassages?\b` 0/17/17/1/11/0 = 46 | **Collision:** "the overview passage" means the README overview text (SP:1820, AR:1315). |
| "price" / "prices the charge list" / "re-price" / `repriced.yaml` | compute the score (evaluate the scoring function); recompute stored values under new weights | SP:1216, SP:1359-1363, AR:945-964, RB:4, CV:345 | `re-pric\|\bpric(e\|es\|ed\|ing)\b` 0/13/19/2/26/1 = 61 | The weights are novel; the operation is evaluation. "re-price" is recomputing stored derived values. It sits close to *monetary* cost ("a paid model call", SP:1469). |

### CLI / MCP surface

| kbase term / pattern | standard term | where (representative) | count (pattern; TH/SP/AR/CV/RB/ST = total) | exact or differs |
|---|---|---|---|---|
| "surface" / "each surface" / "the served surface" / "decision surface" | interface (CLI vs MCP); exposed API; HTTP endpoint | SP:603, SP:1796, SP:2529, AR:58, AR:1271, CV:417 | `\bsurfaces?\b` 0/24/36/13/1/0 = 74 | **Differs, polysemous.** Senses: CLI vs MCP rendering (SP:603); everything exposed (AR:58); an HTTP request shape (SP:1796 "The decision surface"); the public op set (SP:2529). |
| "bound" / "binds" / "binding" (command → tool) | exposed (registered) as an MCP tool | SP:2571, AR:32, CV:186-198 | `\bbound\b` 0/11/21/6/7/1 = 46 | **Collision:** "the bound" means the `--through` stage limit (SP:207 "the bound excepted", AR:1362); numeric bounds (SP:45, RB:233). |
| "family tool" | multiplexed tool with an `operation` discriminator | SP:2572, SP:2590, AR:2188-2224, CV:191 | `family` 1/9/27/9/0/0 = 46 | Exact. |
| "located" / "not located" | server with or without a default project root | SP:2608, SP:2635, AR:2238, AR:2285 | `\blocated\b` 0/11/11/1/0/0 = 23 | Exact. |
| "spelled" / "spelling" / "spells" | rendered / formatted / serialized as | SP:603, SP:616, AR:1622, AR:1569, CV:418 | `\bspell` 0/31/44/17/6/0 = 98 | Mostly "formatted for this surface". Other senses: RB:59 "spells the criterion it names" (= is a mnemonic); CV:311 "only its spelling differs" (= number formatting). |

### Phrasing patterns

| kbase term / pattern | standard term | where (representative) | count (pattern; TH/SP/AR/CV/RB/ST = total) | exact or differs |
|---|---|---|---|---|
| "X is \<doc / page / package\>'s" / "are [page §…]'s" | X is defined in (owned by) \<doc\> | SP:203, SP:457, SP:490, AR:657, AR:778, AR:999 | `` (§[0-9.]+\|\)\|`)'s[.:;,)] `` 0/32/18/1/0/1 = 52 | Exact. This is a citation idiom (the dead-link half belongs to another survey). |
| "What this release adds to the page and holds" / "What the page leaves to this document" | additional requirements; refinements | SP:793, SP:808, SP:835, SP:209, SP:501 | `What this release adds\|What the page leaves` 0/18/0/0/0/0 = 18 | Exact. This is boilerplate for "this section adds to …". |
| "the one X" / "one home" / "second home" / "second copy" / "lives once" / "monopoly" | single source of truth; sole owner | SP:1046, AR:361, AR:539, AR:1703, CV:215, CV:300, CV:119 | `the one \|one home\|second home\|lives once\|second copy` 0/34/57/13/2/0 = 106; `monopol` 0/0/2/2/0/0 = 4 | Exact. This is a DRY statement and does not describe a special mechanism. |
| "carries" / "carries no" / "carry" | contains / includes (a field); transports; supports | SP:303, SP:342, SP:440, AR:60, RB:6, ST:41 | `\bcarr(y\|ies\|ied\|ying)\b` 2/201/112/22/47/23 = 407 | **Differs, polysemous.** Senses: has a field or contains (SP:303, SP:342); admits or runs (SP:440 "a repository carries one run at a time"); transports (SP:1732, "travels" alongside); in scoring, *supports* (ST:41 "whether anything carries a node", RB:244 "nothing left to carry the assertion"). That scoring sense is a domain sense and must not be flattened to "contains". |
| "refused, naming X" / "naming" | the error message identifies X | SP:48, SP:168, SP:185, AR:1025 | `naming` 0/130/61/4/15/1 = 211 | Exact. |
| "holds" / "and holds" | contains; remains true; holds a lock | SP:793, SP:2088, AR:1377 | `holds?\b` 1/104/63/16/19/9 = 212 (all senses) | Polysemous. Senses: contains (SP:2088); remains in force (SP:793); holds a lock (AR:1377). |
| "re-point(s)" | rewrite link targets (update references) | SP:824, SP:849, SP:896, AR:432 | `re-point` 0/13/7/1/0/0 = 21 | Exact. |
| "moves" / "what moves one" | changes (a value); relocates | SP:1208, AR:962, CV:159 | `\bmoves?\b` 0/51/25/14/2/4 = 96 | Polysemous with `move-node` (relocation). |
| "reads as" / "answers as converted" | parses as; returns results as if | SP:953, SP:2005 | `reads? as\b` 0/17/11/4/4/0 = 36 | Exact. |
| "fold" / "folded" / "folds" | case-normalize; merge into; reduce | SP:633, SP:1298, AR:581, AR:818 | `\bfold` 0/12/5/1/1/0 = 19 | Polysemous. Senses: lowercase folding (SP:633); merging result files into the ledger (SP:1298); an arithmetic reduce (AR:581). |
| "issued through" / "issue" | apply / execute (a write) | AR:80, AR:82 | `\bissue[ds]?\b` 1/20/19/3/0/0 = 43 | Exact. It overlaps with "re-issue" above. |
| "mechanical" / "mechanical draft" / "Monotone safety" | deterministic, rule-based (no model); graceful degradation to a deterministic fallback | TH:22, TH:45, AR:45, AR:49, AR:1297 | `mechanical` 7/0/6/4/1/0 = 18 | Exact. CV:485-489 uses "mechanical" for a *dispatch rating* (rote task), which is a second sense. |
| "gate" / "dead-link gate" / "gate stage" / `edit-gate` | check / validation step (CI gate) | SP:1506, SP:1523, AR:81, AR:183, CV:27 | `\bgate\b` 0/7/14/8/1/0 = 30 | Exact. At AR:1917, "checked at the same gate" means the load check. |
| "volume" / "volume root" | input partition: one paper's top-level directory or its top `.tex` file | SP:20, SP:115, SP:3062 | `\bvolumes?\b` 0/77/47/1/2/0 = 127 | Defined twice: "volume root" is the top `.tex` file (SP:20), while "volume" is a top-level directory holding `index.md` (SP:115, SP:3062). |
| "sheet" / "volume digest" | rendered graph diagram (SVG); overview diagram | SP:95-155, AR:609 | `\bsheets?\b` 0/54/31/1/0/2 = 88; `digest` 0/19/11/0/9/0 = 39 | **Collision on "digest":** a hash (SP:1346 "one digest over every instruction", AR:839) vs the SVG overview (SP:110). |

### CONVENTIONS process terms

| kbase term / pattern | standard term | where (representative) | count (pattern; TH/SP/AR/CV/RB/ST = total) | exact or differs |
|---|---|---|---|---|
| "probe" (vs "verification") | ad-hoc exploratory check vs regression test | CV:129-133 | `\bprobe\b` 0/23/20/4/0/0 = 47 | **Collision:** the `probe` subcommand (SP:1434), which accounts for every SP/AR hit. |
| "The trap:" | pitfall; anti-pattern | CV:146, CV:152, CV:167, CV:172 | `The trap` 0/0/0/20/0/0 = 20 | Exact. |
| "omnibus" | aggregate test target (suite) | CV:29, CV:227, CV:610 | `omnibus` 0/0/0/5/0/0 = 5 | Exact. |
| "leg" | test case / test step | CV:164, CV:585, CV:620 | `\bleg\b\|\blegs\b` 0/0/0/5/0/0 = 5 | Exact. |
| "seat" / "briefing a seat" | agent role / agent definition | CV:13-15 | `\bseat\b` 0/0/0/4/0/0 = 4 | Exact. |

## Excluded on purpose (novel concepts)

These terms name the novel parts: the local judgments, how a model is asked for them, the combining
formula, and claim-graph extraction. They were left out of the table:

- **Quantities:** `rigor`, `bearing`, `strength`.
- **Relations:** `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted`.
- **Node kinds:** `claim`, `analysis`, `experiment`, `work`, `axiom`.
- **Judgement vocabulary:** charge / charged occurrence / finding, the `flt-*` ids, the statement and
  method vocabularies, `none`, "one item, one quantity, one call".
- **Formula terms:** P(n), D(n), L(n), C(n), "premise", "lift", "cap", "participates" / "contributes",
  "the trace".
- **Band names:** `ok to build on` … `unsubstantiated`.
- **Claim-graph extraction stages:** co-linearity cull, dependency attribution, analysis split,
  unmarked references.
- **Format elements:** statement marker, canonical anchor.

Places where a novel term is defined in bespoke wording for standard mechanics:

- **Cull** (SP:330-333). "A claim several answers cull goes into the earliest of their partners …;
  following those to a claim culled into nothing gives its root" is a union-find root, or the
  transitive closure of merges. "Culled into" means "merged into".
- **`demoted`** (SP:2521-2538, AR:514-533). This is a tombstone record for a removed feedback edge:
  "cut" means removed, and `restore-demoted` means undo. The relation name is domain vocabulary, but
  the mechanism is standard.
- **"station"** (AR:560-562, ST:305, SP:2382). This names which operator of the formula (min, max or
  cap) an edge feeds. The standard term is "term" or "operand position".
- **`*pending*`** (SP:2203 "the format's one no-number literal"; ST:98-117). It is a null sentinel.
  Its propagation is bespoke and is not SQL NULL: `max(⊥, x) = x` but `min(⊥, c) = ⊥`, so
  that part is genuinely novel.
- **"band ladder" / "rung"** (SP:2276-2286, RB:274, AR:1650). These are threshold buckets, each an
  inclusive lower bound. The labels are novel; the binning is standard. Also, "cut" doubles as a
  threshold here (RB:291).
- **"derivation"** in the formula sense (D(n), SP:2231, ST:303) is novel. It collides with the
  derived-view sense in the persistence table above.
- **"the walk"** in the strength sense (ST:95, AR:577) means propagation over the DAG. The
  arithmetic is novel; the traversal is standard.
- **"A value is a projection of a ledger line's ids alone"** (CV:345, RB:575) means the value is a
  pure function of the stored findings. That is an event-sourcing projection, and the standard term
  already fits.
- **"carries"** in the scoring sense (ST:41, RB:244) means "supports". It is domain usage and is
  listed in the table only because of the collision.
