### State store layout and format

**Pattern:** Versioned on-disk schema with sequential migrations.

The state store's contents and path are SPEC §6 The state store is versioned and SPEC §6 Git holds the
recoverable record. `--state-dir` defaults to `statedir.Dir`: `statedir.Key` under `statedir.Base`
([§12](../12-constants/index.md)). The layout:

- `format`, and `lock`, the run lock's second file, with its holder record ([Locks](locks.md));
- `progress.jsonl`, and the rotated `progress-<UTC timestamp>.jsonl` files;
- `records/`: the document graph's records and the pre-pass count report;
- `reports/`: `<stage-id>.yaml`, each stage's records and findings, named by the result's
  `stages[].report`, and the captures of a build or score run the MCP server starts
  (`<kind>-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`, `<kind>` being `build` or
  `score`);
- `scratch/trails/<start-commit>/`, one directory per build, holding `answers/`; `asks/`, one file per
  stage, `<kind>.yaml`, one YAML document per group (the group's shared template inputs once, each
  item's own inputs, its `prompt-sha256` from `asks.AskDigest`, its outcome and its calls), written by
  a stage whose wave stopped as well as by one that finished; `captures/`, every call's request, reply
  and usage; and `replays/<replay-id>/`;
- `scratch/regenerate/`, where a resume rebuilds the records;
- `score/<run>/`, a scoring run's directory ([§6](../6-kb-model-and-maintenance/scoring/the-ledger-and-the-state-directory.md)).

`<state-dir>/format` is one integer, `buildstore.StoreFormat`, written when a build first takes the
run lock (`readyStore`); an absent file is format 1. `build`, `status`, `wait`, `cancel`, `build-trail`
and the MCP `build` tool read it at open (`MonitorOptions.Open`, `walk.open`), and a store newer than
`StoreFormat` is refused (`buildstore.StoreVersion`; `check: state-dir`, `key` the marked `state-dir`
flag, `path` the store, SPEC §7 Item). A build converts an older store in place, in `readyStore`,
through `storeUpgrades` in `internal/build`: one step per version, each writing `format` when it
completes, so an interrupted upgrade resumes at the step it stopped at, and a gap in the chain is an
error. The chain is independent of the KB's `formats.Chain` ([§12](../12-constants/format-migration-spec-10/index.md)). At
format 1 `storeUpgrades` has no steps.

