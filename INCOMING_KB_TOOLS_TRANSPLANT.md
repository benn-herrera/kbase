# Incoming: the kb_tools pipeline transplant

**Status:** context capture from working discussions 2026-09-12 and
2026-10-01. Nothing here overrides SPEC.md or ARCHITECTURE.md yet. When the
port lands, this document disperses into ARCHITECTURE.md / SPEC.md per the
house doc taxonomy and is deleted.

---

## Bottom line

**The build target is exactly what kb_tools builds** (owner, 2026-10-02). None
of kbase's own KB design carries over — no summaries, no page splitting, no
kbase page grammar — because none of it ever worked.

**kbase's tree-design approach inverts.** The pipeline blueprint in
`../adjagent/kb_tools` derives the document skeleton mechanically and asks
inference only to *refine an existing structure*. kbase stage 3 does the
opposite — the heavy tier designs the tree from the survey, at a declared
no-fallback seam.

**Where upstream stands (owner, 2026-10-01).** The mechanical pipeline is
essentially settled. The *location* of every inference pass is settled; the
prompts and their arrangement are still under development. Builds with
inference enabled run against local inference alone, and every inference
stage is a constrained, narrow ask with no tool use and no file reads — so
nothing in the design is beyond this repo's existing call runner.

**Consequences for the port:**

1. **Port the mechanical pipeline first.** It is the settled part and most of
   the volume, and under upstream's `--no-inference` it delivers a real KB —
   so kbase reaches a complete, checkable deliverable before any model is
   involved.
2. **Build each inference seam as plumbing; import its prompt as text.** The
   seam — call runner, verifier, retry — is kbase's and is built where the
   pass sits. The prompt is developed and proven upstream and arrives later;
   while it is still moving there, a change costs a text swap, not a redesign.
   Tuning prompts here instead would give a bad run two candidate causes with
   nothing to separate them.
3. **Build no further repair machinery around stage 3.** The accumulated
   rulings there — content floor, merge-direction preference, cross-parent
   merge accounting, index pruning on absorption, candidate cap, mechanical
   pre-batching, absorbed-container call skipping, interposition — are patches
   around *the model proposed a tree*. More of them is more to unwind.
4. **The harness is the part that stays.** Orchestration, resume, delivery and
   verification machinery in this repo is solid and is not what the transplant
   replaces.

---

## Why: the failure this exits

kbase never reached a satisfactory mechanical process because it was solving
too many problems at once, and its history stalled grinding in mincing steps
within a consistent neighborhood of failure.

SPEC.md §6 already states the honest version: every prompt definition is a
marked stub, so *a build today is evidence about the machinery, not about
tree-design or summary quality*. The machinery was therefore being tuned
against unmeasured output. The volume of ruled repair machinery clustered
around stage 3 is the signature of that.

The upstream work is the escape hatch: overriding design choices **already
proven to work**, arriving as a part that mostly fits rather than a shape to
be determined from scratch. That also changes what a misfit means — a failure
during integration is diagnostic (it names where this corpus class differs
from the one the blueprint was proven against) rather than another
uninformative "still wrong."

---

## Scope: LaTeX only, and why the old breadth was part of the failure

**One of the problems kbase was solving at once was too many input sources**
(owner, 2026-09-12). Predicting the common overlap of different document types
for a problem space not yet explored is unwise, and this project did not escape
that unscathed. **LaTeX is the only input on the design table for the
foreseeable future** (owner, 2026-10-01) — not a first focus with others
queued behind it. Software is malleable: if another input type is ever added,
the generalization gets found then, from two real cases rather than from one
and a guess.

That generality splits in two, and the halves should not share a fate.

**The half that now pays off is the format seam itself.** ARCHITECTURE.md §4
makes the survey artifact the format-independence boundary, confines parser
code to an adapter package, enforces it with an import-policy test over the
whole module, and *deliberately defers the adapter interface* until a second
implementation exists so it can be derived rather than guessed. That restraint
was right, and a general LaTeX processor is precisely what it was built to
receive.

