package main

func init() {
	rootCmd.AddCommand(monitorCommand("cancel", "stop the running build or score run, resumably",
		`cancel signals the build or score run holding the state store and waits for
it to let go: the unit or item in flight is abandoned with nothing written for
it, the run exits cancelled, and the next invocation resumes it. Its result is
one YAML document on stdout.`, cancelScore))
}
