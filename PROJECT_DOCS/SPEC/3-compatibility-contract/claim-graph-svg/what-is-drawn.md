#### What is drawn

The four scoring relations — `supports`, `founds`, `lifts` and `refutes` — each in
a stroke of its own, every stroke pointing in the push direction, from the supporter to the node it
supports. `references` edges are not drawn; a `demoted` record ([§10](../../10-metadata-format-version/index.md)) is drawn dotted red, as the
edge it replaced. A node with no drawn edge is not drawn as a node; it is listed in a table titled
"unattached (N)", one row per node with a link and a tooltip. Of the drawn `supports` edges, only
the transitive reduction appears; it checks an edge only against the edges still kept.

- *Edges*: each relation is drawn in one style of its own — `supports` black solid, and `founds`,
  `lifts` and `refutes` each a stroke beside it — whatever the build knew of how an edge was found:
  the sheet reads no build record. A `demoted` record is drawn dotted red in the direction of the
  edge it replaced; its tooltip calls it `cut`, the overview diagram counts such records beside the
  other classes (`1 cut`) and draws a bundle of them, and its legend row reads "cut from supports:
  part of a circle".
- *Nodes*, seven shapes: a claim's three — a labelled block (bold frame), an equation and prose —
  and an analysis (dashed rounded), an experiment (component shape), an axiom (grey, bold frame)
  and a work (note). An edge end that no record has is drawn as a ghost box, dashed red, labelled
  with the id. Claims and analyses are filled by strength band, `*pending*` in grey. A node's
  tooltip reads `<id> [<kind>, <band>] <title>`, the band omitted for a node that has none, every
  backslash in it shown as written (`$\Gamma$` does not become the graph's name).
- *Links*: every node links to its canonical place ([§10](../../10-metadata-format-version/canonical-path-and-canonical-anchor.md)) —
  a claim's or a work's entry on its summary page, an analysis's or an experiment's section in the
  leaf hosting it, an axiom's heading in `axioms.md` — and a claim with no host links nowhere; an
  overview-diagram box links to the volume's index and sheet.
- A legend. The SVG root is `width="100%"` with a `viewBox`, so it scales to the viewer.

**Without `dot` on `PATH`**, no sheet is rendered: an existing sheet is left as it is, and where
`kb-root/claim-graph.svg` does not exist the render writes a placeholder there — an SVG naming the
missing tool and, beneath it, `index sha256:` followed by the first 12 hex digits of the SHA-256 of
the `.index/` files — the two record files — concatenated in sorted path order, deterministic.
The command says so in one line on stderr, and its result names the lack ([§7](../../7-tool-result-contract-and-exit-codes/index.md)).

---

