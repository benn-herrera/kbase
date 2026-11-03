### R — `build`, `init`, `status`, `cancel`, `wait`, `build-trail` (expert)

**Design:**
- [§1.1 Nodes](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) — titles unique within a kind, single-line values
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "Order": a remaining mention is refused at its entry
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "Values": the build's values take the value check
- [§2.3 The derivation after every write](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md) — the build's writes are followed by it
- [§2.5 The domain rule](../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md) — the build records in `kb-root/.build/`
- [§3.1 Node writes](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md) and [`add-claim`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-claim-add-analysis-add-experiment.md) — "Existing nodes" and the add refusals the
  build's values meet
- [§3.1 `remove-node`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md) — the op the build's elision moves onto
- [§3.5 Runs and run control](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/index.md), and [`build`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)
- [§3.5 `status`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/status.md), [`cancel`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/cancel.md), [`wait`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/wait.md) and [`build-trail`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)
- [§3.7 `init`](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md) — the shared `kb-root` refusal
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — notes and shared bodies: `build`'s notes, `status`' and `cancel`'s bodies
- [§4.5 Served resources](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md) — no subscription

Relies on stage 1: build records under `kb-root/.build/`. Relies on stage 2: `add-claim` and
`add-work`, and `remove-node`, each in its own `cmd/<op>.go`, run through `write.Run`.

**Scope:**
- `internal/build/` (`rows.go` `kbRootGuard`, `kbRootState`; `build.go` `startOverRemedy`, the
  refresh calls; `seed.go`; `evidence_test.go` `TestStatusDocument`);
- `internal/claimgraph/write.go` (the build's claim and work writes move to `add-claim` and
  `add-work`, with unique titles);
- `internal/claimgraph/split.go` (the analyses the split writes, with unique titles);
- `internal/claimgraph/graph.go` (`readGraph`'s join from a block claim to its block, which a
  title made unique no longer meets by title);
