package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/pipeline"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
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

	// Logger takes the walk's and the survey's diagnostics — skipped files,
	// front-matter decisions, folded link resolutions. It is a separate
	// channel from Stderr, which carries the verb's own output; nil means
	// the caller has none to hand over.
	Logger log.Logger
}

// runSurvey ingests a corpus, surveys it, and reports.
//
// When the artifact is bound for stdout the summary moves to stderr, so a
// caller piping the JSON gets JSON and nothing else. The consequence is that
// a script must know which --json value was passed to know which stream the
// summary is on; the alternative, a summary interleaved with the payload,
// makes the payload unparseable, which is worse.
func runSurvey(opts surveyOptions) error {
	if err := requireStreams(opts.Stdout, opts.Stderr, "survey"); err != nil {
		return err
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		return fmt.Errorf("survey: a corpus directory is required")
	}
	lg := opts.Logger
	if lg == nil {
		lg = log.Discard()
	}

	// The composition root is where a source format is chosen: the neutral
	// walk is told which extensions the adapter calls documents, and the
	// adapter turns those bytes into the neutral artifact. One format exists,
	// so this is one pair of calls — a registry or a format switch buys
	// nothing until there is a second adapter to select between.
	corpus, err := ingest.Walk(root, markdown.Extensions(), lg)
	if err != nil {
		return err
	}
	artifact, err := markdown.Survey(corpus, tokens.Estimator{}, lg)
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
//
// The mode is pipeline.ArtifactFileMode rather than a local copy of the same
// number: the artifact carries gists and titles lifted out of the user's
// corpus — actual content, not just metadata about it — so it is owner-only
// for the same reason every other derived file is, and §9's constants table
// names one place for that fact.
func writeArtifact(artifact survey.Artifact, path string, stdout io.Writer) error {
	if path == jsonToStdout {
		return artifact.WriteJSON(stdout)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, pipeline.ArtifactFileMode)
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
			Logger:   processLog.logger,
		})
	},
}

func init() {
	surveyCmd.Flags().StringVar(&surveyFlagJSON, "json", "",
		"write the full survey artifact to this path ("+jsonToStdout+" for stdout)")
	rootCmd.AddCommand(surveyCmd)
}
