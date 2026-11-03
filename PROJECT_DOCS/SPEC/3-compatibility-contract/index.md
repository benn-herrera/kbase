## 3. Compatibility contract

- A KB kbase builds is navigable in Claude Code or opencode through the kbase skill's reading
  route, and maintainable through the same skill's maintenance route by kb-maintainer;
  `kbase score` grades the KB's rigor and bearings.
- Exact: the metadata layer, the load-bearing forms, and the KB's placement (`kb-root/` beside
  `.git`; reading starts at `kb-root/entry-point.md` and descends through `index.md` files).
- Free and untested: presentation — wrap width, bullet characters and the like.
- Byte equality is asserted only where a named reader needs it:

| Byte-equal | Reader that needs it |
|---|---|
| Statement markers, `<!-- kbase-id: <id> -->` attached with no space ([page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)) | The marker reader; The add ops' and `move-node`'s locate and splice |
| `.index/format.yaml` | `kbload`'s stamp read — the version is the bytes the file holds — and every save, which rewrites it only where its bytes change |
| Register pages, `kb-summary.md` ([page §1.6](../../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)) | `verify`'s freshness check (`register-freshness`: each page rendered in memory and compared with the bytes on disk) |
| `.index/nodes.yaml`, `.index/edges.yaml` | The write ops' idempotence: whole files rendered deterministically |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | The precheck of a write batch's derivation and of `update-format` (`check: agents-redirect`, [page §1.7](../../../NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)) |
| The `` ``` math `` fence opener | The inventory's display-maths fence reader |
| The up-link line | `kb.UplinkMarker` and the readers that key on it |
| The claim-graph sheets' DOT, against the DOT goldens (`internal/sheet/testdata/`) | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, a re-render leaves an unchanged sheet's bytes as they are |

Contents:

- [§3 Stamped documents](stamped-documents.md)
- [§3 Formats](formats.md)
- [§3 `claim-graph.svg`](claim-graph-svg/index.md)
