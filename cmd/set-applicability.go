package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-applicability",
		"rewrite a claim-to-work pairing's applicability",
		`set-applicability rewrites the applicability annotation on the depends-on
bullet by which a claim rests on an external work. A pairing with no bullet
is refused: the op re-scores an edge, it does not create one. A refresh
follows unless --no-refresh.`))
}
