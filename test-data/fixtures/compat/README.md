# compat — the op script

`ops.yaml` is a sequence of write-API ops in kbase's values form.
`just test-integration-opscript-results` applies it through `./bin/kbase` to a
scratch copy of the results fixture, wants every step `done` and `verify`
green, then re-issues every step and wants each `unchanged` with `kb-root/`
byte-identical.

Each step names its `op`, its `values`, and optionally a `name` by which later
steps refer to the id it mints. The script inserts every claim it depends on,
each hosted in the chosen leaf by its `document` and `locator`, so it runs on a
KB with no claims as well as on one with many.

## Placeholders

A string value that is exactly one of these is replaced when the script is
rendered against a KB; the choices are recorded beside the rendered steps.

| Placeholder | Becomes |
|---|---|
| `{leaf}` | the first leaf, in path order, whose body holds a plain-prose line of five or more words the locator search finds exactly once; where no leaf holds one, the first leaf holding such a run of plain prose within a line |
| `{locator}` | that line, or that run, of the leaf's body |
| `{minted:<name>}` | the id the step named `<name>` minted, substituted at run time from that step's output |

## Where the rendering lives

`TestStageOpScript` in `internal/write/opscript_test.go` resolves the
placeholders and writes each step as a values file; it skips unless its flags
name the script, the KB and the output directory.
