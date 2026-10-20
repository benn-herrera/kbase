package main

func init() {
	rootCmd.AddCommand(writeOpCommand("add-rests-on",
		"add rests-on premises and references to a register entry",
		`add-rests-on appends rests-on and references bullets to one entry, below
the bullets already there. A rests-on premise defaults to a pending bearing
when the values name none, and an edge the entry already holds is not written
a second time; the result notes how many were. A refresh follows unless
--no-refresh.`))
}