**The half that was speculative is breadth of formats, and Markdown as the
assumed shape of the world.** Carried in the contract today: MDX read as a
document with JSX and `import` lines surviving as inert text (§4, ruled
2026-08-18); the `.md` → `.mdx` extension ladder in link resolution (SPEC §4.5);
the PDF preprocessing-adapter slot; and the two-address-space resolver, which
exists because *documentation site generators* serve `page.md` at `page/` — a
Markdown-web concern with no LaTeX analogue at all.

**What this makes stale: ARCHITECTURE.md §1's "Why it exists" and both
validation targets.** First target use is stated there as interrogating large
tech doc sets, with target 1 Rojo and target 2 Roblox `creator-docs` — both
Markdown. Under a LaTeX focus those are the wrong shakedown corpora. The
replacement is not invented here, but note that a LaTeX focus is what makes
this project coherent with its own stated market (an academic lab, running
local inference, unable to audit a general agent harness) and with kb_tools'
own corpus. Worth stating rather than leaving to be inferred.

---

## Where the parts come from

**`../adjagent`** — an agent- and command-set *generator*. Single-sourced
templates rendered per model family (`templates/family/gemma-4.toml`), installed
into consuming projects as hash-verified artifacts. It supplies **vetted agent
definitions** and, in Claude Code, a proven harness to test them against the
target models. `templates/agents/kb-docent.md.tmpl`,
`kb-maintainer.md.tmpl` and `kb-claim-scorer.md.tmpl` are the KB-side set.

**`../adjagent/kb_tools`** — a stdlib-only Python KB toolchain, named in
ARCHITECTURE.md §11 as this project's "true ancestor." That description is now
badly out of date: kb_tools is a complete build pipeline, not a set of gates.
It supplies the **pipeline blueprint**. It was built to be brought back here
for appliance implementation; that was the intent from the start.

**This repo** — the appliance that has to ship, and the harness.

---

## The blueprint's spine

Settled, per the owner; process details around it are still moving.

**Mechanical passes produce a surprisingly high-quality draft KB — including
the claim graph.** Inference is invoked as a *targeted utility* over that
draft. The ask is never "construct this from nothing."

From `kb_tools/SPEC.md`, The Driver's Contract:

- *The build derives the tree and grades none of it… **A seat is handed the
  tree; no seat proposes one.***
- *The build renders the leaves and copies nothing through inference.*
- *The build authors the claim graph and grades none of it* — structure is the
  build's, every rigor value written as the `*pending*` literal, grading a
  later pass over work that already exists.

Both halves transfer. THESIS.md is shared with kb_tools (owner, 2026-10-01):
the navigation hierarchy *and* the argument hierarchy are drafted
mechanically, and inference refines each.

