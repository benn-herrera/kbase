### A model call runs inside its queue item

A model call is made through a `Slot` from the invocation's queue, inside the item `Go` launches;
the item writes files only in its `after`, once its permit is released (ARCHITECTURE I11, I12).
Pitfalls:

- A goroutine pool or semaphore beside the queue, which re-introduces a second bound.
- A `WithTimeout` around `Go`, which times an item out for its wait.
- Groups launched from per-group goroutines, which loses submission order.
- A common file — a ledger, a record, a progress line — written from an item while its wave
  is in flight.
