# ACTIVE PLAN – the node-matching gap: numbering in the measurement, the matcher, the reading

**Status:** approved; wave N1 next. Succeeds the closed two-tranche plan, whose record is carried
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
