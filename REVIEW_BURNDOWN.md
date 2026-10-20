# Inter-sprint review burndown

Closing the `2.0.0` landing and the dist pivot on `kb-2.0.0-format`. Reviews were taken at tip
`bfa7bdc`; the branch's merge-base with `main` is `dccb5cf`. Three reports: structural
(`.opencode-temp/review-architect.md`), security and contract drift (both reproduced in the
transcripts; every item this plan acts on is restated here, so no item depends on reading them).
This document is the triage and the work orders. It was reviewed by the prompt-engineer seat;
where its findings changed an anchor or an acceptance, the text below is the corrected one.

A finding is in scope when it cites a written rule, whatever its label. Items closed by an owner
ruling are recorded once, with the ruling, and are not re-opened.

## Rulings

- **The installer's trust anchor is the delivery of the installer.** The security review's blocking
  items 1 and 2 propose a fingerprint pinned inside the script; a hostile delivery moves the pin
  with it, so it adds nothing. No code change. The prompt, `y`/`o`/`N` and the non-terminal
  instructions stand. Anything requiring a pinned fingerprint is out of scope.
- **A lift stands over a pending derivation.** Design §2.3: "`max(pending, lift)` is the lift".
  `internal/index/strength.go:242-267` is right; `SPEC.md:1008`, `README.md:184` and
  `STRENGTH_SCORING_DIAGRAM.md:115` are stale. WB5 replaces the three sentences; W1 proves with a
  probe that the lift escapes a pending rigor in **both** arms (claim and analysis) before the
  replacement sentence is written.
- **An experiment may target an analysis.** `BearingLegal`'s supports arm
  (`internal/kb/schema.go:236-238`) is right. The design is internally inconsistent: §1's table
  (`:44`) and §2.5's index table (`:612`) permit it; §2.2's table (`:244`) and its "Why analysis
  targets exist" paragraph (`:268-274`) forbid it. The design owner amends §2.2; no code change.
- **A KB document that is a symlink is refused, not followed.** Reads skip it
  (`internal/kbload/kbload.go:130-133`), writes follow it (`internal/atomicfile/atomicfile.go:37-45`);
  the asymmetry is the vulnerability. Applies to reader, writer and converter. SPEC §10 and
  ARCHITECTURE §11 gain the refusal with the other loader faults.
- **A conversion may not drop bytes.** Where a legacy document holds content the converter cannot
  carry, the conversion refuses, naming the file and the content; it neither deletes the source nor
  invents a destination for the remainder.
- **`build`'s migration exemption stands.** The design exempts `build` (`:912-915`, I16) on the
  ground that a mid-build KB is not a finished state. `build` resuming a pre-upgrade trail and
  saving it (`internal/build/build.go:646-655`) is that case: it carries no record, and that is
  the ruling. No change; architect S5 is closed here.
- **`lifts` and `refutes` reach an analysis.** Evidence found after an analysis was written can
  raise or cap it, because what such evidence bears on is the conclusion the analysis states. The
  design's §2.2 table, §2.5 index table and §2.3 arithmetic now say so; the code follows in W1.

## Design amendments (coordinator, before W5)

The design is the authority and is outside a coder's scope; these are edited directly, under the
rulings above, before the doc pass reads it.

- §2.2 `:244` — drop "(an experiment never targets an analysis)"; `:268-274` — an experiment may
  source an analysis's premise.
- I14 `:1166-1169` — the obsolete set is `claim-quality.md` and `invariants.md`, not "two index
  files"; say which writer owns the entry point's stamp (the save, today
  `internal/index/refresh.go:438-445`, while the converter also rewrites it at
  `from_1_0_0.go:123-149`; W2 makes one of them the writer and the other the reader).
- If the open decision lands, §2.2 `:246-247` and §2.5 `:612` follow it.

## Work order

