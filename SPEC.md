# SPEC – kbase

The contract: what any compliant implementation of kbase must do. Consumer-facing
outcomes only. [ARCHITECTURE.md](ARCHITECTURE.md) states how this implementation meets
it; [THESIS.md](THESIS.md) states why the build has its shape.

**Status vocabulary.** An unmarked statement is decided. **Pending slice** marks a
mechanism whose design waits on the first implementation cut (ARCHITECTURE, "Pending
slice"); the seam and the kb_tools module it ports are named where it appears. **Needs
ruling** marks a point the design leaves open; §10 lists them all.

---

## 1. What kbase is

kbase is a Go binary that builds and maintains a knowledge base (KB) from LaTeX. Volume
roots go in, a KB comes out, and a set of maintenance subcommands then operates on the
living KB.

- A **volume root** is the top `.tex` file of a paper: the file `00README.json`'s
  `toplevel` entry names, else the sole file containing `\documentclass`.
- A **KB** is a `kb-root/` directory beside a repository's `.git`: the Markdown
  document tree, its metadata layer, and `.index/`.
- The **metadata layer** is what kb_tools' write API renders into a KB: the frontmatter
  comment block, registers, Tier-2 markers, `.index/*.jsonl`.
- The **load-bearing forms** are the Markdown shapes KB readers key on: the up-link line
  (kb_tools `UPLINK_MARKER`), the cross-reference anchor form, the `` ``` math `` fence,
  the citation span, and the labelled blockquote (kb_tools SPEC, "The Document-Tree
  Contract", points 3, 7, 9, 10, 12).

kbase builds exactly what kb_tools builds. Input is LaTeX only; no other input format is
in scope. Two consumers use the binary: a person at a shell, and personant (a sibling Go
project) whose model calls kbase subcommands as builtin tools. kbase is personant's
peer; personant's rules govern personant.

---

## 2. Given interfaces

| Interface | Observed version | Contract |
|---|---|---|
| kb_tools (reference implementation, `adjagent/kb_tools/`) | adjagent commit `88ad25fdcccafdf608e09b8c360155c4b2c44eee` | Every kb_tools citation in this repository's documents names this commit. Sections cited below are kb_tools' SPEC unless noted. |
| pandoc | 3.12 (`pandoc-api-version` 1.23.1.2) | Required on the host, not shipped. |
| git | 2.56.0 | Required on the host. The recoverable record of a build (§6). |

**pandoc.** The accepted range's lower bound is 3.12. Its upper bound is the next major
`pandoc-api-version`, exclusive, where major means the first two components under
Haskell's versioning policy: 1.23.x is accepted, 1.24 is not. Only the owner widens the range. The
range and the `pandoc-api-version` are enforced at preflight; a missing pandoc is
refused with an error naming pandoc's install page. kbase runs pandoc LaTeX to JSON with
`--citeproc` and no filters.

**kb_tools' KB contract**, cited by section at the pinned commit:

- "What a KB Is"
- "The Document-Tree Contract"
- "Claim-Graph Nodes and Edges"
- "Derived Metadata, Defined"
- "The Claim-Graph Sheet"
- "Citation Grammar"
- "Project Scoping"
- "The Write API's Contract"

kbase restates none of them. A KB kbase produces satisfies them except where §4 names a
divergence.

---

## 3. Compatibility contract

- A KB kbase builds is navigable by kb_tools and the kb-docent agent (Claude Code or
  opencode) and maintainable by kb-maintainer.
- A KB kb_tools builds is fully queryable and modifiable by kbase.
- kb_tools' build-pipeline intermediates are out of scope.
- Exact: the metadata layer, the load-bearing forms, and the KB's placement (`kb-root/`
  beside `.git`; the docent starts at `kb-root/entry-point.md` and descends through
  `index.md` files).
- Free and untested: presentation — wrap width, bullet characters and the like.
- Byte equality is asserted only where a named reader needs it:

| Byte-equal | Reader that needs it |
|---|---|
| Metadata rendering (register entries, frontmatter, markers, derived-field placeholders) | kb_tools' parser; refresh's placeholder recognition; `kb_write`'s locate/splice |
| `.index/*.jsonl` and derived fields | Each toolchain's freshness gate (dry-run refresh diffed against disk) |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | kb_tools' `kb_index_lib.unmigrated_agents_file` (refresh and verify exit 2 otherwise) |
| The `` ``` math `` fence opener | kb_tools' fence scanner |
| The up-link line | `UPLINK_MARKER` |

**Acceptance bar.** Each toolchain's own checks run green over the other's output, with
one named exception: kb_tools' sheet freshness check on a kbase-written placeholder
(below) is red until kb_tools' refresh runs.

**Formats.** A new KB keeps kb_tools' metadata formats. JSON versus YAML is a load-time
parser choice over identical schemas. Breaking format changes wait until kbase is proven.

**Theorem numbering** is whatever pandoc prints, as in kb_tools.

### `claim-graph.svg`

A placeholder: an SVG showing "NYI" and, beneath it, a digest of the index it was
generated from: `index sha256:` followed by the first 12 hex digits of the SHA-256 of
the `.index/*.jsonl` files concatenated in sorted path order. The output is
deterministic. A sheet is kbase's own when it is that placeholder (it carries the "NYI"
text). `kbase refresh` writes it only where no sheet exists or the existing one is
kbase's own; `kbase verify` freshness-checks only kbase's own sheet and leaves a
kb_tools-drawn sheet unchecked. No other command's behaviour depends on the sheet's
contents (ARCHITECTURE §6).

---

## 4. Named divergences

A named divergence is a difference from kb_tools that this list names. This is the whole
list of differences in KB contents, maintenance-op behaviour and the tool interface; any
unlisted difference there is a defect. Build-orchestration differences are the "Not
ported" list in ARCHITECTURE §4.

| Divergence | kbase | kb_tools |
|---|---|---|
| Values transport | YAML (JSON accepted) on stdin or `--values` (§8) | A TOML file |
| Exit codes | The table in §7 | kb_tools' own codes (e.g. write ops: 7 refused, 8 retry) |
| Dead-link gate scope | `kb-root/` only | The whole repository |
| Build-state location | Outside the worktree (§6) | kb_tools' own run directory |
| `claim-graph.svg` | The placeholder in §3 | The drawn sheet |
| Stamped KB documents (`AGENTS.md`, `CONVENTIONS.md`, `README.md`) | Give the maintenance commands for both toolchains: kbase's subcommands and kb_tools' `kb_util` ops | Give kb_tools' only |
| Tool results | YAML documents (§7) | `[kb-write] STATUS` report lines |
| Node-pass record | YAML (`kb-build-node-pass.yaml`) | `kb-build-node-pass.json` |
| README overview passage | Until the owner rules on the overview stage's input (§10), comes from mechanically assembled input | A seat reads the KB tree |

---

## 5. Build behaviour

```
kbase build <volume-root> [--bibliography FILE]... [--charter FILE] [--no-inference]
            [--through <stage>] [--state-dir DIR]
```

**Output and placement.** The KB is written only at `<git root>/kb-root/`. Build state
is never written into `kb-root/`.

**Fresh versus resume** is derived from the `kb-build:` commit trail (§6), never
configured. A build against a populated `kb-root/` with no `kb-build:` commit trail is
refused. `build` refuses outside a git worktree, naming `git init`. It refuses if the
kbase-owned paths it commits are dirty, and only then.

**Determinism.** The same inputs and pandoc version give a byte-identical `kb-root/` and
byte-identical records.

**`--no-inference`** is first-class. It drops the rows that spend inference, still walks
and records every stage, and states the rows it dropped. The run is a real KB built
without those rows, not a stopped walk. Which rows drop follows kb_tools' classification
(kb_tools ARCHITECTURE, "The Driver": `--no-inference` "drops rows and bounds nothing",
`spends_inference`; SPEC, "The Driver's Contract").

**`--through <stage>`** bounds the run at the named stage. A bounded run is not a failed
one: outcome `bounded`, exit 0 (§7).

**Refusals** (outcome `refused`):

- Unparseable source: names the paper and the reader error.
- Unloadable include: names each file and the line that named it.
- An unclassified metadata key: a pandoc metadata key outside kb_tools' content and
  apparatus lists (`kb_docgraph/outline.py`).

**Degradation, not refusal.** An unreadable bibliography: the build continues without it
and reports the file.

**Monitoring.** `build` runs in the foreground; personant backgrounds it. `kbase status`
and `kbase cancel` (§6) observe and stop it.

**What a build produces** is kb_tools' output, as listed in the cited sections of §2:
Markdown leaves; the index, up-link and cross-reference structure; load-bearing forms;
claim-graph registers and nodes; the derived index. The build's stages and what each
does are in ARCHITECTURE, "Stage table". How each stage computes its product is
**Pending slice** except where ARCHITECTURE states it.

**Records and inference.** Reader facts the claim graph needs travel as records in the
build state store, not in `kb-root/` (ARCHITECTURE, "Records"). Each inference pass uses
kb_tools' prompt, fragments and seat definition unchanged (ARCHITECTURE, "Inference
seams"); no stage exits on a model's opinion.

---

## 6. Build state, ledger and monitoring

**Git holds the recoverable record.** Each stage boundary is a commit in the user's
repository, with subject `kb-build: <stage-id> | <display name>` and a structured body.
A commit carries `kb-root/` and the tracked build records, at kb_tools' paths at the
repository root: `kb-build-charter.md` and the node-pass record
`kb-build-node-pass.yaml` (kb_tools' `kb-build-node-pass.json`, in YAML). Resume position is read from the commit trail. A user may reset to a stage
commit and resume from there. Per-leaf work inside an inference stage stays uncommitted
until the stage boundary. Commits are scoped by pathspec to kbase-owned paths.

**The state store holds what does not belong in the repository** — run lock and pid,
`progress.jsonl`, per-call evidence, the docgraph records (§5), and a per-unit answer
cache keyed by input hash. Nothing in it is authoritative for position. The store lives at
`$XDG_STATE_HOME/kbase/<key>/`, where `<key>` is the first 16 hex digits of the SHA-256 of
the absolute, symlink-resolved `kb-root/` path, overridable with `--state-dir`; it holds
paid-for inference and survives `git clean`.

**Maintenance operations never require git.**

### `kbase status`

Returns a YAML document with: state (`none`, `running`, `cancelled`, `failed`,
`bounded`, `finished`); pid; timestamps; each stage recorded or not, with its commit; the
current stage's units done and total; recent refusals and fallbacks; the
`--no-inference` flag; and the resume command. Progress is persisted to a file so a late
or reconnecting monitor catches up.

### `kbase cancel`

Signals the lock holder. The in-flight call is abandoned with nothing written for it;
the event is logged, the lock is released, and the build process exits `cancelled`.
Cancel costs only the in-flight unit; the build is resumable.

---

## 7. Tool-result contract and exit codes

**Tool results.** Every subcommand, `models` and `configure` included, writes one YAML document to
stdout and nothing else. stderr is for humans. Keys are in fixed order; strings are
quoted on emit; a refusal enumerates every offending item. The `outcome` key is the
contract. `--help` and `--version` are human-facing and outside it. The key set per subcommand beyond `outcome` is **Needs ruling**.

**Exit codes** derive from `outcome`:

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, including a `--through`-bounded run |
| `refused` (including verify findings and usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | Concurrent writer or live lock; re-issue identically later |
| `failed` | 3 | Defect in the tool, its input or the environment; re-issuing will not fix it |
| `cancelled` | 4 | Stopped on request; resumable |

---

## 8. Maintenance subcommands

Every subcommand below is a builtin tool personant can call. kb_tools' semantics govern
each, cited from the sections named; kbase's differences are only those in §4 and the
rules in this section.

**Values input.** YAML (JSON accepted) on stdin or `--values`. The per-op key vocabulary
is closed. It is kb_tools' `kb_write` `values.OP_FIELDS`, and kbase keeps no prose copy of
it. A refusal names the key, its position, and the allowed set. A prose value containing
U+0008 or U+000C is refused.

**Idempotence.** Every mutating op is idempotent under re-issue: the same values issued
twice leave `kb-root/` byte-identical to issuing them once, and the second call reports
`unchanged`. Inserts adopt an existing (register, title, host) entry.

**Writes leave nothing stale.** A mutating op ends with a refresh unless `--no-refresh`
is given.

**Batches** are all-or-nothing, as in kb_tools SPEC, "The Write API's Contract".

### Write ops

Each is named after kb_tools' `kb_util` op and follows "The Write API's Contract".
Their result keys are **Needs ruling**.

### `insert-claim-entry`
### `insert-support-entry`
### `insert-experiment-entry`
### `insert-work-entry`
### `set-work-strength`
### `set-applicability`
### `set-rigor`
### `set-rationale`
### `add-depends-on`
### `set-frontmatter`
### `mark-claim-in-leaf`
### `set-on-point-fraction`

The register-creating inserts take `--create` where kb_tools' op takes it.

### `render-citation`

Reads and writes nothing; prints the sanctioned authority-citation string
(kb_tools SPEC, "The Write API's Contract"; "Citation Grammar").

### Queries

Each is named after a kb_tools `kb_cmd` query and has its semantics (kb_tools
ARCHITECTURE, "Query Surface"; `kb_cmd/cli.py`). Over a kb_tools-built KB, each returns what `kb_cmd
--json` returns, as data.

### `deps`
### `gated-on`
### `cited-by`
### `find`
### `referenced-by`
### `solidity-below`
### `subtree`
### `show`
### `weak-points`
### `stats`

### `render-claim-graph`

Renders the claim-graph sheet from the index (§3); kb_tools semantics: "The Claim-Graph
Sheet". Result keys: **Needs ruling**.

### `refresh`

Derives the metadata layer's derived fields and `.index/*.jsonl` and writes
`claim-graph.svg` (§3), per kb_tools SPEC, "Derived Metadata, Defined". Result keys:
**Needs ruling**.

### `verify`

Checks the KB's freshness, links and citations under the same contract. Findings are
outcome `refused`. The dead-link gate scope is `kb-root/` (§4). `verify` checks the
placeholder sheet fresh (§3). Result keys: **Needs ruling**.

### `status`, `cancel`

See §6.

---

## 9. Configuration

Two TOML files in the resolved configuration directory, read under the strict-load
contract (§9.3).

**Global flags.** Every subcommand accepts `--config-dir DIR`, `--log-level
debug|info|warn|error` (default `warn`), `--log-file PATH` (tees diagnostics to a file;
console output is never redirected away) and `-h`/`--help`; the root command also accepts
`-v`/`--version`, which prints exactly the version string and a newline. The directory is
`--config-dir`, else `$KBASE_CONFIG_DIR`, else `~/.config/kbase`.

**Provider commands.** `kbase models [--provider NAME] [--timeout DURATION]` lists the
selected provider's model identifiers, sorted ascending, in its result document (key names:
**Needs ruling**). `kbase configure
[--provider NAME] [--model-map heavy=ID,light=ID] [--timeout DURATION]` assigns a model
id to each tier and writes `config.toml` under §9.5. Each writes its result under §7. Provider selection for both is
`--provider`, else `config.toml`'s `provider`, else the sole `providers.toml` entry;
zero usable providers refuses naming `providers.toml`, and two or more with none
selected refuses listing every entry name.

### 9.1 `providers.toml` — the endpoint pool

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

No other key is recognized inside an entry. An entry that fails validation (empty
`baseUrl`, or `type`/`api` set to anything but its one accepted value) is **dropped** with
a warning naming the entry and the reason (never key material); the rest of the pool
still loads. A key-file read failure faults just that entry the same way. An absent
`providers.toml` is an empty pool, not a load error.

### 9.2 `config.toml` — the choices

```toml
provider = "..."             # active providers.toml entry name

[models]
heavy = "..."                 # model id for the heavy tier
light = "..."                  # model id for the light tier
```

A tier left unset resolves to "not configured" — no defaulting, no guessing. There is no
`[dev]` table; an old `[dev]` key fails strict load (§9.3).

### 9.3 Strict-load contract

Both files are decoded strictly: any key or table not modeled by the schemas above fails
the **whole file's** load. The message names the file path and every offending key by
its full dotted path (e.g. `models.typo_key`), pluralizing "unknown key"/"unknown keys"
when more than one. A malformed-TOML file fails with a parser-detail message naming the
file and line. Only key **names** ever appear in either message, never values, since
`providers.toml` may carry credential material. `kbase configure` cannot repair a config
file it cannot load: every subcommand loads configuration before doing anything else.

### 9.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile` and the host is not Windows, kbase stats
the resolved key-file path. If its permission bits grant group-read or other-read, it
prints a warning naming the path and the offending mode and recommending `chmod 600`.
The entry stays usable. This is the only file-permission behaviour kbase applies to a
file it did not create.

### 9.5 `configure`'s targeted update (byte-preservation contract)

`kbase configure` never re-serializes `config.toml`; it edits the existing bytes in
place.

- An absent or blank file gets a full commented template, with `provider`,
  `models.heavy` and `models.light` filled in.
- Otherwise **only** the top-level `provider` key and the `[models]` table's
  `heavy`/`light` keys are rewritten. Every other byte — comments, unrelated keys and
  tables, spacing, key order — is preserved exactly. A rewritten line's own trailing
  `# comment` is kept; its column position is not guaranteed.
- A missing `provider` key is inserted just after the leading comment block; missing
  `heavy`/`light` keys are appended inside (or as a new) `[models]` table.
- Before writing, kbase decodes both the original and the proposed file as TOML and
  refuses the write, leaving the file untouched, unless the only difference between the
  two decoded structures is exactly the values it intended to change.
- The write is atomic (temp file in the same directory, then rename). A `config.toml`
  that is a symlink has its target rewritten; the symlink stays.
- Any failure along this path leaves the file completely unwritten.

---

## 10. Needs ruling

| Item | Seam |
|---|---|
| The writer route (pandoc's gfm writer over the transformed JSON with `--wrap=none`, or a Go writer) | Decided at the first slice gate: pandoc's writer unless the slice finds a load-bearing form it cannot produce, in which case the owner rules. |
| Per-subcommand result keys beyond `outcome` (§7, §8) | Designed after the slice. |
| The overview stage's input | kb_tools' overview stage asks a seat to read the KB tree, which a one-shot call cannot. Until ruled, kbase builds the stage from mechanically assembled input (entry point, index child lists, volume overview leaves) and a stub prompt held as an embedded template file. |
