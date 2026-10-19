# Phase 1 — the `2.0.0` format

Status: ready. Landing: one commit series, merged once. Depends on: nothing.
Gates: `just test`, `just edit-gate`, `just test-integration`, `just test-integration-mcp`.

## One landing — read this first

`just test` is red from the first rename until the last fixture is restamped. That is expected and
is not licence to commit a partial tree: a tree of renamed types against old keys is the failure this
warning exists to prevent. Work the packages below in order on one branch; run the cheap gates as you
go; run the full gates only when WP7 is complete; merge once. `just checkpoint` fails until then and
must not be "fixed" by loosening a gate.

The order inside the landing is the work-package order, WP1 through WP7. Two things are pinned
inside it: `kb.Discover`'s requiredness gate is satisfiable only once WP5 has restamped every
fixture, and the version bump is WP5's alone.

## Outcome

A KB at `2.0.0` carries three quantities and four scoring relations. A `1.0.0` KB opens through one
converter. No derived-value equality is promised: lifts move station, `founds` scales, and untokened
work edges stop pending.

## The model

**Three quantities.**

| Name | What it is | Home | Replaces |
|---|---|---|---|
| `rigor` | one node's local value, `[0,1]` or `*pending*` | claim, support, work, experiment | `confidence`, `quality`, the work's and the experiment's `strength` |
| `bearing` | one scoring edge's scalar, `[0,1]` or `*pending*` | every scoring edge | `applicability`, `fraction`, the pair `strength` |
| `strength` | the derived value, `[0,1]` or pending | claim, support | `solidity` everywhere |

**Four relations, one direction: `Source` is the supporter, `Target` the supported.**

| Relation | Source | Target | Bearing | Contribution |
|---|---|---|---|---|
| `supports` | claim, experiment | claim, support | required | `bearing × source strength` into the min |
| `supports` | invariant, axiom | claim, support | illegal | `1.0` into the min |
| `founds` | work | claim, support | required | `bearing × work rigor` into the min |
| `lifts` | support, experiment | claim | required | `bearing × lifter strength`, maxed above the min |
| `refutes` | support, experiment | claim | required | cap `1 − bearing × refuter strength`; smallest cap wins |
| `references`, `demoted` | claim | claim | illegal | none |

`RelationDepends`, `RelationRestsOn` and `RelationStrengthens` retire. `RelationSupports` keeps its
name and not its old rows: old support-pair edges become `lifts`; old `depends`→claim edges become
`supports`. `BearingLegal(relation, sourceKind, targetKind)` answers only *may this pairing carry a
bearing*, not *may this edge exist*.

**The walk.**

```
derivation = round2(min(rigor, every supports contribution, every founds contribution))
final      = round2(min(max(derivation, every lift), every cap))
```

- A pending `rigor` leaves that node's strength unknown; nothing else pends.
- A pending bearing, or a pending source strength, excludes its edge: no contribution, no lift, no
  cap. `0.0` is a judgement and participates (the min goes to zero; a `0.0` refute caps at `1.0`).
- A non-`run` experiment is inert as a lifter or refuter; its `supports` edge is excluded like any
  unjudged premise.
- One rounding per published quantity, over unrounded products. `MinDependencyStrength` multiplies
  every participating edge by its bearing and omits excluded edges.
- `Result` carries `Rigor`, `MinDep`, `Derivation`, `Lift`, `Cap`, `Final`; `RenderStrengthTrace`
  emits the five forms in the design's innermost-first order (min-only; lift above; cap binding;
  both; lift with a pending derivation).
- Walk order follows value dependency, which is the push direction. The acyclicity path graph is its
  reverse: one rule, `g.Add(e.Target, e.Source)`, for every scoring relation.

**Authoring.** The register stays claim-centric. A claim's or support's `- rests-on:` bullets name
premises; the parse flips them: a `clm-` or framework token yields `supports` **into** the entry, a
`wrk-` token yields `founds` **into** the entry. An experiment's and a support's own pair lists
(`supports:` on an experiment, `lifts:`, `refutes:`) are authored on the source and are already
push. `rests-on` is the list key only; it is not a relation name in `2.0.0`.

**One scoring target per bullet.** Two or more `clm-`/`wrk-` tokens on one bullet is refused;
framework tokens do not count. A writer never emits one.

**Register shape, in this order for writers (readers are position-insensitive):**

```
- rigor: 0.85
- strength: 0.62 (use as input only) [= min(0.85, 0.62)]
- rationale: …
- rests-on:
  - clm-bbbbbb — Second Claim (strength 0.90) (bearing 0.4) [ctx note]
  - wrk-aaaaaa — A Cited Work (bearing 0.8) [Theorem 3]
  - INVARIANT-3 — Framework premise (a context)
- references:
- demoted:
- strengthen-by:
```

`(strength …)` on a premise bullet records the **source's** derived strength (not the
contribution). `(bearing …)` judges the edge, and on a `founds` bullet it judges the finding named
in the trailing `[context]`, not the work as a whole. A work bullet has no `(strength …)`; a
framework bullet has neither bearing nor strength.

