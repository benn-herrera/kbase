### Merged metadata is refused, not merged

Two branches' record files merge by text: a duplicate edge, a dangling edge or a cycle the merge
leaves is a load error, `check: metadata`, which every command that loads the KB refuses. No op
resolves it: the recovery is in git, returning the record files to a consistent commit, such as the
one before the merge, and reapplying the other side's changes through write ops
(SPEC §10 The load: every fault refused).

