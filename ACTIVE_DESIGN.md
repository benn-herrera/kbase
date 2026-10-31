# Design: served-surface follow-ups, thin stamped documents, tooling

Promoted from ROADMAP. Each item states the outcome; the plan built from this design rates and
scopes each.

## Served surface

- **Retry and failure guidance travels with the result, not the instructions.** Every `retry` and
  `failed` item's `detail` ends with what the outcome asks of the caller — re-issue the identical
  call later; re-issuing will not fix it — and the MCP instructions drop "Outcome retry: … outcome
  failed: …", which every session carries on every turn. SPEC §7 and §11 move with it.
- **The MCP instructions carry only what must hold before a call.** Every session carries them on
  every turn, so situational text moves to where it is needed, and text a tool definition already
  states goes:
  - `show-config` is named in the remedy of every configuration and provider refusal, not in the
    instructions;
  - each `claim-rigor.md` page opens with a generated line saying it is prose — take values from the
    query tool, never edit it — in place of the instructions' exception;
  - the sentences restating the query and write tools and `refresh`'s sheets, and the sheet and
    `.dot` specifics, go; the help-resource sentence goes once every single tool's description
    carries the pointer, as the family tools' do;
  - what stays is preventive: the `.index/` read/write prohibition with its existence-check
    allowance, the session rule (trimmed), and the skill pointer until adjagent's reference pages
    are slimmed. About 1,500 characters fall to about 500, measured by `just measure-mcp-surface`.
- **A value refusal carries no `allowed` where its domain is no closed set.** `set-rigor` given
  `rigor: "high"` refuses today with `allowed` listing the op's key names
  (`test-data/fixtures/mcp/refusal.jsonl`), where SPEC §7 defines `allowed` as "the closed set the
  value must come from". The item omits `allowed` there; its `detail` already states the range.
- **`refresh`'s `sheet: placeholder` becomes `sheet: not-drawn`.** The value means "nothing was
  drawn, whether or not a sheet stands" (SPEC §8), which `placeholder` misnames beside an older full
  drawing still on disk. SPEC §7/§8, `cmd/refresh.go`, `refresh`'s help, and the tests and
  results-fixture leg that read the value move with it.

## Stamped KB documents

- **The stamped `AGENTS.md` and `CONVENTIONS.md` carry no control-surface text.** Documenting
  commands, results and workflow there goes stale beyond kbase's reach once a KB is stamped — the
  existing KBs' copies still name `edge-census` and `render-claim-graph`, and are fixed by hand. The
  templates keep only what nothing else says about a KB: that it is a kbase KB worked through the
  kbase skill and MCP server, the prohibition on reading or writing `kb-root/.index/`, the scope pin
  (authored versus distilled), and the session rule where a harness without the MCP server needs
  it. The sheet and `.dot` specifics and every command's documentation leave them, the served
  surface carrying them (ARCHITECTURE §3). SPEC §3 "Stamped documents" moves with it; with thin
  templates, CONVENTIONS' "regenerated whole" rule needs no restamp mechanism. adjagent's guardrail
  backs the `.index/` prohibition mechanically (adjagent `346976f`).

## Tooling

- **Keep `go.mod`'s `go` line within Go's supported window.** Go supports its two newest minors; at
  each release's `tap-update`, raise the line where it has fallen behind the older of them.
- Revise every justfile `[doc]` string for brevity.
- Review every script under `tools/scripts/` for concision, removing guards and checks against
  states that cannot arise.
- **Progress for `build --replay` and `score --replay`.** Both hold every row in memory until the
  walk drains and write nothing before, so a long replay shows no count of items done or left.
