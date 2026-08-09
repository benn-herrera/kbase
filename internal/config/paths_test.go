package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDirEnvOverride: the override wins outright, and a blank or
// whitespace-only setting is treated as unset — an empty KBASE_CONFIG_DIR
// exported by a shell profile must not resolve every config path to the
// process working directory.
func TestDirEnvOverride(t *testing.T) {
	for _, tc := range []struct{ name, env string }{
		{"unset", ""},
		{"whitespace only", "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvConfigDir, tc.env)
			home, err := os.UserHomeDir()
			if err != nil {
				t.Skipf("no home directory in this environment: %v", err)
			}
			dir, err := Dir()
			if err != nil {
				t.Fatalf("Dir: %v", err)
			}
			if want := filepath.Join(home, defaultDirParent, defaultDirName); dir != want {
				t.Errorf("Dir() = %q, want %q", dir, want)
			}
		})
	}

	t.Run("override", func(t *testing.T) {
		want := t.TempDir()
		t.Setenv(EnvConfigDir, want)
		dir, err := Dir()
		if err != nil {
			t.Fatalf("Dir: %v", err)
		}
		if dir != want {
			t.Errorf("Dir() = %q, want the override %q", dir, want)
		}
	})
}

func TestFilePaths(t *testing.T) {
	dir := t.TempDir()
	if got, want := ProvidersPath(dir), filepath.Join(dir, ProvidersFileName); got != want {
		t.Errorf("ProvidersPath = %q, want %q", got, want)
	}
	if got, want := ConfigPath(dir), filepath.Join(dir, ConfigFileName); got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}
