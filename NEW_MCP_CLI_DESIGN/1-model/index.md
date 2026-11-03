## 1. The model

A KB lives at `kb-root/` beside the repository's `.git`. It holds kbase's records under
`kb-root/.index/`, the build's records under `kb-root/.build/` ([§2.5](../2-conventions/2.5-domain-rule.md)), the sheets ([§1.6](1.6-derived-files.md)), and
Markdown documents of these classes:
- the entry point, `entry-point.md`;
- index documents, `index.md`;
- `axioms.md` ([§1.1](1.1-nodes.md));
- the three maintained-block documents and the `CLAUDE.md` redirect ([§1.7](1.7-maintained-blocks.md));
- the summary pages, `kb-summary.md` ([§1.6](1.6-derived-files.md)), derived;
- leaves: every other Markdown document under `kb-root/`.

Statement markers are read from leaves alone. The marker on a summary entry is annotation and hosts
nothing.

Sections:

- [§1.1 Nodes](1.1-nodes.md)
- [§1.2 Edges and relations](1.2-edges-and-relations.md)
- [§1.3 Demoted edges](1.3-demoted-edges.md)
- [§1.4 Statement markers](1.4-markers.md)
- [§1.5 Citations](1.5-citations.md)
- [§1.6 Derived files: summary pages and sheets](1.6-derived-files.md)
- [§1.7 Maintained-block documents](1.7-maintained-blocks.md)
- [§1.8 Format version](1.8-format-version.md)
