#### The runner

**Pattern:** Bounded worker pool over a job list, results written in one batch at the end.

`score.Runner` owns the item loop and no I/O of its own. It takes the enumerated items; the `Launch`
interface, which hands each item the `Ask` its one model call goes through; the `Write` interface; the
ledger; the model name the ledger lines record; and a logger. Because the item text is composed in this
package, it also takes the `kb.Source` and `query.Index` the renderers read, and the `graph` they
read, built once from them (`graphOf`); `Invoke` passes in the `graph` it built for the scope.

The pass is read once, before the loop, into its classes. Per item, one lookup of the item's class
yields both the instruction its call sends and the table its value is computed under, so the
composer, the parse's fixed set and the value computation all read that one fact. The launch loop
composes each item's prompt and launches the items in the set's order, at most as many at once as the
invocation's `model.Queue` has permits. Each launched item makes its call, parses the reply as the charges
it records against that item text, computes the value, keeps it in its own result cell and writes its
result file.
A text the input guard refuses, a reply the parse refuses, and a failed call or write each append
`failed` with nothing written, and the loop continues.

`prepare` returns the entry and the `call` the item will make: class, item text and prompt, or nil where
the class cannot be derived or the input guard refuses the text. `attempt` returns the entry and whether
the loop's stop interrupted the call. Nothing shared is written until the pool has drained (invariant
I11, [§2](../../2-invariants.md)); the ledger's `flush` then writes the cells' values, appends their captures,
then appends their lines, in the set's order. Writing at the loop's end rather than per item follows
from I11: with calls in flight concurrently, per-item writes would land in completion order, while
writing in the set's order makes the concurrency setting change wall time only. `Counts` comes from the ledger's last status per item, not from this
invocation's attempts, so a resumed run reports the whole run. A cancelled context stops the loop and
records nothing for an item without an answer; an interruption outranks a failure in the run's outcome.

