### The multichoice-ask interface

**Pattern:** Dependency injection with an architecture fitness test.

Each inference pass reaches the model only through `internal/asks`' multichoice-ask interface,
injected by `build`; nothing in `claimgraph` knows a network exists. `cmd/importpolicy_test.go`
enforces it two ways: `claimgraph` may not reach `internal/model` or `internal/asks/call` through any
chain of imports, and only `build` imports `internal/asks/call` directly.

- **`internal/asks`** contains the embedded templates and fragments, the composer and `Batch`. Every
  claim-graph model call is one question answered by one letter from a fixed set the build offers.
  `asks.Decide` takes the offered letter with the highest confidence, the first offered winning a tie,
  or else a one-character reply that is an offered letter. A reply with no letter is retried once
  with the identical question, the attempt ordinal keeping the two apart in the answer cache, then the
  item takes its deterministic draft and is recorded `defaulted`. The group record's outcome for an
  item is one of `answered`, `re-asked` (the retry gave the letter), `defaulted` and `failed` (its
  call never completed); these are record values (`asks.Outcome`). `AskGroup` launches the first item
  of a group of more than one alone and waits for it, so the server prefills and caches the group's
  shared prefix once, then launches the rest.
- **Compose.** Each kind composes its state through `MultichoiceItem.Compose`, its kind's template
  rendered from the item's template inputs. `asks.RecordedItems` builds items from a group record
  through the same constructor.
- **`internal/asks/call`** contains the caller, imported by `build` alone: `Caller.Launch` puts each
  question to the decision role's model through the item's slot, and `PassageLaunch` makes the overview
  call through `Slot.Chat`. It writes the per-call captures and the answer cache (see
  [the answer cache](../8-ledger-state-store-and-monitoring/the-answer-cache.md)). A chat capture's request record has the effective
  `temperature` and `thinking`, the values `model.SamplingOf` reports. A decision capture's question
  has its `true-letter` where it has one. The overview capture's request has its `excerpts` and, on a
  retry, its `rejected-lines`, the inputs `asks.OverviewPrompt` renders. Each file is written once,
  after the item that made the call has released its permit ([§2](../2-invariants.md), I10 and I11).
- **Batch records.** `Batch.Wait` returns a record for each group. A batch that stops still returns
  its records: an item whose call never completed is recorded `failed`, one cancelled or never launched
  is absent, and so is a group with no item that made a call. The stage writes the records before it stops. A
  finished batch records every group it was given. A batch made by `asks.NewReplayBatch` records a
  failed item and continues.
- **Templates.** Templates (`*.tmpl.md`) and fragments are embedded files ([§2](../2-invariants.md), I7). A
  prompt is a `@!slot!@` template, and a call's system prompt is the fragment named for it:
  `overview-system`, the one system fragment, is the overview call's. A multichoice question has no
  system message. `asks.Render` builds the one user turn. Tests key on the offered letters, the slot
  set, the system fragment and the closing line's letters, and none asserts a template's prose. A
  template change is an experiment with a measured result.
- **Overview.** The overview text is written from excerpts composed deterministically from the tree,
  checked for structure and retried once through the `overview-correction` fragment. A call ending
  `truncated-empty` fails the stage without that retry.

