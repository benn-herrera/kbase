# If we build a LaTeX engine, do it right

**Status:** partly planned, not under way. If taken up, it becomes its own
project in its own repository, and this document moves there. Its first
consumer, kbase, reads LaTeX through pandoc behind one package that both reads
and writes (`internal/latex/pandoc`, planned in kbase's `ACTIVE_PLAN.md`);
this library replaces its reader half.

**Owner** means the project owner, reached in the session that dispatches this
work. **"To verify"** marks a claim the architect checks during research,
before planning; the result goes into the project's ARCHITECTURE, and a
falsified claim that changes a requirement goes back to the owner.

---

## The aim

A separate, cross-project library that reads LaTeX **completely**: full macro
expansion and complete semantic comprehension of a document, down to the last
detail anyone might actually care about.

**Interpret, don't parse.** LaTeX's meaning exists only in execution, so the
library runs a faithful TeX engine over the real LaTeX macro library rather
than approximating it. The engine's target is closed and exactly specified —
TeX's primitives (`tex.web`) plus the extensions the modern kernel requires,
on the order of 600 primitives — so the real `latex.ltx`, expl3 and packages
run as their authors wrote them.

## Build on existing work

Adopt what exists; invent only what is new (the event stream, the invertible
semantic layer, the round-trip oracle and its gates).

| Part | Reference |
|---|---|
| Engine behaviour | `tex.web`, the e-TeX and XeTeX change files |
| Engine correctness | Differential traces against the reference engines; Knuth's `trip.tex` and e-TeX's `etrip` serve only as further round-trip inputs |
| Semantic hook points | The LaTeX kernel's hook system (`\AddToHook`, environment and command hooks) |
| Semantic schema | Adopted by literature search (see "The semantic capture schema") |
| Shaping | HarfBuzz (pure-Rust port rustybuzz; to verify) |
| Font metrics | TFM, plus OpenType fonts present in the bundle (whether shaping is in v1 is the architect's, from the corpus's fontspec use) |
| File lookup | kpathsea's search rules, over the bundled macro library and any consumer-supplied one |
| Ground truth | The reference engines `tex`, `etex`, `pdftex`, `xetex` |

## Engine

- **Personality: XeTeX's primitive set**, including the pdfTeX-compatible
  primitives XeTeX carries. `\directlua` is a diagnostic, never executed.
- **Scope:** tokenizer with mutable category codes; full expansion (`\edef`,
  `\csname`, `\futurelet`, `\afterassignment`, conditionals); registers and
  grouping; box and glue arithmetic with real font metrics; `\write`/`\read`
  streams for the two-pass `.aux` file; alignments.
- **No typesetting.** The library is not a renderer: the paragraph builder,
  page builder and output stage are out of scope, beyond the stubs measuring
  packages need. Rendering is the reference engines' job, in the oracle.
- **Format dumping** (the equivalent of `.fmt`) exists from the start: loading
  `latex.ltx` plus expl3 through an interpreter on every run is slow.
- **pdflatex-declared papers:** the architect measures, before planning, what
  share of the corpus needs pdfTeX-specific behaviour (including legacy 8-bit
  `inputenc` input) that XeTeX's primitives do not give, and brings it to the
  owner with three options: add pdfTeX primitives to the personality; exclude
  those papers; or accept their failure within the round-trip share. Papers
  declaring lualatex are out of the corpus.

## Secure by default

Shell escape (`\write18`) is never executed; an attempt is a diagnostic. File
reads and writes are confined to the document's tree, the macro libraries and
a scratch directory. Step, time and memory budgets fail cleanly, with defaults
the caller may raise. A parse is cancelled through the event callback's return
value (see "Interface"); no call is made from another thread.

## Completed artifacts only

The library consumes the files LaTeX's helper programs produced and never runs
the programs: the `.bbl`, makeindex's `.ind`, minted's frozen cache and the
like are used when shipped; without them the content is absent and a
diagnostic says so. The multi-pass loop is the library's own: re-run until
`.aux` reaches a fixed point, bounded, a non-converging document reported.
How events relate to passes (streamed per pass or from the final pass only)
and whether a shipped `.aux` is read are the architect's decisions.

**Bibliographies: the `.bbl` is required.** A `.bbl` is LaTeX the engine
executes like any other input, so reading a paper needs no bibliography
backend. Regenerating a `.bbl` is out of scope. A `.bbl` older than its
citations reproduces LaTeX's own `[?]` and a diagnostic naming the key.

## The LaTeX macro library

The engine knows only primitives; LaTeX itself — the kernel (`latex.ltx`,
which loads expl3), classes, packages, font metrics, hyphenation patterns — is
a standard library written in TeX that the engine executes.

- **The library bundles the most recent TeX Live annual release**, identified
  by its year, with its format pre-dumped. Its contents are chosen by the architect from the licences;
  anything not redistributable goes back to the owner. Bundled files keep
  their licence notices.
- **The API consumer may supply an alternate macro library**, searched ahead
  of the bundled one; older or extra packages, and biblatex versions matching
  older `.bbl` formats, reach the engine this way.
- **The library makes no network access at run time** and fetches nothing:
  no automated fetch of any package or release.
- Older versions join the bundle only where corpus measurement shows a
  failure they fix.

## Derived requirements

- **Provenance through expansion.** Every token carries its origin (file,
  line, column of the characters it was read from); a token produced by
  expansion carries the call site in the author's text and the chain of
  macro definitions it passed through, stored as compact interned IDs into a
  location table. Every node, event and diagnostic inherits it, and events
  carry the source text the semantic layer needs (maths as the author's exact
  characters, for instance). It exists from the first engine code: every
  engine path carries it.
- **Determinism.** The clock, random seed, `\jobname` and any environment a
  document can observe are caller-supplied inputs with fixed defaults
  (`SOURCE_DATE_EPOCH` convention). Same inputs, same output.
- **Fonts and images.** A missing font gets fallback metrics and a
  diagnostic. Images are read for dimensions only (PDF, PNG, JPEG, EPS
  headers); a missing image is a zero-size box and a diagnostic, as in
  LaTeX's draft mode.
- **Engineering.** One engine instance per thread, no global state; the event
  schema carries its own version, separate from the ABI's; the library is not
  named TeX (Knuth's terms).

## Two components, two oracles

TeX expansion interleaves with execution token by token (it depends on
registers set by execution, and `\catcode` assignments change how later
characters are read), so it is not a separable preprocessing pass. The split:

| Component | What it is | Oracle |
|---|---|---|
| **Engine:** tokenizing, expansion, assignments, registers, conditionals, grouping, file I/O | One evaluation engine, emitting **execution events** (internal: expansions, assignments, primitive actions) | Differential traces against reference `xetex` (`\tracingall`, register values, `.log`, `.aux`, `.toc`) over a primitive test suite and the corpus |
| **Semantic layer:** execution events → **semantic events** (the public stream) and the node table; node table → regenerated source | A static transformer running alongside the engine during the parse; it reads only what execution events carry, never TeX source directly | Round-trip typesetting identity plus the non-degeneracy gates |

### Round trip
The semantic tree is complete when regenerating source from it and
typesetting that source yields **byte-identical typeset output** to
typesetting the original. Any difference in typeset output is a hard failure;
a semantic alteration always is one. The regenerated *source* may differ
wherever the difference has no effect on the output: whitespace that typesets
the same, or equivalent constructs that normalize to one internal
representation and are regenerated in one canonical form. That is expected,
not a failure.

- The reference engine typesets both sides, using the compiler the paper
  declares in arXiv's `00README.json` (present for some papers only; to
  verify), pdflatex when it declares none; this library never typesets.
  Compare typeset output in a form free of PDF timestamps and IDs (extended
  DVI or shipped-out page boxes — the architect's choice), with
  `SOURCE_DATE_EPOCH` fixed on the reference engine too.
- **The reference installation is pinned:** the host's reference TeX
  installation and the bundled macro library report the same TeX Live year,
  and the oracle refuses to run on a mismatch.
- **The tree is lossless; consumers project it.** It keeps everything that
  affects the page; a consumer drops what it does not need.

### Non-degeneracy gates
Storing the raw token stream as one opaque node would pass every round trip
with no semantic content. So:
- Structure is nodes at the level the author wrote: sections, environments,
  theorem blocks, labels, references, citations, macro invocations — the
  **structural constructs**.
- Opaque raw-token nodes are a counted, budgeted fallback, with a budget of
  zero for structural constructs.
- The oracle comparisons are immutable gates; the mechanism
  that keeps an agent from loosening one is the architect's to design. Only
  the owner amends a gate or a recorded threshold.

## The semantic capture schema

Adopted from a literature search for the best known approach, with the tweaks
this library's use needs. Candidates include the LaTeX Project's tagged-PDF
structure tree (status to verify) and LaTeXML's document schema; the search is
not limited to them. If the gap between the best known approach and this
library's needs is too large for tweaks, the architect designs the
difference, stating what the adopted schema could not carry and why. The
schema decides the semantic event kinds and the node-table fields; the data
layout in "Interface" is fixed.

## Interface

**Contract: an api_gen definition** (`.adef.toml`). api_gen is the owner's
code generator in the `linux-kernel-driver-sandbox` repository at
`vdev/api_gen/` (`/Users/agent-user/projects/linux-kernel-driver-sandbox/vdev/api_gen/`);
its SPEC.md, "Outputs", lists what it emits today: a C header, a header-only
C++ wrapper, a LuaJIT module, a Rust binding, a Rust ABI relay and Rust and
C++ implementation stubs. **v1 bindings:** the C header, Rust, and Go (a new
emitter). Other languages (WASM, a Java Foreign Function & Memory emitter,
others) are future emitters, not v1.

**The interface is idiomatic; the data layout is one flat, C-shaped form for
every language:** fixed-size records, links by index, strings as offsets into
a text arena.

### Buffer rules
- **Calls into the library:** memory the caller passes is borrowed for the
  duration of the call. No buffer is returned; to get data out, the caller
  provides the buffer and the generated code copies into it. The largest value
  returned by value is a 16-byte struct.
- **Calls into the caller's callback:** the views the library passes are
  library-owned, read-only, and valid only for that callback.

### Two entry points
**1. Semantic event stream: a callback with granularity N**, delivered during
the parse as the semantic layer produces events.
- N = 1 delivers every event as it happens; N = k delivers batches of k; N = 0
  delivers everything in one call at the end.
- Each call receives views of the accumulated stream from the beginning, of the
  slice new since the previous call (each with its count), and of the text
  arena.
- The callback's return value says whether to continue.
- The last call delivers the remainder and is flagged final. A consumer may
  also ask to be called at given structural event kinds (a section closing,
  for example).
- Across calls, consumers keep indices, never pointers: the buffers may move
  between calls; indices into them are stable for the whole document.
- Events are semantic (for example: section opened, environment begun, label
  defined, reference made, citation made, text run, maths span). A reference
  names its label when made; its resolution arrives in a later event.

**2. Parsed result.** After the parse, a callback receives the complete,
resolved document as two views: a flat node table (records carrying, for
example, kind, parent, first-child and next-sibling indices, source span and
text-arena offsets) and the text arena. Every node is closed and every
reference resolved. No tree is exposed during the parse; a consumer wanting
one builds it from the event stream.

### Callback rules
- A mandatory `user_data` pointer.
- Called only on the caller's thread, during the call that drives the parse.
- Bindings expose lazy accessors over the views and never copy the
  accumulated views eagerly; a binding wanting native objects copies only the
  new slice each call.
- Languages whose boundary crossings are slow (LuaJIT, Python) use a large N
  or N = 0; languages with cheap callbacks (C, Rust, Zig) can use N = 1.

### api_gen additions this library needs
- A Go emitter (cgo bindings).
- Platform library naming beyond Linux `lib<name>.so`, and a static-link mode
  (Rust `staticlib`) so a Go consumer stays a single binary.
- Typed record arrays in caller-provided buffers, with the callee reporting
  how much it wrote and how much it needs.
- Variable-length text as offsets into an arena carried in the same call.
- A failure-description call (construct and source position) on the same
  mechanism.
- Callback registration with the rules above.

Every emitter renders each new construct or refuses it by name.

## Implementation language: Rust

The C ABI is the primary surface. **Targets:** macOS arm64, Linux amd64,
Windows amd64 — kbase's distribution targets. Go consumers link through cgo; building
from source needs cargo; release builds stay single-host cross-builds with zig
as the cross C toolchain (`cargo-zigbuild`, `zig cc` as Go's `CC`).

## First consumer: kbase

kbase (`/Users/agent-user/projects/kbase`) builds a navigable Markdown
knowledge base with a claim graph from a LaTeX document set. Per document it
needs:
- sections at every level, with titles, numbers and labels;
- theorem-like blocks: declared display name, number, optional title, label,
  extent; proofs, with the references in their opening text;
- labels, and every reference with its macro type, the label(s) it names and
  the text immediately before it;
- citations with their keys, and the bibliography entries the `.bbl` renders;
- display and inline maths as the author's exact source characters;
- prose with its meaning-bearing treatments (emphasis, lists, footnotes,
  quotations);
- title and abstract;
- provenance on all of the above.

This list is the acceptance test for the Go binding: for each document it
delivers everything above. The shape of the Go API is the architect's;
fitting it into kbase is kbase work under kbase's own plan.

## Done for v1

- The engine passes the differential-trace suite against reference `xetex`.
- Round-trip identity holds, with zero structural fallbacks, on a share of the
  papers the pinned reference installation typesets without error as their
  declared compiler; the architect measures that set first and sets the share
  from it.
- Secure by default: tests show shell escape refused, file access confined,
  each budget enforced, and cancellation working.
- Determinism: two runs with the same inputs give byte-identical events and
  parsed results.
- Provenance: every node and event carries an origin; regeneration at the
  author's level passes the non-degeneracy gates.
- Format dumping in place, with load time measured.
- Both entry points exercised by tests: N = 1, N = k, N = 0, early stop, the
  final-call flag; the parsed result's node table walked and checked.
- The v1 bindings (C header, Rust, Go) generated from the definition and
  tested.
- The Go binding delivers the "First consumer: kbase" list for every corpus
  paper inside the round-trip share.
- api_gen extractable: its own tests pass standalone, and it contains nothing
  specific to this library.
- **Every requirement stated in this document has a test that names it** —
  including the consumer-supplied macro library and its search order, no
  run-time network access, the stale-`.bbl` diagnostic, the bounded `.aux`
  loop and its non-convergence report, font and image fallbacks, no global
  state, the event schema's version, calls at chosen structural event kinds,
  the macro-definition chain in provenance, and licence notices in the
  bundle.

## Decided by the architect

Recorded in the project's ARCHITECTURE with the reason; no owner sign-off:
the semantic schema adoption; the format-dump design; the mechanism that keeps
oracle gates immutable; the round-trip share for v1; the typeset-output
comparison form; performance targets; the bound on `.aux` passes; events
versus passes; the bundle's contents within the licences; whether shaping is
in v1; whether api_gen's changes stay consistent with its THESIS (judged and
reported).

## Handoff

**Seat.** Dispatch with a seat that can research, write documents and
dispatch other agents — not an architect definition limited to reading
files.

**Corpus.** The 50-paper arXiv sample defined by kbase's
`prep-test-integration-arxiv` recipe; copy its ID list and the recipe into the
new repository. The sample is pandoc-clean by construction (papers pandoc
could not read were dropped), so it under-represents exactly what pandoc gets
wrong; a further freely downloadable set may be identified later. No paid
bulk access.

**api_gen.** Vendored into the project, with full control given to the
implementing agents provided it stays reusable and generic: at the end it must
be extractable as its own repository with nothing specific to this library
attached, consistent with its THESIS.md. Whether the extracted copy replaces
the one in `linux-kernel-driver-sandbox` is the owner's decision at the end.

**Process.** The owner's house process: contract documents (THESIS, SPEC,
ARCHITECTURE, CONVENTIONS) before code; an `ACTIVE_PLAN.md` the agents execute
from; prompt-engineer review of anything model-facing; immutable oracle gates.
For design decisions the architect may run the multi-agent design debate
process (the `mad-design-referee` agent and its participants, installed from
the owner's `adjagent` repository).

**Runtime and budget.** Undecided by the owner; a candidate is running the
whole effort against local inference under opencode instead of Claude Code.
Plan so that each phase can run under either, naming any step that depends on
one of them (the design-debate agents above are Claude Code definitions), and
state an estimated cost per phase for the owner's runtime and budget decision.

**Order of work.**
1. **Owner first:** name, repository and licence; host installation of the
   reference engines (`xetex`, `pdftex`, `etex`) with the full macro library,
   at the TeX Live year the bundle will carry (the current host has BasicTeX
   only).
2. **Architect, after step 1:** research — the schema search, every "to
   verify", the pdflatex measurement, the typesettable-set measurement —
   then the contract documents and the plan in the new repository: the
   architect specifies them, tech-writer writes SPEC and ARCHITECTURE,
   prompt-engineer writes CONVENTIONS. Research scripts go in scratch.
3. **Stops for owner**, with the architect handing back: the drafted
   documents and plan for agreement; the pdflatex share with its three
   options; the per-phase cost estimate for runtime and budget; anything the
   licences put back to the owner.
4. **No library code before steps 1 and 3.**
