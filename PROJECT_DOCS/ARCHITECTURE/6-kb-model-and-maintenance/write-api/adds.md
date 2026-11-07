#### Adds

An add of an item that exists is `unchanged` where identical and refused otherwise (SPEC §8
Idempotence); it upserts nothing. Against the records as the batch's earlier entries left them, it
finds the node its kind holds with its label, for a work or an axiom (`labelHolder`), else with its
title (`titleHolder`), through `standing`. Where none stands it mints the id (`kb.MintID`). Where one
does, `addOver` runs the add over that node: it writes the node's record in place (`put`), and a
hosted node's marker, as a fresh add would write them; an axiom's add over a standing axiom first
requires a heading of its text carrying that axiom's marker, and reports `marked`. Any refusal the
add meets becomes the "node exists" refusal, and an add that meets none returns an `existsError`
wrapping it. `applyEntries` unwraps that error with `errors.As` and decides by its per-entry byte
comparison of the record files and the leaves: an entry that moved no byte is `unchanged`, and one
that moved any, or met a refusal after the add, is refused with the wrapped refusal. The comparison is what SPEC's "would change no byte" names, so the values, the
manually-scored flag, the judged hash and the marker's place are compared without a rule of their
own. An `add-edge` over a standing edge does the same with the "edge exists" refusal. A
hosted node's marker is placed through `markAtLine` once `checkNamedLine` has refused a line past the
end, a fence's opening or closing line, and a line without the snippet.

A hosted add and `add-work` run through `keepSummaryLinks`, which updates references to every summary
entry whose repeat suffix the new entry shifts, comparing `anchored` before and after. `summaryMayShift`
first rules out, without a rebuild, an entry whose slug (repeat suffixes stripped) no other title and
no fixed rigor or strength heading shares, so a build's many adds rebuild nothing extra.

`add-axiom` marks the one unmarked heading of its text in `axioms.md`, or appends its marked heading,
and refuses two unmarked headings of its text. An appended heading goes after every heading of
`axioms.md` and shifts no summary link, so `add-axiom` skips `keepSummaryLinks`. `axioms.md` travels
with the leaves as a `leafEdit`.

