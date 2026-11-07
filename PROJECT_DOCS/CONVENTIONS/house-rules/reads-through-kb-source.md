### Reads through `kb.Source`

Every KB file is read through `kb.Source` from `kbload.Open` (ARCHITECTURE I8). Pitfall: an
`os.ReadFile` of a KB file in a verb or package, which sees the on-disk form of an unconverted KB.
The rule covers file reads only: a directory listing of `kb-root/` (`os.ReadDir` in
`internal/index/refresh.go` and `internal/kb/deadlinks.go`) sees the disk, not the source, and
nothing checks it.
