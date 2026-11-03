### P — `edges`, `trace` and the contribution function (expert)

**Design:**
- [§1.2 Edges and relations](../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md) — each relation's effect, which `contribution` reports
- [§2.8 Results and exit codes](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) — "Lists" and "Sorting"
- [§3.4 Queries](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/index.md) — the section's rules
- [§3.4 `edges`](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md) and [`trace`](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/trace.md)
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings

**Scope:**
- `internal/index/strength.go` (`MinDependencyStrength`, `liftStrength`, `capStrength`);
- `internal/query/` (`EdgesPayload`; a new trace);
- `cmd/edges.go`;
- `cmd/trace.go` (new);
- `cmd/surface.go`;
- the demoted record's replaced relation, carried to the row: `internal/kb/register.go` (`kb.Edge`),
  `internal/kb/indexstream.go` (`kb.EdgeRow`), `internal/index/derive.go` (`graphEdges`),
  `internal/index/records.go` (`edgeRows`). Today `EdgeRecord.ReplacedRelation` is dropped there,
  and re-matching a demoted row to `rows.Resolved.Demoted` in `query` is ambiguous (several
  demoted records can join one pair). `remove-node`'s `removed-edges` rows take the field through
  `query.EdgeRowRecord` with no edit of their own;
- `internal/index/derive_bench_test.go`: `just bench-load` fails at `aa6a64a` before any change,
  because `BenchmarkLoadDerive` feeds the 3.0.0 sheet fixture's raw bytes to a 4.0.0 strict load.
  The bench converts the fixture first (through `kbload`, which `index` tests may import), and
  that fix lands, with its own passing run, before the before-measurement is taken.

**Facts acted on:**
- [Page §3.4](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md) `edges`:
  - `id` with `direction`, defaulting to `both`;
  - repeatable `relation`;
  - rows with `manually_scored` and `replaced_relation`;
  - a fixed order.
- [Page §3.4](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/trace.md) `trace`:
  - `direction` required;
  - the full walk;
  - `sort` by `depth` or `contribution`;
  - rows in `edges` form plus `depth`, `strength`, `contribution`.
- `contribution` is "the value the scoring code computes … reusing that code, never a restated
  formula". Today it is computed inline in three helpers: `git grep -n 'func MinDependencyStrength\|func liftStrength\|func capStrength' -- '*.go'` →
  `internal/index/strength.go:151`, `:174`, `:193`. One per-edge function is factored out of them,
  and the three helpers and `trace` all call it.
- `git grep -l 'EdgesPayload' -- '*.go'` → `cmd/edges.go`, `internal/query/payload.go`,
  `internal/query/query_test.go`.
- `test-data/fixtures/mcp/edges-relation.jsonl` pins today's `edges` (Rule 8).
- `just bench-load` tees its numbers to `test-data/transient/bench-load/log.txt` (`justfile:141–143`),
  and each run overwrites the last.

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): `edges` and `trace` citing [page §3.4](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- [ARCHITECTURE §6 "Scoring"](../../PROJECT_DOCS/ARCHITECTURE/6-kb-model-and-maintenance/scoring/index.md): the per-edge contribution function.

**Tests:**
- The strength derivation's existing tests stay green unchanged. That is the evidence that the
  factoring changes no value.
- `trace` `in` and `out` over a diamond.
- `contribution` ascending gives the weakest input first.
- A planned experiment's row has a null contribution.

**Done-line** (after Rule 13's two): `just test-integration`; `just bench-load` before the
factoring and again after it, each log copied to `.claude-temp/P-bench-load-before.txt` and
`.claude-temp/P-bench-load-after.txt`. The report states ns/op and allocs/op for both runs. No
threshold applies.
