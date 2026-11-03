## Last step: the surface against the page (once, scratch only)

**Design:**
- [The whole design](../../NEW_MCP_CLI_DESIGN/entry-point.md) — the whole page: this step evaluates the final shape against it

After package X, an architect seat runs `just measure-mcp-surface` and compares the captured surface
with the [`NEW_MCP_CLI_DESIGN/`](../../NEW_MCP_CLI_DESIGN/entry-point.md) tree.

**What the reviewer receives:** the [`NEW_MCP_CLI_DESIGN/`](../../NEW_MCP_CLI_DESIGN/entry-point.md) tree, the capture directory
`test-data/transient/measure-mcp-surface/`, and this section.

- The capture: `surface.yaml`, `run/frames.jsonl`, and `run/served/` with `resources.jsonl`,
  `required.jsonl`, `building.jsonl`, `located.jsonl` and `cli-help/`.
- Each capture's mode, in [page §4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md) terms:
  - not located: `run/frames.jsonl`, `resources.jsonl` (no repository) and `required.jsonl`
    (`--project-dir-required`);
  - located, a repository with no `kb-root/` yet: `building.jsonl`;
  - located, a KB that opens: `located.jsonl`.
- The reviewer first checks that it is the release build: `run/served/cli-help/kbase.score.txt` must
  name no `--replay`.
- The comparison covers:
  - the 16 tools and their read-only annotations ([page §3](../../NEW_MCP_CLI_DESIGN/3-operations/index.md));
  - the four resource rows, `kbase://sheet/` from `located.jsonl`, and no subscribe capability
    ([page §4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md));
  - the eight instruction items in each of the three captured cases ([page §4.1](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md), [§4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md));
  - the CLI-only commands ([page §4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md));
  - the 42 operations ([page §3](../../NEW_MCP_CLI_DESIGN/3-operations/index.md) table), each present in `cli-help/` or as a `query` or `write`
    operation;
  - for each command, the shared body in `resources.jsonl`'s `kbase://help/<command>` equals the
    one in `cli-help/kbase.<command>.txt`, and the two differ only by their notes, each surface's
    spelling of an argument, and the CLI's usage line ([page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)).
- Anything the capture does not hold is reported as "not captured", never checked by running
  `kbase` by hand (Rule 11).
- The reviewer writes the findings to `.claude-temp/surface-check/<YYYY-MM-DD>/report.md`, each with
  the file and line of the capture it rests on.
- The coordinator gives the report to the page's owner, once. A finding is fixed in the served text
  or the code, or settled on the page by its owner.

This comparison is one-time. It is never a test, a recipe or a gate, nothing in the tree calls it,
and no finding of it becomes a burndown item that adds one.

Once its findings are settled and the served text has stopped moving, the coordinator maps the
final served text for adjagent: each text adjagent renders (the instructions, the tool
descriptions, the help resources, the stamped templates) with where it lives, so adjagent can
finish its release preparation.
