package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-on-point-fraction",
		"rewrite a support-to-claim on-point fraction",
		`set-on-point-fraction rewrites one supports pair, in the hosting document's
frontmatter where one hosts the support and in the register entry's staging
block otherwise. A pair authored in neither home is refused. A refresh
follows unless --no-refresh.`))
}
