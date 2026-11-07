### `edit-<kind>`

`edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work` and `edit-axiom` change the fields of the node an entry's `id` names. A field given replaces that field's value whole; at least one is required.

| Op | `id` (required) | Fields it may change |
|---|---|---|
| `edit-claim` | `clm-…` | `title`, `rigor`, `rationale` |
| `edit-analysis` | `ana-…` | `title`, `rigor`, `rationale` |
| `edit-experiment` | `exp-…` | `title`, `status`, `rigor` |
| `edit-work` | `wrk-…` | `label`, `title`, `rigor`, `rationale` |
| `edit-axiom` | `axm-…` | `label`, `title` |

Fields take the domains of the adds ([§8](add-kind.md)). `label` is checked as `add-work` and `add-axiom` check it.

**Result**: `id`; and on an `edit-work` entry that changes `label`, `rewritten`, each line it rewrote a reference on, `path` and `line`. An edit is `unchanged` where it changes no value, no `manually-scored` flag and no `judged-hash`.

**Rigor.** The `manually-scored` flag and the `judged-hash` an edit writes are defined in [§10](../10-metadata-format-version/provenance-and-the-judged-hash.md). An edit given a `rigor` on a claim, an analysis or an experiment hashes the node's statement as the edit leaves it; an edit giving no `rigor` keeps the record's `judged-hash`.

**Links stay live.** A title change, or an axiom's label change, keeps every link to the node live, in the same write:
- `edit-axiom` rewrites every `axioms.md` heading that ends in its marker, keeping the marker, and appends its heading where none exists, creating `axioms.md` where the file is absent.
- A citation of the node in the naming form ([§10](../10-metadata-format-version/citations.md)) gets the new title as its text. Any other link text is a quote or a paraphrase, the author's words, and stays.
- Where the edit moves an anchor — a hosted node's, a work's or an axiom's heading or summary entry, or another entry's repeat suffix shifting with it — every link into the old anchor is updated to the new one. A link updated is an inline link or a reference definition outside code, in any document but the summary pages, which the edit does not rewrite: the rebuild of the derived views renders them. A citation of the node whose destination resolved to the node's link target before the change points at the target after it, written relative to the citing document. Any other link is unchanged.
- Where a rewritten naming-form citation sits in a heading and moves an anchor — that heading's own, or a later same-text heading's `-1`, `-2` suffix — every link whose destination resolves to a moved heading, citation or not, is updated to its new anchor.

An edit writes no document other than those the updates above reach.

**A work's `label` change** rewrites, in the same write, every reference that has the old key as a key, so that none dangles and the export still finds the work's entry. It reads citation spans, reference-list entries and links into them outside code, a blockquote's prefixes read past, in every document but the summary pages:
- each key token of each citation span's `data-cites` (`<span class="citation" data-cites="…">`, the imported LaTeX citations), and the span's text where it is the key-only rendering, the parenthesised key list;
- each reference-list entry's `id="ref-<key>"`, and each link into `#ref-<key>`.

kbase citations need no rewrite, since they name the node by id and the naming form's text is the title. Prose that mentions the key as words, such as a rationale, is the author's and stays.

**Refusals**:
- no field given, or a field the kind does not have: a shape refusal whose `allowed` is the fields the kind's edit may change, neither `id` nor `operation` among them;
- an `id` of another kind: at `id`, the remedy naming the edit op for that kind;
- an `id` no node has: `check: unknown-id` at `id`, `detail` `no node <id>`;
- a `rigor` or `status` outside its domain; a prose value containing a line break ([Values input](write-batches/values-input.md));
- a `title` or a `label` another node of the kind has: at that key, the `detail` naming that node;
- a work `label` that is not a citation key ([§10](../10-metadata-format-version/five-node-types.md)): at `label`;
- a work `label` a citation span or a reference-list entry already uses as a key, since the rewrite would merge two references into one: at `label`, one item per such line, each naming it by `path` and `line`;
- a `title` under which a naming-form citation of the node would not parse, whole, as one link — an unbalanced `[` or `]` —: at `title`, naming the citing `path:line`.

