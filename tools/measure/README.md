# tools/measure — build quality against a reference KB

Two read-only measurements of a built KB against a reference KB of the same corpus (the author's
hand-curated one). Python 3.11+, stdlib only; kb_tools is imported from `PYTHONPATH` and its readers
are used, not reimplemented. Beside them sit the theorem-number walker over a volume's LaTeX and its
one-off oracle ("latex_numbering.py" and "check_numbering.py" below), which import nothing outside
this directory, `number_match.py`, the comparison's number class, which imports only the
walker, and `statement_match.py`, its statement class, which imports nothing. The tests of the
walker, both classes, the comparison's tag stripping and mark test, the shortlist sweep's rankings
and the pool's category rules run with `just test-measure` (the comparison's, the sweep's and the
pool's import kb_tools).

```
PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/compare_to_pristine.py \
    --ours <kb-root> --reference <kb-root> --out .claude-temp/<name>/compare [--threshold 0.30] \
    [--volume-root <root .tex> ...]

PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/measure_unmarked_shortlist.py \
    --ours <kb-root> --edges .claude-temp/<name>/compare/edges.tsv --out .claude-temp/<name>/shortlist
```

In this repository `<dir holding kb_tools/>` is `.claude/agents`, the installed agent set the
recipes use (`just measure-kb-roots` sets it). Run the shortlist measurement after the comparison,
on the same `--ours`: it reads the comparison's `edges.tsv`. Both read a KB at the metadata format
the installed kb_tools reads and refuse any other `kb-format` stamp on any kb-root they are given.
Migrate a copy first (`kbase refresh` on a copy of the repository, never on the fixture itself).

## Privacy

The measurement corpus is private. Its directory reaches these scripts only as a command-line
argument; nothing tracked names it, its paths or its text. `--out` is required and must be a
directory under the project's `.claude-temp/` (not `.claude-temp/` itself). Any other `--out` is
refused with a message naming this rule, and nothing is written. The outputs quote claim titles and
text from both KBs, so they stay in scratch. The scripts print only the list of files they wrote,
never results.

## Exit status and failures

- **0**: every output file was written.
- **2**: an argument is unusable. This covers a kb-root that is not a directory, an `--edges` or a
  `--volume-root` that is not a file, a `--threshold` outside (0, 1], an `--out` outside
  `.claude-temp/` or naming an existing file, and `--volume-root` given a number of times other than
  the number of volumes `--ours`' entry point lists. Nothing is written.
- **1**: an input cannot be read as a whole. This covers a kb-root holding no KB document, a
  kb-root with no entry point or whose `kb-format` stamp is not the one the installed kb_tools
  reads (the message names both versions), a reference with no claim entry, a tree or graph the kb_tools readers refuse, and an `edges.tsv`
  lacking the columns below. Nothing is written.

One bad document or node does not stop either script. The comparison logs it, leaves that claim out
(a reference claim keeps its title with empty text), and lists it under "Read failures" in
`summary.md`. The shortlist measurement skips an `edges.tsv` line whose field count differs from the
header and counts it in `summary.md`.

Output is deterministic. Identical inputs give byte-identical files. Rows are in a stated order,
ties are broken by id, and counts are listed most first, then by name.

## How a claim of the reference is matched

Four classes are tried in order, and a reference claim takes the first that fires. `number`,
`label` and `statement` match one-to-one; `cosine` may match a claim to several nodes.

1. **`number`**, only with `--volume-root`. A reference title that opens with a result word and a
   number (`Theorem 5.11 — …`, `Conjecture C.5.1: …`) is keyed by the word and the number's numeric
   part. A trailing lower-case letter the author added by hand (`5.10c`) is recorded as a suffix and
   not matched. The walker (below) runs over each volume root. Each environment whose `name` is that
   word and whose `number` is that number is located. Exactly one must exist across all volumes. Our
   block it matches sits in the volume directory that root maps to and its display line prints the
   same word with pandoc's counter equal to the environment's `ordinal_in_group`. The match is
   one-to-one: when two reference claims reach one block, neither matches.
