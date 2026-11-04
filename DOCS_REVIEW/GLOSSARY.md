# Glossary decisions for the doc rewrite

How the rewrite of THESIS, SPEC, ARCHITECTURE, CONVENTIONS and the scoring schemes words each house
term. Source: `SURVEY-vocabulary.md`.

Standing rules:

- **One word, one sense.** A house word with several senses is replaced site by site, by sense, never by
  find-and-replace.
- **Identifiers stay.** Command, file, field and flag names (`build-trail`, `ledger.jsonl`, `.index/`,
  `manually-scored`, `judged`) are interface; only the prose around them changes.
- **Novel terms stay, defined plainly.** Where a novel term's mechanics are standard, its definition says so.

## A. Replace with the standard term

| House term | Proposed wording | Note |
|---|---|---|
| "the derivation", "refresh", "re-derivation" (after a write) | rebuild the derived views | "derivation" is then reserved for D(n) in the strength formula |
| "derived layer", "derived rows", "the index" (derived sense) | derived views; the in-memory index | `.index/` holds source records: call it "the record files" in prose |
| "register page" / "summary page" | summary page | one name for one generated page |
| "landing", "the one-op landing" (score results) | writing the results (one write batch) | "land" for repository work → commit; for a band → falls in |
| "the existence rule": add of what stands refused, … | create fails if the item exists; delete fails if absent; an update to the same value is a no-op | the `Idempotence` heading stays: it is correct |
| "adopt" / "nothing adopted" | upsert / no upsert | |
| "birth", "birth-once", "scratch creation" | create; create-once | |
| "spine", "spine seed", "skeleton" | navigation tree (THESIS sense); scaffold (`init` sense) | |
| "the mark" (`manually-scored`) | the `manually-scored` flag | frees "mark" for statement markers |
| "dirty" (scoring sense) | stale | "dirty" kept only for uncommitted git state |
| "pre-flight commit" | snapshot commit | "preflight" kept for precondition checks |
| "stamp" (template documents) | write if absent | "stamp" kept for the format version only |
| "holder lock", "operation lock", "holds the state store" | the run lock | plus "the KB write lock": two locks, two names |
| "ledger" (git sense, `gitledger`) | commit history | "ledger" kept for the scoring run's `ledger.jsonl` |
| "commit trail" / "the trail" (stage commits) | checkpoint commits | `build-trail` the command keeps its name |
| "ring row", "ring cut", "the cycle walk" | cycle breaking: remove the back edge DFS finds | |
| stage-table "row" ("its row spends …") | stage | |
| "spends inference", "paid-for inference" | makes model calls; cached model answers | |
| "dial" | connect to; send a request to | |
| "census" | count report | |
| "re-ask-then-default" | retry once, then use the deterministic default | |
| "closes on", "the closing record" | ends with; the final record | |
| "seam" | interface | |
| queue "token(s)" | slot | "token" kept for LLM tokens |
| "closed text" / "the guard" | the item text / the input guard | |
| "closed set", "closed vocabulary" | fixed set | |
| "the prompt shelf" | the prompt templates | |
| "frame" (prompt sense) | template wrapper | "frame" kept for JSON-RPC |
| "passage" (prompt sense) | section (of the prompt input) | |
| "price", "re-price" | compute the value; recompute values under current weights | the per-fault amount is a "weight" |
| "surface" (CLI vs MCP) | interface; "both interfaces" | "decision surface" → decision endpoint |
| "bound" / "binds" (command → tool) | exposed as an MCP tool | `--through` "bound" → stop stage |
| "located" / "not located" | with / without a default project | |
| "spelled", "spelling", "spells" | rendered as | |
| "X is ⟨page/doc⟩'s" | X is defined in ⟨doc⟩ | |
| "What this release adds to the page and holds" | (removed by the SPEC rehoming) | |
| "the one X", "one home", "second home", "lives once" | single source of truth | |
| "re-point" | update references | |
| "reads as" | parses as | |
| "issue(d) through" | apply(ied) through | |
| "gate" (doc sense) | check | `edit-gate` keeps its name |
| "volume root" | the top `.tex` file | "volume" defined once: one paper's directory |
| "digest" (the SVG overview) | overview diagram | "digest" kept for hashes |

## B. Split by sense, site by site

| House term | Senses → wording |
|---|---|
| "stands" | exists · is unchanged · is placed · remains · (a person's value) is kept |
| "re-issue" | repeat (idempotence) · retry (caller or transport) |
| "the walk" | stage pipeline · item loop · strength propagation · DFS · traversal |
| "carries" | has / contains · runs · transports · **supports** (scoring sense, keep the domain word) |
| "holds" | contains · remains in force · holds a lock |
| "moves" | changes · relocates (`move-node`) |
| "fold" | case-fold · merge into · reduce |
| "cut" | remove (an edge) · threshold (a band) |
| "ask" | model call (the request) · prompt (the template) · question · `[asks]` keeps its config name |
| "mechanical" | deterministic, no model (THESIS keeps "mechanical draft") · rote (dispatch rating) |

## C. Keep: novel, define plainly

| Term | Plain definition to state once |
|---|---|
| cull, "culled into" | merge; the survivor of a chain of merges is its root (union-find) |
| `demoted` | a tombstone for an edge removed to break a cycle; `restore-demoted` undoes it |
| "station" | which term of the formula (min, max, cap) an edge feeds |
| `*pending*` | a null with its own propagation rules (`max(⊥, x) = x`, `min(⊥, c) = ⊥`) |
| band ladder, rung | threshold buckets, each an inclusive lower bound |
| "judged hash" | the input hash a value was scored against; a changed hash marks the value stale |
| "maintained block" | a generated region between marker comments |
| "family tool" | one MCP tool with an `operation` argument |
| "sheet" | a rendered claim-graph diagram (SVG) |
| "host" | the document holding a node's statement |
| "mint" | generate a new unique id |

## D. CONVENTIONS process terms

| House term | Proposed |
|---|---|
| "probe" (process sense) | exploratory check (the `probe` command keeps its name) |
| "The trap:" | Pitfall: |
| "omnibus" | aggregate target |
| "leg" | test case |
| "seat" | agent role |

## Left as is

"naming" (in "refused, naming X") — plain English, one sense.
