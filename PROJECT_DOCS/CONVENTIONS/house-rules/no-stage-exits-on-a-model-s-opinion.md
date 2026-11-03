### No stage exits on a model's opinion

Every stage exits on a comparison of artifacts or a
return code; a new stage does not get a third kind of exit.
- **A model call is made through a `Slot` from the invocation's queue, inside the item `Go`
  launches; the item writes files only in its `after`, once its token is released.** The trap: a
  goroutine pool or semaphore beside the queue, which re-introduces a second bound; a `WithTimeout`
  around `Go`, which times an item out for its wait; groups launched from per-group goroutines,
  which loses submission order; or a common file — a ledger, a record, a progress line — written
  from an item while its ask batch is in flight (ARCHITECTURE I11).
- **No prompt reaches a model from an in-code string.** Every prompt is an embedded template file
  under `internal/asks`, rendered with its slots filled, and its prose lives in that file alone —
  never in a code string, a test, or a second copy elsewhere. A new or changed template gets
  prompt-engineer review before first use, and a template change lands as an experiment with a
  measured result. A call's system prompt is the fragment named for it, never an agent definition.
