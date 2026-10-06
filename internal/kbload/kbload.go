// Package kbload opens a KB for a command. It reads the format stamp before
// anything else, refuses a KB newer than this kbase reads, and migrates an
// older one in memory, so every reader sees the current form through the
// source it returns. It is the one package that imports internal/migrate.
package kbload

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/migrate"
	"kbase/internal/result"
)

// The refusal classes the loader raises, and the remedy for a KB newer than
// this kbase.
const (
	CheckFormat  = "kb-format"
	CheckKBRoot  = "kb-root"
	UpdateRemedy = "update kbase"
)

// RecordFiles is every repository-relative path a build record has stood at
// under any format version: a path a migration drains is one a build that
// commits the records must commit the removal of.
func RecordFiles() []string {
	return slices.Concat(buildrecords.RecordFiles, migrate.SupersededRecordPaths())
}

// Open is the KB at kbRoot as this kbase reads it. A KB stamped at the
// current major and minor version is read from disk; one stamped older is
// read through migrate.Chain, its files in the current form in memory and the
// paths the migration made obsolete carried in the source. A missing entry
// point, an unreadable stamp and a newer major or minor version are a
// result.Refusal.
func Open(kbRoot string) (*kb.Source, error) {
	kbRoot, err := filepath.Abs(kbRoot)
	if err != nil {
		return nil, err
	}
	stamp, err := formatStamp(kbRoot)
	if err != nil {
		return nil, err
	}
	have, err := parseVersion(stamp)
	if err != nil {
		return nil, formatRefusal(kbRoot, "", fmt.Sprintf("%s's %s is %q, which is not a major.minor.patch version", kb.EntryPointFile, kb.FormatKey, stamp))
	}
	current, err := parseVersion(kb.FormatVersion)
	if err != nil {
		return nil, err
	}
	switch {
	case compareMinor(have, current) > 0:
		return nil, formatRefusal(kbRoot, UpdateRemedy, fmt.Sprintf("the KB's metadata format is %s and this kbase reads and writes %s", stamp, kb.FormatVersion))
	case compareMinor(have, current) == 0:
		return kb.OnDisk(kbRoot), nil
	}
	return migrated(kbRoot, stamp, fmt.Sprintf("%d.%d.0", have[0], have[1]))
}

// migrated is the KB at kbRoot, stamped stamp, converted from the format
// from in memory.
func migrated(kbRoot, stamp, from string) (*kb.Source, error) {
	files, err := coveredFiles(kbRoot)
	if err != nil {
		return nil, err
	}
	out, obsolete, err := migrate.Chain(from, kb.FormatVersion, files)
	if err != nil {
		return nil, formatRefusal(kbRoot, "", fmt.Sprintf("the KB's metadata at format %s does not convert to %s: %v", stamp, kb.FormatVersion, err))
	}
	return kb.MigratedSource(kbRoot, stamp, out, obsolete), nil
}

func formatRefusal(kbRoot, remedy, detail string) error {
	return result.Refusal{{Check: CheckFormat, Path: filepath.Join(kbRoot, kb.EntryPointFile), Remedy: remedy, Detail: detail}}
}

// formatStamp is the entry point's stamp, in whichever frontmatter form it
// opens with.
func formatStamp(kbRoot string) (string, error) {
	entry := filepath.Join(kbRoot, kb.EntryPointFile)
	data, err := os.ReadFile(entry)
	if errors.Is(err, fs.ErrNotExist) {
		return "", result.Refusal{{Check: CheckKBRoot, Path: entry, Detail: fmt.Sprintf("%s: %v", entry, kb.ErrNoEntryPoint)}}
	}
	if err != nil {
		return "", err
	}
	stamp, yamlForm, err := kb.FormatStamp(string(data))
	if err != nil {
		return "", formatRefusal(kbRoot, "", err.Error())
	}
	if !yamlForm {
		stamp = kb.UnstampedFormatVersion
		if s, ok := migrate.SupersededStamp(data); ok {
			stamp = s
		}
	}
	return stamp, nil
}

// coveredFiles is every file a format covers, under its repository-relative
// slash path: kb-root's documents, its index in either spelling, and the
// build records beside it in either spelling.
func coveredFiles(kbRoot string) (migrate.Files, error) {
	repo := filepath.Dir(kbRoot)
	files := migrate.Files{}
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
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(kbRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		inIndex := path.Dir(rel) == kb.IndexDir && (strings.HasSuffix(rel, ".jsonl") || strings.HasSuffix(rel, ".yaml"))
		if strings.HasSuffix(rel, ".md") || inIndex {
			return read(p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, rel := range RecordFiles() {
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

func parseVersion(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != len(out) {
		return out, fmt.Errorf("%q is not a major.minor.patch version", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || strconv.Itoa(n) != p {
			return out, fmt.Errorf("%q is not a major.minor.patch version", v)
		}
		out[i] = n
	}
	return out, nil
}

// compareMinor orders two versions by major and minor alone: a patch
// changes the bytes a writer produces, never what they mean.
func compareMinor(a, b [3]int) int {
	if a[0] != b[0] {
		return a[0] - b[0]
	}
	return a[1] - b[1]
}
