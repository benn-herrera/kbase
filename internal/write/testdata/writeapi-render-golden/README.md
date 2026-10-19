# writeapi-render-golden — one committed specimen per rendered metadata shape

`internal/write` composes the KB metadata bytes these goldens pin: each `*.md` here is the
renderer's output for one shape, committed so `TestRenderGoldens` can re-render it at test time and
byte-compare.

These are inputs to tests only: `testdata` is embedded in no build and carried by no install.

Each file holds one shape's exact bytes plus the single trailing newline a text
file ends with; the renderers themselves return text with no trailing newline,
because where an entry sits among its siblings is the store's business.

`TestRenderGoldens` also requires this directory and the render test's shape table to name the same
set — a golden with no shape, or a shape with no golden, fails before any byte is compared.

The `frontmatter-*.md` files are KB format 1.0.0 YAML frontmatter, byte-equal to what kbase's
writer gives the same values.

| File | Shape |
|---|---|
| `claim-entry-full.md` | a `clm-` register entry carrying every optional field, and every depends-on bullet form |
| `claim-entry-minimal.md` | a `clm-` entry carrying none — pending score, no dependencies, no strengthen-by |
| `support-entry.md` | a `sup-` entry: `quality:` in place of `confidence:`, no `strengthen-by` |
| `support-entry-staged.md` | a `sup-` entry staging its beneficiary fan-out in the register — the fan-out's pre-leaf home |
| `frontmatter-claims.md` | the YAML frontmatter's primary `claims:` field, in the canonical inline-list shape |
| `frontmatter-no-claim.md` | the `no-claim:` branch of the primary field |
| `frontmatter-path-stable.md` | the `path-stable:` document attribute, double-quoted where it is not plain-safe, like `no-claim:` |
| `frontmatter-hosts.md` | a container hosting an experiment and two supports under `experiment-nodes:` / `support-nodes:`, with a pending on-point fraction |
| `markers.md` | the `<!-- id: … -->` register marker and the `<!-- claim-quality: … -->` Tier-2 marker |
| `depends-on-bullets.md` | each depends-on bullet shape — claim and framework target, with and without title and context |
| `no-edge.md` | the entry-level `- no-edge: <reason>` foreign-domain exemption |
| `citation.md` | the sanctioned `["excerpt"](path#anchor)` authority-citation form |

## What the goldens are for

Every other test in `internal/write` asserts a property someone chose to assert — that a re-issued
op is idempotent, that a written entry reads back field for field, that an edge survives its
demotion. A regression outside that chosen set is invisible to all of them. The byte-compare is what
catches a change nobody meant to make.

Four properties of these bytes are load-bearing in a way a reader diffing them should know:

- `## <title>` sits **above** `<!-- id: … -->`. kbase's reader binds a marker to the *preceding*
  `##` heading, so an inverted entry takes its predecessor's title, and a register's first entry —
  with no heading above it — yields no record. That inversion is the whole 2026-09-02 failure: five
  claim records lost across the frozen corpus, with no id or reference check firing
  (`../writeapi-regression`).
- The depends-on separator is a real em-dash, U+2014, with a space on each side.
  The reader cuts a bullet's head at ` — ` and at the first ` (`; a hyphen cuts nothing, so the head
  runs on over the title and every `clm-`-shaped token in it becomes a phantom edge.
- `- solidity: *pending*` and the `(solidity *pending*)` annotation are **derived-field
  placeholders**, never values. They are required rather than decorative: refresh *replaces* each in
  place and never inserts one, so an entry rendered without those slots would never receive a value.
  The leaf-references footer is the third derived line, and the one of the three refresh also
  inserts where it is missing.
- In `support-entry-staged.md` the `- supports:` block sits directly under
  `- quality:` and its pairs follow it with nothing between. That contiguity is
  the grammar: the staged-supports reader ends the block at the next `- ` sibling bullet, so a line
  inserted between the opener and the first pair truncates the fan-out to nothing. The block is
  rendered straight after the score line, ahead of every optional bullet and the unconditional
  `- solidity:` line, so no derived line ever sits above it.

## Regenerating them

`just test` re-renders every shape on every run — that *is* the test. There is no
refresh recipe: a golden here is a handful of lines, and the failure prints both sides whole, so a
deliberate move is applied by changing the renderer, then editing the golden to the bytes the
failure names.

**A refresh is a decision, not a repair.** The golden's whole value is that a
change nobody meant to make shows up as a failing byte-compare; editing one to
make a red test green throws that away. Read the diff, decide the change was
intended, then apply it.
