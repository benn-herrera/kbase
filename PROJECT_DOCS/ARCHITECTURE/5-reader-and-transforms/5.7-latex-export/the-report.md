#### The report

`kbase-export-report.yaml` is one YAML list: the traversal's findings (`export.Report`: a target
not walked, a path named twice, a label collision), then the filter's, then, where `preamble.tex`
exists, a line naming it as the author's. `writeReport` reads the filter's findings back from
`kbase-export-findings.json`, so the report exists only after pandoc has run.

