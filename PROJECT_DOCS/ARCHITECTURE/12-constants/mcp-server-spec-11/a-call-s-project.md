#### A call's project

`mcp.Tools` wraps every tool in `inProject`. The wrapper adds the `project-dir` property to the
schema, and on each call `callProject` resolves the project: the `project-dir` the call names, which
`buildstore.ProjectRoot` requires to be a git worktree root (the same check `--project-dir` gets on
the command line), else the server's default project. It removes `project-dir` from the arguments
and hands the inner tool the project. No tool closes over a directory: a command tool sets it as the
call's invocation directory, a write op passes it to `Calls.WriteOp`, and a launch starts its child
there.

The default project is `mcp.Located`, whose zero value is the explicit none: `cmd/mcp.go` sets it
from `kb.RepositoryRoot` of the invocation directory only where that finds a repository, so no
reader ever passes `""` to a path function that would read it as the working directory. The server
runs in one of two modes, chosen in `cmd/mcp.go` before serving (SPEC §11 Two modes). With a
default project, `projectDirDescription` describes `project-dir`, that text followed by the
clause on the server's own project. Where `cmd/mcp.go` finds none, or under
`--project-dir-required`, the project is `mcp.ProjectRequired()`, the zero `mcp.Located`: `inProject`
lists `project-dir` in each schema's `required` with `projectDirMeaning` alone as its description.
The `callProject` refusal, a usage refusal keyed `project-dir`, stays behind that schema for a call
whose `project-dir` is empty. The global `--project-dir` given to `mcp` is a usage refusal keyed
`--project-dir` before serving; startup never fails for want of a project.

