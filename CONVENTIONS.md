# CONVENTIONS – KBase

Guidance for agents (automated or human) working on this codebase.
**Read ARCHITECTURE.md first.** It is the authoritative design reference and
supersedes any inference you draw from code alone. Future intent is
ROADMAP.md; the plan under way is `ACTIVE_PLAN.md`, and partly-planned work
waits in `ROADMAP_PLANS/` (see "Plan & Execute Process").

---

## Project Overview
kbase is a Go binary that builds a knowledge base (KB) from a LaTeX document
set and maintains it afterwards. It builds exactly what kb_tools builds — the
reference implementation at `adjagent/kb_tools/` in the sibling `adjagent`
repository — and keeps its own provider management and inference invocation.
Every KB function is a subcommand that personant's model calls as a builtin
tool. kb_tools is the differential oracle: every difference from it is a named
divergence (SPEC "Named divergences") or a defect. The contract is SPEC.md;
how this implementation meets it is ARCHITECTURE.md.

---

## Build and Run

Run `just` (the default recipe) for the up-to-date recipe list with
docstrings — that output, not this file, is the reference for available
actions. Key habits: `just edit-gate` after every change, `just checkpoint`
at checkpoints, and anything LIVE (provider + network) is excluded from the
hermetic `just test-integration` omnibus and says so in its docstring.

`just dist` cross-compiles every target from one host (pure Go, no extra
toolchain setup needed) and runs `checkpoint` first — a cross-build is what
users receive, so it ships only from a tree that passes the full gate.

**No built binaries live in the repo.** `bin/` and `dist/` are gitignored
build products; distribution is the `just dist` tarball attached to a GitHub
release by the user. Never commit build outputs or publish releases.

**Test data layout.** `test_data/fixtures/` holds version-controlled test
data we author, own, and maintain. `test_data/transient/` (gitignored) holds
cloud-sourced corpus clones, generated test data, and test output. Never
write generated or downloaded data into `fixtures/`.

---

## Module Structure

The package map and its dependency rules are ARCHITECTURE "Package map"; this
file does not restate them. Working rules for packages that carry over:

