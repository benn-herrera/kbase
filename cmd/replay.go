//go:build kbasedev

package main

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kbase/internal/kb"
	toolresult "kbase/internal/result"
	"kbase/internal/statedir"
)

// changedLocalFlags is the names of cmd's own non-persistent flags the command
// line set.
func changedLocalFlags(cmd *cobra.Command) []string {
	var given []string
	cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			given = append(given, f.Name)
		}
	})
	return given
}

// replayStateDir is the state store a development replay reads, absolute.
// --state-dir names it, a relative one resolving against the project root
// where workDir has one and against workDir where it has none; without it,
// it is the store of the kb-root at or above workDir, and a workDir with none
// is refused naming --state-dir.
func replayStateDir(stateDir, workDir string) (string, error) {
	if stateDir != "" {
		base := workDir
		if !filepath.IsAbs(stateDir) {
			repo, err := kb.RepositoryRoot(workDir)
			if err != nil {
				return "", err
			}
			if repo != "" {
				base = repo
			}
		}
		return filepath.Abs(kb.ProjectPath(base, stateDir))
	}
	root, refusals, err := kbRootFrom(workDir)
	if err != nil {
		return "", err
	}
	if refusals != nil {
		return "", toolresult.Refusal{{Check: checkUsage, Key: "--state-dir",
			Detail: "a replay without --state-dir finds its store through the kb-root at or above the working directory, there is none here, and --state-dir names the store"}}
	}
	return statedir.Dir("", root)
}

// replayTakes is the flags a replay takes beside --replay as its refusal names
// them: flags save replay itself, and the persistent --config-dir.
func replayTakes(flags []string) string {
	var names []string
	for _, name := range flags {
		if name != "replay" {
			names = append(names, "--"+name)
		}
	}
	names = append(names, "--config-dir")
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
