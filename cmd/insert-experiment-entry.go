package main

func init() {
	rootCmd.AddCommand(writeOpCommand("insert-experiment-entry",
		"mint an experiment id and declare it in its hosting document",
		`insert-experiment-entry mints an exp- id and appends its declaration to the
frontmatter block of the document that hosts it, and reports the id. An
experiment the document already hosts with the same values is adopted. The
document must exist with a frontmatter block. A refresh follows unless
--no-refresh.`))
}
