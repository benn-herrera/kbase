## 1. What kbase is

kbase is a Go binary that builds and maintains a knowledge base (KB) from LaTeX. One or more volume
roots go in, one KB comes out — its entry point listing every volume — and a set of maintenance
subcommands then operates on the living KB, at a shell or, through `kbase mcp` ([§11](11-the-mcp-server.md)), from any
external agent harness that speaks MCP. A KB can also be created empty by `init` ([§8](8-maintenance-subcommands/index.md)) and filled by
authoring with those same subcommands — no volume, no LaTeX, no build: LaTeX stays the only input
format of `build`, and `init` is the zero case of the tree, not another input.

- A **volume root** is the top `.tex` file of a paper: the file `00README.json`'s `toplevel` entry
  names, else the sole file containing `\documentclass`.
- A **KB** is a `kb-root/` directory beside a repository's `.git`: the Markdown document tree and
  its metadata layer.
- The **metadata layer** is what kbase writes into a KB: the node and edge records in `.index/`,
  the statement markers in the leaves, the format stamp in `.index/format.yaml`, and the register
  pages kbase renders from them (the format of [§10](10-metadata-format-version/index.md)).
- The **load-bearing forms** are the Markdown shapes KB readers key on: the up-link line, the
  cross-reference anchor form, the `` ``` math `` fence, the citation span, and the labelled
  blockquote.

Input is LaTeX only; no other input format is in scope.

---

