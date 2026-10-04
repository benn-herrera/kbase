package main

func init() {
	rootCmd.AddCommand(writeOpCommand("insert-claim-entry",
		"mint a claim id and write its register entry",
		`insert-claim-entry mints a clm- id and writes its canonical register entry in
one act, and reports the id. An entry already in the register with the same
title and values is adopted rather than written twice. The register is
created only under --create. A refresh follows unless --no-refresh.`))
}
