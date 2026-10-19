package config

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHome expands a leading ~ in a configured path from the user's home
// directory: ~ alone is the home directory, and ~ followed by the platform's
// separator — /x, or \x on Windows — is x beneath it. It is the only expansion
// kbase performs on a path it was handed: ~user is not supported, and a ~
// anywhere else is literal.
//
// Where the home lookup fails, or answers with a path that is not absolute,
// the string is returned exactly as written: a path kbase cannot resolve is
// one to report under its own spelling, never one to resolve against a guess.
func expandHome(path string) string {
	rest, ok := strings.CutPrefix(path, "~")
	if !ok || (rest != "" && !os.IsPathSeparator(rest[0])) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return path
	}
	if rest == "" {
		return home
	}
	return filepath.Join(home, rest[1:])
}
