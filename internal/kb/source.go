package kb

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"unicode/utf8"
)

// Source is where kbase reads one KB's files — kb-root and the build records
// beside it — named by absolute path. A KB at the current format version is
// read from disk. One loaded at an older version is read from the bytes its
// migration produced, which stand in for the disk's at the same paths, and
// carries the paths the migration made obsolete; those read as absent.
//
// A writer that replaces a file records the new bytes with Put, so later
// reads in the same command see them whichever way the source is backed.
type Source struct {
	root, repo string
	version    string
	// files is keyed by repository-relative slash path.
	files    map[string][]byte
	obsolete []string
	migrated bool
}

// OnDisk is the source of the KB at kbRoot read from disk, as the current
// format version, with no stamp read: the loader's for a KB stamped current,
// and a build stage's over the tree it is writing.
func OnDisk(kbRoot string) *Source {
	root := filepath.Clean(kbRoot)
	return &Source{root: root, repo: filepath.Dir(root), version: FormatVersion, files: map[string][]byte{}}
}

// MigratedSource is the source of the KB at kbRoot loaded at version and
// migrated in memory: files are the current form's bytes and obsolete the
// paths the migration made so, each keyed by repository-relative slash path.
func MigratedSource(kbRoot, version string, files map[string][]byte, obsolete []string) *Source {
	s := OnDisk(kbRoot)
	s.version, s.migrated = version, true
	s.files = maps.Clone(files)
	s.obsolete = slices.Clone(obsolete)
	return s
}

// Root is the KB's absolute kb-root.
func (s *Source) Root() string { return s.root }

// Repo is the directory kb-root sits in, where the build records live.
func (s *Source) Repo() string { return s.repo }

// Version is the format version the KB carried when it was loaded.
func (s *Source) Version() string { return s.version }

// Migrated is whether the KB was loaded at an older version and its files
// are read in the current form only in memory.
func (s *Source) Migrated() bool { return s.migrated }

// Saved records that the KB now stands on disk in the current form, its
// obsolete paths gone: later reads are of a current KB.
func (s *Source) Saved() {
	s.version, s.migrated, s.obsolete = FormatVersion, false, nil
}

// Obsolete is the repository-relative paths the migration made obsolete.
func (s *Source) Obsolete() []string { return slices.Clone(s.obsolete) }

// MigratedFiles is every repository-relative path the migration produced,
// sorted; none where the KB was read at the current version.
func (s *Source) MigratedFiles() []string {
	if !s.migrated {
		return nil
	}
	return slices.Sorted(maps.Keys(s.files))
}

// RepoRel is p as a repository-relative slash path, ok false where p is not
// under the repository.
func (s *Source) RepoRel(p string) (string, bool) {
	rel, err := filepath.Rel(s.repo, p)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// Path is the absolute path of a repository-relative slash path.
func (s *Source) Path(repoRel string) string {
	return filepath.Join(s.repo, filepath.FromSlash(repoRel))
}

// KBPath is the absolute path of a kb-root-relative slash path.
func (s *Source) KBPath(kbRel string) string {
	return filepath.Join(s.root, filepath.FromSlash(kbRel))
}

func (s *Source) isObsolete(rel string) bool { return slices.Contains(s.obsolete, rel) }

// ReadFile is the file's bytes as the KB reads them.
func (s *Source) ReadFile(p string) ([]byte, error) {
	if rel, ok := s.RepoRel(p); ok {
		if b, ok := s.files[rel]; ok {
			return b, nil
		}
		if s.isObsolete(rel) {
			return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
		}
	}
	return os.ReadFile(p)
}

// ReadText is the file's text as Python's text-mode read returns it: UTF-8,
// with \r\n and \r translated to \n.
func (s *Source) ReadText(p string) (string, error) {
	data, err := s.ReadFile(p)
	if err != nil {
		return "", err
	}
	return decodeText(p, data)
}

func decodeText(p string, data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("%s: not UTF-8", p)
	}
	return TranslateNewlines(string(data)), nil
}

// IsFile is whether p names a regular file of the KB, following symlinks.
func (s *Source) IsFile(p string) bool {
	if rel, ok := s.RepoRel(p); ok {
		if _, ok := s.files[rel]; ok {
			return true
		}
		if s.isObsolete(rel) {
			return false
		}
	}
	return IsFile(p)
}

// Put records that p now holds data.
func (s *Source) Put(p string, data []byte) {
	if rel, ok := s.RepoRel(p); ok {
		s.files[rel] = slices.Clone(data)
	}
}

// DiskFile is p's bytes on disk now, whatever the source holds for it.
func (s *Source) DiskFile(p string) ([]byte, error) { return os.ReadFile(p) }

// DirFiles is every regular file directly in dir as the KB reads it — on
// disk less the obsolete ones, and those the source holds — absolute and
// sorted.
func (s *Source) DirFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if p := filepath.Join(dir, e.Name()); !e.IsDir() && s.IsFile(p) {
			out = append(out, p)
		}
	}
	for _, p := range s.held(dir) {
		if filepath.Dir(p) == dir && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out, nil
}

// held is every regular file under dir the source holds in memory and the
// disk does not, as absolute paths.
func (s *Source) held(dir string) []string {
	var out []string
	for rel := range s.files {
		p := s.Path(rel)
		if Within(dir, p) && p != dir && !IsFile(p) {
			out = append(out, p)
		}
	}
	return out
}

// walk is WalkDir over dir's files on disk less the obsolete ones, then the
// files the source holds that the disk does not; skip decides whether a
// directory below dir is descended into.
func (s *Source) walk(dir string, skip func(name string) bool, visit func(p string) error) error {
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && skip(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if rel, ok := s.RepoRel(p); ok && s.isObsolete(rel) {
			return nil
		}
		return visit(p)
	})
	if err != nil {
		return err
	}
	for _, p := range s.held(dir) {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(splitPath(filepath.Dir(rel)), skip) {
			continue
		}
		if err := visit(p); err != nil {
			return err
		}
	}
	return nil
}

// splitPath is a relative path's directory names, outermost first.
func splitPath(rel string) []string {
	var out []string
	for rel != "." && rel != "" {
		out = append(out, filepath.Base(rel))
		rel = filepath.Dir(rel)
	}
	slices.Reverse(out)
	return out
}

// ErrNotExist reports whether err is a missing file.
func ErrNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
