- **Remove the inert `provider` key from `config.toml` at the next major.** It loads and chooses
  nothing today, and a file-mode run reports it superseded (SPEC §9). It leaves the schema once the
  reader-facing examples, `USER_CONVENTIONS.md` and adjagent's `references/scoring.md` have moved off
  it; until then strict load keeps modelling the key so the file a user has stays loadable and
  `kbase configure` can rewrite it.
- **Remove the inert `readerConcurrency` key from `config.toml`'s `[asks]` at the next major.**
- **Remove the environment inference mode** (`KBASE_API_BASE_URL`, `KBASE_API_KEY_FILE`,
  `KBASE_DECISION`, `KBASE_TEXT`, `KBASE_SCORING`). Nothing outside this repository uses it; the
  tests and integration scripts that do move to a scratch `--config-dir`, leaving file mode the one
  configuration path.

## The served surface

What an agent needs to drive kbase comes from kbase itself, and the text every session carries is
kept small: tool definitions and the MCP instructions load into every session's context on every
turn, where an MCP resource is read only when asked for. Measured from `./bin/kbase mcp` before this
work: 42 tools, `tools/list` about 35,000 characters, of which input schemas are about 28,000 and the
instructions 1,443.

- **Bind families, not verbs.** The queries are one MCP tool and the write ops one MCP tool, each
  naming its operation from an enum, each operation still a subcommand behind it. A family tool's
  arguments are checked by kbase, whose refusal names the key and the allowed set as the CLI's does.
  Commands that share no schema stay one tool each.
- **Per-operation detail is read on demand**, from an MCP resource or a describe call, not carried
  in every session's tool list. Text repeated across commands' help (the read-only and migration
  paragraph, the paging rules) is stated once.
- **One strength query** replaces `weak-points` and `strength-below`: claims filtered by `--min` and
  `--max` strength, `--min-dependents`, `--sort strength|dependents`, and `--pending` for the claims
  with no strength yet. No band flag: band names are prose, and a one-word alias for each is a
  second table to remember.
- **`edge-census` folds into `stats`** and leaves the surface; `stats` already counts the
  manual and model values its breakdown repeats.
- **Deleting a claim is one operation**, in place of the `drop-claim-edges`, `drop-inbound-edges`,
  `drop-claim-node` sequence.
- **`render-claim-graph` folds into `refresh`.** Drawing the sheets is a step of `refresh`, not an
  operation of its own.
- **`USER_CONVENTIONS.md` is retired.** What it carries that an agent needs and no served surface
  gives moves to a command's help, the MCP instructions or an MCP resource; the kbase skill
  (adjagent) takes something only as a last resort, each such case ruled on.
- `models`, `rewind-build` and `edges` stay: setup, recovery from a failed build, and the KB-wide
  list of one relation's edges are user operations. No command is gated on the `kbasedev` build
  tag by this work, and none is made CLI-only.
