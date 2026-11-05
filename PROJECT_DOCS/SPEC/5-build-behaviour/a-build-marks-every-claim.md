### A build marks every claim

Every claim a build mints is hosted in the call that mints it: its
statement marker ([§10](../10-metadata-format-version/index.md)) is in its leaf, a leaf's only claim and an equation included — an
equation's at the end of the line inside its display-maths fence that has its `\label{…}`,
never on the fence's closing line; a block claim's on its locator's line. The analysis split
(below) moves a prose claim's marker to just after the sentence its title is drawn from, on the line
that sentence ends on. It hosts each analysis it mints by a marker on the first blank line of the
blockquote that is its statement, or at the end of the last line of its claim's paragraph ([§10](../10-metadata-format-version/index.md)).
Where either line closes a fenced block, the marker goes at the end of the line before it.