**Faults.** Each is raised by `kb.Discover`, before any state is built, at the bullet's first
physical line, as one `kb.AnnotationError` carrying `AnnotationFault{Field, Path, Line, Allowed,
Remedy, Detail}`. The result item is `check: metadata`, `key` the field name.

| # | A `rests-on` bullet | Fault |
|---|---|---|
| R1 | names two or more scoring targets | ambiguous; remedy: split the bullet |
| R2 | yields a scoring edge and has no `(bearing …)` | missing; remedy: add the token |
| R3 | carries a `(bearing …)` outside `[0,1] ∪ {*pending*}` | domain; `allowed` names the set |
| R4 | carries a `(bearing …)` and names no scoring target | misplaced |
| R5 | carries two or more `(bearing …)` tokens | ambiguous |

A `references` or `demoted` bullet carrying a bearing is R4. A bullet naming one scoring target that
resolves to no node stays today's logged drop: no edge exists, so nothing is required.

## Non-goals — do not "finish" these

- No writer rule and no `verify` check pairing a numeric bearing with a pending `(strength …)`.
  `refresh` rewrites the strength annotation independently; such a rule would blank an authored
  bearing or refuse a legitimate save. The pairing is inert because a pending source strength
  excludes the edge.
- No bearing drawn on the sheet; no second visual axis.
- No pending count, no "unscored" query, no alias for a retired name, no fifth relation.
- No refutation authored on a claim or a work in `2.0.0`.
- `### Quality`, `claim-quality.md`, `<!-- claim-quality: … -->`, `QualityFieldKeys`,
  `strengthen-by`/`strengthen_by_count`, `confidence` in `asks.Reply`/`ItemRecord`/
  `CandidateEntry`, `DependentsOf`/`deps -i`, and the band names keep their words. They name a
  section, a file, a marker, a text list, a letter-confidence or a graph relation — not a retired
  field.

## Work packages

### WP1 — `internal/kb`: the vocabulary, the parse, the gate

**Scope.** `internal/kb/{schema,register,records,leaf,discover,indexstream,format,render}.go` and
their tests.

**Build.**

- `type Bearing struct { Value float64; Pending bool }`; `kb.Fraction` goes. Zero `Value` is the
  judgement `0.0`; presence is never read off `Value`. `FanOutRecord.Staged/Declared` become
  `*Bearing` (nil = that end does not record the edge); `Agrees` compares pointed-to values and
  treats two pendings as agreement; `Ends` tests nil.
- `Edge` becomes `{Source, Target, Relation, TargetKind, SourceStrengthRecorded *float64, Context
  *string, Bearing Bearing, Origin string}`. `SupportPair` and `StrengthensPair` retire; every
  scoring collection is `[]Edge`. `ClaimEntry.Premises` and `SupportEntry.Premises` (rename
  `DependsOn`→`Premises` on both; rename `SupportQuality`→`SupportEntry`), `SupportEntry.Lifts`,
  `SupportEntry.Refutes`,
  `ExperimentNode.Supports/Lifts/Refutes`, `ExperimentNode.Rigor`.
- `Relations` = `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted`. Delete the three
  retired constants and `IndexDependsOn`'s double duty: `IndexEdges = "edges"` for the index file
  base and `ListRestsOn = "rests-on"` for the register list key. `IndexSupportedBy` and
  `SupportedByRow` retire
  with the file — `edges.yaml` carries every scoring edge including lifts. `IndexStrengthenBy` and
  `StrengthenByRow` stay: strengthen-by is a text list, not a score.
- Parse: `premises(line, entryID, entryKind) []Edge` replaces `dependsOn`; the flip lives here and
  nowhere else. Pair-line readers emit `Edge`s and require a readable value; a pair with no value is
  malformed, not pending. `ParseBearing`/`BearingText`/`BearingFromValue` and one `BearingRE`; the
  pending literal is unchanged. `ParseBearing` accepts `[0,1]` and the literal only.
- Render: `RenderBearingAnnotation`, `RenderStrengthAnnotation`; `(strength …)` first, `(bearing …)`
  second, `[context]` last; `parenContext` skips `(bearing …)` and nothing else.
- `Discover` gains the fault sink: each bullet reader records `(field, line, fault)`; `Discover`
  collects them into one `AnnotationError`. Naming the one stale id stays a logged drop.
- Index stream: `NodeRow` carries one `Rigor *float64` (key `rigor`) instead of `Confidence`,
  `Quality` and the work's `Strength`; `DerivationStrength`, `Strength`, `PremiseCount`; drop
  `ExperimentalSolidity`. `EdgeRow.Bearing any` sits after `SourceStrengthRecorded` (renamed from
  `TargetSolidityRecorded`, key `source_strength_recorded`) and before `Context`; the edge `Strength`
  column goes; `Fraction` goes. Delete `seedIndexRow`'s `relation: "depends"` default: a row missing
  its relation reads `""` and is refused.
- The version stamp is untouched here; WP5 owns the bump to `2.0.0`.

