// Package kbload opens a KB for a command. It reads the format stamp before
// anything else, refuses a KB newer than this kbase reads, and migrates an
// older one in memory, so every reader sees the current form through the
// source it returns. It is the one package that imports the formats tree's root.
package kbload

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"kbase/internal/buildrecords"
	"kbase/internal/formats"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// The refusal classes the loader raises, and the remedy for a KB newer than
// this kbase.
const (
	CheckFormat     = "kb-format"
	CheckConversion = "conversion"
	UpdateRemedy    = "update kbase"
)

// Open is the KB at kbRoot as this kbase reads it. Its metadata-format stamp
// stands in kb.FormatFile, or, where that file does not stand, in the entry
// point's frontmatter. A KB stamped at the current major and minor version is
// read from disk; one stamped older is read through formats.Chain, its files
// in the current form in memory and the paths the migration made obsolete
// carried in the source. An unreadable stamp, a KB stamped in neither place,
// a newer major or minor version and an older KB its conversion refuses are a
// result.Refusal; a kb-root with no entry point and no stamp file is that
// refusal alone.
func Open(kbRoot string) (*kb.Source, error) {
	kbRoot, err := filepath.Abs(kbRoot)
	if err != nil {
		return nil, err
	}
	stamp, at, err := formatStamp(kbRoot)
	if err != nil {
		return nil, err
	}
	have, err := formats.ParseVersion(stamp)
	if err != nil {
		return nil, formatRefusal(kbRoot, at, "", fmt.Sprintf("%s's %s is %q, which is not a major.minor.patch version", at, kb.FormatKey, stamp))
	}
	current, err := formats.ParseVersion(kb.FormatVersion)
	if err != nil {
		return nil, err
	}
	switch {
	case formats.CompareMinor(have, current) > 0:
		return nil, formatRefusal(kbRoot, at, UpdateRemedy, fmt.Sprintf("the KB's metadata format is %s and this kbase reads and writes %s", stamp, kb.FormatVersion))
	case formats.CompareMinor(have, current) == 0:
		return kb.OnDisk(kbRoot), nil
	}
	return migrated(kbRoot, stamp)
}

// migrated is the KB at kbRoot, stamped stamp, converted in memory.
func migrated(kbRoot, stamp string) (*kb.Source, error) {
	files, err := coveredFiles(kbRoot)
	if err != nil {
		return nil, err
	}
	out, obsolete, warned, err := formats.Chain(stamp, kb.FormatVersion, files)
	if err != nil {
		if r := conversionRefusal(kbRoot, err); r != nil {
			return nil, r
		}
		return nil, formatRefusal(kbRoot, kb.EntryPointFile, "", fmt.Sprintf("the KB's metadata at format %s does not convert to %s: %v", stamp, kb.FormatVersion, err))
	}
	return kb.MigratedSource(kbRoot, stamp, out, obsolete, migrationWarnings(warned)...), nil
}

// conversionRefusal is the refusal of a conversion that err stopped on what a
// KB holds — bytes the conversion cannot carry, or the 2.0.x load's refusal as
// that load reported it — and nil for any other err.
func conversionRefusal(kbRoot string, err error) result.Refusal {
	var surplus *formats.ConversionError
	var ann *formats.AnnotationError
	var bad formats.MalformedError
	switch {
	case errors.As(err, &surplus):
		return result.Refusal{{Check: CheckConversion, Path: filepath.Join(filepath.Dir(kbRoot), filepath.FromSlash(surplus.Path)),
			Line: surplus.Line, Detail: err.Error()}}
	case errors.As(err, &ann):
		items := make(result.Refusal, len(ann.Faults))
		for i, f := range ann.Faults {
			items[i] = result.Item{Check: kb.CheckMetadata, Key: f.Field, Path: f.Path, Line: f.Line, Remedy: f.Remedy, Detail: f.Detail}
			if f.Allowed != "" {
				items[i].Allowed = []string{f.Allowed}
			}
		}
		return items
	case errors.As(err, &bad):
		return result.Refusal{{Check: kb.CheckMetadata, Detail: bad.Error()}}
	}
	return nil
}

// migrationWarnings is the chain's warnings, each path kb-root-relative where
// it lies inside kb-root.
func migrationWarnings(warned []formats.Warning) []kb.MigrationWarning {
	out := make([]kb.MigrationWarning, len(warned))
	for i, w := range warned {
		out[i] = kb.MigrationWarning(w)
		if rel, ok := strings.CutPrefix(w.Path, kb.KBDir+"/"); ok {
			out[i].Path = rel
		}
	}
	return out
}

