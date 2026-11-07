### Two modes

The server has a default project or it does not. With a default project, a call may leave out `project-dir`. Without one, every call names its project.

- **With a default project.** `kbase mcp` found one, searching upward from its working directory for the repository root. `project-dir` is optional in every tool's schema, and a call that omits it runs in the default project.
- **Without a default project.** `kbase mcp --project-dir-required`, or a server whose search finds no repository. `project-dir` is in the `required` list of every tool's schema and the instructions say how a session gets the path. A server that finds no repository serves exactly as one given `--project-dir-required` does, instructions included; the flag forces this mode from inside a repository. A server with no default project lists no sheet ([Resources](resources.md)).
- **Startup never fails for want of a project.**
- **No project flag.** The global `--project-dir` given to `mcp` is a usage refusal ([§7](../7-tool-result-contract-and-exit-codes/index.md)) before it serves.
- **`project-dir`** is the absolute path of the project's git worktree root, the directory holding `.git`, where `kb-root/` is or will be; it is never `kb-root/` itself. The property's description says that much in each mode, and with a default project also that leaving it out means the server's own project. A call runs in the project it names where it names one, else in the default project. A call naming an empty one where there is no default project is `refused`, `check: usage`, `key: project-dir`. A `project-dir` that is no git worktree root is `refused`, `check: worktree`, `key: project-dir`, `path` the absolute path; a relative one resolves against the server's working directory.
- **Registration** is done by the external agent harness: one server entry, `kbase mcp` (with a default project, run from inside the project) or `kbase mcp --project-dir-required` (without one, for a harness that serves many projects from one place). README gives entries for two external agent harnesses as examples.