`W3 → W2 → W1 → W4 → W5`. W2's symlinked-register refusal depends on W3's reader rule; W5's text
describes rules W1–W3 land and the installer W4 rewrites. Each package: `just edit-gate` and the
`just test-pkg` commands listed. Each `just test-pkg` takes one package per invocation.

### W3 — no write follows a symlink out of the tree, and the read surfaces share one refusal

**Landed.** `14e3e2c`, follow-up `f6dc699`.

Security 3; architect S3, S4. `internal/atomicfile`, `internal/kb`, `internal/kbload`,
`internal/index` (save), `internal/query`, `cmd/render-claim-graph.go`.

- `atomicfile.Write` resolves symlinks and renames over the resolved target
  (`atomicfile.go:37-45`), reached from every KB write (`refresh.go:131-137`, `:398-458`, `:497`,
  `:516`) and so from `refresh`, `init`, every write op and MCP. Refuse when the path is a symbolic
  link, and refuse a resolved target outside the tree. The root is `kb.Source.Root()` threaded to
  the writer; the test is `filepath.Rel(root, resolved)` starting with `..`. Do not infer a root
  from `os.Getwd()`.
- The reader refuses a symlinked KB document in the same terms (`kbload.go:130-133` currently
  skips it silently), so a conversion cannot stamp a KB whose register it never saw.
- **S3.** `query.Load`'s backstop passes a null on a scoring row whose source id resolves to no
  node (`load.go:83-95`): `kinds[e.Source]` is `""`, `BearingLegal` is false, the row is skipped.
  I13 requires null only where an edge cannot carry a bearing. Require both ends to resolve, or
  refuse an orphan scoring row with `check: index`.
- **S4.** `render-claim-graph` checks only that `.index/` exists (`cmd/render-claim-graph.go:45-51`)
  then reads it through the sheet; a converted-but-unsaved KB must refuse with `check: kb-format`
  and remedy `kbase refresh`, as `query.Load` does (`:35-40`).
- New check names: the symlink refusal and the conversion-content refusal (W2) get names ruled
  here — `symlink` and `conversion` — and W5 adds them to SPEC §7's enumeration.

Acceptance: a 2.0.0 KB whose `.index/claims.yaml` is a relative symlink outside the tree refuses on
`refresh`, target untouched; the same in a 1.0.0 tree refuses without stamping; an orphan scoring
row is refused by `query.Load`; `render-claim-graph` over a migrated KB refuses with the remedy.

Landed in this pass (see the branch). Two follow-ups ride on it:
- the **current-format** reader still follows a symlinked document (`internal/kb/source.go`
  `Files`/`DirFiles` via `IsFile`, which `os.Stat`s through the link); the ruling's refusal must
  reach it, not only the loader's conversion walk;
- a migrated KB is refused as `index` by `query.Load` and as `kb-format` by `render-claim-graph`;
  one state gets one name (`index`, with the `kbase refresh` remedy, is the one `query.Load`
  already uses).

```
just test-pkg ./internal/atomicfile
just test-pkg ./internal/index
just test-pkg ./internal/kb
just test-pkg ./internal/kbload
just test-pkg ./internal/query
just test-pkg ./cmd
```

### W2 — the converter is a fixed point, stays inside the KB, and drops no bytes

**Landed.** `df09c14`.

Architect B2, N3, N7; security 4, 5, 6. `internal/migrate`, `internal/kbload`, `internal/index`.

- `simplePremise` calls `insertBearing` unconditionally (`from_1_0_0.go:354-382`), so converting
  `2.0.0` bytes adds a second bearing and the reader refuses R5 (`register.go:502-505`). Skip when
  a bearing is present; do not split a single-target bullet that already carries one. Acceptance:
  `Chain("1.0.0","2.0.0", register-2.0.0) == register-2.0.0`, and a refresh that fails after a
  register write is re-openable and refreshable.
