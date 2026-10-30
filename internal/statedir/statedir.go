// Package statedir resolves the state directory of one KB: where kbase writes
// what it keeps outside the repository — a build's store, a scoring run's
// ledger. One KB has one state directory whatever verb names it.
package statedir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// KeyHexDigits is the key's length: the first 16 hex digits of the SHA-256 of
// the resolved kb-root/ path.
const KeyHexDigits = 16

// Dir is the state directory for the KB at kbRoot: override, else
// $XDG_STATE_HOME/kbase/<key>, $XDG_STATE_HOME defaulting to
// ~/.local/state. A relative override is the project root's, the directory
// holding kb-root/. A directory inside kb-root/ is refused.
func Dir(override, kbRoot string) (string, error) {
	dir := override
	if dir != "" {
		dir = kb.ProjectPath(filepath.Dir(kbRoot), dir)
	}
	if dir == "" {
		key, err := Key(kbRoot)
		if err != nil {
			return "", err
		}
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "state")
		}
		dir = filepath.Join(base, "kbase", key)
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if kb.Within(kbRoot, dir) {
		return "", result.Refusal{{Check: result.CheckUsage, Path: dir, Key: "--state-dir",
			Detail: fmt.Sprintf("the state directory %s is inside %s; build state is never written there", dir, kbRoot)}}
	}
	return dir, nil
}

// Key names the state directory of the kb-root/ at kbRoot, whether or not it
// exists yet; its directory must.
func Key(kbRoot string) (string, error) {
	abs, err := filepath.Abs(kbRoot)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Dir(abs)); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(kb.ResolvePath(abs)))
	return hex.EncodeToString(sum[:])[:KeyHexDigits], nil
}
