### T — `verify` and `export-latex` (expert)

**Design:**
- [§1.6 Derived files: summary pages and sheets](../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md) — a volume, and the summary pages `verify` compares
- [§2.8 Results and exit codes](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) — "Check values": each fault's check name
- [§3.6 `verify`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)
- [§3.6 `export-latex`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md)
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings

Relies on stage 1: the `bare-id` fault, load errors and `update-format`. Relies on stage 2: the
empty write batch, the stale page's remedy.

**Scope:**
- `internal/argmark/`, `internal/mcp/spelling.go` (a remedy naming a write batch spells the batch
  on both surfaces: at the command line, the values document piped into the command;
  `argmark.Call` carries a list of mappings);
- `internal/index/verify.go` (the check constants and remedies);
- `internal/index/pages.go` (`CheckRegisterFreshness`);
- `internal/index/links.go` (the dead-link gate's check strings);
- `internal/kb/deadlinks.go` (`brokenFragment`: every fragment checked);
- `cmd/verify.go`;
- `cmd/export-latex.go`, `internal/export/report.go`.

**Facts acted on:**
- [Page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md) `verify`:
  - its faults and their remedies: `update-format`, the empty batch, or git and prose work;
  - demoted records as `findings`;
  - each fault's check name.
- [Page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s check table names the `verify` faults: `dead-link`, `unknown-id`, `bare-id`,
  `frontmatter`, `index-freshness`, `statement-marker`, `axiom-heading`, `register-freshness`,
  and the finding `demoted`.
- Today's check strings, as constants and as literals:
  - `git grep -n 'Check\w* *= *"' -- 'internal/index/*.go'` →
    - `"register freshness"` (`pages.go:16`);
    - `"frontmatter"`, `"statement marker"`, `"axiom heading"` (`verify.go:53–55`);
    - `"index freshness"` (`verify.go:121`).
  - `git grep -n 'Check: *"' -- 'internal/index/*.go' | grep -v _test` →
    `internal/index/links.go:31` (`"dead link"`) and `:50` (`"unknown id"`).
  - `bare-id` is stage 1's (`git grep -n '"bare-id"' -- internal/index`).
- `git grep -l 'CheckStatementMarker\|CheckAxiomHeading\|CheckIndexFreshness\|CheckRegisterFreshness\|CheckFrontmatterPresence' -- '*.go'` →
  `cmd/maintenance_test.go`, `internal/index/{pages,pages_test,refresh_test,verify}.go`.
- [Page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md) `export-latex`:
  - a `volume` as [page §1.6](../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md) defines it;
  - "simplified", not "degraded";
  - its refusals, "a load error" among them.
- The dead-link gate checks every link's fragment, "anchors included" ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)). Today
  it checks a fragment only on a link into a register page (`internal/kb/deadlinks.go`,
  `brokenFragment`; `internal/index/links.go:15`). A fragment is live where it is one of the target
  document's heading anchors (`kb.HeadingAnchors`) or, on a register page, an entry's anchor.
  Package R (stage 3) made the build emit no dangling fragment, so every built KB passes.
- The degraded-load help sentence went with the degraded state in stage 1. It was
  `degradedRefusedHelp`, "A KB whose load is degraded is refused, naming each fault; …", in
  `export-latex`'s help (`cmd/export-latex.go:145` at `68021ba`). Re-check:
  `git grep -n 'degradedRefusedHelp\|load is degraded' -- cmd internal` → expected nothing. A hit
  is this package's to remove from `export-latex`'s help, which names [page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md)'s "a load error"
  refusal in its place.

**Contract updates:**
- [SPEC §8 `verify`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/verify.md) and `export-latex`, citing [page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.

**Tests:**
- Each fault's check name is [page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s, `dead-link`, `unknown-id` and `frontmatter` included.
- A stale summary page's remedy is the empty batch, and running that batch clears the fault.
- A `volume` that is not a volume is refused.
- A link whose fragment names no heading of its target document is a `dead-link` fault; one into
  an existing heading is not.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-export-forms`.
