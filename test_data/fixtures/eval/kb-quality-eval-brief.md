# Fixture: KB inferential-quality evaluation brief (instrument v5)

The standing instrument for grading the inferential quality of a built KB.
Every evaluation uses the prompt body below unchanged, so reports compare row
for row across runs. The body is never edited per run; an edit is a new
instrument version, entered in the changelog below with owner sign-off.

Dispatch contract: fresh `general-purpose` agent, `model: sonnet`, cold
context, background. Substitute exactly four parameters, nothing else:

- `{kb-root-path}` — absolute path to the built KB's `kb-root/`
- `{output-report-path}` — absolute path for the report (convention:
  `<project root>/.claude-temp/eval/<run-tag>/report.md`, one per run)
- `{corpus-note}` — one phrase describing corpus and build context, read
  after a comma in the body's description of the KB (e.g. "built by kbase
  from three LaTeX volumes: a paper, its appendix volume and a short note")
- `{walk-set}` — the walk-set file's contents, verbatim: a numbered list,
  one question per item, each item `[category] "question"`, category one of
  `locate`, `synthesize`, `trace-support-chain`

The walk-set file stays outside version control, in the caller's scratch
under `.claude-temp/`: its questions name the corpus's content. Reports
compare only between runs given the same walk-set file, so hold one file
fixed, byte for byte, across every run a comparison spans.

---- PROMPT BODY BELOW — SUBSTITUTE PARAMETERS, CHANGE NOTHING ELSE ----

You are an independent evaluator examining a freshly built knowledge base for INFERENTIAL quality — the quality of the judgment calls that shaped it. You had no part in building it; bring fresh eyes and no loyalty to its choices.

The KB: {kb-root-path} — a hierarchical Markdown knowledge base with a claim-dependency graph, {corpus-note}. Mechanically verified green, including a citation-integrity validator.

READ-ONLY throughout. Do NOT run `git show`, inspect git tags, or consult any prior/reference KB or earlier evaluation reports — evaluate only what exists in kb-root/ now, independently. Do not modify anything anywhere except writing your one output file.

**Out of scope**: mechanical consistency (link integrity, frontmatter validity, index freshness, citation-channel conformance) — structurally guaranteed and separately verified. Do not spend attention re-checking it.

**In scope — the judgment calls**: taxonomy shape (domain decomposition, hierarchy depth, what became a domain vs a subtopic), leaf granularity (chunking of content into files), entry-point design, intermediate index summaries (do they accurately and discriminatively represent their subtrees?), claim-vs-support assignments and dependency-edge judgment in the claim graph, invariant placement (what went into kb-root/invariants.md), naming choices.

**The test — calibrate carefully**: NOT "would you have done it differently" (you always could; that is noise). The question is whether any call could have been made differently in a way that MATERIALLY improves the artifact, where quality means: clear discovery paths; accurate summaries at intermediate nodes; high likelihood that even a low-parameter model can find and synthesize answers; in one phrase — low drag in the substrate, leaving maximum model attention for the real task. Every wrong turn a summary invites, every non-discriminating index, every mis-chunked leaf is attention taxed away from the reader's actual problem.

**Method**:
1. Task-driven walks: run every question in the fixed walk set below, each exactly as written; log and report those first, in listed order. Then keep walking — add your own questions, and press variant hunts off any walk that resolved cleanly (re-ask the same target one step narrower; ask what the KB itself says about the thing you just found). The fixed set makes runs comparable; the walks you invent are where a substrate's edges usually show, so do not stop at the fixed set. Walk each from entry-point.md down as a small model would — following summaries, not your own shortcuts. Log every point where the substrate fights back: a summary that misleads, an index that doesn't discriminate between its children, nesting deeper than the content warrants, a leaf that chunks two ideas or splits one.
2. Summary-faithfulness sampling: pick ~8 intermediate index nodes; compare each summary against its actual subtree for accuracy and discrimination.
3. Claim-graph judgment sweep: sample claims and supports; assess whether clm/sup assignments and depends-on edges reflect sound judgment about what is load-bearing.

**Fixed walk set** (step 1 — every one, verbatim, in this order; the bracketed tag is the walk's category):

{walk-set}

A question whose subject this build does not contain is itself a result: report it as a walk that found nothing, with what you looked at, rather than substituting a nearby question.

**Output**: write your report to {output-report-path}. Structure: verdict summary first (the overall shape: headroom-above-a-floor vs material-quality-loss, with the count of material findings); then material findings each with severity, the specific alternative call that would have been better, and the quality dimension impacted; then a section of immaterial observations (calls that could differ without material impact — these are evidence of headroom, list them briefly); then your walk logs as an appendix (the raw evidence). Be the evaluator who would rather report three real findings than thirty defensible ones.

---- PROMPT BODY ABOVE — MAINTAINER NOTES BELOW, NEVER EXTRACTED ----

## Changelog

- **v5 — 2026-10-04.** The fixed walk set leaves the body: it is supplied
  per comparison as the `{walk-set}` parameter. Walk-set externalization
  only — no other pending revision to this instrument is folded in.
- **v4 — 2026-09-18.** Dispatch model changes from `fable` to `sonnet`. The
  instrument's quality criterion is "high likelihood that even a
  low-parameter model can find and synthesize answers," and its method
  directs the evaluator to walk "as a small model would — following
  summaries, not your own shortcuts"; a frontier model simulating a weak one
  measures the simulation, not the substrate, so sonnet is the ceiling this
  instrument judges from. The judgment columns (findings, severity, verdict)
  compare only between reports from the same evaluator model. Model change
  only.
- **v3 — 2026-09-14.** The per-finding attribution step retires. It
  classified each finding against the agent definition responsible for that
  part of the build, and the build has no such agents: it is
  driver-controlled, with inference confined to individual asks, so the
  instrument has no ground for a classification of cause. Reports from v3 on
  carry no attribution column. Attribution removal only.
- **v2 — 2026-09-01.** A fixed walk set replaces per-run invented questions,
  so runs compare row for row; the free walks after it stay. Walk-set change
  only.
- **v1 — 2026-08-31.** Original; each run invented its own walks.

## Building a walk set

Harvest the questions from the walk appendices of prior runs' free walks,
de-duplicated on target rather than wording, each survivor keeping its
source walk's phrasing verbatim. Every question is corpus-facing: it names
source content — a result, a symbol, a labelled theorem, a volume's subject
— never one build's directory or index shape, so the set transfers across
builds of the same corpus. Never promote a variant hunt into the set: the
findings free hunting produces come from hunts a fixed list would not have
contained, and promoting one turns the evidence that free hunting pays into
a checklist item — the free walks in step 1 exist to keep regenerating that
behavior. The walk-set file carries the questions and nothing else; which
question exposed which finding stays out of it, because an evaluator who
knows which probes drew findings follows the prior run's path and reports a
re-check as an independent walk.
