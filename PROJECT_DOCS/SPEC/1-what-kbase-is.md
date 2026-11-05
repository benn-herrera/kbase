## 1. What kbase is

kbase is a program that builds, maintains and scores a knowledge base (KB) from LaTeX papers.
`kbase build` reads one or more papers, each given as its top `.tex` file, through pandoc ([§2](2-given-interfaces/index.md)) and
writes one KB: a directory of Markdown files that describes the papers' structure and the argument
they make, as a graph of claims ([§10](10-metadata-format-version/index.md)). The build derives its draft from the LaTeX deterministically,
and model calls only refine it ([§5](5-build-behaviour/index.md)). Maintenance subcommands then edit the KB through validated,
all-or-nothing batches of writes, each followed by a rebuild of the derived views ([§8](8-maintenance-subcommands/index.md)), and query,
check and score it. `kbase score` has a model judge each node's rigor and each scoring edge's
bearing, and the strength of every claim and analysis is derived from those values ([§10](10-metadata-format-version/index.md), the
scoring schemes). The subcommands run at a shell or, through `kbase mcp` ([§11](11-the-mcp-server/index.md)), from any external
agent harness that speaks MCP. A KB can also be created empty by `init` ([§8](8-maintenance-subcommands/index.md)) and filled by
authoring with those same subcommands, with no LaTeX and no build; LaTeX stays the only input
format of `build`.

- A paper's **top `.tex` file** is the file `00README.json`'s `toplevel` entry names, else the sole
  file containing `\documentclass`.
- A **KB** is a `kb-root/` directory beside a repository's `.git`: the Markdown document tree and
  its metadata layer.
- The **metadata layer** is what kbase writes into a KB: the node and edge records in `.index/`,
  the statement markers in the leaves, the format version in `.index/format.yaml`, and the summary
  pages kbase renders from them (the format of [§10](10-metadata-format-version/index.md)).
- The **load-bearing forms** are the Markdown shapes KB readers key on: the up-link line, the
  cross-reference anchor form, the `` ``` math `` fence, the citation span, and the labelled
  blockquote.

Input is LaTeX only; no other input format is in scope.

---

