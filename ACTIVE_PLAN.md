# ACTIVE PLAN – closed; the carried record

**Status:** no plan under way. The two tranches (the metadata format version, the MCP server) and
the review wave are delivered and closed, with the record in the git history. Next: a progress
merge to `main`, then the inference R&D plan, which inherits this record and replaces this file.
kbase is the canonical source of the KB toolchain's design; kb_tools adopts its changes, and leads
only on defect fixes found in use (SPEC §2). The contract documents govern on any disagreement with
this file, and this file is corrected.

## Rules still in force

- **Rolling rule (every wave exit).** Acceptance lines that play out → the next wave starts; a
  reasonable gap → the coordinator adjusts, records the adjustment in the checkpoint commit,
  continues; a large gap → stop, settled with the owner. A checkpoint commit between every dispatch;
  the diff is audited, not the report.
- **The measurement corpus is private** (the owner's directories under `test_data/transient/`, named
  in nothing tracked); the arxiv fixtures and the compat harness are the public evidence. In those
  fixture repositories kbase is exercised through an untracked justfile the owner keeps there —
  `just kb-verify`, `just kb-stats`, `just kb-refresh`, running `bin/kbase` by default; the
  checked-in Makefile's `kb-*` targets are kb_tools' and are not kbase's test procedure.
- **Both toolchains move together.** A format change is handed to adjagent with the wave that lands
  it; a change is finished when kb_tools can read what kbase writes.

## Carried record

**Decided earlier, still binding.** Templates are authored here under prompt-engineer review; no
test asserts a prompt's prose; the inference passes' measured constraints — no batching of
candidates into one N-letter answer, no embedding model in the shortlist, no persona system prompt
on a letter ask, no design assuming prefix-cache reuse. *Jointly established units* (a strongly
connected component a person promotes into one node carrying the solidity its members share; the DAG
constraint over units) are a later major bump, after the demoted relation has been lived with.

**Successor R&D:** the node-matching gap (seven of the fixture author's 43 claims match no minted
node; the rest match noisily; recall against the author's graph cannot be read until it closes); the
directional shortlist (`ROADMAP_PLANS` once written); the uncertainty pool and hit list
(`ROADMAP_PLANS/UNCERTAINTY_POOL_AND_HIT_LIST.md`).

**Owner items.** One named divergence stands in SPEC §4, the build inputs on the trail, with its
hand-off in `../adjagent/KB_BUILD_INPUTS_HANDOFF.md`. adjagent findings carried from the port:
`kb-docent` and `kb-maintainer` name kb_tools' maintenance surface only (the docent's query
surface, with `strengthen_by` and `gated-on`, is in `../adjagent/KB_QUERY_ROWS_HANDOFF.md`'s
successor record on adjagent's side); the Edit/Write-only coder rule belongs in adjagent's shared
coder chunk (drafted in `../adjagent/proposed-agents-md-edit.md`).

**Carried facts.**

- Every refresh, so every write op, runs `dot` once per sheet with no deadline; the wall time per
  write op on the fixture is unmeasured (`internal/sheet`, `renderSVG`).
- No arxiv fixture paper produces a ring, so the demoted relation's end-to-end case runs on a
  synthetic two-lemma paper (`internal/build`, `TestDemotedRing`); a KB whose cuts are plain
  `references` (built by either toolchain before it carried the relation) draws no cuts until
  rebuilt.
- A default-limit `solidity-below` result on a scored KB can exceed personant's 8 KB cap; the arXiv
  fixtures carry no scores.
- On Windows `status` cannot see a running build (no advisory lock).
- A block inside the abstract is treated as not at the margin; no corpus paper exercises it.
- Python edge cases not reproduced: non-ASCII digits, final-sigma lowercasing, a directory named
  `*.md`; line splitting on rarer Unicode separators affects reported line numbers only.
- The server is not deterministic at temperature 0; about one percent of nodes is the noise floor
  for any single build comparison.
- The sheet reads a labelled block as "a blockquote whose first non-empty line opens bold"
  (adjagent's rule, which its goldens require: a label and its statement on one line, `**Theorem
  1.** …`), while the claim graph reads label lines through `kb.LabelLine`; the two recognisers
  differ (`internal/sheet`, `internal/kb`).
- The MCP `build` tool's stdout and stderr captures accumulate under the state store's `reports/`;
  nothing removes them (`cmd/mcp.go`).
- A build started through MCP that refuses inside a stage row (pandoc preflight, the charter)
  returns `done` from the start, and `status` shows the refusal only under `recent-refusals`.
- The `0.9.0 → 1.0.0` converter does not handle a document carrying two comment blocks or a CR-only
  document (`internal/migrate`); neither occurs in any fixture.
- A build over a KB that already stands at an older format drains its obsolete files only at the
  first save after spine-seed.
- The trail records only `--bibliography` files given on the command line; a `.bib` found beside a
  volume root by default is not recorded, so adding one is not caught at resume (`internal/build`).
  A hand-made `kb-build:` commit with an empty body is not compared.
- Policy-test gaps: an `exec.Cmd{Path: …}` literal passes the exec owner's binary check
  (`cmd/execpolicy_test.go`); the import test treats `cmd` as one package, except for the `model`
  rule, which it checks per file (`cmd/importpolicy_test.go`).
