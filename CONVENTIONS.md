# CONVENTIONS – KBase

Guidance for agents (automated or human) working on this codebase. **Read ARCHITECTURE.md first.**
It is the authoritative design reference and supersedes any inference you draw from code alone.
Future intent is ROADMAP.md; the plan under way is `ACTIVE_PLAN.md`, and partly-planned work waits
in `ROADMAP_PLANS/` (see "Plan & Execute Process").

---

## Project Overview
kbase is a Go binary that builds a knowledge base (KB) from a LaTeX document set — or creates one
empty with `kbase init` — and maintains it afterwards. It keeps its own provider management and
inference invocation. The contract is SPEC.md; how this implementation meets it is ARCHITECTURE.md.

---

## Build and Run

Run `just` (the default recipe) for the up-to-date recipe list with docstrings — that output, not
this file, is the reference for available actions. Key habits: `just edit-gate` after every change,
`just checkpoint` at checkpoints, and anything LIVE (provider + network) is excluded from the
hermetic `just test-integration` omnibus and says so in its docstring.

`just dist` cross-compiles every target from one host (pure Go, no extra toolchain setup needed) and
stages the signed distro tarball and installer. Run it on a tree that has passed `just checkpoint`: a
cross-build is what users receive, so the gate is a precondition of the release, not a step the
recipe runs.

**No built binaries live in the repo.** `bin/` and `dist/` are gitignored build products;
distribution is the `just dist` tarball attached to a GitHub release by the user. Never commit build
outputs or publish releases.

**Test data layout.** `test-data/fixtures/` holds version-controlled test data we author, own, and
maintain. `test-data/transient/` (gitignored) holds cloud-sourced corpus clones, generated test
data, and test output. Never write generated or downloaded data into `fixtures/`.

---

## Module Structure

The package map and its dependency rules are ARCHITECTURE "Package map"; this file does not restate
them. Working rules for packages that carry over:

