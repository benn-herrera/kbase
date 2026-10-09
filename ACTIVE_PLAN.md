# ACTIVE PLAN – the uncertainty pool and the hit list

**Status:** closed 2026-10-08. U1 done and stopped at a large gap under the rolling rule,
settled with the owner; U4 done (`../adjagent/KB_BUILD_PROCESS_HANDOFF.md`); M done (the mark
test re-keyed, which found no new evidence; `TAG_RE` fixed, which moved the baseline to 37 of 43
matched and 20 depends / 21 unmatched; the shortlist script refusing an older format; `just
test-measure`); U2 and U3 parked. U1's reading (`tools/measure/measure_pool_yield.py`, runs
`.claude-temp/measure/measure_pool_yield/20261008T152600` and `…152603`): on the 2026-10-08 build
the edge categories the build declares — `demoted` (6 rows), `references` (4), `defaulted` (0) —
contain none of the author's 47 missed edges; the claim categories — `unsupported` (159 claims,
half the KB, since no scoring pass has run on this fixture and every score is pending) and
`unanchored` (124) — catch 26 and 23 of them, about what a random set of that size would. The
declared union is 234 members, 0.74 per claim, 77 % yield: the "pool half the node count" failure
the notes name. Saturday's build reads the same. A hit list over the declared categories would
list most of the KB and point at nothing; the signal that would localise the work is the
near-miss ranking, which cosine cannot give and the classifier asks are meant to. U2 as written
is therefore not worth building now. Owner's decision 2026-10-08: U2 stopped; U1 stays the
instrument, run beside recall after every tactic change; U3 goes with U2; U4 and M run now. The
hit list returns when a scoring pass has run on the fixture and a ranking signal puts the author's
premises near the top. The first scoring pass ran 2026-10-08 (`.claude-temp/scoring/verdict.md`):
a headless `claude-reaper` session on Flash-Next graded the 114 claims the comparison matches plus
their cones, 19 batches, 0 ungraded, verify green, tagged `scored-20261008` in the fixture; on the
20 exactly matched claims the scorer's ranks agree with the author's (Spearman 0.87) about one band
lower. 202 claims remain pending; a second pass over them is the precondition the pool's claim
categories still need. Succeeds the node-matching plan,
closed 2026-10-08 with its record carried below. kbase is the canonical source of the KB
toolchain's design; kb_tools adopts its changes, and leads only on defect fixes found in use
(SPEC §2). The contract documents govern on any disagreement with this plan, and this plan is
corrected. Owners: PC python-coder, GC go-coder, TW tech-writer, PE prompt-engineer.

## Where the node-matching plan left the problem

