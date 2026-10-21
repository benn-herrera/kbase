# Intersprint review — the scoring driver sprint

**Range.** `git diff main...HEAD` on `kb-score`: Phase 2 (`kbase score`, the scoring loop) and Phase 4
(the inference asking rework, the harness back end and the rigour-and-bearing rubric).

**Seats.** Structural (architect), security, contract text (tech-writer). Each reviewed the range
independently and reported graded findings; nothing was modified by a reviewer.

**How to read a row.** `id — grade — where — fault — fix — rating`. A **rating** of `mechanical`
dispatches on `reaper/Qwen3.8-Flash-Next`; `expert` inherits the coordinator's model. Rows under
`## Rulings` need an owner's decision before they can be dispatched at all.

## Progress

| closed | by |
|---|---|
| B1, A-4 | `fd94dc7` — the `malformed` status and its count |
| B2, B3 | recorded `00f0cf4`, landed in the documents at `28bb57f` |
| T-1–T-3, T-5–T-13; A-3 | `28bb57f` — the mechanical documentation batch |
| A-1, A-12, A-13 | `1f17ee4` — with two corrections: `Reference.String` has production callers, and A-13's assertion could not have failed |
| S-6, S-10, S-11 | `7eb7bb9` |
| S-7 | `a5c473b` — sited in `HarnessArgv`, the only place that still holds a model |
| T-4, and `CONVENTIONS.md:52` after A-12 | one writer pass: the design's supersession and the two sentences the fixes falsified |

| S-1, S-2, A-15 | `5b9cfb5` |
| A-2, A-5–A-8, A-14, S-4, S-5 | `670e093` |
| A-9 | `e6a2d3d` — the renderers take the graph the run loaded; the index is read once |
| A-10 (not in that batch), A-11, A-16, the guard's patterns | `bb3e942` and the pool template's header still open |
| S-9 | closed as not a finding: credentials are key files, never inline |

| S-8 | `e7843d4` — `Profile.ReplacedLayer`, recorded where the lower entry drops out of the merge; the add-only line pinned byte-for-byte |
| A-10 | `a1ac27e` — the pool template names a reference's provider part |
| S-8's contract sentence | `01f878e` — §9.3, with the README and `USER_CONVENTIONS` citing it |

**No row is open.** Every finding from the three seats is fixed or recorded as refused with its reason.

## Rulings

**B1 — `malformed` as a status.** **Settled: drop it.** No ledger written before the charge-line
answer is a comparable the project needs, so nothing is kept for their sake: the constant, the
count and the result key go (`fd94dc7`). The SPEC lines T-2's writer would otherwise keep are the
writer's to remove.

**B2 — which verbs load configuration.** `SPEC.md:1155`, `SPEC.md:1175`, `USER_CONVENTIONS.md:259`,
`INTEGRATING_KBASE_VIA_MCP.md:39` and `README.md:442` say every verb loads the three files.
`loadVerbContext` reaches only `build`, `score`, `models`, `configure` and `show-config`; `verify`,
`refresh`, `stats`, `set-rigor`, `query` and `kbase mcp` load none. *Name the five verbs* in those
five sites, or *make every verb load* — the latter puts a broken configuration in front of `verify`
and the MCP server, which is the case the strict-load rule exists to report and the case a clone
would feel hardest.

**B3 — a repository-supplied key file.** **Settled: leave it, and state the trust.** A key file is
not confined by convention: one key is shared across inference applications, so a common location is
legitimate and kbase cannot tell a key from anything else on disk — `KBASE_API_KEY_FILE` has the same
reach. So the fix is disclosure, not confinement: a layer that supplies an endpoint supplies its key
path too, and trusting a cloned configuration means trusting both. One sentence in §9.4.

## Essential

