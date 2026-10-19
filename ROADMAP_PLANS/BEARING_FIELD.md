# `rigor`, `bearing`, `strength` — four scoring relations at 2.0.0

The next KB-format change; not under way. **It lands as one change, not six checkpoints**: the
vocabulary, the fixtures and the walk move together, so `just test` is green only at the end. A
partial landing is a tree of renamed types against old keys.

## Outcome

Three names. A node's local worth is `rigor`; a scoring edge's relevance scalar is `bearing`; the
value the KB publishes is `strength`. Four relations state the arithmetic, all stored
supporter → supported: `supports` and `founds` multiply into the min, `lifts` raises above it,
`refutes` caps from above. The register stays claim-centric under a `- rests-on:` list that the parse
flips.

The corpus is re-derived and re-scored under the new arithmetic; no mechanical equality is promised.

## The model

**Three quantities.**
- `rigor` — claim, support, work, experiment. Replaces `confidence`, `quality`, and the work's and
  experiment's `strength`.
- `bearing` — every scoring edge. Replaces `applicability`, `fraction`, and the `strengthens` pair's
  `strength`.
- `strength` — the published derived value, what bands and the sheet read. Replaces `solidity`.

**The walk.**
```
derivation = min(rigor, every supports contribution, every founds contribution)
strength   = max(derivation, every lift)                 # a lift raises above the min
strength   = min(strength, every refute cap)             # a cap bounds from above
```
`derivation` is the intermediate min; `strength` is the published value. A support or experiment
lifts `strength` above `derivation`; a refutation caps it. Several refutations take the *minimum*
cap; they do not stack.

**Relations, stored push (source = supporter).**
- `supports` — an internal result (claim or experiment) → claim or support. Contribution
  `bearing × supporter strength`, into the min.
- `founds` — an external work → claim or support. Contribution `bearing × work rigor`, into the min.
  It multiplies; the old gate retires, so `0.5` is half and `0.0` contributes zero rather than
  skipping. A low bearing is a penalty on the citing claim — borrowing a great name for a weak
  argument — and the remedy is to remove the edge, not to keep a low value. The bearing is judged
  for the **finding the bullet names** — the piece cited, not the whole work that contains it — and
  the `[context]` is where that finding is named; the work's `rigor` stays the work's own standing.
- `lifts` — post-hoc corroboration (support node or experiment) → claim.
  `bearing × lifter strength`, into the max.
- `refutes` — counter-evidence (support node or experiment) → claim. Cap
  `1 − bearing × refuter strength`.
- Framework premises are `supports` from the invariant or axiom with **no bearing**, contributing
  `1.0`.
- `references` and `demoted` are **not** push: they keep the citer's frame (claim → claim), the
  exception to the direction rule. `demote-claim-edge` and `resolve-demoted` each perform one flip at
  their boundary.

**`RelationSupports` keeps its name, not its old rows.** The old `supports` rows become `lifts`; the
old `depends` rows become `supports`. Mapping old→new by name moves a lift into the min and every
supported claim falls.

**Legality.** `BearingLegal(relation, sourceKind, targetKind)` answers *may this pairing carry a
bearing*, not *may this edge exist*: `supports`→claim from a claim or experiment yes, from a framework
no; `founds`→claim from a work yes; `lifts`/`refutes`→claim from a support or experiment yes. One
rule, used by the parse, the writer and `verify`.

**Pending.** A pending `rigor` leaves that node's `strength` pending; nothing else pends. A pending
`bearing` means the edge does not participate: excluded from the min, no lift, no cap. `0.0`
participates with zero, so the claim's min goes to zero. A bearing is never a poison — it is either
a link or it is not one, and an unjudged edge is not. A pending derivation with a lift standing above
it reads as the lift: the claim has no derivation number but does have corroboration.

**Direction and authoring.** Stored scoring edges are `Source` = supporter, `Target` = supported. The
register stays claim-centric: `- rests-on:` on the claim or support, and the parse flips — a `clm-` or
framework token becomes `supports` into the entry, a `wrk-` token becomes `founds` into it. The
inversion appears in exactly three places: the parse, `deps`, and the writer's `authoredHome`; the
last two must derive their mapping from `kb`, not spell it again.

