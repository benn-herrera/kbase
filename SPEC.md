# Kbase SPEC

The contract: what any compliant implementation of kbase must do to be
judged compliant, from a clean-room reimplementation's point of view.
[ARCHITECTURE.md](ARCHITECTURE.md) is the design reference — the *how* and
*why*, and the single source for tuning constants this document cites but
does not restate. Internal types, package paths, and other implementation
detail are deliberately absent below; where the current binary's behavior
looks arbitrary rather than contractual, it is flagged in "Needs ruling"
at the end rather than enshrined.

---

## 1. CLI contract

### 1.1 Global flags and process contract

Every verb accepts:

- `--config-dir DIR` — configuration directory.
- `--log-level debug|info|warn|error` (default `warn`).
- `--log-file PATH` — tees diagnostics to a file; console output is never
  redirected away.
- `-h`/`--help`, and `-v`/`--version` on the root command only (prints
  exactly the version string and a trailing newline, nothing else).

**Configuration-directory precedence** (every verb): `--config-dir` flag,
else `$KBASE_CONFIG_DIR`, else `~/.config/kbase`.

**Exit codes.** Success is `0`. Every failure — a bad flag, an unknown
command, a config load failure, a missing required flag, a refused
`--annex`, a locked job directory, a failed catalogue request, a refused
delivery — is `1`; kbase uses no other exit code. An error is printed once,
to stderr, as `error: <message>`. A bare invocation (no verb) prints help
to stdout and exits `0`.

### 1.2 `kbase models`

```
kbase models [--provider NAME] [--timeout DURATION]
```

Lists the selected provider's model catalogue: one identifier per line,
sorted ascending, to stdout. No corpus, no `--out`.

- Provider selection: `--provider`, else `config.toml`'s `provider`, else
  the sole entry in `providers.toml`. Zero usable providers refuses,
  naming `providers.toml`. Two or more entries with none selected refuses,
  listing every pool entry name.
- A pool entry that failed to load is reported to stderr as a warning and
  does not abort the command as long as another entry is usable.
- `--timeout` (default `30s`) bounds the catalogue request.
- A one-line summary (provider, model count, elapsed time) is written to
  stderr.

### 1.3 `kbase configure`

```
kbase configure [--provider NAME] [--model-map heavy=ID,light=ID] [--timeout DURATION]
```

Assigns a model id to each pipeline tier (`heavy`, `light`) and writes the
result into `config.toml`. Provider selection is identical to §1.2.

**Auto-detection, exact and testable.** Normalize a catalogue id by
dropping everything through its last `/`, lowercasing the remainder, then
splitting it into tokens on `/`, `:`, `_`, `-`, `.`, and space (the family
check alone treats `.` as non-splitting, so a dotted version stays one
token). An id is a gemma-4-family member if and only if its tokens contain
`gemma` immediately followed by exactly `4` as two tokens, or the fused
token `gemma4`, or the whole token `gemma.4` — no other spelling counts,
and the version token must be exactly `4` (so a `gemma-4.1-*` successor
id is excluded, not silently absorbed). A non-family id never enters
either tier's candidate set. Within a family id: a whole token `a4b` or a
whole token `26b` puts it in the **light**-tier candidate set (both name
the same mixture-of-experts variant); a whole token `31b` with neither of
those puts it in the **heavy**-tier candidate set; a family id with none
of these tokens is reported separately as unclassified, in neither set.

- A tier fills automatically only when its candidate set holds **exactly
  one** id.
- Zero candidates: refuses, listing any unclassified gemma-4 ids first (if
  any), then the provider's full catalogue (or that it offered none), then
  the `--model-map` syntax.
- Two or more candidates: refuses, listing every candidate and the count,
  then the `--model-map` syntax.
- `--model-map tier=id` entries always win over auto-detection for that
  tier. A mapped id absent from the live catalogue is a warning, not a
  failure. A malformed entry, an unknown tier name, or the same tier named
  twice in one `--model-map` refuses before any network call.
- On success: `config.toml` is updated by the targeted, byte-preserving
  update described in §2.5, and a one-line confirmation (provider, heavy
  id, light id, file written) goes to stderr.
- On any failure, `config.toml` is left completely unwritten.

### 1.4 `kbase survey`

```
kbase survey <corpus-dir> [--json PATH|-]
```

Deterministic, offline corpus inventory — no provider, no configuration
directory. Walks `corpus-dir` for `.md` files, takes them into byte custody
(NFC-normalized; §3.2), and computes per-file heading trees with byte
offsets, section token estimates, the intra-corpus link graph,
first-paragraph gists, front-matter extraction, and the legal cut-candidate
list.