| id | grade | where | fault | fix | rating |
|---|---|---|---|---|---|
| S-1 | essential | `internal/score/text.go:277-300`; `score-*.tmpl.md:20-26` | The corpus is fenced in twelve backticks and nothing checks the text for them: a paper carrying that line closes the frame and the paragraph after it sits outside the "do not obey the corpus" instruction — yielding `none` (the maximum, `faults: []`) or arbitrary instructions to a tool-enabled harness. The guard covers four token spellings only (`text.go:16`). | Refuse an assembled text carrying the frame's structure — a twelve-backtick run, and for `bearing` the `## Source:`/`## Target:` labels — through `scoreTokenRE`'s existing refusal. Not a content-adaptive fence: the render must stay a function of the template (criterion 51). | expert |
| S-2 | essential | `internal/score/text.go:132-141` | `endPassages` labels a node's title and every statement passage with the same end, so `locate` collapses them and **every** `rigor` charge records `passage: "title"`, whatever it cited; `passageStatement` has no production path, and the test builds a passage set the renderer never produces. | Give title and statement distinct ends and build the test's closed text through the renderer. | expert |
| S-3 | essential | `internal/config/providers.go:241-276` | See **B3**. | | — |
| A-1 | essential | `ARCHITECTURE.md:1065`; `cmd/importpolicy_test.go:31-120` | The document says the dependency rules are enforced by the import-policy test; the three this branch adds (`score` ⟂ `model`/`harness`, `config` ⟂ everything but `atomicfile`, `statedir`'s importers) have no rule. The boundary holds today by arrangement. | Add the three rules to `dependencyRules`. | expert |
| A-2 | essential | `SPEC.md:1036-1038`; `cmd/score.go:150-159` | "when a key is configured and the base URL is `http://` … one line on stderr" — `CleartextWarning` is called from `cmd/build.go:127` only, so a `score` run against a plain-HTTP lab endpoint sends the key and says nothing. | Warn in `score` as `build` does, or scope SPEC's sentence to `build` — the first is the promise the document makes. | expert |
| A-3 | essential | `CONVENTIONS.md:129-135` | The patch-bump rule describes a loader that chains a same-minor fixture; `internal/kbload/kbload.go:62-68` reads a `2.0.x` KB as current with no chain. Both fixtures still say `2.0.0`, which is harmless under the real loader and false under the rule. | Say what the loader does: an older patch is read as current and restamped on the next save, so a leg that pins bytes after a write is what moves. | mechanical |
| A-4 | essential | `SPEC.md:430`, `:878-880` | See **B1**. | | — |
| T-1 | essential | `SPEC.md:1155,1175`; `USER_CONVENTIONS.md:259`; `INTEGRATING_KBASE_VIA_MCP.md:39`; `README.md:442` | See **B2**. | | — |
| T-2 | essential | `SPEC.md:74`, `:1510`, `:1513` | The current format is stated as `2.0.0` while `SPEC.md:1261` and `README.md:327` say `2.0.1`; a `0.9.0` KB's `migration` record reads `2.0.1`. | Spell `2.0.1` at all three sites. | mechanical |
| T-3 | essential | `SPEC.md:1673-1675`; `INTEGRATING_KBASE_VIA_MCP.md:180-182` | The MCP server "loads the configuration every verb loads, harness profiles included, and uses none of them" — `cmd/mcp.go` loads no configuration at all; a broken `harnesses.toml` leaves it serving. | State that the server loads none, that the verbs it exposes load their own, and that the build child inherits the server's environment and `--config-dir`. Follows B2. | mechanical |
| T-4 | essential | `RIGOR_AND_BEARING_SCORING_DESIGN.md:237-246,483-484,849,868,916-921,995-996,1004` | The design's header invites a reader to treat it as the parse's authority; it states a three-word span floor and a refused shared span (both settled the other way), an open table identity, and a `refuted` ladder that no longer exists. | Mark it at its head as superseded by the scheme on the parse, the run's identity and the ladder, and put those sites in the past tense. | mechanical |

## Recommended

| id | grade | where | fault | fix | rating |
|---|---|---|---|---|---|
| S-4 | recommended | `cmd/score.go:435-445`; `internal/score/run.go:114-116` | A child's stderr (bounded at 64 KiB, arbitrary bytes) is carried verbatim into `failures[].detail` and a warn log line, so repository-chosen terminal control data reaches a document and a terminal. | One channel: the ledger line's `reply`. Anything that reaches a document or a log is bounded and stripped of control sequences. | expert |
| S-5 | recommended | `cmd/score.go:175,386-391` | `--resume` is joined into a path and that path feeds `os.RemoveAll`: `--resume ../../..` reaches a directory named `harness` outside the state store. Run ids kbase mints are `run-<hex>`. | Validate the value's shape before joining; a path kbase deletes recursively is one it created. | expert |
| A-5 | recommended | `cmd/score.go:515-527` | The printed resume command drops `--config-dir`, so a resumed run can resolve a different profile or reference under the same run id — against SPEC §8's own reason clause. | Carry the flag value and append it when set. | mechanical |
| A-6 | recommended | `internal/score/run.go:195-213`; `ledger.go:446-454` | An item's pass is derived three times (the node, the edge, the pass name), so a rigor run handed an edge item would price it as bearing silently. | Hold the pass once on the runner and derive both from it; delete `tableOf`. | expert |
| A-7 | recommended | `cmd/score.go:490-498`; `run.go:240-248`; `ledger.go:543-548` | An item's name is spelled three ways, so a `repriced.yaml` line does not join to `scored` without a translation. | One helper both records and log lines call. | mechanical |
| A-8 | recommended | `run.go:91-95`; `cmd/score.go:410-417` | The `--limit` cut is spelled twice ("as the runner cuts them"), and the two decide which items `cancelled` names. | Export the walked slice and have `cmd` read it. | expert |
| A-9 | recommended | `internal/score/text.go:241-265` | Every item re-reads the node and cite indexes, though the runner holds the index: ~2,000 index parses on a 2,000-claim KB, and a second reading path for a fact the runner owns. | Build the maps once in the run and pass them to the renderers. | expert |
| A-10 | recommended | `internal/config/template.go:36-37` | The `providers.toml` template still teaches "what `provider` in config.toml … name", the key SPEC calls superseded. | Name a reference's provider part instead. | mechanical |
| A-11 | recommended | `SPEC.md:838-843`; `internal/kb/faults.go:107` | A recorded id the running table no longer holds charges nothing, so a re-price can move a value *up* — stated only in a code comment. | One sentence beside SPEC §8's re-price rule. | mechanical |
| T-5 | recommended | `SPEC.md:787-813`; `USER_CONVENTIONS.md:203`; `README.md:20-21` | A `--harness` run refuses without `models.scoring`, though the harness section says it consults no pool and `USER_CONVENTIONS.md:203` exempts it from needing a provider. | State that the scoring role must name a model even where the profile carries no `{model}`, and qualify the other two sites. | mechanical |
| T-6 | recommended | `CONVENTIONS.md:279-287`; `PLAN_4_INFERENCE.md:1-30` | The plan-page rule requires the plan to name the page in its header; the plan that exemplifies it names it only in a table far below. | Name the scheme in the plan's header, or drop "in its own header" from the rule. | mechanical |
| T-7 | recommended | `SPEC.md:990`; `README.md:472` | "`config.toml` is created where absent on any `configure` run" — a run over an empty pool refuses before writing anything. | Name the condition: a run that resolves a provider, and `--create` with no `--provider`. | mechanical |

## Minor

| id | where | fault | rating |
|---|---|---|---|
| S-6 | `internal/harness/harness.go:67,127-128` | A relative `argv[0]` passes the setup probe in kbase's cwd and is executed in the per-item cwd: the probe certifies a program the run never starts. Resolve once, absolutely. | expert |
| S-7 | `internal/config/resolve.go:336-397` | A model id beginning with `-` becomes an option in the child's argv. Reject the form, or require it as an option value. | expert |
| S-8 | `cmd/provider.go:265-283` | A repository profile replacing a same-named user profile announces identically to adding one. Name the layer it displaced. | mechanical |
| S-9 | `cmd/show-config.go:149-158` | **Closed, not a finding.** kbase's credentials are key files, never inline; a profile's command is a harness CLI's argv, and that CLI carries its own credentials. Reporting the argv as written is correct, and §9.7's "names and paths, never secrets" is not contradicted. | — |
| S-10 | `internal/harness/harness.go:100-114` | `{prompt}` may appear twice inside one element (the two-element case is refused); the prompt then reaches the child twice. | expert |
| S-11 | `internal/harness/harness.go:121-125` | "Leaves nothing running behind it" holds only for a child that stays in its group; `setsid` escapes it, and on Windows only the direct process is killed. State the guarantee each host gives. | mechanical |
| A-12 | `internal/config/resolve.go:118-126`; `inference.go:26` | `EnvironmentReference`, `Reference.String` and `EnvironmentProvider` have no production caller. | mechanical |
| A-13 | `cmd/configure.go:199`; `SPEC.md:428` | `configure`'s dialed path omits `provider` where SPEC says it is null. | mechanical |
| A-14 | `cmd/score.go:217`; `internal/score/ledger.go:78` | A dry run's `run.yaml` carries `backend: ""`, which is not one of the three values SPEC names. | mechanical |
| A-15 | `internal/score/text.go:115-127` | The `## Source:`/`## Target:` labels sit inside the citable extent, so a reply can cite a label as evidence. | expert |
| A-16 | `internal/harness/harness.go:46` | `waitDelay` is missing from ARCHITECTURE §12's constant table. | mechanical |
| T-8 | `README.md:544-551` | The acknowledgements table omits `spf13/pflag`, a direct dependency. | mechanical |
| T-9 | `README.md:548` | The TOML decoder row names two of the three files. | mechanical |
| T-10 | `SPEC.md:920-921` | The created `[models]` values are said to be "filled in"; they are written blank for the user to fill. | mechanical |
| T-11 | `RIGOR_AND_BEARING_SCORING_SCHEME.md:377` | The ledger is cited to SPEC §7, which states no ledger line. | mechanical |
| T-12 | `README.md:296-298` | A `bounded` run carries no `failures`; the sentence does not say so. | mechanical |
| T-13 | `PLAN_4_INFERENCE.md:1647-1649` | The plan describes a `## Not settled` entry the scheme does not hold. | mechanical |

## What the review found sound

- The rubric's landing matches the scheme row by row: the ten malformed cases, `none`,
  `flt-pending`'s two spellings, the one-character floor, `both` at the earlier match, duplicate
  collapse, overlap refusal and the closed sets. The price is one table in integer hundredths whose
  reachable values are the scheme's eleven and six.
- The harness's exec surface: no shell on any path, literal substitution never re-scanned, NUL and
  length refused before a process starts, both streams bounded and drained, its own process group,
  the per-item directory emptied, nothing started before the KB opens, and one production caller.
- SPEC §8's `score` section: every testable claim re-ran against the binary and matched — flags,
  refusals, closing records, `resume`'s placement and content, the re-price, the dry run.
- The fifteen invariants each have code that enforces them, with A-1's three exceptions.
