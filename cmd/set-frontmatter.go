package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-frontmatter",
		"replace or insert a document's frontmatter block",
		`set-frontmatter replaces a document's whole frontmatter block, or places a
first one, carrying refresh's roll-ups over. The values are the block's whole
authored content: a field left out is removed, and a key this op cannot
render or a hosted node the values do not restate is refused. A refresh
follows unless --no-refresh.`))
}
