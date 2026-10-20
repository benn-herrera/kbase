# Phase 1 — the `2.0.0` format

Status: reissued for the node-pass rework. Landing: one commit series on `kb-2.0.0-format`, merged
once. Depends on: nothing.
Gates: `just test`, `just edit-gate`, `just test-integration`, `just test-integration-mcp`.

Design of record: [`STRENGTH_SCORING_DESIGN.md`](STRENGTH_SCORING_DESIGN.md). Where a step here and
the design disagree, the design is right and this document is to be corrected.

WP1–WP5 are **committed checkpoints, not finished work**: WP1 at `d2c49dc`, WP2–WP4 at `7e0cfee`,
WP5 at `81300cf`. They implement the model as it stood before the node pass — six node kinds with
`support` and `invariant`, `sup-` ids, `support-nodes`, `Strength.SupResults`, an axiom premise
exempt from the bearing requirement, `work-` ids, and a register named `claim-quality.md` under a
`### Quality` heading. Each work package below therefore states what is landed (verify, do not redo)
separately from what the rework adds, and no package is done because a commit carries its name.

A name in a **Landed** paragraph is in the tree at its commit and is to be checked there; a name in a
**Build** bullet that `rg` does not find is a target name this plan creates. `internal/kb`'s node
pass landed at `5ef3bc9`, so `AnalysisNode`, `AnalysisEntry`, `ParseAnalysisEntries`,
`AnalysisNodesKey`, `AnaIDKey`, `State.Analyses`, `State.Axioms`, `AxiomNode`, `ParseAxioms`,
`kb.AxiomsFile` and `kb.AxiomSource` exist. Still targets: `Strength.AnalysisResults`,
`AnalysisFinals`, `insert-analysis-entry`, and WP7's `RigorFieldKeys` and `claim-rigor.md`, with the
rest of WP2–WP7's lists.

## One landing — read this first

`just test` passes at `d103548` and goes red from the first `support`→`analysis` rename until the
last fixture is restamped. That is expected and is not licence to commit a partial tree: a tree of
renamed types against old keys is the failure this warning exists to prevent. Work the packages
below in order on `kb-2.0.0-format`; run the cheap gates as you go; run the full gates only when WP7
is complete; merge once. The rework continues the landing already on the branch; it is not a second
series. `just checkpoint` fails until then and must not be "fixed" by loosening a gate.

The order inside the landing is the work-package order, WP1 through WP7. Two things are pinned
inside it: `kb.Discover`'s requiredness gate is satisfiable only once WP5 has restamped every
fixture, and the version bump is WP5's alone — it is already landed as `2.0.0`, and no other package
moves it.

## Outcome

A KB at `2.0.0` carries three quantities, five node types and four scoring relations. A **valid**
`1.0.0` KB opens through one converter without hand edits; a file already malformed under `1.0.0` is
refused by the load (R2–R5) and is not the converter's to fix. No derived-value equality is promised:
lifts move station, `founds` scales, and untokened work edges stop pending.

## The model

**Three quantities.**

| Name | What it is | Home | Replaces |
|---|---|---|---|
| `rigor` | one node's local value, `[0,1]` or `*pending*` | claim, analysis, work, experiment | `confidence`, `quality`, the work's and the experiment's `strength` |
| `bearing` | one scoring edge's scalar, `[0,1]` or `*pending*` | every scoring edge | `applicability`, `fraction`, the pair `strength` |
| `strength` | the derived value, `[0,1]` or pending | claim, analysis | `solidity` everywhere |

**Five node types, one word each.** `claim`, `analysis`, `experiment`, `work`, `axiom`. The old
`invariant` and `axiom` kinds collapse to `axiom`; `framework` is prose for the document that
declares axioms, never a node type. An axiom has no rigor and its strength is `1.0` by definition.
A legend in the contract maps the prose words (`invariant`, `framework premise`, `cited work`, …) to
the five; the graph uses only the five. **Ids carry a fixed three-letter prefix**: `clm-`, `ana-`,
`exp-`, `wrk-`, and `axm-<label>` for an axiom, whose label is written as authored
(`[A-Za-z0-9][A-Za-z0-9._-]*`). The converter rewrites a `1.0.0` `sup-` id to `ana-`, every authored
work id from `work-<citation key>` to `wrk-<citation key>` — `WorkIDPattern`'s shape, not a
six-character body, and contexts included — and `INVARIANT-<label>`/`Axiom N` citations to
`Axiom <label>`.

**The register is the rigor vocabulary.** The register file is `claim-rigor.md` (`kb.RegisterFile`),
an entry's section heading is `### Rigor`, its Tier-2 marker is `<!-- claim-rigor: … -->`, and the
section's field list is `RigorFieldKeys`; the derived heading is `### Strength`. The tree still
carries the old names until WP7's first step renames them, and the converter renames the file,
rewrites both headings and the marker in the `1.0.0` pass.

**Four relations, one direction: `Source` is the supporter, `Target` the supported.**

| Relation | Source | Target | Bearing | Contribution |
|---|---|---|---|---|
| `supports` | claim, experiment, axiom | claim, analysis | required | `bearing × source strength` into the min (an axiom's strength is `1.0`) |
| `founds` | work | claim, analysis | required | `bearing × work rigor` into the min |
| `lifts` | analysis, experiment | claim | required | `bearing × lifter strength`, maxed above the min |
| `refutes` | claim, analysis, experiment | claim | required | cap `1 − bearing × refuter strength`; smallest cap wins |
| `references`, `demoted` | claim | claim | illegal | none |

`RelationDepends`, `RelationRestsOn` and `RelationStrengthens` retire. `RelationSupports` keeps its
name and not its old rows: a `1.0.0` support node's `supports:` pairs become `lifts`; old
`depends`→claim edges become `supports`. `BearingLegal(relation, sourceKind, targetKind)` answers
only *may this pairing carry a bearing*, not *may this edge exist*: every scoring pairing answers
yes, and only `references` and `demoted` answer no.

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
  composes innermost-first. It has four forms today — `[= min(rigor, minDep)]`,
  `[= max(…, lift)]`, `[= min(…, cap)]`, and the two nested — and a pending derivation with a lift
  renders `max(*pending*, 0.80)`. The design's fifth form, `[= lift <value>]` (§2.3: the derivation
  is pending and a lift stands), is not implemented; WP2 adds it, and after the rework the trace
  emits one of the design's five.
- Walk order follows value dependency, which is the push direction. The acyclicity path graph is its
  reverse: one rule, `g.Add(e.Target, e.Source)`, for every scoring relation.

**Authoring.** The register stays claim-centric. A claim's or analysis's `- rests-on:` bullets name
premises; the parse flips them: a `clm-` or `Axiom …` token yields `supports` **into** the entry, a
`wrk-` token yields `founds` **into** the entry. An experiment's and an analysis's own pair lists
(`supports:` on an experiment, `lifts:`, `refutes:`) are authored on the source and are already
push. A claim's `refutes:` list (`- <clm-id>: <bearing>`) is likewise authored on the source claim:
a refutation is a claim carrying a `refutes` edge, so a later refutation can target the refuting
claim. `rests-on` is the list key only; it is not a relation name in `2.0.0`.

**One scoring target per bullet.** Two or more scoring tokens — `clm-`, `wrk-` or `Axiom …` — on
one bullet is refused; there is no exempt token. A writer never emits one.

**Register shape, in this order for writers (readers are position-insensitive):**

```
### Rigor
- rigor: 0.85
- rationale: …
- rests-on:
  - clm-bbbbbb — Second Claim (strength 0.90) (bearing 0.4) [ctx note]
  - wrk-aaaaaa — A Cited Work (bearing 0.8) [Theorem 3]
  - Axiom S1 — The design rule (bearing 0.8) [a context]
- refutes:
  - clm-eeeeee: 0.7
- references:
- demoted:
- strengthen-by:

### Strength
- strength: 0.36 (use as input only) [= min(0.85, 0.36)]
```

A claim's or analysis's entry carries **two H3s**: `### Rigor` for the authored fields and
`### Strength` for the derived line and its trace; a work's entry carries `### Rigor` alone, since a
work has no derived value. The writer emits them, so a minted entry and a converted one have the
same shape; the converter rewrites `### Quality` to `### Rigor` and `### Solidity` to `### Strength`
and changes nothing else in the entry. The order above is the authored fields' own token sequence,
which is what `refresh`'s regexes and the writer see, and it is not a reordering of the entry's
headings.

`(strength …)` on a premise bullet records the **source's** derived strength (not the
contribution). `(bearing …)` judges the edge, and on a `founds` bullet it judges the finding named
in the trailing `[context]`, not the work as a whole. A work or axiom bullet has no
`(strength …)` — neither has a derived value — and an axiom bullet carries its bearing like any
other premise, with its context in `[context]`.

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
- No `invariant`, `framework` or `support` node type and no alias for `axiom`; the contract's legend
  maps the prose words, and no `target_kind`, index row or sheet stroking names any of the three.
  `support` survives only as the relation name `supports`; `sup-` is not an id prefix.
- No refutation authored on a work in `2.0.0`. A claim's `refutes:` list **is** the refutation
  primitive; a work's counter-evidence is carried by an analysis node the work founds.
