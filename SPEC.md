# Kbase SPEC

Externally observable behaviors, contracts, and obligations — the *what*.
[ARCHITECTURE.md](ARCHITECTURE.md) is the authoritative design reference (the
*how*) and the single source for mechanisms and tuning constants.

---

## 1. Configuration / UX

- **Single model family, pinned: the appliance operates against gemma-4-family
  models.** Auto-detection selects only gemma-4 models, and the default pipeline
  is pure gemma-4; an out-of-family model runs a stage only via explicit manual
  override, never silently. (Rationale and tier mapping: ARCHITECTURE.md §3,
  §10. Generated KBs are consumed BYOM — §3, §5.)
- Required config: provider API URL + key.
- **Model auto-detection:** on configure, the app discovers available models from
  the provider and auto-selects gemma-4 family models (max-convenience default).
  Matching must be tolerant of provider naming variance (`google/gemma-4-31b-it`,
  `gemma4:31b-a4b`, …).
- **Fail loud and list** on ambiguity or no-match: "no gemma-4 family detected; found
  these; use `--model-map` to assign." Never silent best-guess — a wrong tier mapping
  is exactly the silent-platform failure mode.
- Manual explicit config (`--model-map`, per-stage overrides) always allowed.
- Resolved model IDs (auto or manual) are stamped into the provenance receipt (§7).

---

## 2. Prompts / agent definitions

- The app's operative agent/prompt definitions are immutable and fully determined
  by the app version; they cannot be modified or overridden at runtime.
- A CLI dump command writes copies to local storage for adaptation, each stamped
  with: app version, a header stating "this is a copy for adaptation; the app does
  not read this file," and the adaptation-guidance note (including the
  recommendation to have a stronger model write a version suited to its
  capabilities).

---

## 3. Generated-KB contract

- Entry-point AGENTS.md states, minimally: **summaries route; leaves answer** (answer
  from leaf text, never index summaries); the annex convention for any raw-lookup
  territories; pointer to `.agents/`.
- `.agents/` directory ships in every KB: docent definition, maintainer definition,
  adaptation note, routing-eval question set.
- **Annex convention** for machine-shaped material (e.g. creator-docs `reference/`):
  such territories are *not* distilled into leaves. The entry point documents the
  lookup convention instead — path grammar, file format, one worked example ("engine
  API classes at `reference/engine/classes/<ClassName>.yaml`; grep there directly").
  A deterministic lookup shim is built only if usage shows agents repeatedly fumbling
  the raw structure — signpost first, machinery on evidence.

---

## 4. Summary review

- Review output is **flags for regeneration only**; the reviewer never edits text,
  and regeneration is performed by the top tier (ARCHITECTURE.md §6 for method).
- Summaries are non-load-bearing for truth: a summary defect may misroute
  navigation but must never be the source of an answer (§3 contract).
- Residual risk is accepted, not solved: containment (routes/answers split) +
  routing eval + spot-checks.

---

## 5. Routing eval

- Each KB ships a generated Q→leaf question set; the eval verifies that
  summaries-only navigation reaches the right leaf.
- It ships in `.agents/` — **BYOM eval for a BYOM artifact**. The appliance stays
  single-family; consumers who want cross-family honesty run the eval with the
  model they brought. gemma-4 consumer results are a conservative lower bound
  (weak-navigator-succeeds is evidence stronger navigators will).
- Known bias, accepted: gemma-generated questions may skew toward gemma-natural
  phrasing; pairs derive from leaf content so skew is mild. Optionally salt with
  frontier-generated questions (outside the appliance).

---

## 6. Token counting

- Token counts are **estimates** (calibrated heuristic — ARCHITECTURE.md §8);
  no call requires an exact tokenizer, and estimator error must never affect
  output correctness. An undercount surfaces as a loud refuse-and-split, never
  silent truncation.
- The calibration source is stamped into the provenance receipt (§7).

---

## 7. Provenance receipt (per KB)

- App version (⇒ embedded prompt set), resolved model IDs per stage, source identity
  (commit hash / content hash), cut lists implied reproducible from source hash.
- Every KB page carries build provenance (`built from upstream <version/commit> @
  <date>`) — staleness is displayed, not solved; refresh/currency is explicitly a
  post-market-fit concern, out of v1 scope.
