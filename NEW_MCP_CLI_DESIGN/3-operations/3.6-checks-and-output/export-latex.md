#### `export-latex` (tool + CLI)

This writes a LaTeX project from the KB through one pandoc run, synchronously.
- It writes into `--out` (default `latex-export/` beside `kb-root/`). kbase owns `gen-tex/` there
  and regenerates it on every export. It takes no lock, runs no git, and writes nothing under
  `kb-root/`.
- Where `--out` holds no `preamble.tex`, a commented starter is written. One that stands is never
  touched.
- The report lists every form the export simplified, and why.
- Compiling `main.tex` is the author's step.
- A re-issue over an unchanged KB writes nothing and reports `unchanged`.

- **Arguments**: `out`, `volume` (repeatable; default every volume).
- **Result**: `written` (absolute), `volumes`, `report`.
- **Refusals**:
  - a `volume` that is not a volume ([§1.6](../../1-model/1.6-derived-files.md));
  - a `gen-tex/` holding files and no export manifest;
  - pandoc missing or failing;
  - a load error.
