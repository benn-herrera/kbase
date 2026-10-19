# compat — the op script

`ops.yaml` is a sequence of write-API ops in kbase's values form. The
compatibility tests apply it to one KB through `./bin/kbase` and, rendered to
TOML values files, through the reference run, then compare what each side
left.

Each step names its `op`, optionally `create: true` (the op's `--create`), its
`values`, and optionally a `name` by which later steps refer to the id it
mints. The script inserts every claim it depends on, so it runs on a KB with
no claims as well as on one with many.

## Placeholders

A string value that is exactly one of these is replaced when the script is
rendered against a KB; the choices are recorded beside the rendered steps.

| Placeholder | Becomes |
|---|---|
| `{leaf}` | the first leaf, in path order, whose frontmatter carries only `kind` and either two or more `claims` or a `no-claim` — a leaf left citing two claims owes each a marker, which a single-claim leaf's first claim may lack — and whose body holds a line the locator search finds exactly once |
| `{register}` | the register of `{leaf}`'s volume, `<first path segment>/claim-quality.md`, created by the first insert where absent |
| `{leaf-claims}` | as a list item: the leaf's current `claims`, in order — `set-frontmatter` restates every attribute it does not own, and a `no-claim` is replaced by the claims the script adds |
| `{locator}` | that line of the leaf's body |
| `{minted:<name>}` | the id the step named `<name>` minted, substituted at run time on each side from that side's own output |

## Where the rendering lives

`TestStageOpScript` in `internal/write/opscript_test.go` resolves the
placeholders and writes each step as kbase YAML and as the reference's TOML
form; `TestCompareOpScript` there compares the two runs' results. Both skip
unless their flags name the script and the runs' output.
