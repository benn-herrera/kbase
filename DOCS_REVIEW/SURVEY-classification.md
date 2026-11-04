# SURVEY — standard vs novel classification of THESIS and ARCHITECTURE

Inputs: `DOCS_REVIEW/THESIS.md` (T), `DOCS_REVIEW/ARCHITECTURE.md` (A), `DOCS_REVIEW/SPEC.md` (S) where A cites
it, and code under `internal/`. `file:line` citations use those abbreviations for the three docs and
repo-relative paths for code. Pattern names are the surveyor's own working-engineer labels; none
was checked against literature.

## 1. Summary

- **Share:** about 94% of A is standard. Novel or mixed is about 140 of 2422 lines, the sum of A:76-82, 535-586, 697-768 and 801-807.
- **Novel core:** (a) the three fault vocabularies and their price, max − Σ per-occurrence weights, floored at 0
  (`internal/kb/faults.go:101-137,172-189`); (b) the strength formula `min(max(min(rigor, bearing×src), lifts), caps)`,
  bare claim 0 and bare analysis own rigor (`internal/index/strength.go:220-242`, S:2229-2247); (c) the ontology:
  5 node kinds × 4 scoring relations (A:568-573) and the analysis split (A:82); (d) possibly the mapping of theorem
  environments and `\ref` onto draft claims and edge candidates (T:24-36). What runs these is standard.
- **Worst framing:** Write API (A:432, a single 1,086-word table cell about a transactional batch); scoring
  runner/ledger/landing (A:666-1005, a job runner with an append-only log); locks (A:1375-1407, pid lockfiles);
  format migration (A:2095-2181, chained upcasters); argmark (A:1618-1630). None of them names its pattern.

## 2. THESIS

| Lines | Claim as presented | Verdict | Which part is which |
|---|---|---|---|
| T:7-12 | "distillation is deserialization"; LaTeX is a flattened tree | **Standard** | Reading the document outline from a parser's AST; pandoc already does this (A:107-118). The metaphor makes ordinary parsing sound like a discovery |
| T:16-22 | Navigation hierarchy is transcription, "a second implementation produces the same tree" | **Standard** | Sectioning → document tree, deterministic build (A:52-54). Nothing novel |
| T:26-30 | Theorem/definition/equation environments are claim nodes with identity | **Mix** | Standard: extracting labelled environments from an AST (a cross-reference index). Novel: treating each one as a *claim node* in an argument graph |
| T:32-36 | `\ref` gives edge candidates, not edges (mention / supported-by / in-support-of) | **Mix** | Standard: a `\ref` cross-reference graph. Novel: the 3-way typing problem it sets up (T:55) |
| T:45-46 | "Inference refines a mechanical draft. It never produces one." | **Standard** | Rules-first baseline with ML refinement and a deterministic fallback; restated as A:49-51 "monotone safety" |
| T:48-55 | Exactly three bounded gaps | **Mix** | Each gap is a known task: (1) claim detection in prose, (2) implicit-reference / pairwise link detection, (3) support-vs-attack relation classification, all from argument mining. Novel: limiting inference to exactly these three, and the 3-option edge vocabulary |
| T:57-60 | Base claims at the bottom; citations are external foundations | **Standard** | A DAG whose sources are axioms or external citations. The ontology terms (`founds`, A:571) are kbase's |
| T:64-77 | Build and score time targets | n/a | Performance goals, not a novelty claim. Keep them as non-functional targets |
| T:81-83 | Local judgments cascaded into a whole-argument picture | **Mix** | Standard family: propagating local scores through a DAG (weakest-link / fuzzy min-max; gradual argument semantics). Novel: *which* local judgments (§1 core a) and *which* combination (§1 core b) |
| T:85-88 | Intelligence-neutral: consistent across human and machine scorers | **Mix** | Standard aim: inter-rater reliability. Novel: getting there through closed fault checklists with line-cited charges (S:1169-1203) |
| T:90-92 | Narrow, hard-to-argue questions give consistency | **Standard** | Rubric decomposition into atomic checklist items, as practised in annotation work and LLM-as-judge setups. Its realization (the fault ids) is novel |
| T:94-101 | Arrangement invariance: same points, different layering or step size → near-same score | **Novel requirement, not shown to be met** | No document argues that the formula satisfies it. Contribution is `bearing × source strength` per hop (`internal/index/strength.go:151-160`), compounded through `min` at each level (`:227`), so a chain's value decays with its depth whenever bearings are below 1. Nothing in `STRENGTH_SCORING_SCHEME.md` covers invariance (grep for depth/chain/layers matched only example 1's title at :262 and :444). Flag for the rewrite: state this as an open property of the novel core, not as a property already held |