**Facts the step rests on — re-run each before editing.**

- `rg -n 'RelationDepends|RelationRestsOn|RelationStrengthens' internal cmd | wc -l` → **54**. Every
  hit retires or is spelled as text by the converter.
- `grep -n 'FormatVersion = ' internal/kb/format.go` → `internal/kb/format.go:10`, `"1.0.0"`.
- `grep -n 'type ExperimentNode' -A4 internal/kb/leaf.go` → `internal/kb/leaf.go:53-56`: `ID, Title,
  CanonicalPath, CanonicalAnchor, Status` and `Strengthens []StrengthensPair`. There is no local
  score and no register entry; its `rigor` lives in the host leaf's `experiment-nodes` mapping.
- `rg -n -i 'confidence|quality|applicability|fraction' internal cmd -g '!*_test.go' -g '!testdata'
  | wc -l` → **303**. The union with `solidity` (190) and `depends-on` (188) is the rename surface;
  the census in WP7 reads it after.

**Acceptance.** A `2.0.0` register with each of R1–R5 is refused by `check: metadata` at the
bullet's line; a claim+framework bullet and a two-id `references` bullet load; a bullet naming one
stale id loads with a logged drop. `0`, `1` and pending round-trip through `ParseBearing`/
`BearingText`.

### WP2 — `internal/index`: the walk and the derived artifacts

**Scope.** `internal/index/{solidity→strength,records,refresh,verify,dependspath,compat_test}.go`.

**Build.**

- Rename the file and its symbols: `ComputeStrength`, `supStrength`, `MinDependencyStrength`,
  `FormatStrength`, `RenderStrengthLine/Annotation/Trace`, `StrengthBelow`. Implement the walk
  above: two stations, one rounding per quantity, pending local and excluded edges, non-run
  experiments inert, `0.0` participating. `Result` carries the six values.
- `records.go`: node rows, edge rows (`bearingValue(f, legal)` is `null` when illegal, else
  `BearingText`), `premise_count`, `edges.yaml`; rows sort by `source` then
  `target`.
- `refresh.go`: rename every surface; the `(strength …)` rewrite keeps its subject (the bullet
  head's **first claim id**, i.e. the premise's value) and its two regexes. `refusal` gains the
  `errors.As` branch that maps `kb.AnnotationError` to one result item per fault.
- `verify.go`: `strength freshness`; a backstop over index rows — a bearing non-null exactly where
  `BearingLegal` allows and in domain, null everywhere else; `target_kind == node_type(target)` for
  every relation. On a migrated source the bearing backstop is skipped.
- `dependspath.go`: `g.Add(e.Target, e.Source)` for each scoring relation; a support that lifts a
  claim it rests on is a cycle.
- `compat_test.go`: the harness's case table is `1.0.0` text. Rename its mutations and probes to
  `2.0.0` (`edges`, `lifts`, `refutes`, `bearing`, `rigor`); `pendingScore` and `pendingApplies`
  become the `rigor` line and `(bearing …)`. The comparisons stay flag-gated; this package makes the
  harness coherent, not live.

**Facts the step rests on.**

- `grep -n 'func ComputeSolidity\|func supSolidity\|func MinDependencySolidity'
  internal/index/solidity.go` → `:274`, `:406`, `:162`.
- `grep -n 'pendingScore\|pendingApplies' internal/index/compat_test.go` →
  `internal/index/compat_test.go:175-176`.
- `grep -n 'func edgeRow\|func buildClaims\|func buildDependsOn' internal/index/records.go` →
  `:86`, `:39`, `:91`; `buildSupportedBy` at `:143` retires with its row type.

**Acceptance.** A lift above the derivation raises the final; one below leaves it; a binding cap
takes the minimum; a halved binding bearing moves the dependent to the exact product; a `0` premise
drives the claim to `0.00` while the same fixture without the edge does not; a pending premise
leaves the claim at its rigor. A refused neighbour does not queue: pending never propagates.

### WP3 — query, sheet, and the query verbs

**Scope.** `internal/query/{payload,query,load}.go`, `internal/sheet/{graph,dot}.go`,
`cmd/{deps,show,stats,weak-points,solidity-below→strength-below}.go`.

**Build.**

- `nodeFields`: claims carry `rigor`, `strength`, `premise_count`; supports `rigor`, `strength`;
  works `rigor`; experiments `status`, `rigor`.
- `DepsPayload` returns every incident **scoring** edge (both directions) plus the node's own
  outgoing `references`/`demoted`, as `relation`, `direction` (`in`/`out`), `target` (the other
  end), `bearing`, `context`; fixed relation order `supports`, `founds`, `lifts`, `refutes`,
  `references`, `demoted`, then the other end's id. `null` means only *this row cannot carry a
  bearing*. The inversion is one adapter here.
- `DependentsOf` and `deps -i` keep their names and read both ends: sources of `supports`/`founds`
  into `id`, targets of `id`'s outgoing scoring edges.
- `stats`: `strength_bands`, `weakest[].strength`, `highest_leverage[].strength`, `edges`;
  `show` `premise_count`.
