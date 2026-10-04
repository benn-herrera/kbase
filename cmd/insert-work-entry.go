package main

func init() {
	rootCmd.AddCommand(writeOpCommand("insert-work-entry",
		"write an external work's register entry",
		`insert-work-entry writes the register entry of a work the corpus cites but
does not contain. Its id is derived from the citation key; a key already
entered with the same values is adopted, and with other values refused. The
register is created only under --create. A refresh follows unless
--no-refresh.`))
}
