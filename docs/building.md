# Building a KB

How to build a KB from LaTeX with `kbase build`, resume or cancel a build, and create an empty KB. Back to the [README](../README.md).

```
kbase build <volume-root>... [--bibliography FILE]... [--no-inference]
            [--through <stage>] [--state-dir DIR] [--ask-concurrency N]
kbase build --resume
```

- **Bibliographies.** By default every `.bib` beside the volume root is used, in sorted order.
  `--bibliography` replaces that set. An unreadable bibliography is reported and the build continues
  without it.
- **`--through <stage>`** stops after the named stage, by id or by display name; the ids are listed below. A bounded run exits 0
  with outcome `bounded`.
- **`--ask-concurrency N`** sets how many model calls are in flight at once, across every role
  (default: `[asks] concurrency` in `config.toml`, else 4).
- **Refusals.** The build refuses outside a git worktree (run `git init`), a fresh build wherever
  `kb-root/` exists, in any state (`check: kb-root`), over dirty paths it owns, on unparseable
  source, on an include it cannot load, when a stage that calls a model has no role configured, and
  while another build runs on the repository (`check: lock`). Each refusal names what it found.
- **Determinism.** The same inputs and pandoc version give a byte-identical document tree; the claim
  graph is identical except for the node ids, which are minted fresh on every build and are unique
  within the KB.
- **No judgement.** Every scoring edge a build writes carries `*pending*` as its bearing, and no
  stage writes a `rigor` or any other number into a score. A claim a build adds an edge to reads
  unchanged until that edge is judged, by `kbase score` or by `edit-edge` with a `bearing`.

Stages, in order: `start`, `document-graph`, `spine-seed`, `claims-declared`, `claims-discovered`,
`equations-minted`, `references-found`, `claims-culled`, `depends-attributed`, `analyses-split`,
`phase-3a`, `overview-drafted`. What each does is in [ARCHITECTURE §4](../PROJECT_DOCS/ARCHITECTURE/4-stage-table/index.md).

`claims-culled` asks, for each pair of linked claims with a prose end, whether one adds nothing to
the other; a prose claim that adds nothing is removed, and its links pass to the claim it was culled
into. It adds no edge: `depends-attributed` asks every surviving link its direction. Under
`--no-inference` it is walked without asks. It records its verdicts and the surviving links in
`kb-build-cull.yaml`.

`analyses-split` splits each claim whose text runs past the line it is titled by. The claim keeps
that line. A new analysis carries the rest: the claim's own block or paragraph, or, where that holds
nothing more, the proof that establishes it. The claim's premises and cited works move onto the
analysis, and the analysis lifts the claim, its bearing left for you to judge. It asks no model.

## Resuming and monitoring

Each stage boundary is a commit in your repository, with subject `kb-build: <stage-id> | <display
name>`. `kbase build <volume-root>...` starts a build; `kbase build --resume` continues it from the
last boundary on the commit trail, and takes no volume roots or `--bibliography`. After an
interrupted stage, it restores the paths the build owns to the last stage commit and re-runs that
stage.

The build owns its inputs. A relative volume root or `--bibliography` resolves against the project
root, and the build orders each list by full path, whatever order you gave it in, so the order you
give does not change the result. Every boundary's body ends with a `volume-root:` line per volume
root, then a `bibliography:` line per `--bibliography` file, each absolute and in that order; the
`start` boundary also records `started-at:`, the commit the build began from. On a branch with no
commit, the build first makes an empty `kb-start:` commit to start from. A resume continues with
the inputs the newest boundary records.

A fresh build is refused wherever `kb-root/` exists, so a different input set is a fresh build
reached only by starting over: remove `kb-root/`, or reset to `started-at` (below). So is a full
build after a `--no-inference` one: `kbase build --resume` over the finished trail is `unchanged`,
and the dropped steps run only in a build that starts over from `started-at` and runs without
`--no-inference`.

`kbase build-trail` (also an [MCP](mcp.md#using-kbase-from-an-agent-harness-mcp) tool) lists the trail,
read-only: each boundary oldest first with its commit and the command that rewinds to it, and
`started-at`, the clean, ready-to-build state before the build, with the command that starts over.
A boundary's command is `git reset --keep <commit>` followed by the recorded `kbase build --resume`,
carrying the flags the build's last run was launched with, the bound excepted; `started-at`'s is the
reset alone, after which a build is fresh and names its own volume roots. kbase runs none of them;
run them as listed. `git reset --keep` refuses rather than discard uncommitted changes in the files
it touches, and `dirty` lists the worktree's changed paths. `kbase status` lists each stage done or
not done, and names `build-trail` as where the commits are.

`build` runs in the foreground (through the [MCP server](mcp.md#using-kbase-from-an-agent-harness-mcp) it
is detached). From the repository, in a second shell:

```sh
kbase status   # state, pid, stage commits, progress, the resume command
kbase cancel   # stop the build; costs only the calls in flight
```

State that does not belong in the repository lives in `$XDG_STATE_HOME/kbase/<key>/`, or in
`--state-dir`: the pid and holder lock that `status` and `cancel` read, `progress.jsonl`, per-call
evidence, the answer cache, and the captured output of a build the MCP server started. It holds
paid-for inference and survives `git clean`.

The store is versioned: `<state-dir>/format` is `1`, and `build`, `status`, `wait`, `cancel`,
`build-trail` and the MCP `build` tool refuse a newer store (`check: state-dir`). Each build
keeps the newest six captured outputs of builds the MCP server started (`reports/build-*`), and the
answer cache, multichoice-ask records and call captures of the current build trail and the five before
it. `status` reports `cache-entries` and `cache-bytes`. Deleting the store is safe but
forfeits the cache.

The operation lock is not in the store: it is `kbase-operation.lock` in the repository's git
directory, so one run — a build or a scoring run, never both — runs per repository whatever
`--state-dir` each names.

The build records are tracked in `kb-root/.build/`, as YAML: `kb-build-node-pass.yaml`,
`kb-build-unmarked.yaml`, `kb-build-classification.yaml` and `kb-build-cull.yaml`.

## Creating an empty KB

A KB can also be created empty, with no LaTeX and no build — `git init` for a KB:

```sh
git init notes-kb && cd notes-kb
kbase init
```

`init` writes the skeleton — an `entry-point.md` headed by the repository's name (or `--title`'s
text), `.index/` with the format stamp at `.index/format.yaml`, `AGENTS.md`, `CONVENTIONS.md` and
the `CLAUDE.md` redirect —
derives its summary pages and claim-graph sheet, and commits `kb-root/` as the birth commit. It runs
no `verify` of its own: `kbase verify` over a freshly born KB is green. Creation is birth-once: a
second `init` is refused (`check: kb-root`), and so is `kbase build` over a created KB. What the KB
holds afterwards is authored with the write ops — add a node, edit its fields, add an edge, cite a
node — in write batches. Details are under
[SPEC §8](../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md).
