#### `status` (tool + CLI)

This reports the current or last run: build or score, whichever is newest.

- **Arguments**: `state-dir`.
- **Build result**:
  - `state-dir`;
  - `state` (`none`, `running`, `cancelled`, `interrupted`, `failed`, `bounded`, `finished`);
    `none` where no build stands: `kb-root/` is absent, or was removed with no build since;
  - `pid`, `started`, `updated`, `ended`, `no-inference`;
  - `stages`, each stage done or not done, without commits, and one line saying that each completed
    stage's commit, with the command to rewind to it, is in `kbase build-trail`;
  - `current` (`stage`, `units-done`, `units-total`);
  - `recent-refusals`, `recent-fallbacks`;
  - `cache-entries`, `cache-bytes`;
  - `resume`.
- **Score result**:
  - `state-dir`;
  - `state` (`running`, `interrupted`, or the run's outcome: `done`, `cancelled`, `failed`,
    `refused`, `retry`);
  - `pid`, `started`, `run`, `pass`, `counts`, `scored`, `failures`, `resume`.
- **Refusals**: outside a git worktree; a `state-dir` inside `kb-root/`; a state store newer than
  kbase.
