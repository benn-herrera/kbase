# tools/survey — arXiv source survey

`arxiv_survey.py` reads arXiv papers' LaTeX source and measures the structure a zero-inference build
can extract: the environments an author declares, how many results carry a `\label`, whether
cross-references point at results or at sections, and which bibliography form the submission ships.
It sizes a corpus before builds are spent on it. Python 3.11+, stdlib plus kb_tools, which is
imported from `PYTHONPATH`.

```
PYTHONPATH=.claude/agents python3 tools/survey/arxiv_survey.py --staged --out .claude-temp/<name>
PYTHONPATH=.claude/agents python3 tools/survey/arxiv_survey.py --fetch math.DG [--fetch cs.LG ...] \
    [--per-category 5] --out .claude-temp/<name>
```

`.claude/agents` is the installed agent set. The survey takes the claim-site vocabulary from it:
`CLAIM_BEARING` and `CLASSIFIED` from `kb_tools.kb_claimgraph.inventory`.

## Why a survey rather than builds

A zero-inference build of one paper takes minutes and halts on the first thing it cannot classify.
The survey takes a second per paper and never halts, so it answers "which of these is worth
building" for a whole sample at once. The environment names it collects are what the closed
classification table would have to admit.

**Sample by category, not by citation.** Construction style follows the field. `math.*` is
theorem-dense and reference-heavy. `cs.LG` states its claims in benchmark tables and declares almost
no environments. Ranking by citations favours the second, which keeps measuring the empty case.
Sample across time as well: arXiv began processing `.bib` files itself in November 2025, so recent
submissions increasingly ship the database, and older ones ship a pre-compiled `.bbl`.

**Counts say what a paper declares; containment says what it connects.** A paper with two hundred
`\ref` and forty theorems may point every reference at a section. Then none of those references is a
claim-to-claim edge. The survey makes two containment readings, which need each environment body's
extent:

- A `\ref` inside a `proof` body is direction-bearing by construction. The proof establishes its
  claim, so that claim rests on what the proof cites.
- A `\ref` whose target `\label` sits inside a claim-bearing body points at a result rather than at
  a document. That count is the ceiling on the number of a paper's cross-references that could
  become claim-to-claim edges.

Both readings are reported as counts and as a fraction of the paper's own `\ref` total.

## Modes

- **`--staged`** surveys the papers already unpacked under `test_data/transient/arxiv/<id>/` (a `/`
  in a pre-2007 id becomes `_`). `just prep-test-integration-arxiv` stages them. Their category
  grouping comes from the justfile's `ARXIV_<CATEGORY> := "<ids>"` variables. A variable counts when
  every word of its value is a versioned arXiv id. `<CATEGORY>` maps to the arXiv category as
  follows: its first `_`-separated token, lower-cased, is the archive, and the rest, upper-cased, is
  the subject class (`MATH_DG` → `math.DG`). The exception is `physics`, whose subject class stays
  lower-case (`PHYSICS_OPTICS` → `physics.optics`). A hyphenated archive (`q-bio`) is not mapped.
  This mode makes no network request.
- **`--fetch <category>`** (repeatable) takes the newest `--per-category` papers of each category
  (default 5) from the arXiv API. It downloads their e-print tarballs into `.claude-temp/survey/`
  and reuses a tarball already there. Requests are spaced 3 seconds apart, arXiv's published
  courtesy. This mode never writes under `test_data/transient/`.

The measurement after the source read is the same in both modes. Only `.tex` files and files without
an extension are read as LaTeX, and files over 8 MB are skipped.

## Outputs and exit status

`--out` is required. It must be a directory under the project's `.claude-temp/` (not `.claude-temp/`
itself). Any other value is refused with exit 2, as is an unusable argument, and nothing is written.
Exit 1 means the justfile holds no category variable. On success the script prints the paths of the
two files it wrote, and its stderr carries one progress line per paper.

Output is deterministic: the same papers give byte-identical files. All JSON is UTF-8 with sorted
keys.

### `papers.jsonl`

It holds one JSON object per paper, sorted by (`category`, `id`). Every row carries `id`, `category`
and `source`. `source` is one of:

- `present`: the paper was measured, and the row carries the fields below;
- `absent`: arXiv serves no source, or the paper is not staged;
- `unparseable`: the measurement raised, and `error` holds the exception.

| Field | Meaning |
|---|---|
| `tex_files` | number of files read as LaTeX |
| `documentclass` | the `\documentclass` names, sorted |
| `newtheorem` | `\newtheorem` handle → display name |
| `labels`, `label_prefixes` | `\label` count; count per prefix before `:` (`<none>` without one) |
| `refs`, `ref_commands` | `\ref`/`\cref`/`\Cref`/`\autoref`/`\eqref` count; count per command |
| `ref_target_prefixes` | count per target label prefix |
| `cites` | `\cite*` command count |
| `proofs` | `\begin{proof}` count |
| `claim_bodies`, `proof_bodies` | outermost bodies of claim-bearing environments (display name, case-folded, in `CLAIM_BEARING`) and of `proof` |
| `labels_in_claim_body` | distinct labels declared inside a claim-bearing body |
| `refs_in_proof`, `refs_in_proof_fraction` | reference commands inside a proof body; that count over `refs` (null when `refs` is 0) |
| `refs_to_claim_label`, `refs_to_claim_label_fraction` | reference commands with a target among those labels; the same fraction |
| `unclassified_display_names` | display names outside `CLASSIFIED`, sorted |
| `has_bib`, `bib_count` | whether, and how many, `.bib` files ship |
| `has_bbl`, `bbl_flavour` | whether a `.bbl` ships: `bibtex` (`thebibliography`, spliceable), `biblatex` (that package's internal format), `unknown`, or null |

### `summary.json`

`categories` maps each category to an aggregate of its rows, and `overall` aggregates every row. An
aggregate holds:

- `papers`, `measured`, and `unmeasured` (id → `source`);
- `refs`, and for each of `refs_in_proof` and `refs_to_claim_label` the total, `_fraction_pooled`
  and `_fraction_paper_mean`;
- `papers_with_refs`;
- `unclassified_display_names` (name → number of papers declaring it).

The pooled fraction is the corpus's own ratio, and a long paper dominates it. The paper mean weights
every paper alike, which is what makes categories comparable. Its denominator is `papers_with_refs`,
because a paper with no references has no fraction to average.
