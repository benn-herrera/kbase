#### Theorem blocks

A labelled blockquote whose first body paragraph opens with its printed number
— `**Display N**.` or `**Display N** (Title).` — becomes `\begin{kb-<name>}[{Title}]\label{v{N}-L}
… \end{kb-<name>}`, `<name>` the label line's name lowercased with spaces as hyphens, the opening
dropped, its title kept. A bare-number opening takes the label line's name for `Display`. For each
environment used, `\kbasetheorem{kb-<name>}{Display}{Plural}` is added to `header-includes`,
`Plural` by an English rule in the filter; the `kb-` prefix is no LaTeX environment's, so the
declaration is unconditional. Without that opening (a proof, which prints its own head and end
mark; a LaTeX environment's name such as `minipage`) the body blocks stand as they are, unquoted,
and the label span's id is kept as a label. The template's `\kbasetheorem` owns the rest: the
`definition` style, since the statements carry their own emphasis; one counter shared by every
kind, numbered within the chapter; and the cleveref names.

