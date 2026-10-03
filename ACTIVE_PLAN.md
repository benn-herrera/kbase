# ACTIVE PLAN – port the kb_tools KB toolchain into kbase

**Status:** approved. **Step 1 done** (contract documents committed; recorded
inputs: adjagent `88ad25fdcccafdf608e09b8c360155c4b2c44eee`, pandoc 3.12,
`pandoc-api-version` 1.23.1.2). **Next: owner review of the Step 1 commit**,
then Step 2 (the slice), after owner launch chores 1–2 (§4). §8 onward is a skeleton: it is **not
executed** and is re-planned when the slice reports. Once Step 1 lands, the
contract documents govern on any disagreement with this plan, and this plan
is corrected.

---

## 1. Terms

| Term | Meaning |
|---|---|
| **kbase** | This repository: a Go binary that builds and maintains a knowledge base (KB) from a LaTeX document set. |
| **kb_tools** | The reference implementation, a working stdlib-Python toolchain at `adjagent/kb_tools/` in the sibling `adjagent` repository (`/Users/agent-user/projects/adjagent`). Its testing tree is `adjagent/kb-testing/`. |
| **personant** | A sibling Go project (`/Users/agent-user/projects/personant`) whose model calls kbase subcommands as builtin tools. |
| **owner** | The project owner, reached in the coordinating session. Decisions marked *owner* wait for them. |
| **KB** | A `kb-root/` directory beside a repository's `.git`: the Markdown document tree, its metadata layer, and `.index/`. |
| **metadata layer** | What kb_tools' write API renders into a KB: the frontmatter comment block, registers, Tier-2 markers, `.index/*.jsonl`. |
| **load-bearing forms** | The Markdown shapes KB readers key on: the up-link line (kb_tools `UPLINK_MARKER`), the cross-reference anchor form, the `` ``` math `` fence, the citation span, the labelled blockquote (kb_tools SPEC, "The Document-Tree Contract", points 3, 7, 9, 10, 12). |
| **named divergence** | A difference from kb_tools that kbase's SPEC lists by name. Any unlisted difference is a defect. |
| **the slice** | Step 2: one thin path through real input, run in three stages, **Slice-1**, **Slice-2**, **Slice-3**. |
| **the tail** | Everything after the slice (§8): a skeleton, re-planned when the slice reports. |
| **claim-graph stages** | kb_tools' own stage letters inside `kb_claimgraph/` (A conformance, B inventory, C identification, D attribution, E assembly, F write, G gate). Always written "kb_claimgraph stage B" etc., never bare. |
| **Seats** | **AR** architect; **TW** tech-writer (with tech-writer-reviewer); **PE** prompt-engineer; **GC** go-coder; **PC** python-coder. |
| **the corpus** | The 50 arXiv ids in `ARXIV_IDS` in kbase's `justfile`, fetched and unpacked to `test_data/transient/arxiv/<id>/` by `just prep-test-integration-arxiv`. |
| **volume root** | The top `.tex` file of a paper: the file `00README.json`'s `toplevel` entry names, else the sole file containing `\documentclass`. |
| **the head** | kb_tools' build stages before validation: `document-graph`, `spine-seed`, `claims-declared`, `claims-discovered`, `equations-minted`, `depends-attributed` (kb_tools ARCHITECTURE, "The Driver"). |
| **charter, dropped rows, scope pin, node-pass record, double-run guard, verdict completeness** | As kb_tools defines them: SPEC "The Driver's Contract" and "Project Scoping"; ARCHITECTURE "The Claim Graph". |
| **citation states** | resolved, unanswered, key-only — kb_tools ARCHITECTURE, the `inventory.py` row. |
| **three-attribute anchor** | The rendered cross-reference `<a href data-reference-type data-reference>` (kb_tools SPEC, "Corpus Invariants"). |
| **`-latex_macros` reading** | Running pandoc's LaTeX reader with macro expansion disabled (`-f latex-latex_macros`), which may preserve author environment names. |

---

## 2. What kbase builds

**Exactly what kb_tools builds**, as Go, with kbase's provider management and
inference invocation kept. None of kbase's previous KB design carries over: no
hierarchical summaries, no page-size splitting, no kbase page grammar.

- **Input: LaTeX only.** No other input format is in scope.
- **The reader is pandoc, required on the host** (not shipped), with an
  enforced version range and `pandoc-api-version`, and a preflight check
  naming the install page when it is missing. kbase runs pandoc LaTeX→JSON
  with `--citeproc` and **no filters**; only one package execs pandoc.
- **Go source pre-passes before the parse: exactly kb_tools' two** —
  `strip_environment_declarations` and `theorem_display_names`
  (`adjagent/kb_tools/kb_docgraph/convert.py`). Nothing else massages source.
  Every pre-pass names the reader-level hole it fills, emits a census of what
  it found, and lives in one registry; a new entry is an ARCHITECTURE change.
- **Go transforms over the JSON replace kb_tools' Lua filter**
  (`adjagent/kb_tools/kb_docgraph/authored_blocks.lua`): Cite span wrapping,
  author-declared Div → labelled blockquote, title-page drop, display maths
  lifted out of emphasis.
- **Markdown plus records.** The leaves are Markdown; alongside, Go emits
  records (JSONL, in the build state store, never in `kb-root/`) carrying
  reader facts: display name, identifier, printed number, optional title,
  owning document, order, references (macro type as pandoc spells it, labels,
  the bounded text run before each), citations (keys, state), references in a
  proof's opening run. Claim-bearing classification and proof→subject binding
  remain claim-graph rules. The claim-graph inventory comes from records, not
  from re-parsing Markdown.
- **Theorem numbering** is whatever pandoc prints, as kb_tools does. Matching
  the typeset document's numbers is a post-port improvement (§11).
- **The Markdown writer** is decided at the Slice-1 gate (§6): pandoc's gfm
  writer over the transformed JSON with `--wrap=none`, unless Slice-1 finds a
  load-bearing form it cannot produce, in which case the owner rules on a Go
  writer.

### System compatibility, both directions

- A KB kbase builds is navigable by kb_tools and the kb-docent agent (Claude
  Code or opencode) and maintainable by kb-maintainer.
- A KB kb_tools builds is fully queryable and modifiable by kbase.
- kb_tools' build-pipeline intermediates are out of scope.
- What must hold exactly: the metadata layer, the load-bearing forms, and the
  KB's placement (`kb-root/` beside `.git`; the docent starts at
  `kb-root/entry-point.md` and descends through `index.md` files).
- **Presentation is free and untested** — wrap width, bullet characters and
  the like.
- **Byte equality is asserted only where a named reader needs it:**

  | Byte-equal | Reader that needs it |
  |---|---|
  | Metadata rendering (register entries, frontmatter, markers, derived-field placeholders) | kb_tools' parser; refresh's placeholder recognition; `kb_write`'s locate/splice |
  | `.index/*.jsonl` and derived fields | Each toolchain's freshness gate (dry-run refresh diffed against disk) |
  | `kb-root/CLAUDE.md` exactly `@AGENTS.md` | kb_tools' `kb_index_lib.unmigrated_agents_file` (refresh and verify exit 2 otherwise) |
  | The `` ``` math `` fence opener | kb_tools' fence scanner |
  | The up-link line | `UPLINK_MARKER` |

- **Acceptance bar:** each toolchain's own checks run green over the other's
  output (§8.3), with one named exception: kb_tools' sheet freshness check on a
  kbase-written placeholder (below).
- New KBs keep kb_tools' metadata formats; JSON vs YAML is a load-time parser
  choice over identical schemas. Breaking format changes wait until kbase is
  proven.

### `claim-graph.svg`
A placeholder: an SVG with "NYI" and, beneath, a digest of the index it was
generated from (`index sha256:` plus the first 12 hex digits of the SHA-256 of
the `.index/*.jsonl` files concatenated in sorted path order). Deterministic;
kbase's verify checks it fresh. A sheet is **kbase's own** when it is that
placeholder (it carries the "NYI" text). kbase's refresh writes the
placeholder only where no sheet exists or the existing one is kbase's own;
kbase's verify freshness-checks only its own sheet and leaves a kb_tools-drawn
sheet unchecked, since it cannot recompose that render. It lives in its own rendering
module (index in, SVG out), called by refresh and verify through the seam the
real graphing algorithm will occupy, so landing the algorithm later replaces
the module's body and nothing else.

### kbase as personant's KB toolset
- kbase owns KB building and every mechanical maintenance function kb_tools
  has — refresh, verify, the write API's ops, render-citation, queries, the
  sheet — each a subcommand personant integrates as a builtin tool.
- kbase is personant's peer: personant's rules govern personant. Personant's
  current mutating-tool refusal, 30s tool cap, 8 KB result cap and
  background-watch mode are personant-side work.
- **Tool results — every subcommand, `models` and `configure` included:** one
  YAML document on stdout and nothing else; stderr for
  humans; fixed key order; strings quoted on emit; a refusal enumerates every
  offending item. Personant's CONVENTIONS "Serialization format" rule governs
  file formats (JSONL for flat records, YAML for documents, TOML for flat
  config). `gopkg.in/yaml.v3` (or its maintained successor path,
  `go.yaml.in/yaml/v3`, per Step 1's registry check) is a general dependency.
- **Values input:** YAML (JSON accepted) on stdin or `--values`; a closed
  per-op key vocabulary; refusal names the key, its position and the allowed
  set; prose values containing U+0008 or U+000C are refused.
- **Every mutating op is idempotent under re-issue:** the same values twice
  leave `kb-root/` byte-identical to issuing them once, and the second call
  reports `unchanged`. Inserts adopt an existing (register, title, host) entry.
- **Writes leave nothing stale:** a mutating op ends with a refresh unless
  `--no-refresh` is given.
- **`build` runs as a monitored background process:** a `status` subcommand
  returns a YAML snapshot; progress is persisted to a file so a late or
  reconnecting monitor catches up; `cancel` costs only the in-flight unit.

### Inference
- Kept from kbase: provider management (`internal/config`), model detection
  (`internal/detect`), the client (`internal/model`), the prompt builder
  (`internal/prompt`), and the call runner (`internal/pipeline`: verification,
  informed retry, fallback, truncation handling).
- Each inference pass is built as plumbing on the runner where kb_tools places
  it; its prompt, fragments and seat definition are imported **byte-identical**
  from upstream at a recorded commit, with the same system-prompt placement.
  kbase changes no prompt prose. Upstream's answer envelope comes with its
  asks.
- Imported prompts are `@!slot!@` templates (kb_tools ARCHITECTURE, "The
  Driver", `prompt_templates.py`) sent with the seat definition as the system
  prompt; kbase's `internal/prompt` builds one user turn. The port adds a
  template composer and system-prompt support.
- **Open (owner), tail only: the overview stage's input.** kb_tools' overview
  stage asks a seat to read the KB tree
  (`adjagent/kb_tools/kb_driver/prompt-templates/phase-5-overview-passage.single.tmpl`),
  which a one-shot call cannot. Until ruled, kbase builds the stage with
  mechanically assembled input (entry point, index child lists, volume
  overview leaves) and a stub prompt held as an embedded template file, never
  an in-code string.

### Steering principles
- No LaTeX parser is written here, partial or whole: LaTeX's tail of cases
  never ends.
- kb_tools is the differential oracle; it works, and every difference from it
  is a named divergence or a defect.
- Byte equality is asserted only where a named reader needs it.
- No stage exits on a model's opinion; every stage exits on a comparison of
  artifacts or a return code.
- The kbase + personant integration is the target; a prior decision that
  prevents it is removed, not designed around.

---

## 3. Execution design (decided)

### Outer run loop
- **Git holds the recoverable record.** Each stage boundary is a commit in the
  user's repository with a structured message, ported from kb_tools' ledger
  (subject `kb-build: <stage-id> | <display name>`), carrying `kb-root/` and
  the tracked build records at kb_tools' paths at the repository root —
  `kb-build-charter.md` and the node-pass record `kb-build-node-pass.yaml`
  (kb_tools' `kb-build-node-pass.json`, in YAML). Resume position is read from
  the commit trail; a user may reset to a stage commit and resume from there.
- **The state store holds what does not belong in the repository:** run lock
  and pid, `progress.jsonl`, per-call evidence, the docgraph records (§2), and
  a per-unit answer cache keyed by input hash (recycled from
  `internal/pipeline`'s store and stamps). Nothing in it is authoritative for
  position.
- The store lives **outside the worktree**, at
  `$XDG_STATE_HOME/kbase/<key>/`, where `<key>` is the first 16 hex digits of
  the SHA-256 of the absolute, symlink-resolved `kb-root/` path; overridable with
  `--state-dir`: it holds paid-for inference and must survive `git clean`.
- Commits are scoped by pathspec to kbase-owned paths; a build refuses only if
  those paths are dirty.
- `build` refuses outside a git worktree, naming `git init`. Maintenance ops
  never require git.
- Per-leaf work inside an inference stage stays uncommitted until the stage
  boundary.

### Monitoring
`kbase build` runs in the foreground; personant backgrounds it. `kbase status`
returns YAML: state (none / running / cancelled / failed / bounded /
finished), pid, timestamps, each stage recorded or not with its commit, the
current stage's units done/total, recent refusals and fallbacks, the
`--no-inference` flag, and the resume command. `kbase cancel` signals the lock
holder; the in-flight call is abandoned with nothing written for it, the event
is logged, the lock released, and the process exits `cancelled`.

### Exit codes
The YAML `outcome` is the contract; the exit code derives from it.

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, incl. a `--through`-bounded run |
| `refused` (incl. verify findings, usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | Concurrent writer or live lock; re-issue identically later |
| `failed` | 3 | Defect in the tool, its input or the environment; re-issuing won't fix it |
| `cancelled` | 4 | Stopped on request; resumable |

---

## 4. Owner launch chores

Before Step 2 (Step 1 needs none of them):
1. **Provision adjagent's dev venv.** Every kb-testing driver recipe depends on
   `_venv`, which `pip install`s on first use (network). Check, in
   `/Users/agent-user/projects/adjagent/kb-testing/`: `just --list`, then `just
   prep-test-data-arxiv 2609.10318v1` and `just stage-arxiv-paper
   2609.10318v1`.
2. **Accept a dev-only Python instrument in kbase** at `tools/slice/`, run only
   through kb-testing's `measure-kb-roots` recipe: it runs kb_tools' own
   readers over both trees, so the comparison uses one normalizer.

Before the tail (not Step 2):
3. **Confirm adjagent's `install` recipe accepts a project path outside
   adjagent** (`just -f /Users/agent-user/projects/adjagent/justfile install
   <abs path>`) — needed by the compatibility harness (§8.3).
4. **Name who runs the live docent check** (§8.3, Direction 1 step 9): an
   interactive Claude Code or opencode session, which agents cannot open.

---

## 5. Step 1 — Contract documents

**Inputs, recorded at the start of the step:** `git -C
/Users/agent-user/projects/adjagent rev-parse HEAD` (the pinned adjagent
commit every kb_tools citation names), and `pandoc --version`. Both are
recorded in SPEC "Given interfaces" and in the step's commit message. The
current SPEC and ARCHITECTURE are read as `git show HEAD:SPEC.md` and `git show
HEAD:ARCHITECTURE.md` from the step's start.

**Output:** one commit replacing SPEC.md, ARCHITECTURE.md, CONVENTIONS.md and
ROADMAP.md, before any code. THESIS.md is unchanged.

**Owners:** TW writes SPEC.md, ARCHITECTURE.md and ROADMAP.md; PE writes
CONVENTIONS.md (agent-facing). AR reviews all four against this plan; AR's
findings return to the authoring seat, and Step 1 is not committed while any
is open.

**YAML package check (TW and PE together):** look up `go.yaml.in/yaml/v3` on
pkg.go.dev. If it is published and maintained, it is the sanctioned YAML
package; otherwise `gopkg.in/yaml.v3`. Record which in CONVENTIONS.

**Rule for every document:** state only what this plan decides. Where a
mechanism is not yet designed, state its seam, the kb_tools module it ports,
and "pending slice" — no invented detail.

### SPEC.md
1. **What kbase is:** LaTeX volume roots in → KB out → maintenance subcommands
   over a living KB; consumers are a person at a shell and personant's model.
2. **Given interfaces**, with observed versions: pandoc (version range: lower
   bound the recorded version, upper bound (exclusive) the next major
   `pandoc-api-version`, major meaning its first two components under
   Haskell's versioning policy (1.23.x → below 1.24);
   only the owner widens it); git; kb_tools' KB contract cited by
   section at the pinned commit — "The Document-Tree Contract", "What a KB Is",
   "Claim-Graph Nodes and Edges", "Derived Metadata, Defined", "The Claim-Graph
   Sheet", "Citation Grammar", "Project Scoping", "The Write API's Contract".
3. **Compatibility contract:** §2's compatibility section, including the
   byte-equality table and the sheet exception.
4. **Named divergences** — the whole list of differences from kb_tools in KB
   contents, maintenance-op behaviour and the tool interface (build
   orchestration differences are §8.2's "Not ported" list): values transport (YAML/JSON, not a
   TOML file); exit codes (§3); the dead-link gate checks `kb-root/` only
   (kb_tools checks the whole repository); build-state location (§3);
   `claim-graph.svg` is a placeholder (§2); the stamped KB documents
   (`AGENTS.md`, `CONVENTIONS.md`, `README.md`) give the maintenance commands
   for both toolchains — kbase's subcommands and kb_tools' `kb_util` ops — where
   kb_tools' give only its own; tool results are YAML documents (§2) where
   kb_tools prints `[kb-write] STATUS` report lines; the tracked node-pass
   record is YAML; until the owner rules on the overview stage's input (§2),
   the README's overview passage comes from mechanically assembled input.
5. **Build behaviour:** `kbase build <volume-root> [--bibliography FILE]...
   [--charter FILE] [--no-inference] [--through <stage>] [--state-dir DIR]`.
   Fresh vs resume is derived from the `kb-build:` commit trail, never
   configured; the KB is written only at `<git root>/kb-root/`; a populated
   `kb-root/` with no `kb-build:` commit trail is refused; `--no-inference`
   is first-class: it drops the rows that spend inference, still walks and
   records every stage, and states the rows it dropped — following kb_tools'
   classification (ARCHITECTURE "The Driver", `--no-inference` and
   `spends_inference`); build state never in
   `kb-root/`; determinism: the same
   inputs and pandoc version give byte-identical `kb-root/` and records.
   **Refusals:** unparseable source (names paper
   and reader error); unloadable include (names each file and the line that
   named it); an unclassified metadata key (a pandoc metadata key outside
   kb_tools' content and apparatus lists, `kb_docgraph/outline.py`).
   **Degradation, not refusal:** an unreadable bibliography — the build
   continues without it and reports the file. Monitoring per §3.
6. **Maintenance subcommands:** one subsection each, named after kb_tools'
   `kb_util` write ops and `kb_cmd` queries verbatim (queries cite
   `kb_cmd/cli.py` as well as ARCHITECTURE "Query Surface"), plus `refresh`,
   `verify`, `render-claim-graph`, `status` and `cancel`, citing kb_tools
   semantics, plus idempotence and writes-leave-nothing-stale (§2).
7. **Tool-result contract:** §2. The key set per subcommand beyond `outcome` is
   **Needs ruling**, designed by AR in the tail. `--help` and `--version` are
   human-facing and outside the contract.
8. **Values input:** §2.
9. **Configuration:** retained from `git show HEAD:SPEC.md` §2 except the
   `[dev]` table, which belonged to the retired pipeline and is removed
   (`--no-inference` replaces its mechanical tree plan; kb_tools' KB carries
   no build-date footer). An old `[dev]` key now fails strict load.
10. **Status markers / Needs ruling:** each item this plan leaves open (the
    writer route until Slice-1; per-subcommand result keys; the overview
    stage's input), one line each.

### ARCHITECTURE.md
- Purpose: auditability for an academic lab on local inference; claim graph in
  scope.
- Invariants: §7.
- Retained principles from `git show HEAD:ARCHITECTURE.md` §3:
  propose-and-verify, the model supplies values, monotone safety, provenance.
- Stage table: `start` → `document-graph` → `spine-seed` → `claims-declared` →
  `claims-discovered` → `equations-minted` → `depends-attributed` → `phase-3a`
  (refresh, verify, readiness stamp) → `overview-drafted`.
- One section per mechanism: pandoc seam; JSON transforms; writer route;
  pre-pass registry; records; tree derivation; partition checks and
  `validate_build`; metadata layer; write API; refresh/verify; index; queries;
  sheet; readiness stamping and README assembly; inference seams; git ledger
  and state store; status/progress/cancel; result emitter; compatibility
  harness.
- Package map: §8.1's packages and dependency rules are decided, as is I1's
  pandoc and git monopoly; each package's internals are "pending slice".
- **Pending slice:** each §6 hypothesis with its test.
- Constants table: only constants this plan names and SPEC does not already
  state — the state-store key form; exit codes and the sheet digest form are
  cited from SPEC, not repeated.

### CONVENTIONS.md
- Keep: Build and Run, Plan & Execute Process, Coordinator Policy, Testing,
  Logging, Key Architecture Constraints (pointing at the new ARCHITECTURE).
- Rewrite: Project Overview; Module Structure (§8.1); Dependency Policy —
  sanctioned: `spf13/cobra` and `spf13/pflag`, `BurntSushi/toml`, the YAML
  package per Step 1's registry check, `golang.org/x/text` if still used;
  goldmark removed.
- Add house rules: only `internal/latex/pandoc` execs pandoc and only
  `internal/ledger` execs git, enforced by recycling
  `internal/survey/importpolicy_test.go`'s AST-scan pattern; file formats
  follow personant's CONVENTIONS "Serialization format" rule (JSONL for flat
  records, YAML for documents, TOML for flat configuration); the pre-pass
  registry rule (§2); never enumerate accepted values of a field kbase does
  not fill; never spell the node-kind list; no prose copy of an op's
  vocabulary; frontmatter writers carry forward attributes they do not own; no
  stage exits on a model's opinion; no prompt from in-code strings, imported
  prompts carry provenance; compatibility is judged by kb_tools' own readers
  and checks, never by a byte diff of presentation; "name the producer before
  you write the refusal"; "a check earns its place by what it reads"; kbase's
  stamped KB templates get PE review before first use. The rules from "never
  enumerate" through "no prompt from in-code strings" adapt kb_tools'
  CONVENTIONS.md; the two quoted rules adapt kb_tools' AGENTS.md; the
  compatibility rule is kbase's own.

### ROADMAP.md
Now: this plan. After §8.4 item 1: display names from `\input`-ed preambles;
typeset-number fidelity; the `-latex_macros` reading. After kbase is proven:
records-native leaves with a Go writer; kb_tools' opt-in document audit; the
real claim-graph sheet algorithm, landing in the rendering module.

**Step 1 done:** every outcome in §2–§3 and every invariant in §7 is stated
in the contract documents; every §6 hypothesis appears in ARCHITECTURE
"Pending slice" with its test. **Decider:** the owner reviews the commit;
Step 2 starts on approval.

---

## 6. Step 2 — The slice

### What it builds
The first cut of the real command: `kbase build <volume-root> --through
document-graph --state-dir DIR [--bibliography FILE]...`, writing `<git
root>/kb-root/` — no dev-only verb. Code lives where it will finally live
(`internal/latex/pandoc`, `internal/latex/prepass`, `internal/docgraph`); keep
or rewrite is decided at slice exit. Writer: pandoc gfm, `--wrap=none`.

**Slice scope.** The slice implements `start`'s pandoc preflight and the
populated-`kb-root/` refusal, writes records to `--state-dir` (required in the
slice), makes **no git commits and takes no lock**: the ledger, lock, resume
and the rest of `start` are §8.4 item 6. `cmd/build.go` is replaced; the
packages its replacement leaves without callers are deleted in the same step.
The rest of §9 executes in the tail.

**Where runs happen.** For each paper and each run, the recipe creates a fresh
fixture repository `test_data/transient/test-integration-slice-arxiv/<id>/run-<n>/`
(`git init`, then a copy of `test_data/transient/arxiv/<id>/`) and runs `kbase
build` there with that paper's volume root. The fixture's `kb-root/` is the
kbase tree compared below. Two runs per paper (run-1, run-2) give the
determinism check.

**Owners.**
- **PC** writes `tools/slice/kbtools_dump.py` and, before GC starts on the
  comparator, `tools/slice/README.md` documenting its output schema (below)
  with the exact field names, taken from kb_tools' own reader attributes.
- **GC** writes the slice code, the Go comparator (reading that YAML) and the
  `test-integration-slice-arxiv` recipe.
- **TW** transcribes the verdicts into ARCHITECTURE "Pending slice"; **AR**
  reviews.

### Hypotheses the slice settles
| Hypothesis | Confirmed by | Refuted by |
|---|---|---|
| No filter layer is needed | kb_tools' conformance and inventory pass on kbase's tree; per-leaf word streams match; citations appear in all three states | A load-bearing form or word no JSON transform can produce |
| Records suffice for the inventory | kb_claimgraph stage B's inventory from records equals kb_tools' item by item; every record joins exactly one document | A needed fact exists only in rendered bytes |
| The preceding text run suffices | The derived word equals kb_tools' `Anchor.preceding_word` for every slice anchor | Space or NBSP handling in the AST loses it |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit |

### kb_tools reference
For each paper, with `<staged>` =
`/Users/agent-user/projects/adjagent/kb-testing/test-data/transient/arxiv/<id>`:
1. In `/Users/agent-user/projects/adjagent/kb-testing/`: `just
   prep-test-data-arxiv "<id>"`, `just stage-arxiv-paper <id>`, `just
   no-inference-kb-driver-arxiv-paper <id>`.
2. Extract the `document-graph` tree: `C=$(git -C <staged> log --format=%H
   --grep='^kb-build: document-graph ')`, `mkdir -p
   /Users/agent-user/projects/kbase/test_data/transient/slice-ref/<id>`, then
   `git -C <staged> archive "$C" kb-root | tar -x -C
   /Users/agent-user/projects/kbase/test_data/transient/slice-ref/<id>/`. The
   reference kb-root is `slice-ref/<id>/kb-root`.
3. In kb-testing: `just measure-kb-roots
   /Users/agent-user/projects/kbase/tools/slice/kbtools_dump.py --ref
   <slice-ref/<id>/kb-root>... --kbase <run-1 fixture's kb-root>... --out
   <file>`. `measure-kb-roots` passes every kb-root it discovers under
   `kb-testing/test-data/transient` as leading arguments, so the script must
   ignore every leading kb-root not named by `--ref` or `--kbase`. It runs
   kb_tools' readers over both trees read-only and writes one YAML document:
   ```yaml
   roots:
     - path: <kb-root>
       role: ref | kbase
       paper: <id>
       tree: [{path, title, parent, children}]      # children in order
       tokens: {<document path>: [<token>, ...]}     # kb_docgraph/text.markdown_tokens
       conformance: {passed: <bool>, failure: <point and message, or null>}
       inventory:                                    # kb_claimgraph/inventory.py
         blocks: [{document, name, identifier, title, locator}]
         anchors: [{document, type, labels, target, fragment, preceding_word}]
         citations: [{document, keys, state}]
         proofs: [{document, subject}]
         labels: {<label>: <document>}
   ```
4. Record the adjagent commit and `pandoc --version` in the evidence.

### Slice-1 — two papers
2609.10318v1 (math.DG; single file, within-section numbering) and 2609.09855v1
(math.ST; multi-file plus `references.bib`) — the two ids in kb-testing's
`ARXIV_LIVE_IDS`, so later inference stages have upstream history to compare.

| Check | Passes when |
|---|---|
| Tree | Identical path set and child order per index |
| Per-leaf content | Word streams under kb_tools' `kb_docgraph/text.markdown_tokens` are equal. Any difference fails; `comparison.yaml` records each with its leaf and token offset |
| Load-bearing forms | kb_tools' Document-Tree conformance passes on kbase's tree; counts match for up-link lines, three-attribute anchors, math fences, `data-cites` citation spans, labelled blockquotes with identifiers |
| Inventory | kb_tools' inventory over kbase's tree equals it over kb_tools' tree, item by item |
| Records | kbase's records equal that inventory |
| Determinism | run-1 and run-2 give byte-identical `kb-root/` and records |

**Slice-1 gate.** All checks pass → the writer route is pandoc's writer, the
owner is told, and Slice-2 starts. Any check fails → execution stops; GC lists
each failure in `comparison.yaml` with the check, the evidence path and what
differs; the owner rules (fix, amend the check, or switch to a Go writer).

### Slice-2 — widening
Same checks as Slice-1 on: 2609.10525v1 (theorems declared in an `\input`-ed
`theorems.tex`), 2609.10291v1 (included preamble, within-section numbering),
2609.10111v1 (no `.bib`), 2609.10385v1 (`\input`-chained sections). Negative
controls, kbase side only (no reference), fetched in kbase with `just
prep-test-integration-arxiv "2609.10029v1 2609.09821v1"`: 2609.10029v1 must
refuse naming the paper and pandoc's error; 2609.09821v1 must refuse naming
each unloaded file and the line that named it. **Pass** → Slice-3 starts.
**Fail** → same rule as the Slice-1 gate.

### Slice-3 — census of the corpus (kbase side only)
Measurement, no pass threshold. `census.yaml` records per paper: unknown AST
node types; pre-pass hits; declarations in `\input`-ed files; numbering
declarations; maths vs source bytes; leaf token sizes.

### Evidence
Recipe `test-integration-slice-arxiv <stage>`, excluded from the hermetic
`test-integration` omnibus (it needs pandoc and the adjagent reference; its
`[doc]` says so): builds `bin/kbase`, runs it, runs the dump script, runs a
corpus-driven Go comparator that skips when data is absent. Outputs under
`test_data/transient/test-integration-slice-arxiv/`: per-paper
`comparison.yaml`, `census.yaml`, `EVIDENCE.md`, `log.txt`.

**Slice done:** every hypothesis has a verdict backed by a file in that
directory, transcribed into ARCHITECTURE. **Decider:** the owner approves the
verdicts; AR then re-plans §8.

---

## 7. Invariants

| # | Invariant |
|---|---|
| I1 | Exactly one package execs pandoc (checks version and `pandoc-api-version`, named errors); exactly one execs git |
| I2 | The claim-graph inventory comes from records; only the prose and marker machinery reads leaf lines; maintenance ops read only the metadata layer |
| I3 | Pre-passes are kb_tools' two, in one registry, each naming its hole and emitting a census |
| I4 | System compatibility per §2; byte equality only where a named reader needs it; presentation untested |
| I5 | One result emitter owns stdout |
| I6 | Every mutating op is idempotent under re-issue |
| I7 | Imported prompt text, fragments and seat definitions are byte-identical to upstream at a recorded commit, with the same system-prompt placement |

Deliberately unspecified: internal AST representation, record field encoding,
layout algorithms.

---

## 8. The tail — skeleton, NOT executed (re-planned after the slice)

### 8.1 Package skeleton (dependencies point downward only)
```
cmd/                      cobra root; one file per subcommand; outcome→exit table
internal/result           YAML result document + emitter (I5)
internal/latex/pandoc     reader + writer seam (I1)       [kb_tools pandoc.py]
internal/latex/prepass    kb_tools' two scans (I3)        [kb_docgraph/convert.py]
internal/docgraph         JSON transforms, whole-volume render+cut, outline,
                          records, partition checks, validate_build
                          [kb_docgraph/*, kb_survey/validate.py]
internal/kb               kb-root model: paths, exclusions, frontmatter and
                          register parse, schema [kb_index_lib parse, kb_schema, kb_links]
internal/index            solidity, aggregates, footers, .index emit + freshness,
                          link + citation gates [kb_index_lib compute, refresh_/verify_*]
internal/write            render / values / store / ops   [kb_write]
internal/query                                            [kb_cmd]
internal/sheet            rendering module: index in, SVG out; today the NYI placeholder
internal/kbdocs           readiness stamp, README assembly [installed/*.tmpl, kb_readme]
internal/claimgraph       conform, inventory(records), identify, attribute,
                          equation(s), endcap, hand_named, assemble, write, gate
                                                          [kb_claimgraph]
internal/asks             embedded templates + composer + envelope parsers
                                                          [prompt_templates, envelope, claimgraph ask]
internal/ledger           git checkpoint seam (I1)        [kb_pipeline ledger half]
internal/build            stage table, state store, resume, --no-inference,
                          --through, status/progress/cancel [kb_driver head, phase-3a, overview]
kept: config, detect, model, prompt, pipeline (runner, store), log, tokens, text, version
```
`docgraph` knows nothing of claims; `claimgraph` never imports `latex/*`;
`write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`,
`claimgraph` or `build`; only `asks` and `build` import `pipeline` and
`model`; only `build` imports `ledger`.

### 8.2 Ported and not ported (against kb_tools' `kb_driver/steps.py` stage table)
**Ported:** `start` (lock, preflight for pandoc and git, charter,
populated-`kb-root/` guard); the whole head; `spine-seed`'s `.index/` seed;
`phase-3a` (refresh, verify, readiness stamp of `AGENTS.md`, `CLAUDE.md`,
`CONVENTIONS.md`, each only if absent); `overview-drafted`; the node-pass
record as a YAML build record; `kb_survey/validate.py`.

**Not ported:** `ov.docent-check` (checks a Claude Code command install);
`spine-seed`'s runner include line and its barrier (the compatibility harness
installs kb_tools' targets through kb_tools' own installer); relay cards and
the exit ladder (replaced by §3); `--permission-mode` and
`inference/claude.py`; `install_location` and the installer;
`kb_survey/manifest.py` and `skeleton.py` (uncalled); the opt-in document
audit (`phase-5`).

### 8.3 Two-way compatibility harness
Recipe `test-integration-compat-arxiv`, excluded from the hermetic omnibus.
Evidence under
`test_data/transient/test-integration-compat-arxiv/{kbase-built,kbtools-built}/<id>/`,
one log per check, roll-up `compat.yaml`, `EVIDENCE.md`. One op script, the
YAML fixture `test_data/fixtures/compat/ops.yaml` (insert-claim-entry,
set-rigor, add-depends-on, mark-claim-in-leaf, insert-work-entry), rendered to
TOML for kb_tools by the harness.

**Direction 1 — kbase-built KB, judged by kb_tools.**
1. Fixture repo under kbase's transient directory: `git init`, unpack the
   paper's sources.
2. Install kb_tools and the agents: `just -f
   /Users/agent-user/projects/adjagent/justfile install <abs dir>` (chore 3).
3. `./bin/kbase build <volume-root> --no-inference`.
4. `PYTHONPATH=.claude/agents python3 -m kb_tools.kb_util install-targets`.
5. `make kb-verify` without a refresh: **expected red** on the sheet freshness
   check only (§2's named exception); any other red is a defect.
6. `make kb-refresh && make kb-verify` — green.
7. `make kb-stats` and the docent's queries (`kb_cmd deps|show|subtree`).
8. The op script through `kb_util`, then refresh and verify — green; then
   `kbase verify` — green.
9. Live docent check (chore 4): `/kb-start` and `/kb-next` in Claude Code and
   in opencode on one fixture, transcript saved beside it, against a
   checklist — entry point reached; descent by index; up-links followed; a
   query answered; `session/` written and ignored by both verifiers.

**Direction 2 — kb_tools-built KB, judged by kbase.**
1. On kb-testing's staged repos, read-only to kbase: `./bin/kbase verify`
   green; every kbase query equals `kb_cmd --json` output as data.
2. A local `git clone` of a staged repo into kbase's transient directory.
3. `kbase refresh`; the diff against kb_tools' derived files is the
   measurement for `.index/` byte equality.
4. The op script through kbase.
5. `kbase verify` green, and the clone's `make kb-verify` green.

### 8.4 Port order and acceptance
1. **docgraph (production):** every corpus paper builds; Slice-1's structural checks hold
   across the corpus; partition checks and `validate_build` green; both negative
   controls refuse as specified.
2. **kb model, index, verify, sheet:** Direction 2 steps 1 and 3 green;
   `.index/*.jsonl` and derived fields byte-identical to kb_tools' refresh;
   `kbase verify` agrees with kb_tools on `adjagent/kb_tools/tests/fixtures/mini-kb`
   and mutated variants; the sheet is the placeholder.
3. **write API:** rendered metadata byte-identical to
   `adjagent/kb_tools/tests/fixtures/writeapi-render-golden/`; kb_tools'
   write-API regression fixtures replay (locate under
   `adjagent/kb_tools/tests/fixtures/` at this stage); I6; values refusal
   shape; Direction 1 step 8 and Direction 2 step 4 green.
4. **queries:** Direction 2 step 1's query comparison passes.
5. **claim graph, mechanical:** register entries and edges set-equal to
   kb_tools' `--no-inference` build by title, host and endpoint titles;
   refresh and verify green; the double-run guard refuses.
6. **build orchestrator, ledger, state store, monitoring:** Direction 1 fully
   green. Readiness documents: `CLAUDE.md` exactly `@AGENTS.md`; `AGENTS.md`
   carries the scope pin; no stamped document contains a `### INVARIANT-`
   heading (kb_tools parses those from `AGENTS.md` as framework nodes);
   stamping only if absent; invocation text names both toolchains' surfaces
   (PE review). Kill-and-resume re-mints nothing; `status` correct at every
   state; cancel loses only the in-flight unit.
7. **inference seams as plumbing:** the node pass, kb_claimgraph stage D's
   selection and the overview stage against mock answers; verdict completeness
   and budgets behave; no stage exits on a model's opinion. The runner gains a
   small exported entry point carrying a system prompt (`CallRunner.Run`
   takes an unexported call today).
8. **prompt import:** I7. Seat definitions are imported in the form adjagent
   renders for the gemma-4 family at the recorded commit; templates and
   fragments as stored. Provenance (adjagent-relative path and commit) lives
   in one manifest beside the embedded files in `internal/asks`, never inside
   an imported file. A recipe re-renders and re-reads them at the recorded
   commit and diffs, checking byte identity. Parse and compliance rates measured with kb-testing's
   `replay-claimgraph-asks` and `compare-claimgraph-asks` recipes against the
   local endpoint. README: every count slot equals the value computed from
   `.index/` and the tree; the prose slot carries the seat's answer verbatim;
   README passes both toolchains' link and citation gates.
9. **personant contract:** every subcommand meets the tool-result contract;
   status, progress and cancel exercised by an out-of-process monitor.

---

## 9. Existing kbase packages

| Package | Disposition |
|---|---|
| `cmd/` | Recycle: keep the cobra root, flags, logging, `models`, `configure`; rewrite `build`; add `status`, `cancel`; delete `survey`, `write-agents` |
| `internal/config`, `detect`, `model`, `prompt` | Keep; `model` gains a system message, `prompt` gains the template layer via `internal/asks` |
| `internal/pipeline` | Recycle: runner kept; store, stamps, lock, tempwork and crashpoint become the state store and answer cache; coordinator and phase table decided at §8.4 item 7 |
| `internal/log`, `tokens`, `text`, `version` | Keep |
| `internal/assemble`, `dissect`, `distill`, `ingest`, `summarize`, `taxonomy`, `treeplan` | Delete |
| `internal/survey` (+ `markdown/`) | Delete; recycle only the import-policy test pattern (I1) |
| The two atomic writers (`internal/pipeline/store.go`, `internal/config/write.go`) | Recycle into one before the write API adds a third |

---

## 10. Risk to watch
The slice's papers and kb_tools' own design were both taken largely against a
small sample. Stop and raise it to the owner when a check fails for a reason
whose fix needs a third pre-pass, a filter, or a change to any of I1–I7.

## 11. After the port
Each also goes upstream to kb_tools as a finding: typeset-number fidelity
(within-section numbering and `\numberwithin` are common in the corpus, and
pandoc's counter disagrees with the typeset document there); display names
from `\input`-ed preambles; the `-latex_macros` reading; records-native leaves.
