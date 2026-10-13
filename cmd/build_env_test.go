package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/config"
)

// TestBuildConfiguredByEnvironmentAlone: with no configuration directory at
// all, the four variables configure the build's provider, the model serving
// each tier, and a key bound for a non-loopback http:// endpoint is one line
// on the verb's stderr.
func TestBuildConfiguredByEnvironmentAlone(t *testing.T) {
	saved := flagConfigDir
	t.Cleanup(func() { flagConfigDir = saved })
	flagConfigDir = filepath.Join(t.TempDir(), "absent")
	key := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(key, []byte("secret-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvAPIBaseURL, "http://reaper.local:4000/v1")
	t.Setenv(config.EnvDecision, "Some-Decision")
	t.Setenv(config.EnvText, "Some-Text")
	t.Setenv(config.EnvAPIKeyFile, key)

	_, providers, err := loadVerbContext()
	if err != nil {
		t.Fatalf("loading an absent configuration directory: %v", err)
	}
	var stderr bytes.Buffer
	p, err := buildProvider(providers, config.EnvOverrides(os.Getenv), &stderr)()
	if err != nil {
		t.Fatalf("provider = %v", err)
	}
	if p.Letters != "Some-Decision" || p.Overview != "Some-Text" || p.Client == nil {
		t.Errorf("provider = %+v, want each tier's own variable on it", p)
	}
	if lines := strings.Split(strings.TrimSpace(stderr.String()), "\n"); len(lines) != 1 || !strings.Contains(lines[0], "cleartext") || strings.Contains(stderr.String(), "secret-key") {
		t.Errorf("stderr = %q, want one cleartext line naming no key", stderr.String())
	}
	if _, err := os.Stat(flagConfigDir); err == nil {
		t.Error("resolving the provider created the configuration directory")
	}
}