## 3. ARCHITECTURE classification

Columns: **What** (in kbase's terms) · **Pattern** · **Departure** (R = the documents give a written reason, cited; N = no reason
found).

### 3a. Invariants and principles (A:18-63)

| Ref | What | Pattern | Departure |
|---|---|---|---|
| I1 A:24 | One package execs each of pandoc, git, dot, self | One adapter per external process (ports and adapters), enforced by an architecture fitness test (`cmd/execpolicy_test.go`, A:2038-2042) | None |
| I2 A:25 | Which layer may read leaf lines | Single source of truth per fact; reads go through a repository | The exception list sits inside the invariant. N |
| I3 A:26 / §5.2 A:120-125 | Two source pre-passes in one registry with a census | Preprocessor registry with instrumentation | Adding one is an architecture change. R: no LaTeX parser (A:55) |
| I4 A:27 | Compatibility judged by readers, not bytes | Semantic or contract testing over golden fixtures | None |
| I5 A:28 / §9 | One emitter owns stdout | A single structured-output writer; outcome → exit-code table | None |
| I6 A:29 | Re-issued write is `refused`/`unchanged` | Idempotent writes (create-if-absent / PUT semantics) | Re-adding something that already exists is *refused*, not a no-op. Reason is in a retired design page (A:29 link). N in the live docs |
| I7 A:30 | Prompts are embedded files | Prompt templates as embedded assets (`go:embed`) | None |
| I8 A:31 | One format version; older KBs converted in memory at load; only `update-format` saves | Read-time upcasting plus an explicit migrate command; repository/loader monopoly | R: A:1967-1968 (frozen converters); S §10 |
| I9 A:32 | MCP tools derived from cobra annotations | Generated API surface from a command registry | None. Over-specified in prose: one 287-word cell (`wc -w` of A:32) |
| I10 A:33 | Every recorded ask can be re-rendered | Record/replay reproducibility (request capture) | None |
| I11 A:34 | During an ask batch, one file per result; shared files written after drain | Fan-out/fan-in: per-task output files, single-writer merge after join | The rule is stated without a reason. N |
| I12 A:35 | Every model call through a `Slot` from one `Queue` of N tokens | Counting semaphore (buffered channel, `internal/model/queue.go:10-17`) with a capability token (unexported wire methods) | None |
| A:44-46 | Propose-and-verify | LLM proposes, deterministic code validates (generate-then-validate guardrail) | None |
| A:47-48 | Model supplies values; machine owns syntax | Constrained or structured output (closed-choice answers) | None |
| A:49-51 | Monotone safety | Graceful degradation to a deterministic baseline | None |
| A:52-54 | Byte-identical document graph; claim graph identical modulo ids | Hermetic, deterministic build | "Modulo ids" because ids are minted randomly. R: S §5 Determinism (S:236) |
| A:55 | No LaTeX parser written here | Reuse an existing parser (pandoc) | None |
| A:58-63 | Served surface is the operating contract | Self-describing API (help as the documentation) | None |

### 3b. Build pipeline (A:67-101, §8 A:1321-1580)

