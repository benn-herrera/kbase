# tools/measure — build quality against a reference KB

Two read-only measurements of a built KB against a reference KB of the same corpus (the author's
hand-curated one). Python 3.11+, stdlib only; kb_tools is imported from `PYTHONPATH` and its readers
are used, not reimplemented.

```
PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/compare_to_pristine.py \
    --ours <kb-root> --reference <kb-root> --out .claude-temp/<name>/compare [--threshold 0.30]

PYTHONPATH=<dir holding kb_tools/> python3 tools/measure/measure_unmarked_shortlist.py \
    --ours <kb-root> --edges .claude-temp/<name>/compare/edges.tsv --out .claude-temp/<name>/shortlist
```

In this repository `<dir holding kb_tools/>` is `.claude/agents`, the installed agent set the
recipes use (`just measure-kb-roots` sets it). Run the shortlist measurement after the comparison,
on the same `--ours`: it reads the comparison's `edges.tsv`.

## Privacy

The measurement corpus is private. Its directory reaches these scripts only as a command-line
argument; nothing tracked names it, its paths or its text. `--out` is required and must be a
directory under the project's `.claude-temp/` (not `.claude-temp/` itself). Any other `--out` is
refused with a message naming this rule, and nothing is written. The outputs quote claim titles and
text from both KBs, so they stay in scratch. The scripts print only the list of files they wrote,
never results.

## Exit status and failures

- **0**: every output file was written.
- **2**: an argument is unusable. This covers a kb-root that is not a directory, an `--edges` that
  is not a file, a `--threshold` outside (0, 1], and an `--out` outside `.claude-temp/` or naming an
  existing file. Nothing is written.
- **1**: an input cannot be read as a whole. This covers a kb-root holding no KB document, a
  reference with no claim entry, a tree or graph the kb_tools readers refuse, and an `edges.tsv`
  lacking the columns below. Nothing is written.

One bad document or node does not stop either script. The comparison logs it, leaves that claim out
(a reference claim keeps its title with empty text), and lists it under "Read failures" in
`summary.md`. The shortlist measurement skips an `edges.tsv` line whose field count differs from the
header and counts it in `summary.md`.

Output is deterministic. Identical inputs give byte-identical files. Rows are in a stated order,
ties are broken by id, and counts are listed most first, then by name.

## How a claim of the reference is matched

Each claim, on both sides, becomes a bag of words: its title plus its statement, tags and markers
removed, lower-cased, words of two or more letters, less kb_tools' `shortlist.STOPWORDS`. The bags
are weighted with `shortlist.idf` and `shortlist.weigh` over one shared vocabulary, then compared by
`shortlist.cosine`. A node of ours **matches** a reference claim when their cosine is at least
`--threshold` (default 0.30). A reference claim may match several nodes of ours, or none.

- **A reference claim's statement** is read from each leaf whose frontmatter lists it. It runs from
  the claim's `<!-- claim-quality: … -->` marker to the next heading or marker. Without a marker, it
  is the leaf's opening section.
- **A node of ours** is read through `graph.read`. Its spans are its labelled block plus every proof
  of that block, its equation fence, or its prose paragraph (`prose.readable`). Without a paragraph,
  the span is just its marker line.

## compare_to_pristine.py — outputs

All TSV files are UTF-8 with one header line. A tab or newline inside a cell is written as a space.

### `claim-matches.tsv`

One row per (reference claim, matched node of ours). Rows run in reference title order, then by
score descending. A reference claim with no match has one row whose `ours_title` is `UNMATCHED` and
whose other ours cells are empty.

| Column | Meaning |
|---|---|
| `ref_id`, `ref_title` | the reference claim |
| `score` | cosine, 3 decimals |
| `ours_id`, `ours_kind`, `ours_title`, `ours_document` | the node of ours; `ours_kind` is `block`, `equation` or `prose` |

### `candidates.tsv`

Each reference claim's five best-scoring nodes of ours, whether or not they reach the threshold.
Same columns as `claim-matches.tsv`, plus `rank` (1–5) after `ref_title`. Use it to see why a claim
went unmatched.

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

**The mark test.** It reads the text of every span of A's matches and looks for a mark of B there. A
mark is either of two things:

- an anchor in the span that resolves to a document hosting a match of B;
- B's printed name and number, taken from our block (`Lemma 2`) or from the reference title.
  Abbreviations count (`Thm`, `Prop`, `Cor`, `Conj`, `Def`), as does an optional `~`.

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

For an `endpoint unmatched (target)` edge, both tests still run, on the reference title alone. The
`mark_split` column is the source tool's and covers only `references only` and `nothing`, while
`evidence` covers every miss. A `marked` row is always `present, unused`. An `unmarked — needs
reading` row is `present, unused` only when the title test hits.

### `summary.md`

The headline figures:

- claim and edge counts on each side, and the read failures;
- the match count, matches per reference claim, and the best score per reference claim;
- the unmatched claims;
- matches by reference top-level directory × our top-level directory;
- recall-class counts;
- the mark split, by scope and at document level;
- the evidence split, overall and by recall class;
- a threshold sweep (0.20–0.50), giving the recall classes and marked misses at each threshold.

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
