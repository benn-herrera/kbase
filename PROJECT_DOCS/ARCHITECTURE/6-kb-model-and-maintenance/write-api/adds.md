#### Adds

An add fails if the item exists (SPEC §8 Idempotence) and upserts nothing. Against the records as the
batch's earlier entries left them, it refuses a title its kind already has (`titleHolder`) and, for a
work or an axiom, a label its kind already has (`labelHolder`); then it mints the id (`kb.MintID`) for
every kind. A
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

