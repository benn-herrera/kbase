package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"go.yaml.in/yaml/v3"

	"kbase/internal/atomicfile"
)

// ExportDirError is a GenTexDir holding files and no export manifest: the
// export-dir refusal, under which nothing is written. Dir is that GenTexDir.
type ExportDirError struct {
	Dir string
}

func (e ExportDirError) Error() string {
	return fmt.Sprintf("%s holds files and no export manifest %s; empty the directory or name another", e.Dir, ManifestFile)
}

// manifest is kbase-export.yaml: every file the export owns in GenTexDir,
// GenTexDir-relative slash paths, sorted.
type manifest struct {
	Files []string `yaml:"files"`
}

// writeOut writes files, by GenTexDir-relative slash path, into dir's
// GenTexDir, and dir's PreambleFile where none stands. It returns the
// absolute paths it wrote, preamble's among them where it was written;
// preamble, that path where this call wrote it and "" otherwise; and whether
// anything changed. A file whose bytes GenTexDir already holds is not
// rewritten; a file the previous manifest listed and this export does not
// own is removed, with every directory under GenTexDir the removal left
// empty; the manifest, listing files and the outputs pandoc and the verb
// write beside them, is written after them. A GenTexDir holding files and no
// manifest is an ExportDirError, and nothing is written. Nothing in dir
// outside GenTexDir but a missing PreambleFile is written, read or removed.
func writeOut(dir string, files map[string][]byte) (written []string, preamble string, changed bool, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, "", false, err
	}
	gen := filepath.Join(dir, GenTexDir)
	previous, found, err := readManifest(gen)
	if err != nil {
		return nil, "", false, err
	}
	if !found {
		entries, err := os.ReadDir(gen)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, "", false, err
		}
		if len(entries) > 0 {
			return nil, "", false, ExportDirError{Dir: gen}
		}
	}
	owned := append(slices.Collect(maps.Keys(files)), OutputFile, FindingsFile, ReportFile)
	slices.Sort(owned)
	body, err := yamlBytes(manifest{Files: owned})
	if err != nil {
		return nil, "", false, fmt.Errorf("encoding %s: %w", ManifestFile, err)
	}
	for _, rel := range slices.Sorted(maps.Keys(files)) {
		p, err := writeChanged(gen, rel, files[rel])
		if err != nil {
			return nil, "", false, err
		}
		if p != "" {
			written = append(written, p)
		}
	}
	for _, rel := range previous {
		if slices.Contains(owned, rel) {
			continue
		}
		err := os.Remove(filepath.Join(gen, filepath.FromSlash(rel)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, "", false, fmt.Errorf("removing %s, which the previous export wrote: %w", rel, err)
		}
		changed = changed || err == nil
		if err := removeEmptyParents(gen, rel); err != nil {
			return nil, "", false, err
		}
	}
	p, err := writeChanged(gen, ManifestFile, body)
	if err != nil {
		return nil, "", false, err
	}
	if p != "" {
		written = append(written, p)
	}
	if preamble, err = writeStarterPreamble(dir); err != nil {
		return nil, "", false, err
	}
	if preamble != "" {
		written = append(written, preamble)
	}
	return written, preamble, changed || len(written) > 0, nil
}

// pandocOutputs is the bytes of each file pandoc writes into dir's GenTexDir —
// OutputFile and FindingsFile — by absolute path, a file absent left out.
func pandocOutputs(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, name := range []string{OutputFile, FindingsFile} {
		p, err := filepath.Abs(filepath.Join(dir, GenTexDir, name))
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[p] = data
	}
	return out, nil
}

// preambleForm is the report form of the line naming the author's
// PreambleFile.
const preambleForm = "preamble"

// writeReport writes dir's GenTexDir ReportFile: one YAML list of the walk's
// findings, walked, then the filter's from FindingsFile, then, where dir's
// PreambleFile exists, a line naming it as the author's. The preamble is
// checked for existence only, never opened. It returns the report's absolute
// path where this call changed its bytes, "" otherwise.
func writeReport(dir string, walked []Finding) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	gen := filepath.Join(dir, GenTexDir)
	findings, err := os.ReadFile(filepath.Join(gen, FindingsFile))
	if err != nil {
		return "", fmt.Errorf("reading the filter's findings: %w", err)
	}
	var filter []Finding
	if err := json.Unmarshal(findings, &filter); err != nil {
		return "", fmt.Errorf("reading the filter's findings %s: %w", filepath.Join(gen, FindingsFile), err)
	}
	report := append(append([]Finding{}, walked...), filter...)
	preamble := filepath.Join(dir, PreambleFile)
	switch _, err := os.Lstat(preamble); {
	case err == nil:
		report = append(report, Finding{Form: preambleForm, Path: preamble,
			Reason: "the author's: the packages and macros the source papers used go here; LaTeX names any missing"})
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	body, err := yamlBytes(report)
	if err != nil {
		return "", fmt.Errorf("encoding %s: %w", ReportFile, err)
	}
	return writeChanged(gen, ReportFile, body)
}

// writeStarterPreamble writes the starter PreambleFile into dir where nothing
// stands at that name, and returns its absolute path, "" where something
// stood. What stands there is the author's: it is never opened.
func writeStarterPreamble(dir string) (string, error) {
	p := filepath.Join(dir, PreambleFile)
	_, err := os.Lstat(p)
	if err == nil {
		return "", nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := atomicfile.Write(p, preambleStarter, nil); err != nil {
		return "", err
	}
	return p, nil
}

// removeEmptyParents removes each directory above rel under dir, innermost
// first, that holds nothing, and stops at the first that holds something.
func removeEmptyParents(dir, rel string) error {
	for p := filepath.Dir(filepath.FromSlash(rel)); p != "."; p = filepath.Dir(p) {
		abs := filepath.Join(dir, p)
		entries, err := os.ReadDir(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return nil
		}
		if err := os.Remove(abs); err != nil {
			return fmt.Errorf("removing %s, which the removal emptied: %w", abs, err)
		}
	}
	return nil
}

// readManifest is the files dir's export manifest lists, and whether dir has
// one. The manifest is on disk where anyone may have edited it, so every path
// it lists is held inside dir before a removal acts on it.
func readManifest(dir string) ([]string, bool, error) {
	p := filepath.Join(dir, ManifestFile)
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var m manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, false, fmt.Errorf("reading the export manifest %s: %w", p, err)
	}
	for _, rel := range m.Files {
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return nil, false, fmt.Errorf("the export manifest %s lists %q, which is no path inside its directory", p, rel)
		}
	}
	return m.Files, true, nil
}

// writeChanged writes data to rel under dir where the bytes there differ,
// and returns the absolute path written, "" where nothing was.
func writeChanged(dir, rel string, data []byte) (string, error) {
	p := filepath.Join(dir, filepath.FromSlash(rel))
	old, err := os.ReadFile(p)
	if err == nil && bytes.Equal(old, data) {
		return "", nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := atomicfile.Write(p, data, nil); err != nil {
		return "", err
	}
	return p, nil
}
