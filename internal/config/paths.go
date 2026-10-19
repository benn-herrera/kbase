// Package config loads kbase's on-disk configuration: the providers.toml
// endpoint pool (WHERE inference happens) and config.toml (WHICH of those
// endpoints and models the pipeline draws on).
//
// Configuration is read from a stack of layers — the repository's own
// directory over the global one, or the single directory a flag or a variable
// names — and the two files merge by different rules: a pool entry replaces a
// same-named entry whole, a config.toml field overrides the corresponding
// field alone. Which layer supplied what is carried into the report, so a
// resolved setting always names where it came from.
//
// Path resolution and file loading are deliberately separate: Layers resolves
// the directories, and every loader takes an explicit path. Tests can
// therefore exercise the loaders against a temp directory without ever
// reaching for — or writing to — a real home.
//
// A leading ~ in a path kbase was handed — a configuration directory, an
// entry's apiKeyFile — expands from the user's home directory; no other
// spelling does (see expandHome).
package config

import (
	"path/filepath"
)

const (
	// EnvConfigDir overrides the configuration directory wholesale. It exists
	// so a test, a CI run, or a second corpus can point kbase at its own
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

// ProvidersPath returns the providers.toml path within dir.
func ProvidersPath(dir string) string { return filepath.Join(dir, ProvidersFileName) }

// ConfigPath returns the config.toml path within dir.
func ConfigPath(dir string) string { return filepath.Join(dir, ConfigFileName) }
