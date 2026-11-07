#### The prompt shelf

**Novel.** The fault criteria the templates carry realize THESIS item 2 (local judgments from a closed
list of faults), defined by the rigor and bearing scoring scheme and SPEC §8 The answer.

**Pattern:** Prompt templates with a lint test.

Three templates, one per class: `score-rigor-statement.tmpl.md`, `score-rigor-method.tmpl.md` and
`score-bearing.tmpl.md`. Each has one `dyn.` slot, which the runner fills with the assembled item text,
and splices four shared fragments (`fragmentSlots`): `fragments/score-system.tmpl.md`, the system
prompt; `score-disposition`, the class-neutral standard every score template judges by, which names
no id; and `score-answer-occurrences` and `score-answer-close`, the answer section's text, which all
three state alike so no vocabulary can drift from the parse or from its siblings. `asks.ScoreRigorStatement`,
`asks.ScoreRigorMethod` and `asks.ScoreBearing` render them through `asks.Render`; `asks.System`
serves the system fragments alone (`systemSlots`), never an answer fragment as a prompt. The templates
are embedded in the binary, written by kbase, with no upstream origin and no version pin.

Each template ends in four alternative slots, right after `score-answer-close` and in this order:
`@!correction-head!@` (`""` or `score-correction-head`), `@!refusal-case!@` (`""` or one
`score-refusal-<case>`), `@!correction-quote!@` (`""` or `score-correction-quote`) and
`@!correction-close!@` (`""` or `score-correction-close`). Composition expands one level, so the
correction is four sibling slots rather than one fragment holding the other three. The refusal cases
are the exported constants `asks.Refusal…`, one per refusal site of the parse, registered in the order
`refusalCases` lists them, and case `<case>`'s fragment is `fragments/score-refusal-<case>.tmpl.md`. The three
render functions take the item text and an `*asks.Correction{Case, Values, Lines}`: `nil` fills every
correction slot with `""`, which renders the first ask; a correction fills head, its case's fragment and
close, and the quote only where `Lines` is non-empty (the lines joined by newlines, inside a
twelve-backtick fence, as `dyn.correction-lines`). `Values` fills the `dyn.` slots the case's fragment
declares, keyed by the bare slot name (`asks.SlotReplyLine`, `SlotOtherReplyLine`, `SlotID`,
`SlotCitedFrom`, `SlotCitedTo`): numbers or a fault id, nothing else. The correction's prose is the
fragments'; Go supplies only those values and the quoted lines. `asks.ScoreInstructionSet` renders a
class's every fixed byte — the first ask, then each case unquoted and quoted, every `dyn.` value
empty — and is the one definition the run's `ask_sha256` and its test read.

The templates carry the criteria and the line form. They name no id the class does not accept, and no
figure or word a model could read as a judgement: no decimal in `[0,1]`, no percentage, no weight, no
maximum, no band's word. This is a deliberate blinding: the weights are `internal/kb`'s, the single
source of truth for them. A quantity of the subject's own, an example's rate or threshold,
is the claim's and stays.

Tests in `internal/asks` assert that each template declares one `dyn.` slot, fills it with the text the
runner composed, splices the shared fragments, names exactly its class's ids (`flt-[a-z0-9-]+` tokens
against `kb.RigorStatement.IDs()`, `kb.RigorMethod.IDs()` or `kb.BearingFaults.IDs()`), carries none of
those judgement forms, and keeps the instruction fixed around a subject of any size. The id scan and the
judgement-form scan also run over every re-ask render of each class, each case quoted and unquoted, its
values empty. A test holds the `refusal-case` choices equal to the `asks.Refusal…` constants.
`TestFragmentsAreEachRegisteredOnce` checks every fragment is registered once, as a spliced
fragment or as an alternative. `TestShelfHasNoStrayDelimiter`
checks every file on the shelf for a delimiter that does not parse as a slot.