- `strengthen-by`/`strengthen_by_count`, `confidence` in `asks.Reply`/`ItemRecord`/
  `CandidateEntry`, `DependentsOf`/`deps -i`, and the band names keep their words. They name a text
  list, a letter-confidence or a graph relation — not a retired field. `### Quality`,
  `claim-quality.md`, `<!-- claim-quality: … -->` and `QualityFieldKeys` are **not** survivors: WP7
  renames them with the rest of the register's vocabulary, and the converter renames them in the
  `1.0.0` pass.

## Work packages

### WP1 — `internal/kb`: the vocabulary, the parse, the gate

**Landed at `d2c49dc` — verify, do not redo.** `kb.Bearing{Value, Pending}` and `kb.Fraction`'s
retirement; `Edge`'s shape with `SourceStrengthRecorded`, `Bearing` and `Origin`, every scoring
collection `[]Edge` with `SupportPair`/`StrengthensPair` gone; `FanOutRecord`'s
`*Bearing` ends; `Relations` as the four plus `references`/`demoted`; `ListRestsOn` and `IndexEdges`;
`premises` with the parse's one flip; `ParseBearing`/`BearingText`/`BearingFromValue`/`BearingRE`;
`RenderBearingAnnotation`/`RenderStrengthAnnotation`; the `AnnotationError`/`AnnotationFault` sink in
`Discover`; R1–R5 over claim and work tokens; `seedIndexRow`'s refusal of a relationless row;
`NodeRow`'s `Rigor`/`DerivationStrength`/`Strength`/`PremiseCount` and `EdgeRow.Bearing`; the
`ParseStrengthLine`/`StrengthTrace`/`ParsePair`/`ParsePremisesBullet` surface WP2–WP5 consume.
What the tree does **not** have is the node pass: `NodeKindSupport` is `"support"`, `NodeKindInvariant`
is a kind, a support is `SupportNode`/`SupportEntry` under `sup-` and `support-nodes`, a claim has no
`refutes:` list, an axiom premise is exempt from R2, and the walk feeds it a hard-coded `1.0` from a
`framework` map rather than a parse-produced bearing.

**Scope.** `internal/kb/{schema,register,records,leaf,discover,indexstream,format,render,frontmatter}.go`
and their tests.

**Build.**

- **Node words.** `NodeKindSupport` becomes `NodeKindAnalysis` with value `"analysis"`; `NodeKinds`
  is five words — `support` renamed in place and `invariant` dropped, so `axiom` and `work` each
  move up one slot: `claim`, `analysis`, `experiment`, `axiom`, `work`. That ordering shift reaches
  the index's kind sort and the sheet's legend order; the id prefix `sup-` becomes `ana-`.
  **The minted-id tables move with it**: `IDKinds` (`schema.go:57`, `{"clm","exp","sup"}`) becomes
  `{"clm","exp","ana"}` and `IDPlaceholders` (`:59`, `sup-xxxxxx`) becomes `ana-xxxxxx`; otherwise
  `IDBody("ana")` panics and `IDBody()` keeps accepting `sup-`.
  `kb.WorkPrefix`'s value becomes `"wrk"` (the constant keeps its name), so `WorkIDPattern` and
  every id check built from it read `wrk-<citation key>`; the parser accepts `wrk-` and no
  `work-`, which the converter has already rewritten by the time it sees the bytes.
  `NodeKindInvariant` and `FrameworkKinds` retire. The support family renames whole: `SupportNode`→
  `AnalysisNode`, `SupportEntry`→`AnalysisEntry` (the register-resident half), `ParseSupportLeaf`→
  `ParseAnalysisLeaf`, `ParseSupportEntries`/`parseSupportEntries`→`ParseAnalysisEntries`/
  `parseAnalysisEntries`, `SupportNodesKey`→`AnalysisNodesKey` (`"analysis-nodes"`), `SupIDKey`→
  `AnaIDKey` (`"ana-id"`), `State.Supports`→`State.Analyses`, `FanOutRecord.SupID`→`AnaID`, the
  `supIDFullRE`/`IDBody("sup")` pair→`anaIDFullRE`/`IDBody("ana")`, and `checkSupportNodes`/
  `checkSupportID`/`renderSupportNodes`/`supportFieldKeys` follow, `parseSupportEntries`' doc
  comment included. The word `support` survives only in the relation `supports`, in prose about it,
  and in WP7's named survivors.
- **Axiom declaration.** `FrameworkNode`→`AxiomNode`; `ParseFrameworkNodes`→`ParseAxioms`. The
  heading pattern changes with the file it reads, so the reader tries three sources in order.
  `axioms.md` (`kb.AxiomsFile`, new) is read by the one `axiomHeading` (`^### Axiom (<label>):\s*(.+)$`),
  and a declaration's id is `axm-<label>`. `invariants.md` (legacy) is read by `invariantHeading`
  (`^### INVARIANT-<label>:`) and `axiomBulletRE` (`- Axiom N: **…**`) — the patterns `axiomHeading`
  cannot match — yielding `axm-<label>` and `axm-<N>` respectively; `AGENTS.md` (legacy,
  `kb.AgentsFile`) is read by the same two legacy patterns with the same ids.
  `kb.InvariantsFile` stays as the legacy constant; `kb.AxiomsFile` is added beside it and joins
  `ExcludeNames` (`kb.go:81`) beside `InvariantsFile`, so neither is read as a document. The lookup
  that walks the chain replaces `FrameworkSource`, and `kb.axiomRE` (`Axiom <label>`) replaces
  `invariantRE`/`axiomRE` as the register's citation token; `invariantRE`'s edge path (source
  `INVARIANT-<label>`) goes. An axiom's id is `axm-<label>` with the label as written under the
  design's alphabet (`[A-Za-z0-9][A-Za-z0-9._-]*`) — `Axiom S1` is `axm-S1`, and the legacy
  `Axiom 1` is `axm-1`; a legacy `### INVARIANT-S1:` heading drops the `INVARIANT-` prefix and keeps
  only the label, `S1`. Its `TargetKind` is `axiom`. Two builders carry it: the citation path
  at `register.go:545` and `ParseFrameworkNodes`' node id at `leaf.go:362`, both `"axm-" + label`. An
  axiom id is not a minted id: it carries the label, so `IDBody`'s `<kind>-[a-z0-9]{6}` does not
  govern it, and any id check that admits an axiom id is spelled for the label.
- **The axiom's anchor.** `ParseFrameworkNodes` anchors a legacy `- Axiom N:` bullet at
  `INVARIANT-S2`'s heading (`leaf.go:350`, `:362`); that special case retires. A declaration under
  `axioms.md` anchors at its own heading: `Slugify` of the heading text after `### `, so
  `### Axiom S1: The design rule` anchors at `axiom-s1-the-design-rule` — the heading's slug, not
  the node id `axm-S1` — and `kb.AnchorSection(text, anchor)` returns its body. A legacy bullet read
  through arm 2 or 3 keeps the old `INVARIANT-S2` anchor until the converter rewrites the file.
- **The legacy-declaration scan.** `LegacyAxiomDeclarations(src *Source) []LegacyDeclaration` is
  new: it scans the legacy source (`AGENTS.md`) with the two legacy patterns this package keeps
  behind the fallback and returns each declaration's path, 1-based line and the token as written
  (`### INVARIANT-S1:` → `INVARIANT-S1`; `- Axiom 1: **…**` → `Axiom 1`). It reads only; it fixes
  nothing, and it returns nothing where the KB carries no `AGENTS.md`; a read failure also yields
  nothing, since the scan reports no error and WP2's finding pass takes none. It stays because it
  has a consumer: WP2's `axiom-source` finding, which the design requires (§2.8).
- **`BearingLegal` gains the axiom source and the claim refuter.** `supports` from claim, experiment
  or axiom; `founds` from work; `lifts` from analysis or experiment; `refutes` from claim, analysis
  or experiment. It stays one predicate and still answers only *may this pairing carry a bearing*.
- **Parse: claim refutations, no exempt bullet.** `ClaimEntry.Refutes` is new and a claim's
  `refutes:` pair list is read like an analysis's `lifts:` (push, source = the claim), with `refutes`
  added to `QualityFieldKeys` so the list folds correctly. An `Axiom …` token yields a
  bearing-carrying `supports` edge into the entry like a `clm-` token, so R2 reaches an axiom bullet
  and no bullet is exempt. Pair-line readers emit `Edge`s and require a readable value; a pair with
  no value is malformed, not pending. A claim entry has no error return, so a malformed `refutes:`
  pair is reported through the annotation-fault sink under the `refutes` key at the pair's line,
  while an analysis's or an experiment's returns a malformed entry as it does today; the load
  refuses either.
- **Render.** `(strength …)` first, `(bearing …)` second, `[context]` last; an axiom bullet has no
  `(strength …)` and carries its `(bearing …)`; its context is `[context]`, so `parenContext` is
  converter-only and retires with the old bullet shape.
- **`Discover` and the index stream** keep their landed rule and follow the renames: a decoded
  analysis/experiment/work node and a non-claim edge target survive `ReadIndex` unchanged, and the
  relationless row is still refused.
- The version stamp is not this package's: it is already `2.0.0` from WP5, and the rework does not
  move it.

**Facts the step rests on — re-run each before editing.**

- `rg -n 'NodeKindSupport|NodeKindInvariant|FrameworkKinds' internal/kb/schema.go` → `:174`, `:176`,
  `:182`, `:214`, `:236-244`: the kind values, the kind list, the bedrock pair and the
  `BearingLegal` arms this package rewrites.
