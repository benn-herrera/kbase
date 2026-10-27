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
recipe runs. `just dist --tag` also runs a lightweight, local `git tag v<version>` once every stage,
signing included, has succeeded; it pushes nothing. It refuses before building anything if that tag
already exists or tracked files have uncommitted changes.

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
  lives here alone — `ModeOf`, `Resolve`, `Select` — and reads no file, opens no client and writes no
  stderr (a refusal's remedy may name the command that fixes it, such as `kbase configure`): the key file `KBASE_API_KEY_FILE` names is read by whoever dials (in
  `cmd`, `internal/modelclient`'s registry; in `show-config`, its preflight). The package imports no kbase package but
  `atomicfile` and `result` (it refuses in `result.Refusal`, each item carrying its check), never
  `model` or `log`: it returns its faults as data, so each caller decides whether one is a warning
  or a result field. A `[models]` value is parsed by one function,
  `ParseReference` (`<provider>/<model>`, split at the first `/`), and `Reference.String` writes such a
  reference back; an environment-supplied id is a bare id of the environment's endpoint, which nothing
  parses. No site splits an id or concatenates a reference by hand.
- **internal/model**: `Endpoint{Name, BaseURL, APIKey, Protocol, Decisions, PromptCacheKey, IdleConns}` is the
  transport-level slice of a provider — `Protocol` saying which chat surface its entry speaks
  (`openai` unless the entry says `anthropic`), `Decisions` whether it speaks the Decisions shape
  (`systemone` unless the entry says `decisions`), and `PromptCacheKey` the run every OpenAI-surface
  ask of it carries, and `IdleConns` the idle connections per host its client keeps (the invocation's
  concurrency; zero leaves the defaults) — and the one mapping from configuration's vocabulary onto `Protocol` and
  `Decisions` (`chatProtocol`, `decisionsSurface`) lives in `internal/modelclient`. The chat asks — the overview and each scoring call — go through
  a `Slot`'s `Chat`: one tool-less request, re-issued identically on an incomplete stream up to the
  attempt count (ARCHITECTURE §12). An attempt's outcome is the classification of what ended it, and
  the two failures are not interchangeable: an endpoint that answered with a non-2xx status refused
  the request and is `ChatProviderRefused` (its `StatusError` matched with `errors.As`, its status and
  body in the reason), where `ChatTransportFailure` is a connection, a name, a certificate or a
  stream that failed with no answer — reading one as the other sends a diagnosis to the path to a
  provider that is answering. The multichoice asks go through a `Slot`'s `Decision`, one `POST` to whichever
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
  beside it. The one self-exec owner is a file, not a package: `internal/mcp/launch.go` alone may
  exec the kbase binary, and the test fails on any other file that does. Four owners are the whole
  list — pandoc, git, `dot` and that self-exec — and a process started anywhere else fails the
  test. The policy scans share one module walk (`cmd/modulescan_test.go`), so they agree on which
  files are the module's source. The scan reads test files as source too — the trap: a package
  test that compiles its output under `pdflatex` has nothing wrong in it and fails the tree. A
  verification needing a binary no package owns is never a package test: it is a runner recipe
  (recipes exec freely). Evidence gathered for the change at hand is not a verification but a
  probe — a script run by hand in scratch, its log cited in the report, with no durable home. A
  probe whose finding must keep holding becomes a verification: the check lands in a recipe. A
  probe whose result a commit or plan cites keeps its command in the commit message, so a later
  reader can re-run it.
- **A pre-pass beyond I3's two is an ARCHITECTURE change, never a patch.** Nothing outside the
  registry massages source.
- **The agent set is adjagent's, never edited here.** Definitions are installed from the sibling
  `../adjagent` checkout and are not tracked in this repository; a finding that needs an agent
  change lands in adjagent, and the set is reinstalled from it.
- **File formats follow personant's CONVENTIONS "Serialization format" rule:** JSONL for flat
  records, YAML for documents, TOML for flat configuration — with one named departure (SPEC §10):
  the KB's two record files, `.index/nodes.yaml` and `.index/edges.yaml`, are each one YAML document,
  a block list of block mappings, so the whole metadata layer has one serialisation. The trap: a
  record spans several lines, so a line-oriented tool — `grep`, a line count, a line-wise merge —
  sees fields, never whole records; read them through the record loader (below).
- **A superseded format's parser lives in `internal/formats` and nowhere else** (I8). The trap:
  keeping a reader for the old form "for compatibility" in `kb` or `index`, which is the
  multi-version reader the design forbids. A converter's test is a golden pair, old bytes in and new
  bytes out. A format package may change until a kbase release ships its version, and is frozen
  after.
- **A `kb-format` patch bump moves the stamp and every fixture that pins its bytes, and stops the
  chain where its last converter does.** Two halves, both load-bearing. `kb.FormatVersion` moves, and
  every `.index/format.yaml` carrying the stamp — or a fixture whose bytes pin it, or a derived
  line the change rewrote — is rewritten in
  the same commit: the loader reads an older patch of the same minor as current — no chain runs — and
  restamps it on the next save, so what moves under a fixture left behind is a leg that pins bytes
  after a write, not one that only reads. And `formats.Chain` must stop at the form its last converter
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
- **The edit and write tools govern text changes, not filesystem operations.** A move is `git mv`, a
  deletion `git rm`, and staging either is part of the landing's set; the landing commits once.
- **An MCP tool is a subcommand's binding** (I9). The trap: a tool listed by hand, a description
  copied into a tool, or a schema typed beside the command's flags. Every command reaches the tree
  in `cmd/surface.go`, one `AddCommand` per family, wrapped `readOnlyTool`, `readWriteTool` or
  `cliOnly`, which set the cobra annotation `kbase.mcp` = `bound` or `excluded` and the read-only
  hint; `cliOnly` takes the command's reason as a required argument and records it as the
  annotation `kbase.mcp.reason`, and the tool-table test fails an exclusion whose reason is empty;
  cobra's own `help` and `completion` are declared `cliOnly` there too. A test fails an
  `AddCommand` outside that file. The table binds only `bound` commands, and the test derives its
  expectation from the annotations, never from a list, failing a command that declares neither. A
  bound command's flag that its verb refuses to run without declares `kbase.required`
  the same way, and the tool's schema lists it among `required`; the trap is a refusal whose flag
  carries no mark, so a model reading the schema cannot tell the flag must be given.
  The exception is a tool no command stands behind: listed, and only where the tool-table test
  (`cmd/mcp_test.go`) pins the set to the `bound` commands plus exactly those tools and the tool
  returns through `mcp.ToolResult`, so text, structured content and `isError` cannot drift from a
  command's. `rewind-build`, built in `internal/mcp` and declared in `cmd/surface.go`, is the only
  one: read-only, schema and description its own. A flag annotated `kbase.mcp` = `excluded`
  (`mcp.ExcludeFlag`) is not part of its command's tool schema: `mcp.CommandSchema` skips it.
  Protocol tests are transcript fixtures under `test-data/fixtures/mcp/`, never strings asserted in
  code.
- **Dev-only diagnostics are compiled out of the shipped binary.** A diagnostic for developing kbase
  itself — today `score --replay` — lives in files carrying `//go:build !kbasedist`, code and tests
  alike; `just dist` builds with `-tags kbasedist`, so the release binary does not contain it. The
  untagged code reaches it only through a hook its tagged file sets. Its flags carry the flag-level
  `kbase.mcp` = `excluded` annotation, so dev and dist builds serve one MCP tool table. SPEC states
  nothing of it; ARCHITECTURE does. The trap: an untagged reference to tagged code, which builds by
  default and breaks only the dist build — `edit-gate` vets with the tag for that reason, and
  `test-dist-no-replay`, in the hermetic omnibus and run first by `test-dist`, runs
  `TestDistBuildHasNoReplay` (`cmd/dist_test.go`, tagged `kbasedist`).
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
- **Never spell the node-kind list.** It is defined once, in `internal/formats/format300`, and named
  everywhere else through `kb`. A site that *iterates* — a census, a breakdown, a union, an
  ordering — reads the list and names no kind. A site that *branches* names kinds, and its table
  must be total over the list. A count derived from the list ("all four kinds") is the same copy in
  a shorter form. Nor a node id's prefix or a minted id's grammar: outside `internal/formats`, code
  composes and matches ids through `kb.ClaimPrefix`, `kb.AnalysisPrefix`, `kb.ExperimentPrefix`,
  `kb.WorkPrefix` and `kb.AxiomPrefix`, and the minted grammar, prefix included, through
  `kb.IDBody`. A literal id in a test fixture or in help prose is not a spelling.
- **Never spell a relation.** The six relation names (`RelationSupports`, `RelationFounds`,
  `RelationLifts`, `RelationRefutes`, `RelationReferences` and `RelationDemoted`) and `Relations`,
  the set of them, live in `internal/formats/format300`, and `ScoringRelations` in
  `internal/kb/schema.go`; every other site names them through `kb` (`kb.RelationSupports`,
  `kb.Relations`, `kb.ScoringRelations`), naming the constant. `cmd/relationpolicy_test.go` is an
  AST scan that fails on a spelling elsewhere, with a short allowlist of files where the same word
  means something else.
- **Read a record file only through the record loader, and write one only through its encoder.**
  `kb.LoadGraph` (rigor clamped, for a reader) and `kb.LoadAuthoredGraph` (rigor as the file holds
  it, for a writer) decode both files strictly through `format300`; `kb.EncodeNodes` and
  `kb.EncodeEdges` render them whole. The trap: a second record struct or a `yaml.Unmarshal` beside
  them, which reads a missing `bearing` as the zero `kb.Bearing` — the judgement `0.0` — where the
  strict decoder refuses the record (ARCHITECTURE §11, "The zero-value hazard").
  `cmd/recordpolicy_test.go` is an AST scan outside `internal/kb` and `internal/formats` that fails
  on a struct field tagged `bearing` or `manually-scored` and on a function that both names a record
  file's path and calls the YAML package; a path handed to a YAML call in another function is
  beyond it.
- **Keep no prose copy of an op's key vocabulary.** The vocabulary is closed, total and per-op,
  stated once in `internal/write`. A second statement — in a doc, a docstring or a comment — drifts;
  point at the definition or the refusal message it produces.
- **Who supplied a value is an options field, never a flag or a values key.**
  `write.Options.Provenance` decides the `manually-scored` flag on the rigor and bearing values a
  write gives (the record types SPEC §8 "Manually scored" names); its zero value,
  `ProvenanceCaller`, marks them a person's, and only kbase's own machine writers — the build's
  write runner, a scoring run's landing and re-price — pass `ProvenanceMachine`. The trap: a `--provenance` flag, a values key or an MCP property, through
  which a session could mark its own value model-scored and have `score` overwrite it.
  `TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`) fails on a flag or a tool property
  that names it.