- `query.Load`'s `requiredFiles` (`internal/query/load.go:17`) drops `depends-on` and gains `edges`;
  `strengthen-by`, `cites` and `subtree-aggregates` stay. It refuses an index that would answer a
  legal edge with no token: `check: index`, remedy `kbase refresh`. A migrated source is refused
  once with the same remedy.
- Sheet: fill and band legend read `strength`; strokes name the four relations; arrows point in the
  push direction; the transitive reduction runs over `supports` push edges; the cut legend reads
  "cut from supports"; band slugs, thresholds and labels unchanged.
- `solidity-below` becomes `strength-below` (`--max-strength`, `DefaultMaxStrength`).
- `weak-points`' own `--max-solidity` (`cmd/weak-points.go:23`) and `query.DefaultMaxSolidity`
  (`internal/query/query.go:23`) become `--max-strength` and `DefaultMaxStrength`, with the help
  sentence reading strength. No alias.

**Facts the step rests on.**

- `grep -n 'func DepsPayload' internal/query/payload.go` → `internal/query/payload.go:103`: it
  emits `applicability` only for `rests-on` today.
- `grep -n 'func (ix \*Index) DependentsOf\|func (ix \*Index) SolidityBelow' internal/query/query.go`
  → `:65`, `:197`.
- `grep -n 'solidity-below' justfile` → `justfile:1120`, the results-fixture leg.

**Acceptance.** `deps <claim>` reports its premises as `in` rows; a support's lifts as `out` rows; a
framework premise's bearing is `null`; a pending bearing is the literal and a `0` is `0`; the order
does not move when the KB's edges are re-sorted.

### WP4 — `internal/write`: the ops and the read-back

**Scope.** `internal/write/{ops,values,render,store}.go`, `cmd/{set-applicability,set-on-point-fraction,
set-work-strength,insert-*,drop-*,add-depends-on,set-rigor,set-frontmatter}.go`, and
`internal/claimgraph`'s writers.

**Build.**

- `set-bearing` (`id` the source, `target`, `relation` one of the four, `bearing` a number in `[0,1]`
  or `*pending*`) replaces an existing token and never inserts one. It locates the authored home
  through one `authoredHome(edge)`: a premise edge on the target entry's `rests-on` bullet whose head
  names the source; a lift/refute edge on the source's pair line, in its leaf or the register's
  staged copy. It refuses an illegal pairing (`BearingLegal`), a relation outside the four, and an
  out-of-domain value; a re-issue is `unchanged`. Model it on `planSetApplicability`
  (`internal/write/ops.go:756`) and `planSetOnPointFraction` (`:2021`).
- `set-rigor` remains one op and reaches claim, support, work and experiment ids. Every register kind
  writes `- rigor:`; an experiment writes `rigor:` in its host leaf's `experiment-nodes` mapping.
  Delete `rigorField`, `registerEntry.ScoreField` and `planWorkInserts`' score patch-back
  (`ops.go:502`, `:677`). `set-work-strength`, `set-applicability` and `set-on-point-fraction`
  retire; no aliases.
- `add-depends-on` becomes `add-rests-on`; `insert-claim-entry`/`insert-support-entry` and
  `add-rests-on` take `rests-on: [{id, context, bearing}]` with a new premise defaulting to
  `*pending*`. `insert-experiment-entry` takes `status`, `rigor`, `supports`, `lifts`, `refutes`;
  `insert-support-entry` takes `lifts`/`refutes` (not `supports`); `insert-work-entry` takes `rigor`
  (required, as `strength` was).
- `renderDependsOnBullet` (`internal/write/render.go:104`) writes `(strength …)` then `(bearing …)`
  on a claim target, `(bearing …)` on a work target, and the context parenthetical on a framework
  target. `renderDemotedBullet` carries no bearing.
- `drop-claim-edges`' `edge-class` values become `rests-on` and `references`; `checkEdgeClass`
  (`internal/write/values.go:273`) keys on the **list** constant, not the index file name.
- `drop-claim-node` (`ops.go:1034`) and `drop-inbound-edges` (`ops.go:868`) widen to the pair lists:
  a `supports`/`lifts`/`refutes` edge into or out of the node is authored on a support or experiment
  leaf, and both ops must see it or they strand an edge.
- `set-frontmatter`'s `experiment-node` table grows `rigor`, `supports`, `lifts`, `refutes`; its
  `support-node` table's `supports` becomes `lifts` and gains `refutes`.
- Read-back: `expectedEdge` carries `kb.Bearing`; `edgeOf` (`internal/write/store.go:103`) copies it
  whole instead of projecting a `*float64`; `edgesEqual` compares `Value` and `Pending`. Every
  expectation on a legal edge says `Pending: true` unless it carries a number; `resolve-demoted`'s
  restore site is the one that otherwise fails its own read-back.
- The build writes no judgement: `claimgraph` sends `rigor`, `add-rests-on`, and
  `(bearing *pending*)` on every edge it adds; nothing in `claimgraph` writes a bearing number.

