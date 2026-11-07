// Package kbwrite is the write of a file under kb-root/ and kbase's scratch
// there, kb.TempDir: every writer under kb-root/ writes through Inside, and
// every holder of the KB write lock clears the scratch through RemoveTemps.
package kbwrite

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"kbase/internal/atomicfile"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// Inside replaces p with data atomically, its temp file made in root's
// kb.TempDir, refusing a path that is a symbolic link or resolves outside root
// as the KB's symlink refusal. root is kb-root.
func Inside(root, p string, data []byte) error {
	err := atomicfile.WriteIn(root, filepath.Join(root, kb.TempDir), p, data)
	var link *atomicfile.LinkError
	if errors.As(err, &link) {
		return result.Refusal{{Check: kb.CheckSymlink, Path: p, Detail: link.Detail}}
	}
	return err
}

// RemoveTemps removes kbRoot's kb.TempDir, which Inside creates on demand and
// which stands only while a writer writes: a writer calls it once its writes
// are done, and before it starts, where a killed writer left the directory,
// its temps included. The directory's parent goes too where that leaves it
// empty, so a reset that removes every tracked file under kb-root leaves no
// directory behind. The caller holds the KB write lock.
func RemoveTemps(kbRoot string) error {
	dir := filepath.Join(kbRoot, kb.TempDir)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Dir(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil || len(entries) > 0 {
		return err
	}
	return os.Remove(filepath.Dir(dir))
}
