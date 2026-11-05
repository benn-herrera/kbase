## 6. KB model and maintenance

Each package and the subsection that states its mechanism.

| Package | Responsibility | Section |
|---|---|---|
| `internal/kb` | kb-root model, schema, record loader, link and marker readers | [§6](the-kb-package.md) |
| `internal/write` | every write, as a batch of ops | [§6](write-api/index.md) |
| `internal/index` | the rows `Derive` yields, strength, summary pages, `verify` checks | [§6](the-derived-layer/index.md) |
| `internal/query` | read queries over the rows `Derive` yields | [§6](the-query-layer.md) |
| `internal/sheet` | claim-graph sheets | [§6](sheet.md) |
| `internal/kbdocs` | readiness documents and README assembly | below |
| `cmd/init.go` | KB creation | [§6](scratch-creation.md) |
| `internal/score` | `kbase score` | [§6](scoring/index.md) |

**Readiness documents (`internal/kbdocs`).** `Stamp` writes `AGENTS.md`, `CONVENTIONS.md` and the
`CLAUDE.md` redirect from `internal/kbdocs/templates/`, each only where absent, and refuses a
`CLAUDE.md` that is not the redirect before it writes. `Maintain` rewrites each such document's
maintained block to its template's: a document without exactly one ordered pair of fence lines is left
untouched and listed in `unfenced`, an absent `AGENTS.md` or `CONVENTIONS.md` is written whole, and
`README.md` is rewritten only where it exists. `AGENTS.md`'s scope pin is `AuthoredKBPin` where no build
record exists. `ComposeExcerpts` and `Overview` assemble the README's overview passage, and `NotProse`
checks the structure of a returned passage.

Contents:

- [§6 The `kb` package](the-kb-package.md)
- [§6 Write API](write-api/index.md)
- [§6 Provenance](provenance.md)
- [§6 Build-only ops](build-only-ops.md)
- [§6 The node vocabulary](the-node-vocabulary.md)
- [§6 The derived layer](the-derived-layer/index.md)
- [§6 Cycle breaking](cycle-breaking.md)
- [§6 The query layer](the-query-layer.md)
- [§6 Frontmatter reading](frontmatter-reading.md)
- [§6 Sheet](sheet.md)
- [§6 Scratch creation](scratch-creation.md)
- [§6 Scoring](scoring/index.md)
