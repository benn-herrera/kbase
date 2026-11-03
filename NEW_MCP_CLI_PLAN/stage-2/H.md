### H — write batches (expert)

**Design:**
- [§1.1 Nodes](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) — which prose fields may be `""`, titles one line
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — the batch in full
- [§2.3 The derivation after every write](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — the lookup-first rule batches report
- [§3.6 `verify`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md) — the empty batch as the remedy for a stale summary page
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the `write` tool description
  and the write ops' notes

Relies on stage 1: no write path over a degraded load; no migration on a write, and every write
over an older KB refused (`cmd/update-format.go` is the one converting writer).

**Scope:**
- `internal/write/` (`ops.go` `Run`, `Options`, `Result`, `runOverSource`; `records.go`
  `editRecords`, `recordEdit`, `conflicts`; `values.go` `parseValues`, `checkProse`);
- `internal/index/refresh.go` (the derivation, computed in memory);
- `internal/sheet/`;
- `cmd/write.go` (new: `kbase write`);
- `cmd/write_op.go` (`writeOpOptions`, `runWriteOp`, `writeOp`, `writeOpCommand`; `--no-refresh`
  goes; `--dry-run` is added);
- `cmd/surface.go` (`toolFamilies`, `servedServer`);
- `internal/mcp/mcp.go` (`writeOpTool`, `familyTool` for `write`, `Calls.WriteOp` at `:190`),
  `internal/mcp/server.go` (`Server.WriteOp`, `:47`);
- `internal/claimgraph/write.go`, `internal/score/invoke.go` (the internal callers of `write.Run`).

**Facts acted on:**
- [Page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) and [§2.3](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md) in full:
  - shape (`entry`, `operation`, `dry-run`, `commit`);
  - the CLI forms;
  - order with no reordering, and no references between entries;
  - all or nothing, the derivation included;
  - the two error phases, and the refusal items with `entry`;
  - dry run, locks and the value rules (CR or LF refused, whitespace collapsed);
  - the result keys and the batch outcome.
- `commit` lands in package I.
- Today:
  - one op per call (`Run(op string, opts Options) Result`, `internal/write/ops.go:198`);
  - every entry's refusal is collected rather than stopped at the first ("An entry's refusal does
    not stop the batch", `editRecords`' doc, `internal/write/records.go:131–134`);
  - the trailing refresh writes after the records (`index.RefreshReporting`,
    `internal/write/ops.go:273`);
  - `checkProse` allows a single line break and refuses an empty value
    (`internal/write/values.go:142–167`). [Page §1.1](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) lets `rationale` and `context` be `""`, and
    [§2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) refuses every line break.
- What the batch rules bind:
  - Every op that stands at `68021ba` runs as a batch of that op alone. It takes the batch's order,
    its all-or-nothing with the derivation, its dry run and its stop at the first apply refusal.
    A test that pins several apply refusals from one call moves (Rule 4), stating the new truth.
  - Those ops keep their own key vocabulary, their value checks (`checkProse` as it stands) and
    their idempotence under re-issue, until package S removes them.
  - This package adds [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s value check beside `checkProse`: a prose value holding CR, LF,
    U+0008 or U+000C is refused, any other whitespace run collapses to one space, the ends are
    trimmed, and `""` is accepted where the op's key allows it. The ops that packages J, K, M, L
    and N land use it for their keys.
- `write.Options.NoRefresh`: the `--no-refresh` flag goes from every subcommand. The `Options`
  field stays for its two internal callers alone, the build (`internal/claimgraph/write.go:29`)
  and the scoring run (`internal/score/invoke.go:316`), which keep their present derivation
  cadence here. Package R (stage 3) sets the build's, and package Q (stage 3) the scoring run's.
  The `NoRefresh` grep below brings in the tests that pass the flag.
- `git grep -l 'write\.Run(\|write\.Options\|write\.Result' -- '*.go'` →
  - `cmd/mcp_test.go`, `cmd/write_op.go`;
  - `internal/claimgraph/{elision,elision_fixture_test,write}.go`;
  - `internal/score/{invoke,invoke_test}.go`.
- `git grep -l 'NoRefresh' -- '*.go'` →
  - `cmd/{format_test,write_op}.go`;
  - `internal/claimgraph/{elision_fixture_test,write}.go`;
  - `internal/score/{invoke,invoke_test}.go`;
  - `internal/write/{ops,records_test,regression_test}.go`.
- `git grep -n 'no-refresh' -- tools/scripts justfile test-data/fixtures` →
  `test-data/fixtures/mcp/resources.jsonl:10` (the write family's help: "the op's values and
  no-refresh flags") and `tools/scripts/test-integration-results-fixture.sh:73`
  (`insert-claim-entry --no-refresh`, expecting `unchanged`). That leg drops the flag and still
  expects `unchanged`, since the old ops stay idempotent.
- `git grep -l 'runWriteOp\|writeOpOptions\|ArgumentEntry\|WriteOp:' -- '*.go'` →
  `cmd/{mcp_test,surface,write_op,writelock_test}.go`, `internal/mcp/server.go`.
- `git grep -l 'OpVocabulary\|write\.Ops()\|writeOpTool\|Calls{\|mcp\.Calls' -- '*.go'` →
  - `cmd/{mcp_test,resultkeys_test,write_op}.go`;
  - `internal/mcp/{mcp,mcp_test,server}.go`;
  - `internal/write/ops.go`.
- `git grep -l 'parseValues\|editRecords\|recordEdit' -- '*.go'` →
  `internal/write/{axiom_test,ops,ops_test,records,values}.go`.
- `git grep -l 'writeOpCommand' -- '*.go'` → 19 files: every `cmd/<op>.go` of the 18 ops, and
  `cmd/write_op.go`.
- The values files already carry their entries under `entry:` (for example
  `test-data/fixtures/mcp/seed.yaml:1`).
- The empty batch re-derives ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) "Shape"). It is the remedy for a stale summary page (page
  [§3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)).
- The build's split and ring cut reach `write.Run` through this package's code: `demote-claim-edge`
  is issued through `writer.raw` (`internal/claimgraph/stages.go:628` → `internal/claimgraph/write.go:58`).
  `test-integration-build-lamb-cycle` is the recipe that runs that path ("checking its cycle cut",
  its doc at `justfile:267`).
- `render-citation` stays a `query` operation until package N.
- [CONVENTIONS.md:504](../../PROJECT_DOCS/CONVENTIONS/testing/results-are-part-of-the-test.md) and [`:522`](../../PROJECT_DOCS/CONVENTIONS/testing/results-are-part-of-the-test.md) name `insert-claim-entry` for the transcript runner and the results
  fixture. They stay true through this stage, and package S (stage 4) moves them.
- The help this package writes (Rule 12, [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)):
  - the `write` tool description: one sentence, then the batch contract as [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) lists it;
  - `kbase://help/write` and the CLI root help carry the batch contract in full ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md));
  - every write op's MCP note: its arguments are batch entries;
  - every write op's CLI note: the batch arrives as YAML (JSON accepted) on stdin or in the file
    `--values` names, with `--dry-run`. Package I adds `--commit` to it.

