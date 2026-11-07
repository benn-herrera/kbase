### `add-<kind>`

`add-claim`, `add-analysis`, `add-experiment`, `add-work` and `add-axiom` each add one node: they mint its id ([§10](../10-metadata-format-version/five-node-types.md)), write its record, and have no edges. A claim's, an analysis's or an experiment's statement marker is placed at the line `document`, `line` and `snippet` name, as [§10](../10-metadata-format-version/statements-and-anchors.md) states; a work and an axiom have no statement marker. A hosted node always has its marker, so placing it is part of the smallest valid add.

| Key | Type | `add-claim` | `add-analysis` | `add-experiment` | `add-work` | `add-axiom` |
|---|---|---|---|---|---|---|
| `title` | string, one line | required | required | required | required | required |
| `rationale` | string, one line, `""` for none | required | required | — | required | — |
| `status` | `run` or `planned` | — | — | required | — | — |
| `rigor` | number in `[0, 1]` or `*pending*` | optional | optional | optional | optional | — |
| `label` | string, one line | — | — | — | required: the citation key | required: any text |
| `document` | kb-root-relative leaf path | required | required | required | — | — |
| `line` | integer ≥ 1 | required | required | required | — | — |
| `snippet` | string | required | required | required | — | — |

An omitted `rigor` is `*pending*`. The `manually-scored` flag and the `judged-hash` an add writes are defined in [§10](../10-metadata-format-version/provenance-and-the-judged-hash.md); an add given a `rigor` hashes the node's statement as its marker places it.

**Result**, per entry: `id`, the minted id; `add-axiom` adds `heading`, `marked` or `appended`.

**Identity.** A node exists where another node of its kind has its `title` anywhere in the KB; a work also exists where another work has its `label`, and an axiom where another axiom has its `label`. Title and label rules are defined in [§10](../10-metadata-format-version/five-node-types.md). An add of a node that exists is `unchanged` where it is identical ([§8](write-batches/idempotence.md)), its `id` the existing node's; otherwise it is refused "node exists" at `title` or `label`, its `detail` `node exists: <id>`. Entries apply in order, so a batch adding one node twice is `unchanged` at an identical repeat and refused at a differing one.

**Refusals**, each at the key named:
- a missing `document`, `line` or `snippet`: a shape refusal naming the key;
- `document` that does not exist or is not a leaf of the KB ([§10](../10-metadata-format-version/documents-of-a-kb.md)): at `document`, the remedy naming a leaf;
- `line` past the document's end, or a fenced block's opening or closing line: at `line`;
- `snippet`, where the line does not contain it: at `snippet`, the `detail` naming the nearby lines that do ([§10](../10-metadata-format-version/statements-and-anchors.md));
- a `rigor` or `status` outside its domain; a prose value containing a line break ([Values input](write-batches/values-input.md));
- an `add-work` `label` that is not a citation key ([§10](../10-metadata-format-version/five-node-types.md)): at `label`.

**`add-axiom`** writes the axiom's record and gives it its heading in `axioms.md`, `### Axiom <label>: <title><!-- kbase-id: <id> -->` ([§10](../10-metadata-format-version/axioms.md)). The axiom's prose under the heading is the author's.
- An unmarked heading of its text is a `###` heading whose text, markers removed and trimmed, is `Axiom <label>: <title>`. Where exactly one exists, it marks that heading by ending its line, trailing whitespace dropped, in the axiom's marker (`heading: marked`). Otherwise it appends its marked heading (`heading: appended`), creating `axioms.md` where the file is absent. It never leaves two headings of one text.
- More than one unmarked heading of its text is refused: `key` `label`, `path` `axioms.md`, `line` the first, the `detail` naming their lines, since kbase does not guess which one the axiom is.
- It appends its heading after every other, so it shifts no anchor of `axioms.md`.

An add whose summary entry shifts an existing entry's repeat suffix (`-1`, `-2`) — a title of the same slug rendered before it — updates every link into the shifted entry to its new anchor in the same write, as an edit does ([§8](edit-kind.md)).

