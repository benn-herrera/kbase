### State store retention

**Pattern:** Keep-last-N retention.

Only a build or a scoring run prunes; `status` and `cancel` remove nothing. Counts are in
[§12](../12-constants/index.md).

- **Captured builds.** After the run event, a build keeps the newest `keptBuilds`+1 captured builds
  under `reports/` — the current build's capture, which exists before the child starts, and
  `keptBuilds` beside it, matching the build directories kept — and removes the rest (`pruneBuildCaptures`, `buildstore.PruneCaptures`). A capture is
  the `.stdout.yaml` and `.stderr.log` pair sharing a stem, ordered by name. It reads no progress record
  to do so.
- **Build directories.** Once its launch checks pass and before its first stage, a build prunes
  `scratch/trails/` (`pruneTrails`). It keeps the current build's directory, named by the commit of the
  newest `start` boundary, and the newest `keptBuilds` others by modification time. On a resume it
  creates the current directory if absent and sets its modification time to now; a fresh build's directory is created by its first model call, so its time is when it
  last gained a subdirectory. The rest are removed whole, with the undivided `scratch/answers/`,
  `scratch/asks/` and `scratch/captures/` directories. `status` computes `cache-entries` and
  `cache-bytes` over every build directory's `answers/`.
- **Progress.** At the same point, a fresh build whose `progress.jsonl` has more than
  `progressRotateLines` lines relocates every line before its own run event to `progress-<UTC timestamp>.jsonl`,
  leaves its run event alone in the record, and keeps the newest `keptProgressFiles` rotated files
  (`rotateProgress`). Nothing reads them. Stage reports are never pruned.
- **Scoring runs.** A scoring run prunes `score/` once its ledger has opened: of its kind (`run-` or
  `dry-`) it keeps the run it names and the newest `KeptRuns`-1 others by `run.yaml`'s start, and removes
  the rest whole. It keeps the newest `KeptRuns` captured runs under `reports/score-*`, by name
  (`buildstore.PruneCaptures`; [§6](../6-kb-model-and-maintenance/scoring/run-retention.md)).

