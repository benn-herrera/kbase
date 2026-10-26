package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func resolveDemotedCommand() *cobra.Command {
	return writeOpCommand(write.ResolveDemoted,
		"remove a demoted edge, or restore the edge it replaced",
		`resolve-demoted resolves the demoted record of the edge from target into id —
the record the build writes in place of a scoring edge it cut to break a cycle.
remove deletes the record, leaving the edge cut; restore puts back the edge it
replaced, bearing included, refused where that would close a cycle (the refusal
names the path) and where no demoted record or standing edge joins the two. An
edge already resolved as asked is left as it stands. A refresh follows unless
--no-refresh.`)
}