**The tell is `--no-inference`.** It is not this project's `[dev] tree_plan =
"mechanical"`. kbase's switch is a dev escape hatch — a tree nobody designed
must say so, no summaries, no provider dialed, existing so a hermetic
end-to-end test has an entrance. kb_tools' is a **first-class delivery mode**:
a real KB, with every ledger op, coverage check, barrier and write still real,
and each stage boundary naming what was dropped so a reader is told rather
than left to infer it. Same mechanism, opposite status.

**The asks are narrow by design.** The return communication protocol is built
to model strengths — not "one-shot this exact output format freehand with
index arithmetic in it." Claude Code *happens* to be the harness and retains
full agentic tooling, but the asks are specifically designed not to need it.
Definitions proven there therefore port to this project's one-shot call
construct.

Note that half of this lesson already landed here independently:
ARCHITECTURE.md §3's *the model supplies VALUES; the machine owns SYNTAX*
(ruled 2026-08-18), with the 833-container shakedown's ~4% rejection rate as
evidence — every rejection a brace or a fence from a model whose grouping was
fine. What is **not** yet fixed here is the other half: the taxonomy answer
grammar is narrow, but the *ask* is still construct-from-nothing. That is the
part the blueprint replaces.

---

## What transfers, and what does not

**Transfers: stage semantics.** What each stage derives, what it asks, what it
hands across the boundary to the next.

**Does not transfer: the outer loop.** Both projects have a full orchestrator.
`kb_driver` has a ledger, boundaries, barriers, a run lock, replay,
`--no-inference` and `--through`. This repo has the phase-op table, the call
runner protocol, chain-stamped artifacts and the resume scan. Only one of them
has to ship inside a binary, and this side already has the outer loop in the
form it needs.

**Why not simply ship kb_tools.** Claude Code is a non-starter as a delivered
harness for the target market: an academic user, even with inference
redirected to a local endpoint on the LAN, has no way to know what the harness
code itself transmits. That is what the appliance exists to answer, and it is
what makes ARCHITECTURE.md §2's single-binary / embedded-prompts invariant
load-bearing rather than a tidiness preference. §2 currently states it without
a rationale; it has one.

**Known divergences to expect at the seam.**

- **Claim graph — in scope, reversing ARCHITECTURE.md §1.** §1 rules it out
  by name as a non-goal ("No claim graph. This is tech-doc dissection, not
  paper/proof dissection."). THESIS.md makes the argument hierarchy half of
  the thesis, so that non-goal is withdrawn and the claim graph transfers with
  the rest of the blueprint.
- **LaTeX.** kb_tools deleted its vendored reader and now shells out to
  **pandoc**, one parse per volume root, cutting documents from a single
  whole-volume rendering so theorem numbering and citations resolve once. It
  takes that as an enumerated exception to stdlib-only, with a named
  `PandocMissingError`. kbase takes pandoc too, but reads only its JSON — see
  the 2026-10-01 decision below.

### Decided 2026-10-01: pandoc's JSON reader, our Go on both sides of it

**pandoc is the LaTeX reader** (owner, 2026-10-01), arrived at by elimination:
building a parser — even a partial one — was ruled out because LaTeX's long
tail never ends; WASM engines, a Perl converter and other stacks were ruled
out as complexity outside our control; and pandoc is the reader with evidence
behind it, characterized failure modes and all, in kb_tools.

**The shape is a hybrid with no hook layer.** Go source pre-passes before the
parse fill the reader-level holes kb_tools documented (`\newtheorem` display
names, `\newenvironment` declarations, unloadable includes checked before
parsing, drawing environments). pandoc runs LaTeX→JSON only — no filters, no
citeproc, no Markdown writer. Go walks the JSON and writes both the Markdown
and the metadata chunk. kb_tools' Lua filter exists to steer pandoc's
Markdown writer; with no writer in the path it has nothing to do.

**Owed before it is built:** whether a pinned pandoc ships in the dist tarball
or is required on the host (GPL obligations attach to shipping it); the pinned
version, enforced against the JSON's `pandoc-api-version`; and loud refusal for
a document pandoc cannot parse at all (5 of ~55 in upstream's sampling), naming
the paper and the reader's error.

**Owed too: theorem and equation numbers that match the typeset document.**
Readers cite results by printed number, and upstream's hand-named-claim
harvest (*follows directly from Lemma 4.6*, no `\ref`) joins on it. Upstream
records that pandoc's own counter can disagree with the author's — the
author's *Proposition 1.3* rendered as *Proposition 17* where numbering runs
within sections — and such a mention then joins nothing. Wherever kbase takes
numbers from, they must agree with the typeset PDF's.

**The pre-passes are the part that can accrete, so three rules hold them.**
Each pre-pass names the reader-level hole it fills, and the list of them lives
in one place. Anything the reader drops that no pre-pass covers is counted and
reported, never absorbed silently. And a list that keeps growing is the signal
to reconsider the reader, not to add another pre-pass — the unmonitored,
locally-reasonable accretion that made LaTeX itself what it is is the failure
these rules exist to prevent.

The rest of this section is the reasoning from 2026-09-12, when pandoc was a
last resort. Its argument against the Markdown intermediate and its smuggled
markings still holds — reading the JSON is how this decision honours it.

**The preferred outcome is one processor, not two.** kb_tools' path is LaTeX →
Markdown → KB, and that intermediate Markdown has to carry LaTeX semantics
Markdown itself has no place for. It does so in band, as markings smuggled
into the rendered output: `<a href="…#…" data-reference-type="ref"
data-reference="…">` on every rewritten cross-reference, `<span id="thm:bif">`
wrapping a labelled block, `\label` surviving verbatim inside a maths fence.
The claim-graph join then reconstitutes the structure by matching those
markings back up.

That intermediate is the hack to remove, and kb_tools' own SPEC says why in
the plainest terms available: *Both ends fail silently… A changed reference
form yields zero anchors, an edgeless claim graph, and a green build.* A
processor that parses LaTeX generally and produces the survey artifact
directly holds the structure natively, and never has to smuggle, rediscover or
re-join it.

**This reopens a ruled decision, and the reconciliation is owed at import
time.** ARCHITECTURE.md §4 ruled 2026-08-12 that *LaTeX is a converter, not an
adapter* — a separate module with its own `go.mod`, writing `.md` that kbase
ingests through the ordinary Markdown front door. A single general LaTeX
processor is the opposite of that: an adapter, `internal/survey/latex`, which
is the shape the ruling withdrew. The format seam was built to receive one —
§4 defers the adapter *interface* until a second adapter exists so it can be
derived rather than guessed, and this would be that second adapter — but the
ruling stands until it is revisited.

Whichever way that goes, the **one parse per volume root** insight holds:
notation, theorem numbering, citation rendering and the bibliography resolve
once across a whole volume, never per document. Slicing independently
re-rendered source instead lets each slice compute its own numbering, and the
defect is a plausible-looking *Theorem 1* in every leaf.

**What survives the transplant unchanged.** A derived skeleton retires the
no-fallback seam and the repair-the-model's-answer class. It does **not**
retire the structural rules themselves: coverage tiling, the content floor,
the whole-file exemption and the naming guarantee are properties of any tree
over an arbitrary corpus, derived or not. They stop being adjudication and
become checks.

---

## The handoff from the mechanical phase to inference

**Markdown plus a metadata chunk, never Markdown alone** (owner, 2026-10-01).
Upstream passes the rendered Markdown tree as the *sole* artifact from its
mechanical phase to its inference phase, and gets structure across by
smuggling it into that Markdown. kbase states what the inference phase needs
instead: the Markdown, plus a metadata chunk carrying blocks, labels,
references, citations, block kinds, display names, numbers and proof-to-subject
bindings as explicit records with positions into the Markdown. The join
becomes data — a missing record is a schema failure, not the silent edgeless
graph upstream's own SPEC warns of — and the Markdown stays plain text. It is
the survey artifact's existing pattern: a sidecar with offsets beside the
bytes it describes.

A reference record carries **the word the page shows before it** and **the
referencing macro's own type**. Upstream filters dependency candidates on that
word (*Section 3*, *Fig. 2* name nothing a premise can hold) and recovers it
from rendered Markdown; in pandoc's JSON it is the text node immediately
before the reference, and the macro type tells a `\ref` — the author wrote
the noun — from a `\cref`, whose noun is generated at typesetting.

**The KB is an analysis and development format, not a presentation format**
(owner, 2026-10-01). So the walk that writes the Markdown *translates*; it does
not render. Three classes:

- **Translate** — text treatments that carry meaning: emphasis, bold, code,
  lists, footnotes, quotations, accents, dashes and quotes.
- **Carry intact** — mathematics, byte-exact. A hard requirement.
- **Drop** — the frills: spacing, sizing, fonts, colour, page layout.

---

## kbase as personant's KB toolset (owner, 2026-10-01)

**kbase + personant integration is the shape of the future.** A prior decision
in either project that prevents it is removed rather than designed around —
personant's current refusal of mutating tools, its 30s tool cap and its 8 KB
result cap included.

**kbase owns KB building and every mechanical maintenance function kb_tools
has** — refresh, verify, the write API's ops, queries, the claim-graph render.
Each is a kbase subcommand, and personant's harness integrates them as builtin
tools the model calls directly, with no freehanded file editing. That is what
maintains a KB kbase delivered, and it moves kbase from a run-to-completion
appliance to a long-lived toolset; ARCHITECTURE.md §1's framing changes with
it.

**`claim-graph.svg` is one more automatically maintained file** (owner,
2026-10-01): kbase renders it, it is derived from the KB's index alone and
authored by nobody, and refresh mints it while verify checks it fresh —
byte-deterministic, so a stale or hand-edited sheet is a freshness failure
like any other derived file.

**The communication format is personant's CONVENTIONS.md "Serialization
format" rule**: JSONL for flat records one per line, YAML for nested or
document-shaped output, TOML for flat hand-edited configuration, and JSON
resisted. **Tool results are YAML on stdout.** Consequences:

- stdout carries exactly one YAML document and nothing else; every
  human-facing line, progress note and log record goes to stderr.
- A refusal is a result too — a YAML document naming the outcome, the reason
  and every offending item, with a nonzero exit — so a model calling the tool
  reads why rather than an empty stdout.
- Key order is fixed and strings are quoted on emit, so the same call yields
  byte-identical output.
- kbase's single failure exit code likely gives way to distinguishable
  outcomes (kb_tools' write API separates written / refused / retry
  unchanged); personant's tool layer decides what it needs.
- ARCHITECTURE.md §3's "machine-to-machine encodings stay JSON" is superseded:
  the metadata chunk's records become JSONL, and the artifact set moves off
  JSON. **`gopkg.in/yaml.v3` is a general kbase dependency** (owner,
  2026-10-01) — already approved in personant; the import-policy test that
  confines `yaml` to the Markdown adapter loosens with it.
- **The KB metadata layer stays format-compatible with kb_tools** (owner,
  2026-10-02; supersedes the 2026-10-01 "new KBs build out in YAML"). kb_tools'
  maintenance tools work, and comparing kbase's behaviour byte-for-byte against
  working examples is critical, so frontmatter, registers and `.index/*.jsonl`
  keep their current formats. JSON vs YAML is a parser choice made at load
  time over identical schemas. Breaking changes to the Markdown formats come
  after kbase is a proven system, not before.
- Model-facing answer grammars are untouched by any of this.

---

## Upstream findings to reconcile at the port (read 2026-10-01)

From kb_tools' contract docs as they stood on 2026-10-01; recheck against
upstream before acting on any of them.

1. **Upstream has no summary stage.** Its document-tree contract makes an index
   a heading plus a child list and nothing else, consistent with THESIS.md's
   silence on summaries. kbase's stage 6 — summaries, leaf-group cards, the
   entry-point summary block — has no upstream counterpart. Resolved
   2026-10-02: kbase builds what kb_tools builds, so no summaries.
2. **Upstream has no page-size budget and no splitting.** A leaf is a whole
   section however large; a paper with almost no sectioning collapsing to one
   leaf is an open hole on upstream's roadmap, with no reader for a finer
   boundary set. kbase's stage 4 — enumerated cut candidates, the light-tier
   adjudication fold, `(k/n)` parts — is an answer to exactly that gap, so on
   this point ideas may flow upstream. kbase itself builds what kb_tools builds
   (2026-10-02), so it does not split either.
3. **Upstream's claim-graph stages read the rendered Markdown.** They find
   blocks, anchors and labels by parsing pandoc's output conventions. The
   metadata chunk above replaces that; the claim-graph stages read records
   instead, which changes the shape of their port.
4. **Upstream's model answers are JSON envelopes**, with composed text in raw
   delimited blocks beside the JSON. ARCHITECTURE.md §3 here rules JSON out of
   every model answer (*the model supplies VALUES; the machine owns SYNTAX*).
   The answer grammar of each imported ask needs reconciling at its seam.
5. **What maintains a KB kbase delivers** — answered: kbase itself, through
   its subcommands as personant's builtin tools (the section above). Upstream's
   claim graph ships `.index/`, registers and a claim-graph sheet, and grading
   happens afterwards through its Python write API; each of those ops becomes a
   kbase subcommand.

---

## What is actually solid in this repo

Harness machinery, and it is the expensive-to-rebuild part:

- the call runner protocol end to end — build, frozen-prompt assertion,
  transport, mechanical validation, one informed retry, seam resolution;
  effort and retry as per-definition declarations; truncation classified apart
  from rejection; rejected responses kept as evidence
- chain-stamped artifacts, the `Valid | Absent | Invalid` verdict model,
  cascade failure, poisoned lanes, lazy chain description
- the temp-work store, the job lock, atomic writes and the durability ordering
- the delivery manifest and the interrupted-versus-finished predicate
- config and provider loading, and `configure`'s byte-preserving update

**Not on this list, deliberately: the KB-building itself.** Stage 4's
near-perfect live compliance is evidence that the value-only ask works, not
that the boundaries it chose were right — nothing has measured the latter.
Protocol adherence is not quality, which is SPEC.md §6's point applied to this
project's own self-assessment.

---

## Risk to watch first

**Input variety is the likeliest place filing turns into recasting.** Under
the LaTeX focus the skeleton comes from LaTeX's declared sectioning, as it does
upstream, so the risk is not the corpus class but the spread within it — what
THESIS.md calls *why the ideal does not hold*: under-marked arguments,
sparse or absent sectioning, `\input`-chained volumes. Upstream's own design
was taken largely against one multi-volume corpus, and a clean run over it
measures that corpus rather than the process; kbase inherits that exposure
along with the blueprint.

Stopping rule: if filing starts turning into recasting, the difference is
corpus-class rather than tolerance, and that is a design question to raise
rather than an implementation to push through.

Worth knowing which direction ideas flow on one point: upstream's roadmap (as
of 2026-09-12) proposed widening the enumerated boundary set to paragraph
breaks, and mechanical `(forced-split I/N)` titling for oversize sections. **kbase already ships
both** — `heading | fence | paragraph` cut candidates, and `<slug>-<k>.md`
titled `(k/n)` — and already realizes that item's closing hope that inference
becomes the exception path rather than the stage: a corpus whose groups all fit
one page makes zero light-tier calls.

---

## Until the import

- Leave stage 3 alone.
- Leave the prompt definitions as the marked stubs SPEC.md §6 declares them
  until each is imported from upstream.
- Harness work, delivery/resume work and gate work are unaffected and
  proceed normally.
- Any LaTeX work follows the 2026-10-01 decision above: pandoc's JSON reader
  with Go pre-passes and a Go walk, never pandoc's Markdown writer.
- Stale spots in ARCHITECTURE.md, each worth correcting whenever its section
  is next touched, import or no import:
  - **§11** describes kb_tools as a "stdlib-only Python deterministic spine —
    refresh, verify, link checking." It is a complete build pipeline now.
  - **§1** states the project's first target use and both validation targets
    as Markdown tech-doc sets (Rojo, Roblox `creator-docs`), and rules out the
    claim graph as a non-goal. See "Scope: LaTeX only" above, and THESIS.md.
  - **§4** rules LaTeX a separate `tex→md` converter module feeding the
    Markdown front door, with pandoc a dev-time oracle only; ROADMAP.md parks
    that module. The 2026-10-01 reader decision above supersedes both.
