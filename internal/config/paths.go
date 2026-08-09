// Package config loads kbase's on-disk configuration: the providers.toml
// endpoint pool (WHERE inference happens) and config.toml (WHICH of those
// endpoints and models the pipeline draws on).
//
// Path resolution and file loading are deliberately separate: Dir resolves
// the config directory, and every loader takes an explicit path. Tests can
// therefore exercise the loaders against a temp directory without ever
// reaching for — or writing to — a real home.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// EnvConfigDir overrides the config directory wholesale. It exists so
	// a test, a CI run, or a second corpus can point kbase at its own
	// configuration without disturbing the user's.
	EnvConfigDir = "KBASE_CONFIG_DIR"

	// ProvidersFileName holds the endpoint pool.
	ProvidersFileName = "providers.toml"
	// ConfigFileName holds the choices that draw from that pool.
	ConfigFileName = "config.toml"

	// The default location, ~/.config/kbase, split into the XDG-ish parent
	// and the application directory beneath it.
	defaultDirParent = ".config"
	defaultDirName   = "kbase"
)

// Dir returns the configuration directory: EnvConfigDir when set,
// otherwise ~/.config/kbase.
//
// It does not create the directory or check that it exists — a fresh
// install has no config at all, and the loaders treat a missing file as
// "unconfigured", not as an error.
func Dir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(EnvConfigDir)); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(home, defaultDirParent, defaultDirName), nil
}

// ProvidersPath returns the providers.toml path within dir.
func ProvidersPath(dir string) string { return filepath.Join(dir, ProvidersFileName) }

// ConfigPath returns the config.toml path within dir.
func ConfigPath(dir string) string { return filepath.Join(dir, ConfigFileName) }
