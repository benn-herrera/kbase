### Write whitespace in a bracket as `PyWhitespace`

Inside a bracket expression, write whitespace as `PyWhitespace`, never `\s`. `PyRE` expands `\s`
into a bracket expression of its own, and nested inside `[...]` that compiles, without error, into
a different pattern. Both are `format40`'s; outside `internal/formats` use `kb`'s re-exports, and a
superseded format package keeps its own copy, as each superseded format does.
