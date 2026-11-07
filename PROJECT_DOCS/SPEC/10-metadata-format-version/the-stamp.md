### The stamp

A KB declares its metadata format as `kb-format: <major>.<minor>.<patch>`, semver, in
`kb-root/.index/format.yaml`: one block mapping holding the one key, no `---` fences. Major: a reader
built for the old major misreads the KB (a renamed field, changed semantics, a changed anchor rule).
Minor: additive; an old reader ignores the addition safely. Patch: the bytes a writer produces
changed, their meaning did not. The current version is `4.0.1`.

