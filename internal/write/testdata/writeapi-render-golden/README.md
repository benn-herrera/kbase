# writeapi-render-golden — one committed specimen per rendered metadata shape

`internal/write` composes the bytes these goldens pin into the documents it writes: each `*.md` here
is the renderer's output for one shape, committed so `TestRenderGoldens` can re-render it at test
time and byte-compare. The records the write API writes are `kb`'s encoding, and the register pages
`refresh`'s rendering; neither is pinned here.

These are inputs to tests only: `testdata` is embedded in no build and carried by no install.

Each file holds one shape's exact bytes plus the single trailing newline a text
file ends with; the renderers themselves return text with no trailing newline,
because where a marker sits in its line is the op's business.

`TestRenderGoldens` also requires this directory and the render test's shape table to name the same
set — a golden with no shape, or a shape with no golden, fails before any byte is compared.

| File | Shape |
|---|---|
| `markers.md` | the `<!-- claim-rigor: … -->` statement marker, for a claim, an analysis and an experiment |
| `citation.md` | the sanctioned `["excerpt"](path#anchor)` authority-citation form |

## What the goldens are for

Every other test in `internal/write` asserts a property someone chose to assert — that a re-issued
op is idempotent, that a written record reads back as composed, that an edge survives its
demotion. A regression outside that chosen set is invisible to all of them. The byte-compare is what
catches a change nobody meant to make.

## Regenerating them

`just test` re-renders every shape on every run — that *is* the test. There is no
refresh recipe: a golden here is a handful of lines, and the failure prints both sides whole, so a
deliberate move is applied by changing the renderer, then editing the golden to the bytes the
failure names.

**A refresh is a decision, not a repair.** The golden's whole value is that a
change nobody meant to make shows up as a failing byte-compare; editing one to
make a red test green throws that away. Read the diff, decide the change was
intended, then apply it.
