# tools/measure — build quality against a reference KB

Two read-only measurements of a built KB against a reference KB of the same corpus (the author's
hand-curated one). Python 3.11+, stdlib only; kb_tools is imported from `PYTHONPATH` and its readers
are used, not reimplemented. Beside them sit the theorem-number walker over a volume's LaTeX and its
one-off oracle ("latex_numbering.py" and "check_numbering.py" below), which import nothing outside
this directory, `number_match.py`, the comparison's number class, which imports only the
walker, and `statement_match.py`, its statement class, which imports nothing. The tests of the
walker, both classes, the comparison's volume-root discovery and title slug, tag stripping,
statement reading, cascade, edge sets, edge verdict, witnesses, claim-match rows, summary sections
and mark test, the shortlist measurement's reading of the build's records, the shortlist sweep's
rankings, the pool's category rules and the cull-aware listing run with `just test-measure` (all but
the walker's and the classes' import kb_tools).

```
PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/compare_to_pristine.py \
    --ours <kb-root> --reference <kb-root> --out .claude-temp/<name>/compare [--threshold 0.30] \
    [--sources <LaTeX sources directory> | --volume-root <root .tex> ...]

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
- **2**: an argument is unusable. This covers a kb-root or `--sources` that is not a directory, an
  `--edges` or a `--volume-root` that is not a file, a `--threshold` outside (0, 1], an `--out`
  outside `.claude-temp/` or naming an existing file, `--sources` given with `--volume-root`, and
  `--volume-root` given a number of times other than the number of volumes `--ours`' entry point
  lists. Nothing is written.
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

Three classes are tried in order, and a reference claim takes the first that fires. `number` and
`statement` match one-to-one; `cosine` may match a claim to several nodes, but never to a node an
earlier class took. There is no `label` class: nothing pairs a reference claim and a node by a
shared `\label`, and `summary.md` says so under its matching heading.

1. **`number`**, only with `--sources` or `--volume-root`. A reference title that opens with a
   result word and a number (`Theorem 5.11 — …`, `Conjecture C.5.1: …`) is keyed by the word and
   the number's numeric part. A trailing lower-case letter the author added by hand (`5.10c`) is
   recorded as a suffix and not matched. The walker (below) runs over each volume root. Each
   environment whose `name` is that word and whose `number` is that number is located. Exactly one
   must exist across all volumes. Our block it matches sits in the volume directory that root maps
   to and its display line prints the same word with pandoc's counter equal to the environment's
   `ordinal_in_group`. The match is one-to-one: when two reference claims reach one block, neither
   matches.
2. **`statement`**: a reference claim's marked statement against our node's statement, by
   token-set overlap (Jaccard over each side's words, read as for `cosine` below) at or above 0.6
   (`STATEMENT_THRESHOLD`). A reference claim's marked statements are read from each leaf citing
   it whose Tier-2 marker names it (kb_index_lib's cites rows; every marker, on either side, is
   read with kb_index_lib's `tier2_markers`, so one naming several claims marks each of them): the
   labelled block, math fence or
   prose paragraph holding the marker's line, else the first of them to open after it before the
   next heading or marker. Any one of a claim's statements may match. Our node's statement is its
   first span (below), without its proofs. Nodes the number class took are not offered. Pairs are
   taken in descending overlap, each only while both ends are free; a pair taken while another at
   the same overlap and sharing an end was open is listed as a tie. A reference claim with no
   marker on any citing page has no statement and falls through to `cosine`.
3. **`cosine`**, for every reference claim no earlier class matched. Each claim, on both
   sides, becomes a bag of words: its title plus its statement, tags and markers removed,
   lower-cased, words of two or more letters, less kb_tools' `shortlist.STOPWORDS`. The bags are
   weighted with `shortlist.idf` and `shortlist.weigh` over one shared vocabulary, then compared by
   `shortlist.cosine`. The nodes the `number` and `statement` classes took are withheld. A node of
   ours **matches** a reference claim when their cosine is at least `--threshold` (default 0.30).
   A reference claim may match several nodes of ours (its **fan**), or none. The **top-1** view
   keeps only the best of them, the first by cosine, then by id; edge recall is reported in both.

**Volume roots.** `--sources` names the LaTeX sources directory and the script finds and maps the
roots itself; `--volume-root` is the explicit form. The two are exclusive.

- **Discovery.** A root is a `.tex` file under the directory (dot-directories skipped) holding
  `\documentclass`, less every file another `.tex` there names in `\input{…}`, `\input …` or
  `\include{…}`; a name is read with and without `.tex`, relative to the naming file's directory
  and to the sources directory. Comments are removed first.
- **Title.** The argument of the first `\title{…}` (balanced braces, over any number of lines),
  rendered to the plain text pandoc's metadata title carries: `\\` (with any `[…]` after it) and
  `\newline` become a space; `\thanks`, `\footnote`, `\vspace` and `\hspace` go with their
  argument; any other command is dropped and its braced argument unwrapped, as is any bare `{…}`
  group; `\&`, `\%`, `\_`, `\#` and `\$` keep their character; `~` is a space; whitespace
  collapses. A root with no `\title`, or one that renders empty, takes its file stem, as kbase does.
