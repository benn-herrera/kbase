### O — `find`, `show`, `stats` (expert)

**Design:**
- [§1.1 Nodes](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) — strength by kind, which `find` reports and filters on
- [§2.8 Results and exit codes](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) — "Lists" and "Sorting"
- [§3.4 Queries](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/index.md), [`find`](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/find.md), [`show`](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/show.md) — the section's rules, `find` and `show`
- [§3.4 `stats`](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/stats.md)
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the `query` tool description
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — shared bodies by command: `find`'s and `stats`' bodies

Relies on stage 1: the 4.0.0 anchors and the citation reader (Stage), from which `show`'s `host`
and `host_anchor` come.

**Scope:**
- `internal/query/` (`query.go` `Find`, `Node`; `payload.go` `FindPayload`, `ShowPayload`,
  `StatsPayload`, `identity`);
- `cmd/find.go`, `cmd/show.go`, `cmd/stats.go`;
- `cmd/query.go` (the `query` tool's shared help);
- `internal/index/derive.go` and its tests (`index.Rows.RegisterAnchors`, beside `RegisterPages`);
- `internal/kbload/testdata/parity-2.0.1/` (the `find` and `show` cases);
- `tools/scripts/test-integration-parity-adjagent-sheet.sh`;
- `tools/scripts/test-integration-results-fixture.sh` (the `find` legs).

**Facts acted on:**
- [Page §3.4](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/index.md) `find`, `show` and `stats`, and [page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) "Lists" and "Sorting":
  - `find` filters over every kind, with rows `id`, `kind`, `title`, `host`, `strength`,
    `dependents`. The one "no node" refusal's remedy, "kbase find lists the nodes that stand"
    (`kb.NoNode`, `internal/kb/kb.go`), holds for every kind only once `find` does; a test refuses
    an unknown id and finds a non-claim node with `find`;
  - `show`'s `host`, `host_anchor` and `summary`, with no `canonical_path` and no `citation_count`;
  - `sort` and `order`, every key defaulting to `ascending`;
  - `limit` only beside `sort`, a negative N taking the last N, and `limit: 0` refused.
- The [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) bodies this package writes (Rule 12): `find`'s help says, in one line, that
  `min: 0` leaves out pending nodes; `stats`' help points to `find`'s strength filters.
- `git grep -l 'FindPayload\|ShowPayload\|StatsPayload' -- '*.go'` →
  - `cmd/{find,score_test,show,stats}.go`;
  - `internal/query/{payload,query_test}.go`;
  - `internal/score/invoke_test.go`;
  - `internal/write/bearing_test.go`.
- The derived rows keep `CanonicalPath` and `CitationCount` (`internal/kb/indexstream.go`,
  `NodeRow`). `ShowPayload` stops emitting `canonical_path` and `citation_count`. `host` is the
  host leaf for every kind, from `Rows.Hosts` (kept as `Index.hosts`): a hosted claim's
  `CanonicalPath` is its summary page, not its host leaf (`internal/index/derive.go`, `nodes`).
  `summary` is `RegisterPages[id] + "#" + RegisterAnchors[id]`, null where `RegisterPages` has no
  entry: `RegisterAnchors` is a new field on `index.Rows` beside `RegisterPages`, set from the
  derivation's page anchors (`x.anchors = x.pageAnchors()` in `deriveGraph`), so no second copy of
  the anchor walk lives in `query`. The rows' other readers, `internal/sheet/graph.go:93`, `:97` and
  `internal/score/text.go:192`, `:216` (`row.CanonicalPath`), are untouched.
- Paging today: `--limit` default `query.DefaultLimit` and `--offset` (`cmd/query.go:132–133`), and
  `query.Page` (`internal/query/payload.go:35`). `git grep -l 'DefaultLimit\|query\.Page' -- '*.go'` →
  `cmd/query.go`, `internal/query/payload.go`.
  - `find` stops using them here.
  - The old list queries (`deps`, `cited-by`, `referenced-by`, `strength`, `subtree`) keep them
    until package S (stage 4), which removes them with those queries.
- The scripts that pass a limit: `git grep -n -- '--limit' -- tools/scripts/test-integration-results-fixture.sh` →
  `:119`, `:131`, `:142` (`score`, package Q) and `:186`, `:187` (`find`, this package). Today
  `:186` expects `find "" --limit 1` done and `:187` expects `find "" --limit -1` refused. Under
  [page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) both are refused without `sort`. The two legs pin [page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s rules instead: `limit`
  without `sort` refused, `limit: 0` refused, and `sort` with a positive and with a negative
  `limit` done.
- The parity recipe compares query answers over a converted 2.0.1 KB with frozen goldens:
  - It passes `--limit 0` to every query but `show` and `stats`
    (`tools/scripts/test-integration-parity-adjagent-sheet.sh:36–37`). Once `find` refuses
    `limit: 0`, the script passes it only to the queries that still page.
  - `for f in internal/kbload/testdata/parity-2.0.1/*.args; do head -1 "$f"; done | sort | uniq -c`
    → among others 10 `find` cases and 15 `show` cases. Each one whose answer shape this package
    moves is a pin (Rule 4). It moves through the golden-side rules of
    `TestConvertedAnswersMatchTheGoldens` (`internal/kbload/parity_test.go:662`) or through its
    `.args`, stating the truth it now tracks.
- `test-data/fixtures/mcp/find-paged.jsonl` pins today's paging (Rule 8).

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): `find`, `show` and `stats` citing [page §3.4](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/index.md), and the list rules citing [page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md).
- [SPEC §7](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/index.md): query result keys.
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.

**Tests:**
- Each `find` filter, one kind at a time.
- `min: 0` keeps exactly the scored nodes.
- `limit` without `sort` is refused, and so is `limit: 0`.
- A negative limit keeps the sort's order.
- A pending value sorts below every number, and a null sorts last both ways.
- `show` for each kind.
- `stats` names `find`'s filters.

**Done-line** (after Rule 13's two): `just test-integration`.
