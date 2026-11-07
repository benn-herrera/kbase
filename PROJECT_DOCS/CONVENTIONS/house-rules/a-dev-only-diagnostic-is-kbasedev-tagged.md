### A dev-only diagnostic is `kbasedev`-tagged

A diagnostic for developing kbase itself, today `score --replay` and `build --replay`, lives in
files with `//go:build kbasedev`, code and tests alike; the untagged build is what users receive
and contains none of that code. A developer opts in with `-tags kbasedev` (`just build`). The
mechanism is in
[ARCHITECTURE, The development replay](../../ARCHITECTURE/6-kb-model-and-maintenance/scoring/the-development-replay.md)
and
[The build's development replay](../../ARCHITECTURE/8-ledger-state-store-and-monitoring/the-build-s-development-replay.md).

Pitfall: an untagged reference to tagged code, which breaks the default build, the one everyone
runs. `edit-gate` vets both tag settings, and `test-dist-no-replay` runs `TestDistBuildHasNoReplay`
(`cmd/dist_test.go`) on the untagged build.
