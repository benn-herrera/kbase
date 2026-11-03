### Shared vocabulary

Every result document is one YAML mapping, with keys in this order:

1. `outcome`.
2. `kb-root`: the absolute path. Present on every subcommand that resolves a KB, which is all of
   them except `models`, `configure-providers`, `configure-models`, `show-config`, `probe` and the `auth` subcommands; `init` omits it from results written
   while no `kb-root/` stands ([§8](../8-maintenance-subcommands/index.md)).
3. `migration`: a conditional record — `from`, `to`, `note`, `warnings` (one item per thing the
   conversion dropped or changed that the converted files do not show: `path`, `line` where it
   names one, `detail`; `[]` when none) — present on the result of every verb that reads a KB
   through the loader on an older format version, and absent otherwise ([§10](../10-metadata-format-version/index.md)):
   `update-format`, `verify`, `export-latex`, the queries, `score`
   and every write op — on `score` and a write op, whose run is refused while the conversion is
   pending ([page §2.6](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)), beside that refusal. `update-format`'s lists only the warnings
   its own keys do not ([§7](item.md)'s per-subcommand table). It follows `kb-root` where the result carries both.
4. The subcommand's own keys, in the order its table gives.
5. Exactly one of these, chosen by `outcome`, and it closes the document: `refusals` for `refused`
   and `retry`; `failures` for `failed`; `cancelled` for `cancelled` — a mapping of `stage` and
   `unit` on a build, and of `item` on a `score` run ([§8](../8-maintenance-subcommands/index.md)). `done`, `unchanged` and `bounded` close
   on no such key.

Keys are kebab-case. The one exception is query data under `results`, which keeps the query data's
own names verbatim (`node_type`, `host_anchor`). The same fact always uses the same key:
`written` (files this call wrote, a path named once however many times the call wrote it — a
converted file a phase writes and a later phase rewrites is still one entry; kb-root-relative inside
`kb-root/`, absolute outside it; `[]` when nothing was written), `id` (one node's id),
`count` (a total before any limit), `resume` (a shell command that continues the work), `state-dir`
(the state store's absolute path).