- `rg -n 'support-nodes|sup-id|SupportNode|SupportEntry|ParseSupport' internal/kb -g '!*_test.go'`
  → the leaf/frontmatter/register/discover family the first bullet renames; `frontmatter.go:284` is
  the key table's support half.
- `grep -n 'invariantHeading\|axiomBulletRE\|invariantRE\|axiomRE' internal/kb/*.go` → `leaf.go:26`,
  `:27`; `register.go:59`, `:60`. `grep -n 'type FrameworkNode' -A4 internal/kb/leaf.go` →
  `leaf.go:81`; `grep -n 'func FrameworkSource\|func ParseFrameworkNodes' internal/kb/leaf.go` →
  `:327`, `:338`.
- `grep -n 'type ExperimentNode' -A5 internal/kb/leaf.go` → `internal/kb/leaf.go:62-66`: `ID, Title,
  CanonicalPath, CanonicalAnchor, Status`, `Rigor *float64` and `Supports, Lifts, Refutes []Edge`.
  There is no register entry; its `rigor` lives in the host leaf's `experiment-nodes` mapping.
- `rg -n 'QualityFieldKeys' internal/kb/register.go` → `:24`, the claim entry's field list that gains
  `refutes`; `supportFieldKeys` at `:28` is the analysis entry's and keeps `lifts`/`refutes`.
- `grep -n 'func BearingLegal' internal/kb/schema.go` → `:235`: its `supports` arm excludes an axiom
  source today and its `lifts`/`refutes` arm excludes a claim refuter.
- `grep -n 'WorkPrefix = \|WorkIDPattern' internal/kb/schema.go` → `:144`, `:148`: the id prefix and
  the pattern built from it.
- `rg -n 'IDKinds|IDPlaceholders' internal/kb/schema.go` → `:57`, `:59`: the minted kinds and the
  authored placeholders, both of which keep `sup` until this package moves them.

**Acceptance.** A `2.0.0` register with each of R1–R5 is refused by `check: metadata` at the
bullet's line, including an axiom bullet with no bearing (R2) and a claim's `refutes:` pair with an
unreadable value (key `refutes`, the pair's line); an axiom-only bullet with a bearing
and a two-id `references` bullet load; a claim+axiom bullet is refused as R1; a bullet naming one
stale id loads with a logged drop. `0`, `1` and pending round-trip through `ParseBearing`/
`BearingText`. `NodeKinds` teaches five types; an `INVARIANT-*` token is not a type. A claim's
`refutes:` list parses to one `refutes` edge per line with the claim as source; an `ana-` id parses
and a `sup-` id is not a node; a `wrk-` id parses as a work and a `work-` id does not; `axm-S1` and
`axm-1` parse as axioms; `BearingLegal` answers yes for an axiom source on `supports` and a claim
source on `refutes`, and no for a work source on either.

### WP2 — `internal/index`: the walk and the derived artifacts

**Landed at `7e0cfee` — verify, do not redo.** `solidity.go` renamed to `strength.go` with
`ComputeStrength`, `MinDependencyStrength`, `FormatStrength`, `RenderStrengthLine/Annotation/Trace`,
`StrengthBelow`, `RoundHalfUp2` and the five-form trace; `bearingValue` and `premise_count` in
`records.go` with the `edges.yaml` rows; `refresh.go`'s renamed surfaces and the `errors.As` refusal
branch;
`verify.go`'s `strength freshness`, bearing backstop and `target_kind` check; `dependspath.go`'s one
reverse rule; `compat_test.go`'s renamed case table (its mutations and probes still spell the support
node — the rework below follows them). The walk therefore exists — under the old node
words and with an axiom special case the design does not have.

**Scope.** `internal/index/{strength,records,refresh,verify,dependspath,citations,compat_test}.go`.

**Build.**

- **The analysis half of the walk takes its new name.** `Strength.SupResults`→
  `Strength.AnalysisResults`, and `SupFinals()`→`AnalysisFinals()` (design §2.9 names both), with the
  `sups`/`supOrder`/`supResults` locals following; the analysis arm reads `st.Analyses` and keys on
  `ana-` ids. The walk's own arithmetic is unchanged.
- **The axiom special case goes**, and the three signatures lose the framework map:
  - `func MinDependencyStrength(premises []kb.Edge, strength map[string]*float64) *float64`
    (was `(premises, strength, framework map[string]bool)`; the `framework[e.Source] → 1.0` branch
    at `:173` goes);
  - `func derive(rigor *float64, premises, lifts, caps []kb.Edge, strength map[string]*float64) Result`
    (was `(…, strength, framework)`);
  - `ComputeStrength(st kb.State) (Strength, error)` — signature unchanged; its `framework` map
    (`:350-352`) and the `derive` argument (`:433`) go.

  An axiom's `1.0` is put in the `strength` map beside a work's rigor and an experiment's rigor
  (`:408` already does this), so an axiom `supports` edge contributes `bearing × 1.0` from the
  parse's own bearing like every other scoring edge. Nothing in this package may reinstate a
  bearingless operand.
- **The fifth trace form.** `RenderStrengthTrace` (`strength.go:136-160`) composes the four forms the
  model section lists; add the design's fifth, `[= lift <value>]`, for the pending derivation with a
  lift standing, and stop rendering `max(*pending*, …)` where the rigor and the min-dep are both
  absent.
- **The citation vocabulary.** `citations.go` owns the words the citation gate sanctions, and the
  declaration word is the axiom's now: `declarationRE` (`:28`, `^###\s+INVARIANT-[A-Z]+[0-9]+\s*:`)
  recognises a `### Axiom <label>: Title` declaration in `axioms.md` as the sanctioned channel, and
  the flagged set gains `Axiom <label>` — a bare `Axiom …` token outside its declaration is a
  citation fault, as `INVARIANT-<label>` already is. `invariantIDRE` (`:27`) / `plainInvariantRE`
  (`:26`) keep flagging an `INVARIANT-<label>` or `Invariant N` token anywhere else — the converter
  rewrites every citation, so a hit in a converted KB is stale authoring or an unconverted file. The
  framework-source section (`:94`, `:302-338`) resolves through the axiom-source lookup and treats
  `axioms.md` as the one declaration document. **What changes with `ExcludeNames`**: `Documents`
  skips `axioms.md`, so no leaf or axiom parse runs over it, but the citation pass already reaches
  it through `kb.MarkdownFiles`; `kb.AxiomSource` tells it which file is the declaration document,
  and that file's `### Axiom <label>:` headings blank as the sanctioned channel. The legacy
  declaration documents (`invariants.md`, `AGENTS.md`) stay sanctioned channels whatever
  `AxiomSource` returns, so a leftover declaration is reported by `axiom-source` alone and never by
  the citation gate.
- **The vocabulary the operator sees.** `verify.go:745`'s "support node id is not …" violation and
  `citations.go:144`'s "bare claim/experiment/support id in prose" remedy text are operator-facing
  strings: both name `analysis`/`ana-` after the rework.
- **`records.go`.** The analysis node row is built from `st.Analyses`; `bearingValue` passes
  `kb.BearingLegal(relation, sourceKind, targetKind)` and is unchanged otherwise. Every id the
  walk and the rows carry is the `2.0.0` id: `clm-`, `ana-`, `exp-`, `wrk-` and `axm-<label>`; the
  `strength` map is keyed by them, so a `founds` edge's source is a `wrk-` id and an axiom premise's
  an `axm-` one.
- `refresh.go`'s `supResults` local and `verify.go`'s `supFinals` and analysis trace reference follow
  the rename.
