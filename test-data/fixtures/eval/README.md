# eval — KB inferential-quality evaluation

`kb-quality-eval-brief.md` is the standing instrument for grading a built KB's
judgment calls — taxonomy, leaf granularity, index summaries, the claim graph —
as distinct from its mechanical consistency, which `kbase verify` covers. A
fresh evaluator agent receives its prompt body and writes one report. The
brief's header is the dispatch contract; its maintainer notes record how the
instrument has changed and how a walk set is built. The body is never edited
per run.

## Parameters

Defined in the brief's header:

- `{kb-root-path}` — the KB under evaluation
- `{output-report-path}` — `.claude-temp/eval/<run-tag>/report.md`
- `{corpus-note}` — one phrase of corpus and build context
- `{walk-set}` — the fixed questions the evaluator walks first

## The walk set

The walk-set file names the corpus's content, so it is never version-controlled:
it lives in the caller's `.claude-temp/`. Reports compare only between runs given
the same walk-set file.

## Dispatching an evaluation

`tools/eval/extract_kb_eval_prompt.py` reads the brief, substitutes the four
parameters (the walk set from the file named by `--walk-set`) and prints the
prompt body; its field mode prints the dispatch model. The session passes the
printed body as the prompt of a background `general-purpose` agent at that
model, then reads the report at `{output-report-path}`.
