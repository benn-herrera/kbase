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

// TempSuffix ends the name of every temp file Write makes. A temp beside a
// KB document is invisible to the KB's walks, which match *.md and the
// register's exact name.
const TempSuffix = ".tmp"

// tempPrefix starts the name of every temp file Write makes for target.
func tempPrefix(target string) string { return "." + filepath.Base(target) + "." }

// Write replaces path's bytes with data through a temp file beside it, synced
// and renamed into place; the directory must exist. A symlink at path is
// followed, so the link survives and its target is rewritten. An existing
// file keeps its mode; a new one is created at CreateMode under the umask.
//
// beforeRename, when set, runs with the temp written and the rename not yet
// done. Calling keep leaves the temp on disk if what follows unwinds, as a
// killed process would leave it.
func Write(path string, data []byte, beforeRename func(keep func())) error {
	return replace(path, followed(path), data, beforeRename, 0)
}

// WriteMode is Write for a file whose permission the caller sets: the file
// lands at exactly mode, whatever the umask and whatever mode a file it
// replaces had. Its temp is created at mode, so the bytes are never readable
// beyond it.
func WriteMode(path string, data []byte, mode fs.FileMode) error {
	return replace(path, followed(path), data, nil, mode.Perm())
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

// WriteIn is Write for a path that must stay inside root: a path that is a
// symbolic link is refused with a *LinkError, and so is one whose resolved
// target leaves root. Neither touches the file the link names.
func WriteIn(root, path string, data []byte) error {
	target, err := resolveIn(root, path)
	if err != nil {
		return err
	}
	return replace(path, target, data, nil, 0)
}

// resolveIn is path as WriteIn writes it: path itself where nothing on its
// way resolves elsewhere, and the refusal of a link at path or of a target
// outside root.
func resolveIn(root, path string) (string, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", &LinkError{Detail: fmt.Sprintf("%s is a symbolic link, and a document is replaced only where it stands", path)}
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

// replace installs data at target, the path Write resolved, as path. A zero
// mode is Write's: an existing file's mode is carried, a new file takes
// CreateMode under the umask. Any other mode is set exactly.
func replace(path, target string, data []byte, beforeRename func(keep func()), mode fs.FileMode) error {
	info, statErr := os.Stat(target)
	createMode := fs.FileMode(CreateMode)
	if mode != 0 {
		createMode = mode
	}
	tmp, err := createTemp(filepath.Dir(target), tempPrefix(target), createMode)
	if err != nil {
		return fmt.Errorf("create a temp file beside %s: %w", path, err)
	}
	keep := false
	defer func() {
		if !keep {
			os.Remove(tmp.Name())
		}
	}()
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
	if beforeRename != nil {
		beforeRename(func() { keep = true })
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	return nil
}

// Sweep removes every temp file Write left beside path, as a process killed
// between the temp and the rename leaves one. The caller must hold whatever
// excludes path's other writers.
func Sweep(path string) error {
	target := followed(path)
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		return err
	}
	prefix := tempPrefix(target)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), TempSuffix) {
			if err := os.Remove(filepath.Join(filepath.Dir(target), e.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// createTemp creates a new file in dir whose name starts with prefix, at mode
// under the umask. os.CreateTemp hard-codes 0600, and chmodding up from it
// would bypass the umask, so the name is chosen here; O_EXCL keeps two writers
// off one name.
func createTemp(dir, prefix string, mode fs.FileMode) (*os.File, error) {
	for range 1000 {
		name := filepath.Join(dir, prefix+strconv.FormatUint(rand.Uint64(), 36)+TempSuffix)
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return f, err
	}
	return nil, fmt.Errorf("no unused %s* name in %s", prefix, dir)
}
