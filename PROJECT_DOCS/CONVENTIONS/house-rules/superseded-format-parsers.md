### Superseded format parsers

Numbered schema migrations. Pitfall: keeping a reader for the old form "for compatibility" in `kb`
or `index`, which is the multi-version reader the design forbids (ARCHITECTURE I8). The live format's package
changes, patch bumps included, and owns the full version a save writes; a superseded format's
package is frozen at the version it shipped. Frozen is what it accepts and what a
conversion produces; its refusal and fault text may change.