- **A file kbase generates is regenerated whole.** The stamped `AGENTS.md` and `CONVENTIONS.md`, the
  register pages and the sheets are kbase's output: a new format or template rewrites each from its
  source, whatever it holds. The trap: merging new text into what stands — a matcher for the old
  text, both forms of every passage and a warning for each one not found, for files no user edits.
- **No stage exits on a model's opinion.** Every stage exits on a comparison of artifacts or a
  return code; a new stage does not get a third kind of exit.
- **A model call is made through a `Slot` from the invocation's queue, inside the item `Go`
  launches; the item writes files only in its `after`, once its token is released.** The trap: a
  goroutine pool or semaphore beside the queue, which re-introduces a second bound; a `WithTimeout`
  around `Go`, which times an item out for its wait; groups launched from per-group goroutines,
  which loses submission order; or a common file — a ledger, a record, a progress line — written
  from an item while its ask batch is in flight (ARCHITECTURE I11).
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
  full precision, so a dependent multiplies its source's exact strength. A register page
  spells it — its `- strength:` line and a premise's `(strength …)` — in the fewest decimals that
  read back as it: four in the ordinary case, more where four would lose it, trailing zeros
  trimmed; and the walk never reads that spelling. A surface that shows a value to a reader puts it
  at two decimals — a query result as the number, the trace on a `- strength:` line as the
  spelling.
