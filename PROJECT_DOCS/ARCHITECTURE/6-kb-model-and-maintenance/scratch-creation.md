### Scratch creation

**Pattern:** Scaffold command.

`cmd/init.go` is the only producer of a KB that is not the build. It composes existing mechanisms and
owns none of its own:

- the format stamp through `kb.EncodeFormatStamp`, the plain entry point, and the two empty record
  files through `kb.EncodeNodes` and `kb.EncodeEdges`, every file through `atomicfile.Write`;
- the loader's `Open` ([§2](../2-invariants.md), I8);
- the readiness documents through `kbdocs.Stamp` with `kbdocs.AuthoredKBPin` in its scope-pin slot, its
  `written` list being the paths `Stamp` returns;
- the derived views through `index.RefreshReporting`;
- the KB write lock through `holdKB`, which takes `write.LockKB` from before `kb-root/` exists
  ([§8](../8-ledger-state-store-and-monitoring/locks.md));
- the create commit through `gitledger.Repo.Commit`, the plain-subject half of `gitledger` beside
  `Record`, its pathspec scoped the same so nothing the user staged rides along.

It refuses a multi-line `--title`, a directory outside a worktree and any existing `kb-root/` before
any write, and decides create-once again under the lock (SPEC §8 `init`). Its order is load-bearing:
the entry point and the stamp file exist before `Open`, the readiness documents before the rebuild,
and the rebuild before the commit. The commit's subject, `kb-init: knowledge base created`, is outside
`gitledger.Prefix`, so the KB has no checkpoint commits and a resume finds nothing to resume.

`init` runs no `verify` of its own: every input to the scaffold is kbase's, so a failing `verify` over
it is a kbase defect, which `test-integration-init` catches by running `verify` right after creation.

