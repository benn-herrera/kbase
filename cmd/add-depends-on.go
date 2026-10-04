package main

func init() {
	rootCmd.AddCommand(writeOpCommand("add-depends-on",
		"add outgoing edges to a register entry",
		`add-depends-on appends depends-on and references bullets to one entry, below
the bullets already there. An edge the entry already holds is not written a
second time, and the result notes how many were. A refresh follows unless
--no-refresh.`))
}
