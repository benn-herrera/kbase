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
| Statement markers, `<!-- kbase-id: <id> -->` attached with no space ([§10](../10-metadata-format-version/statements-and-anchors.md)) | The marker reader; the add ops' and `move-node`'s locate and splice |
| `.index/format.yaml` | The format-version read — the version is the bytes the file contains — and every save, which rewrites it only where its bytes change |
| Summary pages, `kb-summary.md` ([§10](../10-metadata-format-version/documents-of-a-kb.md)) | `verify`'s freshness check (`register-freshness`: each page rendered in memory and compared with the bytes on disk); no person or session edits a page or takes a value from it |
| `.index/nodes.yaml`, `.index/edges.yaml` | The write ops' idempotence: whole files rendered deterministically |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | The precheck of a write batch's rebuild of the derived views and of `update-format` (`check: agents-redirect`, [Stamped documents](stamped-documents.md)) |
| The `` ``` math `` fence opener | The inventory's display-maths fence reader |
| The up-link line | The readers that key on the up-link line |
| The claim-graph sheets' DOT, against the DOT goldens | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, a re-render leaves an unchanged sheet's bytes as they are |

Contents:

- [§3 Stamped documents](stamped-documents.md)
- [§3 Formats](formats.md)
- [§3 `claim-graph.svg`](claim-graph-svg/index.md)
