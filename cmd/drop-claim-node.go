package main

func init() {
	rootCmd.AddCommand(writeOpCommand("drop-claim-node",
		"delete a claim's register entry, marker and frontmatter claims entry",
		`drop-claim-node deletes a claim node entirely: its register entry
block, its claim-rigor marker in every leaf claiming it, and its id from
each such leaf's claims list. A leaf whose claims list would empty is
refused — an empty claims list is a no-claim decision set-frontmatter makes.
An entry still naming the claim in its edges is refused: run
drop-claim-edges and drop-inbound-edges first. A refresh follows unless
--no-refresh.`))
}