- **A scoring-model fact lives once, in `internal/kb`.** The thirteen fault ids, the
  word `none`, each id's weight per occurrence and each table's maximum live in `faults.go` and
  nowhere else — the statement table, the method table and the bearing one, which a node's class
  selects among. No table holds the retired pending id; `Price` still reads one from an old ledger
  line. A template names only the ids its class accepts, and carries no figure or word a
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
- **A citation is looked up in the closed text the call carried, never in a register page.** The parse's
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
- **Inside a bracket expression, spell whitespace as `PyWhitespace`, never `\s`.** `PyRE` expands
  `\s` into a bracket expression of its own, and nested inside `[...]` that compiles, without
  error, into a different pattern. Both are `format300`'s; outside `internal/formats` use `kb`'s
  re-exports, and a superseded format package keeps its own copy, as `format200` does.
- **A float computation that decides a ranking sums in Python's order with its compensation**
  (`shortlist.go`'s `pySum`): Go's plain summation ranks a near-tie differently, and a differently
  ranked shortlist asks a different pair.
- **A `justfile` recipe whose body is a shebang script is a script under `tools/scripts/`, named for
  its recipe, and the recipe passes the script its inputs as environment variables.** The recipe line
  is `NAME="{{NAME}}" tools/scripts/<recipe>.sh`, so a command-line variable override reaches the script
  and no recipe re-invokes `just` for a private body. The script runs under its own
  `#!/usr/bin/env bash` with at least `set -u`, reads only what the recipe sets (the recipe's variables
  and parameters, and `just` built-ins such as `invocation_directory()` as variables of its own), and
  opens by refusing an unset one — `: "${NAME:?…}"`, with no apostrophe in the message, which the macOS
  `/bin/bash` 3.2 reads as an open quote. Scripts that need one another call each other by path, not
  through `just`. The one exception to environment inputs is a variadic list a script passes on
  untouched (the volume roots and build flags of `test-integration-build-live-fixture`), which arrives
  as positional arguments.
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
- Any plan that is activated, regardless of its location, gets a post-sprint review section
  at the end. 
  - That section contains the review, revision, and burndown steps. 
  - Unless otherwise specified the reviewers are an architect and a coder reviewing with THESIS.md, SPEC.md,
  ARCHITECTURE.md, and CONVENTIONS.md as their pre-task reading list. 
  - They do NOT get the plan doc. Their job is to review the changes against standards and suitability for purpose, not rehash the implementation. 
  - Their review brief includes reviewing the tests of all touched code
  - The burndown list is written to a separate `<plan>_REVIEW_BURNDOWN.md` and gets the same
    prompt engineer review and revision pass as any other document containing agent instructions.

### Plan dispatchability
A work package is dispatchable only when the following conditions have been met

- It names its scope in files, packages or symbols — never a span of design sections
- Carries the command and its answer, or the quoted line, for each fact it acts on
- A package that changes a type, a signature or a test double names in its scope every file
  that references it — `git grep -l '<Name>' -- '*.go'`, test files included — and carries
  that command and its answer
- States its done-line in runner targets
- Has received a usability review by the prompt engineer (its definition contains the criteria)
- Has been revised by the architect based on the findings of the prompt engineer

A plan whose packages lack these is not dispatchable. It is not offered for approval as a dispatch
plan, and none of its packages is dispatched.

## Coordinator Policy

For the session-level agent managing dispatched coder agents.

**Audit the diff, not the report.** After a dispatched agent reports, audit the DIFF before
accepting — the report is an index, the diff is the evidence. Check the diff against the recurring
violation classes:

- integration tests invoke the app binary; no dev-only entry points, verbs, or test scaffolds around
  the real surface — save a diagnostic the shipped binary does not contain (see "Dev-only
  diagnostics")
- verifications are repeatable invocations; evidence lands in inspectable artifacts, never only in
  the report
- no special-case file lifecycles: every path kbase writes belongs to a location SPEC or
  ARCHITECTURE names
- no gate, pin, or assertion loosened to make something pass — expected-truth updates must state the
  new truth they track
- no second home for a value, string, or mechanism that has one
- nothing beyond the dispatched scope, even improvements

Violations block acceptance: fix (redispatch or direct) before commit. A report that flags a
departure from the package's scope text is redispatched or escalated, never accepted on the
coordinator's reading of what the scope meant. Style and judgment calls
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

**A test helper an external test package needs is defined in that package.** An unexported name does
not cross between `package p` and `package p_test`, and a test-support package that imports `p` cannot
be imported by `p`'s in-package tests — the cycle is refused. Where the two need the same fixture,
each package keeps one copy and the pair states why.

**Where a directory holds both an in-package test file and an external `p_test` one for the same
production file, the external file is `<file>_external_test.go`.** The two cannot share a file name,
and the in-package file keeps the name that matches the production file —
`internal/model/modeltest/` holds `mockllm_test.go` and `mockllm_external_test.go` for `mockllm.go`.

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
  the results fixture, inserts `seed.yaml`'s entries through `insert-claim-entry`, and requires
  exactly the expected lines on stdout.
- Integration recipes: `test-integration-<process>-<corpus>`; shared corpus fetch
  `prep-test-integration-<corpus>`; anything needing network or a live provider is excluded from the
  hermetic omnibus `test-integration` and says so in its `[doc]`, and a recipe that gains a tool
  dependency — pandoc, python3, expect, the adjagent clone — leaves the omnibus the same way, what needs no
  tool staying in it as a recipe of its own.
- A script is checked where it stands: after changing one, `bash -n tools/scripts/<name>.sh`, and
  `just edit-gate` runs `bash -n` over every `tools/scripts/*.sh`, so a syntax error fails the gate
  whether or not the script's recipe is run. That is a syntax check only and does not replace running
  the recipe; a recipe that is LIVE or needs an absent tool is checked by `bash -n` on its script and
  `just --dry-run <recipe> <args>`, whose rendered line must set every variable the script requires.
  `shellcheck` is not a project prerequisite and the gate does not call it.
- The results fixture ships no claims: `test-data/fixtures/results/kb-root` holds its papers and nothing
  else, so a leg needing a claim to score cannot run against it as it stands — it looks ready and is
  not, for that purpose. Copy the fixture's repo to a scratch directory and insert what the leg needs
  through `insert-claim-entry`: a claim to score needs no host, and one a leg needs hosted takes a
  `document` and a `locator`, as the transcript staging above does. A claim given a `rigor` is
  manually scored, and a scoring run skips it.
- A Go test that must drive `kbase mcp` over pipes the recipe holds starts no process: the recipe
  starts the server on two named FIFOs and passes their paths with `-args`, and the test opens the
  write end first, so neither side waits on the other's open.
- Every integration test preserves its full log, its stats, and its run artifacts under
  `test-data/transient/<test-name>/` — nothing is written to an ephemeral location and tossed. A leg
  that must run outside any repository runs in a system temporary directory and copies its whole
  run tree under `test-data/transient/<test-name>/` before removing it. Every
  integration test passes `--state-dir` under that directory to each command that uses the state
  store, so nothing reaches the user's real `$XDG_STATE_HOME`; a command that never touches the
  store (`refresh`, `verify`, the queries) takes no such flag. `build` refuses an unknown `--through`,
  a `--state-dir` inside `kb-root/` and a `--ask-concurrency` under 1 before it resolves and creates
  its state store, and those refusals carry no `state-dir`; its later refusals follow the store. Every
  recipe invocation of `build` takes `--state-dir`, refused outcomes included.
- A scratch copy of a fixture repository under this repository needs its own `.git` (an empty
  directory serves) before kbase runs over it. The trap: kbase resolves the repository root at the
  nearest `.git` upward, so a copy without one answers from this repository's own `kb-root/`,
  silently.
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
