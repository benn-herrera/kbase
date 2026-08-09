# Community KB sharing — design capture

**Status:** post-v1 design note, captured from working discussion 2026-08-08.
Nothing here is scheduled; the appliance is complete without it. When this
feature is built, this document disperses into ARCHITECTURE.md / SPEC.md per
the house doc taxonomy.

## Concept

The gpuinfo.org model applied to KB artifacts: users who convert a
documentation corpus may **voluntarily** upload the resulting KB to a shared
cloud database with a web front end; kbase may **check the database before
building** and offer an existing conversion's entry-point URL instead of
processing locally. The economics mirror gpuinfo exactly — the expensive step
(distilling millions of tokens) is duplicated across users for popular corpora
(e.g. Roblox creator-docs) and collapses to a cache hit.

Why the existing design is unusually ready for this:

- The provenance stamp (`app version + resolved model IDs + source identity`)
  is already the natural database key, and reproducibility makes shared
  artifacts verifiable rather than trust-me blobs.
- BYOM consumption means one shared KB serves every consumer regardless of
  their model.

## Decided posture (converged 2026-08-08)

- **Strictly optional; no PID collected.** Ever.
- **Contact posture: default-off, opt-in at `configure`** (recorded in
  config.toml), because even a PID-free lookup transmits a corpus fingerprint
  — it reveals what material someone is processing. Per-run overrides in both
  directions: `--check-shared` and `--local-only`. `--local-only` is the
  unconditional "hell with your cloud, just do my stuff" switch: zero contact
  with the share service regardless of config.
- **Failure posture: monotone safety, verbatim.** The service being slow,
  down, or dead-forever degrades to "build locally, one log line" — never
  blocks, never errors. Consequence: an abandoned share service leaves the
  appliance exactly as functional as it is today; the service is a
  convenience layer, never a dependency.
- **Relationship framing** (why opt-in matters here when a cloud inference
  provider is already allowed): the inference endpoint is a relationship the
  user configures with a party they chose; the share service is
  infrastructure this project operates, contacted on the appliance's own
  initiative. The second kind requires explicit consent; the first already
  has it.

## Corpus identity & versioning (the TBD, partially noodled)

A straight content hash identifies an exact snapshot but cannot relate two
versions of the same material. Approach:

- **Git-sourced corpora solve themselves** (and are the dominant case —
  both validation targets are repos): normalized remote URL = material
  identity; commit = version; commit date/ancestry = lineage and
  latest-detection.
- **Non-git fallback:** similarity fingerprinting over the survey artifact,
  which already exists per-corpus (per-file structure, sizes, hashes) —
  file-hash-set overlap distinguishes "same material, new version" from
  "different material" cheaply. Threshold and canonicalization TBD.
- DB entry: (material identity, version, provenance stamp, entry-point URL,
  source license).

## License gate (hard requirement)

Uploading a KB publishes a derivative work of the source corpus. The upload
path must record the source license in provenance and refuse (or at minimum
sternly warn on) upload for sources without a redistribution-permitting
license. CC-BY-class sources upload with attribution; arbitrary corpora do
not upload.

## Open questions (unresolved, in rough order of hardness)

1. **Trust in shared artifacts.** Provenance is self-reported; a shared KB is
   model output. Mitigations to evaluate: shipping routing-eval results with
   the entry, spot rebuild-verification (reproducibility makes cheating
   detectable in principle), submission signing, community flagging. None
   selected.
2. **Service commitment.** DB, web UI, hosting, moderation, abuse handling —
   an operating burden of a different kind than shipping a binary. The
   monotone-failure posture caps the blast radius but does not staff the
   service.
3. **Versioned-content UX**: how the check-first offer presents "a newer
   version of this corpus exists but only the older one is converted."
4. Non-git fingerprint canonicalization details.