`deps <id>` reports every incident *scoring* edge (both directions) **plus `id`'s own outgoing
`references` and `demoted`** rows, as `relation`, `direction` (`in`/`out`), `target` (the other end),
`bearing`, `context`. A claim's premises are its `in` rows whose relation is `supports` or `founds`.
`deps -i` and `DependentsOf` keep their names and meaning.

**Two passes, one ask each.** Pass A: the rigor of one work, seeing the work alone. Pass B: the
bearing of one work to one target, seeing both statements, never the rigor answer. kbase composes;
the model never returns a pair. **External works are outside both sets:** a `wrk-` node is never
assigned a rigor and a `founds` edge never a bearing, because neither can be read from the corpus —
a person authors them through `set-rigor` and `set-bearing`. The commands refuse those inputs so a
dispatcher mistake surfaces.

**Version and migration.** `2.0.0`: renamed fields and changed semantics, both major triggers. One
`1.0.0 → 2.0.0` step. Not value-identical: a support lift moves from the local min to above it (a
supported claim rises where the lift exceeds the derivation), and `founds` scales (a work bearing
`< 1` falls; `0` zeroes). The old `applicability` numbers were authored as gates, so after
conversion every work premise's bearing means something stronger than it did and needs a human
re-read; the announcement says so alongside the rescore. Every unannotated premise converts to
`1.0`, not pending, so it keeps participating exactly as it did; a newly authored citation is the
only edge that starts pending. The installed base is two KBs; both are re-derived and re-scored by
their first `refresh`.

