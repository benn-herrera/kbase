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
	// The layers a resolved field names: the environment above the files, and
	// the layer each file-supplied field's value came from.
	const fromEnv, fromPool, fromConfig = LayerSourceEnvironment, LayerSourceRepository, LayerSourceGlobal
	files := Config{
		Models:  ModelMap{Decision: "file-decision", Text: "file-text"},
		Sources: Sources{Decision: fromConfig, Text: fromConfig},
	}
	entry := &Provider{Name: "pool", BaseURL: "https://file.example/v1", APIKey: "file-key", Layer: fromPool}
	decisionEntry := &Provider{Name: "pool", BaseURL: "https://file.example/v1", APIKey: "file-key", Layer: fromPool,
		DecisionsFlavor: FlavorDecisions}
	env := Overrides{BaseURL: "https://env.example/v1", Decision: "env-decision", Text: "env-text", APIKeyFile: keyFile(t, "env-key")}
	higher := Overrides{BaseURL: "https://flag.example/v1"}
	for _, tc := range []struct {
		name   string
		entry  *Provider
		layers []Overrides
		want   Inference
	}{
		{"the earlier layer over the later one", entry, []Overrides{higher, env}, Inference{"pool", "https://flag.example/v1", "env-key", "env-decision", "env-text", "systemone", Sources{fromEnv, fromEnv, fromEnv, fromEnv}}},
		{"env over file", entry, []Overrides{env}, Inference{"pool", "https://env.example/v1", "env-key", "env-decision", "env-text", "systemone", Sources{fromEnv, fromEnv, fromEnv, fromEnv}}},
		{"file alone", entry, nil, Inference{"pool", "https://file.example/v1", "file-key", "file-decision", "file-text", "systemone", Sources{fromPool, fromConfig, fromConfig, fromPool}}},
		{"env alone", nil, []Overrides{env}, Inference{"environment", "https://env.example/v1", "env-key", "env-decision", "env-text", "systemone", Sources{fromEnv, fromEnv, fromEnv, fromEnv}}},
		{"a specific variable overrides its one field", entry, []Overrides{{Decision: "env-decision"}}, Inference{"pool", "https://file.example/v1", "file-key", "env-decision", "file-text", "systemone", Sources{fromPool, fromEnv, fromConfig, fromPool}}},
		{"both specifics absent resolves from the file only", entry, []Overrides{{APIKeyFile: keyFile(t, "env-key")}}, Inference{"pool", "https://file.example/v1", "env-key", "file-decision", "file-text", "systemone", Sources{fromPool, fromConfig, fromConfig, fromEnv}}},
		{"an entry naming its decision surface", decisionEntry, nil, Inference{"pool", "https://file.example/v1", "file-key", "file-decision", "file-text", "decisions", Sources{fromPool, fromConfig, fromConfig, fromPool}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := files
			if tc.entry == nil {
				cfg = Config{}
			}
			got, err := ResolveInference(cfg, tc.entry, nil, t.TempDir(), tc.layers...)
			if err != nil || got != tc.want {
				t.Errorf("= %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

// TestEnvOverrides: the environment's layer reads each variable into its
// field, trimmed.
func TestEnvOverrides(t *testing.T) {
	getenv := func(name string) string {
		switch name {
		case EnvAPIBaseURL:
			return " http://env.example/v1 "
		case EnvDecision:
			return " clef "
		case EnvText:
			return "Qwen3.8-27B"
		case EnvAPIKeyFile:
			return "/keys/reaper.txt"
		default:
			return ""
		}
	}
	want := Overrides{BaseURL: "http://env.example/v1", Decision: "clef", Text: "Qwen3.8-27B", APIKeyFile: "/keys/reaper.txt"}
	if got := EnvOverrides(getenv); got != want {
		t.Errorf("EnvOverrides = %+v, want %+v", got, want)
	}
	if got := EnvOverrides(func(string) string { return "" }); got != (Overrides{}) {
		t.Errorf("EnvOverrides with an empty environment = %+v, want the zero layer", got)
	}
}

func TestResolveInferenceNamesWhatItLacks(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "absent.txt")
	providers, configFile := ProvidersPath(dir), ConfigPath(dir)
	for _, tc := range []struct {
		name      string
		entry     *Provider
		layers    []Overrides
		key, path string
		detail    string
	}{
		{"no base URL", nil, nil, "baseUrl", providers,
			"KBASE_API_BASE_URL is unset and " + providers + " supplies no base URL: no usable provider"},
		{"no model", nil, []Overrides{{BaseURL: "https://env.example/v1"}}, "models.decision", configFile,
			configFile + " sets no models.decision or models.text"},
		{"no text model", nil, []Overrides{{BaseURL: "https://env.example/v1", Decision: "clef"}}, "models.text", configFile,
			configFile + " sets no models.text"},
		{"an unreadable key file", nil, []Overrides{{BaseURL: "https://env.example/v1", Decision: "clef", Text: "t", APIKeyFile: missing}}, EnvAPIKeyFile, missing, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveInference(Config{}, tc.entry, errors.New("no usable provider"), dir, tc.layers...)
			var lack LackError
			if !errors.As(err, &lack) {
				t.Fatalf("err = %v, want a LackError", err)
			}
			if key, path := lack.Named(); key != tc.key || path != tc.path || (tc.detail != "" && lack.Detail != tc.detail) {
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
