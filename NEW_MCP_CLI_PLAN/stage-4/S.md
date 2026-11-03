### S — remove the superseded ops (expert)

**Design:**
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — the batches the rewritten op script issues
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — a re-issued add or remove is refused
- [§3 Operations](../../NEW_MCP_CLI_DESIGN/3-operations/index.md) — the 42 operations and 16 tools that remain
- [§3.1 Node writes](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md), [`add-claim`, `add-analysis`, `add-experiment`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-claim-add-analysis-add-experiment.md), [`add-work`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-work.md) and [`add-axiom`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-axiom.md) — the keys the rewritten scripts send
- [§3.2 `restore-demoted`](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/restore-demoted.md), which stays

Relies on stage 2: every replacement write op and `citations-of`, and `kbase write`. Relies on stage
3: the new queries, and the build's and the scoring run's writes on the new ops.

**Scope:**
- `cmd/`: `add-pairs.go`, `add-rests-on.go`, `set-bearing.go`, `set-rigor.go`, `set-rationale.go`,
  `set-status.go`, `insert-claim-entry.go`, `insert-analysis-entry.go`, `insert-experiment-entry.go`,
  `insert-work-entry.go`, `insert-axiom-entry.go`, `mark-claim-in-leaf.go`, `drop-claim-edges.go`,
  `drop-inbound-edges.go`, `drop-edge.go`, `drop-claim-node.go`, `render-citation.go`, `deps.go`,
  `cited-by.go`, `referenced-by.go`, `strength.go`, `subtree.go`; their `cmd/surface.go` lines;
  `cmd/query.go` paging;
