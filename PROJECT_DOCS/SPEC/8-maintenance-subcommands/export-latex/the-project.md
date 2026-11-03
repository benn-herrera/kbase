#### The project

`gen-tex/` is kbase's and every export regenerates it: `main.tex`, the whole
export; `defaults.yaml`, the pandoc defaults file that produced it, its paths relative to itself;
`kbase-export-front.md`, the entry point's prose, and `kbase-export-vol{N}.md`, one per exported
volume in order, its documents in walk order — read in that order, each file's footnotes its own;
`kbase-export.latex`, the template;
`kbase-export.lua` and `kbase-xref.json`; `assets/`, a copy of every non-Markdown file under each
exported volume's directory less the claim-graph sheets ([§3](../../3-compatibility-contract/index.md)), at its `kb-root/`-relative path;
`kbase-export-findings.json`, the
filter's findings, which the export merges into the report; the report and the manifest. None of it
is for editing. **The preamble is the author's.** `main.tex` loads `preamble.tex` from the out
directory, beside `gen-tex/`, after the template's base packages and before its cross-reference
packages. Where no `preamble.tex` stands, the export writes a starter — commented, loading nothing,
advising the author to add the packages and macros their sources used; where one stands, the
export never reads, writes or removes it. Whenever a `preamble.tex` exists, the report carries a
line naming it as the author's. A built KB keeps its maths as the
sources wrote it, so a macro or package those sources' preambles supplied is the author's to add
there; LaTeX names any that is missing. The author may re-run `pandoc -d defaults.yaml` from
`gen-tex/` without kbase. Compiling `gen-tex/main.tex` is the author's step; kbase runs no TeX
engine. How the project is produced is ARCHITECTURE §5.7.

