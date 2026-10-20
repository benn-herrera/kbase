package main

func init() {
	rootCmd.AddCommand(writeOpCommand("drop-inbound-edges",
		"remove every entry's edge bullets naming one claim",
		`drop-inbound-edges removes each register entry's rests-on and references
bullets and each claim's refutation pair naming id, with their continuation
lines, and a list's header where no bullet is left — and, where an analysis's
or experiment's leaf authors a lift or refutation of id, that pair. An entry
naming no bullet at id is left as it stands. A refresh follows unless
--no-refresh.`))
}
