package main

func init() {
	rootCmd.AddCommand(writeOpCommand("render-citation",
		"print the sanctioned authority-citation string",
		`render-citation reads and writes nothing in the KB beyond reading the cited
document: it checks the excerpt appears verbatim in the section the anchor
names and returns the citation, its link spelled relative to the citing
document, one per entry.`))
}
