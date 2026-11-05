### Retry and attempt outcomes

**Pattern:** Retry with backoff on retryable status.

`chat` repeats a request identically while it does not complete, up to `model.ChatAttempts`. The
caller's `ChatRequest.Backoff` gives the pause before each repeat, its last entry repeating. The overview
call (`overviewBackoff`) and `kbase score`'s calls (`scoreBackoff`) pass a backoff; the build's
multichoice questions and `probe` pass none and repeat at once. The only recorded reason for the
multichoice questions' immediate repeat was parity with the tool kbase was ported from, which no longer
applies; `scoreBackoff`'s reason — a 429 or 5xx repeated at once is three attempts inside one outage —
applies to them equally. A pause the call's deadline cannot cover ends the call at
once, and its reason names the attempts it made (`gaveUp`). Values are in [§12](../12-constants/index.md).

`retryableStatus`, a 429 or a 5xx, is the only non-2xx status repeated; any other ends the call on the
attempt that met it. So does an answer that reaches the completion cap before any answer text:
repeating the request identically reaches the cap the same way. `Slot.Decision` has its own loop with the same
count and the same status rule, no pause, no streaming and no `ErrChatIncomplete`.

Each attempt ends in one outcome constant: `ChatOK` (`ok`), `ChatServedNothing`,
`ChatTruncatedEmpty`, `ChatProviderRefused` (a non-2xx status) or `ChatTransportFailure` (no answer
from the provider). An attempt records whether the call reasoned, from a reasoning delta on the stream
or a usage block reporting reasoning tokens, so a call that thought without streaming the thought
counts. The error a failed call returns wraps `ErrChatIncomplete`, has the last attempt's outcome
and text, and wraps that attempt's `StatusError` where it met one. That text is what a failed item's
reason records (SPEC §8 A provider call's failure names what ended it).