**Announcement.** The loader records the step it ran for that call:
`(s *Source) Migration() (Migration, bool)` — set by `kbload.migrated`, gone with the Source. `cmd`'s
one `migrationNotice` writes the stderr line and returns the document's conditional `migration: {from,
to, note}`, placed after `kb-root`, on every verb that loads the KB and not on `status`/`cancel`.

## Work packages

**WP1 — `internal/kb`.**

- Rename the type system: `confidence`/`quality`/work-`strength` → `rigor`; `applicability`/
  `fraction`/edge-`strength` → `bearing`; `solidity` → `strength`. Go symbols follow: `DependsOn` →
  `Premises`, `DependsOnEdges` → `OutgoingEdges`, `DependsGraph` → `PremiseGraph`,
  `TargetSolidityRecorded` → `SourceStrengthRecorded`, `LocalQuality` → `Rigor`.
- One `kb.Bearing{Value float64; Pending bool}`; `ParseBearing`/`BearingText`; the pending literal
  unchanged. `(strength …)` on a premise bullet records the **supporter's `strength`** — the derived
  value, not its `derivation` — because that is what `refresh` reads today (`finals[target]`); bands
  read the same number.
- Two constants, not one: the resume of the index file (`edges.yaml`, `lifted-by.yaml`) and the
  register list key (`rests-on`). `checkEdgeClass` and the drop ops take the list constant; index
  readers take the index constant. `IndexDependsOn` currently serves both, which is why a single
  rename breaks the other home.
- The register key is `- rests-on:`; `premises()` flips it. A framework token yields `supports` with
  no bearing. The support node's pair list is authored in **both** homes — the register's staged
  `sup-` entry and the leaf's `support-nodes` mapping — and both become `lifts:`. An experiment's
  `strengthens:` list becomes `lifts:`; its `rigor` lives in its host leaf's `experiment-nodes`
  mapping (it has no register entry).
- Requiredness gate in `kb.Discover` over the notes the parse records, with the fault table:
  R1 missing bearing on a legal edge; R2 a bullet naming several scoring targets; R3 a misplaced
  bearing; R4 an out-of-domain token; R5 two bearings on one bullet. All five refuse with path and
  line. A bullet naming one stale id remains a logged drop, not a refusal — refusing would make a KB
  with one stale id unopenable.

**WP2 — index, query, sheet.**

- Index files `edges.yaml` and `lifted-by.yaml`; rows sort by `source` then `target`.
- Node rows carry `rigor`, `derivation_strength`, `strength`, `build_status`, `build_band`,
  `rationale`, `premise_count`, `strengthen_by_count`, `citation_count`. Edge rows carry `source`,
  `target`, `relation`, `target_kind`, `source_strength_recorded`, `bearing`, `context`, plus `origin`
  on demoted. `fraction`, the edge row's `strength`, and the separate experimental branch are all
  gone (`experimental_solidity` dropped: an experiment is a `lifts` edge now).
- `strength-below` replaces `solidity-below`; `strength_bands` replaces `solidity_bands`; `show`,
  `stats`, `weak-points` and the query payloads print the new keys.
- `deps` gains `direction`; the inverse query reads both ends.
- The sheet draws push arrows, names the four relations in its stroke legend, reads `strength` for
  fill and bands; the `rests-on` diamond becomes the `supports`/`founds` rows; band slugs and
  thresholds are unchanged.
- `strengthen-by` is a list of textual items, not a relation: it, `strengthen_by_count`,
  `strengthen-by.yaml` and `gated-on` **stay**.

**WP3 — `internal/write`, and every writer that consumes it.**

- `set-bearing` (`id`, `target`, `relation`, `bearing`) replaces an existing token through one
  `authoredHome(edge)`: a premise edge on the target entry's `rests-on` bullet; a lift/refute edge on
  the source's pair line in either home. It never inserts; refuses an illegal pairing, a relation
  outside the four, a value outside `[0,1] ∪ {*pending*}`; idempotent.
- `set-rigor` absorbs `set-work-strength` and reaches work and experiment ids: every register kind
  writes `- rigor:`, an experiment writes `rigor:` in its leaf mapping. `set-applicability`,
  `set-on-point-fraction` and `set-work-strength` retire.
- Inserts: `add-rests-on` replaces `add-depends-on`; `insert-*` take `rests-on` of `{id, context,
  bearing}` with a new premise defaulting to `*pending*`; `insert-experiment-entry` takes `status`,
  `rigor`, `supports`, `lifts`, `refutes`; `insert-work-entry` takes `rigor` (required);
  `insert-support-entry` takes `lifts`/`refutes`.
- `renderDependsOnBullet` writes `(strength …)` then `(bearing …)` on a claim target and
  `(bearing …)` on a work target.
- `expectedEdge` carries `kb.Bearing`; `edgeOf` copies it whole; `edgesEqual` compares value and
  pending; `normalized`'s rebuild carries it too; every expectation for a legal edge states
  `Pending: true` where the writer renders pending, including `resolve-demoted … restore`.
- The build's own writers — `internal/claimgraph/write.go` sends `insert-work-entry` with `strength`
  and `add-build-edges` with `depends-on`/`references` — move to `rigor`, `add-rests-on` and the
  pending default. Nothing in `claimgraph` may write a bearing number.

**WP4 — version, converter, fixtures, announcement.**

- `kb.FormatVersion = "2.0.0"`; `internal/migrate/from_1_0_0.go` with a narrow `1.0.0` reader and one
  `convert100To200`: rename `- confidence:`/`- quality:`/`- strength:` → `- rigor:`; `- depends-on:`
  → `- rests-on:`; `- solidity:` and `(solidity …)` → `- strength:`/`(strength …)`; `(applicability
  X)` → `(bearing X)` carrying `X`; an untokened claim bullet gains `(bearing 1.0)`; an untokened
  work bullet gains `(bearing 1.0)` too, so it participates as the old gate did; support-node and
  experiment `supports:`/`strengthens:`
  lists → `lifts:`, each value becoming its bearing; each experiment gains `rigor: 1.0`; a
  multi-target bullet splits into one per scoring target, title and `[context]` copied, framework
  tokens on the first, the recorded `(strength …)` on the bullet it was read from and
  `(strength *pending*)` on the others; the old index files are declared obsolete.
- Inserted-token position: after `(strength …)`, else before `[context]`, else end of line; never
  before an existing parenthetical. A carried token stays where it was. The converter stamps nothing,
  cannot fail, and holds no second copy of the domain.
- Fixtures: a **new** golden pair `register-1.0.0 → register-2.0.0` with a real register in it —
  `internal/migrate/testdata/1.0.0` has no register and no pair line, so it exercises none of these
  renames. The whole-KB golden for the 0.9.0 chain stays as it is. Every current-form fixture, inline
  literal, write-api golden and sheet golden moves; refusal probes move to `3.0.0` and `2.1.0`; the
  `init`/`refresh` rendered-template goldens move with the templates.
- `kb.Migration`/`Source.Migration` and `cmd`'s one `migrationNotice` as above; `write.Result` carries
  no migration record — the command layer reads it from the source it opened.

**WP5 — the gate, verify, the query refusal.** `kb.Discover` collects the R1–R5 notes into
`MalformedError` with path and line; the CLI and MCP emit them. `verify` never sees the rows the gate
refuses — it returns the load refusal, one item per fault — and its own job is the backstop over index
rows (a bad bearing, a misplaced one). `query.Load` refuses a migrated source (`kb-format`, remedy
`kbase refresh`), then over a current index refuses a legal row without a token or an illegal row
with one (`index`, remedy `kbase refresh`). `drop-claim-node` and `drop-inbound-edges` must see the
new pair lists; today they read claim bullets only.

**WP6 — contract, templates, fixtures they force.** SPEC §3 (the sheet's drawn relations),
§5 (attribution writes `supports`, not `depends`), §7 (the **shared** key order gains `migration`;
`deps` shape), §8 (the op set), §10 (the three names, four relations, the walk, required
annotations, migration and its non-identity, the query refusal). ARCHITECTURE §6 Write API, §10
refresh/verify and the conformant-reader sentence, §11 loader, §12 constants and Format migration.
`README`, `INTEGRATING`, `USER_CONVENTIONS`' op inventory, `ROADMAP` (next major → `3.0.0`;
"intervolume" replaced with "external work"), and the `justfile`'s `strength-below` line. The KB's
own templates (`internal/kbdocs/templates/README.tmpl.md`, `CONVENTIONS.tmpl.md`) name the retired
vocabulary in every installed KB — `tech-writer` owns the prose; moving their goldens is WP4's.

**Outside this repository, with owners.** `kb-tools`' reference schema must accept `2.0.0` before the
compat harness compares anything; its owner updates `kb_schema.py`. The RFE
`../adjagent/CLAIM_SCORE_BEARING.md` becomes two passes with one ask each; its owner rewrites it, and
per the operator's rule it gets prompt-engineer review before the scorer reads it. Its "rigor is
unaffected" clause is already false after this change (work and experiment rigor move), and its
"one claim and one `depends` target" names a retired relation.

## Facts this plan acts on

Re-establish each before editing; the symbol is the check, not the line.

- `rg -n -i 'solidity' internal cmd -g '!*_test.go' -g '!testdata'` → 190 lines today; widen to
  `rg -n 'solidity|confidence|quality|applicability|fraction|depends-on|strengthens'` for the union.
- `rg -n 'func ComputeSolidity|func supSolidity' internal/index/solidity.go` → the walk (`~340-430`).
- `rg -n 'func normalized|func edgesEqual' internal/write/store.go` → the rebuild and comparator.
- `rg -n 'func replaceApplicability' internal/write/ops.go` → the replace-only splice model.
- `rg -n 'func planDropInboundEdges|func planDropClaimNode' internal/write/ops.go` → the two drop ops.
- `rg -n 'type ExperimentNode' internal/kb/leaf.go` → no local score today.
- `rg -n 'IndexDependsOn' internal cmd` → the constant with two homes.
- `grep -rh 'kb-format' test_data/fixtures | sort | uniq -c` → two files, both `1.0.0`; no
  `claim-quality.md` under `test_data/fixtures`.

## Acceptance

1. `set-rigor` writes the local value for claim, support, work and experiment (the last in its leaf
   mapping); `set-bearing` writes a bearing on each of the four relations and refuses a relation
   outside them, an illegal pairing and a value outside `[0,1] ∪ {*pending*}`.
2. Round-trip: registers and leaves re-read with annotations unmoved and unduplicated;
   `(strength …)` still before `(bearing …)`; and on a fixture where a supporter's `derivation` and
   `strength` differ, the annotation carries the **strength**.
3. `deps <claim>` returns every incident scoring edge with `direction` and the other end, plus the
   claim's outgoing `references`/`demoted`; its premises are the `in` rows of `supports`/`founds`;
   null appears only on rows that cannot carry a bearing.
4. The walk is exact: one numeric fixture per relation; a pending `rigor` leaves `derivation`
   pending, and the strength is the lift if one stands, else pending; a pending `bearing` excludes
   its edge entirely; `0.0` contributes zero; a pending refutation caps nothing.
5. A `lifts` edge from a support whose `bearing × lifter strength` exceeds the claim's `derivation`
   raises `strength` to it; an experiment's did already.
6. `founds` scales: `0.5` contributes half the work's rigor, `0.0` contributes zero rather than
   skipping.
7. The converter, asserted on the new golden's bytes: `rigor:`, `rests-on:`, `lifts:`, `(bearing …)`
   carried from the old value, the `1.0`/`*pending*` defaults, the experiment `rigor: 1.0`, and the
   split bullets with title and context intact; the golden's output loads through `kb.Discover`.
8. A damaged `1.0.0` bullet still converts, and the load refuses it by name with path and line.
9. The migrating call carries `migration` after `kb-root` in its document and the sentence on
   stderr; the saving call carries it; later calls do not.
10. A 2.0.0 register carrying each of R1–R5 is refused naming file and line; `verify` reports the
    load refusal it receives, and its index backstop finds a bad or misplaced bearing without masking
    the rest. A bullet naming one stale id loads with a logged drop.
11. A query over an unmigrated KB is refused with remedy `kbase refresh`; `verify` still runs.
12. A build writes `*pending*` bearings and no number; `resolve-demoted … restore` reads back what it
    wrote.
13. The sheet fixture's DOT emits each premise edge source→target (not `dir=back`), its legend names
    the four relations, and its band slugs and thresholds are unchanged.
14. The index and query corpora do not collapse: every value they computed at `1.0.0` is reproduced,
    except where a test exercises the lift's station or a `founds` bearing below `1`; the zero-valued
    `kb.State` fixtures carry `Bearing{Value: 1}` where they reach the walk.

## Gates

One landing: `just test`, `just edit-gate`, `just test-integration` (including the results-fixture
member that runs `strength-below`), `just test-integration-mcp`. The compat harness is not a gate
until `kb-tools` accepts `2.0.0`. `cmd/resultkeys_test.go` needs `migration` in the **shared** key
order and the renamed sub-keys. Write files with the edit and write tools only.

## Limits

No bearing is drawn on the sheet. No “unscored” query: the record cannot distinguish a judged edge
from a skipped one. A claim or work cannot `refutes` another: counter-evidence is a support node or
an experiment, and a contrary claim is asserted the other way — additive later. The appliance version
stays `0.6.0`: kbase has not started major versions, and the format version is independent of it.

Two read-only verbs are a separate change and the scoring passes' only input: `rigor-text <id>` and
`bearing-text <source> <target> <relation>`, each returning closed, score-free text. They depend on this
format (field names, and where the scores they strip live) but not on the walk; the seats cannot go
live before they exist. Three things that change must resolve: the index carries titles, rationales
and edge contexts but no statement body and no derivation passage, so the verbs render from leaves
(or the index grows that text), which runs against the index-only query rule as it stands; no command
can tell a human from a scorer, so the work exclusion lives in the verbs' refusals and the seats'
tool allowlists, not in `set-rigor`/`set-bearing`; and the cited finding is prose in `[context]`,
with no id or range tying a `founds` bearing to the piece it judges.
