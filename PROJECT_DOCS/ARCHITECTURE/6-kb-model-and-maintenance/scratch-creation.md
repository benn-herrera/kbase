### Scratch creation

`cmd/init.go` is the only producer of a KB that is not the build. It composes
existing mechanisms and owns no new one: the format stamp through `kb.EncodeFormatStamp` and the
plain entry point,
the two empty record files through
`kb.EncodeNodes` and `kb.EncodeEdges`, every file through
`atomicfile.Write`, the loader's `Open` (I8), the readiness stamp through `kbdocs.Stamp` with
`kbdocs.AuthoredKBPin` in its scope-pin slot — its `written` list is the paths `Stamp` returns —
the derived layer through `index.RefreshReporting`, the lock through `write.LockKB`, and the birth
commit through `gitledger.Commit` — the plain-subject half of `gitledger` extracted beside `Record`, its
pathspec scoping the same so nothing the user staged rides along. Its order is load-bearing: the
entry point and the stamp file stand before `Open`, the stamps stand before the refresh, and the refresh 
stands before the commit. It runs no `verify` of its own: every input to the skeleton is kbase's,
so a red verify over it is a kbase defect, which `test-integration-init` catches by running 
`verify` right after birth.

