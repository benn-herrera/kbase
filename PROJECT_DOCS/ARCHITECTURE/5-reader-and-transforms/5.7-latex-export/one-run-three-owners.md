#### One run, three owners

The verb (`cmd/export-latex.go`) opens the KB through `kbload.Open`, with the in-memory format
migration of [§12](../../12-constants/format-migration-spec-10/index.md) where the KB needs it. It refuses a load error, and
each `--volume` that is not a volume of `kb.Volumes` (`notVolumes`). It hands the KB to `export.Run`
with the pandoc calls as two function values it supplies: the version check (`pandoc.Preflight`)
and the one `pandoc.RunDefaults` call. The verb takes no KB write lock and runs no git.

`export.Run` is the export's one sequence, its steps unexported so nothing drives them but it:

1. `walk` traverses the KB.
2. `selectVolumes` narrows to `--volume`. A name the entry point does not list is an
   `UnknownVolumesError`, which the verb maps to a usage refusal.
3. `assemble` builds `gen-tex/`'s inputs in memory.
4. The version check runs, so a pandoc refusal precedes any write.
5. `writeOut` writes the inputs.
6. The defaults run is `pandoc -d defaults.yaml` in `gen-tex/`, so every path in the defaults file
   is relative and the author's re-run from there resolves the same way.
7. `writeReport` writes the report.

The verb maps what `Run` returns onto the result document.

| Owner | Owns |
|---|---|
| Go (`internal/export`) | What pandoc cannot know: the navigation tree's order and ranks, the label namespaces, the KB's contract forms as data. The stream files, the tables, the defaults file, assets, the manifest |
| The filter (`kbase-export.lua`) | Every rewrite of the parsed document: references, citations, links, labels, headings, theorem blocks, figures, maths fixes, the bibliography. Constant code over the tables |
| The template (`kbase-export.latex`) | The LaTeX preamble, package order, theorem machinery, running heads, front and main matter |

Both the filter and the template are embedded (`filter.go`) and written verbatim, so their bytes do
not vary with the KB.

