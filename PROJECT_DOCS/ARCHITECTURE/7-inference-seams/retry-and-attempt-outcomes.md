### Retry and attempt outcomes

**Pattern:** Capped exponential backoff with full jitter, honouring `Retry-After`.

One rule governs every model call, `chat` and `decision` alike, and `internal/model` owns it: callers
pass no backoff. The rule is a `model.Retry` on the `model.Queue` the call's slot came from —
`Attempts`, `Base` and `Draw`, the random source — which `NewQueue` sets to `DefaultRetry()` and a
slot copies as `Queue.Go` issues it. A request that did not get through — a transport error, a 429 or
5xx, an incomplete stream, an undecodable body — is repeated identically, up to `Attempts`, the pause
before attempt n drawn uniformly from [0, `Base` × 2ⁿ⁻²] (`Retry.pause`): a 429 or 5xx repeated at once
is every attempt inside one outage, and the jitter keeps concurrent calls from re-issuing together. A
429 or 503 whose `Retry-After` reads as delay-seconds or an HTTP-date pauses that long instead; the
header travels on `StatusError.RetryAfter`. A pause the call's deadline cannot cover ends the call at
once (`Retry.before`), and its reason names the attempts it made and, where `Retry-After` set the
pause, the delay the server asked for (`gaveUp` in `chat`; the decision loop's error likewise). Values
are in [§12](../12-constants/index.md). A test sets the rule on its own queue; no shipped switch exists
for it.

A retry that changes the request because the answer was flawed — refused, malformed, failing its
check, a decision question left unanswered — is a new call the caller makes, not a repeat inside the
loop, and goes out at once.

`retryableStatus`, a 429 or a 5xx, is the only non-2xx status repeated; any other ends the call on the
attempt that met it. So does an answer that reaches the completion cap before any answer text:
repeating the request identically reaches the cap the same way. The decision loop
(`OpenAIClient.decision`, handed the slot's rule by `Slot.Decision`) has the same count, pauses and
status rule, with no streaming and no `ErrChatIncomplete`.

Each attempt ends in one outcome constant: `ChatOK` (`ok`), `ChatServedNothing`,
`ChatTruncatedEmpty`, `ChatProviderRefused` (a non-2xx status) or `ChatTransportFailure` (no answer
from the provider). An attempt records whether the call reasoned, from a reasoning delta on the stream
or a usage block reporting reasoning tokens, so a call that thought without streaming the thought
counts. The error a failed call returns wraps `ErrChatIncomplete`, has the last attempt's outcome
and text, and wraps that attempt's `StatusError` where it met one. That text is what a failed item's
reason records (SPEC §8 A provider call's failure names what ended it).