- No `--json`: prints a human summary to stdout in the shape `survey ok:
  files=N bytes=N tokens=N sections=N top-level=N` followed by `links:
  internal=N unresolved=N external=N anchor=N`, and exits.
- `--json PATH`: additionally writes the full artifact (schema
  `kbase.survey/2`, §3.2) to `PATH`; the human summary still goes to
  stdout.
- `--json -`: the artifact goes to stdout instead, and the human summary
  moves to stderr.
- Same corpus bytes always produce a byte-identical artifact — no clock,
  network, or environment dependence anywhere in this verb.
- Refuses if `corpus-dir` does not exist or is unreadable, or if the
  `--json` path cannot be created.

### 1.5 `kbase build` — the delivery verb

```
kbase build <corpus-dir> --out DIR [--title TITLE] [--annex PREFIX]...
                         [--keep-temp-work]
```

Builds a complete, verified knowledge base. Requires **both** tiers to be
configured (§2.2) and refuses before the corpus is read if either is not,
naming the missing one: the heavy tier designs the tree and writes the
summaries, the light tier adjudicates page boundaries.

- `--out` is required; receives the delivered tree, and only the delivered
  tree, under the output-directory contract (§3.1).
- `--title TITLE` is the documentation set's own title as the user states
  it (`--title "Rojo v7 Documentation"`). The knowledge base is titled
  `KBase for <TITLE>`, and that is its entry-point heading, the text of
  every down-link that points at a domain from it, and the start link the
  shipped fixtures carry. Absent or blank, `TITLE` falls back to the
  corpus directory's base name. The resolved title is recorded in
  `run.json` (§3.8) and is an output-affecting input: changing it over an
  existing `--out` re-derives the tree plan rather than resuming the one
  built under the old title.
- `--annex PREFIX` is repeatable. Validation, in order: an empty prefix
  refuses; a prefix that is not a clean, corpus-relative path (no `/`
  prefix, no `.`/`..` segments) refuses; a prefix naming no surveyed
  document refuses; two declared prefixes that are equal or nested refuse
  (declare the outer one only). All annex validation happens before any
  model is dialed.
- `--keep-temp-work` keeps `temp-work/` after a run that succeeds (a
  failed or interrupted run always keeps it regardless), and with it the
  run record (§3.8), which lives at the temp-work root.
- Nothing is delivered unless every one of the ten guarantees (§4.8)
  holds; a failure refuses the whole delivery and names which guarantee
  failed (and, where applicable, the offending node) in the run record
  (§3.8).
- **Page boundaries.** A group the tree plan sized into more than one page
  has its interior boundaries adjudicated by the light tier, one call per
  boundary, against the mechanical splitter's proposal (§3.4). It is a
  refinement seam: a boundary whose answers do not verify keeps the
  mechanical cut, the run record counts it as fallen back, and the
  delivery is unaffected. A corpus whose groups all fit one page has no
  boundary to adjudicate and makes **zero** light-tier calls — the correct
  outcome, recorded as `boundariesAdjudicated: 0` rather than as an absence.
  Under `[dev] tree_plan = "mechanical"` this stage stays mechanical too,
  which is what makes that switch "no model call anywhere".

---

## 2. Configuration files

Both files below live in the resolved configuration directory (§1.1), are
TOML, and are read under the strict-load contract (§2.3).

### 2.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; preferred over apiKeyUnsafe
apiKeyUnsafe = "..."   # inline key; discouraged
type = "inference"     # optional; the only accepted value, also the default
api = "openai"         # optional; the only accepted value, also the default
```

No other key is recognized inside an entry. An entry that fails validation
(empty `baseUrl`, or `type`/`api` set to anything but its one accepted
value) is **dropped** with a warning naming the entry and the reason
(never key material); the rest of the pool still loads. A key-file read
failure faults just that entry the same way. An absent `providers.toml` is
an empty pool, not a load error.

### 2.2 `config.toml` — the choices

```toml
provider = "..."             # active providers.toml entry name

[models]
heavy = "..."                 # model id for the heavy tier
light = "..."                  # model id for the light tier

[dev]                          # optional; every switch defaults off/empty
                                # when the table or the key is absent
telemetry = true|false          # local inference-timing diagnostics (info log level)
keep_temp_work = true|false     # keep temp-work/ after a SUCCESSFUL run too
                                 # (a failed run always keeps it, regardless)
build_date = "YYYY-MM-DD"       # pins the date in every page's provenance
                                 # footer (§4.6); default is today, UTC