- **cmd/**: verb logic lives behind a plain options-struct function; the cobra `RunE` is a thin
  loader shell, so verbs unit-test without process/network.
- **internal/config**: loaders take explicit paths (tests never touch a real home). Per-entry
  provider faults never abort the pool; fault reasons carry no key material. Resolution is pure and
  lives here alone — `ModeOf`, `Resolve`, `Select` — and reads no file, opens no client, writes no
  stderr and names no verb: the key file `KBASE_API_KEY_FILE` names is read by whoever dials (in
  `cmd`, the registry; in `show-config`, its preflight). The package imports no kbase package but
  `atomicfile`, never `model`, `result` or `log`. A `[models]` value is parsed by one function,
  `ParseReference` (`<provider>/<model>`, split at the first `/`), and `Reference.String` writes such a
  reference back; an environment-supplied id is a bare id of the environment's endpoint, which nothing
  parses. No site splits an id or concatenates a reference by hand.
- **internal/model**: `Endpoint{Name, BaseURL, APIKey, Protocol, Decisions, PromptCacheKey}` is the
  transport-level slice of a provider — `Protocol` saying which chat surface its entry speaks
  (`openai` unless the entry says `anthropic`), `Decisions` whether it speaks the Decisions shape
  (`systemone` unless the entry says `decisions`), and `PromptCacheKey` the run every OpenAI-surface
  ask of it carries — and the one mapping from configuration's vocabulary onto `Protocol` and
  `Decisions` lives in `cmd`. The chat asks — the overview and each scoring call — go through
  `model.Chat`: one tool-less request, re-issued identically on an incomplete stream up to the
  attempt count (ARCHITECTURE §12). An attempt's outcome is the classification of what ended it, and
  the two failures are not interchangeable: an endpoint that answered with a non-2xx status refused
  the request and is `ChatProviderRefused` (its `StatusError` matched with `errors.As`, its status and
  body in the reason), where `ChatTransportFailure` is a connection, a name, a certificate or a
  stream that failed with no answer — reading one as the other sends a diagnosis to the path to a
  provider that is answering. The letter asks go through `model.Decision`, one `POST` to whichever
  surface the endpoint names (ARCHITECTURE §7; SPEC §9.1): `systemone` asks `<base-url>/systemone` one
  closed-set question (`choice` or `noul`), answered by probabilities over the offered letters;
  `decisions` speaks the OpenAI Decisions shape at `<base-url>/decisions` — the state as `input`,
  questions a named array with a `noul` asked as a `predicate` carrying no choices, its criteria
  folded into `instructions`, and an `answers` array where a `refusal` is unreadable and takes the
  seam's re-ask/default path. Either surface is re-issued identically on a transport error or
  retry-class 5xx. API keys never appear in logs or error strings. The `[models]` keys are
  `decision`, `text` and `scoring`, and each is a `<provider>/<model>` reference in file mode.

---

## Key Architecture Constraints

The invariants and principles in ARCHITECTURE "Invariants" and "Principles" bind every code change;
violating any is a blocking defect. They are deliberately not restated here — ARCHITECTURE.md is the
single source. Working habits they impose:

- Before writing code, have those two sections fresh in context; check your change against them
  before considering it done.
- When a requested change collides with an invariant or principle, stop and surface the collision —
  do not quietly pick a side.
- Constants live in ARCHITECTURE "Constants" and as named constants in code; never introduce a magic
  number alongside them.

---

## House Rules

- **The exec monopoly (I1) is enforced by an AST-scan test** over every Go file
  (`cmd/execpolicy_test.go`), and ARCHITECTURE §11's dependency rules by `cmd/importpolicy_test.go`
  over the transitive import graph. Reach pandoc, git or `dot` through the owning package, never
  beside it. The one self-exec owner is a file, not a package: `cmd/mcp.go` alone may exec the kbase
  binary, and the test fails on any other file that does. Four owners are the whole list — pandoc,
  git, `dot` and that self-exec — and a process started anywhere else fails the test. The policy
  scans share one module walk
  (`cmd/modulescan_test.go`), so they agree on which files are the module's source. The scan
  reads test files as source too — the trap: a package test that compiles its output under
  `pdflatex` has nothing wrong in it and fails the tree. A verification needing a binary no
  package owns is never a package test: it is a runner recipe (recipes exec freely). Evidence
  gathered for the change at hand is not a verification but a probe — a script run by hand in
  scratch, its log cited in the report, with no durable home. A probe whose finding must keep
  holding becomes a verification: the check lands in a recipe. A probe whose result a commit or
  plan cites keeps its command in the commit message, so a later reader can re-run it.
- **A pre-pass beyond I3's two is an ARCHITECTURE change, never a patch.** Nothing outside the
  registry massages source.
- **The installed agent set is a pinned artifact, never edited here.** `.claude/agents/` and
  `.claude/commands/` are tracked so a checkout pins them; a finding that needs an agent change
  goes to the upstream project, adjagent, as a hand-off, the fix lands there, and the set is
  updated from it.
- **File formats follow personant's CONVENTIONS "Serialization format" rule:** JSONL for flat
  records, YAML for documents, TOML for flat configuration — with one named departure from `1.0.0`
  (SPEC §10): the KB's index is a YAML stream in flow style, one `--- {…}` record per line, so the
  whole metadata layer has one serialisation while every line-oriented tool still sees one record
  per line.
- **A superseded format's parser lives in `internal/migrate` and nowhere else** (I8). The trap:
  keeping a reader for the old form "for compatibility" in `kb` or `index`, which is the
  multi-version reader the design forbids. A converter's test is a golden pair, old bytes in and new
  bytes out. The one permitted exception is a frozen copy of the superseded readers in test code
  only, for the parity test (`internal/kbload/parity_test.go`).
- **A `kb-format` patch bump moves the stamp and every fixture that pins its bytes, and stops the
  chain where its last converter does.** Two halves, both load-bearing. `kb.FormatVersion` moves, and
  every fixture whose bytes carry the stamp — or a derived line the change rewrote — is rewritten in
  the same commit: the loader reads an older patch of the same minor as current — no chain runs — and
  restamps it on the next save, so what moves under a fixture left behind is a leg that pins bytes
  after a write, not one that only reads. And `migrate.Chain` must stop at the form its last converter
  produces, because **no converter leads to a patch**: the chain breaks
  out where the major and minor already match the target, and the next save supplies the stamp. The
  trap on that side is teaching the chain to reach the patch, which is a converter whose output
  differs from its input only by the stamp.
- **Every KB file is read through `kb.Source` from `kbload.Open`.** The trap: an `os.ReadFile` of a
  KB file in a verb or package, which sees the on-disk form of an unmigrated KB.
  `cmd/readpolicy_test.go` fails on any direct KB file read outside its allowlist. The rule covers
  file reads only: a directory listing of `kb-root/` (`os.ReadDir` in `internal/index/refresh.go`
  and `internal/kb/deadlinks.go`) sees the disk, not the source, and nothing checks it.
- **A result field's integer is `int`, never `int64`.** `result.Write` refuses `int64`.
- **`just fmt` is the sanctioned formatter.** Running it to clear a gofmt alignment finding is not
  an edit outside the tools.
- **An MCP tool is a subcommand's binding** (I9). The trap: a tool listed by hand, a description
  copied into a tool, or a schema typed beside the command's flags. Each command declares its
  binding with the cobra annotation `kbase.mcp` = `bound` or `excluded`; the table binds only
  `bound` commands, and the test derives its expectation from the annotations, never from a list.
  A flag its verb refuses to run without declares `kbase.required` the same way, and the tool's
  schema lists it among `required`; the trap is a refusal whose flag carries no mark, so a model
  reading the schema cannot tell the flag must be given.
  The exception is a tool no command stands behind: listed, and only where the tool-table test
  (`cmd/mcp_test.go`) pins the set to the `bound` commands plus exactly those tools and the tool
  returns through `toolResult`, so text, structured content and exit code cannot drift from a
  command's. `rewind-build` in `cmd/mcp.go` is the only one: read-only, schema and description its
  own. Protocol tests are transcript fixtures under `test-data/fixtures/mcp/`, never strings
  asserted in code.
- **A command resolves its starting directory with `invocationDir(cmd)`; no `os.Getwd` in `cmd`
  outside that function.** The trap: `os.Getwd()` anywhere a verb's logic runs ignores `kbase mcp
  --kb-root`, so the tool reads the server's directory instead of the served KB.
  `cmd/invocationdir_test.go` enforces it.
- **Never enumerate what you will accept in a field kbase does not fill.** pandoc fills the
  reference macro type and Div classes; an author fills metadata keys. Read such a field whole and
  branch only on the values you must act on, passing everything else downstream under its own name.
  A table that must stay closed counts and reports its residue. A pattern or branch that enumerates
  accepted values leaves an unanticipated one no trace: a reference whose type the pattern did not
  list is read as no reference. An answer to one of kbase's own asks is not this case — that format
  is ours, and its parser stays strict.
- **Never spell the node-kind list.** It is defined once, in `internal/kb`'s schema. A site that
  *iterates* — a census, a breakdown, a union, an ordering — reads the list and names no kind. A
  site that *branches* names kinds, and its table must be total over the list. A count derived from
  the list ("all four kinds") is the same copy in a shorter form.
- **Never spell a relation.** The relation names (`kb.RelationSupports`, `kb.RelationFounds`,
  `kb.RelationLifts`, `kb.RelationRefutes`, `kb.RelationReferences` and `kb.RelationDemoted`, with
  `kb.Relations` and `kb.ScoringRelations` holding the sets) live in `internal/kb/schema.go` and
  nowhere else; a site that needs one names the constant. `cmd/relationpolicy_test.go` is an AST
  scan that fails on a spelling elsewhere, with a short allowlist of files where the same word means
  something else.
- **Read an index line only through `kb.ReadIndex`.** The row types (one per index file), the file
  inventory and the reader `kb.ReadIndex[T]` live in `internal/kb/indexstream.go`; `index` writes
  from them, `query`, `sheet` and the tests read through them. The trap: a second row struct or an
  untyped decode beside it. The exception is `verify` and `citations`, which read untyped on purpose
  to see malformed shapes.
- **Keep no prose copy of an op's key vocabulary.** The vocabulary is closed, total and per-op,
  stated once in `internal/write`. A second statement — in a doc, a docstring or a comment — drifts;
  point at the definition or the refusal message it produces.
- **A frontmatter writer carries forward every attribute it does not own.** `set-frontmatter`
  replaces the whole block, so a key left out is a key removed. A writer reads the block as it
  stands and restates what is not its own.
- **No stage exits on a model's opinion.** Every stage exits on a comparison of artifacts or a
  return code; a new stage does not get a third kind of exit.
- **No prompt reaches a model from an in-code string.** Every prompt is an embedded template file
  under `internal/asks`, rendered with its slots filled, and its prose lives in that file alone —
  never in a code string, a test, or a second copy elsewhere. A new or changed template gets
  prompt-engineer review before first use, and a template change lands as an experiment with a
  measured result. A call's system prompt is the fragment named for it, never an agent definition.
- **No test asserts a prompt's prose.** A test keyed on a template's wording is a second copy of the
  template; tests key on the offered letters, the slot set, the system fragment and the closing
  line's letters.
- **Compatibility is judged by kbase's own readers and checks, never by a byte diff of
  presentation.** Byte equality is asserted only where SPEC "Compatibility contract" names a reader
  that needs it.
- **A derived strength is one value; only its spelling differs.** The walk computes and publishes at
  full precision, so a dependent multiplies its source's exact strength. A record — the `- strength:`
  line and a premise bullet's `(strength …)` — carries the value in the fewest decimals that read
  back as it: four in the ordinary case, more where four would lose it, trailing zeros trimmed; and
  it is never the source of a dependent's arithmetic — the walk reads the value, not the record's
  spelling. A surface that shows a value to a reader puts it at two decimals — a query result as the
  number, the trace on a `- strength:` line as the spelling — while `verify` quotes the record's
  spelling, because its message is about the record.
- **A scoring-model fact lives once, in `internal/kb`.** The fourteen fault ids, `flt-pending`, the
  word `none`, each id's weight per occurrence and each table's maximum live in `faults.go` and
  nowhere else — the statement table, the method table and the bearing one, which a node's class
  selects among. A template names only the ids its class accepts, and carries no figure or word a
  model could read as a judgement on the answer — no decimal in `[0,1]`, no percentage, no weight, no
  maximum, no band's word — while a quantity of the subject's own, an example's rate or threshold, is
  the claim's and stays. `internal/asks` asserts both: the `flt-…` tokens it names are exactly its
  class's set, checked against `kb`, and its text holds none of those spellings. The trap: a weight in
  a template, or a second copy of the id list in the parser, which is a second home for a fact and
  drifts where the prompt does not.
- **A `flt-*` id is frozen for its meaning once named.** The set may grow or lose members, but a
  fault that has been named keeps its id for life: a ledger line records ids, and a rename would
  silently re-point every line that carried the old one. A new fault takes a new id; a retired fault's
  id never carries a new meaning.
- **A citation is looked up in the closed text the call carried, never in the register.** The parse's
  one job is where a span is, not whether it earns its id: a span is an anchor, and the criteria's
  satisfaction is the model's judgement and a later reader's audit, never a check. The trap: a
  validity test dressed as a parse, which refuses defensible answers on a question no code can
  settle.
- **A scoring value is derived in integer hundredths.** Weights, maxima and sums are integers; the
  value is `maximum − Σ weights`, floored at zero and exact at two decimals. The trap: subtracting in
  `float64`, where three weights land `0.3499999999999999` and the record's four-decimal rule breaks.
  A value is a projection of a ledger line's ids alone, which is what makes a landing checkable and
  a run re-scorable without asking a model.
- **A scoring page's value tables are derived, not authored.** A page's maxima, its reachable-value
  tables and its worked figures are functions of the ceiling and the weights, so they move in the
  commit that moves either. The trap: leaving a page alone because no test pins its numbers — a value
  table is a second copy of the arithmetic, and a stale one states a value the code cannot produce.
  The reasoning a page gives for a superseded choice stays, marked superseded in one note; what moves
  is the numbers.
- **Name the producer before you write the refusal.** Before writing code that refuses, raises, or
  reports a finding on a state, name what produces that state. If nothing on a reachable path
  produces it, the code restates a design the rest of the code already states; delete it. If it can
  fire on something else — a guard firing on a corpus that spells its token in prose — its only
  reachable outcome is a build failing for a reason that does not exist. Making a failure survivable
  is a different act; this rule does not reach it.
- **A check earns its place by what it reads.** It reads what arrived from somewhere kbase did not
  compute: pandoc's output, an author's LaTeX, a hand edit on the maintenance path, values on stdin,
  a model's answer to one of our asks, a KB this version did not write. A check over a value kbase
  computed restates the computation. That list is not closed; a new source is argued against the same test.
  Process separation is not the test. Each check argues its own case in the report that lands it; a
  comment asserting boundary status is not that argument.
- **Compare a label in the page's spelling.** pandoc escapes `&<>"'` in attribute values, and
  differently for a `Link` than for a `Span` or `Div`. A fact kbase records to be compared against
  the page carries the escaped form the page carries; never unescape one side to match the other.
  A label carried out of the page into another format — the LaTeX export — is HTML-decoded once, at
  its read; the page-spelling rule governs comparisons against the page.
- **In a pandoc Lua `topdown` walk, what a filter returns in place of its element never meets the
  walk's filters — not its own type's, not the enclosing list's — though its children are walked.**
  One element or a list, block or inline alike (pandoc 3.12.1). The trap: a filter that returns
  anything but nil or the element it was given — a `BlockQuote` filter returning the quote's body
  expects the `Figure` filter to see a figure in it, and it does not. Such a filter walks its output
  itself, as a list (`pandoc.Blocks{…}:walk(f)`, `pandoc.Inlines{…}:walk(f)`; an element's own
  `:walk` skips that element), and returns the result with `false` as the second value, or the
  descent walks those children twice.
- **Inside a bracket expression, spell whitespace as `kb.PyWhitespace`, never `\s`.** `kb.PyRE`
  expands `\s` into a bracket expression of its own, and nested inside `[...]` that compiles,
  without error, into a different pattern.
- **A float computation that decides a ranking sums in Python's order with its compensation**
  (`shortlist.go`'s `pySum`): Go's plain summation ranks a near-tie differently, and a differently
  ranked shortlist asks a different pair.
- **A public integration recipe re-invokes `just` for its private body, so a command-line variable
  override does not reach it.** A recipe that must take a configuration at the command line declares
  it as a parameter.
- **kbase's stamped KB documents get prompt-engineer review before first use.** The templates
  `phase-3a` stamps into `kb-root/` and the README assembly template are read by models in every KB
  kbase builds.

---

## Plan & Execute Process

Plans are tracked: progress and intent are worth more than a tidy tree, and a plan in an ignored
file is one power-cycle from gone.

- **`ACTIVE_PLAN.md`** at the repo root is the one plan under way: decisions confirmed at kickoff,
  work-package boundaries, sequencing and status. Dispatched agents execute out of it; status
  updates as work packages land. It carries outcomes, their consequences and the principles still
  steering open choices — not decision history, which lives in commits. When the work lands, its
  outcomes move into the contract documents and the file is replaced by the next plan.
- **`ROADMAP_PLANS/`** holds partly-planned work that is not under way: a plan with enough design
  done that someone could pick it up, usually behind a ROADMAP.md item. A plan leaves when it
  becomes the active plan, is finished, or is abandoned. A plan there cites only tracked paths.
- **A plan may name a normative page outside the contract documents, and that page outranks the
  plan.** The contract documents are THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md. A plan
  may make a page of its own subject normative — `RIGOR_AND_BEARING_SCORING_SCHEME.md` is such a
  page, stating the scoring rubric the contract cites — and on
  that subject the page is the requirement while the plan is the route. Where a plan row and the
  page disagree, the work implements the page and the plan's line is reported as moved, in the
  commit that lands it; the page is not edited by the landing, and the contract documents cite it
  rather than restate it. A plan that needs such a page names it in the plan, so a reader knows which
  source settles a disagreement.
- Working drafts and half-formed notes are scratch (`.claude-temp/`), not plans.

## Coordinator Policy

For the session-level agent managing dispatched coder agents.

**Audit the diff, not the report.** After a dispatched agent reports, audit the DIFF before
accepting — the report is an index, the diff is the evidence. Check the diff against the recurring
violation classes:

- integration tests invoke the app binary; no dev-only entry points, verbs, or test scaffolds around
  the real surface
- verifications are repeatable invocations; evidence lands in inspectable artifacts, never only in
  the report
- no special-case file lifecycles: every path kbase writes belongs to a location SPEC or
  ARCHITECTURE names
- no gate, pin, or assertion loosened to make something pass — expected-truth updates must state the
  new truth they track
- no second home for a value, string, or mechanism that has one
- nothing beyond the dispatched scope, even improvements

Violations block acceptance: fix (redispatch or direct) before commit. Style and judgment calls
outside these classes: flag only if egregious.

**Direct coding at the session level**: permitted, but read the topic-specific coder agent
definition first (plus this file if not fresh in context) so session-level code holds the same
contract dispatched code does.

## Testing

**No verification is a one-off.** Every verification is a repeatable invocation — a unit test or a
runner recipe — never an ad-hoc command sequence that lives only in a conversation.

**Integration tests invoke the app binary.** An integration test runs `./bin/kbase` directly — the
system end-to-end as delivered. Behavior switches (`--state-dir`, `--config-dir`) are flags in the
recipe's invocation. In-process calls to verb functions are unit tests of verb logic, never
integration tests.

**Results are part of the test.** Results that cannot be examined are not results.

**A test harness selects its inputs by content the KB contract fixes, never by presentation.** A
choice keyed on line wrapping or bullet style holds on one KB's output and silently finds nothing on
another's.

**A fixture whose answer shape a change invalidates moves in the same commit, and the commit body
names it.** When a change moves what an answer must look like — the parse now reads charge lines, so
a stub answering `0.5` is no answer — the fixture that answers in the old form moves with it, not in
a later package: a stub left behind makes every leg over it red at the boundary, and reads as a
failure of the change that came after. The commit body names the fixture and what it now answers, the
way it names any other truth it moved, so a reader can see which change made it answer differently.

- MCP protocol tests are transcript fixtures, `test-data/fixtures/mcp/*.jsonl`: each request line is
  followed by the expected response line, or `null` where none is sent. Placeholders: `${KB_ROOT}`,
  `${VERSION}`, `${ID1}`, `${ID2}`, `${REPO}` (the repository root) and `${STATE_DIR}`, applied to
  requests as well as responses. The detached-build test needs a kbase binary to re-execute:
  `TestMain` runs the test binary as kbase when `KBASE_TEST_RUN_AS_KBASE` is set. The runner stages
  the results fixture, inserts `kb-root/b` and `seed.yaml` through `insert-claim-entry --create`,
  and requires exactly the expected lines on stdout.
- Integration recipes: `test-integration-<process>-<corpus>`; shared corpus fetch
  `prep-test-integration-<corpus>`; anything needing network or a live provider is excluded from the
  hermetic omnibus `test-integration` and says so in its `[doc]`, and a recipe that gains a tool
  dependency — pandoc, python3, the adjagent clone — leaves the omnibus the same way, what needs no
  tool staying in it as a recipe of its own.
- `just` never syntax-checks a recipe body: it parses its own syntax, not the shell inside a body, so a
  body can be invalid shell and still parse. After changing a `[private]` body, render it and check the
  shell by hand — `set -o pipefail; just --dry-run <private-recipe> <its args> 2>&1 | bash -n`. Both
  parts are load-bearing: `--dry-run` renders to stderr, so a plain `| bash -n` feeds it an empty stream
  and passes vacuously, and `pipefail` keeps a justfile that did not parse from reading as a pass. The
  check runs only when the justfile parses — that failure is loud already at the next `just` invocation —
  and it does not replace running the recipe. A habit, not a target or a script: there is deliberately
  no `just check-recipes`, because a checker built on `just --dry-run` cannot run when the file it would
  check is the thing that is broken.
- The results fixture ships no claims: `test-data/fixtures/results/kb-root` holds its papers and nothing
  else, so a leg needing a claim to score cannot run against it as it stands — it looks ready and is
  not, for that purpose. Copy the fixture's repo to a scratch directory and insert what the leg needs
  through `insert-claim-entry --create`, as the transcript staging above does.
- A Go test that must drive `kbase mcp` over pipes the recipe holds starts no process: the recipe
  starts the server on two named FIFOs and passes their paths with `-args`, and the test opens the
  write end first, so neither side waits on the other's open.
- Every integration test preserves its full log, its stats, and its run artifacts under
  `test-data/transient/<test-name>/` — nothing is written to an ephemeral location and tossed. A leg
  that must run outside any repository runs in a system temporary directory and copies its whole
  run tree under `test-data/transient/<test-name>/` before removing it. Every
  integration test passes `--state-dir` under that directory to each command that uses the state
  store, so nothing reaches the user's real `$XDG_STATE_HOME`; a command that never touches the
  store (`refresh`, `verify`, the queries) takes no such flag. `build` resolves and creates its
  state store before its first refusal; every recipe invocation of `build` takes `--state-dir`,
  refused outcomes included.
- Unit tests pass hermetically (no network, no corpus) beside their packages; their `go test` log is
  preserved under `test-data/transient/unit_tests/`. Corpus-driven property tests skip when the
  corpus is absent and are wired into an integration recipe where it is guaranteed present.
- Observational evidence (counts, measurements, A/B numbers) is emitted to an inspectable location,
  never left in agent reports or scrollback.

---

## Dependency Policy

Stdlib-first; a new module requires a ruling. Sanctioned:

- `github.com/spf13/cobra` and `github.com/spf13/pflag` — the CLI.
- `github.com/BurntSushi/toml` — configuration files.
- `go.yaml.in/yaml/v3` — YAML; not `gopkg.in/yaml.v3`.
- `golang.org/x/text` — sanctioned only while a package imports it.

Add with `just add-dependency <module>@<version>`, only after the import exists — `go mod tidy`
drops a dependency nothing imports.

---

## Logging

Project-specific bindings only (the general logging doctrine lives in the coder agent definitions):

- `internal/log` is the **only** logging seam (four-method `Logger`: `Debug`/`Info`/`Warn`/`Error`,
  alternating key/value pairs). Built **once**, in the composition root (`cmd/main.go`), from the
  persistent `--log-level` (default warn) and `--log-file` flags. `--log-file` tees; it never
  redirects.
- **Threaded, not global**: a component that logs takes a `log.Logger` parameter, so its logging is
  visible in its constructor signature. No package-level logger, no setter; `log.Discard()` covers a
  caller with nothing to hand it.
- Never call `slog` directly, and never use `fmt.Fprintf(os.Stderr, ...)` for a diagnostic. Verb
  *output* — a summary line, a user-facing warning — is a different channel and stays on the verb's
  `Stderr` writer.
