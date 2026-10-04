package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-rationale",
		"rewrite a register entry's rationale",
		`set-rationale rewrites one claim's, support's or external work's rationale,
over the whole span the reader folds into it. A refresh follows unless
--no-refresh.`))
}
