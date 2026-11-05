#### Idempotence

Assembly is deterministic in the KB and the kbase version, so a repeat export reproduces every
assembled file byte for byte and `writeOut` writes none. pandoc rewrites `main.tex` and the
findings each run, so `export.Run` compares their bytes before and after the defaults run; the
outcome is `unchanged` when nothing was written, removed or rewritten with different bytes, the
report included.

