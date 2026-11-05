### Analysis split (`analyses-split`)

The stage runs between `depends-attributed` and `phase-3a`. It makes no model call, so it runs in
full under `--no-inference`.

A claim's bounds are its own block or paragraph and the proof blocks that establish it. A proof
establishes the claims its opening names or, where its opening names none, the claim whose block it
follows. A claim's statement proper is the line the build titles it by: a block claim's display
line, or a prose claim's paragraph up to and including the sentence its title is drawn from.

A claim whose bounds contain text beyond its statement proper is split into the claim, which narrows
to its statement proper, and an analysis. An equation node is never split, and neither is a claim
whose bounds are its statement proper alone. The analysis's statement is the claim's own block or
paragraph where that contains text beyond the statement proper. Otherwise it is the first proof, in
reading order, that establishes the claim. Another proof establishing it is no part of this
analysis's statement; its text is a statement only where it is the first proof of some other claim
it establishes. A claim whose analysis's statement would be a blockquote containing no blank line is
left unsplit, there being no line to mark the analysis on; the stage's report counts each such
claim as `unsplit-no-blank-line`.

The claim keeps its record, every edge out of it and every `references` edge into it. The analysis
is minted with the claim's title, rigor `*pending*` and a rationale stating the split. Every
`supports` and `founds` edge into the claim is relocated onto the analysis. So does every `demoted` record
into the claim that replaced a `supports` edge, each with its source, context, bearing and
`manually-scored` unchanged. One `lifts` edge runs from the analysis to the claim, bearing
`*pending*`, `manually-scored: false`. Every path through a split claim before the stage runs
through its analysis after it, so the stage closes no cycle and removes no edge to break one.

