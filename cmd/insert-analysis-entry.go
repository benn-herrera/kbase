package main

func init() {
	rootCmd.AddCommand(writeOpCommand("insert-analysis-entry",
		"mint an analysis id and write its register entry",
		`insert-analysis-entry mints an ana- id and writes its canonical register entry,
staging its lifts and refutations there until a document hosts it, and reports
the id. An entry already in the register with the same title and values is
adopted rather than written twice. The register is created only under
--create. A refresh follows unless --no-refresh.`))
}
