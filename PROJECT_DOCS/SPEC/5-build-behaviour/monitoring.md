### Monitoring

At the command line `build` runs in the foreground, and SIGINT or SIGTERM stops it resumably; over
MCP the `build` tool starts it detached ([§11](../11-the-mcp-server/index.md)). `kbase status`, `kbase cancel` and `kbase wait` ([§6](../6-build-state-ledger-and-monitoring/index.md))
observe, stop and await it.

**What a build produces**: Markdown leaves; the index pages, up-link and cross-reference structure;
load-bearing forms; the claim graph's node and edge records, among them an analysis for each claim
whose bounds contain text beyond its statement proper; the summary pages.

