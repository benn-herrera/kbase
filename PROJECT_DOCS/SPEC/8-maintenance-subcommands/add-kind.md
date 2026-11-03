### `add-<kind>`

`add-claim`, `add-analysis`, `add-experiment`, `add-work` and `add-axiom` are [page §3.1](../../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md)'s node
adds: each op's keys, defaults, result and refusals, and the identity an add checks — a title its
kind holds anywhere in the KB, a label a work or an axiom of its kind holds — refused "node exists"
([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)). A hosted node's marker is placed by `document`, `line` and `snippet` as [page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)
states; the mark an add writes is [page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s, and `title` and `rationale` take [page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s
"Values" check. What this release adds to the page and holds:

- A refusal names its key: `title` or `label` for "node exists" (its `detail` is
  `node exists: <id>`), `line` for a line past the document's end or a fenced block's opening or
  closing line, and `snippet` for a line that does not contain it. The snippet is compared with each
  whitespace run collapsed to one space, and the `snippet` refusal's `detail` names up to five lines
  holding it, those nearest the named line.
- An add given a `rigor`, `*pending*` included, records the judged hash of the node's statement
  as its marker places it ([page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)); one given none writes the never-judged `judged-hash`, all
  zeros.
- `add-axiom` reads an unmarked heading of its text as a `###` heading whose text, markers removed
  and trimmed, is `Axiom <label>: <title>`; it marks that heading by ending its line, trailing
  whitespace dropped, in the axiom's marker, and creates `axioms.md` where the file is absent. Its
  refusal of more than one such heading is keyed `label`, its `path` `axioms.md` and its `line` the
  first.
- An add whose summary entry shifts a standing entry's repeat suffix (`-1`, `-2`) — a title of
  the same slug rendered before it — re-points every link into the shifted entry to its new anchor
  in the same write, as an edit does ([§8](edit-kind.md)). `add-axiom` appends its heading after
  every other, so it shifts no anchor of `axioms.md`.