- `internal/write/` (each removed op's apply function, keys and helpers, and `RenderCitation`);
- `internal/query/` (each removed payload, `Page`, `DefaultLimit`);
- `test-data/fixtures/compat/ops.yaml`, `test-data/fixtures/compat/README.md`;
- `internal/write/opscript_test.go`;
- `cmd/relationpolicy_test.go` (`relationSeriesExempt`: each removed op's file leaves the list,
  since the test fails on an entry whose file no longer exists);
- `tools/scripts/test-integration-opscript-results.sh`, `test-integration-results-fixture.sh` (its
  `render-citation`, `cited-by` and `referenced-by` legs), `test-integration-parity-adjagent-sheet.sh`;
- the parity cases of the removed queries: each `internal/kbload/testdata/parity-2.0.1/<n>.args`
  whose first line is `deps`, `cited-by`, `referenced-by` or `subtree`, with its `<n>.exit` and
  `<n>.out`;
- the `justfile` docs that name a removed op: `git grep -n -E 'add-pairs|add-rests-on|set-bearing|set-rigor|set-rationale|set-status|insert-[a-z]+-entry|mark-claim-in-leaf|drop-claim-edges|drop-inbound-edges|drop-edge|drop-claim-node|render-citation|cited-by|referenced-by|deps|subtree' -- justfile`
  → none at `93840bc` but `strength` in the `strength-walk` README-asset recipe, which is not the
  op.

The MCP runner, the init and refused-write scripts, `seed.yaml`, the results `values/` files, the
Go transcript staging and the CONVENTIONS staging lines already issue only the new ops (the stage 3
burndown, `93840bc`).

**Facts acted on:**
- `citations-of` answers through its own `RunE` (`cmd/citations-of.go`) only because
  `queryCommand` still pages. With `cmd/query.go`'s paging gone, it joins `queryCommand`.
- `render-citation`'s unbalanced-bracket check is shared with `add-citation`; removing
  `render-citation` keeps it.
- The fixtures and scripts that name each old op, by
  `git grep -l -- '<op>' -- tools/scripts test-data/fixtures`, run per op name. The answers at
  `93840bc` (`mcp/` and `compat/` under `test-data/fixtures/`, scripts under `tools/scripts/`):
  - `add-rests-on`, `set-bearing`, `set-rigor`, `insert-claim-entry`, `insert-work-entry`:
    `compat/ops.yaml`, `mcp/resources.jsonl`;
  - `add-pairs`, `set-rationale`, `set-status`, `insert-analysis-entry`, `insert-experiment-entry`,
    `insert-axiom-entry`, `mark-claim-in-leaf`, `drop-claim-edges`, `drop-inbound-edges`,
    `drop-edge`, `drop-claim-node`: `mcp/resources.jsonl`;
  - `render-citation`: `mcp/frame-errors.jsonl`, `mcp/refusal.jsonl`, `mcp/resources.jsonl`,
    `test-integration-results-fixture.sh`;
  - `cited-by`, `referenced-by`: `mcp/frame-errors.jsonl`, `mcp/resources.jsonl`,
    `test-integration-results-fixture.sh`.
- `deps`, `strength` and `subtree` are also plain words and result keys, so a plain grep returns
  files that use the word and not the op. Their op uses:
  `git grep -l -E 'help/<op>"|"operation": *"<op>"|run <op> |kbase"? <op>( |$)|"name": *"<op>"' -- tools/scripts test-data/fixtures`,
  run per op → `mcp/resources.jsonl` and `test-integration-results-fixture.sh` for each of the
  three. `mcp/frame-errors.jsonl` pins the `query` tool's operation list, which names them, and
  moves with that list.
  - Not this package's, though `git grep -lw` returns them: `test-data/fixtures/eval/kb-quality-eval-brief.md`
    ("subtrees") and `tools/scripts/strength-walk.sh` (the README asset) use the word;
    `mcp/find-paged.jsonl`, `mcp/gui-project-dir.jsonl` and `mcp/stats.jsonl` carry the `strength`
    row key and `subtree_aggregates`, which stand.
- The parity cases: `for f in internal/kbload/testdata/parity-2.0.1/*.args; do head -1 "$f"; done | sort | uniq -c`
  → at `68021ba` 28 `deps`, 14 `cited-by`, 14 `gated-on`, 14 `referenced-by`, 11 `subtree`,
  10 `find`, 15 `show`, 1 `stats`. Package A (stage 1) retired the `gated-on` cases. The cases of a
  query this package removes are retired with it: their three files are deleted.
  `TestConvertedAnswersMatchTheGoldens` reads whatever `.args` files stand
  (`internal/kbload/parity_test.go:671`).
- After this package no parity query pages. The parity script passes no `--limit`: its `paging`
  array and the `case` line that sets it
  (`tools/scripts/test-integration-parity-adjagent-sheet.sh:36–37`) are deleted, and its header
  comment (`:10`) and evidence line (`:54`) stop naming `--limit 0`.
- The build-only op `add-build-edges` reads its keys from `add-pairs`'s spec
  (`opFields["add-pairs"]`, `internal/write/edges.go:83`). That spec stays under the build-only op
  when `add-pairs` goes.
- The op script's re-issue contract: every step re-issued is `unchanged`, with `kb-root/`
  byte-identical (`test-data/fixtures/compat/README.md`; `tools/scripts/test-integration-opscript-results.sh:80–83`).
  Under [page §2.10](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) a re-issued add or remove is `refused`. The rewritten script:
  - runs through `kbase write` batches: adds, then edges and edits ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md), "No references
    between entries");
  - wants each re-issued batch `refused` or `unchanged`;
  - keeps `kb-root/` byte-identical.
- `TestMCPWriteOpSchemas` and `TestValuesOpHelpNamesItsKeys` (`cmd/mcp_test.go:618`, `:740`) derive
  their list from the tree, so they need no hand edit. `cmd/resultkeys_test.go:67`
  (`case slices.Contains(write.Ops(), verb):`) files every `write.Ops()` member.

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): the removed ops' sections go, and the old ops' adoption text in "Idempotence" (opening
  "An insert of a claim, an analysis or an experiment adopts by title and host") goes with them.
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): a bullet listing each removed op and its replacement.
- [CONVENTIONS "Testing"](../../PROJECT_DOCS/CONVENTIONS/testing/index.md): any line naming the op script's or the parity script's removed ops.
- `test-data/fixtures/compat/README.md`.

**Tests:**
- Every removed name is absent from `kbase --help`, `tools/list` and `resources/list`. The
  transcripts pin this.
- The rewritten op script lands, and its re-issue leaves `kb-root/` identical.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-integration-build-lamb`.
