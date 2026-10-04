# tools/slice — kb_tools reference dump

`kbtools_dump.py` runs kb_tools' own readers, read-only, over KB trees and writes one
document describing each. The slice comparator reads it. This file is the dump's schema
of record.

```
PYTHONPATH=<fixture>/.claude/agents python3 tools/slice/kbtools_dump.py \
    --ref <paper id>=<kb-root dir>... --kbase <paper id>=<kb-root dir>... --out <file>
```

`--ref` and `--kbase` are each repeatable; at least one root is required. The flag that
supplied a root is its `role`. Python 3.11+, stdlib only; kb_tools is imported from
`PYTHONPATH`, never reimplemented.

**Exit status.** 0 when the document was written, including when a reader failed on a
tree (that failure is recorded in the tree's entry, below). Nonzero, with a message on
stderr and no output written, when a flag value is unusable: no `=`, an empty id, a
path that is not a directory, the same `(paper, role)` pair given twice, no root at
all, or an `--out` whose parent is not a directory.

## Format

The output is JSON, which is valid YAML 1.2: read it with any YAML or JSON parser. It is
UTF-8, written with non-ASCII characters literal (not `\u` escapes), two-space indent,
a trailing newline, and **every object's keys sorted**. List order is meaningful and
stated per list below. Identical inputs give byte-identical output.

All line numbers are **0-based** line indices into the document's text, as kb_tools'
readers count them. All document paths are POSIX paths relative to the kb-root.

## Pairing

A paper's comparison pairs the `role: kbase` root with the `role: ref` root that has
the same `paper`. Roots are sorted by `(paper, role, path)`, so within a paper the
`kbase` root comes before the `ref` root. A paper given under only one role has no
counterpart.

## Schema

```yaml
roots:                       # sorted by (paper, role, path)
  - path: <str>              # the kb-root path exactly as given on the command line
    role: kbase | ref
    paper: <str>             # the id before "=" in the flag value
    errors: {<field>: <str>} # see "Failures"; {} when every reader succeeded
    tree: [<TreeEntry>] | null
    tokens: {<document path>: [<str>]} | null
    forms: {<document path>: <Forms>} | null
    conformance: <Conformance> | null
    inventory: <Inventory> | null
```

### `tree` — `kb_claimgraph.tree.read`

One entry per document of `Tree.documents` (the kb-root's `.md` files after
`kb_index_lib.kb_files`' exclusions), **sorted by `path`**.

| Field | Type | Meaning |
|---|---|---|
| `path` | str | `Document.path` |
| `heading` | str \| null | `Document.heading`: the first `# ` line as the page shows it; null when the document has none |
| `parent` | str \| null | `Tree.parents[path]`. Set when line 1 contains `UPLINK_MARKER` (`↑`) and a Markdown link to a `.md` path; the value is that link resolved against the document's directory, **whether or not a document exists there** (conformance point-3 reports one that does not). Null when line 1 lacks the marker or such a link — the entry point included |
| `children` | [str] | `Tree.children[path]`: the targets of the `.md` links on lines 2 onward, resolved to kb-root paths, **in the order the document lists them**, duplicates removed; `[]` when none |

### `tokens` — `kb_docgraph.text.markdown_tokens`

`markdown_tokens(Document.text)` for every document in `tree`, keyed by its path. Each
value is the document's token list **in document order**. Every document is present,
indexes and the entry point included; a document with no tokens maps to `[]`.

### `forms` — element counts of the load-bearing forms

The inventory lists anchors one per label and citations one per key, so its lengths are
not element counts. `forms` counts elements, per document, for every document in
`tree`, using kb_tools' own patterns and constants over the same text the matching
inventory scan reads:

| Field | Type | Counts | kb_tools symbol | Text matched |
|---|---|---|---|---|
| `uplink_lines` | int (0 or 1) | line 1 contains the marker | `kb_index_lib.UPLINK_MARKER` | line 1 of `Document.text` |
| `anchors` | int | cross-reference anchor elements (one per `<a href … data-reference-type … data-reference …` opening tag, however many labels it names) | `kb_claimgraph.tree.ANCHOR_RE`, the pattern `inventory`'s anchor scan matches | `unquote(strip_markers(text))` |
| `citation_spans` | int | `<span class="citation" data-cites="…">` elements, however many keys each carries | `kb_claimgraph.inventory.CITATION_SPAN_RE` | `unquote(strip_markers(text))` |
| `math_fences` | int | lines equal, after stripping surrounding whitespace, to the fence opener | `kb_claimgraph.inventory.FENCE_OPEN` | `unquote(text)` |
| `labelled_blocks` | int | label lines (bare or wrapped in an identifier span) | `kb_claimgraph.inventory.LABEL_LINE_RE` | each line of `Document.text` |

`math_fences` counts opener lines, not closed fences: kb_tools has no element-level
fence pattern, only the opener constant its scan compares each line against. An opener
line inside an already open fence counts here and not in `inventory.fences`. Labelled
blocks with an identifier are counted by the `blocks` items whose `identifier` is
non-null.

### `conformance` — `kb_claimgraph.conform.gate`

The Document-Tree conformance gate. It stops at the first failed check.

| Field | Type | Meaning |
|---|---|---|
| `passed` | bool | true when `gate` returned |
| `failure` | object \| null | null when `passed`; otherwise the `ConformanceError` (a `kb_claimgraph.report.ClaimGraphError`) it raised |
| `failure.check` | str | `ClaimGraphError.check`, which `conform._refuse` spells `point-<n>`, `<n>` the contract point's number (e.g. `point-3`) |
| `failure.detail` | str | `ClaimGraphError.detail`: the message |

An exception other than a `ClaimGraphError` is not a conformance verdict: `conformance`
is null and the exception is in `errors`.

### `inventory` — `kb_claimgraph.inventory.scan`

Every reading of the `Inventory` it returns, each list **in scan order**: documents in
sorted path order, items within a document in line order (`proofs` follow the same
document order). Each item carries every field of its reader dataclass, under the
dataclass's own attribute name.

No label→document map is emitted: no kb_tools reader produces one. The labels the
inventory reads are `fences[].labels` (equation labels) and `blocks[].identifier`
(block labels), each with its `document`.

`blocks` — `inventory.Block`, one per labelled blockquote:

| Field | Type | Meaning |
|---|---|---|
| `document` | str | document path |
| `environment` | str | the name on the label line (the environment's display name) |
| `identifier` | str \| null | the `<span id>` the source's `\label` became; null when none |
| `display` | str \| null | the locator span (below); null when the content yields none |
| `title` | str \| null | the title read off `display` (below); null exactly when `display` is |
| `start` | int | line of the label line |
| `end` | int | the line after the block's last |

`display` is read from the block's **content**: lines from two below the label line to
the block's end, with metadata markers removed, blockquote markers removed, lines joined
with spaces and every whitespace run collapsed to one space. It is a **prefix** of that
string: the opening bold printed name (`**Lemma 3**`) plus a following `.` if any; or,
where a parenthesised optional argument follows the printed name, through its closing
`)` plus a following `.` if any. For a claim-bearing environment the prefix is then
extended word by word until it occurs in no other block's content string in the same
document; if the whole string still does, `display` is null. It keeps any markup the
content carries (citation spans included). Null also when the content does not open with
a bold printed name.

`title` is the optional argument where one was read and the prefix was not extended;
otherwise `display` with `*` runs removed, trimmed, and trailing periods stripped. In both cases
each citation span is then replaced by its rendered text.

`fences` — `inventory.MathFence`, one per `` ``` math `` fence:

| Field | Type | Meaning |
|---|---|---|
| `document` | str | document path |
| `start` | int | the opening fence line |
| `end` | int | the line after the closing fence line |
| `labels` | [str] | `\label{…}` names inside the fence, in order |

`anchors` — `inventory.Anchor`, **one per label**: a cross-reference whose label
attribute names several labels yields one item per label, sharing every other field.

| Field | Type | Meaning |
|---|---|---|
| `document` | str | document the anchor sits in |
| `line` | int | the line the anchor opens on |
| `reference_type` | str | the `data-reference-type` attribute (`ref`, `eqref`, …) |
| `href` | str | the `href` attribute as written |
| `target` | str \| null | the kb-root path `href`'s document part resolves to; null for a bare fragment or when it resolves to no document of the tree |
| `fragment` | str | the part of `href` after `#`; `""` when none |
| `label` | str | one label of the `data-reference` attribute |
| `hosting_environment` | str \| null | `environment` of the labelled block it sits inside; null in surrounding prose |
| `preceding_word` | str \| null | the word the page shows before the anchor (carried across a printed list); null when none |

`citations` — `inventory.Citation`, **one per key** of each `data-cites` span:

| Field | Type | Meaning |
|---|---|---|
| `document` | str | document path |
| `key` | str | one citation key |
| `state` | str | `resolved`, `unanswered` or `key-only` (`CitationState` values) |
| `line` | int | the line the citation span opens on |

`works` — `inventory.Work`, one per rendered reference-list entry (`<div id="ref-…">`):

| Field | Type | Meaning |
|---|---|---|
| `document` | str | document holding the reference list |
| `key` | str | the citation key the entry is keyed by |
| `text` | str | the entry div's inner content, read after metadata and blockquote markers are removed, with every `<…>` tag removed and every whitespace run collapsed to one space, trimmed |

`proofs` — `inventory.Proof`, one per block whose environment case-folds to `proof`:

| Field | Type | Meaning |
|---|---|---|
| `document` | str | document path |
| `start` | int | the proof block's label line |
| `end` | int | the line after the proof block's last |
| `subjects` | [Block] | the claim-bearing blocks it establishes, each a full `blocks` item; `[]` when none (order below) |
| `head` | [[str, str]] | the opening emphasis run's anchors as `[href, label]` pairs, **sorted** (the reader holds a set) |

`subjects` order: where `head` is non-empty, one block per anchor of the proof's
document that lies inside the proof, is in `head`, and whose fragment names a
claim-bearing block of its target document — in anchor scan order, repeats kept. Where
`head` is empty, at most one block: the block immediately above the proof, if it is
claim-bearing.

### Coverage for the Records check

Which dumped facts kbase's records must carry (ARCHITECTURE §5.5), and which are
outside the comparison. The comparator reads this table, not its own judgment.

| Fields | Class | In the Records check |
|---|---|---|
| every item's `document` | placement | yes |
| `blocks[].environment`, `identifier` | reader | yes |
| `blocks[].display`, `title` | page-defined | no |
| `fences[].labels` | reader | yes |
| `anchors[].reference_type`, `label`, `hosting_environment` | reader | yes |
| `anchors[].href`, `target`, `fragment` | placement | yes |
| `anchors[].preceding_word` | page-defined | no |
| `citations[].key`, `state` | reader | yes |
| `works[].key` | reader | yes |
| `works[].text` | page-defined | no |
| `proofs[].head` | reader (the opening run's anchors) + placement (their `href`) | yes |
| `proofs[].subjects` | claim-graph rule (proof-to-subject binding) | no |
| every `start`, `end`, `line` | positional (below) | no |

### Positional fields

These fields record line positions, which depend on line wrapping — presentation — and
may legitimately differ between roots for the same item. The comparator excludes them
from item equality:

- `blocks[].start`, `blocks[].end`
- `fences[].start`, `fences[].end`
- `anchors[].line`
- `citations[].line`
- `proofs[].start`, `proofs[].end`
- `proofs[].subjects[].start`, `proofs[].subjects[].end`

## Failures

A reader that raises on one tree does not stop the dump. The tree's entry records it:

- `errors` maps the failing field's name (`tree`, `tokens`, `forms`, `conformance` or
  `inventory`) to `"<exception class>: <message>"`, and that field is null.
- Every field reads the `Tree` that `kb_claimgraph.tree.read` returns. When that read
  raises, `errors` holds the single key `tree` and `tree`, `tokens`, `forms`,
  `conformance` and `inventory` are all null.
- When the read succeeds and only building the `tree` entries fails, `errors.tree` is
  set and `tree` is null, while `tokens`, `forms`, `conformance` and `inventory` are
  populated (or carry their own errors).
- A conformance *failure* is not an error: it is `conformance.passed: false` with its
  `failure`.
