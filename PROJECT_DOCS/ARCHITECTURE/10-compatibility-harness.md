## 10. Compatibility harness

Compatibility is judged by kbase's own readers and checks, never by a byte diff of presentation. The
hermetic gate is `just test-integration`, whose nine members — `test-integration-results-fixture`,
`test-integration-init`, `test-integration-opscript-results`, `test-integration-mcp`,
`test-integration-config`, `test-integration-auth`, `test-integration-parity-adjagent-sheet`,
`test-integration-refused-write-adjagent-sheet` and `test-dist-no-replay` — each judge what their
process produced: the first eight run `./bin/kbase` over fixtures they stage, judging the result
document against the verb's keys, the tree it wrote, the transcript it served, the streams and files a credentials command left, or — for the parity
recipe — the frozen answers the 2.0.1 binary gave over the KB the conversion reads, each moved only
by a rule naming a designed outcome of the conversion; `test-dist-no-replay` judges the default
build's command tree for the compiled-out replay. Each of the first eight runs the script of its own name
under `tools/scripts/`, and each writes its log under `test-data/transient/<recipe>/`. The recipes
the rule in CONVENTIONS.md "Testing" keeps out of the gate run by name —
`test-integration-mcp-build`, the MCP notification probe that builds a paper, among them.

---