| Ref | What | Pattern | Departure |
|---|---|---|---|
| A:69, 86-88, 1350-1358 | Stages with a git commit at each boundary; resume re-runs the interrupted stage | Checkpoint/restart pipeline; stage = idempotent unit | Position is read from the commit trail, never from the state store. R: A:1330-1331, S §6 (S:397) |
| A:90-95 | `--through`, `--no-inference` by row classification | Pipeline stop-after flag; feature flag per step | `spendsInferenceInPart` runs with a nil launcher. R: A:93-94 |
| A:73 `start` | Preflight, lock, record inputs | Preflight checks plus a job manifest | None |
| A:74-75 | Document graph, spine seed | Deterministic transform stage | None |
| A:76 `claims-declared` | Claims from author-declared environments | **Mixed.** Standard: AST extraction. Novel: environment → claim node (T:26-30) | None |
| A:77 `claims-discovered` | Model finds prose claims | Standard task: claim detection by an LLM classifier. **Novel:** what counts as a claim | None |
| A:78 `equations-minted` | Equation nodes minted | Mechanical extraction | None |
| A:79 `references-found` | Adjacency links by rule; per-source shortlist of proximity slots + TF-IDF cosine wildcards; one yes/no ask per pair | Candidate generation (blocking) then pairwise classification, as in record linkage and retrieve-then-rerank. TF-IDF cosine: `internal/claimgraph/shortlist.go:163-178` | Slot counts 3 + 2 (A:2089-2090) have no reason. N |
| A:80 `claims-culled` | Pairwise "adds nothing" verdicts; culled claim collapses into an earlier survivor; cycles among targets broken | Near-duplicate detection, then a canonical representative (entity resolution / dedup clustering), `internal/claimgraph/cull.go:44-50` | The survivor is the earliest in reading order. Stated as a rule (cull.go:44-50; S:318), with no reason why. N |
| A:81 `depends-attributed` | One ask per candidate for direction/type; all edges written; then cycles cut | Pairwise relation classification, then cycle removal (see 3d) | Cycles are written first and cut afterwards in the same stage. R: A:524-525, 1911-1915 |
| A:82 `analyses-split` | A claim with text beyond its statement gets an analysis node hosted at a computed position | **Mixed.** Standard: deterministic text segmentation. **Novel:** the claim/analysis split itself (ontology) | None |
| A:83-84, 1315-1317 | Readiness stamps; LLM overview with a structure check and one re-ask | Generate → validate → retry once | None |
| A:1294, 1440-1443 | Answer cache keyed by model, system, prompt, sampling and attempt ordinal | Content-addressed memoization cache | The attempt ordinal is part of the key. R: A:1443 |
| A:1296-1297 | An unreadable reply is asked once more, then defaults to the mechanical draft | Retry, then fallback | None |
| A:1371-1374 | Missing records regenerated and checked byte-equal against the commit | Reproducible-build verification | None |
| A:1416-1434, 2080-2082 | Keep newest N builds, trails, runs and progress files | Keep-last-N retention policy plus log rotation | None |
| A:1408-1415 | State-store `format` integer with a stepwise upgrade chain | Versioned on-disk schema with sequential migrations | None |
| A:1453-1523 | Build dev replay | Offline eval / record-replay regression harness | Compiled out by a build tag. R: A:1017 |
| A:1524-1528, 1544-1568 | `status`, `progress.jsonl`, `cancel`, `wait` | Background job with a status file, signal cancel and a polling wait | None |
| A:1529-1534 | Trail-order refusal; stale `.git/index.lock` refusal | Precondition checks | None |
| A:1569-1580 | Recorded documents re-spelled on re-emit | Late-bound presentation (store neutral, render per surface) | None |

### 3c. Reader and LaTeX export (§5, A:105-424)

| Ref | What | Pattern | Departure |
|---|---|---|---|
| §5.1 A:107-118 | pandoc seam: version check, include-escape check | Wrapped external tool with a version gate; path-traversal check | None |
| §5.3 A:127-136 | Go transforms over the pandoc JSON AST | Pandoc AST filter (written in Go) | Go rather than Lua. R: A:193-195 (records are emitted) |
| §5.4 A:138-141 | gfm writer `--wrap=none` | Off-the-shelf writer | R: §13 slice verdict |
| §5.5 A:143-176 | Records JSONL beside the leaves | Sidecar metadata stream | Spelling escaped to match the page. R: A:164-166 |
| §5.6 A:178-188 | Partition, maths-survival, anchor and dead-link checks | Round-trip / differential conformance checks; link checker | No fix loop. N |
| §5.7 A:190-424 | `export-latex`: Go assembles, Lua filter, template, manifest | Pandoc multi-file conversion with a Lua filter and a custom template; manifest-tracked output dir (owned files plus stale-file sweep) | Workarounds for pandoc 3.12.1 writer bugs, each with a reason (A:321-336). 235 lines of standard pandoc engineering, written up as design |
| §13 A:2352-2379 | Slice verdicts | Spike/prototype decision record | History rather than mechanism; it would read as an ADR |

