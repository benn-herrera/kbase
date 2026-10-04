package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-rigor",
		"rewrite a claim's or support's local rigor",
		`set-rigor rewrites one entry's authored rigor — confidence on a claim,
quality on a support, read off the id — and no derived line. A refresh
follows unless --no-refresh.`))
}
