## House Rules

- **The exec monopoly (I1) is enforced by an AST-scan test** over every non-test Go file
  (`cmd/execpolicy_test.go`), and ARCHITECTURE §11's dependency rules by `cmd/importpolicy_test.go`
  over the transitive import graph. Reach pandoc, git or `dot` through the owning package, never
  beside it. The one self-exec owner is a file, not a package: `internal/mcp/launch.go` alone may
  exec the kbase binary, and the test fails on any other file that does. Four owners are the whole
  list — pandoc, git, `dot` and that self-exec — and a process started anywhere else fails the
  test. The policy scans share one module traversal (`cmd/modulescan_test.go`), so they agree on which
  files are the module's source. The monopoly binds the shipped code: the scan skips test files,
  and a test may start the processes its fixtures need. A test needing a binary the environment
  does not guarantee still belongs in a runner recipe, under "Testing"'s tool-dependency rule.
  Evidence gathered for the change at hand is not a verification but an
  exploratory check — a script run by hand in scratch, its log cited in the report, with no durable home. An
  exploratory check whose finding must keep holding becomes a verification: the check is added to a recipe. An
  exploratory check whose result a commit or plan cites keeps its command in the commit message, so a later
  reader can re-run it.
- **A pre-pass beyond I3's two is an ARCHITECTURE change, never a patch.** Nothing outside the
  registry massages source.
Contents:

- [The agent set is adjagent's, never edited here](the-agent-set-is-adjagent-s-never-edited-here.md)
- [Serialization formats](serialization-formats.md)
- [Superseded format parsers](superseded-format-parsers.md)
- [`kb-format` patch bumps](kb-format-patch-bumps.md)
- [Reads through `kb.Source`](reads-through-kb-source.md)
- [A result field's integer is `int`, never `int64`](a-result-field-s-integer-is-int-never-int64.md)
- [`just fmt` is the sanctioned formatter](just-fmt-is-the-sanctioned-formatter.md)
- [Never spell the node-kind list](never-spell-the-node-kind-list.md)
- [Never spell a relation](never-spell-a-relation.md)
- [A file kbase generates is regenerated whole](a-file-kbase-generates-is-regenerated-whole.md)
- [No stage exits on a model's opinion](no-stage-exits-on-a-model-s-opinion.md)
- [No test asserts a prompt's prose](no-test-asserts-a-prompt-s-prose.md)
- [A scoring-model fact lives once, in `internal/kb`](a-scoring-model-fact-lives-once-in-internal-kb.md)
- [A scoring value is derived in integer hundredths](a-scoring-value-is-derived-in-integer-hundredths.md)
- [Name the producer before you write the refusal](name-the-producer-before-you-write-the-refusal.md)
- [Compare a label in the page's spelling](compare-a-label-in-the-page-s-spelling.md)
- [Name a tool argument through the marker](name-a-tool-argument-through-the-marker.md)