- **N3.** A single-target axiom bullet carrying `(applicability …)` converts to two bearings
  (`from_1_0_0.go:295-305`, `:354-363`); the fixed point covers the single-target case, and the
  split path (`:400-406`) must carry the token the same way or refuse it.
- The loader filters by suffix and regularity but never by `kb.ExcludeDirs` (`kb.go:80`), so the
  converter renames and deletes under `kb-root/tools/` and `kb-root/session/`. The converter's file
  set is the reader's file set.
- `invariants.md` is declared obsolete when a declaration is found (`from_1_0_0.go:527-543`) and
  `axiomsDocument` keeps only the two recognised shapes (`:556-603`), destroying the rest. Under
  the ruling: carry the declarations; if the file holds anything else, refuse and name it. When no
  declaration is found the file is already left alone — keep that.
- A symlinked register is skipped, then the entry point is stamped `2.0.0` with no register
  (`refresh.go:407-446`). W3's reader refusal makes it a refusal before any write; W2 asserts the
  conversion path.
- **N7.** Two writers for the entry point's `kb-format`: the converter (`from_1_0_0.go:123-149`) and
  the save (`refresh.go:438-445`). **The save owns the stamp** — the converter leaves the entry
  point as it found it, I14's text as written. Remove the converter's rewrite; the golden pair's
  expected entry point stays `1.0.0` until a save stamps it.

Acceptance: fixed point; no write or delete outside the reader's file set; a legacy `invariants.md`
with prose refuses and survives; a symlinked register refuses; one stamp writer.

```
just test-pkg ./internal/migrate
just test-pkg ./internal/kbload
just test-pkg ./internal/index
```

### W1 — a claim's refutation reaches the walk, the records and the sentences

**Landed.** `e641761`, with the readers' legality follow-up `e0f8a3e` and the write-side experiment
premise `b150de8`.

Architect B1, N1, N2, N6; S1, S2. `internal/index`, `internal/kb`, `internal/write`,
`internal/query`.

- **B1.** `ClaimEntry.Refutes` is parsed (`register.go:749-763`) and written, but `caps` come from
  analyses and experiments only (`strength.go:363-383`), claim rows emit premises, references and
  demoted only (`records.go:113-117`), and `AuthoredDependsGraph` adds premises only
  (`dependspath.go:26-28`). Design §2.2:247 and §2.4:430-431 require the claim refuter.
  Acceptance: a 0.30 refuter against a 0.80 target at bearing 1.0 yields 0.70; the edge appears in
  `edges.yaml`, `deps`, `DependentsOf`, `stats`, the sheet and the acyclicity check.
- **N2.** Three operator sentences omit the claim refuter: `README.md:232`,
  `internal/sheet/dot.go:125`, `internal/query/query.go:50`. (README is W5's file; W1 states the
  new sentence in its report, W5 lands it.)
- **S1.** `Axiom <label>` citations are not resolved at parse or write
  (`register.go:493-495`, `:521-526`; `values.go:376-392`; `ops.go:435-452`), so one typo writes a
  KB that `refresh` refuses with a detail-only `metadata` item. Design §2.5:579-582 says a token
  resolving to no node is a logged drop; resolve at `Discover` and at `checkPremiseTarget`, and
  carry path, line and key in the coverage fault.
- **S2.** Leaf and analysis pair-line faults bypass `AnnotationFault` and carry no line
  (`leaf.go:189-205`, `register.go:840-855`), and `scorePairs` `continue`s on a list or bool value
  (`leaf.go:196-197`). Design §2.5:553-556 and A6 require check, key, path and line; route both
  pair readers through `AnnotationFault` and refuse an unreadable value.
- **N1.** The four-relation set and the score domain are spelled three and two times
  (`strength.go:123-130`, `query.go:60-68`, `values.go:39-41`, `:185-202`,
  `register.go:97-109`). I10 asks for one `ParseBearing`/`BearingText` pair; make the write API
  call `kb`'s predicate and `kb.Scoring(relation)`.
