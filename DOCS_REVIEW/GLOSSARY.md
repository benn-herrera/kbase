# Glossary decisions for the doc rewrite

How the rewrite of THESIS, SPEC, ARCHITECTURE, CONVENTIONS and the scoring schemes words each house
term. Source: `SURVEY-vocabulary.md`, which holds each term's senses and example sites.

## Standing rules

1. **Names stay.** Anything in backticks, any quoted refusal or message text, every code symbol,
   command, file, flag, config key, result field and relation name stays exactly as written
   (`build-trail`, `ledger.jsonl`, `.index/`, `manually-scored`, `mcp.Spelling`, "edge exists",
   `supports`). Rewrite only the prose around them. Names beat every row below.
2. **Headings stay** unless a row says to rename them. If you rename a heading, list its old and new
   anchor in your report. Edit no link outside your own sections.
3. **Prose only.** Rows apply to running text, never to code blocks, examples of output, or quoted
   material.
4. **One word, one sense, site by site.** Never find-and-replace. Read each site, decide its sense,
   and write that sense's wording.
5. **Longest match wins.** Where two rows match the same phrase, the row with the longer phrase
   applies ("the one-op landing" follows the "landing" row, not "the one X").
6. **No listed sense fits:** use the plainest verb that keeps the meaning, and list the site
   (`file:line`) in your report.
7. **Unlisted house term:** keep it, list it with `file:line` in your report, and coin no
   replacement.
8. **Novel terms stay** (section C). Never replace them; their plain definitions are written once,
   at the place C names.

## A. Replace with the standard term

| House term | Write | Note |
|---|---|---|
| "the derivation", "refresh", "re-derivation" (regenerating derived files after a write) | rebuild the derived views | "derivation" is reserved for D(n) in the strength formula. "re-derive" in the score-resume sense (SP:1359) follows the "price" row |
| "derived layer", "derived rows" | derived views | for "the index", see B |
| "register page" / "summary page" | summary page | one name for one generated page |
| "the existence rule": an add of what stands refused, … | create fails if the item exists; delete fails if it is absent; an update to the same value is a no-op | the `Idempotence` heading stays: it is correct |
| "adopt" / "nothing adopted" | upsert / no upsert | |
| "birth", "birth-once", "scratch creation" | create; create-once | |
| "the mark" (`manually-scored`) | the `manually-scored` flag | frees "mark" for statement markers |
| "dirty" (scoring sense) | stale | "dirty" kept only for uncommitted git state |
| "pre-flight commit" | snapshot commit | "preflight" kept for precondition checks |
| "holder lock", "operation lock", "holds the state store" | the run lock | with "the KB write lock": two locks, two names |
| "ledger" (git sense, `gitledger`) | commit history | "ledger" kept for the scoring run's `ledger.jsonl` |
| "commit trail" / "the trail" (stage commits) | checkpoint commits | |
| "ring row", "ring cut", "the cycle walk" | cycle breaking: remove the back edge DFS finds | |
| stage-table "row" ("its row spends …") | stage | |
| "spends inference", "paid-for inference" | makes model calls; cached model answers | |
| "dial" | connect to; send a request to | |
| "census" | count report | |
| "re-ask-then-default" | retry once, then use the deterministic default | |
| "closes on", "the closing record" (prose) | ends with; the final record | the heading "The closing record" stays (rule 2) |
| queue "token(s)" | slot | "token" kept for LLM tokens |
| "closed text" / "the guard" | the item text / the input guard | |
| "closed set", "closed vocabulary" | fixed set | |
| "the prompt shelf" | the prompt templates | |
| "frame" (prompt sense) | template wrapper | "frame" kept for JSON-RPC |
| "passage" (prompt sense) | section (of the prompt input) | |
| "price", "re-price", "re-derive" (score resume) | compute the value; recompute values under the current weights | the per-fault amount is a "weight" |
| "bound" / "binds" (command → tool) | exposed as an MCP tool | `--through` "bound" → stop stage |
| "located" / "not located" | with / without a default project | |
| "X is ⟨page/doc⟩'s" | X is defined in ⟨doc⟩ | |
| "What this release adds to the page and holds" | (removed by the SPEC rehoming) | |
| "one home", "second home", "lives once", "second copy" | single source of truth | |
| "the one X", only where the sentence claims a fact or mechanism is defined in exactly one place (e.g. SP:1405) | the single source of X | "the one X" meaning "the only X" (SP:81, SP:606) stays plain English |
| "re-point" | update references | |
| "reads as" | parses as | |
| "issue(d) through" | apply(ied) through | |
| "gate" (doc sense) | check | `edit-gate` keeps its name |
| "volume root" | the top `.tex` file | "volume" defined once: one paper's directory |
| "digest" (the SVG overview) | overview diagram | "digest" kept for hashes |

