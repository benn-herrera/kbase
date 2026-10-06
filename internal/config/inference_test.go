package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func keyFile(t *testing.T, key string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(p, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveInferencePrecedence(t *testing.T) {
	files := Config{Models: ModelMap{Heavy: "file-heavy", Light: "file-light"}}
	entry := &Provider{Name: "pool", BaseURL: "https://file.example/v1", APIKey: "file-key"}
	env := Overrides{BaseURL: "https://env.example/v1", Model: "env-model", APIKeyFile: keyFile(t, "env-key")}
	flags := Overrides{BaseURL: "https://flag.example/v1"}
	for _, tc := range []struct {
		name   string
		entry  *Provider
		layers []Overrides
		want   Inference
	}{
		{"flag over env", entry, []Overrides{flags, env}, Inference{"pool", "https://flag.example/v1", "env-key", "env-model", "env-model"}},
		{"env over file", entry, []Overrides{env}, Inference{"pool", "https://env.example/v1", "env-key", "env-model", "env-model"}},
		{"a variable overrides its one field", entry, []Overrides{{Model: "env-model"}}, Inference{"pool", "https://file.example/v1", "file-key", "env-model", "env-model"}},
		{"file alone", entry, nil, Inference{"pool", "https://file.example/v1", "file-key", "file-heavy", "file-light"}},
		{"env alone", nil, []Overrides{env}, Inference{"environment", "https://env.example/v1", "env-key", "env-model", "env-model"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := files
			if tc.entry == nil {
				cfg = Config{}
			}
			got, err := ResolveInference(cfg, tc.entry, nil, tc.layers...)
			if err != nil || got != tc.want {
				t.Errorf("= %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestResolveInferenceNamesWhatItLacks(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.txt")
	for _, tc := range []struct {
		name      string
		entry     *Provider
		layers    []Overrides
		key, path string
	}{
		{"no base URL", nil, nil, EnvAPIBaseURL, ProvidersFileName},
		{"no model", nil, []Overrides{{BaseURL: "https://env.example/v1"}}, EnvModel, ConfigFileName},
		{"an unreadable key file", nil, []Overrides{{BaseURL: "https://env.example/v1", Model: "m", APIKeyFile: missing}}, EnvAPIKeyFile, missing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveInference(Config{}, tc.entry, errors.New("no usable provider"), tc.layers...)
			var lack LackError
			if !errors.As(err, &lack) {
				t.Fatalf("err = %v, want a LackError", err)
			}
			if key, path := lack.Named(); key != tc.key || path != tc.path || !strings.Contains(lack.Detail, tc.key) {
				t.Errorf("named %s, %s (%q); want %s, %s", key, path, lack.Detail, tc.key, tc.path)
			}
		})
	}
}

func TestCleartextWarning(t *testing.T) {
	for _, tc := range []struct {
		name, url, key string
		warns          bool
	}{
		{"no key", "http://reaper.local:4000/v1", "", false},
		{"loopback by name", "http://localhost:4000/v1", "k", false},
		{"loopback by address", "http://127.0.0.1:4000/v1", "k", false},
		{"loopback v6", "http://[::1]:4000/v1", "k", false},
		{"https", "https://reaper.local/v1", "k", false},
		{"non-loopback with key", "http://reaper.local:4000/v1", "secret-key", true},
	} {
		got := CleartextWarning(tc.url, tc.key)
		if (got != "") != tc.warns || strings.Contains(got, "secret-key") || strings.Contains(got, "\n") {
			t.Errorf("%s: %q, want warning %t, one line naming no key", tc.name, got, tc.warns)
		}
	}
}
