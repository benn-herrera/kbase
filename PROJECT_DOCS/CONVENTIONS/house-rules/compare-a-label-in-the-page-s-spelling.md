### Compare a label in the page's spelling

pandoc escapes `&<>"'` in attribute values, and
differently for a `Link` than for a `Span` or `Div`. A fact kbase records to be compared against
the page uses the escaped form the page uses; never unescape one side to match the other.
A label carried out of the page into another format — the LaTeX export — is HTML-decoded once, at
its read; this rule governs comparisons against the page.

---

