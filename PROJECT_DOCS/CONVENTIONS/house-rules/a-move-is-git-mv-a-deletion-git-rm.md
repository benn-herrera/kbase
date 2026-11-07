### A move is `git mv`, a deletion `git rm`

Moving a file is done with `git mv` and deleting one with `git rm`, because the edit and write tools
govern text changes, not filesystem operations. Staging either is part of the change's commit; the
change is committed once.
