#### Template

`book`, `oneside` and `openany`, so every page is laid out alike and no blank page
precedes a chapter, with options for `hyperref`, `url`, `cleveref` and `geometry` (`margin=1in`,
on LaTeX's default letter paper) passed before the class so that an author's own load of them
does not clash and options the preamble passes them take effect; engine-branched fonts
(`iftex`); the maths packages, `amsthm` and `\kbasetheorem`; the Unicode mappings for Greek,
arrows, relations and sub- and superscript digits; the writer-conditional blocks (tables,
highlighting, strikeout and the rest, switched by pandoc's template variables); then
`\InputIfFileExists{../preamble.tex}`; then `geometry`, `microtype`, `hyperref`, `bookmark`,
`cleveref` last, then the `header-includes`. The preamble's path is relative to the working
directory, so the author compiles `main.tex` from inside `gen-tex/`; where `../preamble.tex` is
absent from the working directory, the template types a notice and loads nothing. The rest:
mixed-case running heads naming the chapter on every page; the first numbered chapter opens the
main matter, everything before it being front matter, the title page its unnumbered page i and
the contents following from ii; `\backmatter` before the bibliography; and
one generation comment naming kbase's version (`kbase-version`, set by the verb in
`defaults.yaml`'s `variables`).

