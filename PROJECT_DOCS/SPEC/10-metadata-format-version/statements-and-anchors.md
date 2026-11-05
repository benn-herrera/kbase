### Statements and anchors

A claim, an analysis and an experiment are *hosted*: each has one *statement marker*,
`<!-- kbase-id: <id> -->`, in a leaf. A node's *host* is the document holding its statement. No
record stores the host or the statement: both are derived from where the marker sits each time the
KB loads, so the marker moves with the text when the author edits the leaf. A work and an axiom
have no statement marker and no host.

**Placement.** A marker is placed at the end of a named line, directly after the last non-blank
character of the line, with no space between. A marker that ends a heading line leaves the
heading's anchor unchanged. A marker is never placed on the opening or closing line of a fenced
block (a display-maths fence included). An analysis paired with a claim has its marker directly
after the claim's marker on the same line, nothing between them:
`# … Cure Is Hard<!-- kbase-id: clm-8350hb --><!-- kbase-id: ana-8350hb -->`. The same marker form
ends each axiom heading in `axioms.md` and each summary entry heading; neither is a statement
marker, and an axiom or work id in a marker hosts nothing.

**Naming a line.** Each op that names a line takes `document` (a leaf, relative to `kb-root/`),
`line` (1-based) and `snippet`. The `line` is the line the op acts on. The `snippet` is a short piece
of text that the line, with each whitespace run collapsed to one space, must contain; it verifies the
line, so it need not be unique in the document. A `line` past the end of the document is refused at
`line`, the remedy to re-read the document and name a line in it. A line that does not contain the
snippet is refused at `snippet`, and the refusal names up to five lines holding it, those nearest the
named line, since the document has likely changed since the caller read it; the remedy is to re-read
the document and name the line holding the snippet. An op that places a marker also refuses a fenced
block's opening or closing line (a display-maths fence included) at `line`, naming the line, the
remedy to name another line. Each op's own refusals are stated with
the op ([§8](../8-maintenance-subcommands/add-kind.md), [§8](../8-maintenance-subcommands/move-node.md), [§8](../8-maintenance-subcommands/add-citation.md)).

**Reading.** A heading's *section* runs from the line after the heading to the next heading as deep
or shallower. A *paragraph* is the run of lines around a marker's line that hold text once the `>`
prefix and markers are removed, all quoted or none, holding no heading line, and in which only the
first line may open a list item; a line of a display-maths fence is in the paragraph whatever its
text.

- A claim's statement is the paragraph its marker sits in. Where the marker ends a heading line, the
  statement is that heading's section.
- An analysis's or an experiment's statement is the section of the nearest heading at or above its
  marker, or the text above the first heading where none precedes it. So a claim marker and an
  analysis marker abutting at the end of a heading line both parse as that heading's section.
- Two layouts that the build's analysis split ([§5](../5-build-behaviour/index.md)) writes parse differently:
  - an analysis marker alone on a blank line inside a blockquote: the analysis's statement is the
    whole quoted run, and a claim the analysis lifts, marked in the same blockquote, has as its
    statement the text of its own marker's line before the marker;
  - a paragraph in which a claim's marker is followed by text and then by the marker of an analysis
    that lifts the claim: the claim's statement is the paragraph up to its marker, and the
    analysis's is the whole paragraph.
- Markers and `>` prefixes are removed from every statement.
- No public write op places the blockquote layout; the add ops and `move-node` can place the
  paragraph layout, and a marker placed into either layout parses by it.
- A hosted node with no marker, or with more than one, is a `verify` fault (`statement-marker`). It
  does not stop the KB loading. A node with more than one marker is hosted by the first in path
  order. A node with no marker has no host, no section anchor and no summary entry.

**Anchors.** A heading's anchor is the one GitHub gives it, so every link resolves on GitHub:
github-slugger over the heading's text as GitHub renders it — HTML comments stripped, closing `#`s
dropped, inline markup reduced to its text (a link to its link text, emphasis and code to their
content) — and trimmed. The algorithm: lowercase; drop every character that is not a letter, mark,
number, connector, space or hyphen; turn each space into one hyphen, without collapsing runs
(`A — B` → `a--b`); and suffix a heading repeated in one document `-1`, `-2`. A hosted node's
*section anchor* is the anchor of the nearest heading at or above its marker.

