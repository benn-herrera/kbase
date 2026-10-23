# ACTIVE PLAN — the edge census

Two read-only queries over the index's edge rows, so "where do each relation's bearings sit" and
"which edges are they" are one call each instead of a `deps` walk of every node:

- `edge-census`: per relation, the edge count, whether the score run's bearing pass asks about it,
  and on a scoring relation the edges at each bearing;
- `edges`: the edge rows themselves, optionally narrowed to named relations, paged like every list
  query.

## Settled shape

1. **`edge-census`** is a mapping query registered like `stats`: `queryCommand(…, false, …)`,
   `cobra.NoArgs`, no flags of its own. `results` is a mapping with one key per relation, in
   `kb.Relations` order — `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted` — every
   relation present at zero. Each value is a record, keys in this order:
   - `edges`: the number of index edge rows of that relation;
   - `walked`: `kb.BearingWalk(relation)` — whether a score run's bearing pass asks about the
     relation's edges (the same predicate `bearingItems` in `internal/query/scoring.go` selects by);
   - on a scoring relation (`kb.Scoring`) only, `bearings`: a list with one record per distinct
     bearing the relation's rows carry, each `bearing` then `edges` (how many rows carry it).
     `bearing` is emitted as the index's text, unchanged, as `deps` emits it. Order: numeric values
     ascending by `strconv.ParseFloat` of the text, then the `*pending*` record, then a
     `bearing: null` record for rows whose bearing is null. The `edges` of the `bearings` records
     sum to the relation's `edges`. A relation with no rows has `bearings: []`
     (`[]result.Record{}`, as `StatsPayload` builds its empty lists). A non-scoring relation
     carries no `bearings` key.
   - A row whose relation is not in `kb.Relations` is counted nowhere.
2. **`edges`** is a list query: `queryCommand(…, true, …)` (it gains `--limit`/`--offset` and the
   list result keys), `cobra.NoArgs`, one flag of its own: `--relation`, a string slice
   (`StringSliceVar`, repeatable and comma-separated), default empty meaning every relation. A value
   not in `kb.Relations` is refused as usage, writing nothing: check `usage`, key `--relation`, the
   value in `detail`, `allowed` naming `kb.Relations` — the same item shape the query verbs already
   use for a bad flag value (find it with `git grep -n 'checkUsage' cmd/*.go internal/query/*.go`
   and follow the nearest query precedent). Each result record, keys in this order: `source`,
   `relation`, `target`, `bearing` (the index's text or null, as `deps`), `context` (null where
   none), `walked` (`kb.BearingWalk(relation)`). Order: relation in `kb.Relations` order, then
   `source`, then `target`, then `context` (null first) — the same tie-breaks `deps` sorts by.
   Rows of a relation outside `kb.Relations` are left out.
3. **Both read the index's edge rows (`Index.edges`) and nothing else**: no derivation, no state
   store, no score ledger. Their `edges` counts are structural; the bearings they report are the
   values the rows hold, which `set-bearing` moves and no derived value does.
4. **Placement.** Payloads `EdgeCensusPayload(ix *Index) result.Record` and
   `EdgesPayload(ix *Index, relations []string) …` in `internal/query/payload.go`, beside
   `StatsPayload`; the list payload returns what the other list payloads return to
   `queryCommand(…, true, …)` — copy `StrengthBelowPayload`'s signature shape. Verbs in
   `cmd/edge-census.go` and `cmd/edges.go` (verb files are hyphen-named, as `cmd/strength-below.go`).

## Established facts

