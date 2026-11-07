#### `claims-discovered`

**Pattern:** LLM classification of prose spans against a fixed answer grammar.

`claimgraph.Discovered` asks, for each paragraph of each leaf's readable prose, whether it states a
result of its own. Its step is `spendsOwnInference`, so `--no-inference` drops it: every leaf stays
`unread`, and every reference counts at `equations-minted`.

- **Pieces.** `renderText` shows a leaf from its first heading as labelled sentences `S1`, `S2`, one
  per line, as the page shows them (markers off, anchors as their text). Sentence boundaries are
  heuristic (`sentenceBounds`): a stop followed by a capital, digit or opening mark, not inside a
  `$` or backtick run and not after a listed abbreviation or an initial. A label naming half a
  sentence still cuts the author's own bytes. A display-maths fence is one unit, shown line by line
  under one label. A paragraph ends at a blank line, a heading or an excluded line, and a list item
  opens one. It is named by its label range, `S3-S5`.
- **Excluded.** Headings and the claim-bearing, proof and definition blocks (`excludedLines`) are
  shown unlabelled as context and end the paragraph they interrupt, so a declared claim is never
  read again. A sentence inside a maths fence cannot open a claim. Code has no exclusion: only the
  ```` ``` math ```` fence is recognised, and any other code block reads as prose.
- **Which paragraphs.** A paragraph is asked when it holds a resolving cross-reference, which owes it
  exactly one verdict (`obligated`), or a sentence that could open a claim (`opensAClaim`: at least
  `minOpeningWords` canonical words, [§12](../../12-constants/index.md), outside a fence). The others get no verdict.
- **Question.** One group per leaf (`asks.Paragraph`, named by the leaf's path), one item per asked
  paragraph, named by its label range. `paragraph.tmpl.md` renders the leaf (`document`, `body`) and
  the paragraph (`paragraph`, `paragraph-text`). `questions/paragraph.question.yaml` is a `noul`
  question, "Does this paragraph state a result of its own?", with a criterion for true and one for
  false. The offered letters are `LetterClaim` (A, the question's `true-letter`) and `LetterNotClaim`
  (B). The answer is one offered letter, taken by `asks.Decide` from the decision API's confidence
  ([§7](../../7-inference-seams/the-decision-api.md)); no reply text is parsed.
- **Nodes.** `judge` walks the answers in paragraph order. A claim's locator is the paragraph's
  excerpt, the author's sentences joined, and its marker goes on the line the excerpt begins on
  (`write.ExcerptLines`). Its title is the paragraph's opener (`opener`: the first sentence that
  could open a claim, else the first) as the page shows it, on one line, cut at the last word boundary
  within `titleMaxChars` ([§12](../../12-constants/index.md)) and ended `…`. A title the leaf already carries takes ` (¶N)`, N the
  paragraph's position among the leaf's readable paragraphs. The id is minted by `add-claim`, and all
  leaves' claims are written in one batch in sorted leaf order once every answer is in.
- **Refused, unreadable or missing answers.** A reply with no offered letter is asked once more — with
  the `decision-correction` text after a refusal, otherwise the identical question
  ([§7](../../7-inference-seams/the-multichoice-ask-interface.md)); a letter on the second ask is
  `re-asked`, otherwise it takes the deterministic default ([§3](../../3-principles.md)): the verdict is
  `defaulted`, cause `no-letter`, and no claim is minted. A claim answer whose excerpt does not begin on exactly one
  line is `defaulted`, cause `unplaceable`. Each default is a progress fallback and counts in
  `stage-C-defaulted`. A call that never completes stops the stage as `inference-failed` after the
  group records are written; the leaves stay `unread` and a resume asks them again.
- **Record and resume.** The node-pass record holds per leaf a state (`unread`, `planned`,
  `landed`), an outcome (`minted`, `no-claim`, `nothing-to-read`) and a verdict per asked paragraph
  (`claim`, `not-a-claim`, `defaulted`) by the 0-based line it begins on. A `planned` leaf is
  written from the record without a model call. The stage exits on a comparison: recomputed over the
  tree it wrote, every obligated paragraph carries a verdict (`verdict-coverage`).
- **Concurrency and caching.** The leaves' groups are one wave of calls, launched through the build's queue
  ([§12](../../12-constants/index.md), ask concurrency) with each group's first item alone ([§7](../../7-inference-seams/the-multichoice-ask-interface.md));
  answers are cached by the digest of the question and state ([§8](../../8-ledger-state-store-and-monitoring/the-answer-cache.md)).

Both stages write through the write ops with the build's provenance ([§6](../../6-kb-model-and-maintenance/build-only-ops.md)); an op
that answers `retry` is re-issued up to `retryLimit` times ([§12](../../12-constants/index.md)), then the stage stops. Everything later
reads claims from the markers in the leaves, so a declared claim and a discovered one are alike to
`references-found` and `claims-culled`; `claims-culled` treats a claim neither block-hosted nor an
equation as a prose claim. `depends-attributed` also reads the verdicts: a reference in a paragraph
judged `claim` is attributed to the claim marked in it, one in a paragraph judged `not-a-claim`
names no source, and one in any other paragraph falls to the block and proof rules.

---

