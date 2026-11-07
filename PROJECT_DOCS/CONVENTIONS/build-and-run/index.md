## Build and Run

Run `just` (the default recipe) for the up-to-date recipe list with docstrings — that output, not
this file, is the reference for available actions. Anything LIVE (provider + network) is excluded
from the hermetic `just test-integration` aggregate target and says so in its docstring.

- edit gate: `just edit-gate`
- checkpoint gate: `just checkpoint`

kbase is distributed through the Homebrew tap `benn-herrera/kbase`, whose formula is maintained in
the sibling clone `homebrew-kbase/` (`just tap`). The formula builds a tagged release from source
with no build tag, so what users receive is the release shape `test-dist-no-replay` vets.
`just tag` tags a release: refused while tracked files have uncommitted changes, it creates a signed
annotated tag `v<version>` locally and pushes it nowhere; the tree is vetted before it, by
`checkpoint` and the integration gates, and the tag runs none. After the release is pushed,
`just tap-update` points the formula at its tarball and commits the change in the tap where the formula changed, then raises
`go.mod`'s `go` line where it has fallen out of Go's supported window and leaves `go.mod`
uncommitted. Windows has no distributed binary; it builds from source. `just tap-local` reinstalls this
machine's brew kbase from a scratch formula over a tarball of the working tree's tracked files,
leaving the tap untouched.

Contents:

- [No built binaries live in the repo](no-built-binaries-live-in-the-repo.md)
- [Test data layout](test-data-layout.md)
- [A recipe's script lives in `tools/scripts/`](a-recipe-s-script-lives-in-tools-scripts.md)
