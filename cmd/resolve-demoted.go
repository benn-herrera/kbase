package main

import "kbase/internal/write"

func init() {
	rootCmd.AddCommand(writeOpCommand(write.ResolveDemoted,
		"remove a demoted edge, or restore it to depends-on",
		`resolve-demoted resolves a demoted edge, one the build's cycle breaking cut
from depends-on: remove deletes it; restore rewrites it as a depends-on edge,
refused where that would close a cycle, the refusal naming the cycle's path.
A pair already resolved as asked is left as it stands. A refresh follows
unless --no-refresh.`))
}
