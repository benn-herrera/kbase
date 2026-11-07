# compat — the op script

`ops.yaml` is a sequence of write batches in kbase's values form.
`just test-integration-opscript-results` applies it through `./bin/kbase write`
to a scratch copy of the results fixture, wants every batch `done` and `verify`
green, then re-issues every batch and wants each to report the outcome its step
states under `reissue`, with `kb-root/` byte-identical: `unchanged` for a
batch whose entries all stand as it states them — an add finding its node or
edge standing with the same values, an edit whose values already stand — and
`refused` for a batch one of whose entries is refused, such as an add whose
node or edge a later step edited, so that it stands with other values. A
`refused` step also states under
`reissue-detail` the prefix its first refusal's `detail` starts with — such as
`"node exists: "` or `"edge exists: "` — and an `unchanged` step states none;
the staging refuses a script that breaks either.

Each step is one batch: its `values` list the batch's entries, each naming its
op under `operation`, and a step may carry a `name` by which later steps refer
to the id its one entry mints. No entry refers to an id another entry of its
own batch mints, so the script adds its nodes first, one batch each, then their
edges, then its edits. It adds every claim it depends on, each hosted in the
chosen leaf at the chosen line, so it runs on a KB with no claims as well as on
one with many.

## Placeholders

A string value that is exactly one of these is replaced when the script is
rendered against a KB; the choices are recorded beside the rendered steps.

| Placeholder | Becomes |
|---|---|
| `{leaf}` | the first leaf, in path order, whose body holds a plain-prose line of five or more words the excerpt search finds exactly once; where no leaf holds one, the first leaf holding such a run of plain prose within a line |
| `{line}` | that line's 1-based number, an integer |
| `{snippet}` | that line, or that run, of the leaf's body |
| `{minted:<name>}` | the id, of any kind, the step named `<name>` minted, substituted at run time from that step's output |

## Where the rendering lives

`TestStageOpScript` in `internal/write/opscript_test.go` resolves the
placeholders and writes each step as a values file; it skips unless its flags
name the script, the KB and the output directory.