tree_plan = "mechanical"        # the only accepted non-empty value: take the
                                 # tree from the corpus's own file/folder
                                 # structure and dial no provider for the run
```

A tier left unset resolves to "not configured" — no defaulting, no
guessing. `kbase build` refuses when either tier is unset, unless
`tree_plan = "mechanical"` puts it on the offline path, where no tier is
read because no provider is dialed. A `tree_plan` value other than empty
or exactly `"mechanical"` refuses at load.

### 2.3 Strict-load contract

Both files are decoded strictly: any key or table not modeled by the
schemas above fails the **whole file's** load. The message names the file
path and every offending key by its full dotted path (e.g.
`models.typo_key`), pluralizing "unknown key"/"unknown keys" when more
than one — `<path>: unknown key "models.typo_key" — check spelling against
the documented schema`. A malformed-TOML file fails with a parser-detail
message naming the file and line. Only key **names** ever appear in either
message — never values, since `providers.toml` may carry credential
material. `kbase configure` cannot repair a config file it cannot load:
every verb loads configuration before doing anything else, so a typo'd key
is a one-line hand edit, never a silent no-op.

### 2.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile` and the host is not
Windows, kbase stats the resolved key-file path. If its permission bits
grant group-read or other-read, it prints a warning naming the path and
the offending mode and recommending `chmod 600` — the entry stays usable;
this is advisory only, and it is the **only** file-permission behavior
kbase applies to a file it did not itself create (contrast §5's
umask-respecting creation-mode guarantee, which is about files kbase
writes).

### 2.5 `configure`'s targeted update (byte-preservation contract)

`kbase configure` never re-serializes `config.toml`; it edits the existing
bytes in place.

- An absent or blank file gets a full commented template, with
  `provider`/`models.heavy`/`models.light` filled in.
- Otherwise, **only** the top-level `provider` key and the `[models]`
  table's `heavy`/`light` keys are ever rewritten. Every other byte —
  comments, unrelated keys and tables (including the entire `[dev]`
  table), spacing, key order — is preserved exactly. A rewritten line's
  own trailing `# comment` is kept, though its column position is not
  guaranteed to match the original (§ Needs ruling).
- A missing `provider` key is inserted just after the leading comment
  block; missing `heavy`/`light` keys are appended inside (or as a new)
  `[models]` table.
- Before writing, kbase decodes both the original file and the proposed
  new one as TOML and refuses the write — untouched — unless the only
  difference between the two decoded structures is exactly the values it
  intended to change. Any other divergence (for instance, a misdetected
  table boundary in a hand-edited file) refuses rather than risks silent
  corruption.
- The write is atomic (temp file, same directory, then rename). A
  `config.toml` that is itself a symlink has its target rewritten; the
  symlink is left in place.
- Any failure along this path — refused verification, disk error,
  unrecognized layout — leaves the file completely unwritten.

---

## 3. On-disk artifacts

### 3.1 The output-directory contract

`--out` receives **only** delivered artifacts, and the delivered set is
exactly the classified one guarantee 5 (§4.8) enumerates: tree-plan nodes
and the fixture manifest, nothing else. Nothing already present in `--out`
is ever touched, so pointing a run at a populated directory is safe. Every
other thing a run produces — stage artifacts, their stamps, the job lock,
the run record (§3.8), the residue of an interrupted write — lives under
`<out>/temp-work/`, a directory kbase creates and the only thing it ever
deletes inside `--out`.

No file kbase writes has a lifecycle of its own: there are exactly two
kinds, delivered (permanent, classified) and temp-work (scratch, one
teardown rule). Nothing is written outside `temp-work/` and later deleted,
and nothing in the delivered tree is exempt from classification.

A run that completes successfully removes `temp-work/`. A run that fails
or is interrupted (including a hard kill) leaves it in place — the next
run of the same `--out` reads it to resume (§5). `--keep-temp-work` (or
`[dev] keep_temp_work = true`) keeps it after a successful run too.

### 3.2 The survey artifact — schema `kbase.survey/2`

```json
{
  "schema": "kbase.survey/2",
  "corpus": {
    "contentHash": "...", "files": N, "bytes": N, "tokens": N, "sections": N,
    "links": {"internal": N, "unresolved": N, "external": N, "anchor": N}
  },
  "files": [ { "...": "..." } ]
}
```

`corpus` is a mechanical roll-up of `files` (every count is the sum of the
per-file entries); `contentHash` is the corpus's content-identity hash.

Each `files[]` entry:

| Field | Meaning |
|---|---|
| `path` | corpus-relative, NFC-normalized, forward-slash separated |
| `sha256` | hash of the file's NFC-normalized custody bytes |
| `bytes`, `tokens` | custody byte length; estimated token count (an estimate — never treated as exact anywhere downstream, §5) |
| `title`, `description`, `tags` | from front matter when present, each word-capped |
| `gist` | a capped first-paragraph summary |
| `metadata` | byte span of a detected YAML front-matter block, when present |
| `preamble` | byte span of headingless content before the first heading, when present |
| `sections` | the heading tree: each node has `level` (1–6; a headingless preamble is level 0 and lives in `preamble`, not here), `title`, `start`/`end` (byte span; `start` is the heading line's first byte), `tokens` (covers the whole subtree), `gist`, `children` |
| `cuts` | the legal cut-candidate list: `{"offset": N, "kind": "heading"\|"fence"\|"paragraph"}` |
| `links` | one entry per distinct destination: `{"kind": "internal"\|"unresolved"\|"external"\|"anchor", "target": "as written", "path": "resolved doc (internal only)", "fragment": "...", "image": true (omitted if false)}` |

**Cut-candidate guarantees** (load-bearing — a validator may rely on all
four): offsets are strictly ascending; every offset is interior (never a
file's first or last byte); at every offset, at least one immediately
adjacent byte is whitespace; a fenced code block contributes only its
outer open/close edges, never an interior offset.

**Tiling guarantee:** `metadata` + `preamble` + `sections` exactly
partition a file's byte length, in that order, with no gap or overlap.

**Determinism:** identical corpus bytes (after NFC normalization) always
produce byte-identical artifact JSON — no field is encoded as a map, and
array order is fixed (corpus-walk order for files, document order for
sections, target order for links).

### 3.3 The tree plan artifact — schema `kbase.treeplan/1`

```json
{
  "schema": "kbase.treeplan/1",
  "corpusHash": "...",
  "budgets": {"leafTokens":N,"summaryInputTokens":N,"summaryTokens":N,
              "entryPointTokens":N,"depthCap":N,"fanOutCap":N,"candidateCap":N},
  "nodes": [ {"...": "..."} ],
  "groups": [ {"...": "..."} ],
  "annexes": [ {"...": "..."} ]
}
```

`budgets` are the shipped, calibrated constants (ARCHITECTURE.md §9) — not
an invocation's to choose. `annexes` is omitted when no `--annex` was
declared.

`nodes[]` — depth-first, every parent listed before its children:
`{"path", "kind": "entry-point"|"index"|"leaf", "parent", "title", "scope",
"group" (leaf only), "part" (split leaves only)}`. Exactly one
`entry-point` node exists, has no parent, and its path is `entry-point.md`.
`scope` is the one-line description rendered in the parent's down-link
list.

`groups[]` — the source span a leaf (or leaf family, if split) draws from:
`{"id", "source": {"file","start","end"}, "budget", "parts"}`. `parts >= 1`;
`parts == 1` means the span is never split. **The interior cut points of a
multi-part group are not recorded here** — only in that group's cut list
(§3.4).

`annexes[]` — `{"prefix", "convention"}`, one per declared `--annex`;
`convention` is the lookup description kbase derives from the survey and
writes into the entry point (§4.3).

**Naming guarantee** (observable — determines delivered paths):
`entry-point.md` at the root; an index's children live under `<slug>/`,
the index itself at `<slug>/index.md`; a leaf is `<slug>.md`; part `k` of
`n` of a split leaf is `<slug>-<k>.md`, titled `"<Title> (k/n)"`. A slug is
derived from a node's own title: runs of whitespace, control and invisible
format characters, dash-class punctuation, and the literal characters
`/ \ : * ? " < > | # % . ( )` collapse to one hyphen; ASCII `A`–`Z`
lowercases (no wider Unicode case-folding — other scripts pass through
unchanged); the result is capped to 8 words, then to 64 bytes (cutting on
a word boundary, never inside a multi-byte character); an empty result
becomes `node`. Within one directory a colliding slug gets an ordinal
suffix (`name`, `name-2`, `name-3`, …) rather than overwriting; `index`
and `entry-point` are reserved and unclaimable by a slug.

### 3.4 The composed cut list

Plain text, one line per span: `<start> <end>\n` (decimal byte offsets
into the source file), in document order, tiling the group's bytes exactly.
One per split group, at `temp-work/cuts/<group>/cutlist.txt`, written by
whichever shape stage 4 ran in — the refined fold's composed list on a live
build, the mechanical splitter's output under `[dev] tree_plan =
"mechanical"`. Never written empty: a fold that composes nothing is an
error, not an empty file. It is a stage artifact and is not delivered.

### 3.5 The summary artifact — schema `kbase.summary/1`

```json
{"schema": "kbase.summary/1", "framing": "...", "conclusionsHeading": "...", "conclusions": "..."}
```

One per index/entry-point node, once the summary stage has run for it.
Never rendered Markdown itself — the delivered page's summary block
(§4.3) is rendered from this artifact at delivery time, so a regenerated
summary can never half-rewrite an already-delivered page.

### 3.6 Stamps — `<artifact-name>.stamp.json`

Every stage artifact under `temp-work/` carries a sidecar of this name,
beside it:

```json
{"schema": 1, "version": "<app version>", "path": "<store-relative path>",
 "inputs": [{"name": "...", "hash": "..."}], "output": "<sha256 of the artifact bytes>"}
```

`inputs` is sorted by name. An artifact is reusable on a later resume
**only** when all of: the artifact and its stamp are both readable and
parse; the stamp's own `path` matches; its `schema` is the version kbase
currently writes; its `version` matches the running app's version exactly;
`output` matches a fresh hash of the artifact bytes; every declared
input's hash still matches. Any single mismatch — including app-version
skew — means the artifact is redone, never trusted.

### 3.7 The job lock — `temp-work/job.lock`

Acquired exclusively at job start (create-if-absent; no read-then-write
race window) and released on a normal exit. Contents are plain text and
informational only: `pid`, `host`, `started` (RFC3339), `version`. A
second `build` pointed at the same `--out` while the lock exists refuses
immediately, naming the lock's contents and the file to delete.

**After a hard kill** (SIGKILL, power loss, container eviction) the lock
file is left behind and is **never broken automatically** — there is no
pid-liveness check and no age timeout, since neither is meaningful across
hosts or containers. The sole remedy is a human deleting
`temp-work/job.lock`; after that, both a resumed run and a full rebuild
proceed normally.

### 3.8 `run.json` — the run record

**`kbase build`**, written to `<out>/temp-work/run.json` — the temp-work
root, which mirrors the delivered tree's root — on every completed attempt
(delivered or refused). It is a development record, so it lives in the dev
mirror rather than in the delivered tree, and it is therefore subject to
§3.1's teardown: a successful run without `--keep-temp-work` keeps no run
record. Contents: `version`, `corpus` (path), `title` (the resolved
doc-set title, §1.5), `corpusHash`, `buildDate`, `treePlan` (`"model"` or `"mechanical (dev)"`), `annexes`
(omitted if none), `budgets` (§3.3's object), `sourceFiles`,
`sourceSections`, `nodes`, `pages`, `sections`, `groups`, `splitGroups`,
`summaries`, `deliveredFiles`, `units`, `unitsProduced`, `unitsReused`,
`elapsedMs`, and `verify`: exactly ten `{"number", "name", "ok"}` entries,
one per guarantee in §4.8, always in the same order. A live (model-in-the
-loop) run additionally carries a `live` object naming: `provider`,
`baseUrl`, `model` and `tier` (the heavy tier's, which designed the tree
and wrote the summaries), `lightModel` (which adjudicated the page
boundaries), `thinking` and `retryThinking` (the heavy definitions'
declared efforts), `taxonomyCalls`, `callsSkipped`,
`boundariesAdjudicated`, `boundariesMoved`, `boundariesFellBack`,
`boundaryRejections`, `promptTokens`, `cachedTokens`, `completionTokens`,
`reasoningTokens`. The four boundary counts are summed over every split
group and are all zero when no group split.

### 3.9 `temp-work/` layout

Mirrors the delivered tree's own relative paths one level down
(`<out>/temp-work/<relative-path>`), plus stage-named siblings for
artifacts that are never themselves delivered: a survey directory, a tree
-plan directory, a per-domain leaves area, a pre-delivery render of the
tree, a verify-report directory (schema `kbase.verify/1`), and `job.lock`
and `run.json` (§3.8) at the root. Every file in it carries a
`.stamp.json` sidecar (§3.6) except `job.lock` and `run.json`, which are
not stage artifacts.

---

## 4. The generated-KB artifact contract

### 4.1 Node kinds

Three kinds, always: **entry-point** (exactly one, at the tree's root),
**index** (a routing page), **leaf** (a verbatim page — possibly one of
several parts of one source span, when the span exceeded the leaf token
budget at tree-design time).

### 4.2 Leaf page layout

In order:

1. The frontmatter block (§4.9), then a blank line.
2. The up-link (§4.4), then a blank line.
3. A synthesized `# <Title>` heading — **unless** the leaf's own first
   non-blank line already opens with a Markdown ATX heading (`#…`), in
   which case none is synthesized (a leaf cut at a `##` boundary opens
   with `##` and gains no redundant `#`; navigation elsewhere always uses
   the node's own title regardless).
4. The verbatim body: byte-identical to the corresponding source span,
   with only link **destinations** rewritten (§4.5) — visible link text
   is never altered.
5. A trailing newline, added if the body lacks one.
6. The provenance footer (§4.6).

### 4.3 Index and entry-point page layout

**Index page**, in order: frontmatter block (§4.9) → up-link → `# <Title>` →
optional summary block (present iff a summary artifact exists for the
node: framing text, a rule, `## <conclusionsHeading>`, conclusions text, a
rule) → `## Derivations and Detail` heading, then one down-link bullet per
child in tree order (`- [<ChildTitle>](<relative-path>) — <ChildScope>`) →
provenance footer.

**Entry-point page** (no up-link — it has none; the frontmatter block is
first all the same), in order: frontmatter block (§4.9) → `# <Title>` →
optional summary block (same shape) → `## Domains` heading, one down-link
bullet per direct child (same bullet grammar) → `## Using this knowledge
base` heading, with the fixed contract text — the invariant, a blank
line, then the routing statement: *"INVARIANT: Answers come exclusively
from the text of one or more leaves. Intermediate nodes are for
navigation only."* / *"Summaries route; leaves answer. An index page
tells you where to go; a page at the bottom of the tree is where the
answer is. Answer from leaf text, never from an index summary — the
summaries are navigation, and they are not the source."* —
then a fixed pointer to `.agents/` → optional `## Annex lookup` section
(present iff at least one `--annex` was declared: one bullet per annex,
`` - `<prefix>` — <convention> ``) → provenance footer.

### 4.4 Up-links

Exact literal format:
`[↑ <root-relative path of parent>](<relative path to parent>)` — the
glyph is U+2191 (↑), one space after it inside the brackets. The label is
the parent's **root-relative delivered path**, not its title:
`[↑ getting-started/porting-tools/index.md](../index.md)` at any level,
`[↑ entry-point.md](../entry-point.md)` at the top.

Label and target are two projections of one tree-plan fact and must name
the same node: resolving the target against the page's own directory
yields the label, exactly (guarantee 2 in §4.8).

Every class-A page (leaf or index) but the entry-point carries **exactly
one** up-link, and it is the **first line under the frontmatter block**,
nothing but the block before it. The entry-point carries none.

### 4.5 Link-target rewriting (rebase)

Only link **destinations** are ever rewritten; visible link text is always
byte-identical to the source. Three rules, in order:

1. A link whose destination names a source heading's fragment lands on
   the leaf page hosting that heading's start offset, fragment preserved.
   If no leaf holds it (the section became a container, not a leaf), it
   falls through to rule 2 while keeping the fragment.
2. A link to a whole source file lands on that file's own leaf if it has
   exactly one, otherwise on the lowest index page whose subtree covers
   every node drawn from that file.
3. A link to a file no delivered node was drawn from is left exactly as
   written in the leaf body, and its destination is recorded as
   unresolved — guarantee 1 in §4.8 exempts these rather than treating
   them as broken.

Fragment-to-heading matching is best-effort (a generic slug of the heading
text: lowercase, letters/digits kept, everything else collapsed to one
hyphen); a miss costs a hop of precision (falls through to rule 2) and
never produces a dead link.

### 4.6 Provenance footer

Exact literal format, the last non-blank line of every delivered class-A
and class-B page:

```
<!-- built from corpus sha256:<hash> @ <YYYY-MM-DD>; kbase <version> -->
```

`<hash>` is the survey's corpus content hash; `<YYYY-MM-DD>` is the build
date (today, UTC, unless pinned by `[dev] build_date`); `<version>` is the
app version that produced the KB.

### 4.7 The fixture manifest

Delivered with every KB, exact set:

```
AGENTS.md
README.md
CLAUDE.md
.agents/docent.md
.agents/maintainer.md
.agents/README-ADAPTATION.md
```

- `AGENTS.md` — states the summaries-route/leaves-answer contract, a
  pointer to the entry point, and a pointer to `.agents/`.
- `README.md` — short human orientation: what this is, where to start,
  that agents should read `AGENTS.md` first.
- `CLAUDE.md` — the bootstrap pointer an agent session rooted at the KB
  picks up on its own: read `AGENTS.md` first, start at the entry point,
  then three directives — the no-crawl invariant verbatim as the entry
  point states it, *"Navigate, don't crawl: follow the tree from the
  entry-point instead of grepping the file set"*, and *"Stop reading when
  the question is answered"*. The invariant is duplicated here rather
  than only pointed at: the hop to `AGENTS.md` is probabilistic, and this
  file is what a session picks up whether or not it takes that hop. No
  directive carries a rationale.
- `.agents/docent.md` / `.agents/maintainer.md` — navigate-this-KB and
  extend-this-KB agent definitions (currently shipped as stubs, §6).
- `.agents/README-ADAPTATION.md` — a note, not an agent definition:
  states that these are copies for adaptation, that
  kbase itself never reads them, and recommends using a stronger model to
  write a version suited to its own capabilities.

Every fixture file carries the same provenance footer (§4.6) as a
tree-plan node; the three definition fixtures additionally open with a
fixed stub notice today.

### 4.8 The ten delivery guarantees

Nothing reaches `--out` unless every one of these holds; a single failure
refuses the **whole** delivery (no partial tree, no warnings-only mode),
and the run record (§3.8) names which guarantee failed and, where
applicable, the offending node.

1. Every file any delivered page links to (excluding destinations already
   recorded as unresolved) exists in the delivered set.
2. Every class-A page but the entry-point carries exactly one up-link, as
   the first line under its frontmatter block, and that up-link's label
   and target resolve to the same node (§4.4); the entry-point carries
   none.
3. The entry-point exists, and every domain it names in `## Domains` was
   itself delivered.
4. Every class-A page is reachable from at least one other class-A page
   (excluding self-links); every fixture file links to the entry-point.
5. The delivered file set is exactly the tree plan's nodes plus the fixed
   fixture manifest — no stray files, none missing.
6. Coverage: every section the survey found is accounted for by exactly
   one delivered page or by a declared annex — nothing silently dropped,
   nothing double-covered.
7. Leaf fidelity: every leaf's delivered bytes are byte-identical to a
   fresh re-derivation from the immutable source corpus.
8. Structural caps hold: index depth, any node's fan-out, and the
   entry-point's own rendered size are within the calibrated budgets
   (§3.3).
9. Every page's rendered bytes conform to its kind's exact grammar
   (§4.2/§4.3), and every fixture file matches its own fixed template.
10. Every class-A page opens with a frontmatter block whose `Location`
    is exactly the path it was delivered under (§4.9).

### 4.9 Page frontmatter

Every class-A page — leaf, index and entry-point alike — opens with this
block, at file-start, before the up-link:

```
---
Location: <root-relative delivered path>
---
```

`Location` is the page's own delivered path, the same string guarantee 5
enumerates and guarantee 10 compares against. Class-B fixtures carry no
frontmatter: their names are root-obvious and `.agents/` is exempt by
design.

kbase writes exactly this one field. Any reader of a delivered page — the
verify gate today, a later kbase pass over an existing KB — is
**forgiving**: it validates the fields it knows and ignores the rest, so a
later version may add a field without invalidating pages an earlier one
delivered. The block's own shape is not forgiven: a first line that is not
the `---` fence, or a fence that never closes, is no block at all and
fails guarantee 10.

H1 headings remain purely subject-matter titles. A page states its address
in this block and in its up-link label, and nowhere else.

**Leaf edge case, intentional.** A verbatim slice that itself contains the
SOURCE document's frontmatter renders it as body text: this block owns
file-start, the envelope wraps the body below it, and body bytes are never
edited (§4.2 item 4).

---

## 5. Behavioral guarantees

- **Determinism.** Leaves never involve a model (verbatim body plus
  mechanical wrapping) and are therefore always byte-deterministic. Given
  identical corpus bytes and an identical resolved configuration (model
  IDs, annex declarations, `[dev] build_date`), `kbase build` delivers a
  byte-identical tree across runs whenever the tree plan is mechanical
  (`[dev] tree_plan = "mechanical"`). A live, model-in-the-loop run's tree
  design and summaries additionally depend on the provider reproducing
  its own output for one prompt across runs — kbase does not control, and
  does not claim, that.
- **Fail-loud, no partial delivery.** `kbase build` either delivers a
  complete, gate-passing tree or delivers nothing new to `--out`.
  `kbase configure` either writes a complete, valid update to
  `config.toml` or writes nothing.
- **Refuse-and-split over truncation.** Any input that would overflow a
  per-call or per-page budget is refused back to the stage that can
  resize it, never silently truncated or evicted. Token counts feeding
  these checks are estimates, with headroom built into every threshold an
  estimate feeds — an estimation error costs one extra loud refusal,
  never a wrong answer.
- **File and directory permissions.** Every file or directory kbase
  itself creates (the delivered tree, stage artifacts and stamps,
  `temp-work/`, run records, the job lock, log files, `config.toml`) is
  created at the maximally-permissive mode (0666 files / 0777
  directories) and left to the process umask to narrow — never `chmod`'d
  after the fact, which would ignore the umask. This is unrelated to, and
  does not affect, the key-file warning in §2.4, which is advisory over a
  file kbase never itself creates.
- **Resume and interruption.** `kbase build` always attempts to resume
  from whatever a prior run of the same `--out` left in
  `temp-work/`: an artifact is reused only when it and its stamp
  affirmatively prove they are still valid for the current inputs (§3.6);
  anything else is redone. A run that fails or is interrupted, including a
  hard kill, always leaves `temp-work/` in place for the next attempt to
  read; a run that completes successfully removes it (unless kept, §3.1).
  There is currently no flag to force a full rebuild short of deleting
  `temp-work/` (or `--out` itself) by hand — see "Needs ruling".
- **The job lock and hard-kill recovery.** See §3.7; the sole remedy after
  an unclean kill is deleting `temp-work/job.lock`.
- **Provenance.** Every delivered page names, in its footer (§4.6), the
  exact corpus content hash and build date it was produced from, plus the
  app version. `run.json` additionally names the resolved provider, model
  and tier for a live run. No token-estimator-calibration source is
  currently stamped anywhere in a run record (§6).

---

## 6. Status markers

Behaviors described elsewhere in this document or in ARCHITECTURE.md that
are not yet true of the current binary:

- **Prompt/definition dump command.** The escape-hatch design calls for a
  CLI command that writes local, adaptable copies of the embedded agent
  definitions. No such command exists today (`kbase --help` lists only
  `build`, `configure`, `models`, `survey`). **PLANNED.**
- **Embedded agent/prompt definitions.** The pipeline runs real model
  calls today, but every prompt definition — taxonomy design, summaries,
  boundary refinement, and the three `.agents/*.md` fixtures — is a marked
  stub; tuned, evaled definitions have not landed. A build today is
  evidence about the machinery, not about tree-design or summary quality.
- **Routing-eval question generation.** No routing-eval question set is
  produced or delivered. The empty `.agents/routing-eval.json` placeholder
  a KB used to ship was removed (ruled 2026-08-15): it had no producer and
  no consumer, and a dev-grade file has no home in the delivered tree. Its
  home is decided when the generation stage lands. **PLANNED.**
- **Chars-per-token calibration.** The token estimator uses a fixed,
  provisional constant; the configure-time calibration call and the
  usage-based refinement are not implemented, and no calibration source is
  stamped anywhere in a run record.
- **Summary review / regeneration.** The find-the-discrepancy review pass
  and flag-driven regeneration do not run as part of `kbase build` today;
  a delivered summary is never reviewed before delivery.

---

## Needs ruling

Behaviors observed in the current binary that look like implementation
accident rather than deliberate contract — flagged rather than enshrined:

1. **`--fresh` is referenced but does not exist.** Internal error messages
   (job-lock contention, stamp mismatches) tell the user to "rerun with
   `--fresh`", but no verb registers such a flag; `kbase build` always
   resumes automatically from an existing `temp-work/`, and the only way
   to force a full rebuild is to delete `temp-work/` (or `--out` itself)
   by hand. Either the flag should be added, or the error text should
   name the actual remedy.
2. **`configure`'s targeted rewrite does not preserve original comment
   alignment.** A rewritten line's trailing comment is kept but
   re-attached at a fixed gap, not its original column. Whether §2.5's
   byte-preservation contract is meant to guarantee column-exact comment
   alignment, or only comment *content* preservation, is undecided.
3. **No disambiguation for duplicate headings in fragment rewriting.**
   When a source document repeats a heading title, §4.5 rule 1 resolves to
   whichever occurrence it reaches first, with no suffix-based
   disambiguation. This degrades gracefully (falls through to a
   whole-file link, never a dead link) but is not a stated guarantee.
4. **`[dev]` switches are documentation-only guardrails, not enforced
   isolation.** Nothing stops a user from pointing a real, live
   `--config-dir` at `[dev] tree_plan = "mechanical"`, or otherwise using
   any `[dev]` switch outside a development context. Whether that is
   acceptable for v1, or should be gated further, is unruled.
