### Never spell the node-kind list

It is defined once, in `internal/formats/format400`, and named
everywhere else through `kb`. A site that *iterates* — a census, a breakdown, a union, an
ordering — reads the list and names no kind. A site that *branches* names kinds, and its table
must be total over the list. A count derived from the list ("all four kinds") is the same copy in
a shorter form. Nor a node id's prefix or a minted id's grammar: outside `internal/formats`, code
composes and matches ids through `kb.ClaimPrefix`, `kb.AnalysisPrefix`, `kb.ExperimentPrefix`,
`kb.WorkPrefix` and `kb.AxiomPrefix`, and the minted grammar, prefix included, through
`kb.IDBody`. A literal id in a test fixture or in help prose is not a spelling.
