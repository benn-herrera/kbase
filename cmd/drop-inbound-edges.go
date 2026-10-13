package main

func init() {
	rootCmd.AddCommand(writeOpCommand("drop-inbound-edges",
		"remove every entry's edge bullets naming one claim",
		`drop-inbound-edges removes each register entry's depends-on and
references bullets naming id, with their continuation lines, and a list's
header where no bullet is left. An entry naming no bullet at id is left as
it stands. A refresh follows unless --no-refresh.`))
}
