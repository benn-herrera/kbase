### A command's starting directory is `invocationDir(cmd)`

No `os.Getwd` in `cmd` outside that function. Pitfall: `os.Getwd()` anywhere a verb's logic runs
ignores the project an MCP call or `--project-dir` names. `cmd/invocationdir_test.go` enforces it.
