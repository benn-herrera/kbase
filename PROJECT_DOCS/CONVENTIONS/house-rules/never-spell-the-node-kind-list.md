### Never spell the node-kind list

Single source of truth: the list is defined in `internal/formats/format40`, and named
everywhere else through `kb`. A site that *iterates* — a count report, a breakdown, a union, an
ordering — reads the list and names no kind. A site that *branches* names kinds, and its table
must be total over the list. A count derived from the list ("all N kinds") is the same copy in
a shorter form. Nor a node id's prefix or a minted id's grammar: outside `internal/formats`, code
composes and matches ids through `kb.ClaimPrefix`, `kb.AnalysisPrefix`, `kb.ExperimentPrefix`,
`kb.WorkPrefix` and `kb.AxiomPrefix`, and the minted grammar, prefix included, through
`kb.IDBody`. `cmd/idprefixpolicy_test.go` fails an id prefix spelled outside `internal/formats`.
A literal id in a test fixture or in help prose is allowed.
