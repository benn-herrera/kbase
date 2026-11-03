#### One run, three owners

The verb (`cmd/export-latex.go`) opens the KB through `kbload.Open`, refuses a load error and
each `--volume` that is not a volume of `kb.Volumes` (`notVolumes`), and
hands the KB to `export.Run` with the pandoc calls as two function values it supplies: the version
preflight (`pandoc.Preflight`) and the one `pandoc.RunDefaults` call. `export.Run` is the
export's one sequence, its steps unexported so nothing drives them but it: it walks the KB
(`walk`), narrows to `--volume` (`selectVolumes`), assembles `gen-tex/`'s inputs in memory
(`assemble`), calls the preflight, writes the inputs (`writeOut`), calls the defaults run, and
writes the report (`writeReport`); the verb maps what it returns onto the result document. The
call is `pandoc -d defaults.yaml` run in `gen-tex/`, so every path in the defaults file is relative
and the author's re-run from there resolves the same way. The preflight runs before `writeOut`, so
a pandoc refusal precedes any write.

| Owner | Owns |
|---|---|
| Go (`internal/export`) | What pandoc cannot know: the spine's order and ranks, the label namespaces, the KB's contract forms as data. The stream files, the tables, the defaults file, assets, the manifest |
| The filter (`kbase-export.lua`) | Every rewrite of the parsed document: references, citations, links, labels, headings, theorem blocks, figures, maths fixes, the bibliography. Constant code over the tables |
| The template (`kbase-export.latex`) | The LaTeX preamble, package order, theorem machinery, running heads, front and main matter |

Both the filter and the template are embedded (`filter.go`) and written verbatim, so their bytes
do not vary with the KB.

