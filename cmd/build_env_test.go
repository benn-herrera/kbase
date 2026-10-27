package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/modelclient"
)

// TestBuildConfiguredByEnvironmentAlone: with no configuration directory at
// all, the environment's endpoint and its two role variables configure the
// build, one client is opened on that endpoint for both roles, and a key bound
// for a non-loopback http:// endpoint is one line on the verb's stderr.
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

	providers, err := loadVerbContext(t.TempDir())
	if err != nil {
		t.Fatalf("loading an absent configuration directory: %v", err)
	}
	var stderr bytes.Buffer
	var dialled []model.Endpoint
	providers.NewClient = func(e model.Endpoint) model.Client {
		dialled = append(dialled, e)
		return modeltest.NewScriptedMock(nil, nil).Client()
	}
	p, err := buildProvider(providers, config.EnvOverrides(os.Getenv), &stderr)(modelclient.New(providers.NewClient, nil, "", 0))
	if err != nil {
		t.Fatalf("provider = %v", err)
	}
	if p.Letters != "Some-Decision" || p.Overview != "Some-Text" {
		t.Errorf("provider = %+v, want each role's own variable on it", p)
	}
	// One environment endpoint serves both roles, so it is dialled once.
	if len(dialled) != 1 || dialled[0].BaseURL != "http://reaper.local:4000/v1" || dialled[0].APIKey != "secret-key" {
		t.Errorf("endpoints dialled = %+v, want the one the environment names, with its key file read", dialled)
	}
	// The environment supplied the endpoint and no pool entry did, so each
	// record says environment and names no entry.
	if len(p.Roles) != 2 {
		t.Fatalf("roles = %v, want one per role the build asks", p.Roles)
	}
	for i, role := range p.Roles {
		if got := roleField(role, "back-end"); got != "environment" {
			t.Errorf("roles[%d] back-end = %v, want environment", i, got)
		}
		if got := roleField(role, "provider"); got != nil {
			t.Errorf("roles[%d] names provider %v, want none", i, got)
		}
	}
	if lines := strings.Split(strings.TrimSpace(stderr.String()), "\n"); len(lines) != 1 || !strings.Contains(lines[0], "cleartext") || strings.Contains(stderr.String(), "secret-key") {
		t.Errorf("stderr = %q, want one cleartext line naming no key", stderr.String())
	}
	if _, err := os.Stat(flagConfigDir); err == nil {
		t.Error("resolving the provider created the configuration directory")
	}
}
