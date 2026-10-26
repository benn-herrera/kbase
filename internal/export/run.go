package export

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"

	"kbase/internal/kb"
)

// Result is what one export reports: the absolute paths it wrote, sorted; the
// directories of the volumes it exported, in order; the report's path; and
// whether anything was written, removed or rewritten with different bytes.
type Result struct {
	Written []string
	Volumes []string
	Report  string
	Changed bool
}

// Run exports src into out's GenTexDir: it walks src, narrows the walk to
// volumes (every volume where it is empty), assembles GenTexDir's inputs
// stamped with kbaseVersion, writes them, converts them with one runDefaults
// call over the defaults file, and writes the report. preflight runs before
// anything is written, so a pandoc refusal precedes every write. pandoc's
// outputs are read before and after its run, so a file whose bytes it
// rewrote unchanged is not reported written.
//
// Errors come back unwrapped: preflight's and runDefaults' as they returned
// them, an UnknownVolumesError for a name no volume answers to, and an
// ExportDirError for a GenTexDir the export does not own.
func Run(ctx context.Context, src *kb.Source, out string, volumes []string, kbaseVersion string,
	preflight func(ctx context.Context) error,
	runDefaults func(ctx context.Context, defaultsPath string) error) (Result, error) {
	rep := &Report{}
	tree, err := walk(src, rep)
	if err != nil {
		return Result{}, err
	}
	tree, err = selectVolumes(tree, volumes)
	if err != nil {
		return Result{}, err
	}
	files, err := assemble(src, tree, kbaseVersion)
	if err != nil {
		return Result{}, err
	}
	if err := preflight(ctx); err != nil {
		return Result{}, err
	}
	written, _, changed, err := writeOut(out, files)
	if err != nil {
		return Result{}, err
	}
	before, err := pandocOutputs(out)
	if err != nil {
		return Result{}, err
	}
	gen := filepath.Join(out, GenTexDir)
	if err := runDefaults(ctx, filepath.Join(gen, DefaultsFile)); err != nil {
		return Result{}, err
	}
	after, err := pandocOutputs(out)
	if err != nil {
		return Result{}, err
	}
	for p, data := range after {
		if old, ok := before[p]; !ok || !bytes.Equal(old, data) {
			written = append(written, p)
		}
	}
	report, err := writeReport(out, rep.Findings())
	if err != nil {
		return Result{}, err
	}
	if report != "" {
		written = append(written, report)
	}
	slices.Sort(written)
	dirs := make([]string, len(tree.Volumes))
	for i, v := range tree.Volumes {
		dirs[i] = v.Dir
	}
	return Result{Written: written, Volumes: dirs, Report: filepath.Join(gen, ReportFile),
		Changed: changed || len(written) > 0}, nil
}
