# Incoming: the kb_tools pipeline transplant

**Status:** context capture from working discussion 2026-09-12. Nothing here
is scheduled and nothing here overrides SPEC.md or ARCHITECTURE.md yet. The
upstream it describes (`../adjagent/kb_tools`) is still in active
development; exact specs are imported when the owner says so, not before.
When that import happens, this document disperses into ARCHITECTURE.md /
SPEC.md per the house doc taxonomy and is deleted.

---

## Bottom line

**kbase's tree-design approach is expected to invert.** The pipeline blueprint
being developed in `../adjagent/kb_tools` derives the document skeleton
mechanically and asks inference only to *refine an existing structure*. kbase
stage 3 does the opposite — the heavy tier designs the tree from the survey, at
a declared no-fallback seam.

**Three consequences for work done before the import:**

1. **Do not build further repair machinery around stage 3.** The accumulated
   rulings there — content floor, merge-direction preference, cross-parent
   merge accounting, index pruning on absorption, candidate cap, mechanical
   pre-batching, absorbed-container call skipping, interposition — are patches
   around *the model proposed a tree*. More of them is more to unwind.
2. **Do not tune prompt definitions in this repo.** They are developed and
   proven upstream (see "Where the parts come from"). Tuning them here means a
   bad run has two candidate causes with nothing to separate them.
3. **The harness is the part that stays.** Orchestration, resume, delivery and
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

## Scope: LaTeX first, and why the old breadth was part of the failure

**One of the problems kbase was solving at once was too many input sources**
(owner, 2026-09-12). Predicting the common overlap of different document types
for a problem space not yet explored is unwise, and this project did not escape
that unscathed. **The focus is LaTeX.** Software is malleable: if another input
type is added later, the generalization gets found then, from two real cases
rather than from one and a guess.

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

The a-fortiori argument for kbase: if the *claim graph* drafts mechanically, a
tech-doc tree certainly does.

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

- **Claim graph.** kb_tools' second half is the `clm`/`exp`/`sup` metadata
  spine. ARCHITECTURE.md §1 rules it out here by name as a non-goal. The
  blueprint's *shape* transfers; the claim-graph payload does not.
- **LaTeX.** kb_tools deleted its vendored reader and now shells out to
  **pandoc**, one parse per volume root, cutting documents from a single
  whole-volume rendering so theorem numbering and citations resolve once. It
  takes that as an enumerated exception to stdlib-only, with a named
  `PandocMissingError`. kbase does not take that exception — see the standing
  constraint below.

### Standing constraint: pandoc is a last resort, and the two-step is what to collapse

**The pandoc shortcut is not taken in kbase other than as an utter last
resort** (owner, 2026-09-12). ARCHITECTURE.md §2 is the first reason — a
shelled system binary is not a self-contained appliance — but it is not the
only one.

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

**The skeleton derivation is the likeliest place filing turns into recasting.**
kb_tools derives its skeleton from LaTeX's declared, closed sectioning
vocabulary, where a `\section` is unambiguous and authored to a convention.
Here it must come from ATX headings written to no standard across hundreds of
files, with a folder hierarchy carrying real structure beside them.

The analogous risk is already named upstream as `kb_tools/ROADMAP.md` item 5
(*the badly structured paper — a finer boundary set, not a different
mechanism*) and item 4 (*generality corpus*: every design decision was taken
against a single document, so a clean run over it measures that document, not
the process). kbase carries the same exposure — its design was taken against
Rojo and `creator-docs`.

Stopping rule: if filing starts turning into recasting, the difference is
corpus-class rather than tolerance, and that is a design question to raise
rather than an implementation to push through.

Worth knowing which direction ideas flow on one point: item 5 proposes
widening the enumerated boundary set to paragraph breaks, and mechanical
`(forced-split I/N)` titling for oversize sections. **kbase already ships
both** — `heading | fence | paragraph` cut candidates, and `<slug>-<k>.md`
titled `(k/n)` — and already realizes item 5's closing hope that inference
becomes the exception path rather than the stage: a corpus whose groups all fit
one page makes zero light-tier calls.

---

## Until the import

- Leave stage 3 alone.
- Leave the prompt definitions as the marked stubs SPEC.md §6 declares them.
- Harness work, delivery/resume work and gate work are unaffected and
  proceed normally.
- Any LaTeX work honours the standing constraint above: no pandoc, and the
  goal is one processor rather than a converter feeding a Markdown front door.
- Two stale spots in ARCHITECTURE.md, both worth correcting whenever those
  sections are next touched, import or no import:
  - **§11** describes kb_tools as a "stdlib-only Python deterministic spine —
    refresh, verify, link checking." It is a complete build pipeline now.
  - **§1** states the project's first target use and both validation targets
    as Markdown tech-doc sets (Rojo, Roblox `creator-docs`). See "Scope:
    LaTeX first," above.
