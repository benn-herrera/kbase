package main

func init() {
	rootCmd.AddCommand(writeOpCommand("set-bearing",
		"rewrite one scoring edge's bearing",
		`set-bearing rewrites the (bearing …) annotation of one scoring edge — a
rests-on premise, an analysis's or an experiment's lift or refutation, or a
claim's own refutation pair — and nothing else. A value is a number in [0, 1]
or *pending*. An edge with no bullet, a relation outside the four scoring
relations, a pairing that carries no bearing and a value outside the domain are
each refused: the op re-scores an edge, it does not create one. A refresh
follows unless --no-refresh.`))
}
