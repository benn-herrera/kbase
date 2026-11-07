### Stamped documents

`AGENTS.md` and `CONVENTIONS.md` under `kb-root/` are the **stamped documents**; with `README.md`
they are the **maintained-block documents**. `README.md` gives the maintenance commands: kbase's
subcommands. The stamped documents point a reader at kbase's tools, never at its records:

- `AGENTS.md` forbids reading or writing anything under `.index/`, save testing that such a path
  exists, and states the rule that a session sets a rigor or a bearing only to a value the user
  gave, both as the MCP server's instructions do, in the same words ([§11](../11-the-mcp-server/served-text.md), items 3
  and 4).
- `CONVENTIONS.md` states that the records and the statement markers are written only by a write
  op, never by hand, and that no agent edits the block kbase maintains in a stamped document.
- A summary page is generated as prose for people and sessions to read. A person or session neither edits one nor takes a value from it, and kbase never reads one back,
  and `verify` reports a page whose bytes differ from the rendering (`register-freshness`).

A **maintained block** is a generated region between marker comments: the lines from
`<!-- vvv-kbase … kbase-vvv -->` to `<!-- ^^^-kbase … kbase-^^^ -->`. Each marker line is
recognised by its two ends alone, so the text between the ends may differ between releases.

- A document containing exactly one opening marker line and one closing marker line, the opening
  first, has its block rewritten: kbase rewrites the span between the two lines and nothing else,
  and the text outside them is the KB's own.
- A document containing any other arrangement of marker lines is left untouched and listed under
  `unfenced` in `update-format`'s result ([§8](../8-maintenance-subcommands/index.md)). That is not a refusal.
- A pair with nothing between its lines is valid; the next `update-format` fills it.
- An absent `AGENTS.md` or `CONVENTIONS.md` is written whole, marker lines included. `README.md` is
  written only by a build, which supplies its overview passage outside the block; `init` and
  `update-format` never create one.
- A block is rewritten on every `update-format` run, and when `init` or a build writes the
  document. A document is written only where its bytes change.

`kb-root/CLAUDE.md` is the redirect to `AGENTS.md`: its content is the one line `@AGENTS.md`. `init`
writes it where absent. A write refuses where `CLAUDE.md` exists and is not the redirect (`check:
agents-redirect`); nothing converts it.