### 3d. KB model, writes, derived layer (§6, A:427-653)

| Ref | What | Pattern | Departure |
|---|---|---|---|
| A:431 | `internal/kb`: names, link grammar, loader, slugger, register-page rule | Domain model / schema package; GitHub-style heading slugs | None. One 350-word cell (`wc -w` of A:431) |
| A:432, 440-475 | Write batch: shape → apply in memory → derive → write; dry run; commit | **Transactional batch (unit of work)** with validate-all-first, dry run and a post-write commit hook | Files are not atomic as a set: each file is temp+rename, and a failure mid-write is `failed` "naming what already landed" (A:473-474). Recovery is git (S:397). N in A |
| A:458-465 | `readBackMarkers`; disk re-checked against what was read | Optimistic concurrency check (compare before write) layered on a lock | None |
| A:432 (tail), 2078 | One advisory KB write lock, 30 s wait → `retry` | Advisory file lock with timeout | Windows has no lock; only the read-compare check. Stated, N |
| A:432 (rename/repoint) | Rename re-points every link and anchor | Referential-integrity maintenance (rename refactoring) | None |
| A:498-510 | `manually-scored` provenance; machine never overwrites a person's value | Manual-override flag (user edits win) | Provenance is an options field, never on the surface. R: A:506-508 |
| A:512-533 | Build-only ops `add-build-edges`, `demote-claim-edge`, `split-claim`; public edge add refuses a cycle | Internal-only privileged ops; DAG insert with cycle check (path finder) | None |
| A:535-556 | Five node kinds, minted ids with a kind prefix | **Novel** ontology; standard prefixed random ids | None |
| A:558-586, 568-573 | Strength: four relations, three stations, worked example | **NOVEL** formula (§1 core b). Execution is a **topological fold over a DAG**: Kahn's order, `internal/index/strength.go:244-296` | Full float precision in the walk while rigor/bearing are integer hundredths. R: A:562-563 |
| A:588-597 | Query layer: one incidence adapter | Read model / query projection | None |
| A:598-607 | Frontmatter recogniser | Heuristic sniffing before parse | None |
| A:609-636 | Sheet: DOT → `dot -Tsvg`, fallback straight splines, ≤4 processes | Graphviz rendering with a bounded process pool; materialized view regenerated on write | Golden tests pin the DOT, not the SVG. R: A:611-613 |
| A:433 | Register pages, freshness, `verify` | Materialized views rebuilt after each write, with a staleness check | None |
| A:638-653 | `init` composes existing mechanisms | Scaffold command | None |
| SPEC S:2540-2559 (A:81, 1908-1915) | Cycle cut: DFS from each node in a fixed order; cut one edge per closed cycle; restart | Greedy feedback-arc removal by DFS back-edges (`internal/index/dependspath.go:86-188`) | Restarts after every cut; preference rule (cut a `supports`→analysis edge first). Restart: R (S:2558-2559, determinism). Preference: N. Not a minimal feedback arc set: N |

### 3e. Scoring (§6 Scoring, A:655-1078)