- **N6.** `WalkRegisters` reconciles staged vs declared lifts only (`discover.go:428-442`) while
  `pairHomes`/`set-bearing` treat a refute as having both homes (`ops.go:1016-1027`); a divergent
  staged refute is resolved silently and `analysis fan-out` misses it. Iterate both lists.
- **Legality and the analysis's final.** `BearingLegal` (`schema.go:236-249`) still sends `lifts`
  and `refutes` to claims only; the design now sends both to a claim or an analysis. The walk must
  publish an analysis's strength as `final` over its derivation with its own lifts and caps
  (`strength.go:242-267`, `:363-383`), the records and the acyclic check must carry the new rows,
  the sheet must draw the strokes into analyses, and `internal/kb/metadata_test.go:171` (the
  experiment→analysis case, already `true`) plus the `lifts`/`refutes` arms follow.
- **The write path still follows a link.** `internal/write/store.go:374` resolves a target through a
  symlink inside kb-root before `index.WriteKBFile` sees it, so a write op on a symlinked document
  still edits through the link; W3 refuses the resolved path, not the link. Refuse the link.
- **A write op over a symlinked KB fails instead of refusing.** `internal/write/ops.go:243` maps
  only `kb.AnnotationError`; a `*kb.SymlinkError` from `Discover` becomes a `failed` outcome. Map it
  to `check: symlink`, and make `ops.go:251`'s `--no-refresh` over a migrated KB use
  `index.CheckIndex` (it still says `kbload.CheckFormat`), so one state has one name.

```
just test-pkg ./internal/index
just test-pkg ./internal/kb
just test-pkg ./internal/write
just test-pkg ./internal/query
```

### W4 — the dist test can fail for what it claims, and the release path is re-runnable

Security 7, 8, 9, 10, 12; architect N4, N5; tech-writer N2.

- The stub `gpg --verify` exits 0 whenever both files exist (`testdata/stubs/gpg`), so no case
  would notice a tampered tarball, a tampered binary, a mismatched `.asc` or a signature by a
  different key. Make the stub verify what it signed and fail otherwise; add cases for a tampered
  tarball, a `.asc` for another file, and a signature by a key absent from the keyring (that last
  is the installer's existing refusal at `build-dist.sh:88-91` — **not** a pinned-fingerprint
  check, which the ruling excludes), each asserting refusal before extraction and before `cp`.
- `curl` always fails, so the download branch never runs. Make it able to succeed and to 404; add
  `--fail --proto '=https' --max-time` to the installer's fetches (`build-dist.sh:79`, `:123-124`)
  and a case that a 404 body is not used as a `.asc`.
- `stage_release` removes the installer `.sh` but not its `.asc` (`build-dist.sh:38`), and the
  signing has no `--yes`; a second `just dist` at one version fails at `sign_installer`. Remove
  the `.asc` with the rest and sign with `--yes`.
- The version string is interpolated into shell text and `rm -rf` unvalidated (`build-dist.sh:29-38`).
  Enforce `^v[0-9]+\.[0-9]+\.[0-9]+$` before use; a mismatch refuses (no existing check name — use
  `usage`).
- `SIGNING_KEY` is a uid (`build-dist.sh:9`). Sign by fingerprint in `build-dist.sh`, and state the
  fingerprint where the recipe documents itself. This is a build-side constant, not the installer
  pin.
- **N4.** The release target triple is spelled four times (`build-dist.sh:20-26`, `:45-50`, `:66-74`,
  `:128-130`); one `targets` table consumed by all four.
- **N5.** `test-dist` skips the three pty cases when `expect` is absent (`test-build-dist.sh:33-34`,
  `:559-564`); make `expect` a documented dependency or fail instead of skipping the trust cases.
- **Golden.** `tools/scripts/testdata/installer-v2.0.0.sh` pins the installer byte for byte
  (`test-build-dist.sh:203-209`); the `curl` flags change those bytes. No recipe regenerates it —
  surface that gap in the commit rather than `cp`ing blind, and show the golden's diff.