- **cmd/**: verb logic lives behind a plain options-struct function; the cobra
  `RunE` is a thin loader shell, so verbs unit-test without process/network.
- **internal/config**: loaders take explicit paths (tests never touch a real
  home). Per-entry provider faults never abort the pool; fault reasons carry
  no key material.
- **internal/model**: `Endpoint{Name, BaseURL, APIKey}` is the transport-level
  slice of a provider. Long pipeline calls use `ConsultDrained` (streaming
  transport, blocking semantics) for idle-timeout robustness. API keys never
  appear in logs or error strings.

---

## Key Architecture Constraints

The invariants and principles in ARCHITECTURE "Invariants" and "Principles"
bind every code change; violating any is a blocking defect. They are
deliberately not restated here — ARCHITECTURE.md is the single source.
Working habits they impose:

- Before writing code, have those two sections fresh in context; check your
  change against them before considering it done.
- When a requested change collides with an invariant or principle, stop and
  surface the collision — do not quietly pick a side.
- Constants live in ARCHITECTURE "Constants" and as named constants in code;
  never introduce a magic number alongside them.

---

## House Rules

- **The pandoc and git monopoly (I1) is enforced by an AST-scan test** over
  every Go file, recycled from the pattern in
  `internal/survey/importpolicy_test.go`. Reach pandoc or git through the
  owning package, never beside it.
- **A pre-pass beyond I3's two is an ARCHITECTURE change, never a patch.**
  Nothing outside the registry massages source.
- **File formats follow personant's CONVENTIONS "Serialization format"
  rule:** JSONL for flat records, YAML for documents, TOML for flat
  configuration.
- **Never enumerate what you will accept in a field kbase does not fill.**
  pandoc fills the reference macro type and Div classes; an author fills
  metadata keys. Read such a field whole and branch only on the values you
  must act on, passing everything else downstream under its own name. A table
  that must stay closed counts and reports its residue. A pattern or branch
  that enumerates accepted values leaves an unanticipated one no trace: a
  reference whose type the pattern did not list is read as no reference. An
  answer to one of kbase's own asks is not this case — that format is ours,
  and its parser stays strict.
- **Never spell the node-kind list.** It is defined once, in `internal/kb`'s
  schema. A site that *iterates* — a census, a breakdown, a union, an
  ordering — reads the list and names no kind. A site that *branches* names
  kinds, and its table must be total over the list. A count derived from the
  list ("all four kinds") is the same copy in a shorter form.
- **Keep no prose copy of an op's key vocabulary.** The vocabulary is closed,
  total and per-op, stated once in `internal/write`. A second statement — in a
  doc, a docstring or a comment — drifts; point at the definition or the
  refusal message it produces.
- **A frontmatter writer carries forward every attribute it does not own.**
  `set-frontmatter` replaces the whole block, so a key left out is a key
  removed. A writer reads the block as it stands and restates what is not its
  own.
- **No stage exits on a model's opinion.** Every stage exits on a comparison
  of artifacts or a return code; a new stage does not get a third kind of exit.
- **No prompt reaches a model from an in-code string.** Every prompt is an
  embedded template file under `internal/asks`, rendered with its slots
  filled. Provenance for imported templates, fragments and seat definitions
  — each file's path relative to the adjagent repository root and the
  adjagent commit it was taken at — lives in one manifest beside the embedded
  files in `internal/asks`, never inside an imported file, whose bytes stay
  identical to upstream. A recipe checks that identity.
- **Compatibility is judged by kb_tools' own readers and checks, never by a
  byte diff of presentation.** Byte equality is asserted only where SPEC
  "Compatibility contract" names a reader that needs it.
- **Name the producer before you write the refusal.** Before writing code that
  refuses, raises, or reports a finding on a state, name what produces that
  state. If nothing on a reachable path produces it, the code restates a design
  the rest of the code already states; delete it. If it can fire on something
  else — a guard firing on a corpus that spells its token in prose — its only
  reachable outcome is a build failing for a reason that does not exist. Making
  a failure survivable is a different act; this rule does not reach it.
- **A check earns its place by what it reads.** It reads what arrived from
  somewhere kbase did not compute: pandoc's output, an author's LaTeX, a hand
  edit on the maintenance path, values on stdin, a model's answer to one of
  our asks, a KB kb_tools built. A check over a value kbase computed restates
  the computation. That list is not closed; a new source is argued against
  the same test. Process separation is not the test. Each check argues its own
  case in the report that lands it; a comment asserting boundary status is not
  that argument.
- **kbase's stamped KB documents get prompt-engineer review before first
  use.** The templates `phase-3a` stamps into `kb-root/` and the README
  assembly template are read by models in every KB kbase builds.

---

## Plan & Execute Process

Plans are tracked: progress and intent are worth more than a tidy tree, and a
plan in an ignored file is one power-cycle from gone.

- **`ACTIVE_PLAN.md`** at the repo root is the one plan under way: decisions
  confirmed at kickoff, work-package boundaries, sequencing and status.
  Dispatched agents execute out of it; status updates as work packages land.
  It carries outcomes, their consequences and the principles still steering
  open choices — not decision history, which lives in commits. When the work
  lands, its outcomes move into the contract documents and the file is
  replaced by the next plan.
- **`ROADMAP_PLANS/`** holds partly-planned work that is not under way: a plan
  with enough design done that someone could pick it up, usually behind a
  ROADMAP.md item. A plan leaves when it becomes the active plan, is finished,
  or is abandoned. A plan there cites only tracked paths.
- Working drafts and half-formed notes are scratch (`.claude-temp/`), not
  plans.

## Coordinator Policy

For the session-level agent managing dispatched coder agents.

**Audit the diff, not the report.** After a dispatched agent reports, audit
the DIFF before accepting — the report is an index, the diff is the
evidence. Check the diff against the recurring violation classes:

- integration tests invoke the app binary; no dev-only entry points, verbs,
  or test scaffolds around the real surface
- verifications are repeatable invocations; evidence lands in inspectable
  artifacts, never only in the report
- no special-case file lifecycles: every path kbase writes belongs to a
  location SPEC or ARCHITECTURE names
- no gate, pin, or assertion loosened to make something pass — expected-truth
  updates must state the new truth they track
- no second home for a value, string, or mechanism that has one
- nothing beyond the dispatched scope, even improvements

Violations block acceptance: fix (redispatch or direct) before commit.
Style and judgment calls outside these classes: flag only if egregious.

**Direct coding at the session level**: permitted, but read the
topic-specific coder agent definition first (plus this file if not fresh in
context) so session-level code holds the same contract dispatched code does.

## Testing

**No verification is a one-off.** Every verification is a repeatable
invocation — a unit test or a runner recipe — never an ad-hoc command
sequence that lives only in a conversation.

**Integration tests invoke the app binary.** An integration test runs
`./bin/kbase` directly — the system end-to-end as delivered. Behavior
switches (`--state-dir`, `--config-dir`) are flags in the recipe's
invocation. In-process calls to verb functions are unit tests of verb
logic, never integration tests.

**Results are part of the test.** Results that cannot be examined are not
results.

- Integration recipes: `test-integration-<process>-<corpus>`; shared
  corpus fetch `prep-test-integration-<corpus>`; anything needing network,
  a live provider, pandoc or the adjagent reference is excluded from the
  hermetic omnibus `test-integration` and says so in its `[doc]`.
- Every integration test preserves its full log, its stats, and its run
  artifacts under `test_data/transient/<test-name>/` — nothing is written
  to an ephemeral location and tossed. Every integration test passes
  `--state-dir` under that directory, so nothing reaches the user's real
  `$XDG_STATE_HOME`.
- Unit tests pass hermetically (no network, no corpus) beside their
  packages; their `go test` log is preserved under
  `test_data/transient/unit_tests/`. Corpus-driven property tests skip
  when the corpus is absent and are wired into an integration recipe
  where it is guaranteed present.
- Observational evidence (counts, measurements, A/B numbers) is emitted
  to an inspectable location, never left in agent reports or scrollback.

---

## Dependency Policy

Stdlib-first; a new module requires a ruling. Sanctioned:

- `github.com/spf13/cobra` and `github.com/spf13/pflag` — the CLI.
- `github.com/BurntSushi/toml` — configuration files.
- `go.yaml.in/yaml/v3` — YAML; not `gopkg.in/yaml.v3`.
- `golang.org/x/text` — sanctioned only while a package imports it.

Add with `just add-dependency <module>@<version>`, only after the import
exists — `go mod tidy` drops a dependency nothing imports.

---

## Logging

Project-specific bindings only (the general logging doctrine lives in the
coder agent definitions):

- `internal/log` is the **only** logging seam (four-method `Logger`:
  `Debug`/`Info`/`Warn`/`Error`, alternating key/value pairs). Built **once**,
  in the composition root (`cmd/main.go`), from the persistent `--log-level`
  (default warn) and `--log-file` flags. `--log-file` tees; it never redirects.
- **Threaded, not global**: a component that logs takes a `log.Logger`
  parameter, so its logging is visible in its constructor signature. No
  package-level logger, no setter; `log.Discard()` covers a caller with
  nothing to hand it.
- Never call `slog` directly, and never use `fmt.Fprintf(os.Stderr, ...)` for
  a diagnostic. Verb *output* — a summary line, a user-facing warning — is a
  different channel and stays on the verb's `Stderr` writer.