- **Slug.** A re-implementation of kbase's `plain` and `slug` (`internal/docgraph/text.go`), which
  name a volume's directory, and must change with them: the title's words lose the emphasis and
  quotation marks kbase deletes (`'` among them, so `Dictator's` reads `dictators`) and the
  punctuation it trims from their ends; then lower-case, every run of characters outside `[a-z0-9]`
  becomes one hyphen, leading and trailing hyphens go; a slug longer than 48 characters is cut to 48
  and back to its last hyphen, then trimmed again; an empty slug is `untitled`.
- **Mapping.** A volume of `--ours`' `entry-point.md` maps to the one root whose slug equals its
  directory. A volume two or more roots reach maps to neither. The number class walks the mapped
  volumes in the entry point's order; with none mapped it is not attempted.

`--volume-root` is repeated once per volume, in the order `--ours`' `entry-point.md` lists its
volume directories, which is the build's volume order. The i-th root maps to the i-th directory. A
wrong mapping, by either form, shows in `summary.md` as page blocks whose printed word and counter
name no walked environment of their volume.

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
score descending (the fan). A reference claim with no match has one row whose `ours_title` is
`UNMATCHED` and whose `class`, `score`, `overlap` and other ours cells are empty.

| Column | Meaning |
|---|---|
| `ref_id`, `ref_title` | the reference claim |
| `class` | the class that matched it: `number`, `statement` or `cosine` |
| `score` | the pair's cosine, 3 decimals, whatever the class |
| `overlap` | for a `statement` row, the Jaccard overlap it matched on, 3 decimals; empty otherwise |
| `ours_id`, `ours_kind`, `ours_title`, `ours_document` | the node of ours; `ours_kind` is `block`, `equation` or `prose` |

### `candidates.tsv`