| Fact | Command | Answer |
|---|---|---|
| Relations and their order | `git grep -n 'var Relations\b' internal/kb/schema.go` | supports, founds, lifts, refutes, references, demoted |
| Which relations carry a bearing; which the bearing pass walks | `git grep -n 'var ScoringRelations\|var BearingWalkRelations' internal/kb/schema.go` | scoring: the first four; walked: supports, lifts, refutes (`founds` carries a bearing set by hand and is not walked) |
| The index's bearing is text | `git grep -n -A7 'func bearingValue' internal/index/records.go`; `git grep -n -A6 'func BearingText' internal/kb/render.go` | a loaded `EdgeRow.Bearing` is a Go `string` (`"0.5"`, `"*pending*"`) or nil (a pairing `kb.BearingLegal` refuses) — never a number. Text order is not numeric order (`*` sorts before digits; `1e-05` after `0.5`) |
| How `deps` emits a bearing and sorts rows | `internal/query/payload.go` around `bearing: e.Bearing` and its `slices.SortStableFunc` | emits the row's value unchanged; sorts by `relationRank`, then other end, direction, context |
| The walk's edge selection | `git grep -n -A6 'func bearingItems' internal/query/scoring.go` | `kb.BearingWalk(e.Relation)` |
| Where the index's edges are held | `git grep -n 'edges  *\[\]kb.EdgeRow' internal/query/load.go` | `Index.edges` |
| Mapping-query pattern | `cmd/stats.go`; `StatsPayload` in `internal/query/payload.go` | `queryCommand(&cobra.Command{…}, false, func(ix, args) { return answered(query.StatsPayload(ix)) })` |
| List-query-with-flag pattern | `cmd/strength-below.go` | `queryCommand(…, true, …)` then `cmd.Flags()…`, then `rootCmd.AddCommand(cmd)` |
| MCP tool description is `Short` | `git grep -n 'tool.Description = cmd.Short' cmd/mcp.go` | the `Short` string is all an MCP client sees |
| MCP tools derive from the command tree | `git grep -n 'func TestMCPToolsAreTheCommandTree' cmd/mcp_test.go` | no MCP transcript fixture is added |
| Compat comparison | `git grep -n 'comparedQueries\|stageCases' internal/query/compat_test.go` | runs cases through a reference toolchain that has neither verb: both stay out of `comparedQueries` and `stageCases` |
| SPEC location of the query headings | `grep -n '^### Queries\|^### `stats`' SPEC.md` | §8 Maintenance subcommands, `### Queries` (~668), `### \`stats\`` (~682) |

## Work package

