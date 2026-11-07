### Dependency rules

**Pattern:** Architecture fitness functions.

`cmd/importpolicy_test.go` checks the rules below over the import graph of the module's production
files, with `cmd/readpolicy_test.go`, `cmd/revealpolicy_test.go` and the exec policy beside it. A
rule marked `direct` in the test checks each package's own imports only; any other rule checks the
transitive graph, so a package outside its list that reaches the named package through any chain
fails the test. A rule that names a `cmd` file is checked at file level (`TestCmdFileImporters`).

- `docgraph` knows nothing of claims. The record schema it writes is the one `claimgraph` reads,
  defined once in `internal/records`, which imports nothing of kbase but `atomicfile`, to write its
  files. `docgraph` reaches no `index`: it writes the tree through `kbwrite`.
- `kbwrite` imports no kbase package but `atomicfile`, `kb` and `result`, so every writer under
  `kb-root/` imports it, below the packages that write there.
- `claimgraph` never imports `latex/*`, and reaches neither `model` nor `asks/call`, transitively.
- `buildrecords` imports no kbase package but `kb`; `claimgraph` reads the build records through it.
  `sheet` reads no build record and imports no kbase package but `kb`.
- `argmark` and `secret` import no kbase package. `argmark` is therefore reached by every producer of
  text an MCP reader can receive.
- `log` imports `argmark` and nothing else of kbase: every line it writes formats a marked argument
  as the command line takes it, so no log line contains a marker. A package that imports only `log`
  reaches `argmark` through it.
- `kb` imports no other kbase package but `log`, `formats/format40` (the live format it re-exports)
  and `argmark`. `docgraph`, `index`, `write`, `query`, `sheet`, `kbdocs` and `claimgraph` read the
  kb-root vocabulary, the link relation (in its two readings, one resolver), the label-name grammar
  and id minting from it. `export` reads the link relation a third way, `kb.ChildList`: an index
  page's child-list lines only, so a body link is never a navigation-tree edge.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph` or `build`.
- The `formats` tree imports nothing of kbase outside itself (I8), so maintenance of a package
  outside it can never change what an old KB converts to. It takes and returns files as bytes, and
  owns the parsers of every superseded format and the live format's definition. Outside the tree,
  only `kbload` imports the root and only `kb` imports `format40`; `kbload` reaches `format40`
  only through `kb` and the root. The write side drains the conversion's delete set; the tree
  touches no file. Within the tree imports run one way: `common` imports nothing of the tree; a
  superseded format package imports `common` and earlier format packages; a converter imports
  `common` and format packages up to its target; the root imports the converters, `common` and the
  format packages. `format40` imports no kbase package, `common` included, and restates any
  declaration it shares with an earlier format. No package but the root imports a converter, and none
  imports a later format (`formatsTreeViolations`).
- A file is opened by path only in the files `fileOpenOwners` in `cmd/readpolicy_test.go` lists, each
  for the purpose it names. A KB's files (the documents, the index and the build records beside
  them) are read through `kb.Source` alone, which `kb/source.go` implements and `kbload` constructs.
  Only `kbload`, `docgraph/validate.go` and `claimgraph/tree.go` construct a `Source` with
  `kb.OnDisk`, and only `write/store.go` and `index/refresh.go` call `Source.DiskFile`.
- `kbload` is imported by `cmd`, `build` and `write` only; `index` does not import it, since a
  refusal is a `result.Refusal`, one shape. `kbload` imports `buildrecords`, `kb`, `result` and the
  `formats` root; `buildrecords` stays free of `kbload`.
- `mcp` is imported by `cmd` alone and reaches no `model`, no `asks/call` and no `build` (I9);
  `mcp/protocol` imports `log` only (and `argmark` through it).
- `internal/model` imports no kbase package but `secret`, the key's type, and no `os`: it touches no
  file and knows no role or consumer (`TestModelImportsNoOS`).
- Only `asks/call`, `build`, `modelclient` and three `cmd` files import `model`: `cmd/build.go`
  (the build's role closure), `cmd/score.go` (the score verb's model call) and `cmd/probe.go` (the probe's
  one call). `internal/model/modeltest` imports it as test support, which nothing shipped imports.
  `modelclient` is the only constructor of a `model.Client`, and a verb opens one only where it is
  about to make a model call. `modelclient` imports no kbase package but `model`, `config`, `result`
  and `log`, and `cmd` alone imports it. Only `build` imports `asks/call`.