// formatRefusal is the refusal of a stamp at kb-root-relative path at.
func formatRefusal(kbRoot, at, remedy, detail string) error {
	return result.Refusal{{Check: CheckFormat, Path: filepath.Join(kbRoot, filepath.FromSlash(at)), Remedy: remedy, Detail: detail}}
}

// formatStamp is the KB's metadata-format stamp: the version kb.FormatFile
// holds where that file stands — where both stand the file is the stamp, for
// the frontmatter's is the superseded place — and else the entry point's
// frontmatter stamp, the standing place of every KB a migration has not
// stamped in the file, read forever. The second return is the stamp's
// kb-root-relative path, for the refusals to name. A kb-root with no entry
// point and no stamp file is the kb-root refusal; a KB stamped in neither
// place, and a stamp that cannot be read, are each the kb-format refusal.
func formatStamp(kbRoot string) (stamp, at string, err error) {
	data, err := os.ReadFile(filepath.Join(kbRoot, kb.FormatFile))
	switch {
	case err == nil:
		stamp, err := kb.DecodeFormatStamp(data)
		if err != nil {
			return "", kb.FormatFile, formatRefusal(kbRoot, kb.FormatFile, "", err.Error())
		}
		return stamp, kb.FormatFile, nil
	case !errors.Is(err, fs.ErrNotExist):
		return "", kb.FormatFile, err
	}
	entry := filepath.Join(kbRoot, kb.EntryPointFile)
	data, err = os.ReadFile(entry)
	if errors.Is(err, fs.ErrNotExist) {
		return "", kb.EntryPointFile, result.Refusal{{Check: kb.CheckKBRoot, Path: entry, Detail: fmt.Sprintf("%s: %v", entry, kb.ErrNoEntryPoint)}}
	}
	if err != nil {
		return "", kb.EntryPointFile, err
	}
	stamp, err = kb.FormatStamp(string(data))
	if err == nil {
		return stamp, kb.EntryPointFile, nil
	}
	detail := err.Error()
	if fm, ferr := kb.ParseFrontmatter(string(data)); ferr == nil {
		if _, stamped := fm[kb.FormatKey]; !stamped {
			detail = fmt.Sprintf("neither %s nor %s's frontmatter carries a %s stamp", kb.FormatFile, kb.EntryPointFile, kb.FormatKey)
		}
	}
	return "", kb.EntryPointFile, formatRefusal(kbRoot, kb.EntryPointFile, "", detail)
}

// HasFormatStamp is whether a format stamp stands where Open reads it: the
// stamp file, or the entry point's frontmatter — including one that will not
// parse, whose refusal Open surfaces rather than a caller covering it with a
// stamp of its own.
func HasFormatStamp(kbRoot string) bool {
	if _, err := os.Stat(filepath.Join(kbRoot, kb.FormatFile)); err == nil {
		return true
	}
	data, err := os.ReadFile(filepath.Join(kbRoot, kb.EntryPointFile))
	if err != nil {
		return false
	}
	fm, err := kb.ParseFrontmatter(string(data))
	if err != nil {
		return true
	}
	_, stamped := fm[kb.FormatKey]
	return stamped
}

// coveredFiles is every file a format covers, under its repository-relative
// slash path: kb-root's documents, its index, and the build records beside
// it. It is the reader's file set: a
// directory leaf discovery never descends into holds no document the
// conversion reads or makes obsolete, and the index is the one excluded
// directory a format covers, since a migration drains and rewrites it.
func coveredFiles(kbRoot string) (formats.Files, error) {
	repo := filepath.Dir(kbRoot)
	files := formats.Files{}
	read := func(p string) error {
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(repo, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	}
	err := filepath.WalkDir(kbRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(kbRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if kb.ExcludeDirs[d.Name()] && rel != kb.IndexDir {
				return fs.SkipDir
			}
			return nil
		}
		inIndex := path.Dir(rel) == kb.IndexDir && strings.HasSuffix(rel, ".yaml")
		if !strings.HasSuffix(rel, ".md") && !inIndex {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return result.Refusal{{Check: kb.CheckSymlink, Path: p,
				Detail: fmt.Sprintf("%s is a symbolic link, and a KB document is read only where it stands", p)}}
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return read(p)
	})
	if err != nil {
		return nil, err
	}
	for _, rel := range buildrecords.RecordFiles {
		p := filepath.Join(repo, rel)
		if !kb.IsFile(p) {
			continue
		}
		if err := read(p); err != nil {
			return nil, err
		}
	}
	return files, nil
}
