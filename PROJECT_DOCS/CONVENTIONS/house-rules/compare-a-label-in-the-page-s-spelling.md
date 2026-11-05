### Compare a label in the page's spelling

pandoc escapes `&<>"'` in attribute values, and
differently for a `Link` than for a `Span` or `Div`. A fact kbase records to be compared against
the page uses the escaped form the page uses; never unescape one side to match the other.
A label carried out of the page into another format — the LaTeX export — is HTML-decoded once, at
its read; this rule governs comparisons against the page.
- **In a pandoc Lua `topdown` traversal, what a filter returns in place of its element never meets the
  traversal's filters — not its own type's, not the enclosing list's — though its children are walked.**
  One element or a list, block or inline alike (pandoc 3.12.1). Pitfall: a filter that returns
  anything but nil or the element it was given — a `BlockQuote` filter returning the quote's body
  expects the `Figure` filter to see a figure in it, and it does not. Such a filter walks its output
  itself, as a list (`pandoc.Blocks{…}:walk(f)`, `pandoc.Inlines{…}:walk(f)`; an element's own
  `:walk` skips that element), and returns the result with `false` as the second value, or the
  descent walks those children twice.
- **Inside a bracket expression, write whitespace as `PyWhitespace`, never `\s`.** `PyRE` expands
  `\s` into a bracket expression of its own, and nested inside `[...]` that compiles, without
  error, into a different pattern. Both are `format400`'s; outside `internal/formats` use `kb`'s
  re-exports, and a superseded format package keeps its own copy, as `format200` does.
- **A float computation that decides a ranking sums in Python's order with its compensation**
  (`shortlist.go`'s `pySum`): Go's plain summation ranks a near-tie differently, and a differently
  ranked shortlist poses a different pair.
- **A `justfile` recipe whose body is a shebang script is a script under `tools/scripts/`, named for
  its recipe, and the recipe passes the script its inputs as environment variables.** The recipe line
  is `NAME="{{NAME}}" tools/scripts/<recipe>.sh`, so a command-line variable override reaches the script
  and no recipe re-invokes `just` for a private body. The script runs under its own
  `#!/usr/bin/env bash` with at least `set -u`, reads only what the recipe sets (the recipe's variables
  and parameters, and `just` built-ins such as `invocation_directory()` as variables of its own), and
  opens by refusing an unset one — `: "${NAME:?…}"`, with no apostrophe in the message, which the macOS
  `/bin/bash` 3.2 reads as an open quote. Scripts that need one another call each other by path, not
  through `just`. The one exception to environment inputs is a variadic list a script passes on
  untouched (the top `.tex` files and build flags of `test-integration-build-live-fixture`), which arrives
  as positional arguments.
- **kbase's stamped KB documents get prompt-engineer review before first use.** The templates
  `phase-3a` writes into `kb-root/` and the README assembly template are read by models in every KB
  kbase builds.

---

