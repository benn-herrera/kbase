package main

import "kbase/internal/build"

func init() {
	rootCmd.AddCommand(monitorCommand("cancel", "stop the running build, resumably",
		`cancel signals the build holding the run lock and waits for it to let the
lock go: the unit in flight is abandoned with nothing written for it, the
build exits cancelled, and the next build invocation resumes it. Its result
is one YAML document on stdout.`, build.Cancel))
}
