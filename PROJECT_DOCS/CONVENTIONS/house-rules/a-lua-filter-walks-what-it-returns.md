### A Lua filter walks what it returns

In a pandoc Lua `topdown` traversal, what a filter returns in place of its element never meets the
traversal's filters — not its own type's, not the enclosing list's — though its children are walked.
One element or a list, block or inline alike (pandoc 3.12.1). Pitfall: a filter that returns
anything but nil or the element it was given — a `BlockQuote` filter returning the quote's body
expects the `Figure` filter to see a figure in it, and it does not. Such a filter walks its output
itself, as a list (`pandoc.Blocks{…}:walk(f)`, `pandoc.Inlines{…}:walk(f)`; an element's own
`:walk` skips that element), and returns the result with `false` as the second value, or the
descent walks those children twice.
