### Dependency rules

- `docgraph` knows nothing of claims; the record schema it writes is the one `claimgraph` reads,
  defined once in `internal/records`, which imports nothing of kbase but `atomicfile`, to write its
  files.
- `claimgraph` never imports `latex/*`.
- `buildrecords` imports no kbase package but `kb`; `claimgraph` reads the build records through it.
  `sheet` reads no build record and imports no kbase package but `kb`.
- `argmark` imports no kbase package, so every producer of text an MCP reader can receive reaches
  it.
- `log` imports `argmark` and nothing else of kbase: every line it writes spells a marked argument
  as the command line takes it, so no log line carries a marker. A package that imports only `log`
  reaches `argmark` through it.
- `kb` imports no other kbase package but `log`, `formats/format400`, the live format it
  re-exports, and `argmark`; `docgraph`, `index`, `write`, `query`, `sheet`,
  `kbdocs` and `claimgraph` read the kb-root vocabulary, the link relation (in its two readings,
  one resolver), the label-name grammar and the id minting from it. `export` reads the
  link relation a third way, `kb.ChildList`: an index's child-list lines only, so a body link is
  never a spine edge.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph` or `build`.
- The `formats` tree imports nothing of kbase outside itself (I8), so maintenance of a package
  outside it can never change what an old KB converts to: it takes and returns files as bytes, and
  owns the parsers of every superseded form and the live format's definition. Outside the tree,
  `kbload` imports the root and `kb` imports `format400`; no other package imports into it, and
  `kbload` reaches `format400` only through `kb` and the root. The write side drains the
  conversion's delete set — the tree touches no file. Within the tree imports run one way:
  `common` imports nothing of the tree; a superseded format package imports `common` and earlier
  format packages; a converter imports `common` and format packages; the root imports the
  converters, `common` and the format packages. `format400` imports no kbase package, `common`
  included, and restates any declaration it shares with an earlier format. No package but the root
  imports a converter, and none imports a later format.
- No package but `kbload`, `atomicfile` and the build's stage machinery opens a KB file; every other
  read goes through the `kb.Source` from `kbload.Open`. Enforced by `cmd/readpolicy_test.go`, which
  fails on any direct KB file read outside an allowlist.
- `kbload` is imported by `cmd`, `build` and `write` only; `index` does not import it (a refusal is
  a `result.Refusal`, one shape). `kbload` imports `buildrecords` and the `formats` root; `buildrecords`
  stays free of `kbload`.
- `mcp` is imported by `cmd` alone and reaches no `model` or `asks/call` (I9); `mcp/protocol`
  imports `log` only.
- `internal/model` imports no kbase package but `secret`, the key's type, and no `os`: it touches
  no file and knows no role or consumer (`cmd/importpolicy_test.go`, `TestModelImportsNoOS` for
  `os`).
- Only `asks/call`, `build`, `modelclient` and three `cmd` files import `model`: within `cmd`,
  file-level, `cmd/build.go`, the build's role closure, `cmd/score.go`, the score verb's ask, and
  `cmd/probe.go`, the probe's one call. `internal/model/modeltest` imports it as test support, which
  nothing shipped imports. `claimgraph` reaches neither `model` nor `asks/call`, transitively.
  `modelclient` is the only constructor of a `model.Client`, and a verb opens one only where it is
  about to ask something. `modelclient` imports `model`, `config`, `result` and `log` and nothing
  else of kbase, and `cmd` alone imports it.
- `internal/config` imports no kbase package but `atomicfile`, `result`, `argmark` and `secret`: it holds the
  grammar, the three roles, `Resolve`, `Select`, the loaded files and the templates, and refuses in `result.Refusal`. It holds the configure commands' rules too (SPEC §9, "Provider
  commands"): `ConfigureModels` — the references given, the match of each provider part against
  the pool of the written layer's stack, each reference against its provider's catalogue, and the targeted write;
  `ProviderEntryRefusals`, an entry's arguments; and the targeted editors in `write.go`
  (`UpdateConfig`, `SetProvider`, `RemoveProvider`), one line editor and one decode-compare guard
  for both files (SPEC §9.5). It reads a catalogue through a function value its caller supplies
  (`cmd/configure-models.go` binds it over the registry's `Catalogue`, under the `--timeout`
  deadline), and returns the catalogue misses and what the run wrote as data, from which the verb
  writes its stderr lines. It never imports `model` or `log`, opens no client and writes no output,
  and returns its faults as data, so each caller decides whether one is a warning or a result
  field. A refusal's remedy may name the command that fixes it (`kbase configure-providers`,
  `kbase configure-models`).
- `internal/score` imports `model` nowhere: the endpoint is below its `Launch` seam, so the runner, the
  guard, the parse and the ledger learn nothing of what answered. It imports `write` for a
  landing's ops and its lock, and never `build`, `kbload`, `statedir` or `kbdocs`: the verb takes
  the run's locks, opens the KB, resolves the state directory and runs the start refresh. A
  landing reopens the KB, to compare each value's judged hash with its item's text as it stands,
  only through the seam the verb supplies (`score.Invocation.Open`, `kbload.Open` in
  `cmd/score.go`).
- The judged hash lives in `internal/asks`, which owns the templates its digest is of: `score`
  imports `write`, so the function cannot live in `score`, and `write` imports `asks` for it
  alone. Neither reaches `model` through it.
- Only `build`, `buildstore` and, within `cmd`, file-level, `cmd/update-format.go` (its clean
  check, and the `git mv` of each root build record into `kb-root/.build/`, `Repo.Move`) and
  `cmd/write_op.go` (a committing batch's commits, made where every write surface runs its batch,
  and `update-format`'s, through the same `kbCommits`) import `gitledger`; `gitledger` imports
  `log` and nothing else of kbase. Every opener of a KB's repository goes through
  `buildstore.OpenRepo`, which refuses `worktree` where git's worktree root is not the directory
  `kb.RepositoryRoot` finds; `cmd/init.go`'s birth commit opens through it and imports no
  `gitledger`. `write` runs no git: a committing batch's pre-flight commits are the caller's
  `write.Options.Preflight`.
- `buildstore` reaches no `model` and no `asks/call`: the build's state, its stage vocabulary and
  its trail are read without the runner, and `build` imports them.
- Only `build` and `cmd` stamp readiness documents — `init` at a KB's birth, and `update-format`
  rewriting the maintained blocks (`kbdocs.Maintain`): within `cmd`, file-level, `cmd/init.go` and
  `cmd/update-format.go` alone import `kbdocs`. `mcp` imports `kbdocs` for the `.index/` rule and the
  session rule alone (`kbdocs.IndexRule`, `kbdocs.SessionRule`), which the instructions carry; it
  stamps nothing.
- `build` imports `write` (the write-lock wait); `cmd` imports `build` and `filelock` (the write
  lock's contention and the holder lock's release; the operation lock's refusal is `build`'s, and the
  held-around-the-run write lock is `write.LockKB`).
- Only `internal/latex/pandoc` execs pandoc; only `internal/gitledger` execs git; only `internal/sheet`
  execs `dot`; and only `internal/mcp/launch.go` (file-level) execs kbase itself. Those four are the
  exec policy's whole list (`cmd/execpolicy_test.go`): a process started anywhere else in non-test
  code, or a listed owner starting a binary other than its own, fails the test; test files are out
  of its scope.
- `internal/export` imports `kb` and `atomicfile`, and nothing else of kbase; the verb opens the
  KB and supplies every pandoc call — the preflight and the one `pandoc.RunDefaults` call — as a
  function value `export.Run` calls.
- `internal/auth` imports nothing of kbase but `atomicfile`, to write its files at their mode, and
  `secret`, and only `cmd` imports it; `config`'s own rule keeps it from the store.
- `internal/secret` imports no kbase package. A key's text leaves a `secret.Key` through `Reveal`
  in four files alone — `internal/model/openai_client.go` and `anthropic_client.go`, setting the
  auth header and scrubbing an echoed key, `internal/model/transport.go`, cutting a status error's
  body around it, and `internal/auth/auth.go`, sealing it — and `cmd/revealpolicy_test.go`, on
  the module walk, fails a production file outside them that names `Reveal`.

The dependency rules are enforced by an import-policy test over the transitive import graph
(`cmd/importpolicy_test.go`), beside the I1 exec-policy test (`cmd/execpolicy_test.go`).

---