2. **`label`** would pair a reference claim and a node carrying the same `\label` identifier. It is
   not implemented. `summary.md` counts the `<span id="…">` spans each side's claim text carries,
   so a reference that carries labels shows it there.
3. **`statement`**: a reference claim's marked statement against our node's statement, by
   token-set overlap (Jaccard over each side's words, read as for `cosine` below) at or above 0.6
   (`STATEMENT_THRESHOLD`). A reference claim's marked statements are read from each leaf citing
   it whose Tier-2 marker names it (kb_index_lib's cites rows): the labelled block, math fence or
   prose paragraph holding the marker's line, else the first of them to open after it before the
   next heading or marker. Any one of a claim's statements may match. Our node's statement is its
   first span (below), without its proofs. Nodes the number class took are not offered. Pairs are
   taken in descending overlap, each only while both ends are free; a pair taken while another at
   the same overlap and sharing an end was open is listed as a tie. A reference claim with no
   marker on any citing page has no statement and falls through to `cosine`.
4. **`cosine`**, for every reference claim no earlier class matched. Each claim, on both
   sides, becomes a bag of words: its title plus its statement, tags and markers removed,
   lower-cased, words of two or more letters, less kb_tools' `shortlist.STOPWORDS`. The bags are
   weighted with `shortlist.idf` and `shortlist.weigh` over one shared vocabulary, then compared by
   `shortlist.cosine`. A node of ours **matches** a reference claim when their cosine is at least
   `--threshold` (default 0.30). A reference claim may match several nodes of ours, or none.

**Volume roots.** `--volume-root` is repeated once per volume, in the order `--ours`'
`entry-point.md` lists its volume directories, which is the build's volume order. The i-th root
maps to the i-th directory. The repository's build trail does not record its inputs, so the mapping cannot be
derived from it, and title-to-directory naming is kbase's to compute, not this script's. A wrong
order shows in `summary.md` as page blocks whose printed word and counter name no walked
environment of their volume.

**Why a numbered reference claim has no `number` match**, as `summary.md` lists it, first that
applies:

- `no environment of that word in the source`: no walked environment's `name` is the word;
- `no walked number`: none walks to the number, and some numbered environment of that word was left
  without one (outside the walker's grammar);
- `the block's number differs`: every environment of that word walks to some other number;
- `ambiguous: …`: more than one environment walks to it, more than one block prints that word and
  counter, or two reference claims reach one block;
- `no block with that word and k`: our volume's pages show no block printing that word and counter;
- `no block node at all for that environment`: the block is there, but the claim graph holds no
  node for it.

- **A reference claim's text for `cosine`** is read from each leaf whose frontmatter lists it. It
  runs from the claim's `<!-- claim-quality: … -->` marker to the next heading or marker. Without a
  marker, it is the leaf's opening section.
- **A node of ours** is read through `graph.read`. Its spans are its labelled block plus every proof
  of that block, its equation fence, or its prose paragraph (`prose.readable`). Without a paragraph,
  the span is just its marker line.

## compare_to_pristine.py — outputs

All TSV files are UTF-8 with one header line. A tab or newline inside a cell is written as a space.

### `claim-matches.tsv`

One row per (reference claim, matched node of ours). Rows run in reference title order, then by
score descending. A reference claim with no match has one row whose `ours_title` is `UNMATCHED` and
whose `class`, `score` and other ours cells are empty.

| Column | Meaning |
|---|---|
| `ref_id`, `ref_title` | the reference claim |
| `class` | the class that matched it: `number`, `statement` or `cosine` |
| `score` | the pair's cosine, 3 decimals, whatever the class |
| `ours_id`, `ours_kind`, `ours_title`, `ours_document` | the node of ours; `ours_kind` is `block`, `equation` or `prose` |

### `candidates.tsv`

Each reference claim's five best-scoring nodes of ours, whether or not they reach the threshold or
the claim matched by an exact class. Same columns as `claim-matches.tsv` less `class`, plus `rank`
(1–5) after `ref_title`. Use it to see why a claim went unmatched.

### `ours-nodes.tsv`

One row per node of ours that was read, sorted by document, then id.

| Column | Meaning |
|---|---|
| `ours_id`, `kind`, `document`, `title` | the node |
| `spans` | the 0-based half-open line ranges read, `start-end`, space-separated |
| `tokens` | how many words its bag holds |

### `edges.tsv`

One row per reference claim-to-claim `depends` edge, A → B (A depends on B), sorted by (A, B). A's
matches and B's matches are the nodes of ours each matched.

| Column | Meaning |
|---|---|
| `ref_source`, `ref_source_title`, `ref_target`, `ref_target_title` | the reference edge A → B |
| `recall` | how our graph holds it, first that applies: `endpoint unmatched (source / target / source and target)`; `shared node` (a node of ours matches both ends); `depends` (an edge of ours from some match of A to some match of B); `depends reversed`; `depends path` (a chain of our depends edges from a match of A reaches a match of B); `depends path reversed`; `references only` (only a `references` edge, either direction); `nothing` |
| `mark_split` | for `references only` and `nothing` only: `marked` when the mark test (below) finds a mark, else `unmarked — needs reading` |
| `marks` | for those same rows, the marks found |
| `ours_sources`, `ours_targets` | A's and B's matches, space-separated |
| `scope` | `same paper` when some match of A and some match of B share a top-level directory, else `cross paper`; empty when an end is unmatched |
| `document_level` | for `references only` and `nothing`: the verdict loosened on B's side to every node of ours hosted in a document hosting a match of B (`depends`, `depends reversed`, `references`, `references reversed`, `nothing`) |
| `evidence` | for every **miss** (`references only`, `nothing`, `endpoint unmatched`): `present, unused`, `none found` or `undetermined` (below); empty otherwise |
| `evidence_basis` | the marks and title hits that decided `present, unused` |
| `pair_class` | the classes that matched A and B, in class order joined by ` + ` (`number + cosine`, `statement + statement`); an end no class matched counts as cosine, the class last tried |

**The mark test.** It reads the text of every span of A's matches, tags and comments removed, and
looks for a mark of B there. B is named only through its matches of ours, never through the
reference title, whose number may predate a restructuring of the sources. A mark is any of:

- an anchor in the span that resolves to a document hosting a match of B;
- an anchor in the span whose label (its `data-reference`) is the label of a match of B — a block's
  `<span id="…">` or an equation's fence label — in the same volume. A `\ref` renders as such an
  anchor, its text pandoc's counter (`Theorem <a … data-reference="thm:x">3</a>`), or the label
  in brackets where pandoc could not resolve it;
- a printed name and number of a block match of B: the one its page shows (`Lemma 2`, which is also
  what a rendered `\ref` to it reads as), or the walked one, the environment's word and the
  author's number the walker gives that block (`Lemma 3.1`; for a `number`-class match, the number
  it matched on). Abbreviations count (`Thm`, `Prop`, `Cor`, `Conj`, `Def`), as does an optional `~`.

**The evidence split.** It answers one question: does our text for A carry evidence of B that the
build did not turn into an edge?

- `undetermined`: A has no match, so we have no text to read for A.
- `present, unused`: the mark test finds a mark, or the **title test** hits. The title test runs
  over the same text and looks for B's title as a whole phrase, case-insensitively, at word
  boundaries. It uses B's reference title and the title of each match of B. It looks only for titles
  of two or more words that carry no name and number, which are exactly the titles the mark test
  cannot see.
- `none found`: neither test hits. Our text for A carries no mark or title of B. The relationship
  may still be stated in words neither test recognises ("by the previous result"). Recovering it is
  the job of the unmarked-reference stage, and it is what the shortlist measurement scores.

For an `endpoint unmatched (target)` edge, only the title test can hit, on the reference title. The
`mark_split` column is the source tool's and covers only `references only` and `nothing`, while
`evidence` covers every miss. A `marked` row is always `present, unused`. An `unmarked — needs
reading` row is `present, unused` only when the title test hits.

### `summary.md`

The headline figures:

- claim and edge counts on each side, and the read failures;
- the match count, overall and by class;
- for the number class: each volume root and the directory it maps to, the page blocks no walked
  environment accounts for, and every numbered reference claim with its match or its reason;
- the label spans each side carries;
- for the statement class: how many reference claims have a marked statement located, the best
  overlap per such claim, the ties, and each claim the number class left with its match, its best
  overlap, or why it has no statement;
- matches per reference claim, and the best cosine per reference claim;
- the unmatched claims, with their best cosine;
- matches by reference top-level directory × our top-level directory;
- recall-class counts, overall and against the pair class (a column for each pair class some edge
  carries);
- the mark split, by scope and at document level;
- the evidence split, overall, by recall class and against the pair class;
- a threshold sweep (0.20–0.50) of the cosine class, number and statement matches held, giving the recall classes
  and marked misses at each threshold.

### Reading the numbers

**Recall** is the share of reference edges whose class is `depends`, or `depends path` when a chain
counts. Read it against the match count: an unmatched end is a matching failure, not an attribution
failure. Re-run with another `--threshold`, or read the sweep, before attributing a change to the
build.

Among misses:

- a `present, unused` edge is a mechanical loss. The text named B, and the build did not draw the
  edge.
- a `none found` edge needs inference.
- an `undetermined` edge says nothing about attribution.

## measure_unmarked_shortlist.py

It measures how much of the reference's **unmarked misses** the production shortlist reaches. An
unmarked miss is an `edges.tsv` row whose `mark_split` is `unmarked — needs reading`. The script
ranks each source with kb_tools' production `shortlist.rank` over the statements
`classify.statements` gives. The pool is every other node, less pairs that `attribute.narrow`
already makes candidates. Sources are every node that is not an equation.

A miss is **reached at K forward** when some pair (s, t), with s a non-equation match of A and t a
match of B, has t within the first K of s's ranking. **Either** also counts the reverse pair (t, s).
Reached is the ceiling the unmarked-reference ask can deliver on that miss: it says the pair would
be asked, not that the answer would be yes.

`attribute.narrow` reads the node-pass record through `kb_pipeline.read_node_pass`, which looks only
for kb_tools' `kb-build-node-pass.json` beside the kb-root. When there is none (a kbase build writes
`kb-build-node-pass.json`, which kb_tools' reader finds), the narrowing runs with every prose
reference unjudged. `summary.md` says which case held. With no record, the excluded candidate pairs,
and so the pools, may differ from the build's own.

### Outputs

`misses.tsv`: one row per unmarked miss, sorted by edge.

| Column | Meaning |
|---|---|
| `edge` | `<ref source>-><ref target>` |
| `locality` | the nearest `unmarked.LOCALITIES` value any (match of A, match of B) pair shares; empty when there is no pair |
| `fwd` | the best 1-based rank any forward pair reaches; empty when none |
| `rev` | the same for the reverse pairs |
| `best_pair` | the forward pair at `fwd`, `<s>-><t>` |

`curve.tsv`: one row per K ∈ {1, 3, 5}.

| Column | Meaning |
|---|---|
| `K` | shortlist length per source |
| `asks` | the asks that K costs: the sum over sources of min(K, pool size) |
| `forward`, `either` | misses reached at K |

`summary.md` holds:

- the inputs and the node-pass case;
- node counts by kind, sources and existing candidates;
- the misses measured, the malformed rows skipped, and the `edges.tsv` node ids this build lacks
  (non-zero means `--edges` came from a different `--ours`);
- the locality of the misses;
- the K table;
- the reach of the stage's own shortlist, `unmarked.plan` at `shortlist.K`, which also leaves out
  each source's own equations.

### Reading the numbers

`forward / either` over the misses measured is shortlist recall at that K, and `asks` is its cost.
If the misses are reached at K = 5 but the build still misses them, the ask is what bounds recall.
If they are not reached, the shortlist is.

## sweep_shortlist.py

```
PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/sweep_shortlist.py \
    --ours <kb-root> --edges .claude-temp/<name>/compare/edges.tsv [--probe-pairs <pairs.tsv>] \
    --out .claude-temp/<name>/sweep
```

The same misses, sources and miss reading as `measure_unmarked_shortlist.py`, ranked several ways.
Every ranking starts from the stage's pool: `shortlist.rank` less each source's own equations, so
`today`'s top `shortlist.K` is `unmarked.plan`'s pairs (`sweep.md` states whether it is). A miss's
rank is its best forward pair's. Each ranking is a pure function of the stage's inputs plus, where
named, the nodes' kinds or titles:

| ranking | order per source |
|---|---|
| `today` | the pool as ranked |
| `blocks first` | the pool's block candidates, then the rest, each in pool order |
| `split budget` | two lists, the block candidates and the rest; top K of each is asked, and a target's rank is within its own list |
| `duplicates removed` | the pool less every candidate whose term set (`shortlist.tokens` of its statement) overlaps the source's at `STATEMENT_THRESHOLD` or above (`statement_match.overlap`) |
| `title terms weighted` | `shortlist.rank` over each statement with its node's title appended once more |
| `… + duplicates removed` | the first ranking applied to the `duplicates removed` pool |

`--probe-pairs` is a probe's `pairs.tsv` (columns `pair`, `source`, `target`); each pair's overlap
and whether `duplicates removed` would drop it are listed. Arguments, privacy and exit statuses are
as above; an `--probe-pairs` lacking those columns exits 1.

`ranks.tsv`: one row per miss, sorted by edge: `edge`; `exact` (`yes` when the edge's `pair_class`
is `statement + statement` and its `recall` `nothing`); `target_kinds` (the kinds of B's matches);
one column per ranking holding the rank, empty when no pair is in the pool; `among blocks` (the
rank among block candidates alone, empty when B has no block match).

`sweep.md`: the reach of the misses and the asks at K ∈ {3, 5, 10} per ranking (asks as above,
summed over every list a ranking asks), the exact pairs' ranks per ranking, and the candidates the
duplicates rule removes per source.

## measure_pool_yield.py

```
PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/measure_pool_yield.py \
    --ours <kb-root> --edges .claude-temp/<name>/compare/edges.tsv --out .claude-temp/<name>/pool
```

The uncertainty pool's size and yield, per category. `--ours`' build records must stand beside
it; the planned unmarked record is required. Arguments, privacy and exit statuses are as above,
and `--ours` is refused as the comparison refuses it: a `kb-format` stamp other than the one the
installed kb_tools reads exits 1, as do an index line that is not a record and a missing or
unplanned unmarked record.

**Categories.** The first five are the build's declared uncertainty; `near-miss@K` is not
declared and is measured for the record.

| category | members | evidence |
|---|---|---|
| `demoted` | every `demoted` row of `.index/depends-on.yaml`, as its (source, target) | the row's `origin` |
| `references` | every `references` row, as its (source, target) | the row's `origin`, usually empty |
| `defaulted` | every planned pair of `kb-build-unmarked.yaml` whose outcome is `defaulted` or `drafted`, or that holds no outcome; `answered` and `re-asked` carry a letter and are decided | the outcome and the letters offered, or `not asked` |
| `unsupported` | every claim whose `solidity` in `.index/claims.yaml` is pending (`null`) and that no `depends`, `rests-on`, `supports` or `strengthens` row of `depends-on.yaml` targets; any `supports` row counts, whatever its fraction | the claim's `canonical_path` |
| `unanchored` | every claim whose `depends_on_count` is 0 (that count covers `depends` edges only) | the claim's `canonical_path` |
| `near-miss@K`, K ∈ {5, 10, 20, 40} | every (source, target) with the target at rank ≤ K in the source's pool, the pool as `sweep_shortlist.py`'s `today` (`shortlist.rank`, own equations left out), whose letter in the unmarked record is not `A` | the rank, and the letter or `not asked` |

Note the direction in `unsupported`: a `depends` or `rests-on` row targeting a claim means
something leans on it, while a `supports` or `strengthens` row targeting it lifts it.

**Yield.** The missed edges are the `edges.tsv` rows whose recall class is `references only`,
`nothing`, `endpoint unmatched` or `depends reversed`. A pair category holds a missed edge when
some (s, t) over its `ours_sources` × `ours_targets` is a member, in either direction. A claim
category holds it when a matched end is a member; an `endpoint unmatched` edge is read at the end
that matched. The **declared union** holds an edge any of the first five holds; its member count
is the distinct pairs plus the distinct claims.

### Outputs

`pool.tsv`: one row per category, then `declared union`.

| Column | Meaning |
|---|---|
| `category` | as above |
| `members` | the member count |
| `per_node` | members divided by the claim count of `.index/claims.yaml`, 3 decimals |
| `yield` | the missed edges the category holds |
| `yield_forward` | for a pair category, those held by a forward pair alone (match of A, match of B); empty otherwise |
| `misses` | the missed edges measured |

`members/<category>.tsv`: the members, sorted. A pair category's columns are `source`, `target`,
`evidence`; a claim category's `claim`, `evidence`.

`misses.tsv`: one row per missed edge, sorted by edge: `edge`, `recall` (the class), then one
column per category saying where it holds the edge — `forward`, `reverse` or `both` for a pair
category, `source`, `target` or `both` for a claim category — empty when it does not.

`summary.md`: the inputs and the node-pass case, the claim count, the missed edges by class and how
many are unmarked, the declared categories' table with each yield's share of the misses, and the
near-miss curve. The curve adds the unmarked misses each K reaches forward, the figure
`measure_unmarked_shortlist.py` reports by rank.

### Reading the numbers

A category earns its place by a high yield at a small size per node. A claim category whose
members are a large share of the claims holds a large share of the misses by chance alone, so read
its yield against its `per_node`.

## latex_numbering.py — the author's theorem numbers

```
python3 tools/measure/latex_numbering.py --volume <root .tex> --out .claude-temp/<name>/numbering
```

A counter walker over a volume's LaTeX source. It needs no `PYTHONPATH`, no network and no binary.
It numbers each theorem-like environment the way the author's declarations do, so a node can be
keyed by the number the author's reader sees (`Theorem 5.11`) rather than by pandoc's sequential
counter.

The walker reads the root file and every `\input{…}`/`\include{…}` it reaches, in document order.
It resolves each path against the including file's directory, then against the root's, and adds
`.tex` when absent. A file it cannot find is skipped with a note. Comments (`%` to end of line, not
`\%`) are stripped first. Text TeX never reads is skipped too: an `\iffalse … \fi` with no
conditional or `\else` inside, a `comment` environment, and the arguments of a macro defined with
an empty body (`\newcommand{\comment}[1]{}`). An `\iffalse` closed any other way is read as text,
noted, and leaves every number after it empty. Declarations count wherever they appear;
environments count only after `\begin{document}`, and reading stops at `\end{document}`.

**The grammar** is amsthm's: `\newtheorem{env}{Name}`, `\newtheorem{env}[shared]{Name}`,
`\newtheorem{env}{Name}[counter]`, `\newtheorem*{env}{Name}` (unnumbered), `\numberwithin`,
`\section`/`\subsection`/`\subsubsection`/`\chapter` (starred forms do not count, nor does a level
deeper than `secnumdepth`), `\appendix` (letters from A, for chapters in `book`, `report` and the
like, otherwise for sections), and `\setcounter` to an integer. Formats are LaTeX's defaults: a
counter numbered within another reads `<parent>.<n>`, and a section reads `<chapter>.<n>` in a
chaptered class. Anything else that touches a counter the walker tracks goes to `notes.txt` and
leaves the numbers drawn through that counter empty, never wrong. That covers a redefined
`\the<counter>`, `\addtocounter`, `\stepcounter`, `\refstepcounter`, `\counterwithin`,
`\newcounter`, `\newaliascnt`, thmtools' `\declaretheorem` and llncs' `\spnewtheorem`, and a shared
or parent counter it does not track. An environment the source never declares (a class's predefined
theorem, say) is not seen.

### Outputs

`environments.tsv`: one row per theorem-like environment, in document order.

| Column | Meaning |
|---|---|
| `ordinal` | 1-based position among all rows |
| `env` | the environment name as written |
| `name` | the printed word its `\newtheorem` declares (`Theorem`), as written; empty for an environment declared otherwise |
| `group` | the counter it draws its number from: its own name, or the shared one; empty when unnumbered |
| `ordinal_in_group` | 1-based count within that counter, never reset: what pandoc's counter shows on the page; empty when unnumbered |
| `number` | the author's number (`5.11`, `C.2`, `3`); empty when unnumbered or outside the grammar |
| `label` | the first `\label{…}` at the environment's own level (one inside a nested list or equation names that, not the environment); empty when none |
| `file`, `line` | where the `\begin` is, relative to the root's directory |

`notes.txt`: one line per construct outside the grammar, or include not found, as
`<file>:<line>: <what and its effect>`. It is empty when there is none.

Exit status: **0** when both files are written; **2** when `--volume` is not a file or `--out`
breaks the privacy rule (nothing is written).

## check_numbering.py — the walker's oracle

```
python3 tools/measure/check_numbering.py --volume <root .tex> --out .claude-temp/<name>/numbering \
    [--continue-on-errors]
```

A one-off development check, run by hand. **It needs `tectonic` on `PATH`**, which on first use
downloads its TeX bundle over the network. No recipe runs it and nothing the product does depends
on it; the walker itself never compiles anything.

It copies the volume's directory (less `.git`) into `<out>/build/` and compiles the root there with
tectonic. It reads the label numbers from the root's `.aux` and every `.aux` it `\@input`s. A
`\newlabel{<label>}{{<number>}{<page>}…}` line gives `<number>`; hyperref's extra fields are
ignored, as is a value that is not a braced group. It then walks the same source and compares every
labelled environment.

tectonic runs XeTeX and stops at the first TeX error, so a paper written for pdflatex often fails on
a package unrelated to numbering. `--continue-on-errors` passes `-Z continue-on-errors` so the
compile goes on past such errors. The summary line then says so, because an error that does touch a
counter would make the `.aux` itself wrong.

`agreement.tsv`: one row per labelled theorem-like environment, in document order: `label`,
`walked` (the walker's number), `aux` (the compiled number; empty when the label is not in the
`.aux`) and `agree`, which is one of `yes`, `no`, or `no walked number`. Its last line is a count of
each, after a `#`. The compile's own output is kept as `<out>/build/tectonic-run.log`, beside
tectonic's `.log` and `.aux`.

Exit status: **0** when every labelled environment agrees, **1** otherwise. **2** when an argument
is unusable (also an `--out` inside the volume's directory), tectonic is not on `PATH`, or the
compile fails or runs past 600 seconds. In that last case the log is still kept under
`<out>/build/`, and no `agreement.tsv` is left (an earlier run's is removed).

### Privacy

Both scripts follow the rule under "Privacy" above. The volume reaches them only as `--volume`, and
`--out` must lie under `.claude-temp/`. `environments.tsv` and `agreement.tsv` carry labels and file
names from the source, and `build/` is a copy of it, so all of it stays in scratch. Each script
prints only the list of what it wrote.
