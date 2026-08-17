package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/assemble"
	"kbase/internal/pipeline"
)

// writeGenericAgentsVerb names this verb in every message it emits.
const writeGenericAgentsVerb = "write-generic-agents"

// writeGenericAgentsOptions is the resolved input of the write-generic-agents
// verb.
//
// It carries no providerOptions and no configuration directory: the samples
// are a function of the app version alone (assemble.GenericAgents), so this
// verb dials nothing and reads nothing.
type writeGenericAgentsOptions struct {
	// Out is the directory the samples are written into. It is required and
	// is created if missing.
	Out string

	// Stderr takes the verb's one-line summary and nothing else.
	Stderr io.Writer
}

// runWriteGenericAgents writes the generic agent-definition samples into Out.
//
// The write is all-or-nothing. Every target is checked for an existing file
// BEFORE any byte is written or any directory created, and a single conflict
// refuses the whole command with every conflict named — a verb that wrote two
// files and refused the third would leave the user reconciling a directory by
// hand. There is no --force: kbase will not help destroy data, and deleting
// the file you mean to replace is one command away.
func runWriteGenericAgents(opts writeGenericAgentsOptions) error {
	if opts.Stderr == nil {
		return fmt.Errorf("%s: Stderr is required", writeGenericAgentsVerb)
	}
	out := strings.TrimSpace(opts.Out)
	if out == "" {
		return fmt.Errorf("%s: --out is required; this verb writes files and has no default target",
			writeGenericAgentsVerb)
	}
	if fi, err := os.Stat(out); err == nil {
		if !fi.IsDir() {
			return fmt.Errorf("%s: --out %s exists and is not a directory", writeGenericAgentsVerb, out)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: --out %s: %w", writeGenericAgentsVerb, out, err)
	}

	samples := assemble.GenericAgents()
	var conflicts []string
	for _, s := range samples {
		path := filepath.Join(out, s.Path)
		switch _, err := os.Lstat(path); {
		case err == nil:
			conflicts = append(conflicts, path)
		case !errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("%s: %s: %w", writeGenericAgentsVerb, path, err)
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%s: refusing to overwrite what is already there:\n%s\n"+
			"nothing was written; there is no --force — delete what you mean to replace and rerun",
			writeGenericAgentsVerb, indentedList(conflicts))
	}

	if err := os.MkdirAll(out, pipeline.CreateDirMode); err != nil {
		return fmt.Errorf("%s: create %s: %w", writeGenericAgentsVerb, out, err)
	}
	for _, s := range samples {
		if err := writeNewFile(filepath.Join(out, s.Path), s.Data); err != nil {
			return err
		}
	}
	fmt.Fprintf(opts.Stderr, "wrote %d agent definitions to %s\n", len(samples), out)
	return nil
}

// writeNewFile creates one sample, refusing a path that appeared between the
// conflict scan and now (O_EXCL). The scan is what makes the refusal
// all-or-nothing and what produces the full conflict list; this is the last
// word on never clobbering a file, and it costs one flag.
func writeNewFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, pipeline.CreateFileMode)
	if err != nil {
		return fmt.Errorf("%s: create %s: %w", writeGenericAgentsVerb, path, err)
	}
	if _, err := f.Write(data); err != nil {
		// The close error is dropped deliberately: the write failure is the
		// one that explains what went wrong.
		f.Close()
		return fmt.Errorf("%s: write %s: %w", writeGenericAgentsVerb, path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("%s: close %s: %w", writeGenericAgentsVerb, path, err)
	}
	return nil
}

var writeGenericAgentsFlagOut string

var writeGenericAgentsCmd = &cobra.Command{
	Use:   writeGenericAgentsVerb,
	Short: "write the generic agent definitions to a directory for adaptation",
	Long: `Write the generic agent definitions — docent.md, maintainer.md and
README-ADAPTATION.md — into a directory you name.

They are samples for your own agent tooling: the definitions kbase runs on are
embedded in the binary and fixed by its version, so kbase neither reads these
copies back nor ships them inside the knowledge bases it builds. Adapt them for
the model you bring; a stronger model will do better with a definition written
for its own capabilities. They ship as marked stubs today.

--out is required and is created if missing. There is no default target: a
command that writes files somewhere you did not name is a command you have to
undo.

If any target file already exists the command refuses, listing every conflict,
writing nothing at all, and exiting 1. There is no --force and there will not
be one — delete what you mean to replace.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWriteGenericAgents(writeGenericAgentsOptions{
			Out:    writeGenericAgentsFlagOut,
			Stderr: cmd.ErrOrStderr(),
		})
	},
}

func init() {
	writeGenericAgentsCmd.Flags().StringVar(&writeGenericAgentsFlagOut, "out", "",
		"directory the definitions are written to (required; created if missing)")
	rootCmd.AddCommand(writeGenericAgentsCmd)
}
