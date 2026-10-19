package config

import (
	"path/filepath"
	"testing"
)

func TestFilePaths(t *testing.T) {
	dir := t.TempDir()
	if got, want := ProvidersPath(dir), filepath.Join(dir, ProvidersFileName); got != want {
		t.Errorf("ProvidersPath = %q, want %q", got, want)
	}
	if got, want := ConfigPath(dir), filepath.Join(dir, ConfigFileName); got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}