| Ref | What | Pattern | Departure |
|---|---|---|---|
| A:666-684 | Runner: enumerate items, launch ≤N, parse, price, hold results, land at end | Bounded worker pool over a job list; batch commit at the end | Landing happens at walk end, not per item. R: I11 rule only (A:34). N on why |
| A:685-695 | `Ask` seam hides the provider | Dependency injection / port | None |
| A:697-715 | Three templates, shared fragments, test that each names exactly its class's ids and no weights | Templates plus lint. **Novel content:** the criteria. Hiding weights from the model is a deliberate blinding. R: A:708-711 | None |
| A:717-744 | Closed text: wrap at 100, numbered gutter; guard refuses scoring tokens and frame forgeries | Line-numbered context for grounded citation; delimiter-based prompt-injection guard | Refuse rather than escape. R: A:741-743 |
| A:745-768 | Parse: closed id set, line refs resolved to passages, duplicates/overlaps | Structured-output validation against a closed grammar | Grammar frozen per format version. R: S:1205-1212 |
| S:1214-1223, `faults.go:172-189` | Price = max − Σ weights | **NOVEL** (§1 core a). The mechanics are a checklist-deduction rubric; the vocabulary and weights are the novelty | Absolute count bias acknowledged. R: `faults.go:126-130` |
| A:770-791 | Scoring set; dirty check = stored judged hash vs hash of (template digest, statements) | Content-addressed staleness check, as in an incremental build keyed on an input hash (`internal/asks/judged.go:17-25,61-68`) | None |
| A:793-807 | Skip manual; `Scoring` vs `BearingWalk` predicates | Filtered work set | `founds` carries a bearing but is never asked. R: A:805-807 |
| A:808-847 | Run ledger: `run.yaml`, `items.jsonl`, append-only `ledger.jsonl`, last line wins, torn-tail trim, resume | **Append-only job log with resume** (at-least-once with idempotent write) | Line appended after the write. R: A:813-815. `ask_sha256` recorded but gates nothing. R: A:844-847, S:1344 |
| A:849-873 | `captures.jsonl` | Request/response capture for replay | None |
| A:882-901 | Run retention | Keep-last-N | None |
| A:903-944 | Landing: under lock, reopen, drop gone or changed items by hash, one batch, re-issue minus refused entries | Optimistic concurrency (hash/ETag compare-and-swap) plus partial-batch retry | None |
| A:945-964 | Re-price: recompute landed values from recorded ids under the current table | Projection rebuild from stored raw evidence (event-sourcing replay) | Lines without `judged` are left as they are. R: A:950-951 |
| A:966-1005 | `score.Invoke`, verb, MCP detached child | Command orchestration; detached background job | Self-exec child. R: A:1001-1003 |
| A:1006-1078 | Score dev replay | Offline prompt-regression harness | Build-tagged. R: A:1017 |

### 3f. Inference seams, config, credentials (§7, A:1082-1317)

| Ref | What | Pattern | Departure |
|---|---|---|---|
| A:1084-1093, 2069 | Three roles, each its own endpoint/model; concurrency default 4 | Role-based model routing from config | No fallback between roles. N |
| A:1094-1116 | `probe` | Health-check / smoke call | None |
| A:1117-1135 | Two wire clients (OpenAI, Anthropic Messages), one sampling type, keep-alive pool | Provider adapters behind one interface; connection pooling | None |
| A:1136-1148 | Thinking controls; Messages `max_tokens = budget + max(256, ceil(chars/4))` | Vendor parameter mapping | The `chars/4` allowance has no reason given. N |
| A:1149-1168, 2067 | 3 attempts; retry 429/5xx/transport; backoff 5 s/30 s; outcome taxonomy | Retry with backoff on retryable status | Build multichoice asks retry with no pause (A:2067). N |
| A:1156-1160 | `prompt_cache_key` = run id | Provider prompt-cache affinity | None |
| A:1172-1181 | 401/403 classified once, carried as data | Typed error classification at the boundary | None |
| A:1182-1214 | Layered config: explicit/env over repo over global; provider table replaced whole, `config.toml` fields overlaid | Layered configuration (git-config/XDG style) | Mixed merge granularity. N in A (may be S §9 Merging) |
| A:1215-1240, 1773-1785 | Credentials store: AES-GCM under a key HKDF derives from a source constant + username; `secret.Key` redacts | Local secrets file (0600) plus a redacting secret type | A itself calls this "obfuscation, not a boundary" (A:1776-1777). No reason for not using the OS keychain. N |
| A:1241-1270 | Pure resolution; one client registry keyed by endpoint+protocol+sampling | Factory / registry with lazy, shared clients | None |
| A:1271-1287 | Decision surface: closed-choice structured answers (`systemone` / `decisions`) | Constrained classification endpoint (structured output) | The decision role is always an OpenAI client. R: A:1283-1286 |
| A:1288-1313 | `claimgraph` reaches models only through an injected seam; import test | Dependency injection plus an architecture fitness test | None |

