### D — load errors replace the degraded state (expert)

**Design:**
- [§2.4 Load errors](../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md) — the three faults that are load errors, and no degraded state
- [§2.8 Results and exit codes](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) — "Check values": `metadata` for a load error
- [§3.6 `export-latex`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md) — its "a load error" refusal, which takes the place of
  `degradedRefusedHelp`

**Scope:**
- `internal/kb/graph.go` (`LoadGraph`; `CheckDegraded`);
- `internal/index/derive.go` (`Rows.Degraded`, `DegradedItems`, `RefuseDegraded`, `resolveGraph`,
  `resolveRecords`);
- `internal/index/refresh.go` (`repair`);
- `internal/write/` (`degradedRefusal`, `overDegraded`);
- `internal/query/load.go`;
- `internal/build/build.go`;
- `cmd/refresh.go` (`refuseDegraded`, `degradedRefusedHelp`), `cmd/score.go`, `cmd/export-latex.go`;
- `internal/claimgraph/depends_stage_test.go`.

**Facts acted on:**
- [Page §2.4](../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md): nine load faults, `check: metadata`, no remedy op. Three of them have no supported
  producer: a duplicate edge identity, a dangling edge and a scoring cycle.
- `git grep -l 'Degraded' -- '*.go'` →
  - `cmd/{export-latex,maintenance_test,refresh,score}.go`;
  - `internal/build/{build,build_test}.go`;
  - `internal/export/filter_test.go`;
  - `internal/index/{derive,derive_test,refresh,refresh_test,verify}.go`;
  - `internal/kb/graph.go`;
  - `internal/query/{load,query_test}.go`;
  - `internal/write/{edges_test,ops,records,records_test}.go`.
- `git grep -l 'CheckDegraded' -- '*.go'` →
  - `cmd/maintenance_test.go`;
  - `internal/build/build_test.go`;
  - `internal/index/{derive,refresh_test,verify}.go`;
  - `internal/kb/graph.go`;
  - `internal/query/query_test.go`;
  - `internal/write/{edges_test,records_test}.go`.
- `refuseDegraded` and its help sentence go with the state. The sentence is `degradedRefusedHelp`
  (`cmd/refresh.go:112–113`: "A KB whose load is degraded is refused, naming each fault; …"), and
  `export-latex`'s help carries it (`cmd/export-latex.go:145`). Its place takes [page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md)'s
  `export-latex` refusal, "a load error".
- The build writes cycles and then cuts them:
  - `AddBuildEdges` and `DemoteClaimEdge` carry `overDegraded: true`
    (`internal/write/ops.go:53`, `:70`);
  - the ring row runs `index.BreakCycles` ([ARCHITECTURE §4](../../PROJECT_DOCS/ARCHITECTURE/4-stage-table.md), `depends-attributed`).

  With no degraded state, those two ops must still write over the cycle the build makes within the
  stage. The mechanism is the coder's choice, within `internal/write` and `internal/build`. The
  records at each stage commit hold no cycle, so every load of the built KB succeeds.

**Contract updates:**
- SPEC, by `git show 68021ba:SPEC.md | grep -n -i 'degraded'`. At `68021ba` every hit before [`:2438`](../../PROJECT_DOCS/SPEC/12-interim-releases/0.9.0-messages-is-the-default-chat-protocol.md)
  is this package's: [`:529`](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/item.md), [`:551`](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/item.md) (§7), [`:818`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/queries.md) (§8 Queries), [`:1141`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/score/refresh-and-format.md), [`:1162`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/score/outcomes.md) (`score`),
  [`:1246–1247`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/refresh.md) (`refresh`), [`:1260–1261`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/verify.md) (`verify`), [`:1702`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/one-version-read-and-written.md), [`:1829–1839`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-load-strict-on-syntax-lenient-on-integrity.md), [`:2127`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-migration-announces-itself.md), [`:2149`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/cycles-cut-as-the-walk-finds-them.md)
  (§10). Each is replaced, citing [page §2.4](../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): one bullet.
- [ARCHITECTURE §11 "Loading"](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/loading.md) ("The integrity faults are `index`'s and do not refuse…") and the §6
  `internal/index` row ([ARCHITECTURE.md:405](../../PROJECT_DOCS/ARCHITECTURE/6-kb-model-and-maintenance/index.md): "the degraded load's resolution").

**Tests:**
- Each of the three faults is a load error naming its file and record.
- A Go test of the depends stage (`internal/claimgraph/depends_stage_test.go`): the records at the
  stage commit, after the ring row cut a cycle its own edges closed, load with no load error.
- The build's ring row still demotes a cycle that its own edges close. Lamb-cycle pins this, and
  its `edges` and `verify` legs, which load the built KB, stay `done`.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-integration-build-lamb-cycle`.
