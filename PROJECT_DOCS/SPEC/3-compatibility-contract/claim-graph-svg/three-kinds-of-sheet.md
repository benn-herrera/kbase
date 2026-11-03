#### Three kinds of sheet

- `kb-root/claim-graph.svg` is the full sheet of the whole KB, always drawn: every volume a cluster
  holding its attached nodes. The sheet's title, and the label of the cluster of nodes no volume
  hosts, is the entry point's first heading, verbatim; `kb-root` where it has none.
  The KB title is the graph's label across the top. A volume's cluster is titled from its `index.md`
  first heading, falling back to the directory name, with the subtitle "N nodes · M unattached".
- `kb-root/claim-graph-digest.svg` is the volume digest: one box per volume listing its node counts
  per kind on one line, its unattached count and its within-volume edge counts, and the cross-volume
  edges as bundles whose counts cover every edge class; each box links to the volume's index and, on
  a second line reading `claim graph →`, to its sheet. The full sheet links to the digest (`volume
  digest →`) and the digest to the full sheet (`← full claim graph`).
- `kb-root/<volume>/claim-graph.svg`, one per volume, a top-level directory holding an `index.md`:
  the volume's claims plus every claim one edge away, drawing only edges with an end in the volume.

The digest and the per-volume sheets exist only where two or more volumes hold nodes; the nodes no
volume hosts are not a volume, and a volume holding no node has no sheet and no digest
box. A one-volume KB has the full sheet alone. A digest or volume sheet the KB no longer calls for
is removed by the next render, and reported ([§7](../../7-tool-result-contract-and-exit-codes/index.md)); a file named `claim-graph.svg` directly inside a
top-level directory of `kb-root/` is by definition such a sheet.

