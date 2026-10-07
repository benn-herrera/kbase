# ACTIVE PLAN – the node-matching gap: numbering in the measurement, the matcher, the reading

**Status:** N1–N5 done; the first build on the new configuration measured
(`.claude-temp/n4/build-verdict.md`): with the letter asks on Qwen3.6-35B-A3B the attribution
ask answers its three letters a third each where Flash-Next answered 79 % "supported by", so five
author edges the unmarked ask got right came out reversed and 121 candidates landed as
`references`; depends edges 193 against Saturday's 415, author edges found 6 against 16. The
light tier fails the classification ask, not only the unmarked one; today's recall is the
model's, not the shortlist's. Next, with the owner: the 33 author-edge pairs the build asked
re-asked on Flash-Next to isolate the model effect; the design change of carrying an unmarked
yes's direction into attribution; a Flash-Next build on the split budget as the shortlist's
baseline. The reading (`.claude-temp/n3/verdict.md`,
`.claude-temp/n3/probe-verdict.md`): two bottlenecks in order — the shortlist never offers the
author's true premise (rank 14–233 at K = 5 over the six exactly matched misses; the stage's
shortlist reached 4 of 25 unmarked misses, 10 of which cross papers), and the ask's scope, the
claim's own words, refuses argument-level dependencies by design (the probe: 2 of 6 true pairs
`A`, 0 false `A` on 21 reverse and random controls, the two near-miss `A`s cross-volume
duplicates). Next, decided with the owner: N4 (shortlist and duplicates, no inference) now; N5
(ask scope) when the inference server is free.

| Wave | Builds | Owner | Acceptance |
|---|---|---|---|
| **N4 — the shortlist, offline** | The stage's pool is already every node (cross-paper admitted) and it already ranks statement text (`internal/claimgraph/shortlist.go`, `planUnmarked`); the lever is the ranking, which puts the true premise at rank 14–233 of 325 while four of the five exactly matched true targets are block nodes among 26. First a measurement sweep (`tools/measure/sweep_shortlist.py`) over candidate rankings — blocks first, a split budget of blocks and the rest, restatements dropped at the statement class's bar, title terms weighted, combinations — each a pure function over the inputs kb_tools' `shortlist.rank` takes, reporting reach of the 25 misses at K = 3, 5, 10 and the asks each costs; then the chosen ranking ported to Go. Decided from the sweep and the offline acceptance count (reach of the 25 misses / the 16 recovered edges kept / asks — today 4 / 16 / 1,485; blocks first at K = 5 11 / 8 / 1,485, losing half the recovered edges, which came from unmarked asks on prose targets; split 4 + 4 to every source 13 / 16 / 2,376): **the split budget offered to block sources only** — every source its best four prose or equation candidates, a block-kind source its best four block candidates besides — 8 / 16 / 1,292. Offering blocks to every source stays a later step once a build shows what the block asks yield. Measured by `measure_unmarked_shortlist.py`'s rank-of-true-pair over the 25 unmarked misses (run 20261006T143455 is the baseline: forward reach 4 of 25 at K = 5) and the near-miss `A`s of the probe. The change lands in `internal/claimgraph`'s shortlist with kb_tools' `shortlist` kept as the reference for the comparison harness; a divergence row in SPEC §4 until kb_tools adopts | GC | More of the 25 misses reached at K = 5 than 4, the six exact pairs' ranks reported, and no regression on the asked set's planned pairs beyond what the duplicates rule removes; checkpoint green |
| **N5 — the ask's scope** | The source's proof or argument extent offered beside its statement as what the claim "uses", the precision controls re-run as the probe did (six true pairs, reverse, near-miss, random, three repeats) | PC (the probe), then GC (the template and the extent) | Done, falsified (`.claude-temp/n5/verdict.md`): with the extent narrow or wide, and with the clause that licenses reading it, the same 2 of 6 true pairs say `A` and the same 4 refuse; the four are the floor relative to this ask and model. Precision rises monotonically to 0 false `A` on 33 controls with extent plus clause (the two restatement `A`s vanish): a precision change available at the price of prompt length, the owner's call. Qwen3.6-35B-A3B on the same probe: 36 of 41 agree, but 0 of 6 true and one random `A`; not a drop-in for this prompt | Succeeds the closed two-tranche plan, whose record is carried
below. kbase is the canonical source of the KB toolchain's design; kb_tools adopts its changes, and
leads only on defect fixes found in use (SPEC §2). The contract documents govern on any
disagreement with this plan, and this plan is corrected. Owners: PC python-coder, GC go-coder, AR
architect.

## The gap, as measured

The comparison of a kbase build against the fixture author's hand-curated KB
(`tools/measure/compare_to_pristine.py`, run `.claude-temp/measure/compare_to_pristine/20261004T170507`)
matches the author's claims to our nodes by TF-IDF cosine over title and statement at 0.30:

- the author's KB: 43 claims, 30 titled as named results, 12 carrying a section-style number
  (`5.11`, `C.5.1`); 71 depends edges;
- ours: 326 claim nodes (271 prose, 26 block, 29 equation); no title starts with a result word, one
  carries a number; a theorem environment becomes a block node titled by its parenthetical name
  alone, because the page shows pandoc's sequential counter ("Proposition 12") where the author's
  scheme is `\newtheorem{theorem}{Theorem}[section]` with shared sub-counters;
- matching: 36 of 43 matched, many to many (97 of our nodes), median best score 0.43; 7 unmatched,
  3 of them numbered results and 4 the author's framework hinges; 27 of 71 edges have an
  unmatched endpoint before recall is asked.

