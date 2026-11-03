#### Idempotence

Assembly is deterministic in the KB and the kbase version, so a re-issue
reproduces every assembled file byte for byte and `writeOut` writes none. pandoc rewrites
`main.tex` and the findings each run, so the verb compares their bytes before and after; the
outcome is `unchanged` when nothing was written, removed or rewritten with different bytes,
the report included.