**Contract updates:**
- [SPEC §8 "Values input"](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/values-input.md), ["Writes leave nothing stale"](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/writes-leave-nothing-stale.md) and "Batches", replaced by a citation of
  [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md).
- [SPEC §8 "Idempotence"](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/idempotence.md), replaced by a citation of [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md) and [§2.10](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md), except its statement
  of the old ops' adoption (the text opening "An insert of a claim, an analysis or an experiment
  adopts by title and host"), which stands until package S.
- [SPEC §7](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/index.md): the write result's keys.
- [SPEC §11](../../PROJECT_DOCS/SPEC/11-the-mcp-server.md): the `write` tool.
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- ARCHITECTURE:
  - §6 (write mechanism);
  - the §11 `internal/write` and `internal/mcp` entries;
  - I9 (the `write` tool's shape).

**Tests:**
- A mixed batch applies in order.
- Shape errors from several entries are all reported, and nothing applies.
- An apply refusal at entry 3 of 5 writes nothing and names `entry: 3`.
- An entry that would leave the records invalid is refused even if a later entry would repair it.
- A dry run's result equals the real run's, with `dry-run: true` and nothing on disk.
- An empty batch re-derives a stale page.
- A failure injected between the record flush and the derived files leaves the records unwritten.
- The [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) value check refuses a value with CR or with LF, collapses a whitespace run, and
  accepts `""`.
- The per-op subcommand takes entries with no `operation`, and `kbase write` takes mixed ones.
- The MCP `write` tool takes a batch: `write-op.jsonl` moves.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-integration-build-lamb`,
`just test-integration-build-lamb-cycle`.