The design notes (`ROADMAP_PLANS/UNCERTAINTY_POOL_AND_HIT_LIST.md`) set the goal: zero human
search, human adjudication only over what the build declares uncertain, with the floor measured
rather than assumed. The floor is now measured, on the fixture against its author's 71 edges, for
the build of 2026-10-08 (Flash-Next, split-budget shortlist, an unmarked yes landing as a depends
edge; `.claude-temp/n4/build-verdict.md`): 19 edges recovered, 27 with an endpoint the build cannot
match (the author's register-only abstractions and pre-surgery numbering), 19 with both ends
matched and no edge. Of those 19, 18 are unmarked: 3 were asked and answered `B`; 15 were never
asked, the true target ranking 6, 7, 10, 14, 20, 21, 21, 22, 28, 30, 31, 33, 40, 40 and 226 in
its source's cosine shortlist (measure run 20261008T102410).

So the pool's hardest category is already answered on this corpus: a near-miss pool drawn from
the cosine ranking catches 4 of the 18 at ten pairs per source and 17 at forty (U1's count by
rank alone; the three asked-and-refused pairs sit at ranks 5, 12 and 16), and forty per source
is 11,179 pairs over 316 nodes — the "pool half the node count" failure the notes name,
forty times over. Cosine does not put the author's missed premises near the top, so no threshold
on rank or score gives a small, high-yield near-miss pool. That category waits on a better
ranking signal, which is the classifier work; the other categories are the build's own
declarations and cost nothing to list.

What the build declares on that fixture today: 6 `demoted` edges, 4 `references` edges, 0
defaulted pairs (every planned pair answered), 316 claims with a pending score (no scoring pass),
159 claims nothing depends on, 124 claims depending on nothing. The unmarked record carries no
rank or score for an asked pair, only the letters offered, the letter and the outcome.

## Decided for the owner's confirmation

- **The hit list is read-only** (open question 2): a list query, emitting SPEC §7 items; the
  existing write ops land an adjudication (`add-depends-on`, `resolve-demoted`, `set-rigor`,
  `insert-*`). It joins the query family, binds to MCP by the annotation like every query, and is
  what the docent's and maintainer's agenda become.
- **One query, one ordering flag** (open question 3): `--for author` orders undecided edges first,
  `--for reviewer` unsupported claims first; the member set is the same.
- **The near-miss category is out until the ranking signal changes** (open question 1): the
  measured curve above is the reason; the list says nothing it cannot stand behind.
- **The record stays as it is.** No category the list carries needs a rank or score, so the
  records-channel change the notes anticipate is not made now; when the near-miss category comes
  in, the stage records what it needs then.
- **Pool size is a build-quality number.** `stats` reports the pool's size per category and per
  node, and the yield measurement joins `tools/measure` so it is taken beside recall after every
  tactic change.

| Wave | Builds | Owner | Acceptance |
|---|---|---|---|
| **U1 — yield measured** | `tools/measure/measure_pool_yield.py`: over a built KB and the comparison's `edges.tsv`, each pool category's size (total and per node) and its yield — the author's missed edges or unmatched claims a reader would find in it — for the declared categories (demoted, references, defaulted, unsupported-and-pending, depending on nothing) and, for the record, the near-miss curve by rank; run over the 2026-10-08 build and Saturday's; README entry | PC | The table written beside the recall numbers in `.claude-temp/`; the declared categories' size per node and yield stated; the near-miss curve as above reproduced |
| **U2 — the query** | `kbase hit-list [--for author\|reviewer] [--limit] [--offset]` in `cmd/` and `internal/query`: members from the index and the three build records (`internal/buildrecords`); each an item with `check` naming its category (`demoted`, `references`, `defaulted`, `unsupported`, `unanchored`), `path` the register or leaf, `key` the node or pair, `detail` the evidence the build saw (origin, letters offered and the default's cause, the pending score), no grade; `--for` the ordering; the MCP binding by annotation, read-only; SPEC §7 row and §8 section; a hermetic test over the results fixture and a transcript under `test_data/fixtures/mcp/`; `stats` gains `pool` (size per category, per node) | GC; TW for SPEC | `test-integration` green with the transcript; over the fixture the list's members equal U1's declared categories; the docent reaches it as a tool (one live question through MCP, the owner's) |
| **U3 — the agenda in use** | The stamped KB documents and the hand-off to adjagent say what the list is for and how a reader lands an adjudication through the write ops; the docent and maintainer surfaces name it (upstream, adjagent's templates) | TW; PE for the stamped text | The stamp tests pass; the hand-off written |
| **U4 — hand-off to kb_tools** | One document in `../adjagent/` carrying the build-process changes since the last hand-offs: the split-budget shortlist (SPEC §4 row), an unmarked yes landing as a depends edge with no attribution ask (SPEC §4 row, the classification record's representation), the hit-list query and `stats`' pool, with their measurements | coordinator | The document written; the SPEC §4 rows it closes named |
| **M — measurement debt** | The comparison's mark test re-keyed on the matched node's walked number and label-rendered numbers so the evidence split is trustworthy; `TAG_RE` no longer eating inequalities (the cosine numbers re-taken); `measure_unmarked_shortlist.py` refusing an older format; a `test-measure` recipe | PC; SDC for the recipe | The evidence split re-read on the 2026-10-08 build; `just test-measure` green |

## Rules this plan runs under

- **Rolling rule (every wave exit).** Acceptance lines that play out → the next wave starts; a
  reasonable gap → the coordinator adjusts, records the adjustment in the checkpoint commit,
  continues; a large gap → stop, settled with the owner. A checkpoint commit between every dispatch;
  the diff is audited, not the report.
- **The measurement corpus is private** (the owner's directories under `test_data/transient/`, named
  in nothing tracked); the arxiv fixtures and the compat harness are the public evidence. In those
  fixture repositories kbase is exercised through an untracked justfile the owner keeps there —
  `just kb-verify`, `just kb-stats`, `just kb-refresh`, running `bin/kbase` by default; the
  checked-in Makefile's `kb-*` targets are kb_tools' and are not kbase's test procedure.
- **Both toolchains move together.** A format change is handed to adjagent with the wave that lands
  it; a change is finished when kb_tools can read what kbase writes.

## Carried record

**Decided earlier, still binding.** Templates are authored here under prompt-engineer review; no
test asserts a prompt's prose; the inference passes' measured constraints — no batching of
candidates into one N-letter answer, no embedding model in the shortlist, no persona system prompt
on a letter ask, no design assuming prefix-cache reuse. *Jointly established units* (a strongly
connected component a person promotes into one node carrying the solidity its members share; the DAG
constraint over units) are a later major bump, after the demoted relation has been lived with. The
letter asks are to become classifier calls; qualifying smaller LLMs for them is not pursued.

**From the node-matching plan (closed 2026-10-08).** The measurement matches the author's claims
one to one by result number, then statement text, then cosine (`tools/measure/compare_to_pristine.py`,
with `latex_numbering.py` and its hand-run tectonic oracle); the unmarked shortlist is a split
budget (SPEC §4); an unmarked yes lands as a depends edge with no attribution ask (SPEC §4);
measured 2026-10-08 at 19 of the author's 71 edges, the floor 27 unmatched endpoints and 19
edges with no credited evidence (`.claude-temp/n4/build-verdict.md`, `.claude-temp/n5/verdict.md`).

**Successor R&D, behind this plan:** the directional shortlist, which the classifier asks
subsume; numbered titles in the product, with polytexnical (ROADMAP "Reader fidelity").

**Owner items.** One named divergence stands in SPEC §4 beyond this plan's, the build inputs on
the trail, with its hand-off in `../adjagent/KB_BUILD_INPUTS_HANDOFF.md`. adjagent findings carried
from the port: `kb-docent` and `kb-maintainer` name kb_tools' maintenance surface only (the
docent's query surface, with `strengthen_by` and `gated-on`, is in
`../adjagent/KB_QUERY_ROWS_HANDOFF.md`'s successor record on adjagent's side); the Edit/Write-only
coder rule belongs in adjagent's shared coder chunk (drafted in
`../adjagent/proposed-agents-md-edit.md`). adjagent is under a major refactor; kb_tools still takes
the build-process changes (wave U4).

**Carried facts.**

- Every refresh, so every write op, runs `dot` once per sheet with no deadline; the wall time per
  write op on the fixture is unmeasured (`internal/sheet`, `renderSVG`).
- No arxiv fixture paper produces a ring, so the demoted relation's end-to-end case runs on a
  synthetic two-lemma paper (`internal/build`, `TestDemotedRing`); a KB whose cuts are plain
  `references` (built by either toolchain before it carried the relation) draws no cuts until
  rebuilt.
- A default-limit `solidity-below` result on a scored KB can exceed personant's 8 KB cap; the arXiv
  fixtures carry no scores.
- On Windows `status` cannot see a running build (no advisory lock).
- A block inside the abstract is treated as not at the margin; no corpus paper exercises it.
- Python edge cases not reproduced: non-ASCII digits, final-sigma lowercasing, a directory named
  `*.md`; line splitting on rarer Unicode separators affects reported line numbers only.
- The server is not deterministic at temperature 0; about one percent of nodes is the noise floor
  for any single build comparison.
- The sheet reads a labelled block as "a blockquote whose first non-empty line opens bold"
  (adjagent's rule, which its goldens require: a label and its statement on one line, `**Theorem
  1.** …`), while the claim graph reads label lines through `kb.LabelLine`; the two recognisers
  differ (`internal/sheet`, `internal/kb`).
- The MCP `build` tool's stdout and stderr captures accumulate under the state store's `reports/`;
  nothing removes them (`cmd/mcp.go`).
- A build started through MCP that refuses inside a stage row (pandoc preflight, the charter)
  returns `done` from the start, and `status` shows the refusal only under `recent-refusals`.
- The `0.9.0 → 1.0.0` converter does not handle a document carrying two comment blocks or a CR-only
  document (`internal/migrate`); neither occurs in any fixture.
- A build over a KB that already stands at an older format drains its obsolete files only at the
  first save after spine-seed.
- The trail records only `--bibliography` files given on the command line; a `.bib` found beside a
  volume root by default is not recorded, so adding one is not caught at resume (`internal/build`).
  A hand-made `kb-build:` commit with an empty body is not compared.
- Policy-test gaps: an `exec.Cmd{Path: …}` literal passes the exec owner's binary check
  (`cmd/execpolicy_test.go`); the import test treats `cmd` as one package, except for the `model`
  rule, which it checks per file (`cmd/importpolicy_test.go`).
- A block node's title is the environment's parenthetical name; the page shows pandoc's own
  sequential counter, not the author's number (`internal/claimgraph`, kb_tools' inventory display
  line). Numbered titles wait on polytexnical.
- The fixture author's reference KB is numbered against the pre-surgery long paper; the
  repository's short papers number themselves afresh, so the `number` match class fires on one
  claim there until the author reconciles (the clone's orientation note names the reconciliation
  as pending). Nine of its 43 claims carry no marker on any page and are matchable only by their
  register entries.
- The comparison's baseline after wave M (tag stripping no longer eating inequalities; the mark
  test keyed on our matched node's page name, walked number and label rather than the reference
  title's number), run 20261008T163115 over the 2026-10-08 build: 37 of 43 author claims matched
  (19 statement, 1 number, 17 cosine); author edges 20 depends, 4 path, 22 nothing, 21 endpoint
  unmatched; evidence over 43 misses 1 present-unused, 38 none found, 4 undetermined. The mark
  test counts a page name like "Theorem 3" found in another volume's text as a mark.
- The measurement tooling's unit tests run by `just test-measure`.
- The unmarked ask probe (`.claude-temp/n3/probe`, 41 pairs with known answers, 123 requests) is
  the qualifier for a model on the letter asks. Probed 2026-10-06 on reaper: Qwen3.8-Flash-Next
  true 2 / 6, 0 false `A` on 33 controls; gemma-4-31B-it the same recall with 5 false `A`;
  Qwen3.6-35B-A3B and Qwen3.8-27B refuse the true pairs; gemma-4-26B-A4B-it is noise. The
  attribution ask after an unmarked yes is where every model but Flash-Next fails
  (`.claude-temp/n4/build-verdict.md`).
- The unmarked record carries no shortlist rank or score for an asked pair (`kb-build-unmarked.yaml`:
  letters offered, letter, outcome); a near-miss category of the hit list would need the stage to
  record them.
