// Package atomicfile replaces a file's bytes so that a reader sees the old
// content or the new, never a mix, and a crash leaves one or the other.
package atomicfile

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CreateMode is the mode a created file asks for. It is not a permission
// posture: the umask decides what lands on disk. It is never passed to
// Chmod, which does not consult the umask.
const CreateMode = 0o666

// tempSuffix ends the name of every temp file a write makes.
const tempSuffix = ".tmp"

// tempPrefix starts the name of every temp file a write makes for target.
func tempPrefix(target string) string { return "." + filepath.Base(target) + "." }

// ignoreFile is the git ignore file a temp directory holds, and ignoreAll its
// content: it ignores every file in the directory, itself included, so git
// never lists or commits anything there.
const (
	ignoreFile = ".gitignore"
	ignoreAll  = "*\n"
)

// Write replaces path's bytes with data through a temp file beside it, synced
// and renamed into place; the directory must exist. A symlink at path is
// followed, so the link survives and its target is rewritten. An existing
// file keeps its mode; a new one is created at CreateMode under the umask.
func Write(path string, data []byte) error {
	target := followed(path)
	return replace(path, target, filepath.Dir(target), data, 0)
}

// WriteMode is Write for a file whose permission the caller sets: the file
// lands at exactly mode, whatever the umask and whatever mode a file it
// replaces had. Its temp is created at mode, so the bytes are never readable
// beyond it.
func WriteMode(path string, data []byte, mode fs.FileMode) error {
	target := followed(path)
	return replace(path, target, filepath.Dir(target), data, mode.Perm())
}

// followed is path with a symlink at it resolved, else path itself.
func followed(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// LinkError is WriteIn's refusal of a path it will not write: the path itself
// is a symbolic link, or what its directory resolves to lies outside the root.
// Detail is the sentence saying which, naming the path.
type LinkError struct{ Detail string }

func (e *LinkError) Error() string { return e.Detail }

// WriteIn is Write for a path that must stay inside root, its temp made in
// tempDir rather than beside path: tempDir, a directory under root and so on
// its filesystem, is created where missing, holding ignoreFile. A path that
// is a symbolic link is refused with a *LinkError, and so is one whose
// resolved target leaves root. Neither touches the file the link names.
func WriteIn(root, tempDir, path string, data []byte) error {
	target, err := resolveIn(root, path)
	if err != nil {
		return err
	}
	if err := readyTempDir(tempDir); err != nil {
		return err
	}
	return replace(path, target, tempDir, data, 0)
}

// readyTempDir creates dir where missing and writes its ignoreFile where that
// does not hold ignoreAll.
func readyTempDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ignore := filepath.Join(dir, ignoreFile)
	if data, err := os.ReadFile(ignore); err == nil && string(data) == ignoreAll {
		return nil
	}
	return Write(ignore, []byte(ignoreAll))
}

// resolveIn is path as WriteIn writes it: path itself where nothing on its
// way resolves elsewhere, and the refusal of a link at path or of a target
// outside root.
func resolveIn(root, path string) (string, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", &LinkError{Detail: fmt.Sprintf("%s is a symbolic link, and a file is replaced only where it stands", path)}
	}
	target := path
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		target = filepath.Join(resolved, filepath.Base(path))
	}
	resolvedRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		resolvedRoot = resolved
	}
	if rel, err := filepath.Rel(resolvedRoot, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &LinkError{Detail: fmt.Sprintf("%s resolves to %s, which is outside %s", path, target, root)}
	}
	return target, nil
}

// replace installs data at target, the path Write resolved, as path, through
// a temp file in tempDir. A zero mode is Write's: an existing file's mode is
// carried, a new file takes CreateMode under the umask. Any other mode is set
// exactly.
func replace(path, target, tempDir string, data []byte, mode fs.FileMode) error {
	info, statErr := os.Stat(target)
	createMode := fs.FileMode(CreateMode)
	if mode != 0 {
		createMode = mode
	}
	tmp, err := createTemp(tempDir, tempPrefix(target), createMode)
	if err != nil {
		return fmt.Errorf("create a temp file for %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	switch {
	case mode != 0:
		if err := os.Chmod(tmp.Name(), mode); err != nil {
			return fmt.Errorf("set %s's mode: %w", path, err)
		}
	case statErr == nil:
		if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
			return fmt.Errorf("carry %s's mode: %w", path, err)
		}
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	return nil
}

// createTemp creates a new file in dir whose name starts with prefix, at mode
// under the umask. os.CreateTemp hard-codes 0600, and chmodding up from it
// would bypass the umask, so the name is chosen here; O_EXCL keeps two writers
// off one name.
func createTemp(dir, prefix string, mode fs.FileMode) (*os.File, error) {
	for range 1000 {
		name := filepath.Join(dir, prefix+strconv.FormatUint(rand.Uint64(), 36)+tempSuffix)
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return f, err
	}
	return nil, fmt.Errorf("no unused %s* name in %s", prefix, dir)
}