Each reference claim's five best-scoring nodes of ours, whether or not they reach the threshold or
the claim matched by an exact class. It is a diagnostic of the raw ranking: the nodes the exact
classes took are not withheld here, so a cosine claim's candidates can include a node it cannot
match. Same columns as `claim-matches.tsv` less `class` and `overlap`, plus `rank` (1–5) after
`ref_title`. Use it to see why a claim went unmatched.

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
| `recall` | how our graph holds it, first that applies: `endpoint unmatched (source / target / source and target)`; then, from a node x of A's matches to a node y of B's matches, x and y not both matches of both ends: `depends` (an edge of ours x → y); `depends reversed` (y → x); `depends path` (a chain of our depends edges, through any nodes, from such an x reaches such a y; the shortest, ties by node ids); `depends path reversed`; `demoted (origin <cited, inferred or not named>)` (a `depends` edge x → y that the build's cycle breaking cut, a `demoted` row of `.index/depends-on.yaml`; `, reversed` inside the brackets when it runs y → x); `references only` (a `references` edge, either direction); then `shared node` (a node of ours matches both ends, and none of the above holds); `nothing`. An edge between a node both ends matched and a node only one end matched counts; only two nodes both ends matched never stand as an edge |
| `witness` | what decided a recovering or cut class, always in the direction the built edge is stored: for `depends`, every built edge x → y as `x->y`, space-separated, sorted; for `depends reversed`, every y → x the same way; for the path classes, the shortest path as `x->m1->…->y`; for `demoted`, the cut row; for `references only`, every `references` row joining them; empty otherwise |
| `path_length` | for `depends path` and `depends path reversed`, the edges on the shortest path; empty otherwise |
| `mark_split` | for `references only`, `demoted` and `nothing` only: `marked` when the mark test (below) finds a mark, else `unmarked — needs reading` |
| `marks` | for those same rows, the marks found |
| `ours_sources`, `ours_targets` | A's and B's matches, space-separated |
| `scope` | `same paper` when some match of A and some match of B share a top-level directory, else `cross paper`; empty when an end is unmatched |
| `document_level` | for `references only`, `demoted` and `nothing`: the verdict loosened on B's side to every node of ours hosted in a document hosting a match of B (`depends`, `depends reversed`, `references`, `references reversed`, `nothing`) |
| `evidence` | for every **miss** (`references only`, `demoted`, `nothing`, `endpoint unmatched`): `present, unused`, `none found` or `undetermined` (below); empty otherwise |
| `evidence_basis` | the marks and title hits that decided `present, unused` |
| `pair_class` | the classes that matched A and B, in class order joined by ` + ` (`number + cosine`, `statement + statement`); an end no class matched counts as cosine, the class last tried |
| `source_class`, `target_class` | the class that matched A, and B; `cosine` for an end no class matched |
| `source_scores`, `target_scores` | A's and B's matches as `<node>=<score>`, by id: the Jaccard overlap for a `statement` match, the cosine for a `cosine` match, the node alone for a `number` match; empty for an unmatched end |

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
`mark_split` column is the source tool's and covers only `references only`, `demoted` and `nothing`, while
`evidence` covers every miss. A `marked` row is always `present, unused`. An `unmarked — needs
reading` row is `present, unused` only when the title test hits.

### `summary.md`

The headline figures:

- claim and edge counts on each side (ours: `depends`, `references` and `demoted` rows), and the
  read failures;
- the match count, overall and by class, under a heading naming the classes the cascade runs and a
  line saying no `label` class exists;
- for the number class: with `--sources`, a `sources:` line naming the directory, then
  `<root> -> <volume>` per mapped pair, `<root>: no volume takes slug <slug>` (or `<root>: volume
  <dir> is taken by <n> roots`) per root left unmapped, and `volume <dir>: no root` (or `volume
  <dir>: <n> roots take its slug`) per volume left unwalked; each volume root and the directory it
  maps to, the page blocks no walked environment accounts for, and every numbered reference claim
  with its match or its reason; `not attempted` with its reason when no volume is walked;
- for the statement class: how many reference claims have a marked statement located, the best
  overlap per such claim, the ties, and each claim that falls through to `cosine` (neither the
  number nor the statement class matched it) with its best overlap or why it has no statement;
- matches per reference claim, and the best cosine per reference claim (for a claim left to
  `cosine`, over the nodes not withheld);
- the unmatched claims, with their best cosine over the nodes not withheld;
- matches by reference top-level directory × our top-level directory;
- the reference claims with no marked statement located, which only `number` or `cosine` can
  match: each with its best cosine (over the nodes not withheld, for a claim left to `cosine`),
  its fan at `--threshold`, and the class that matched it or `unmatched`;
- **Number and statement disagree**: each claim the `number` class matched that the `statement`
  class, run as though the number class had taken nothing, matches to another node, with both
  nodes and the claim's statement overlap with each; `not attempted` when no volume is walked;
- recall-class counts at `--threshold`, fan and top-1 side by side, with the path classes' rows
  of length ≤ 2 counted beside them; per recovering class (`depends`, `depends reversed` and the
  two path classes), the rows against the distinct witnesses they carry, in both views; across
  all recovering rows, the built edges their witnesses name (a path's edges one by one), counted
  per row and distinct, in both views; and, over
  the fan, the counts against the pair class (a column for each pair class some edge carries), and
  against the weakest end: the lower of the two ends' best cosine over the ends `cosine` matched,
  banded `< 0.40`, `0.40-0.50`, `>= 0.50`, or `exact both` when neither end is a cosine match;
- the mark split, by scope and at document level;
- the evidence split, overall, by recall class and against the pair class;
- a threshold sweep (0.20–0.50) of the cosine class, number and statement matches held, giving
  the recall classes and marked misses at each threshold, one row for the fan and one for top-1.

### Reading the numbers

**Recall** is the share of reference edges whose class is `depends`, or `depends path` when a chain
counts. Read it against the match count. `endpoint unmatched` is a matching failure, but it fires
only when an end matched **no** node. The comparison cannot tell a wrong match from a right one: an
end matched to the wrong node reads as whatever our graph holds between the wrong nodes, usually
`nothing`, `references only` or `shared node`, and is counted with the build's outcomes. So
`nothing` is an upper bound on what the build missed. Judge a row by its ends' classes and scores
in `edges.tsv` and the weakest-end counts in `summary.md`: an edge whose ends both matched by
`number` or `statement`, or by a high cosine, is the firmer reading. A class that holds in the
fan but not at top-1 rests on a node that was not a cosine end's best. A long path is weak
evidence: in a dense enough graph two fans are joined by some chain by chance, so read `depends
path` by its length. Where rows outnumber distinct witnesses, one built edge or path is credited
with several reference edges; the distinct built edges count that credit across classes, a path
and a direct edge sharing an edge included. Re-run with another `--threshold`, or read the sweep, before
attributing a change to the build.

Among misses:

- a `present, unused` edge is a mechanical loss. The text named B, and the build did not draw the
  edge.
- a `none found` edge needs inference.
- an `undetermined` edge says nothing about attribution.

## measure_unmarked_shortlist.py

It measures how far the build reached the reference's **unmarked misses**. An unmarked miss is an
`edges.tsv` row whose `mark_split` is `unmarked — needs reading`.

**The build's own records** give the headline. Two kbase records beside the kb-root are read: the
unmarked record `kb-build-unmarked.yaml`, through kb_tools' `kb_pipeline.read_unmarked` (its
`planned` pairs and each asked pair's letter), and the cull record `kb-build-cull.yaml`, its
`links` only, parsed with kb_tools' `kb_yaml.parse` as `culled_matches.py` reads its `culls`. Every
pair (s, t) over a miss's matches, s a match of A and t a match of B, s ≠ t, is read both ways:
whether the record planned (s, t) and (t, s), the letter each was answered with, and whether the
cull record's links hold the unordered pair. A pair's **reach**, furthest last:

- `never planned`: planned neither way;
- `planned, not answered`: planned, and no planned way holds a letter;
- `planned, answered no`: some planned way was answered, none with `A` (yes);
- `planned, answered yes; link absent from them`: some planned way was answered `A`, and the cull
  record's links do not hold the pair;
- `planned, answered yes; link in the cull record's links`: as above, and the links hold it.

A miss takes the furthest reach any of its pairs has. Without a cull record beside the kb-root no
links are read, `summary.md` says so, and every answered-yes miss reads as absent from them. A
missing or unplanned unmarked record exits 1.

**kb_tools' recomputation** follows, below the record-based reading and headed as such. The script
ranks each source with kb_tools' production `shortlist.rank` over the statements
`classify.statements` gives. The pool is every other node, less pairs that `attribute.narrow`
already makes candidates. Sources are every node that is not an equation. A miss is **reached at K
forward** when some pair (s, t), with s a non-equation match of A and t a match of B, has t within
the first K of s's ranking. **Either** also counts the reverse pair (t, s). These numbers, the K
table, `misses.tsv`, `curve.tsv` and the `unmarked.plan` reach, are kb_tools' pipeline recomputed
over the tree as it stands: for a kbase build, the shortlist and plan they measure are kb_tools',
not the ones that build ran.

`attribute.narrow` reads the node-pass record through `kb_pipeline.read_node_pass`, which looks only
for `kb-build-node-pass.yaml` beside the kb-root. When there is none, the narrowing runs with every prose
reference unjudged. `summary.md` says which case held. With no record, the excluded candidate pairs,
and so the pools, may differ from the build's own.

### Outputs

`reach.tsv`: one row per pair (s, t) of each unmarked miss, misses sorted by edge.

| Column | Meaning |
|---|---|
| `edge` | `<ref source>-><ref target>` |
| `reach` | the miss's reach, as above |
| `source`, `target` | s, a match of A, and t, a match of B |
| `planned` | `forward` when the record planned (s, t), `reverse` when it planned (t, s), `both`, or empty |
| `letter_forward`, `letter_reverse` | the letter the record holds for (s, t) and for (t, s); empty when not asked or none was chosen |
| `linked` | `yes` when the cull record's links hold the pair, else `no` |

`misses.tsv` (kb_tools' recomputation): one row per unmarked miss, sorted by edge.

| Column | Meaning |
|---|---|
| `edge` | `<ref source>-><ref target>` |
| `locality` | the nearest `unmarked.LOCALITIES` value any (match of A, match of B) pair shares; empty when there is no pair |
| `fwd` | the best 1-based rank any forward pair reaches; empty when none |
| `rev` | the same for the reverse pairs |
| `best_pair` | the forward pair at `fwd`, `<s>-><t>` |

`curve.tsv` (kb_tools' recomputation): one row per K ∈ {1, 3, 5}.

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
- the records read (planned and asked pair counts; the cull record's link count, or that there is
  none) and the misses by reach;
- the locality of the misses;
- kb_tools' recomputation: the K table, and the reach of kb_tools' `unmarked.plan` at
  `shortlist.K` over this tree, which also leaves out each source's own equations.

### Reading the numbers

The reach says how far the build's records carried each miss. `never planned`: the shortlist did
not offer it; `planned, answered no`: the ask declined it; `planned, answered yes` with the link
absent: the yes is not among the links the cull record holds as surviving; with the link kept, the
loss lies after the cull. kb_tools' `forward / either` over the misses is
kb_tools' shortlist recall at that K, and `asks` its cost: it says what kb_tools' plan would have
reached, not what the build did.

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

**Records read.** `.index/claims.yaml` and `.index/depends-on.yaml` through kb_tools'
`kb_load.read_index`, and kbase's unmarked record `kb-build-unmarked.yaml` through kb_tools'
`kb_pipeline.read_unmarked` (its `planned` pairs and each asked pair's letter). Every category
but `kb_tools-near-miss@K` is read from these; `kb_tools-near-miss@K` ranks with kb_tools'
`shortlist.rank` recomputed over the tree, not with anything the build ran.

**Categories.** The first five are the build's declared uncertainty; `near-miss@K` and
`kb_tools-near-miss@K` are not declared and are measured for the record.

| category | members | evidence |
|---|---|---|
| `demoted` | every `demoted` row of `.index/depends-on.yaml`, as its (source, target) | the row's `origin` |
| `references` | every `references` row, as its (source, target) | the row's `origin`, usually empty |
| `defaulted` | every planned pair of `kb-build-unmarked.yaml` whose outcome is `defaulted` or `drafted`, or that holds no outcome; `answered` and `re-asked` carry a letter and are decided | the outcome and the letters offered, or `not asked` |
| `unsupported` | every claim whose `solidity` in `.index/claims.yaml` is pending (`null`) and that no `depends`, `rests-on`, `supports` or `strengthens` row of `depends-on.yaml` targets; any `supports` row counts, whatever its fraction | the claim's `canonical_path` |
| `unanchored` | every claim whose `depends_on_count` is 0 (that count covers `depends` edges only) | the claim's `canonical_path` |
| `near-miss@K`, K ∈ {5, 10, 20, 40} | every (source, target) with the target among the first K of the source's `planned` pairs in the unmarked record, in the record's order (the build's own shortlist), whose letter in the record is not `A`; a K at or above the plan's length per source is the whole plan | the position, and the letter or `not asked` |
| `kb_tools-near-miss@K`, K ∈ {5, 10, 20, 40} | the same with the target at rank ≤ K in the source's pool as `sweep_shortlist.py`'s `today` (kb_tools' `shortlist.rank`, own equations left out); a pair the build planned outside that rank is not a member | the rank, and the letter or `not asked` |

Note the direction in `unsupported`: a `depends` or `rests-on` row targeting a claim means
something leans on it, while a `supports` or `strengthens` row targeting it lifts it.

**Yield.** The missed edges are the `edges.tsv` rows whose recall class is `references only`,
`demoted`, `nothing`, `endpoint unmatched` or `depends reversed`. A pair category holds a missed edge when
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
many are unmarked, the declared categories' table with each yield's share of the misses, the
near-miss curve over the build's plan with the plan's longest per-source length, and below it,
headed as kb_tools' recomputation, the curve under `shortlist.rank`. Each curve adds the unmarked
misses each K reaches forward.

### Reading the numbers

A category earns its place by a high yield at a small size per node. A claim category whose
members are a large share of the claims holds a large share of the misses by chance alone, so read
its yield against its `per_node`.

## culled_matches.py

```
PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/culled_matches.py \
    --matches <pre-cull compare>/claim-matches.tsv --cull <repo>/kb-build-cull.yaml \
    --out .claude-temp/<name>/culled
```

It lists the reference claims whose `number` or `statement` match, in a comparison made over a
tree from before the build's co-linearity cull, is a node the cull then removed. Such a claim
loses its exact match in a comparison over the culled tree and falls to `cosine`, where it can
match unrelated nodes; this listing shows where that happened and which node absorbed the
culled one. Rows of class `cosine` are not listed.

It reads one kbase record, `kb-build-cull.yaml` (SPEC §6 gives its fields), and only its `culls`
(`id`, `into`), parsed with kb_tools' `kb_yaml.parse`, the parser kb_tools' own build records are
read with. `measure_unmarked_shortlist.py` reads the same record's `links` the same way; no
measurement here reads `kb-build-classification.yaml`. It reads no kb-root. A `--matches` lacking the columns
`ref_id`, `ref_title`, `class` or `ours_id`, or a `--cull` with no `culls` list or an entry
lacking `id` or `into`, exits 1; arguments, privacy and the other exit statuses are as above.

### Outputs

`culled-matches.tsv`: one row per exact-class match whose node was culled, in the order of
`--matches`.

| Column | Meaning |
|---|---|
| `ref_id`, `ref_title`, `class` | the reference claim and the exact class that matched it |
| `ours_id` | the node it matched, which was culled |
| `into` | the claim the cull record says it was culled into |
| `survivor` | the end of the `into` chain: `into` itself unless that was culled too, then its `into`, and so on |

`summary.md`: the inputs, the culls in the record, the rows by class and the reference claims
affected.

### Reading the numbers

Each row is a reference claim whose exact match the cull took away. If `survivor` states the same
result, the claim's statement match moved to it, and a comparison over the culled tree that matches
the claim elsewhere is reading the cull, not the build's attribution. Weigh a change in the
comparison's statement-class count against these rows before reading it as a change in the build.

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
