package main

func init() {
	rootCmd.AddCommand(writeOpCommand("drop-claim-edges",
		"remove a claim's depends-on or references bullets",
		`drop-claim-edges removes a claim entry's outgoing edge bullets of one
class, depends-on or references, or of both when edge-class is omitted;
continuation lines go with their bullet, and a list's header goes where no
bullet is left. A class with no list is left as it stands. Demoted rows are
resolve-demoted's to remove. A refresh follows unless --no-refresh.`))
}