## B. Split by sense, site by site

| House term | Senses → write |
|---|---|
| "stands" | exists · is unchanged · is placed / points at ("stands at", SP:505) · is written ("stands bare", SP:606) · as it is now ("as it stands", SP:217) · remains · (a person's value) is kept |
| "re-issue" | repeat (idempotence) · retry (caller or transport) |
| "the walk" | stage pipeline · item loop · strength propagation · DFS · traversal |
| "carries" | has / contains · runs · transports · **keep "carries"** in the scoring sense (an edge giving a node strength, ST:41). Never write "supports" there: `supports` is a relation name |
| "holds" | contains · remains in force · holds a lock |
| "moves" | changes · relocates (`move-node`) |
| "fold" | case-fold · merge into · reduce |
| "cut" | remove (an edge) · threshold (a band) |
| "ask" | model call (the request) · prompt (the template) · question · concurrent call batch ("ask batch", AR:34) · `[asks]` and `ask_sha256` keep their names |
| "landing", "land", "the one-op landing" | writing the results, one write batch (score) · commit (repository work) · falls in (a band) |
| "the index" | the record files (`.index/` source records, SP:625, SP:1002) · the in-memory index (derived, SP:2045) · index page (`index.md`) |
| "stamp" | format version (the stamp) · write if absent (template documents) · records ("stamps `ask_sha256`", AR:839) |
| "spine", "skeleton" | keep "spine" in THESIS (the argument's backbone) · navigation tree · scaffold (`init`) |
| "seam" | interface, named by what it injects (the `Write` interface, the model-call interface) |
| "surface" | interface (CLI vs MCP only) · public operations (AR:523, SP:2529) · API flavour (AR:2068) · decision endpoint ("decision surface") |
| "spelled", "spelling", "spells" | formatted as (output form, SP:636) · write out as a literal ("Never spell a relation", CV:249) · write as (CV:387) · its formatting (CV:311) |
| "mechanical" | deterministic, no model (THESIS keeps "mechanical draft") · rote (dispatch rating) |

## C. Keep the term at every site; define it once

These name novel concepts. Never replace them. Each definition is written once, at the place given
(section titles as in the new SPEC outline); elsewhere the term is used bare.

| Term | Definition | Written once, at |
|---|---|---|
| cull, "culled into" | merge; the survivor of a chain of merges is its root (union-find) | SPEC §5 Co-linearity cull |
| `demoted` | a tombstone for an edge removed to break a cycle; `restore-demoted` undoes it | SPEC §10 Cycles |
| "station" | which term of the formula (min, max, cap) an edge feeds | STRENGTH scheme |
| `*pending*` | a null with its own propagation rules (`max(⊥, x) = x`, `min(⊥, c) = ⊥`) | STRENGTH scheme |
| band ladder, rung | threshold buckets, each an inclusive lower bound | SPEC §10 The band ladder |
| "judged hash" | the input hash a value was scored against; a changed hash marks the value stale | SPEC §10 Provenance and the judged hash |
| "maintained block" | a generated region between marker comments | SPEC §10 Documents of a KB |
| "family tool" | one MCP tool with an `operation` argument | SPEC §11 |
| "sheet" | a rendered claim-graph diagram (SVG) | SPEC §3 `claim-graph.svg` |
| "host" | the document holding a node's statement | SPEC §10 Statements and anchors |
| "mint" | generate a new unique id | SPEC §10 Five node types |

## D. CONVENTIONS process terms

| House term | Write |
|---|---|
| "probe" (process sense) | exploratory check (the `probe` command keeps its name) |
| "The trap:" | Pitfall: |
| "omnibus" | aggregate target |
| "leg" | test case |
| "seat" | agent role |

## Left as is

"naming" (in "refused, naming X") — plain English, one sense.