So recall against the author's graph cannot be read. The author's number is the exact key for the
named results, and it is recoverable from the LaTeX: the counter declarations are in the source
(`grep -hoE '\\(newtheorem|numberwithin)…' <fixture>/*.tex` lists `[section]` resets and shared
counters), labels sit on ten of the fixture's theorem-like environments, and the pandoc filter
carries `\label` identifiers forward on a span.

## Decided

- **The number is produced in the measurement, not the product.** A counter walker in
  `tools/measure` (Python, stdlib) reads the volume's `.tex` files in include order and numbers each
  theorem-like environment as the author's scheme does; the comparison's matcher uses it. Node
  titles, the node-pass record and the sheets do not change: that keeps the compat harness, the
  record bytes and kb_tools parity untouched, and leaves numbered titles in the product to
  polytexnical (ROADMAP "Reader fidelity"), which replaces the walker when it lands. The walker is
  R&D tooling in a directory declared as such; deleting it moves nothing else.
- **The walker's grammar is amsthm's and no more:** `\newtheorem{env}{Name}`,
  `\newtheorem{env}[shared]{Name}`, `\newtheorem{env}{Name}[section|subsection]`,
  `\numberwithin`, `\section`/`\subsection`/`\appendix` (letters), `\setcounter`. Outside it the
  walker yields no number, never a wrong one.
- **The oracle is a one-off, by hand, in scratch.** A script beside the walker compiles a paper
  with tectonic under `.claude-temp/` and reads `\newlabel{<label>}{{<number>}…}` from the `.aux`;
  every labelled result's walked number must equal it. Nothing tracked execs tectonic; no recipe
  requires it; the exec-monopoly test does not learn of it.
- **The matcher's order:** result word plus number (exact), then label where both sides carry one,
  then cosine at the threshold as today; each match carries its class, and the recall tables are
  split by it.
- **The measurement corpus stays private** (the rules in `tools/measure/README.md`); the public
  arxiv corpus is the walker's open evidence.

| Wave | Builds | Owner | Acceptance |
|---|---|---|---|
| **N1 — numbering in the measurement** | `tools/measure/latex_numbering.py`: the walker over a volume root (includes followed), yielding for each theorem-like environment in document order its counter group, its ordinal in the group, the author's number, its label if any; `tools/measure/check_numbering.py`: the one-off oracle (tectonic in scratch, `.aux` read) reporting agreement per labelled environment; README entries for both, the oracle's host requirement stated | PC | Over the public arxiv corpus (`test_data/transient/arxiv/<id>`) and the fixture: every labelled theorem-like environment's walked number equals its `.aux` number, with the papers outside the grammar listed by name and reason; the walker runs with no network, no binary, in under a second per paper |
| **N2 — the matcher** | `compare_to_pristine.py` matches a reference claim whose title names a result word and number to our block node whose page display line is the k-th of its counter group (pandoc's counter) and whose walked number is that number; then by label; then cosine; `claim-matches.tsv` and `summary.md` carry the match class; the recall and evidence tables split by class | PC | On the fixture, the 12 numbered reference claims match exactly or are listed with the reason; the unmatched set and the recall tables re-read beside the 2026-10-04 run in `.claude-temp/`, numbers only in scratch |
| **N2b — the statement class** (added under the rolling rule: the fixture's reference numbers predate a restructuring of its sources, so the key that survives the author's surgery is the statement text) | Between `label` and `cosine`: the reference claim's marked statement on its page, read as `_our_claim` reads ours, against our node's first span, by token-set overlap at a bar read off the distribution (0.6, the one gap; the three pairs just under it are true matches and the first wrong pair sits at 0.48), one to one; the comparison refuses a KB at another metadata format instead of reading zero nodes | PC | Done: 17 of the 34 reference claims with a marked statement match by statement; 9 reference claims have no marker on any page and fall to cosine; the 7 unmatched stay unmatched (6 unmarked, 1 wrong best pair); the recall table is re-read with one-to-one matching (run `20261006T143311` beside `20261006T142231`) |
| **N3 — the reading** | The split of the 71 author edges into floor (no textual evidence) and reducible, by match class; wave 4's parked question — does the unmarked ask say `A` on the author's true pairs — answered over the exactly matched pairs; the framework hinges and the prose granularity characterised | coordinator, with the owner | A verdict note in `.claude-temp/` and the next inference change named with the owner, as a wave of this plan |

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
constraint over units) are a later major bump, after the demoted relation has been lived with.

**Successor R&D, behind this plan:** the directional shortlist (`ROADMAP_PLANS` once written); the
uncertainty pool and hit list (`ROADMAP_PLANS/UNCERTAINTY_POOL_AND_HIT_LIST.md`); numbered titles in
the product, with polytexnical (ROADMAP "Reader fidelity").

**Owner items.** One named divergence stands in SPEC §4, the build inputs on the trail, with its
hand-off in `../adjagent/KB_BUILD_INPUTS_HANDOFF.md`. adjagent findings carried from the port:
`kb-docent` and `kb-maintainer` name kb_tools' maintenance surface only (the docent's query
surface, with `strengthen_by` and `gated-on`, is in `../adjagent/KB_QUERY_ROWS_HANDOFF.md`'s
successor record on adjagent's side); the Edit/Write-only coder rule belongs in adjagent's shared
coder chunk (drafted in `../adjagent/proposed-agents-md-edit.md`).

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
- The comparison's tag stripping (`TAG_RE` in `tools/measure/compare_to_pristine.py`) deletes
  text between `<` and `>` inside an inequality as if it were an HTML tag, lowering overlap and
  cosine scores on both sides; fixing it changes the October cosine numbers.
- `measure_unmarked_shortlist.py` still accepts a KB at an older metadata format silently.
- The measurement tooling's unit tests run by `python3 -m unittest discover -s tools/measure`;
  no recipe runs them.
