#### The parse resolves the numbered lines

`assemble` returns the closed text and a line per line of
it — the number the call writes, the passage its bytes lie in, and its byte range — because a charge's
`passage`, `lines` and `offset` are facts about that rendering: a `rigor` call's two passages are a
node's `title` and `statement`, a `bearing` call's are the `source`'s and the `target`'s.
`score.ParseAnswer` (`internal/score/answer.go`) reads a reply against them and the item's class's
`kb.FaultTable`: a charge line's field is one number or a range `N-M`, which `closedText.look`
resolves — every number one the call shows, all of them in one passage, and nothing between them — so
a range crossing from one passage to another, or over a blank line, a label or a heading, is
`malformed`. The recorded `citation` is the cited lines as the call carried them, without the gutter,
joined by newlines, and `offset` is that quote's byte offset in the closed text. The edge's
`## Context` — assembled but carrying no number — is a site for no charge; a numbered line lies in one
passage, so `both` is no longer produced, and a line written earlier may carry it and keeps the
meaning it was written with. The read yields `Answer{Faults, Moot, Value}`: the charges, the charge
lines a reply recorded beside a retired `flt-pending` (parsed, priced nowhere, and no longer produced),
and the price `kb` gives the ids. The parse also refuses a charge
naming no line, an id no table holds — `flt-pending` among them — and a number the call does not show.
Duplicate and
overlap collapsing is the parse's, over cited line sets: an identical set written twice collapses to
one charge, and two sets of one id that overlap without coinciding are refused. The ids the ledger
records are the whole input of the price, which is what makes a landing checkable and a run
re-scorable without a model. The context stays in the assembled text because the ask judges an edge
on it; the parse merely refuses to anchor a charge there.

