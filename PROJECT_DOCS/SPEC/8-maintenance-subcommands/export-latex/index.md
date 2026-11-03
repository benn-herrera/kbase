### `export-latex`

```
kbase export-latex [--out DIR] [--volume NAME]...
```

Writes a LaTeX project from the KB's current state into `--out` (default `latex-export/` beside
`kb-root/`; a relative `--out` resolves against the project root ([§9](../../9-configuration/index.md))). Every KB file is read
through the one loader ([§10](../../10-metadata-format-version/index.md)'s stamp rules apply); nothing under `kb-root/` is written; a KB whose
load fails is refused with the load error ([page §3.6](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md)). `--volume`
is repeatable, each naming a volume as [page §1.6](../../../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md) defines one;
the named volumes export in entry-point order. A `--volume` that is not a volume is a usage
refusal, `allowed` every volume, and so is one naming a volume the entry point's list does not
name, which the export does not reach ([§7](../../7-tool-result-contract-and-exit-codes/index.md)). The out directory may be any directory, the author's own
LaTeX project included: kbase owns only its `gen-tex/` subdirectory and writes nothing else there
but a missing `preamble.tex`. **Manifest refusal.** A `gen-tex/` that exists holding files without
the export manifest `gen-tex/kbase-export.yaml` is refused whole, nothing written — `check:
export-dir`, remedy: empty the directory or name another `--out`.

Contents:

- [§8 The project](the-project.md)
- [§8 Content](content.md)
