package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/ingest"
	"kbase/internal/survey"
	"kbase/internal/tokens"
)

// jsonToStdout is the --json value that sends the artifact to stdout instead
// of to a file — the usual convention, and what makes the verb pipeable.
const jsonToStdout = "-"

// surveyOptions is the resolved input of the survey verb.
//
// It carries no providerOptions: stages 1 and 2 are entirely deterministic
// (ARCHITECTURE.md §4), so this verb needs no provider, no key, and no
// configuration directory. Wiring it to the pool would invent a failure mode
// — "no provider selected" on a command that never calls one.
type surveyOptions struct {
	// Root is the corpus directory to walk.
	Root string

	// JSONPath is where the full artifact goes: empty writes none,
	// jsonToStdout writes it to Stdout, anything else names a file.
	JSONPath string

	// Stdout takes the verb's output: the human summary, or the artifact
	// when JSONPath is jsonToStdout.
	Stdout io.Writer

	// Stderr takes failure detail, and the summary when stdout is carrying
	// the artifact.
	Stderr io.Writer
}

// runSurvey ingests a corpus, surveys it, and reports.
//
// When the artifact is bound for stdout the summary moves to stderr, so a
// caller piping the JSON gets JSON and nothing else. That is the same split
// the models verb makes between its list and its summary line.
func runSurvey(opts surveyOptions) error {
	if opts.Stdout == nil || opts.Stderr == nil {
		return fmt.Errorf("survey: Stdout and Stderr are required")
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		return fmt.Errorf("survey: a corpus directory is required")
	}

	corpus, err := ingest.WalkMarkdown(root)
	if err != nil {
		return err
	}
	artifact, err := survey.Survey(corpus, tokens.Estimator{})
	if err != nil {
		return err
	}

	summary := opts.Stdout
	if opts.JSONPath == jsonToStdout {
		summary = opts.Stderr
	}
	if opts.JSONPath != "" {
		if err := writeArtifact(artifact, opts.JSONPath, opts.Stdout); err != nil {
			return err
		}
	}

	topLevel := 0
	for _, f := range artifact.Files {
		topLevel += len(f.Sections)
	}
	t := artifact.Corpus
	fmt.Fprintf(summary, "survey ok: files=%d bytes=%d tokens=%d sections=%d top-level=%d\n",
		t.Files, t.Bytes, t.Tokens, t.Sections, topLevel)
	fmt.Fprintf(summary, "links: internal=%d unresolved=%d external=%d anchor=%d\n",
		t.Links.Internal, t.Links.Unresolved, t.Links.External, t.Links.Anchor)
	return nil
}

// writeArtifact writes the survey JSON to path, or to stdout for "-".
func writeArtifact(artifact survey.Artifact, path string, stdout io.Writer) error {
	if path == jsonToStdout {
		return artifact.WriteJSON(stdout)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("survey: create %s: %w", path, err)
	}
	if err := artifact.WriteJSON(f); err != nil {
		// The close error is dropped deliberately: the write failure is the
		// one that explains what went wrong, and reporting a second failure
		// from tearing down an already-failed file buries it.
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("survey: close %s: %w", path, err)
	}
	return nil
}

var surveyFlagJSON string

var surveyCmd = &cobra.Command{
	Use:   "survey <corpus-dir>",
	Short: "inventory a Markdown corpus: headings, sizes, links, gists",
	Long: `Walk a Markdown corpus and report its structure: per-file heading tree with
byte offsets, section token estimates, the intra-corpus link graph, and
first-paragraph gists.

This is the deterministic head of the pipeline — it runs entirely offline and
calls no model, so it needs no provider and no configuration. A summary goes
to stdout; --json writes the full artifact, which is what the taxonomy stage
consumes. With --json -, the artifact goes to stdout and the summary moves to
stderr.

Every structural fact is a byte offset into the source, which is never
modified: the same corpus always produces the same artifact.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSurvey(surveyOptions{
			Root:     args[0],
			JSONPath: surveyFlagJSON,
			Stdout:   os.Stdout,
			Stderr:   os.Stderr,
		})
	},
}

func init() {
	surveyCmd.Flags().StringVar(&surveyFlagJSON, "json", "",
		"write the full survey artifact to this path ("+jsonToStdout+" for stdout)")
	rootCmd.AddCommand(surveyCmd)
}
