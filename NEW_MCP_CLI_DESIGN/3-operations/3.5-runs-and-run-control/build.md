#### `build` (tool + CLI)

This builds a KB from LaTeX volume roots into `kb-root/`. A volume root is a paper's top `.tex`
file: the file `00README.json`'s toplevel entry names, else the sole file containing
`\documentclass`. Each stage boundary is a commit, label `kb-build: <stage> | <display>`, scoped
to the paths the build owns. The start boundary records `started-at`, the commit the build began
from. The *trail* is the `kb-build:` commits on the current branch. Resetting the branch to
`started-at` discards it. The KB a build writes holds every title unique within its kind ([§1.1](../../1-model/1.1-nodes.md)),
as any write does.

- **Arguments**:
  - `volume-root` (paths, repeatable; relative paths resolve against the project root);
  - `bibliography` (`.bib` paths, repeatable; default every `.bib` beside any volume root);
  - `no-inference` (bool);
  - `through` (a stage id or display name);
  - `resume` (bool);
  - `state-dir`;
  - `ask-concurrency` (integer ≥ 1).

  A fresh build takes volume roots. `resume` takes none and continues with the inputs the trail
  records.
- **Result**: `state-dir`, `roles`, `through`, `no-inference`, `resumed`, `restored`, `stages`
  (`stage`, `commit`, `dropped`, `report`), `resume`.
- **MCP result**: once the build has entered a stage, `done` with `kb-root`, `state-dir`, `pid`,
  `resume` and `wait` (the `kbase wait` command line). Where the build exits first, the build's own
  document. Where it is still starting after 5 s, `done` with `state: starting`, `state-dir`, `pid`
  and `wait`; the build keeps running, and `status` follows it.
- **Refusals**:
  - a fresh build where `kb-root/` exists in any state, a skeleton-only one and one holding an
    interrupted build's records included (`check: kb-root`; the same refusal as `init`). This is
    the one refusal of a fresh build over existing ground;
  - `resume` with nothing to resume while `kb-root/` stands: remedy, remove `kb-root/` to start
    over. kbase does not inspect half-built state.
  - `resume` where `kb-root/` does not exist, whatever the trail records (usage): remedy, a fresh
    build;
  - `resume` given volume roots or a bibliography (usage);
  - neither volume roots nor `resume` (usage);
  - dirty paths the build owns (`check: dirty-paths`);
  - outside a git worktree: remedy `git init`;
  - a run holding the operation lock;
  - a pending conversion of an existing KB;
  - a missing decision or text role without `no-inference`.
