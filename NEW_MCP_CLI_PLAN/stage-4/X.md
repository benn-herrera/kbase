### X — `measure-mcp-surface` captures the release build (expert)

**Design:**
- [§4.1 The instructions](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md) — item 1, which each captured mode varies
- [§4.3 Server modes and `project-dir`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md) — the modes the captures cover
- [§4.5 Served resources](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md) — `kbase://sheet/`, served only in located mode with a KB

**Scope:**
- `justfile` (a new `build-release` recipe; `measure-mcp-surface`'s dependency, its `BIN_DIR` and
  its doc);
- `tools/scripts/measure-mcp-surface.sh` (a located-with-KB capture; its header comment and
  evidence line).

**Facts acted on:**
- `justfile` `build`: `go build -tags kbasedev -o {{BIN_DIR}}/kbase ./cmd` (`justfile:113`). Its doc
  (`:110`) says an untagged build "is the release shape and carries none".
- `measure-mcp-surface: build` (`justfile:258`), and it passes `BIN_DIR="{{BIN_DIR}}"` (`:260`).
  The script runs `${root}/${BIN_DIR}/kbase` (`tools/scripts/measure-mcp-surface.sh:30`). So it
  measures the development build today.
- Build outputs go under `bin/` ([CONVENTIONS "Build and Run"](../../PROJECT_DOCS/CONVENTIONS/build-and-run/index.md)). `bin/` is gitignored
  (`.gitignore:5`).
- A recipe doc is at most 200 ASCII characters (`tools/scripts/check-just-docs.sh`, run by
  `edit-gate`). `measure-mcp-surface`'s doc (`justfile:257`) is 196 characters today.
- The script's captures and the [page §4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md) mode each serves:
  - `run/frames.jsonl` and `run/served/resources.jsonl`: `kbase mcp` in a temporary directory
    under no repository, so not located;
  - `run/served/required.jsonl`: `kbase mcp --project-dir-required`, not located;
  - `run/served/building.jsonl`: `kbase mcp` in a repository with no `kb-root/` (`served/building/`
    holds a bare `.git`), located, the case [page §4.1](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md) item 1 describes as "the repository's where
    `kb-root/` is yet to be built".

  No capture serves located mode with a KB that opens, where [page §4.1](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md) item 1 names the KB's path
  and [page §4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md) lists `kbase://sheet/` resources.

**Work:**
1. Add `build-release`, which runs `mkdir -p {{BIN_DIR}}/release` and then
   `go build -o {{BIN_DIR}}/release/kbase ./cmd`, with no tag. Its doc, 128 characters:
   `build bin/release/kbase for the host platform with no build tag: the release shape the tap ships, carrying no development replay`.
2. Make `measure-mcp-surface` depend on `build-release` in place of `build`, and pass
   `BIN_DIR="{{BIN_DIR}}/release"`.
3. Replace `measure-mcp-surface`'s doc with this text, 197 characters:
   `hermetic, a measurement and no gate: ./bin/release/kbase mcp, the release build, under no repository; surface.yaml holds its sizes, run/served/ every resource read, the other modes and every --help`.
4. Add a capture `run/served/located.jsonl`: `kbase mcp` in a fresh git repository under the run
   tree, holding a KB that the same binary's `kbase init` made there, sent `initialize`,
   `notifications/initialized`, `resources/list`, and a `resources/read` of every
   `kbase://sheet/` URI that list names. Its exit goes to `located.exit`, checked like the other
   captures'. The KB comes from `init` alone: no fixture is copied, and nothing under `.index/` is
   written by hand (Rule 10). The script's header comment and its evidence line name the capture.

**Done-line** (after Rule 13's two): `just --dry-run measure-mcp-surface`, `just measure-mcp-surface`.
