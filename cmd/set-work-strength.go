package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-work-strength",
		"rewrite an external work's standing",
		`set-work-strength rewrites one external work's strength line and nothing
else. A refresh follows unless --no-refresh.`))
}
