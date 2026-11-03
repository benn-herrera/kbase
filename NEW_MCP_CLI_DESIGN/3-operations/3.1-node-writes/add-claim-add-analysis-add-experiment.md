#### `add-claim`, `add-analysis`, `add-experiment` (write ops)

These mint an id, write the node record, and place its statement marker at the named line ([§1.4](../../1-model/1.4-markers.md)).
They carry no edges.

| Key | Type | `add-claim` | `add-analysis` | `add-experiment` |
|---|---|---|---|---|
| `title` | string, one line | required | required | required |
| `rationale` | string | required | required | — |
| `status` | `run` \| `planned` | — | — | required |
| `rigor` | number in [0,1] \| `*pending*` | optional (default `*pending*`) | optional | optional |
| `document` | kb-root-relative leaf path | required | required | required |
| `line` | integer ≥ 1 | required | required | required |
| `snippet` | string | required | required | required |

A hosted node always has its marker: a node with no marker is a `verify` fault, so placing it is
part of the smallest valid add.

- **Result** (per entry): `id`, minted.
- **Refusals**:
  - a missing `document`, `line` or `snippet`: shape, naming the missing key;
  - `document` not a leaf of the KB: remedy, name a leaf;
  - `line` past the document's end;
  - the line does not contain `snippet`: remedy, re-read the document, with the nearby lines where
    the snippet appears;
  - `line` is a fenced block's opening or closing line: remedy, name another line;
  - `rigor` outside `[0, 1] ∪ {*pending*}`: `allowed` the range;
  - `status` outside `run`, `planned`: `allowed`;
  - a prose value holding a line break ([§2.2](../../2-conventions/2.2-write-batches.md), Values);
  - a pending conversion ([§2.6](../../2-conventions/2.6-format-conversion.md)), a lock ([§2.2](../../2-conventions/2.2-write-batches.md)), and a `CLAUDE.md` that is not the redirect ([§1.7](../../1-model/1.7-maintained-blocks.md)).
