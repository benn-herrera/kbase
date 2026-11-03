#### `edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom` (write ops)

These change the record fields given. A field given replaces that field's value whole. At least
one field is required. A title change, or an axiom's label change, also keeps every link to the
node live, in the same write:
- `edit-axiom` rewrites the axiom's heading in `axioms.md`, keeping its marker (see `add-axiom`);
- where the node's anchor moves (an axiom's heading, a work's summary entry, a hosted node's
  summary entry,
  [§1.5](../../1-model/1.5-citations.md)), every link into the old anchor is re-pointed: each
  citation of the node (found by the id in its link title, as `citations-of` finds them) and
  each ordinary link into that heading. Only the link target changes, as `move-node` re-points;
- a citation whose link text is the old title, the naming form, gets the new title as its text.
  Any other link text is a quote or a paraphrase, the author's words, and stays.
- where a rewritten naming-form citation sits in a heading and moves an anchor — that heading's
  own, or a later same-text heading's `-1`, `-2` suffix — every link whose destination resolves
  to a moved heading, citation or not, is re-pointed to its new anchor.

An edit writes no other document.

| Op | `id` | Fields it may change |
|---|---|---|
| `edit-claim` | `clm-…`, required | `title`, `rigor`, `rationale` |
| `edit-analysis` | `ana-…`, required | `title`, `rigor`, `rationale` |
| `edit-experiment` | `exp-…`, required | `title`, `status`, `rigor` |
| `edit-work` | `wrk-…`, required | `label`, `title`, `rigor`, `rationale` |
| `edit-axiom` | `axm-…`, required | `label`, `title` |

- Setting `rigor` on a claim, an analysis or an experiment marks it manually scored ([§2.9](../../2-conventions/2.9-manually-scored-and-judged-hash.md)).
- A work's label change rewrites, in the same write, every reference that carries the old key
  as a key, so that no reference dangles and the export still finds the work's entry:
  - each key token of each citation span's `data-cites` (`<span class="citation"
    data-cites="…">`, the imported LaTeX citations), and the span's text where it is the
    key-only rendering, the parenthesised key list;
  - each reference-list entry's `id="ref-<key>"`, and each link into `#ref-<key>`.

  kbase citations need no rewrite, since they name the node by id and the naming form's text is
  the title. Prose that mentions the key as words, such as a rationale, is the author's and stays.
- **Result** (an `edit-work` that changes `label`): `rewritten`, each rewritten reference's `path`
  and `line`.
- **Result**: `id`; `unchanged` where every value given already stands (and no mark changes).
- **Refusals**:
  - an `id` no node carries;
  - an `id` of another kind: remedy, the op for its kind, named from the prefix;
  - no field given: shape;
  - a field the kind does not carry: shape, `allowed` the kind's fields;
  - a value out of domain;
  - a `title` or a `label` another node of the same kind carries ([§1.1](../../1-model/1.1-nodes.md)), naming that node's id;
  - a work `label` that is not a citation key ([§1.1](../../1-model/1.1-nodes.md));
  - a work `label` already used as a citation key in a `data-cites` span or a reference-list entry,
    naming `path:line`, since the rewrite would merge two references into one;
  - a `title` with an unbalanced `[` or `]` where a naming-form citation of the node would carry it,
    so that it would not read as one link, naming the citing `path:line`.
