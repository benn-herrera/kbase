### 3.4 Queries

Every query reads the KB and writes nothing. It takes no lock. An older-format KB answers as
converted ([§2.6](../../2-conventions/2.6-format-conversion.md)), and a load error refuses it ([§2.4](../../2-conventions/2.4-load-errors.md)). Every query's data sits under `results`: a
list with `count` for the list queries, a mapping for `show` and `stats`.

Ops:

- [`find` (query op)](find.md)
- [`show` (query op)](show.md)
- [`edges` (query op)](edges.md)
- [`trace` (query op)](trace.md)
- [`stats` (query op)](stats.md)
