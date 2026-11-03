### One version read and written

kbase reads and writes exactly the current version (ARCHITECTURE
[§12](../12-interim-releases/index.md)). An older stamp is read through migration (below). A newer major or minor is refused by every
subcommand that reads the KB — `build` on an existing KB, `update-format`, `verify`, the write ops, the
queries, `score` and `export-latex`; `status` and `cancel` do
not load the KB — outcome `refused`, one
item `check: kb-format` naming the KB's version and kbase's, remedy "update kbase". No subcommand
writes an older format, and nothing downgrades.

**Migration happens at load, and only `update-format` saves it**
([page §2.6](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)). Reading an older KB converts it — the
KB's files as bytes in, the current form's files as bytes out, plus the set of repository-relative
paths the conversion made obsolete and the conversion's warnings — version step by version step, in
memory; every command then sees the current form, and a converted, unsaved KB answers every query
as converted. A read-only command (`verify`, the queries, `export-latex`) writes
nothing, and every other write — a write batch, dry run or not, `score`, `build`
resuming over the KB — is refused while the conversion is pending (`check: index`, remedy
`kbase update-format`). `update-format` ([§8](../8-maintenance-subcommands/index.md)) saves the conversion, in this
order: `.index/edges.yaml`, then `.index/nodes.yaml`, then every other file the conversion produced
(each document with its frontmatter removed); then the register pages and the
sheets; then the stamp file is written at the version the KB is still at, so that a stamp always
stands and always names the version the tree is at; then the obsolete paths are removed, less any
the save itself wrote; then the entry point, whose frontmatter the conversion stripped; and last
the stamp file is rewritten at the current version — the stamp is the mark that the migration
completed — after which it rewrites the maintained blocks. The removals are reported under `removed` ([§7](../7-tool-result-contract-and-exit-codes/index.md)). Every read of a KB file goes through one
loader, `internal/kbload`, which serves the migrated bytes to every command on an older KB
(ARCHITECTURE §11). A save that stops part-way leaves the old form
standing, stamped at the version it converts from, with some files already current. Where
`.index/nodes.yaml` then stands, both record files are the current form's —
no `2.0.x` KB carries a `nodes.yaml`, and it is written second — so the next load takes them as they
stand, strips any remaining frontmatter unread and derives nothing from the `2.0.x` registers or
frontmatter. Such a tree is not clean, so `update-format` refuses it (`check: dirty-paths`); the recovery
is to restore `kb-root/` and any moved build record from `HEAD`, then run `update-format` again
([page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)). On an unmigrated KB `verify` reports
`index-freshness` with remedy `kbase update-format`.

**`1.0.0`**, the first major bump, is one change of serialisation: standard YAML frontmatter (`---`
fenced, the same keys and values), with one change of shape — a leaf's
repeated per-node key groups (`exp-id`, `status`, `strengthens`; `sup-id`, `supports`) become one
mapping each in a top-level `experiment-nodes:` or `support-nodes:` list, keys and order kept, a
one-node leaf a one-element list; the build records as YAML (`kb-build-*.yaml`); `.index/*.yaml` as
YAML streams, one document per line, each the record as a JSON object after the `--- ` marker; a
writer never folded a record across lines. The YAML frontmatter stood at the top of the file, so the
up-link line ([§3](../3-compatibility-contract/index.md)) was the first line after its closing `---`.

**`2.0.0`**, the second major bump, is the scoring vocabulary: three quantities named, five node
types in place of six, and one scalar on every scoring edge. It kept the serialisation above and
changed field names, relation names and the arithmetic those fields feed. Its patch `2.0.1` changed
the bottom band's words on a derived line, and nothing else.

**`3.0.0`**, the third major bump, moves the graph into records. Every node and every edge is a
record in one of two authored files, `.index/nodes.yaml` and `.index/edges.yaml`; a statement marker
in a leaf hosts each claim, analysis and experiment; the register pages are rendered from the
records and never read back; and no document carries frontmatter at all, the stamp standing in
`.index/format.yaml`. Everything else — the index the queries, the sheets and `score`
read — is derived at load and never stored: strength,
derivation strength, build status and band, the premise and citation counts, a scoring edge's
`target_kind`, the citations, subtree aggregates, each document's kind (the entry point is the entry
point, a document whose child list names a child an index, any other document a leaf), each node's
host, canonical path and anchor. The scoring vocabulary of `2.0.0` —
five node types, three quantities, four scoring relations, one arithmetic — stands unchanged.

**`4.0.0`**, the fourth major bump, is what
[page §1.8](../../../NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md) defines. Of it, kbase now writes the
records' half: each scored record's `judged-hash`
([page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)), an
experiment's `status` `run` or `planned`, no `no-edge` field on a claim or an analysis and no
`strengthen-by` field on a claim ([page §1.1](../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)), and a
`references` edge from any kind to any kind
([page §1.2](../../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)); its markers, anchors and
summary pages ([page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md),
[§1.6](../../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)); and its citations
([page §1.5](../../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)). The rest of this section
describes `4.0.0`; `1.0.0`, `2.0.x` and `3.0.0` are the forms it supersedes, and their names appear
only in the migrations below.

