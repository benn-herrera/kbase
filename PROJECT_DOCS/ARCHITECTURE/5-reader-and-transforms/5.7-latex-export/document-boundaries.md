#### Document boundaries

The first heading of each document, outside code fences, has the document's label as a `{#d{N}-…}`
id, written by Go (`kb.FirstHeading` over `kb.StripCodeFences` finds the heading); a volume's index
has `vol{N}`. A document with no heading opens with one naming its path (the file name less its extension; the directory name for an index). Headings are used
rather than fenced divs because a `:::` line inside a code fence closes a fenced div early. Go marks
the boundary and the filter reads it: the document a node belongs to is the last heading before it
whose id is a `documents` key, and before the first one the front matter.