**Facts the step rests on.**

- `grep -n 'func planSetApplicability\|func planSetOnPointFraction\|func planDropInboundEdges\|func
  planDropClaimNode' internal/write/ops.go` → `:756`, `:2021`, `:868`, `:1034`.
- `grep -n 'func edgeOf' -A14 internal/write/store.go` → `:103-112`; the `Fraction.Set` projection is
  the three-state collapse this changes.
- `grep -n 'func checkEdgeClass' internal/write/values.go` → `:273`, and its detail names
  `kb.IndexDependsOn` at `:280`.
- `grep -n 'func renderDependsOnBullet' internal/write/render.go` → `:104`.

**Acceptance.** `set-bearing` lands on each of the four relations and re-reads the same state;
`*pending*` re-reads as pending and `0` as `0`; a missing token is a load fault before the op runs;
an illegal pairing, an unknown relation and an out-of-domain value are refused. `set-rigor` writes
claim, support, work and experiment homes. A new premise is `*pending*` and moves no claim. A build
leaves every edge it writes pending and no claim's value moves.

### WP5 — version, converter, announcement, fixtures

**Scope.** `internal/migrate/{migrate,from_1_0_0}.go` and testdata, `internal/kbload/kbload.go`,
`internal/kb/source.go`, `cmd/main.go`, `internal/query/load.go`, all fixtures.

**Build.**

- `internal/migrate/from_1_0_0.go`: `convert100To200(in Files) (Files, []string, error)`, registered
  as the `1.0.0 → 2.0.0` step. It imports nothing of kbase (the package rule) and carries its own
  narrow `1.0.0` reader: entry bounds, the field lines it renames, the two annotations, one head
  cut. It cannot fail: a file it does not locate it does not touch; a damaged `1.0.0` file is the
  load's to refuse.
- Renames and rewrites: `- confidence:`/`- quality:`/work `- strength:` → `- rigor:` (value copied
  byte for byte); `- depends-on:` → `- rests-on:`; `- solidity:`/`(solidity …)` →
  `- strength:`/`(strength …)`; `(applicability X)` → `(bearing X)` carrying `X`, `*pending*` and
  `0` included; an untokened claim or work premise bullet gains `(bearing 1.0)`; support
  `supports:` and experiment `strengthens:` become `lifts:`, each value its bearing; every
  experiment gains `rigor: 1.0`; a multi-target bullet splits into one bullet per scoring target,
  title and context carried, framework tokens on the first, the recorded `(strength …)` on the
  bullet it was read from and `(strength *pending*)` on the others; `.index/depends-on.yaml` and
  `.index/supported-by.yaml` are declared obsolete. Inserted-token position: after the
  `(strength …)`, else before the trailing `[context]`, else end of line, never before an existing
  parenthetical. A carried token stays where it was.
- No direction flip in bytes. The flip is the parse's; the index is regenerated.
- Version and load: `FormatVersion = "2.0.0"` — the bump is this package's, and no part of WP1–WP4
  moves it; `kbload.migrated` records the step; `Source` gains
  `migration` and `Migration() (Migration, bool)`, with
  `type Migration struct { From, To, Note string }`, set by `MigratedSource`, survived by `Saved`,
  and no in-process step clears it. `cmd`'s one `migrationNotice` writes the stderr sentence once per
  call and returns the document's conditional `migration: {from, to, note}` after `kb-root`, on
  every verb that loads the KB and never on `status`/`cancel`.
- `query.Load`'s refusal (WP3) is what a migrated source meets: `check: index`, remedy
  `kbase refresh`, named once; the command writes nothing before it.
- Fixtures:
  - New golden pair `internal/migrate/testdata/register-1.0.0` → `register-2.0.0`: a real register
    carrying claim, support and work score lines; an indented score line; a stray wrong-kind key; a
    claim bullet with strength and context; a work bullet with and without an applicability; a
    framework bullet; a two-claim bullet; a claim+work bullet; a support premise list; a support's
    `supports:` pairs; an experiment's `strengthens:` pairs; a title containing `bearing 0.5`.
  - A second, byte-only old→new case carries the malformed `(applicability sometimes)`; it converts
    to `(bearing sometimes)` and is asserted to be refused by `kb.Discover` as R3. It is not part of
    the loading golden pair: a goldens file that cannot load would make the pair's load assertion
    say nothing.
  - `internal/migrate/testdata/1.0.0`, `internal/migrate/testdata/0.9.0` and
    `internal/kbload/testdata/{0.9.0,loose-0.9.0}` stay byte-unchanged; the load path now runs two
    steps.
  - Restamp: `test_data/fixtures/init/skeleton` (stamp only), `test_data/fixtures/results`,
    `internal/sheet/testdata/adjagent-sheet/kb-root` (stamp, `.index/depends-on.yaml` →
    `.index/edges.yaml`, `.index/supported-by.yaml` removed, `rigor` in claim and support rows; the
    eight old `depends` rows become `supports` — `bearing: "1.0"` on the six claim-target rows and
    `null` on the two framework rows — the old `rests-on` work row becomes `founds` with its
    `fraction 0.7`, and the old `supports 0.5` and `strengthens 0.8` become `lifts` at those
    bearings; DOT goldens byte-identical),
    `internal/write/testdata/writeapi-render-golden/*`, and
    `internal/write/testdata/writeapi-regression/part{1..5}-claim-quality.md`.
  - Every inline register literal in `internal/index`, `internal/query`, `internal/write` and
    `internal/kb`.
  - Inline `kb.State` corpora carry `Bearing{Value: 1}` on every scoring edge that reaches the walk.
    A coder who updates only the parse fixtures sees the index expectations collapse to zero; the
    `internal/index` `dep()` helper is the one-line fix.
