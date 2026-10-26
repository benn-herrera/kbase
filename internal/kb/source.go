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

// Migration is what a load did to an older KB: the format version it carried,
// the one it was converted to, the one sentence that says what changed, and
// what the conversion dropped or changed that the converted files do not show.
type Migration struct {
	From, To, Note string
	Warnings       []MigrationWarning
}

// MigrationWarning is one thing a migration dropped or changed: the file it
// names — kb-root-relative inside kb-root, repository-relative beside it — its
// 1-based line (0 where it names none), and one sentence.
type MigrationWarning struct {
	Path   string
	Line   int
	Detail string
}

// MigrationNote is the one sentence a migrated KB's record carries: every
// migration ends in the 3.0.0 form, which marks each judged value
// model-scored.
const MigrationNote = "every rigor and bearing that score judges is now marked model-scored, so a default score run re-judges it; to keep one, re-assert its current value with set-rigor or set-bearing, only where the user asks"

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
	files     map[string][]byte
	obsolete  []string
	migrated  bool
	migration *Migration
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
// paths the migration made so, each keyed by repository-relative slash path,
// and warnings what the migration reports it dropped or changed.
func MigratedSource(kbRoot, version string, files map[string][]byte, obsolete []string, warnings ...MigrationWarning) *Source {
	s := OnDisk(kbRoot)
	s.version, s.migrated = version, true
	s.files = maps.Clone(files)
	s.obsolete = slices.Clone(obsolete)
	s.migration = &Migration{From: version, To: FormatVersion, Note: MigrationNote, Warnings: slices.Clone(warnings)}
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

// Migration is what this load converted, and the sentence that says so; ok is
// false where the KB was read at the current version.
func (s *Source) Migration() (Migration, bool) {
	if s.migration == nil {
		return Migration{}, false
	}
	return *s.migration, true
}

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

// Removed records that p no longer stands, so later reads fall to the disk.
func (s *Source) Removed(p string) {
	if rel, ok := s.RepoRel(p); ok {
		delete(s.files, rel)
	}
}

// DiskFile is p's bytes on disk now, whatever the source holds for it.
func (s *Source) DiskFile(p string) ([]byte, error) { return os.ReadFile(p) }

// isSymlink is whether p itself, not what it resolves to, is a symbolic link.
func isSymlink(p string) bool {
	info, err := os.Lstat(p)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}

// DirFiles is every regular file directly in dir as the KB reads it — on
// disk less the obsolete ones, and those the source holds — absolute and
// sorted. A file that is a symbolic link is refused rather than read through.
func (s *Source) DirFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() || !s.IsFile(p) {
			continue
		}
		if e.Type()&fs.ModeSymlink != 0 {
			return nil, &SymlinkError{Path: p}
		}
		out = append(out, p)
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
