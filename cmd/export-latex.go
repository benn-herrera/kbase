package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"kbase/internal/build"
	"kbase/internal/export"
	"kbase/internal/kb"
	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/version"
)

// exportLatexOptions is everything export-latex takes. WorkDir is where the
// search for kb-root/ starts, and what a relative Out resolves against; Out
// is the out directory, latex-export/ beside kb-root/ where it is empty;
// Volumes names the volume directories to export, every one where it is
// empty.
type exportLatexOptions struct {
	WorkDir string
	Out     string
	Volumes []string
	Stdout  io.Writer
	Stderr  io.Writer
	Logger  log.Logger
}

// exportOutDefault is the out directory export-latex writes beside kb-root/
// when --out names none.
const exportOutDefault = "latex-export"

// checkExportDir names the refusal of a gen-tex/ kbase did not write.
const checkExportDir = "export-dir"

func runExportLatex(ctx context.Context, opts exportLatexOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "export-latex"); err != nil {
		return 0, err
	}
	outcome, fields := exportLatex(ctx, opts)
	return emitResult("export-latex", opts.Stdout, outcome, fields)
}

// exportLatex opens the KB and has export.Run export it, supplying the pandoc
// calls. It takes no lock and writes nothing under kb-root/.
func exportLatex(ctx context.Context, opts exportLatexOptions) (string, []toolresult.Field) {
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	fields := []toolresult.Field{{Key: "kb-root", Value: root}}
	out := opts.Out
	switch {
	case out == "":
		out = filepath.Join(filepath.Dir(root), exportOutDefault)
	default:
		out = kb.ProjectPath(filepath.Dir(root), out)
	}
	src, refusals, err := openKB(root)
	if err != nil {
		return failed(fields, err)
	}
	if refusals != nil {
		return refused(fields, refusals...)
	}
	fields = withMigration(fields, src, "export-latex", opts.Stderr)
	if outcome, out, stop := refuseDegraded(fields, src); stop {
		return outcome, out
	}
	// RunDefaults preflights too, but only after the export has written: the
	// separate preflight puts a pandoc refusal before any write.
	res, err := export.Run(ctx, src, out, opts.Volumes, version.Current,
		func(ctx context.Context) error {
			_, err := pandoc.Preflight(ctx)
			return err
		},
		func(ctx context.Context, defaultsPath string) error {
			return pandoc.RunDefaults(ctx, opts.Logger, defaultsPath)
		})
	var unknown export.UnknownVolumesError
	var foreign export.ExportDirError
	switch {
	case errors.As(err, &unknown):
		items := make([]toolresult.Item, len(unknown.Names))
		for i, name := range unknown.Names {
			items[i] = toolresult.Item{Check: checkUsage, Key: "--volume",
				Detail: fmt.Sprintf("--volume %s names no volume of the entry point's list", name)}
		}
		return refused(fields, items...)
	case errors.As(err, &foreign):
		return refused(fields, toolresult.Item{Check: checkExportDir, Path: foreign.Dir,
			Remedy: "empty the directory or name another --out", Detail: foreign.Error()})
	case err != nil:
		return pandocRefusedOrFailed(fields, err)
	}
	fields = append(fields,
		toolresult.Field{Key: "written", Value: append([]string{}, res.Written...)},
		toolresult.Field{Key: "volumes", Value: res.Volumes},
		toolresult.Field{Key: "report", Value: res.Report})
	if res.Changed {
		return toolresult.Done, fields
	}
	return toolresult.Unchanged, fields
}

// pandocRefusedOrFailed is a missing or out-of-range pandoc refused, as the
// build refuses it, and any other pandoc error failed.
func pandocRefusedOrFailed(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	var refusal toolresult.Refusal
	if errors.As(build.PandocRefusal(err), &refusal) {
		return refused(fields, refusal...)
	}
	return failed(fields, err)
}

func exportLatexCommand() *cobra.Command {
	var out string
	var volumes []string
	cmd := &cobra.Command{
		Use:   "export-latex",
		Short: "write a LaTeX project from the KB through pandoc",
		Long: `export-latex writes a LaTeX project from the KB's current state into --out
(default: latex-export/ beside kb-root/), converting it in one pandoc run. The
KB is the kb-root/ beside the repository's .git; no git is run, no lock is
taken, and nothing under kb-root/ is written. Each --volume names a volume directory to export, in the
entry point's order; one naming no volume of the entry point's list is
refused. kbase owns --out's gen-tex/ alone and regenerates it on every export:
main.tex, the defaults file pandoc ran (re-runnable as pandoc -d defaults.yaml
from gen-tex/), the Markdown stream, the filter, the template, the tables, the
copied assets and the report, gen-tex/kbase-export-report.yaml, every form the
export degraded and why. A gen-tex/ holding files and no export manifest is
refused, nothing written. Where --out holds no preamble.tex, a commented
starter is written for the packages and macros the sources used; one that
stands is never read, written or removed. Compiling main.tex is the author's
step. A re-issue over an unchanged KB writes nothing and reports unchanged. ` +
			degradedRefusedHelp + ` Its result is one YAML document on stdout.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			code, err := runExportLatex(cmd.Context(), exportLatexOptions{WorkDir: wd, Out: out, Volumes: volumes,
				Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), Logger: processLog.logger})
			if err != nil {
				return err
			}
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "the out directory (default: latex-export/ beside kb-root/)")
	cmd.Flags().StringArrayVar(&volumes, "volume", nil, "a volume directory under kb-root/ to export; repeatable (default: every volume)")
	return cmd
}
