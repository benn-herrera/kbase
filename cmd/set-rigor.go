package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-rigor",
		"rewrite a node's local rigor",
		`set-rigor rewrites one node's authored rigor — a claim's, an analysis's and a
work's register line, or an experiment's rigor in its host leaf's
experiment-nodes mapping — and no derived line. A refresh follows unless
--no-refresh.`))
}
