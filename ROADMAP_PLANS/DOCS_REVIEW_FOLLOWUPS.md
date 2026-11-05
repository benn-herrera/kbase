# Follow-ups from the project-document rewrite

The rewrite of THESIS, SPEC, ARCHITECTURE and CONVENTIONS left SPEC ahead of the code in places, and
found code defects and gaps. Each item states the change and where it lives. ARCHITECTURE describes
only landed code, so each code change updates ARCHITECTURE in the same change.

## SPEC rules the code does not yet meet

- **The decision-API setting's name.** `providers.toml` key `decisionApi`, `configure-providers` flag
  `--decision-api`, `show-config` field `decision-api`, with no alias for `decisionsFlavor`,
  `--decisions-flavor` or `decisions-flavor` (SPEC §9.1 The decision API). Code: `internal/config`,
  `cmd/configure-providers.go`, `cmd/show-config.go`.
- **An identical add is `unchanged`.** An add that would change no byte of `kb-root/` succeeds as
  `unchanged`, reporting the existing item; a differing add stays refused (SPEC §8 Idempotence,
  `add-<kind>`, Edge ops). An identical `add-citation` — the line already holds exactly the link it
  would write — is `unchanged` too (SPEC §8 `add-citation`). Code: `internal/write`. ARCHITECTURE
  invariant I6 and its detail change with it.
- **A refused restore names the edge now standing.** A `restore-demoted` refused because no demoted
  record exists says, where a scoring edge between its ends now exists, that it does and names `edges`
  (SPEC §8 `restore-demoted`). Code: `internal/write/edges.go`.
- **Help text** (SPEC §11 Served text):
  - the help of an op that takes a `rationale` says that what would strengthen a node is answered by
    `trace` with `sort: contribution`, `order: ascending`;
  - a help that tells a caller to copy an edge row into an edge op gives the bearing-copy advice;
  - `update-format`'s help says to run it after upgrading kbase;
  - the help of an op that takes a `snippet` recommends quoting a few distinctive words, not the whole
    line.
- **Leaf discovery** excludes names SPEC's leaf definition does not state (`internal/kb/kb.go`
  `ExcludeDirs`, `ExcludeNames`: `session/`, `tools/`, `invariants.md`,
  `claim-quality-closure-roadmap.md`). Align the code with SPEC §10 Documents of a KB.

## Code changes decided in the rewrite

- **One retry rule for every model call.** A call that did not get through (transport error, 429,
  5xx, an undecodable response body) waits 5 s, then 30 s. A retry that changes the request because
  the answer came back flawed (refused, malformed, failing its check) goes out at once. Decision calls
  repeat comms failures at once today (`internal/model/decision.go`); bring them in line.
  ARCHITECTURE §7 Retry and §12 Constants change with it.

## Tests missing

- Each op's key list against SPEC §8's key tables (SPEC states the keys; the code keeps them in
  `internal/write`).
- Only the result emitter writes stdout (ARCHITECTURE invariant I5 has no enforcing test).
- The sheet's determinism: one DOT rendered twice through the same `dot` gives identical bytes.

## Code defects

- `Ledger.Repriced()` (`internal/score/ledger.go`) has no caller outside its file.
- `asks.Lint` (`internal/asks`) is called only by a toy test; the real template checks live in
  `internal/score/score_test.go`.

## Documents

- SPEC §7's long table cells — the `remedy` rule and the `write`, `show` and `stats` result rows —
  become lists or sub-tables.
- Once adjagent's `dev-process` skill lands, remove from CONVENTIONS the generic process it carries
  (Plan & Execute Process, Coordinator Policy and similar), keeping what is specific to kbase.
