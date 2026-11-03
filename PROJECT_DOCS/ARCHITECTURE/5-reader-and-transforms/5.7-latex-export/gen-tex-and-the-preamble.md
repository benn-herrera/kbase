#### `gen-tex/` and the preamble

`writeOut` owns `gen-tex/`. A `gen-tex/` holding files and no
`kbase-export.yaml` is an `ExportDirError` (the `export-dir` refusal) and nothing is written. The
manifest lists every file the export owns — the assembled files plus `main.tex`,
`kbase-export-findings.json` and the report — and is written after the files it names. A file
whose bytes are already there is not rewritten; a file the previous manifest listed and this
export does not own is removed, with the directories the removal empties; each manifest path is
checked to lie inside `gen-tex/` before any removal. The starter `preamble.tex` is written beside
`gen-tex/` only where nothing stands at that name and is never opened afterwards; the report
checks its existence only. These are the only writes outside `gen-tex/`.

