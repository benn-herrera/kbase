### `update-format`

```
kbase update-format [--commit]
```

`update-format` converts a KB in an older format to the current one ([§10](../10-metadata-format-version/one-version-read-and-written.md)) and, whether or not the format changed, rebuilds every summary page and claim-graph sheet ([§3](../3-compatibility-contract/index.md)) and rewrites the maintained blocks ([§3](../3-compatibility-contract/stamped-documents.md)). It runs alone, over a clean `kb-root/` and clean build records the conversion relocates, so that any problem is purely the conversion's and restoring `kb-root/` and any moved build record from `HEAD` reverses it. While a conversion is pending every other write is refused ([§10](../10-metadata-format-version/one-version-read-and-written.md)).

- It writes each file only where its bytes change, and removes a page or a sheet the KB no longer calls for. Without `dot`, the rule defined in [§3](../3-compatibility-contract/index.md) applies.
- Its result has no `warnings` key of its own: a conversion's warnings are in `migration.warnings`.
- Every run replaces each bare id — a node id `verify` reports as `bare-id` ([§8](verify.md)) — with that node's citation in the naming form ([§10](../10-metadata-format-version/citations.md)), every occurrence on a line; where the id is a link's whole text, the whole link is replaced by the naming form. A heading's anchor the citation moves is re-pointed as `add-citation` re-points it ([§8](add-citation.md)). A work's and an axiom's bare ids are cited like any other.
  - One exception: a node with no statement marker has no place a citation can point at, so its bare id stays. That node's `statement-marker` fault ([§8](verify.md)) is what clears it; the next run cites the id.
- The move of the build records is made once the KB loads, before the first write, so a KB whose load fails moves no record. Each moved record is reported twice: its root path, repository-relative, in `removed`, and its new path, kb-root-relative, in `written`.
- `--commit` (`commit`): without it `commits` is `[]` and the `git mv` of each build record is staged. With it, a run that writes or removes anything commits the conversion as one commit, labelled `kb-write: update-format <from> → <to>` (the format before and after, the same version twice where it did not change), holding `kb-root/` and each moved record's root path. Its clean-tree precondition makes that commit exactly the conversion; an `unchanged` run commits nothing. A commit that fails leaves the conversion written, outcome `done`, with `uncommitted` saying why, as a write batch's does ([§8](write-batches/index.md)).
- `bare-ids` lists every bare id the run cited, the conversion's included, and is present on every `done` and `unchanged`, `[]` where none. Where the format changed, the result also lists the conversion's minted work and axiom ids, dropped `no-edge` texts and dropped `strengthen-by` texts, under `ids`, `no-edge` and `strengthen-by`, only on `done` and `unchanged`. On a refusal or a failure none of the four is listed; the conversion's warnings stay in `migration.warnings` and on stderr.

**Refusals**, each before anything is written:
- outside a git worktree, or where git is redirected (`check: worktree`, [§8](write-batches/index.md)), and a repository with no `kb-root/` (`check: kb-root`);
- `kb-root/`, or a build record the conversion would move, with uncommitted changes (`check: dirty-paths`, naming each path; remedy: commit or discard them). The clean check covers `kb-root/` and those records at the repository root, and nothing else outside `kb-root/` ([§5](../5-build-behaviour/output-and-placement.md));
- a lock: another writer holding the KB write lock past 30 s is `retry`; a build or a scoring run holding the run lock is `refused` at once (`check: lock`, [One write lock](write-batches/one-write-lock.md));
- a KB newer than kbase (`check: kb-format`, remedy "update kbase", [§10](../10-metadata-format-version/one-version-read-and-written.md));
- a load error, naming each fault with its file and record (`check: metadata`), a missing record file included (`check: index`, remedy "restore it from git"); `update-format` repairs none ([§10](../10-metadata-format-version/the-load-every-fault-refused.md));
- a conversion that cannot convert a file's content, naming the file and line (`check: conversion`);
- a bare id whose naming-form citation would not read as one link, its node's title holding an unbalanced `[` or `]`: at `id`, naming the node and the line (`path`, `line`); remedy: give that node a title whose brackets balance (`edit-<kind>`, [§8](edit-kind.md)), then re-run.

Result keys: [§7](../7-tool-result-contract-and-exit-codes/index.md).