- `cmd/resultkeys_test.go` learns the conditional shared key: strip `migration` after `kb-root`, and
  add `migration`'s sub-keys `{from, to, note}` to `documentedSubKeys`.

**Facts the step rests on.**

- `grep -n 'migration' cmd/resultkeys_test.go` → no output: the shared-key test has no `migration`
  strip, and will fail the first document that carries it.
- `grep -n 'func Chain\|var converters' internal/migrate/migrate.go` → `:33`, `:27`; the chain is a
  list one entry long.
- `grep -n 'func MigratedSource\|func (s \*Source) Saved' internal/kb/source.go` → `:42`, `:65`.

**Acceptance.** A `1.0.0` KB opens, is `2.0.0` after one `refresh`, and `verify` reports it stale
before that. The golden pair converts byte for byte; its `2.0.0` output loads through `kb.Discover`
with the same edge set (push pairs) and every carried token value, while the byte-only case's
`(bearing sometimes)` is refused as R3. A `2.1.0` and a `3.0.0` KB are
refused `check: kb-format`. A query on a converted-but-unsaved KB is refused with remedy
`kbase refresh`. A `0.9.0` KB yields one `0.9.0 → 2.0.0` record.

### WP6 — contract text, templates, the diagram

**Scope.** `SPEC.md`, `ARCHITECTURE.md`, `README.md`, `USER_CONVENTIONS.md`, `ROADMAP.md`,
`INTEGRATING_KBASE_VIA_MCP.md`, `internal/kbdocs/templates/{README,CONVENTIONS}.tmpl.md`,
`RIGOR_SCORING_DIAGRAM.md`.

**Build.** Each edit is a rename or a statement of the model as built; none is a redesign.

- **SPEC §3**: the sheet paragraph — nodes filled by strength band, strokes naming the four
  relations and their push direction.
- **SPEC §5**: attribution writes `supports`, not `depends`.
- **SPEC §7**: the shared vocabulary gains the conditional `migration` record after `kb-root`;
  `deps`' records are `relation`, `direction`, `target`, `bearing`, `context`; `show`'s keys are
  `rigor` and `strength`; `stats`' are `strength_bands` and `edges`; the query list's
  `solidity-below` becomes `strength-below`; the `stats`/`weak-points` headings gain one sentence
  each that the value surfaces read the derivation while `dependents`, `premise_count` and the
  counts are structural.
- **SPEC §8**: `set-bearing` replaces `set-applicability` and `set-on-point-fraction`; `set-rigor`
  absorbs `set-work-strength`; `add-depends-on` becomes `add-rests-on`; `drop-claim-edges`'
  `edge-class` value becomes `rests-on`; the insert value shapes are named.
- **SPEC §10**: one paragraph for the three quantities and the four relations' arithmetic; one for
  edge direction and authoring; one for required bearings, one scoring target per bullet, the R1–R5
  refusal by path and line, the index column's position and `null`'s meaning; the `rigor`/`strength`
  renames with the unchanged section/file/marker names; the migration paragraph with the three
  value changes stated explicitly and the announcement; the version statements and the closing
  sentence become `2.0.0`; the `demoted` paragraph keeps `1.0.0` and its `depends` references become
  `supports`.
- **ARCHITECTURE §6**: the one bearing op and the one rigor op, replace-only, read-back, and that no
  op renders a scoring bullet without a token and no build path writes a judgement; a claim's and a
  support's local value are one field. **The query layer**: `deps`' incidence adapter, and only it;
  no text verb is added (Phase 2 documents the internal renderers). **The derived layer**: point at
  `internal/index/strength.go` as the arithmetic's authority and carry the four-relation table with
  the worked example — this is where the root diagram's content lands. **§11's `internal/kbload`
  entry**: the requiredness gate, the loader monopoly, the zero-value hazard (a register parsed
  outside `Source` reads a missing token as `0.0`, a zero contribution), the migration record and
  announcement. **§12 constants**: the format row, the pending literal, the conversion record.
  Rename §4's stage-table `depends-attributed` row, §6's mechanism table's index row and §3's sheet
  paragraph.
- **README** and **USER_CONVENTIONS**: the query and op lists, the sheet's fill, the state of a
  claim that carries no rigor.
- **INTEGRATING_KBASE_VIA_MCP.md**: `add-depends-on` → `add-rests-on`, `solidity-below` →
  `strength-below`; no other change.
