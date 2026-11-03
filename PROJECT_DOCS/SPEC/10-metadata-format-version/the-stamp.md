### The stamp

A KB declares its metadata format as `kb-format: <major>.<minor>.<patch>`, semver,
in `kb-root/.index/format.yaml`: one block mapping carrying the one key, no `---` fences. The entry
point's frontmatter is the stamp's superseded place, standing in every KB older than `3.0.0` and
read wherever it still stands. Three forms are
superseded: `1.0.0`, which `2.0.0` replaced, `2.0.x`, which `3.0.0` replaced, and `3.0.0`, which
`4.0.0` replaces. Major: a reader
built for the old major misreads the KB (a renamed field,
changed semantics, a changed anchor rule). Minor: additive; an old reader ignores the addition
safely. Patch: the bytes a writer produces changed, their meaning did not.

