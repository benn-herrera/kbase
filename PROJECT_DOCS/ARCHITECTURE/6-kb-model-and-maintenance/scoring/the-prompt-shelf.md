#### The prompt shelf

Three templates, one per class — `score-rigor-statement.tmpl.md`,
`score-rigor-method.tmpl.md` and `score-bearing.tmpl.md` — each with the one `dyn.` slot the runner
fills with the assembled text, and three shared fragments in `fragmentSlots`:
`fragments/score-system.tmpl.md`, the system prompt, and `score-answer-occurrences` and
`score-answer-close`, the answer section's text all three state alike, so no vocabulary can drift from
the parse or from its siblings. `asks.ScoreRigorStatement`, `asks.ScoreRigorMethod` and
`asks.ScoreBearing` render them through `asks.Render`; `asks.System` serves the system fragments alone
(`systemSlots`), never an answer fragment as a prompt. Provenance is the shelf's rule: embedded in the
binary, written by kbase, no upstream origin and no version pin to drift. The templates carry the
criteria and the line form; they carry no id the class does not accept, and no figure or word a model
could read as a judgement — no decimal in `[0,1]`, no percentage, no weight, no maximum, no band's
word — while a quantity of the subject's own, an example's rate or threshold, is the claim's and
stays: the price is `kb`'s and one fact lives once. A test asserts each template declares one `dyn.`
slot, fills it with the text the runner composed, splices the shared fragments, names exactly its
class's ids (`flt-[a-z0-9-]+` tokens against `kb.RigorStatement.IDs()`, `kb.RigorMethod.IDs()` or
`kb.BearingFaults.IDs()`), carries none of those judgement spellings, and holds the instruction fixed
around a subject of any size; the shelf's `asks.Lint` covers every file as it does every template.

