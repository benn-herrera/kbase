#### Tables

`kbase-xref.json` is generated from the traversal, each document's first heading and the
bibliography sources, and named to the filter by the metadata key `kbase-xref`. The field-level
schema is the head comment of `kbase-export.lua`; the keys:

- `documents`: heading id → path, volume `N`, rank;
- `paths`: kb-root-relative path → heading id (href resolution);
- `volumes`: in order, each with its `vol{N}` label and title;
- `theorems`: every display name `kb.LabelLine` reads off a label line of the front matter or an
  exported document, so the label-name grammar stays in Go;
- `bibliography`: citation key → entry body as gfm text. Each exported volume's `references.md`
  entries (`kb.BibliographyEntries`) and, where any exported volume has none, every work record (a
  work's title); the first entry of a key is kept;
- `report`: the path the filter writes its findings to;
- `assets`: kb-root-relative path of a copied file → its path under the out directory; the
  claim-graph sheets (SPEC §3 `claim-graph.svg`) are not copied, being drawings of the metadata;
- `entry_point`: the entry point's path, the front matter's source;
- `streams`: the stream file names in reading order.