### 3g. Surfaces, packaging, constants (§9-§12, §14)

| Ref | What | Pattern | Departure |
|---|---|---|---|
| §10 A:1592-1606 | `just test-integration` members over fixtures | Black-box integration tests against the built binary, with golden fixtures | None |
| §11 A:1610-1873 | Package map | Package responsibility listing | Entries restate mechanism (the `kb` entry runs 31 lines, A:1648-1678). No framing |
| A:1875-1922 | Loader: migration record, strict load, integrity faults, rigor ceiling | Schema validation at load, fail-closed (no degraded mode) | Cycles are refused at load but tolerated inside one build window. R: A:1911-1915 |
| A:1938-1945 | Zero-value hazard | Go zero-value pitfall; required-field decoding | R: in place |
| A:1947-2055 | Dependency rules plus `importpolicy_test.go`, `readpolicy_test.go`, `revealpolicy_test.go` | Layered architecture enforced by fitness tests (ArchUnit-style) | None. The pattern is never named, so 110 lines read as bespoke rules |
| A:1618-1630, 1631-1642 | `argmark`: text names an argument by a marker; each surface spells it | Late-bound, per-surface rendering of identifiers (presentation layer / message templating) | No standard framing given. The departure is the mechanism's existence; the reason is surface-neutral recorded text (A:1569-1576), only partially stated |
| §12 A:2059-2093 | Constants table | Config constants registry | None |
| A:2095-2181 | Format migration: N→N+1 pure converters, golden pairs, frozen per-version packages, ordered save, `git mv` of records | Chained schema migrations (upcasters) with golden tests; crash-resumable ordered save | Frozen packages with no outward imports. R: A:1967-1968. Save order. R: A:2120-2126 |
| A:2183-2348 | MCP: tool table from annotations, families, JSON-RPC stdio, detached build/score children, polling instead of notifications | MCP server generated from a CLI command tree; JSON-RPC over stdio; daemonized child plus status polling | No notifications. R: A:2345-2347 (S §11) |
| §14 A:2383-2422 | Limits | Known-limitations section | Merge conflicts refused, recovered via git. R: A:2407-2411 |

### 3h. Material with no standard framing that should have one

| Location | Currently framed as | Name it as |
|---|---|---|
| A:432, 440-475 | Bespoke batch phases ("shape", "apply", "derive", "write") | Transactional unit of work with dry run; validate-all-then-apply; post-commit derivation of materialized views; per-file atomic rename, not a multi-file transaction |
| A:808-964 | "ledger", "walk", "landing", "re-price", "result cell" | Job runner with an append-only event log, resume from the last status per item, optimistic-concurrency commit, projection rebuild |
| A:1375-1407 | "operation lock", "holder lock", "starting" windows | Repo-scoped mutex lockfile plus a pidfile-style holder record; standard lockfile race windows |
| A:2095-2181 | Converter prose, line by line | Schema migration chain with read-time upcast and an explicit `migrate` save |
| A:33-35 (I10-I12) | Invariants in kbase idiom | Record/replay; fan-out/fan-in single-writer merge; counting semaphore with a capability token |
| A:1440-1443, 1294 | "answer cache" | Content-addressed memoization (the key includes the attempt ordinal) |
| A:770-791 | "judged hash", "dirty" | Input-hash staleness, the same mechanism as make or Bazel action keys |
| A:717-744 | "closed text and its guard" | Delimiter-based prompt-injection defense; line-numbered grounding |
| A:79-80 | Stage prose | Blocking + pairwise classification; near-duplicate clustering with a canonical representative |
| A:1947-2055 | Dependency rules | Architecture fitness functions |
| A:1618-1642 | `argmark` / `Spelled` / `Verbatim` | Per-surface presentation layer |
| A:190-424 | Export design | Pandoc multi-file conversion with a Lua filter; manifest-owned output directory |
| A:2352-2379 | Slice verdicts | Spike result / ADR. Move to a decision record if it does not steer current mechanism |
