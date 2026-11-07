#### The parse resolves the numbered lines

**Pattern:** Structured-output validation against a fixed grammar.

`assemble` returns the item text and one `line` per line of it: the number the call writes, the section
its bytes lie in, and its byte range. A charge's `passage`, `lines` and `offset` are facts about that
rendering. A `rigor` call's two sections are a node's `title` and `statement`; a `bearing` call's are the
`source`'s and the `target`'s. `score.ParseAnswer` (`internal/score/answer.go`) reads a reply against
those lines and the item's class's `kb.FaultTable`. A charge line's field is one number or a range
`N-M`, which `closedText.look` resolves; a range that crosses sections, a blank line, a label or a
heading is `malformed`. The recorded `citation` is the cited lines as the call sent them, without the
gutter, joined by newlines, and `offset` is that quote's byte offset in the item text. The edge's
`## Context` has no number and is a site for no charge, so the `passage` value `both` is never produced.
Grammar, duplicates and overlaps: SPEC §8 The answer.

A refusal is a `malformedError`: its text is the reason the ledger line records, `the reply is no
answer: ` and the case, and it carries the case as an `asks.Refusal…` constant, the slot values that
case's fragment names (the reply line numbers, the id and the cited numbers, each as the reply wrote it)
and the 1-based numbers of the reply lines the case names (none for an empty reply or for `none` beside
charges), which the runner quotes in a re-ask's correction. A reason names the case, the id and the line
numbers, never a field's text, and no other text of the reply. The reason is the ledger's alone: the
re-ask states the case in its fragment's words ([the prompt shelf](the-prompt-shelf.md)).

The three weight tables are `internal/kb`'s: `kb.RigorStatement`, `kb.RigorMethod` and
`kb.BearingFaults`, each a `kb.FaultTable` of ids and per-occurrence weights in integer hundredths, with
`IDs`, `Accepts` and `Price`. The parse yields `Answer{Faults, Moot, Value}`: the charges, `Moot` (charge
lines recorded beside a `flt-pending`, which no table holds; `ParseAnswer` produces none, and a recorded
line may carry them), and the `Price` of the ids. `Price` of a recorded line carrying `flt-pending` is
`*pending*`. The value is defined in SPEC §8 The value. The ids a line records are the whole input of the
value, which makes a written value checkable and a run recomputable without a model.

The grammar is fixed for as long as the ledger's format version is unchanged: a rule loosened later
would re-read lines already written under a new grammar (SPEC §8 The answer).