- **ROADMAP.md**: the bearing item is shipped at `2.0.0`; drop "intervolume" (no referent exists);
  move the next major bump to `3.0.0`.
- **`internal/kbdocs/templates/{README,CONVENTIONS}.tmpl.md`**: product prose for installed KBs;
  `solidity`→`strength`, the two local-score names→`rigor`, `depends-on.yaml`→`edges.yaml`,
  `solidity_bands`→`strength_bands`. Their rendered goldens move in WP5.
- **`RIGOR_SCORING_DIAGRAM.md`**: tracked at the root since `7561fbe`. Its content moves into
  ARCHITECTURE's derived layer and the file is removed with `git rm` in this landing. A fourth
  document restating the walk would rot against SPEC and ARCHITECTURE.
- **`ROADMAP_PLANS/BEARING_FIELD.md`**: already removed with the checkpoint that queued this plan;
  its content is folded here. Do not recreate it.
- **CONVENTIONS.md**: no change. Say so in the commit body so a reviewer does not look for one.

**Acceptance.** Every edited sentence holds against `./bin/kbase --help`, one run of each renamed
query, and the golden pair. `rg -n 'solidity' README.md USER_CONVENTIONS.md SPEC.md ARCHITECTURE.md`
returns only §10's superseded-format history and the code spans that name the old form.

### WP7 — the census and the close-out

**Build.** Run the census before and after; read the hits, do not count them:

```
rg -n -i 'solidity' internal cmd -g '!*_test.go' -g '!testdata'                       # 190 today
rg -n -i 'confidence|quality|applicability|fraction' internal cmd -g '!*_test.go' -g '!testdata'
                                                                                       # 303 today
rg -n '"confidence"|"quality"|"solidity"|"applicability"|"fraction"' internal cmd -g '*_test.go'
                                                                                       # 19 today
rg -n 'RelationDepends|RelationRestsOn|RelationStrengthens' internal cmd                # 54 today
rg -n 'depends-on' internal cmd                                                        # 188 today
```

After: the first three return only the survivors named under Non-goals and the migration's spelling
of old keys as text; the relation constants return nothing; `depends-on` returns only the
converter's read of the old key and SPEC §10's `1.0.0` description of it. Assert the survivor list as
a complete set in a test or a reviewed grep, not as a count.

## Contract deltas

Phase 1 owns: SPEC §3, §5, §7 (the `migration` record and the renamed keys), §8, §10; ARCHITECTURE
§3's sheet paragraph, §4's stage row, §6, the derived layer, §11, §12; README; USER_CONVENTIONS;
ROADMAP; `INTEGRATING_KBASE_VIA_MCP.md`'s renamed verbs; the two
`kbdocs` templates. CONVENTIONS is unchanged. Phase 2 owns SPEC §3's scoring sentence, §7's `score`
row, §8's `score` entry and §11's exclusion list.

## Acceptance observations

Each is an observation; the falsifier is what a reviewer runs to disprove it. A work package's
`**Acceptance.**` block is its done-check; these are the landing's.

1. **A judgement on any scoring edge.** `set-bearing` lands on each of the four relations and
   `verify` returns `done`; the value re-reads as the same state. *Falsified by:* a refusal, a fault
   after the write, or a written `*pending*` that re-reads as a number.
2. **It survives the round trip.** `refresh` neither strips nor moves the token; `0` re-reads as
   `0`; `(strength …)` stays before `(bearing …)` before `[context]`. *Falsified by:* a lost or
   duplicated annotation, a moved position, or a `0` that reads as pending.
3. **One call reads every edge about a node.** `deps <id>` reports every incident scoring edge and
   the node's own outgoing `references`/`demoted`, with `direction`; a claim's premises are `in`
   rows; `null` appears only where a bearing is illegal. *Falsified by:* a missing key, a null on a
   scoring edge, an ambiguous row, or an order change.
4. **The walk composes exactly.** A claim's strength is `min(rigor, every product)`; a lift above it
   raises the final; the smallest cap binds; halving one binding bearing yields the exact product; a
   `0` premise drives the claim to `0.00` while deleting the edge does not; a pending premise leaves
   the claim at its rigor. *Falsified by:* a contribution that is not the product, a rounding applied
   twice or before a min/max, a lift inside the min, a cap that is not the smallest, or a structural
   fact that moves with a bearing.
5. **An old KB opens without hand edits.** A `1.0.0` KB opens through migration with `- rigor:`,
   `(bearing …)` on every scoring edge, `- strength:`/`(strength …)` and the renamed relation keys;
   `verify` reports it stale; after one `refresh` it is `2.0.0`. Claims whose lift exceeded their
   derivation read higher; a work bearing below `1` reads lower; a `0` work bearing reads `0.00`; an
   untokened work edge now reads a number, and an explicitly pending one is excluded so its claim
   reads its own value. *Falsified by:* a required hand edit, a refused load, an unscaled work edge,
   or a claim still pending on an untokened work edge.
