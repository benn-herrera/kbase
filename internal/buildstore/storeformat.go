package buildstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"kbase/internal/result"
)

// StoreFormatFile, at the state store's root, holds the store's layout
// version: the names of its files and the shapes of the records in them. A
// store without one is version 1.
const StoreFormatFile = "format"

// StoreFormat is the layout version this kbase reads and writes.
const StoreFormat = 1

// StoreVersion is the layout version of the store at dir; one newer than
// this kbase's is refused.
func StoreVersion(dir string) (int, error) {
	path := filepath.Join(dir, StoreFormatFile)
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
	if v > StoreFormat {
		return 0, result.Refusal{{Check: CheckStateDir, Path: dir, Key: "--state-dir", Remedy: "update kbase, or use another --state-dir",
			Detail: fmt.Sprintf("the state store %s is at format %d, and this kbase knows format %d and older", dir, v, StoreFormat)}}
	}
	return v, nil
}
