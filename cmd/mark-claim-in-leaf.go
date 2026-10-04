package main

func init() {
	rootCmd.AddCommand(writeOpCommand("mark-claim-in-leaf",
		"place a claim's marker at a located line of a document",
		`mark-claim-in-leaf appends a claim's claim-quality marker to the one body line
its locator names. The claim must be in the document's own claims list; a
locator that names no line or several is refused. A refresh follows unless
--no-refresh.`))
}
