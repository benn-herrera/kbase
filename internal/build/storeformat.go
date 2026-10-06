package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"kbase/internal/atomicfile"
	"kbase/internal/result"
)

// storeFormatFile, at the state store's root, holds the store's layout
// version: the names of its files and the shapes of the records in them. A
// store without one is version 1.
const storeFormatFile = "format"

// storeFormat is the layout version this kbase reads and writes.
const storeFormat = 1

// storeUpgrades converts a store in place from the version it is keyed by to
// the next.
var storeUpgrades = map[int]func(dir string) error{}

// storeVersion is the layout version of the store at dir; one newer than
// this kbase's is refused.
func storeVersion(dir string) (int, error) {
	path := filepath.Join(dir, storeFormatFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("the state store's %s holds %q, not a format version", path, b)
	}
	if v > storeFormat {
		return 0, result.Refusal{{Check: checkStateDir, Path: dir, Key: "--state-dir", Remedy: "update kbase, or use another --state-dir",
			Detail: fmt.Sprintf("the state store %s is at format %d, and this kbase knows format %d and older", dir, v, storeFormat)}}
	}
	return v, nil
}

// readyStore brings the store at dir to storeFormat and stamps it; its
// holder lock is held.
func readyStore(dir string) error {
	v, err := storeVersion(dir)
	if err != nil {
		return err
	}
	if err := upgradeStore(storeUpgrades, dir, v, storeFormat); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, storeFormatFile)); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeStoreFormat(dir, storeFormat)
}

// upgradeStore converts the store at dir from version from to version to
// through steps, stamping each version it reaches so an interrupted upgrade
// resumes where it stopped. A version no step leads on from is an error
// naming it.
func upgradeStore(steps map[int]func(dir string) error, dir string, from, to int) error {
	for v := from; v < to; v++ {
		step, ok := steps[v]
		if !ok {
			return fmt.Errorf("state store: no upgrade leads from format %d toward %d", v, to)
		}
		if err := step(dir); err != nil {
			return fmt.Errorf("state store format %d → %d: %w", v, v+1, err)
		}
		if err := writeStoreFormat(dir, v+1); err != nil {
			return err
		}
	}
	return nil
}

func writeStoreFormat(dir string, v int) error {
	return atomicfile.Write(filepath.Join(dir, storeFormatFile), []byte(strconv.Itoa(v)+"\n"), nil)
}
