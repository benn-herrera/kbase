### Plan dispatchability
A work package is dispatchable only when the following conditions have been met

- It names its scope in files, packages or symbols — never a span of design sections
- Contains the command and its answer, or the quoted line, for each fact it acts on
- A package that changes a type, a signature or a test double names in its scope every file
  that references it — `git grep -l '<Name>' -- '*.go'`, test files included — and contains
  that command and its answer
- States its done-line in runner targets
- A done-line that names a before/after measurement recipe contains, from planning, that recipe's passing run at the base commit
- Has received a usability review by the prompt engineer (its definition contains the criteria)
- Has been revised by the architect based on the findings of the prompt engineer

A plan whose packages lack these is not dispatchable. It is not offered for approval as a dispatch
plan, and none of its packages is dispatched.

