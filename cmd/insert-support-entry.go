package main

func init() {
	rootCmd.AddCommand(writeOpCommand("insert-support-entry",
		"mint a support id and write its register entry",
		`insert-support-entry mints a sup- id and writes its canonical register entry,
staging its beneficiary fan-out there until a document hosts it, and reports
the id. An entry already in the register with the same title and values is
adopted rather than written twice. The register is created only under
--create. A refresh follows unless --no-refresh.`))
}
