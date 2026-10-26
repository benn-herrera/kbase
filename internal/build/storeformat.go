package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"kbase/internal/atomicfile"
	"kbase/internal/buildstore"
)

// storeUpgrades converts a store in place from the version it is keyed by to
// the next.
var storeUpgrades = map[int]func(dir string) error{}

// readyStore brings the store at dir to buildstore.StoreFormat and stamps it;
// its holder lock is held.
func readyStore(dir string) error {
	v, err := buildstore.StoreVersion(dir)
	if err != nil {
		return err
	}
	if err := upgradeStore(storeUpgrades, dir, v, buildstore.StoreFormat); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, buildstore.StoreFormatFile)); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeStoreFormat(dir, buildstore.StoreFormat)
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
	return atomicfile.Write(filepath.Join(dir, buildstore.StoreFormatFile), []byte(strconv.Itoa(v)+"\n"), nil)
}