6. **Every malformed bullet is refused by name.** R1–R5 each produce a `kb.Discover` refusal with
   `check: metadata`, `key`, `path` and `line`. *Falsified by:* a write or load that accepts one, a
   verify that passes such a row, or a fault with no file and line.
7. **A legal edge never reads as silence.** Every scoring edge reads a number or `*pending*`; a
   dropped token is a refusal, not a state. *Falsified by:* a scoring edge with no token in a loaded
   KB, or an absent token that reads as pending without a fault.
8. **The build is not a judge.** Every premise bearing it writes is `*pending*`; a claim it attaches
   an edge to reads unchanged until that edge is scored; a DOT golden differs only where a band
   moved. *Falsified by:* a built number, a built pending that overwrote a number, or a moved claim
   on an unscored edge.
9. **A rigor/bearing disagreement is a score, not a fault.** A high-rigor claim whose only premise
   bears `0.0` reads `0.00` and verifies `done`; a numeric bearing beside a pending `(strength …)`
   verifies `done`; no command counts pending bearings or compares the two. *Falsified by:* a
   finding for any of these, or a new count.
10. **A multi-target `1.0.0` bullet migrates.** Two claims, or a claim and a work, become one bullet
    per scoring target, legal under R1, with the same edge count and no lost framework token.
    *Falsified by:* a converter refusal, a load refusal on the converted bytes, or a changed edge
    count.
11. **An experiment keeps its contribution.** `strengthens` pairs of different strengths become
    `lifts` bearings of those strengths plus `rigor: 1.0`, and each lifted claim's final is unchanged
    where no other cause applies. *Falsified by:* a bearing that is not its old strength, a missing
    `rigor`, or a moved lifted value.
12. **The migration says what it did.** On a `1.0.0` KB, `verify` and a refused `deps` each carry
    exactly one `migration` record after `kb-root` and write the sentence once to stderr, until the
    saving call; a `0.9.0` KB yields `0.9.0 → 2.0.0`. *Falsified by:* a missing key, a duplicate, a
    key on a current KB, or a change visible only in a log.
13. **Three names cross the boundary, and the survivors are named.** `show` returns `rigor` and
    `strength`; `deps` returns `bearing`; `stats` returns `strength_bands`; the CLI offers
    `strength-below`; no score surface names `confidence`, `quality`, `applicability`, `fraction`
    or `solidity`, and the Non-goals list — a letter-confidence map, a quality section,
    strengthen-by, the band names — is the complete set of old words left. *Falsified by:* two names
    for one quantity, a score number that changed in the rename, or an old score name outside the
    list.
14. **The ledger-free surfaces are unchanged.** `stats`' counts, `dependents`, `premise_count`, the
    cycle report and the sheet's strokes do not move when a bearing changes. *Falsified by:* any of
    them differing across two runs that differ only in a bearing.

## Gates

`just test`; `just edit-gate`; `just test-integration` (its results-fixture leg now calls
`strength-below`); `just test-integration-mcp`. The compat harness is **not** a gate: its
comparisons cannot run until the reference toolchain reads `2.0.0` (Phase 3). `go vet ./...` must
pass with the harness's renamed case table.

## Ordering traps

Each rule stated in a work package is not repeated here. These are the ones whose cost is paid by
the *order* the work is done in, or that a coder must hold across packages.

1. **The annotation's subject does not flip.** `(strength …)` on a premise bullet records the
   *source's* derived strength, read off the bullet head's first claim id. `refresh` rewrites it
   there. Flipping it to the target would make every bullet repeat the entry's own line.
2. **`supports` keeps its name, not its rows.** Old support-pair edges are `lifts`; old
   `depends`→claim edges are `supports`. Mapping old rows by name moves a lift into the min.
3. **`deps`' order and `direction`.** Relation order `supports`, `founds`, `lifts`, `refutes`,
   `references`, `demoted`; then the other end's id. `null` means only *illegal here*. An inbound
   reference is `referenced-by`'s answer, not a second row.
4. **`DependentsOf` reads both ends.** Sources of `supports`/`founds` into `id`; targets of `id`'s
   outgoing scoring edges. A references or demoted edge makes neither end a dependent.
5. **The sheet's arrows flip.** Premise edges draw supporter→supported, so the arrowhead reverses
   relative to today; the transitive reduction runs over `supports` push edges (a work is a leaf,
   so `founds` reduces nothing); the adjagent-sheet DOT goldens stay byte-identical.
6. **`drop-claim-node` and `drop-inbound-edges` see pair lists.** A `lifts`/`refutes` edge is
   authored on its source; a drop that reads only claim bullets strands it.
7. **The converter imports nothing of kbase** and holds no second copy of the domain; it carries an
   existing token as text and lets the load refuse it by name. `1.1.0` does not exist and cannot be
   added.
8. **The index is never converted.** It is derived; the converter declares the two old paths
   obsolete and `refresh` writes the new ones.
9. **`justfile:1120`.** The results-fixture leg calls `solidity-below`; it moves with the command,
   and the recipe's key check catches a stale invocation.