```
just test-dist
just edit-gate
```

### W5 — the contract text catches up

Tech-writer: blocking B1, B2; should-fix S1–S12; notes N1, N3–N11. One writer, no code.

Blocking: `just dist` runs no checkpoint and never did — the gate is a precondition
(`CONVENTIONS.md:24-26`, `README.md:380`; copy the recipe's own sentence, `justfile:1988-1990`);
a lift over a pending derivation publishes a number (`SPEC.md:1008`, `README.md:184`,
`STRENGTH_SCORING_DIAGRAM.md:115`) — wording from W1's probe.

Should-fix, each with its anchor: `USER_CONVENTIONS.md:16-22` gains the `migration` key after
`kb-root`; `SPEC.md:1086` says the converter leaves every file byte for byte, but it removes
`claim-quality.md` and `invariants.md` — say them; `CONVENTIONS.md:142` names `kb.RelationDepends`
and `ARCHITECTURE.md:903` names `kb.ReadFormatVersion`, neither exists; `ARCHITECTURE.md:714-718`
says `test-integration` has four members, it has five, and `test-dist` runs against stubs, not the
binary; SPEC §7's `check` list (`:389`) gains `metadata`, `index`, `symlink`, `conversion`;
`README.md:78` says `init` runs a `verify`, it does not (SPEC §8 already says so); a sheet is
rewritten when its bytes change (`USER_CONVENTIONS.md:151`); an older-format KB answers no query
(`INTEGRATING_KBASE_VIA_MCP.md:47`, `README.md:219`); `render-citation` and `export-latex` join the
newer-format refusal list (`SPEC.md:919-921`); the sheet's `supports` row admits `experiment →
analysis` (`STRENGTH_SCORING_DIAGRAM.md:39`) — now matching the code, which the ruling confirms;
a README Install section covers the tarball, the installer, the `y/o/N` prompt and signature
failure modes.

Notes, one line each: N1 the artifact list gains `install-kbase-<ver>.sh.asc` (`justfile:1985`);
N3 SPEC `:958-959` cites ARCHITECTURE §10 for the golden pairs, they are in §12; N4 `- quality:`
renames only on an analysis entry and `Axiom N` needs no rewrite (`SPEC.md:1066`, `:1082`);
N5 `show` carries no edges and `referenced-by` returns paths (`SPEC.md:1150-1151`); N6 the node
record's keys are not exhaustive (`SPEC.md:417`); N7 `USER_CONVENTIONS` omits provider keys, the
`removed` key and four ops; N8 `build` is not a migration-record carrier (already stated; make the
ARCHITECTURE list agree, `:900-902`); N9 SPEC §5 and the README do not say a build writes no
judgement; N10 the MCP tool list omits `init` and `export-latex` (`README.md:272-274`); N11 the
template's sheet rule counts nodes, not claims (`CONVENTIONS.tmpl.md:89-90`).

Acceptance: for each edited sentence, the command whose output it now describes (`./bin/kbase
<verb> --help` or the run), recorded in the commit body; the WP6 census greps return only
historical 1.0.0 mentions.

```
just test-pkg ./internal/kbdocs
just edit-gate
```

## Out of scope, explicitly

- Installer pinned fingerprint (ruling 1); security items 1 and 2, and item 8's `cp`-into-preexisting-
  directory note beyond the `--fail`/`--proto` hardening in W4.
- Security 11 (`index` written before a later phase refuses) and 13 (unconfirmed packet-spoofing,
  WKD timing) — noted, not actioned.
- `build`'s migration record (ruling 6).
- `test_data` → `test-data` (deferred chore, `ACTIVE_PLAN.md`).

## Gates

The pass closes with `just test`, `just test-integration` and `just checkpoint` on a tree whose only
changes are the ones above.
