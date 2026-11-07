### The multichoice-ask interface

**Pattern:** Dependency injection with an architecture fitness test.

Each inference pass reaches the model only through `internal/asks`' multichoice-ask interface,
injected by `build`; nothing in `claimgraph` knows a network exists. `cmd/importpolicy_test.go`
enforces it two ways: `claimgraph` may not reach `internal/model` or `internal/asks/call` through any
chain of imports, and only `build` imports `internal/asks/call` directly.

- **`internal/asks`** contains the embedded templates and fragments, the composer and `Wave`. Every
  claim-graph model call is one question answered by one letter from a fixed set the build offers.
  `asks.Decide` takes the offered letter with the highest confidence, the first offered winning a tie,
  or else a one-character reply that is an offered letter. A reply with no letter is asked once more
  (`resolve`, bounded by `asks.MaxAsks`), the attempt ordinal keeping the two asks apart in the answer
  cache. Where the first reply was `Refused`, the re-ask's `Correction` is the `decision-correction`
  fragment (`asks.DecisionCorrection`); otherwise the re-ask is the identical question. If the re-ask
  decides nothing either, the item takes its deterministic draft and is recorded `defaulted`. The group record's outcome for an
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
  has its `true-letter` where it has one, and a corrected re-ask's request has the `correction` text it
  sent. The overview capture's request has its `excerpts` and, on a
  retry, its `rejected-lines`, the inputs `asks.OverviewPrompt` renders. Each file is written once,
  after the item that made the call has released its permit ([§2](../2-invariants.md), I10 and I11).
- **Wave records.** `Wave.Wait` returns a record for each group. A wave that stops still returns
  its records: an item whose call never completed is recorded `failed`, one cancelled or never launched
  is absent, and so is a group with no item that made a call. The stage writes the records before it stops. A
  finished wave records every group it was given. A wave made by `asks.NewReplayWave` records a
  failed item and continues.
- **Templates.** Templates (`*.tmpl.md`) and fragments are embedded files ([§2](../2-invariants.md), I7). A
  prompt is a `@!slot!@` template, and a call's system prompt is the fragment named for it:
  `overview-system`, the one system fragment, is the overview call's. A multichoice question has no
  system message. `asks.Render` builds the one user turn. A
  template change is an experiment with a measured result.
- **Overview.** The overview text is written from excerpts composed deterministically from the tree,
  checked for structure and retried once through the `overview-correction` fragment. A call ending
  `truncated-empty` fails the stage without that retry.

