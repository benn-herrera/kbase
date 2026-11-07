## 10. Compatibility harness

**Pattern:** Black-box integration tests with golden fixtures.

Compatibility is judged by kbase's own readers and checks, never by a byte diff of presentation
([§2](2-invariants.md), I4). The hermetic check is `just test-integration`, whose nine members each judge
what their process produced:

- `test-integration-results-fixture`, `test-integration-init`, `test-integration-opscript-results`,
  `test-integration-mcp`, `test-integration-config`, `test-integration-auth`,
  `test-integration-parity-adjagent-sheet` and `test-integration-refused-write-adjagent-sheet` run
  `./bin/kbase` over fixtures they stage. They judge the result against the verb's keys, the
  tree it wrote, the transcript it served, or the streams and files a credentials command left. The
  parity recipe judges the frozen answers the 2.0.1 binary gave over the KB the conversion reads, each
  moved only by a rule naming a designed outcome of the conversion. Each runs the script of its own name
  under `tools/scripts/` and writes its log under `test-data/transient/<recipe>/`.
- `test-dist-no-replay` judges the default build's command tree for the compiled-out replay.

The recipes that the rule in CONVENTIONS.md "Testing" keeps out of that check run by name;
`test-integration-mcp-build`, the MCP notification probe that builds a paper, is one.

Unit tests use golden-tree conformance fixtures: `internal/formats/testdata/` contains old bytes in and
new bytes out for each converter, with decoded-values goldens for the record files and a README naming
what each pair pins, and `internal/sheet/testdata/` contains the claim-graph sheet fixture and its DOT
goldens. A conversion or decode that differs from its golden is written under
`test-data/transient/unit_tests/formats/got/` with the golden's name, and a golden is replaced only by
copying that file over it.

---

