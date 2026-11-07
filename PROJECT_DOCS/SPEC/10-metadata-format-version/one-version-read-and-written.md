### One version read and written

kbase reads and writes exactly the current version. The superseded forms are `1.0.0`, `2.0.x` and
`3.0.0`; an older stamp is read through migration (below). A newer major or minor, such as `4.1.0` or
`5.0.0`, is refused by every subcommand that reads the KB — `build` on an existing KB,
`update-format`, `verify`, the write ops, the queries, `score` and `export-latex`; `status` and
`cancel` do not load the KB — outcome `refused`, one item `check: kb-format` naming the KB's version
and kbase's, remedy "update kbase". No subcommand writes an older format, and nothing downgrades.

**Migration happens at load, and only `update-format` saves it.** Reading an older KB converts it in
memory, version step by version step: each step takes the KB's files to the next form's files and
reports the repository-relative paths it made obsolete and its warnings. Every command then sees the
current form, and a converted, unsaved KB answers every query as converted. Nothing is written. The
conversion is deterministic: the same files convert to the same result on every load and in
`update-format`, minted ids included. A read-only command (`verify`, the queries, `export-latex`)
writes nothing, and every other write — a write batch, dry run or not, `score`, `build` resuming over
the KB — is refused while the conversion is pending (`check: index`, remedy `kbase update-format`),
so a format change never rides along with another write.

`update-format` ([§8](../8-maintenance-subcommands/index.md)) saves the conversion in this order:

1. `.index/edges.yaml`, then `.index/nodes.yaml`, then every other file the conversion produced (each
   document with its frontmatter removed);
2. the summary pages and the sheets;
3. the stamp file, at the version the KB is still at, so that a stamp always exists and always names
   the version the tree is at;
4. removal of the obsolete paths, less any the save itself wrote;
5. the entry point, whose frontmatter the conversion stripped;
6. the stamp file, rewritten at the current version: the stamp is what records that the migration
   completed;
7. the maintained blocks.

The removals are reported under `removed` ([§7](../7-tool-result-contract-and-exit-codes/index.md)). A save that stops part-way leaves the old form in
place, stamped at the version it converts from, with some files already current. Where
`.index/nodes.yaml` then exists, both record files are the current form's — no `2.0.x` KB has a
`nodes.yaml`, and it is written second — so the next load takes them as they are, strips any
remaining frontmatter unread and derives nothing from the `2.0.x` registers or frontmatter. Such a
tree is not clean, so `update-format` refuses it (`check: dirty-paths`); the recovery is to restore
`kb-root/` and any moved build record from `HEAD`, then run `update-format` again. On an unmigrated
KB `verify` reports `index-freshness` with remedy `kbase update-format`.

