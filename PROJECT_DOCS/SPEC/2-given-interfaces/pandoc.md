### pandoc

The accepted range's lower bound is 3.12. Its upper bound is the next major
`pandoc-api-version`, exclusive, where major means the first two components under Haskell's
versioning policy: 1.23.x is accepted, 1.24 is not. Only the owner widens the range. The range and
the `pandoc-api-version` are enforced at preflight; a missing pandoc is refused with an error naming
pandoc's install page. The build's reader runs pandoc LaTeX to JSON with no filters, and with
`--citeproc` exactly when a bibliography is offered; `export-latex` ([§8](../8-maintenance-subcommands/index.md)) is the one run with a
filter.

---