| WP | Rating | Seat / model | Status |
|---|---|---|---|
| WP1 | expert (small) | go-coder (definition's pin) | landed |

Rated expert: the refusal item and list-payload return follow precedents the coder must locate.

### WP1 — the two verbs, their tests and the contract text

Help text (wrap the raw strings at the width `cmd/stats.go` uses):
- `edge-census` `Short`: `the edge count per relation, and per bearing on the scoring relations`.
  `Long`: `edge-census counts the index's edges of each relation, says whether a score run's bearing
  pass asks about that relation, and for each scoring relation — supports, founds, lifts and
  refutes — counts the edges at each bearing, ascending, then the pending ones. The edge counts are
  structural; the bearing counts read the bearings the index holds; neither reads the derivation.`
- `edges` `Short`: `the edges of the named relations, or of every relation, with their bearings`.
  `Long`: `edges lists the index's edge rows — source, relation, target, bearing, context, and
  whether a score run's bearing pass asks about the edge — ordered by relation, source and target.
  --relation narrows the list to the named relations; it repeats or takes a comma-separated list.`
  `--relation` usage string: `only edges of this relation (supports, founds, lifts, refutes,
  references, demoted); repeatable`.

Tests:
- `internal/query` unit tests over a constructed index:
  - census: every relation present, in order; `walked` per relation; a non-scoring relation has no
    `bearings`; a value whose text order differs from its numeric order (`"1e-05"` vs `"0.5"`) sorts
    numerically; `*pending*` after the numbers; a null-bearing scoring row a `bearing: null` record
    last; each relation's `bearings` `edges` sum to its `edges`; the relations' `edges` sum to
    `len(ix.edges)`; a relation with no rows has `edges: 0`, `bearings: []`.
  - edges: unfiltered returns every row in the stated order with the stated keys; `--relation
    founds` returns only founds rows with `walked: false`; two relations return both; an unknown
    relation is the usage refusal.
- `cmd/resultkeys_test.go` `keysFor`: `edge-census` → `mapping-query`; `edges` → `list-query`.
- `cmd/resultdocs_test.go` done-sweep: add `{"edge-census"}`, `{"edges"}`,
  `{"edges", "--relation", "founds"}`; refused sweep: add `run("refused", "edges", "--relation",
  "nope")`.
- `cmd/format_test.go` `queryCases` ("every query"): add `{"edge-census"}` and `{"edges"}`.
- `justfile`, the results-fixture run list: after `run stats done stats` add `run edge-census done
  edge-census` and `run edges done edges`.

Contract text (exact):
- SPEC.md §8 `### Queries`: after `### \`stats\`` add the headings `### \`edge-census\`` and
  `### \`edges\``; after the `stats` paragraph (the one beginning "`stats` reads the derivation")
  add: "`edge-census` and `edges` read the index's edge rows and no derivation: their edge counts are
  structural, and the bearings they report are the values the rows hold, which `set-bearing` moves.
  Each reports, per relation, `walked`: whether a score run's bearing pass asks about that
  relation's edges — `supports`, `lifts` and `refutes` — where a `founds` bearing is the user's,
  written by `set-bearing`."
- SPEC.md "Bounded results" paragraph: after "`stats`' ranked sections stay at 10." insert
  "`edge-census`' `bearings` hold one record per distinct bearing value."
- SPEC.md §7 result-keys table:
  - the list-query row: add `edges` to its first cell's verb list, and inside its `results`
    parenthetical, after the `deps` sentence, add "; `edges`' records are `source`, `relation`,
    `target`, `bearing` (as `deps`' bearing), `context` and `walked` (bool)";
  - the `show`, `stats` row: first cell `` `show`, `stats`, `edge-census` ``; append before the
    closing parenthesis: "; `edge-census`' mapping carries one record per relation in this order —
    `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted` — each `edges`, the relation's
    row count, `walked` (bool), then on a scoring relation `bearings`: one record per distinct
    bearing its rows carry, `bearing`, as the index writes it, and `edges`, numbers ascending, then
    `*pending*`, then null, `[]` where the relation has no rows".
- `USER_CONVENTIONS.md` (~147): "`show <id>` and `stats` return a mapping." → "`show <id>`, `stats`
  and `edge-census` return a mapping."; and add `edges` to the list queries enumerated right after
  it, after `weak-points`.
- `README.md` (~249): the query list ends "…, `weak-points`, `stats`, `edge-census` and `edges`."
- `internal/kbdocs/templates/README.tmpl.md` "Current Figures":
  - "The current ones come from `kbase stats`, run from the project root that holds this KB." →
    "The current ones come from `kbase stats` and `kbase edge-census`, run from the project root
    that holds this KB."
  - "It does not split the edges by class. `deps` reports each edge at a node with its
    `relation`, its class, and its `bearing`." → "`kbase edge-census` splits the edges by relation
    and counts the edges at each bearing; `kbase edges --relation <relation>` lists them. `deps`
    reports each edge at a node with its `relation` and its `bearing`."
- `internal/kbdocs/templates/CONVENTIONS.tmpl.md` operations table, after the `Claim-graph counts`
  row: `| Edge counts by relation and bearing | \`kbase edge-census\` |` and
  `| The edges of a relation | \`kbase edges --relation <relation>\` |`.
- ARCHITECTURE.md: no change (it lists no query verbs).

## Gates

`just edit-gate`, `just test`; before the landing `just test-integration` and `just checkpoint`.

## Not in this plan

- Fault-id counts per edge: the index holds a bearing, not the charges that priced it.
- A `--bearing` filter on `edges`.
- The adjagent skill's references to the query set (the sibling project's change).

## Post-sprint review

Once the landing is committed, one pass reviews the changes since the plan was activated. The
reviewers are an architect and a coder; before they start they read THESIS.md, SPEC.md,
ARCHITECTURE.md and CONVENTIONS.md, and they do not read this plan. Their subject is the changes
against those standards and whether the result is fit for its purpose, and their brief includes the
tests.

**Review**: findings against the four documents. **Remediation**: each finding landed in a follow-up
commit or declined with the reason recorded. **Burndown**: what remains, one fact to an item, below.
