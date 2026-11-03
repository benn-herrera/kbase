#### Labels

Three families that cannot collide: volume `vol{N}`; document `d{N}-{path-slug}`
(`export.DocumentLabel`: the path below the volume directory less its extension, each segment
lowercased with runs outside `[a-z0-9]` one hyphen, joined by hyphens; an index slugs as its
directory name and `index`); author `v{N}-{label}`. Go mints the first two; `selectVolumes`
numbers them after `--volume` narrows the list. A document label another path already owns is a
walk finding, and that document gets no id. The author spelling is the filter's `namespaced`,
the one function every rule that writes a label or a reference calls, so a raw `\label` in maths
and an id the writer prints agree: it keeps ASCII letters, digits and `+-.:;=_` and spells any
other character `ux{hex}`, whatever the label's own shape.