- `internal/config` imports no kbase package but `atomicfile`, `result`, `argmark` and `secret`. It
  holds the grammar, the three roles, `Resolve`, `Select`, the loaded files and the templates, and
  refuses in `result.Refusal`. It holds the configure commands' rules too
  (SPEC §9 Provider commands): `ConfigureModels` (the references given, the match of each provider
  part against the pool of the written layer's stack, each reference against its provider's
  catalogue, and the targeted write); `ProviderEntryRefusals` (an entry's arguments); and the
  targeted editors in `write.go` (`UpdateConfig`, `SetProvider`, `RemoveProvider`), with one line
  editor and one decode-compare check for both files
  (SPEC §9.5 The configure commands' targeted update (byte-preservation contract)). It reads a
  catalogue through a function value its caller supplies (`cmd/configure-models.go` binds it over
  the registry's `Catalogue`, under the `--timeout` deadline), and returns the catalogue misses and
  what the run wrote as data, from which the verb writes its stderr lines. It never imports `model`
  or `log`, opens no client and writes no output, and returns its faults as data, so each caller
  decides whether one is a warning or a result field. A refusal's remedy may name the command that
  fixes it (`kbase configure-providers`, `kbase configure-models`).
- `internal/score` imports `model` nowhere: the endpoint is below the verb's model-call interface,
  so the runner, the input guard, the parse and the ledger learn nothing of what answered. It imports
  `write` for a writing-the-results batch's ops and its lock, and never `build`, `kbload`,
  `statedir` or `kbdocs`: the verb takes the run's locks, opens the KB, resolves the state directory
  and runs the start rebuild of the derived views. A batch reopens the KB, to compare each value's
  judged hash with its item's text as the text is now, only through the function the verb supplies
  (`score.Invocation.Open`, `kbload.Open` in `cmd/score.go`). Only `cmd` and `internal/mcp` import
  `score`.
- The judged hash is defined in `internal/asks`, which owns the templates its hash covers: `score`
  imports `write`, so the function cannot be defined in `score`, and `write` imports `asks` for it
  alone. Neither reaches `model` through it.
- Only `build`, `buildstore` and, within `cmd`, `cmd/update-format.go` (its clean check, and the
  `git mv` of each root build record into `kb-root/.build/`, `Repo.Move`) and `cmd/write_op.go` (a
  committing batch's commits, made where every write interface runs its batch, and
  `update-format`'s, through the same `kbCommits`) import `gitledger`; `gitledger` imports `log` and
  nothing else of kbase. Every opener of a KB's repository goes through `buildstore.OpenRepo`, which
  refuses `worktree` where git's worktree root is not the directory `kb.RepositoryRoot` finds;
  `cmd/init.go`'s create commit opens through it and imports no `gitledger`. `write` runs no git: a
  committing batch's snapshot commits are the caller's `write.Options.Preflight`.
- `buildstore` reaches no `model` and no `asks/call`: the build's state, its stage vocabulary and
  its checkpoint commits are read without the runner, and `build` imports them.
- Only `build`, `cmd` and `mcp` import `kbdocs`: `build` and `cmd` stamp readiness documents (within
  `cmd`, file-level, `init` at a KB's creation and `update-format` rewriting the maintained blocks
  with `kbdocs.Maintain`). `mcp` imports it for the `.index/` rule and the session rule alone
  (`kbdocs.IndexRule`, `kbdocs.SessionRule`), which the instructions carry; it stamps nothing.
- Only `build`, `buildstore` and `cmd` import `statedir`.
- `build` imports `write` (the write-lock wait).
- Only `internal/latex/pandoc` execs pandoc; only `internal/gitledger` execs git; only
  `internal/sheet` execs `dot`; and only `internal/mcp/launch.go` (file-level) execs kbase itself.
  Those four are the exec policy's whole list (`cmd/execpolicy_test.go`): a process started anywhere
  else in non-test code, or a listed owner starting a binary other than its own, fails the test;
  test files are out of its scope.
- `internal/export` imports `kb` and `atomicfile`, and nothing else of kbase; the verb opens the KB
  and supplies every pandoc call (the preflight and the one `pandoc.RunDefaults` call) as a function
  value `export.Run` calls.
- `internal/auth` imports nothing of kbase but `atomicfile`, to write its files at their mode, and
  `secret`; only `cmd` imports it, and `config`'s own rule keeps it from the store.
- A key's text leaves a `secret.Key` through `Reveal` in four files alone:
  `internal/model/openai_client.go` and `anthropic_client.go` (setting the auth header and scrubbing
  an echoed key), `internal/model/transport.go` (cutting a status error's body around it) and
  `internal/auth/auth.go` (sealing it). `cmd/revealpolicy_test.go`, on the module walk, fails a
  production file outside them that names `Reveal`.

---