- `internal/claimgraph/elision.go` (the build's node removal moves to `remove-node`), and
  `cull.go`, its caller;
- `internal/claimgraph/gate.go` (the build's gate derivation);
- `internal/buildstore/` (`rewind.go` `Rewind`, `RewindOptions`, its reset command; `status.go`);
- `internal/mcp/` (`resources.go` `RewindBuildTool`, `BuildTrailResource`, `BuildResource`;
  `watch.go`; `launch.go` `BuildLaunch`);
- `internal/mcp/protocol` (the subscription);
- `internal/write/` (`Options.NoRefresh`, which goes once the build no longer sets it);
- `cmd/build.go`, `cmd/init.go`, `cmd/status.go`, `cmd/cancel.go`, `cmd/wait.go`;
- `cmd/build-trail.go` (new);
- `cmd/surface.go`, `cmd/resultkeys_test.go`;
- `tools/scripts/test-integration-mcp-build.sh` and its `justfile` doc;
- `tools/scripts/test-integration-build-lamb.sh`, `tools/scripts/test-integration-build-lamb-cycle.sh`;
- `tools/scripts/test-integration-monitor-arxiv.sh`, `tools/scripts/test-integration-build-arxiv.sh`.

**Facts acted on:**
- [Page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md):
  - `build`'s refusals ("the one refusal of a fresh build over existing ground"), `resume`'s
    remedy ("remove `kb-root/` to start over"), and the MCP result with `state: starting` after
    5 s;
  - `status`'s build and score results;
  - `cancel` and `wait` speak of any run;
  - `build-trail`'s result and its commands: "The reset is `git reset --keep <commit>`", with no
    target argument;
  - the build's KB holds unique titles.
- [Page §2.3](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md): the build's writes are followed by the derivation. How often the derivation runs inside
  a build is the coder's choice. The derived files are current at every stage commit.
- [Page §3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md) `init`: the shared `kb-root` refusal.
- [Page §4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md): no subscription.
- The [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) texts this package writes (Rule 12): `build`'s MCP and CLI notes, as Q writes
  `score`'s; `status`' and `cancel`'s bodies speak of any detached run, naming `build` and `score`
  as its kinds.
- Every reset a remedy or a command names is `git reset --keep <commit>` ([page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md) `build-trail`).
  `git grep -n 'reset --hard' -- '*.go' ':!*_test.go'` →
  - `internal/build/build.go:713`, `:715`, `:722` (`startOverRemedy`). `:713` also names
    `rewind-build`'s `started-at` target, which goes: the start-over command is `build-trail`'s
    `started-at` command;
  - `internal/buildstore/rewind.go:79`, `:81`, which becomes `build-trail`'s per-boundary command.
  - A fresh build over an existing `kb-root/` and a `resume` with nothing to resume take page
    [§3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)'s own refusals and remedies.
- Today:
  - `kbRootGuard` accepts a spine-only `kb-root/` (`internal/build/rows.go:74–84`);
  - `rewind-build` is a listed tool (`cmd/surface.go:166`);
  - `Rewind` takes a target (`targetBoundary`, `internal/buildstore/rewind.go:138`);
  - the build writes with the derivation skipped, `NoRefresh: true` in `w.options`
    (`internal/claimgraph/write.go:29`), which its writes at `:41` and `:58` use;
  - the build's own derivation runs: `git grep -n 'refresh(\|Refresh(' -- internal/build internal/claimgraph ':!*_test.go'`
    → `internal/build/build.go:495`, `:501`, `:505`, `internal/build/rows.go:349`,
    `internal/build/seed.go:57`, `internal/claimgraph/gate.go:15`. Stage 1 renamed `refresh`; if the
    names moved, `git grep -n 'index\.[A-Z][A-Za-z]*(' -- internal/build internal/claimgraph ':!*_test.go'`
    finds the calls.
- The build's writes through ops that package S (stage 4) removes:
  - `insert-claim-entry` and `insert-work-entry` (`internal/claimgraph/write.go:79`, `:132`). They
    move to `add-claim` and `add-work`. A quote locator becomes `line` plus `snippet`.
  - `drop-claim-node` (`internal/claimgraph/elision.go:23`). It moves to `remove-node`. That op
    refuses a remaining mention at the entry ([page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md)), as `drop-claim-node` already does
    under the batch (package H). The build orders its entries so that each is valid where it stands
    ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) "Order"). Today the cull stage (`internal/claimgraph/cull.go`) issues
    `plan.Culls` in plan order, so a culled claim whose prose names a claim culled earlier in the
    batch stops the build; `TestIssueCullsDropsAClaimNamedOnlyByAnotherCull` pins that. Order the
    culls namers first; two culled claims naming each other need an edit entry ahead of either
    removal, the remedy [page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md) names.
  - The build-only ops `add-build-edges`, `demote-claim-edge` and `split-claim`
    (`internal/write/ops.go:23`, `:28`, `:33`) stay.
- The build's values meet the new ops' rules. `add-claim` refuses a prose value holding a line
  break ([page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-claim-add-analysis-add-experiment.md) Refusals, [§2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) Values), and refuses a title its kind holds. The build issues
  single-line values, and imposes title uniqueness within each kind at its writes, in
  `internal/claimgraph/write.go` and `internal/claimgraph/split.go`: those are the build's
  node-minting writes (`git grep -n 'write\.Run(\|w\.run(' -- internal/claimgraph ':!*_test.go'`
  → `write.go:41`, `:58`, `:79`, `:132`, `:169`, `split.go:69`). The mechanism is the coder's,
  within those two files and `internal/claimgraph/graph.go`, where `readGraph` joins a block
  claim to its block by title (`blocks[p][title]`); a title the build made unique must still
  join to its own block, by a match that is exact, never by parsing the counter back off.
- `git grep -l 'RewindBuildTool\|BuildTrailResource\|BuildResource()\|BuildTrailURI\|BuildURI\|buildSubscription\|protocol\.Subscription' -- '*.go'` →
  - `cmd/{init_test,mcp_notify_test,mcp_test,surface}.go`;
  - `internal/mcp/{help,resources,server,watch}.go`.
- `git grep -l 'buildstore\.Rewind\|RewindOptions\|kbRootGuard\|kbRootState' -- '*.go'` →
  - `internal/build/{build,build_test,rows}.go`;
  - `internal/buildstore/{rewind,rewind_test}.go`;
  - `internal/mcp/resources.go`.
- `git grep -l -- 'kbase://build' -- tools/scripts test-data/fixtures justfile` → `justfile`,
  `test-data/fixtures/mcp/resources.jsonl`.
- What `kbase://build` holds that `status` lacks: the coder compares the two by their result keys,
  and lists in the Commit body what moved into `status` and what was dropped.
- The `status` consumers: `git grep -n 'status --state-dir' -- tools/scripts` →
  `test-integration-build-arxiv.sh:108`, `:140`, `test-integration-monitor-arxiv.sh:81`, `:117`,
  `:130`, `test-integration-results-fixture.sh:233`. What they pin:
  - `test-integration-monitor-arxiv.sh` reads `state` and `resume` (`:82`, `:118–119`: `resume`
    begins `kbase build `), and the build result's `resumed: true` (`:127`). It checks each
    result's key set through `TestResultDocument` (`cmd/resultkeys_test.go:259`).
  - `documentedKeys["status"]` (`cmd/resultkeys_test.go:21`) already lists [page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/status.md)'s build-result
    keys in its order. `documentedSubKeys["status.stages"]` (`:44`) is `{"stage", "commit"}`, and
    [page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/status.md)'s stages carry no commit, so that pin moves.
  - `TestStatusDocument` (`internal/build/evidence_test.go:22`), which `test-integration-build-arxiv.sh`
    runs over its `status` output, finds the first unrecorded stage by a nil `commit`. That pin
    moves to the stage's done or not-done mark.
- [CONVENTIONS "House Rules"](../../PROJECT_DOCS/CONVENTIONS/house-rules/index.md), [CONVENTIONS.md:178](../../PROJECT_DOCS/CONVENTIONS/house-rules/just-fmt-is-the-sanctioned-formatter.md): "`rewind-build`, built in `internal/mcp` and
  declared in `cmd/surface.go`, is the only one". With `rewind-build` gone, no listed tool remains.

- Built leaves hold no link whose fragment names no heading. Today the build keeps pandoc's
  reference wrappers, a link around a cross-reference whose fragment is the LaTeX label, e.g.
  `[Appendix <a href="…#…" data-reference-type="ref" …>7</a>](#apx:omitted_details)`
  (`internal/export/testdata/excerpts/link-around-anchor.md`). The build unwraps it to its inner
  link, which names the target document's heading, so package T's gate, which checks every
  fragment, passes over every built KB. Where the wrapper is produced:
  `git grep -ln 'data-reference' -- internal` → `internal/claimgraph/tree.go`,
  `internal/docgraph/partition.go` (and their tests); the export filter
  (`internal/export/kbase-export.lua`) reads the wrapper and is out of scope.

**Contract updates:**
- [SPEC §5](../../PROJECT_DOCS/SPEC/5-build-behaviour/index.md), [§6](../../PROJECT_DOCS/SPEC/6-build-state-ledger-and-monitoring/index.md) (`kbase status`, `kbase cancel`, `kbase wait`), [§8 `init`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/init.md) and [§11](../../PROJECT_DOCS/SPEC/11-the-mcp-server.md) (resources,
  subscription), citing [page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/index.md), [§3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md) and [§4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- ARCHITECTURE I9 (no listed tool), §8 and the §11 `internal/mcp` and `internal/buildstore` entries.
- [CONVENTIONS "House Rules"](../../PROJECT_DOCS/CONVENTIONS/house-rules/index.md) ([CONVENTIONS.md:176–179](../../PROJECT_DOCS/CONVENTIONS/house-rules/just-fmt-is-the-sanctioned-formatter.md)): no listed tool remains.

**Tests:**
- A spine-only `kb-root/` refuses a fresh build and `init` with `check: kb-root`.
- `resume` with no trail while `kb-root/` stands names the remedy.
- `build-trail` at the CLI and over MCP gives the same document, each command a
  `git reset --keep <commit>`.
- The MCP build still starting after 5 s returns `state: starting`.
- `test-integration-mcp-build` polls `status` to the end.
- A cross-reference wrapper in a built leaf is unwrapped to its inner link (a unit test over the
  `link-around-anchor` excerpt).
- The lamb legs, in `test-integration-build-lamb.sh` and `test-integration-build-lamb-cycle.sh`:
  - titles are unique within each kind: for each kind, the built KB's `find` rows hold no title
    twice;
  - at each `kb-build:` commit, the commit's `kb-root/` exported into a scratch directory under the
    leg's own output (with `git archive`, never checked out in the leg's repository), beside its
    own `.git` ([CONVENTIONS "Testing"](../../PROJECT_DOCS/CONVENTIONS/testing/index.md): an empty directory serves), answers an empty
    `kbase write --dry-run` batch with outcome `unchanged`: the derived files match the records
    there.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-integration-build-lamb`,
`just test-integration-build-lamb-cycle`, `just test-integration-mcp-build`,
`just test-integration-monitor-arxiv`, `just test-integration-build-arxiv`.
