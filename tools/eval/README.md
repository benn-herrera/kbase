# tools/eval — KB inferential-quality evaluation prompt

`extract_kb_eval_prompt.py` turns the evaluation instrument
`test_data/fixtures/eval/kb-quality-eval-brief.md` into the prompt a session dispatches. Python
3.11+, stdlib only.

## What the instrument is for

It grades a built KB's judgment calls, not its mechanical consistency, which `kbase verify` covers.
The judgment calls are its taxonomy, leaf granularity, index summaries and claim graph. A fresh
evaluator agent reads the prompt body, walks the KB and writes one report. Every evaluation uses the
same body, so reports compare row for row across runs.

## Dispatching an evaluation

The brief's header is its dispatch contract: a fresh `general-purpose` agent, cold context, in the
background, at the model the header names. The session runs both modes:

```
model="$(python3 tools/eval/extract_kb_eval_prompt.py --field model <brief>)"
prompt="$(python3 tools/eval/extract_kb_eval_prompt.py --walk-set <walk-set file> \
    <brief> <kb-root-path> <output-report-path> <corpus-note>)"
```

It dispatches `$prompt` at `$model` and reads the report at `<output-report-path>` when the agent
finishes.

- **Field mode** prints the backtick-quoted `` `model: …` `` token from the header's `Dispatch
  contract:` sentence, newline-terminated.
- **Body mode** prints the text between the `---- PROMPT BODY BELOW` line and the `---- PROMPT BODY
  ABOVE` line (or the end of the file), with the four parameters substituted.

## The four parameters

The brief's header defines them:

- `{kb-root-path}`: the KB under evaluation;
- `{output-report-path}`: where the report goes, by convention
  `.claude-temp/eval/<run-tag>/report.md`;
- `{corpus-note}`: one phrase of corpus and build context;
- `{walk-set}`: the contents of the `--walk-set` file, substituted verbatim. It is the numbered list
  of fixed questions the evaluator walks first.

**The walk set lives outside the repository**, in the caller's `.claude-temp/`, because its
questions name the corpus's content. Reports compare only between runs given the same walk-set file,
byte for byte.

## Refusals

Each refusal writes `error: …` to stderr, prints nothing on stdout and exits 1:

- the brief has no `---- PROMPT BODY BELOW` line;
- field mode: a field other than `model`; no `Dispatch contract:` line above the body; or no ``
  `model: …` `` token within the two lines after that one;
- body mode: arguments other than `--walk-set <file>` followed by the four values; a walk-set file
  that does not exist; a `{` or `}` in the kb-root path, report path or corpus note; or a body
  missing any of the four placeholders, which the message names.

The walk-set contents are exempt from the brace refusal because questions may carry LaTeX. They are
substituted last, so a placeholder-shaped string inside them is never substituted.
