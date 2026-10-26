package main

import "github.com/spf13/cobra"

func dropClaimEdgesCommand() *cobra.Command {
	return writeOpCommand("drop-claim-edges",
		"remove a claim's supports, founds or references edges",
		`drop-claim-edges removes a claim's own edge records of one class — its
premises (every supports and founds edge into it, an experiment's support
included) or its references (the references edges out of it) — the class
edge-class names, or both when edge-class is omitted. A claim holding no edge
of the class is left as it stands. Demoted records are resolve-demoted's to
remove. A refresh follows unless --no-refresh.`)
}
