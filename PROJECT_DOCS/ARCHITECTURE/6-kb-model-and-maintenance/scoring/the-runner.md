#### The runner

`score.Runner` owns the loop and no I/O of its own. It takes the enumerated items, a
`Launch` seam, which hands each item the `Ask` its one call goes through, a `Write` seam, the ledger, the model name the lines record and a
logger — and, because the text is composed in that package, the `kb.Source` and `query.Index` the
renderers read, and the `graph` they read built once from them (`graphOf`), which `Invoke` hands
over where it built it for the scope. The pass is read once before the walk into its `class`es; per item one derivation of
the item's class yields the instruction its one call carries and the table its charges are priced
under, and the composer, the parse's closed set and the price all read that one fact. Per item it
composes the closed text, asks, parses the reply as the charges it records against that text, prices
them into the item's result cell and writes its result file; when the walk ends the run lands the result cells' values and appends their captures and lines; a text the guard refuses, a reply the parse refuses, or a call or
write that fails appends `failed` with nothing written, and the loop continues. `prepare` returns,
beside the entry, the `call` the item will make — class, closed text and prompt, nil where the class cannot be derived or the guard refuses the text — and `attempt` returns the entry and whether the walk's stop interrupted the call;
the walk's end writes every capture before every line (below). Every item's prompt
is composed in the walk's launch loop, which launches the items in the set's order through the run's
`Launch` seam, at most N at once; each answer is held in its item's result cell and written in the
set's order when the walk ends. `Counts` is computed from the ledger's last status
per item, not this invocation's attempts, so a resumed run reports the whole run. A cancelled context
stops the walk and records nothing for an item without an answer.