- **The legacy-axiom finding.** `LegacyAxiomFindings(src) ([]Finding, error)` is a new pass in
  `verify.go` beside `DemotedFindings`: it maps `kb.LegacyAxiomDeclarations(src)` (WP1's scan) to one
  `Finding{Check: "axiom-source", Path, Line, Detail: the token}` per declaration, with
  `RefreshFixable: false`. The design requires it (§2.8: a leftover declaration `verify` reports as
  `check: axiom-source`, informational, not refresh-fixable, never a load refusal). This package
  also appends it in `cmd/verify.go` beside `DemotedFindings`, the one line outside `internal/index`
  it owns. It fires only
  when the KB carries both `axioms.md` and an `AGENTS.md` with a declaration; a `1.0.0` KB (no
  `axioms.md`) yields none. `cmd`'s `verify` appends it to the `findings` list, never to the
  `refusals` list — `Verify`'s findings render as refusals, which is why the pass is separate.
- `compat_test.go`: the case table still spells `support-nodes`/`sup-` (`:313`, `:373-380`). Rename
  its mutations and probes to `ana-`/`analysis-nodes`/`analysis`; `pendingScore` and `pendingArc`
  (`:176-177`) already read the `rigor` line and `(bearing …)`. The comparisons stay flag-gated: this
  package makes the harness coherent, not live.

**Facts the step rests on.**

- `grep -n 'SupResults\|SupFinals\|supResults' internal/index/*.go | grep -v _test` →
  `strength.go:27`, `:41-42`, `:44`, `:424`, `:438`; `records.go:84`; `refresh.go:235`, `:266`;
  `verify.go:821`, `:837`.
- `grep -n 'framework' internal/index/strength.go` → `:167`, `:173`, `:235`, `:350-352`, `:408`,
  `:433`: the parameter and the map this package removes.
- `grep -n 'func MinDependencyStrength\|func ComputeStrength\|func buildClaims' internal/index/*.go`
  → `strength.go:167`, `strength.go:333`, `records.go:60`.
- `grep -n 'pendingScore\|pendingArc' internal/index/compat_test.go` → `:176-177`;
  `grep -n 'support-nodes' internal/index/compat_test.go` → `:313`; the `supLeaf`/`sup-` cases run
  `:373-380`.
- `grep -n 'func RenderStrengthTrace' internal/index/strength.go` → `:136`, the four-form composer
  this package extends with the fifth.
- `grep -n 'plainInvariantRE\|invariantIDRE\|declarationRE' internal/index/citations.go` → `:26-28`;
  the framework-source section runs `:94` and `:302-338`.

**Acceptance.** A lift above the derivation raises the final; one below leaves it; a binding cap
takes the minimum; a halved binding bearing moves the dependent to the exact product; a `0` premise
drives the claim to `0.00` while the same fixture without the edge does not; an axiom premise
contributes `bearing × 1.0`, so a `0.4` bearing caps its contribution at `0.4` and a `0.0` bearing
drives it to zero; a pending premise leaves the claim at its rigor. `Strength.AnalysisResults`
carries one entry per `ana-` id and `Strength.Results` one per `clm-` id. A refused neighbour does
not queue: pending never propagates. A pending rigor with a live lift renders `[= lift <value>]`,
and one with no lift and no cap renders nothing. A KB carrying `axioms.md` and an `AGENTS.md` with a
legacy declaration returns one `axiom-source` finding per declaration, `verify` stays `done`, and the
item carries no `remedy`; a `1.0.0` KB with the same `AGENTS.md` and no `axioms.md` returns none.

### WP3 — query, sheet, and the query verbs

**Landed at `7e0cfee` — verify, do not redo.** `DepsPayload`'s incidence adapter with `relation`,
`direction`, `target`, `bearing`, `context` in the fixed relation order; `DependentsOf` reading the
push direction; `stats`' `strength_bands`/`weakest[].strength`/`highest_leverage[].strength`/`edges`
and `show`'s `premise_count`; `query.Load`'s `requiredFiles` (`load.go:17`, already `edges` and not
`depends-on`), its bearing refusal and its one migrated-source refusal; the sheet's strength fill,
four strokes, push arrows and transitive reduction; `strength-below`, `--max-strength` and
`DefaultMaxStrength`. What remains is the node word: `nodeFields`, `load.go`'s build-band branch and
the sheet's node label still say `support`.

**Scope.** `internal/query/{payload,query,load}.go`, `internal/sheet/{graph,dot}.go`,
`cmd/{deps,show,stats,weak-points,strength-below}.go`.

**Build.**

- `nodeFields`: claims carry `rigor`, `strength`, `premise_count`; analyses `rigor`, `strength` (and
  their `build_band`, as today — an analysis's `rests-on` list gets no count or listing here: the
  design's `show` record for a non-claim node is the derived pair, and the authored premises are
  `deps`' `in` rows, so a second surface would have no client); works `rigor`; experiments `status`,
  `rigor`; axioms are identity only — no local value. The record's kind key is `analysis` and the
  analysis arm is keyed on `kb.NodeKindAnalysis`; `internal/query/load.go:56` and
  `internal/sheet/graph.go:142` test `NodeKindSupport` today and follow it. The kind-count key and
  the digest's count noun take the kind's English plural — `analysis` counts as `analyses` — in
  `stats` and in the sheet, not `kind+"s"`.
- `DepsPayload` returns every incident **scoring** edge (both directions) plus the node's own
  outgoing `references`/`demoted`, as `relation`, `direction` (`in`/`out`), `target` (the other
  end), `bearing`, `context`; fixed relation order `supports`, `founds`, `lifts`, `refutes`,
  `references`, `demoted`, then the other end's id. `null` means only *this row cannot carry a
  bearing*. The inversion is one adapter here.
- `DependentsOf` and `deps -i` keep their names. A dependent is a node `id` supports, lifts or
  refutes, so the answer is the targets of `id`'s outgoing scoring edges — under the push
  direction, not the sources of edges into `id` (those are `id`'s premises, which `deps` reports as
  `in` rows).
- Sheet: the node legend follows the five kinds — the `NodeKindAnalysis` row replaces the support
  row with the label `analysis` (`internal/sheet/dot.go:43`), and the `NodeKindInvariant` row
  retires with the kind (`:45`); band slugs, thresholds, strokes and the cut legend are unchanged.
- The user-visible prose in this package names the node word `analysis` — the lift and refute stroke
  meanings (`internal/sheet/dot.go:121`, `:123`), the band comment at `graph.go:47` and the
  dependents doc at `query.go:48-51`.

**Facts the step rests on.**

- `grep -n 'func DepsPayload' internal/query/payload.go` → `internal/query/payload.go:117`.
- `rg -n 'NodeKindSupport|NodeKindInvariant' internal/query/*.go internal/sheet/*.go` →
  `load.go:56`, `payload.go:81`, `dot.go:43`, `dot.go:45`, `graph.go:142`: the five sites the node
  words move.
- `grep -n 'func (ix \*Index) DependentsOf\|func (ix \*Index) StrengthBelow'
  internal/query/query.go` → `:52`, `:186`.
- `grep -n 'strength-below' justfile` → `justfile:1120`, the results-fixture leg.

**Acceptance.** `deps <claim>` reports its premises as `in` rows; an analysis's lifts as `out`
rows; an axiom premise's bearing is a number or the literal; a pending bearing is the literal and a
`0` is `0`; the order does not move when the KB's edges are re-sorted. A `founds` row names the
`wrk-` work as its `target`, and an axiom-sourced `supports` row names `axm-<label>`. `show` returns
a node of kind `analysis` with `rigor` and `strength`, and no `support` kind anywhere in a result
document.

### WP4 — `internal/write`: the ops and the read-back

**Landed at `7e0cfee` — verify, do not redo.** `set-bearing` with `authoredHome(edge)` and its
replace-only rule; `set-rigor` reaching claim, analysis, work and `exp-` ids (through `recKind` in
`ops.go`, with `checkRigorID` accepting all four); `add-rests-on` and the inserts' `rests-on` lists;
`insert-claim-entry`,
`insert-experiment-entry` and `insert-work-entry`'s `rigor`; `renderRestsOnBullet` and
`renderDemotedBullet`; `checkEdgeClass` on `kb.ListRestsOn`; the read-back through `expectedEdge`,
`edgeOf` and `edgesEqual`; the annotation gate in `write.Run`; `claimgraph`'s pending-only bearings.
What remains is the node word and the claim `refutes:` list: the id checkers still admit `sup` and
not `ana`, `insert-support-entry` is the analysis insert, and no op writes a claim's refutations.

**Scope.** `internal/write/{ops,values,render,store}.go`, `cmd/{insert-analysis-entry,
insert-claim-entry,insert-experiment-entry,insert-work-entry,drop-*,add-rests-on,set-bearing,
set-rigor,set-frontmatter}.go`, and `internal/claimgraph`'s writers. `internal/build/inference_test.go`
is not this package's: its captured edges are WP5's fixture sweep.

**Build.**

- Absorb WP1's reader signatures: `kb.ParseAnalysisEntries` returns
  `(order, map[string]kb.AnalysisEntry, error)`, and `kb.ParsePair` returns `(Edge, bool, error)`. A
  malformed pair value is the op's refusal, not a skip.
- **The id checkers.** `checkEntryID` and `checkTargetID` admit `clm` and `ana` (not `sup`);
  `checkSourceID` (`values.go:300-305`) admits `clm`, `ana` and `exp`, and gains an `axm-<label>`
  branch on the design's alphabet (`[A-Za-z0-9][A-Za-z0-9._-]*`), built on `checkWorkID`'s model
  beside its `WorkPrefix` branch — without it `set-bearing` refuses an axiom-sourced `supports`
  edge that the acceptance requires to land. `insert-support-entry` becomes
  `insert-analysis-entry` (file, op name, help text, `mcpBinding` registration), and every op that
  mints or accepts a `sup-` id follows.
- **The entry's two headings.** `renderEntry` (`write/render.go:196`, its field list at `:198-206`)
  emits `### Quality` today; it emits `### Rigor` for the authored fields and `### Strength` for the
  derived line (design §2.5), so a minted entry and a converted one have the same shape.
  `renderWorkEntry` (`:244`, heading at `:249`) emits `### Rigor` and no `### Strength`, since a work
  has no derived value. The reader flips in the same package: `LocateRegisterEntries` and
  `qualityStart` match `### Rigor`, and every field reader stops at an H3 — `collectSubBullets`, the
  rationale fold, the strengthen-by collector and the refutes pair reader — or a `refutes:` pair
  absorbs the `### Strength` heading and the entry the writer mints is refused. Every writer test
  and render golden that pins a heading moves with them.
- **`checkPremiseTarget`** (`values.go:354-366`) is the add-rests-on and insert-* premise checker and
  today accepts a framework token in its old spelling only (`invariantRE` `^INVARIANT-[A-Z]+[0-9]+$`
  and `axiomTokenRE` `^Axiom [0-9]+$`, `values.go:27-28`). It accepts an `Axiom <label>` token —
  the register's citation form — as the framework target, refuses `INVARIANT-<label>`, and its
  refusal text names the axiom. Without this `add-rests-on` keeps refusing `Axiom S1`.
- **`set-bearing`.** Unchanged in shape and rule; its arguments are `id` (the source), `target`,
  `relation`, `bearing` (`values.go:630-635`). Its `authoredHome(edge)` resolution gains the
  claim-sourced refute's third home — the source claim's `refutes:` pair list — beside the premise
  bullet and the leaf/register pair line named today.
- **`insert-claim-entry`** takes a `refutes` pair list (`[{id, bearing}]`, bearing defaulting to
  `*pending*`). `renderRestsOnBullet` renders premise bullets only, so the list gets its own
  renderer, `renderRefutesList`, emitting `- refutes:` and one `- <clm-id>: <bearing>` line per edge
  after the `rests-on` list and before `references`. `drop-claim-node` and `drop-inbound-edges` widen
  to it: a `refutes` edge out of the dropped claim is authored on that claim's own list, and an op
  that reads only `rests-on`, lift and leaf pair lists strands it.
- `drop-inbound-edges` and `drop-claim-node` keep seeing a `supports`/`lifts`/`refutes` edge
  authored on an **analysis** or experiment leaf.
- `set-frontmatter`'s `experiment-node` table carries `rigor`, `supports`, `lifts`, `refutes`; its
  `analysis-node` table (today `support-node`, `ops.go:2102`, `:2275`, `:2365`) carries `ana-id`
  and `lifts`/`refutes` — not `supports`, and not `rigor`, which is the analysis's register entry's
  local value (§2.5).
- The build still writes no judgement: `claimgraph` sends `rigor`, `add-rests-on`, and
  `(bearing *pending*)` on every edge it adds — claim premises, since the build mints no axiom
  bullet — and nothing in `claimgraph` writes a bearing number. Its own inline register corpora and
  build-edge values are restamped here; `internal/build/inference_test.go`'s captured edges are
  WP5's. The stage finding is `pass-3-add-rests-on`.
- The read-back's expectation normalization (`normalized`, `store.go:578`) spells an axiom token's
  edge id `"axm-" + label`, so a re-issued op's expectation matches the parse's `axm-` source.
- Every `sup-` id, `supportDecl`/`supportNode` renderer and `support-node` table name in this
  package follows WP1's rename; no op accepts a `sup-` id after it. The help text and refusals an
  operator reads name `analysis`: `insert-analysis-entry`, `set-rationale`, `set-rigor`,
  `set-bearing` and `drop-inbound-edges` (`cmd/*.go`).

**Facts the step rests on.**

- `grep -n 'func planSetRigor\|func planSetBearing\|func planDropInboundEdges\|func planDropClaimNode\|func planResolveDemoted'
  internal/write/ops.go` → `:823`, `:892`, `:1124`, `:1414`, `:1909`; `planResolveDemoted` is the
  restore site whose read-back must say `Pending: true`.
- `grep -n 'func edgeOf' -A7 internal/write/store.go` → `:106-112`; it already copies the `kb.Bearing`
  whole, which is the rule the rest of the package follows.
- `grep -n 'func checkEdgeClass' internal/write/values.go` → `:256`; it already keys on `kb.ListRestsOn`.
- `grep -n 'func renderRestsOnBullet\|func renderDemotedBullet' internal/write/render.go` → `:117`,
  `:144`.
- `grep -n 'checkEntryID\|checkTargetID\|checkSourceID' internal/write/values.go` → `:294`, `:296`,
  `:300-306`: the three id checks that still name `sup`.
- `grep -n 'support-node' internal/write/ops.go` → `:2102`, `:2275`, `:2365`; `ls cmd/insert-*.go`
  → `insert-claim-entry.go`, `insert-experiment-entry.go`, `insert-support-entry.go`,
  `insert-work-entry.go`.
- `grep -n 'func checkPremiseTarget' -A14 internal/write/values.go` → `:354-366`; its two spellings
  are `invariantRE`/`axiomTokenRE` at `:27-28`.
- `rg -n '"axiom-' internal/write/store.go` → `:578`, the expectation normalization's id builder.
- `rg -n 'func renderEntry|func renderWorkEntry|### Quality' internal/write/render.go` → `:196`,
  `:244`, `:203` and `:249`: the two entry renderers and the headings they pin.

**Acceptance.** `set-bearing` lands on each of the four relations, including a claim-sourced
`refutes` pair, and re-reads the same state — `{id: "wrk-smith2020", target: "clm-…", relation:
"founds"}` and `{id: "axm-S1", target: "clm-…", relation: "supports"}` among them; `*pending*`
re-reads as pending and `0` as `0`; a missing token is a load fault before the op runs; an illegal
pairing, an unknown relation and an out-of-domain value are refused. `set-rigor` writes claim,
analysis, work and experiment homes and accepts a `wrk-` id; no op accepts a `sup-` id or a `work-`
id. `insert-analysis-entry` mints an `ana-` id and writes `analysis-nodes`. `insert-claim-entry` with
a `refutes` list writes one pair line per edge, `*pending*` by default.
`add-rests-on {id: "Axiom S1", bearing: 0.8}` succeeds and writes an axiom premise bullet, while
`{id: "INVARIANT-S1"}` is refused by name. A minted claim entry and a minted work entry carry
`### Rigor`, and the claim's derived line sits under `### Strength`; no writer emits `### Quality`.
A new premise is `*pending*` and moves no claim. A build leaves every edge it writes pending and no
claim's value moves.

### WP5 — version, converter, announcement, fixtures

**Landed at `81300cf` — verify, do not redo.** `internal/migrate/from_1_0_0.go` with the
`1.0.0 → 2.0.0` step (`convert100To200`, registered in `migrate.go`) and its own narrow reader;
the score-key, list-key and relation renames; the bearing insertion (`(applicability X)` →
`(bearing X)`, `(bearing 1.0)` on an untokened claim or work bullet) and the multi-target split;
`FormatVersion = "2.0.0"`; `Source.Migration()`/`MigratedSource`/`Saved`; `cmd`'s
`migrationNotice`; `query.Load`'s migrated-source refusal; the `register-1.0.0`/`register-2.0.0`
and `bad-token` golden pairs; `cmd/resultkeys_test.go`'s `migration` strip. The converter cannot
fail: a file it does not locate it does not touch, and a file already malformed under `1.0.0` is
carried as bytes for the `2.0.0` load to refuse by fault name (R2–R5) — the refusal is the load's,
not a migration failure, and the converter never makes an invalid `1.0.0` file valid.

What the converter does **not** do yet: it writes no `axioms.md`, rewrites no
`INVARIANT-<label>`/`Axiom N` citation, moves no framework bullet's parenthetical into `[context]`,
emits `sup-` ids, the `support-nodes` key and an analysis's pair list under `supports:`, leaves
every `work-` id as it found it, and renames neither the register file, its headings nor its
Tier-2 marker. The register **file**, its Tier-2 marker and the field-key names are WP7's first
step. The entry **headings** are this package's: the reworked reader refuses `### Quality`, so a
converted entry must carry `### Rigor` and `### Strength` before its golden can load.

**Scope.** `internal/migrate/{migrate,from_1_0_0}.go` and testdata, `internal/kbload/kbload.go`,
`internal/kb/source.go`, `cmd/main.go`, all fixtures.

**Build.**

- **The entry headings.** Every register entry's `### Quality` becomes `### Rigor` and its
  `### Solidity` becomes `### Strength`; heading lines only, the fields beneath them unchanged.
  Without the first, the reworked load refuses a converted entry; the second is the design's
  converter row and keeps the pair on one shape.
- **The id prefixes the converter writes.** Two rewrites, both over the authored bytes: a work id
  `work-<citation key>` → `wrk-<citation key>`, matched by `WorkIDPattern`'s shape rather than a
  fixed body, everywhere a work id appears — register bullets, `references`/`demoted` bullets,
  `strengthens`/`lifts` pair keys, leaf `claims:` lists and any `[context]` that names one — and a
  `1.0.0` support node's identity in all three places it is stored: `support-nodes:` →
  `analysis-nodes:`, `- sup-id: sup-…` → `- ana-id: ana-…`, and a register's
  `<!-- id: sup-… -->` marker → `<!-- id: ana-… -->`. The converter already reads the old spelling —
  the `sup` row of the rename table (`:18`), the marker regex (`:26`), `frontmatter100To200`'s
  section test (`:112`), `convertNode`'s key branch (`:150`) — but today it rewrites only the
  section's keys and leaves the old bytes in place. The emitted bytes carry only `ana-`,
  `analysis-nodes` and `wrk-`, and `kind` stays read from the input id so the field-rename table is
  unchanged. `axm-` needs no rewrite: a `1.0.0` axiom declaration has no authored id to carry.
- **The axiom vocabulary — not started.** Read `invariants.md` (else legacy `AGENTS.md`), write
  `axioms.md` with one `### Axiom <label>: Title` heading per `### INVARIANT-<label>:` section and
  per `- Axiom N: **…**` bullet, moving an axiom bullet's body beneath its heading; rewrite every
  `INVARIANT-<label>`/`Axiom N` citation in register bullets to `Axiom <label>` and give that bullet
  the `(bearing …)` an axiom premise now requires — inserted before the trailing `[context]`, so
  `Axiom S3 (bearing 1.0) [a labelling convention]`; move a framework bullet's trailing
  parenthetical into `[context]`. Labels are preserved, nothing renumbered, and the id the load
  derives is `axm-<label>` (`axm-S3`, `axm-1`). `invariants.md` is declared obsolete and removed
  with the rest. A legacy `AGENTS.md` that also carries instructions is left in place: its
  declarations move, its axioms become inert, and `verify` reports each remaining declaration as
  `check: axiom-source` with its path and line (design §2.8) — informational, not refresh-fixable,
  and never a load refusal.
- No direction flip in bytes. The flip is the parse's; the index is regenerated.
- Fixtures:
  - The golden pair `internal/migrate/testdata/register-1.0.0` → `register-2.0.0` already carries a
    claim, analysis and work score line; an indented score line; a stray wrong-kind key; a claim
    bullet with strength and context; a work bullet with and without an applicability; a framework
    bullet (`- INVARIANT-S3 (a labelling convention)`); a two-claim bullet; a claim+work bullet; an
    analysis premise list; an analysis's `supports:` pairs; an experiment's `strengthens:` pairs; and
    a title containing `bearing 0.5`. Its input gains an `invariants.md` carrying both an
    `### INVARIANT-<label>:` section and an `- Axiom N: **…**` bullet, whose citations the converted
    register uses; its expected side gains `axioms.md`, and is restamped to
    `ana-aaaaaa`/`ana-bbbbbb`, `analysis-nodes`, `ana-id`, `lifts:`, `wrk-smith2020` for every
    `work-smith2020` the input carries, and the citation
    `Axiom S3 (bearing 1.0) [a labelling convention]` — the bearing before the trailing context, as
    the writer and the insertion rule put it. The input stays `1.0.0` bytes. The register file's own
    name (`claim-quality.md` → `claim-rigor.md`) is WP7's row, so the pair is named by its old file
    here and renamed in that pass.
  - The byte-only case `bad-token-1.0.0` → `bad-token-2.0.0` carries the malformed
    `(applicability sometimes)`; it converts to `(bearing sometimes)` and is asserted to be refused
    by `kb.Discover` as R3. It is not part of the loading golden pair: a goldens file that cannot
    load would make the pair's load assertion say nothing.
  - `internal/migrate/testdata/1.0.0`, `internal/migrate/testdata/0.9.0` and
    `internal/kbload/testdata/{0.9.0,loose-0.9.0}` stay byte-unchanged; the load path runs two
    steps.
  - Restamp: `test_data/fixtures/init/skeleton` (stamp only), `test_data/fixtures/results`,
    `internal/sheet/testdata/adjagent-sheet/kb-root` (stamp; the two axiom-sourced `supports` rows
    in `.index/edges.yaml` take source `axm-S1`/`axm-1` and `"bearing": "1.0"` where they carry
    `null`; `sup-aaa007` becomes `ana-aaa007` with `node_type: analysis`; the `invariant` node
    becomes the axiom node for `Axiom S1` (`axm-S1`) with `node_type: axiom` and
    `canonical_path: axioms.md`; the work node and the `founds` row's source become
    `wrk-smith2020`; the old work premise stays
    `founds` at `0.7`, and the old analysis `0.5` and experiment `0.8` stay `lifts`; `rigor` stays on
    every claim and the analysis row; the DOT goldens change with the arrow flip and the four stroke
    names, and the diff is reviewed), `internal/write/testdata/writeapi-render-golden/*`, and
    `internal/write/testdata/writeapi-regression/part{1..5}-claim-quality.md` (restamped here, its
    file name renamed by WP7).
  - Every inline register literal in `internal/index`, `internal/query`, `internal/write`,
    `internal/kb`, `internal/build` (`inference_test.go`'s captured edges) and `cmd`
    (`format_test.go`'s query table and index-file list, `resultdocs_test.go`, `resultkeys_test.go`,
    `maintenance_test.go`), plus `test_data/fixtures/mcp/*.jsonl`'s §7 keys.
  - Inline `kb.State` corpora carry `Bearing{Value: 1}` on every scoring edge that reaches the walk,
    keyed on `st.Analyses` and `ana-` ids. A coder who updates only the parse fixtures sees the index
    expectations collapse to zero; the `internal/index` `dep()` helper is the one-line fix.

**Facts the step rests on.**

- `rg -n 'sup' internal/migrate/from_1_0_0.go` → `:18` (the rename table's support row), `:26` (the
  entry-marker regex, which matches a `sup-` id and leaves it), `:112` (the `support-nodes` section
  test) and `:150` (`supports:` under `support-nodes`): the four places the old spelling is read and
  the new one must be written.
- `rg -n 'work-' internal/migrate/from_1_0_0.go` → no output: the converter has no work-id rewrite
  today, and the `WorkPrefix`/`WorkIDPattern` pair it must mirror is `internal/kb/schema.go:144`,
  `:148`.
- `rg -n 'axioms\.md' internal/migrate` → no output, and
  `ls internal/migrate/testdata/register-1.0.0/kb-root` → no `invariants.md`: the axiom step and its
  fixture both have to be written.
- `grep -n 'func convert100To200\|func Chain\|var converters' internal/migrate/*.go` →
  `from_1_0_0.go:49`, `migrate.go:36`, `migrate.go:27`.
- `grep -n 'func MigratedSource\|func (s \*Source) Saved\|func (s \*Source) Migration'
  internal/kb/source.go` → `:54`, `:87`, `:78`.
- `rg -n 'sup-|support-nodes' internal/migrate/testdata/register-2.0.0` → the expected side's
  remaining old words, which the restamp removes.

**Acceptance.** A **valid** `1.0.0` KB opens, is `2.0.0` after one `refresh`, and `verify` reports
it stale before that; no hand edit is needed for any of it. A `1.0.0` KB that is already malformed
is refused by the load at the fault, and the converter neither repairs nor refuses it. The golden
pair converts byte for byte; its `2.0.0` output loads through `kb.Discover` with the same edge set
(push pairs, analysis edges and the axiom premise included) and every carried token value, carries
`axioms.md`, cites `Axiom S3`, names no `sup-` id and no `work-` id, while the byte-only case's
`(bearing sometimes)` is refused as R3. A `2.1.0` and a `3.0.0` KB are refused
`check: kb-format`. A query on a converted-but-unsaved KB is refused with remedy `kbase refresh`. A
`0.9.0` KB yields one `0.9.0 → 2.0.0` record.

### WP6 — contract text and templates

**Scope.** `SPEC.md`, `ARCHITECTURE.md`, `README.md`, `USER_CONVENTIONS.md`, `ROADMAP.md`,
`INTEGRATING_KBASE_VIA_MCP.md`, `internal/kbdocs/templates/{README,CONVENTIONS,AGENTS}.tmpl.md`.
`STRENGTH_SCORING_DESIGN.md` and `STRENGTH_SCORING_DIAGRAM.md` are **not** in a coder's scope: the
design is the authority and the diagram is its human-facing picture, so where the built behaviour and
either disagree the coder reports the drift (path, section, the behaviour that contradicts it) and
stops.

**Build.** Each edit is a rename or a statement of the model as built; none is a redesign.

- **SPEC §3**: the sheet paragraph — nodes filled by strength band, strokes naming the four
  relations and their push direction.
- **SPEC §5**: attribution writes `supports`, not `depends`.
- **SPEC §7**: the shared vocabulary gains the conditional `migration` record after `kb-root`;
  `deps`' records are `relation`, `direction`, `target`, `bearing`, `context`; `show`'s keys are
  `rigor` and `strength`; `stats`' are `strength_bands`, `edges` and the plural count keys
  (`analyses`, not `analysiss`); the query list's
  `solidity-below` becomes `strength-below`; the `stats`/`weak-points` headings gain one sentence
  each that the value surfaces read the derivation while `dependents`, `premise_count` and the
  counts are structural. The item's `check` list (`SPEC.md:385`) gains `axiom-source`, and the
  `verify` row's `findings` text (`:408`) names the legacy axiom declaration: an `axioms.md` plus an
  `AGENTS.md` still carrying a declaration is one `axiom-source` item, informational, no `remedy`.
- **SPEC §8**: `set-bearing` replaces `set-applicability` and `set-on-point-fraction`; `set-rigor`
  absorbs `set-work-strength`; `add-depends-on` becomes `add-rests-on`; `drop-claim-edges`'
  `edge-class` value becomes `rests-on`; the insert value shapes are named.
- **SPEC §10**: one paragraph for the five node types and the prose legend (invariant/framework ⇒
  axiom); one for the three quantities and the four relations' arithmetic; one for edge direction
  and authoring; one for required bearings, one scoring target per bullet, the R1–R5
  refusal by path and line, the index column's position and `null`'s meaning; the `rigor`/`strength`
  renames with the register's final vocabulary — `claim-rigor.md`, the entry's two H3s (`### Rigor`
  for the authored fields, `### Strength` for the derived line and its trace), the Tier-2 marker
  `<!-- claim-rigor: … -->` — and the id prefixes (`clm-`, `ana-`, `exp-`, `wrk-`, `axm-<label>`);
  the migration paragraph with the three value changes stated explicitly, the register's
  file/heading/marker rename, the `work-`→`wrk-` rewrite and the `axiom-source` finding, and the
  announcement; the version statements and the closing sentence become `2.0.0`; the `demoted`
  paragraph keeps `1.0.0` and its `depends` references become `supports`.
- **ARCHITECTURE §6**: the one bearing op and the one rigor op, replace-only, read-back, and that no
  op renders a scoring bullet without a token and no build path writes a judgement; a claim's and a
  analysis's local value are one field. **The query layer**: `deps`' incidence adapter, and only it;
  no text verb is added (Phase 2 documents the internal renderers). **The derived layer**: point at
  `internal/index/strength.go` as the arithmetic's authority and carry the four-relation table with
  the worked example, and cite `STRENGTH_SCORING_DIAGRAM.md` rather than reproducing it. **The node
  vocabulary**: the five types, the legend, and the axiom-source reader (`axioms.md`, one heading
  per declaration).
  **§11's `internal/kbload` entry**: the requiredness gate, the loader monopoly, the zero-value
  hazard (a register parsed outside `Source` reads a missing token as `0.0`, a zero contribution),
  the migration record and announcement, and the axiom source chain (`axioms.md`, else legacy
  `invariants.md`, else `AGENTS.md`) with the informational `axiom-source` finding.
  **§12 constants**: the format row, the pending literal, the conversion record.
  Rename §4's stage-table `depends-attributed` row, §6's mechanism table's index row and §3's sheet
  paragraph.
- **README** and **USER_CONVENTIONS**: the query and op lists, the sheet's fill, the state of a
  claim that carries no rigor.
- **INTEGRATING_KBASE_VIA_MCP.md**: `add-depends-on` → `add-rests-on`, `solidity-below` →
  `strength-below`; no other change.
- **ROADMAP.md**: the bearing item is shipped at `2.0.0`; drop "intervolume" (no referent exists);
  move the next major bump to `3.0.0`.
- **`internal/kbdocs/templates/{README,CONVENTIONS,AGENTS}.tmpl.md`**: product prose for installed
  KBs — WP1–WP5 left all three on the old words, so this package finishes them. Word swaps:
  `solidity`→`strength`, the two local-score names→`rigor`, `depends-on.yaml`→`edges.yaml`,
  `solidity_bands`→`strength_bands`, `claim-quality.md`→`claim-rigor.md`,
  `### Quality`→`### Rigor`, `<!-- claim-quality:`→`<!-- claim-rigor:` and `invariants.md`→
  `axioms.md` — the contract text is written in the final vocabulary, and WP7's first step makes the
  tree agree with it. Two sentences are not swaps: `README.tmpl.md:29` ("`supports`
  and `strengthens` run the other way") must describe the `2.0.0` rows — old support pairs are now
  `lifts`, and `supports` is the premise relation — and `CONVENTIONS.tmpl.md:12-14`'s
  `depends`/`supports` cycle sentence must name the four scoring relations and the acyclicity rule
  over all of them. `AGENTS.tmpl.md:30` is a stamped document (`readinessDocs`, `kbdocs.go:30-31`)
  that must say axioms live in `axioms.md`, not `invariants.md`. There is no rendered kbdocs golden:
  the checks are `internal/kbdocs/kbdocs_test.go` (which asserts the stamped text and the redirect),
  `cmd/init_test.go:149` (the born tree's stamped documents) and the validation gate's restamp at
  `internal/build/rows.go:350`.
- **`STRENGTH_SCORING_DIAGRAM.md`** is cited, not edited: ARCHITECTURE points at it for the
  human-facing arithmetic picture rather than restating the walk. It already carries the five node
  types, the four relations and the three-letter id prefixes from `d103548`; a drift between it and
  the built behaviour is reported as scope above says.
- **`ROADMAP_PLANS/BEARING_FIELD.md`**: already removed with the checkpoint that queued this plan;
  its content is folded here. Do not recreate it.
- **CONVENTIONS.md**: no change. Say so in the commit body so a reviewer does not look for one.

**Acceptance.** Each edited sentence is checked against a named observation, not against `--help` in
general:

- every renamed verb/flag sentence — one run of that verb (`./bin/kbase <verb> --help`, then the run
  the sentence describes);
- every `deps`/`show`/`stats` key name — one run against `test_data/fixtures/results` and a key
  diff against SPEC §7;
- every migration and version sentence — the design's §2.8 rewrite table read line by line, and a
  `2.1.0` refusal. The `register-2.0.0` golden still carries the old names until WP7, so this
  package checks its prose against the design, not against the golden; the golden is WP7's step one.
- every `verify` sentence — one run on a KB carrying the finding;
- the sheet sentences — the regenerated adjagent-sheet DOT goldens.

`rg -n 'solidity' README.md USER_CONVENTIONS.md SPEC.md ARCHITECTURE.md` returns only §10's
superseded-format history and the code spans that name the old form.
`rg -n 'claim-quality|### Quality' README.md USER_CONVENTIONS.md SPEC.md ARCHITECTURE.md` returns
only the migration paragraph's description of the old names.

### WP7 — the register's vocabulary, the census, the close-out

**Step one — the register's vocabulary.** The design settles the register's names on `rigor`, and
this package runs the rename in one pass before the census, so the census reads the final vocabulary.
The rename is mechanical and total:

- `kb.RegisterFile` (`kb/register.go:16`) becomes `claim-rigor.md`; every consumer follows the
  constant (`kb.Registers`, `discover.go:58-61`, and its call sites at `:68`, `:117`, `:205`;
  `internal/index/verify.go:190`; `internal/export/assemble.go:394`; `internal/claimgraph/graph.go:46`,
  `write.go:182`; `internal/index/citations.go:344`), and the excluded-name list at
  `internal/kb/kb.go:81` keeps `claim-quality-closure-roadmap.md`, which is a different document.
- The heading: WP4 lands the reader and writer flip — `qualityStart`, the field spans and the
  writer's two emissions. What remains here is `internal/index/verify.go`'s `### Quality` matches
  (`:368`, `:375`, `:387`); until they read `### Rigor`, `qualityBlocks` checks nothing on a
  minted entry.
- The Tier-2 marker: `tier2InlineRE` (`kb/leaf.go:21`), `inBodyMarkerRE` (`:29`),
  `internal/index/verify.go:397`'s `tier2RE`, `internal/index/citations.go:30`'s `tier2MarkerRE` and
  `write/render.go:94`'s `renderTier2Marker` read `<!-- claim-rigor: … -->`; the operator-facing
  strings that name it (`cmd/mark-claim-in-leaf.go:6`, `cmd/drop-claim-node.go:7`, `write/ops.go`
  `:1407`, `:1638`, `:1687`, `:2454`, `write/values.go:229`) are reworded with it.
- `QualityFieldKeys` (`register.go:24`) becomes `RigorFieldKeys`; `write/store.go:309`'s
  `claimFoldBreakRE` and every other reader of the list follow. `RegisterEntry.RigorLine`,
  `EntryLocation.RigorStart`/`RigorEnd` and `rigorStart` follow with them, and their call sites in
  `index/refresh.go` and `write/{ops,store}.go`.
- The `"support fan-out"` check name (`index/verify.go:948`) becomes `"analysis fan-out"`, and the
  orphan-block key `quality block` becomes `rigor block`. Neither has a golden; the keys alone move.
- The converter gains the row that produces these names: `claim-quality.md` → `claim-rigor.md`,
  `### Quality` → `### Rigor`, `<!-- claim-quality: … -->` → `<!-- claim-rigor: … -->`, and
  `### Solidity` → `### Strength` — heading only, the entry's fields unchanged. The file rename is a
  key change in the returned `Files` map, on an exact basename match against the converter's own
  `"claim-quality.md"` literal — the package imports nothing of kbase — so
  `claim-quality-closure-roadmap.md` is untouched; and the old path is **appended to the converter's
  `obsolete` list**, because a key change alone writes the new file and leaves the old one on disk.
  `kb.MigratedSource` (`internal/kb/source.go:54-95`) carries those paths, and the saving `refresh`
  removes them. Its own reader keeps the old spellings (the package rule), and the golden pair's
  input keeps the old file name while its expected side carries the new one.
- Every fixture and golden that carries the old names: the golden pair's two sides'
  `claim-quality.md` paths, `internal/write/testdata/writeapi-regression/part{1..5}-claim-quality.md`
  (renamed), `internal/sheet/testdata/adjagent-sheet/kb-root`'s register, every inline register
  literal, and `test_data/fixtures/results` where it names one.
- The contract text is already in these names: WP6 writes them, and this step is what makes the tree
  agree.

**External consequence.** Any script, note or external reader that opens `claim-quality.md` by name,
or greps for `<!-- claim-quality:`, stops resolving after a KB is migrated. The Phase 3 reference
reader must know the new filename, heading and marker (`PLAN_3_TOOLING.md`), and the migration
announcement is the only signal a converted KB gives that it did.

**Acceptance — step one.** `TestConvert100To200Goldens` passes with the expected side's register at
`kb-root/claim-rigor.md` and no `claim-quality.md` in the golden; on a real KB, the saving `refresh`
leaves no `claim-quality.md` in `kb-root` (the obsolete path removed with the rest). One `kbase init`
in a temp repo writes `claim-rigor.md` under `### Rigor` with a `<!-- claim-rigor: … -->` marker,
`kb.Registers` finds it, and `verify` returns `done`. `rg -n 'claim-quality|QualityFieldKeys'
internal cmd -g '!testdata'` returns only the converter's read of `1.0.0` bytes,
`claim-quality-closure-roadmap.md` and its exclusion entry; `rg -n '### Quality' internal cmd` returns
only the converter's rewrite rule for the heading.

**Step two — the census.** WP1–WP5 landed the three-quantity renames, so this is no longer a
before-and-after over the tree: it is the check that the renames are total, run over the reworked
tree, and it is also where the node-word rename is held. Read the hits, do not count them:

```
rg -n -i 'solidity' internal cmd -g '!*_test.go' -g '!testdata'                       # 12 today
rg -n -i 'confidence|quality|applicability|fraction' internal cmd -g '!*_test.go' -g '!testdata'
                                                                                       # 125 today
rg -n '"confidence"|"quality"|"solidity"|"applicability"|"fraction"' internal cmd -g '*_test.go'
                                                                                       # 4 today
rg -n 'RelationDepends|RelationRestsOn|RelationStrengthens' internal cmd                # 0 today
rg -n 'depends-on' internal cmd -g '!testdata'                                          # 15 today
```

Those counts are the tree **before step one**: once the register's vocabulary lands, `quality` drops
out of the second and third lines and what remains is the converter's read of the old keys.

The node-word renames the design's §2.9 commands do not reach — `support` is not one of its three
quantities — are held by two more:

```
rg -n 'NodeKindSupport|SupportNode|SupportEntry|ParseSupport|support-nodes|sup-id|"sup-' \
   internal cmd -g '!testdata' -g '!*_test.go'                                          # 77 today
rg -n 'NodeKindInvariant|FrameworkNode|FrameworkKinds|FrameworkSource|invariants\.md' \
   internal cmd -g '!testdata' -g '!*_test.go'                                          # 30 today
```

Those two patterns miss the unexported family the review found live: `supportFieldKeys`
(`kb/register.go:26-28`, `:787`), `supportDecl`, `supportNode`, `SupportNodes`, `checkSupportNodes`,
`renderSupportNodes` (`write/render.go:86`, `:265`, `:304-330`, `write/values.go:503-504`, `:569`,
`write/ops.go:2323`) and `supportNodeFields`. A third pattern holds them, test files included, since
the read-back expectations spell the same words:

```
rg -n 'Support[A-Z][A-Za-z]*|support(Decl|Field|Node|Entries|ID)' internal cmd -g '!testdata'
                                                                                      # 100 today
rg -n 'WorkPrefix|"axiom-' internal cmd -g '!testdata'                                 # 22 today
```

The boundary is deliberately absent: `checkSupportNodes` and `ParseSupportEntries` put a word
character before `Support`, and a `\b` there hides the very sites this line exists to find. The
last line is the id grammar's own check: after the rework it returns `WorkPrefix` uses (whose
value is now `"wrk"`) and the `"axm-"` id builders, and a `"axiom-"` literal anywhere is a builder
that did not move.

After: the first three return only the survivors named under Non-goals and the migration's spelling
of old keys as text. The `solidity` hits that are neither the converter's read of `1.0.0` bytes are
`internal/kbdocs/templates/*.tmpl.md` (WP6's, and this census is the check that WP6 finished) and
`cmd/refresh.go:205`'s doc comment, which this package rewords — it is the one code comment in the
census with no other owner. The relation constants return nothing.
`depends-on` returns only the converter's read of the old key and SPEC §10's `1.0.0` description of
it. The support censuses return only the converter's read of `sup-` and `support-nodes` — the
relation word `supports` matches none of the three patterns, and a frozen `0.9.0` reader in
`internal/kbload/parity_test.go` follows the type rename like any other call site. The invariant
census returns only the converter's read of `invariants.md`/`AGENTS.md` and the load's legacy
fallback. Assert the survivor list as a complete set in a test or a reviewed grep, not as a count.

The register's old vocabulary is the same kind of check:
`rg -n 'claim-quality|### Quality|### Solidity|QualityFieldKeys' internal cmd -g '!testdata'`
(130 today) returns after step one only the converter's read of `1.0.0` bytes, the
`claim-quality-closure-roadmap.md` exclusion entry and that document's own name, and no marker,
heading or constant.

`rg -n 'rests-on' internal cmd -g '!testdata'` (97 today) is the design's fourth check: after the
rename it returns the register's list key `ListRestsOn`, the `add-rests-on` op name, and prose about
the list. `internal/sheet/dot.go:492`'s comment still spells a stroke count `rests-on` and is
reworded with it. A `rests-on` used as a relation name is the finding.

## Contract deltas

Phase 1 owns: SPEC §3, §5, §7 (the `migration` record, the renamed keys and the `axiom-source`
check), §8, §10; ARCHITECTURE §3's sheet paragraph, §4's stage row, §6, the derived layer, §11,
§12; README; USER_CONVENTIONS; ROADMAP; `INTEGRATING_KBASE_VIA_MCP.md`'s renamed verbs; the three
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
5. **A valid old KB opens without hand edits.** A **valid** `1.0.0` KB opens through migration with
   `- rigor:`, `(bearing …)` on every scoring edge, `- strength:`/`(strength …)` and the renamed
   relation keys; `verify` reports it stale; after one `refresh` it is `2.0.0`. Claims whose lift
   exceeded their derivation read higher; a work bearing below `1` reads lower; a `0` work bearing
   reads `0.00`; an untokened work edge now reads a number, and an explicitly pending one is
   excluded so its claim reads its own value. A `1.0.0` file that was already malformed is refused
   by the load at its fault and is not repaired. *Falsified by:* a required hand edit on a valid
   `1.0.0` KB, a refused load of a valid one, a refusal that names no token, an unscaled work edge,
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
10. **A multi-target `1.0.0` bullet migrates.** Two claims, a claim and a work, or a claim and an
    axiom become one bullet per scoring target, legal under R1, with the same edge count and no
    lost axiom token. *Falsified by:* a converter refusal, a load refusal on the converted bytes,
    or a changed edge count.
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
    or `solidity`, and the Non-goals list — a letter-confidence map, strengthen-by, the band names —
    is the complete set of old words left. The register itself is `claim-rigor.md` under a
    `### Rigor` heading with a `<!-- claim-rigor: … -->` marker and `RigorFieldKeys`, and the derived
    heading is `### Strength`. *Falsified by:* two names for one quantity, a score number that
    changed in the rename, an old score name outside the list, or a register under the old file,
    heading or marker.
14. **The ledger-free surfaces are unchanged.** `stats`' counts, `dependents`, `premise_count`, the
    cycle report and the sheet's strokes do not move when a bearing changes. *Falsified by:* any of
    them differing across two runs that differ only in a bearing.
15. **The graph has five node types, one id grammar, and an axiom is a source like any other.**
    `show`, `deps`, the index and the sheet name `claim`, `analysis`, `experiment`, `work`, `axiom`
    and nothing else; no `sup-` id, `support-nodes` key or `Support*` symbol remains; every id is
    `clm-`, `ana-`, `exp-`, `wrk-` or `axm-<label>`, and no `work-` or `axiom-` id survives a
    migration; an axiom premise carries a bearing, contributes `bearing × 1.0`, and a `0.4` bearing
    halves it. *Falsified by:* an `invariant`, `framework` or `support` `node_type`, a `sup-`,
    `work-` or `axiom-` id in a result document or a golden, a bearingless scoring edge, or an axiom
    contribution that ignores its bearing.

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
4. **`DependentsOf` reads the push direction only.** The targets of `id`'s outgoing scoring edges,
   not the sources of edges into `id` (those are its premises, reported as `deps` `in` rows). A
   references or demoted edge makes neither end a dependent.
5. **The sheet's arrows flip.** Premise edges draw supporter→supported, so the arrowhead reverses
   relative to the `1.0.0` goldens; the transitive reduction runs over `supports` push edges (a work
   is a leaf, so `founds` reduces nothing); the adjagent-sheet DOT goldens change with the arrow
   flip, and that diff is reviewed rather than asserted byte-identical.
6. **`drop-claim-node` and `drop-inbound-edges` see pair lists.** A `lifts`/`refutes` edge is
   authored on its source, and a claim's `refutes` edge on the refuting claim's own entry; a drop
   that reads only `rests-on` bullets strands it.
7. **The converter imports nothing of kbase** and holds no second copy of the domain; it carries an
   existing token as text and lets the load refuse it by name. `1.1.0` does not exist and cannot be
   added.
8. **The index is never converted.** It is derived; the converter declares the two old paths
   obsolete and `refresh` writes the new ones.
9. **`justfile:1120`.** The results-fixture leg calls `strength-below`; a stale invocation is caught
   by the recipe's key check.
10. **`support` is not a word to replace.** `supports` names a relation that stays, and `supports:`
   is an experiment's pair list; only the node family (`SupportNode`, `support-nodes`, `sup-`,
   `NodeKindSupport`) moves to `analysis`/`ana-`. A blanket replacement turns the relation into a
   node name and breaks every edge.
11. **The converter reads the old spelling and writes the new one.** `sup-`/`support-nodes` and
   `work-<citation key>` are `1.0.0` bytes on the input side and never appear on the output side; a
   converter that emits `sup-` or `work-` produces a KB the reworked load refuses.
12. **The register's old names are one string in five places.** `RegisterFile`, `qualityStart`'s
   `### Quality`, `tier2InlineRE`'s `claim-quality:`, `QualityFieldKeys` and the converter's read of
   them move together in WP7's step one; renaming the file but not the marker (or the constant but
   not the writer) leaves every register unreadable. `claim-quality-closure-roadmap.md` is a
   different document in an exclusion list and is not renamed.
13. **The id prefixes are one spelling across three layers.** A `wrk-` id has to be written, parsed
   and indexed as `wrk-`; half a rename leaves a `founds` edge whose source names no node, and the
   load's logged drop hides it as a stale id.
