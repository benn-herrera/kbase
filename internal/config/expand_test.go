package config

import (
	"os"
	"path/filepath"
	"testing"
)

// stagedHome points the home directory lookup at a temp directory — whichever
// variable the platform reads — and returns it. layer_test.go's homeDir reads
// the answer back; this one is for a test that knows what the answer is.
func stagedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// TestExpandHome: a leading ~ expands from the home directory — ~ alone is the
// home directory itself — while ~user and a ~ anywhere but the start are
// literal. Where there is no home directory, or the one given is not absolute,
// the string stays exactly as written.
func TestExpandHome(t *testing.T) {
	for _, tc := range []struct {
		name, home, path, want string
	}{
		{"a path beneath the home directory", "/home/reader", "~/k", "/home/reader/k"},
		{"the home directory itself", "/home/reader", "~", "/home/reader"},
		{"a user's own directory is not supported", "/home/reader", "~other/y", "~other/y"},
		{"a ~ inside a path is literal", "/home/reader", "a/~/b", "a/~/b"},
		{"a path with no ~ at all", "/home/reader", "relative/k", "relative/k"},
		{"an absolute path", "/home/reader", "/abs/k", "/abs/k"},
		{"no home directory", "", "~/k", "~/k"},
		{"a home directory that is not absolute", "relative/home", "~/k", "~/k"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", tc.home)
			t.Setenv("USERPROFILE", tc.home)
			if got := expandHome(tc.path); got != tc.want {
				t.Errorf("expandHome(%q) with home %q = %q, want %q", tc.path, tc.home, got, tc.want)
			}
		})
	}
}

// TestExpandHomeKeyFile: a ~-expanded apiKeyFile is read under the home
// directory and never joined to the providers.toml directory, while a ~ the
// entry spells anywhere but the start stays literal and resolves beside that
// file.
func TestExpandHomeKeyFile(t *testing.T) {
	home := stagedHome(t)
	keys := filepath.Join(home, "keys")
	if err := os.MkdirAll(keys, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, keys, "home.txt", fakeKey)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "~other"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "~other"), "key.txt", fakeKey)
	writeProviders(t, dir, `
[homekey]
baseUrl = "http://home.example/v1"
apiKeyFile = "~/keys/home.txt"

[tildeuser]
baseUrl = "http://user.example/v1"
apiKeyFile = "~other/key.txt"
`)
	providers, faults, err := LoadProviders(ProvidersPath(dir))
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(faults) != 0 {
		t.Fatalf("faults = %+v, want none: the home key file and the literal one both read", faults)
	}
	if got := providers["homekey"].APIKey; got != fakeKey {
		t.Errorf("homekey.APIKey (len %d), want the key under the home directory", len(got))
	}
	if got := keyFilePath(providers["homekey"], dir); got != filepath.Join(home, "keys", "home.txt") {
		t.Errorf("keyFilePath = %q, want %q — a ~-expanded path is never joined to the config directory",
			got, filepath.Join(home, "keys", "home.txt"))
	}
	if got := providers["tildeuser"].APIKey; got != fakeKey {
		t.Errorf("tildeuser.APIKey (len %d), want the key its literal ~other/ names beside the config file", len(got))
	}
	// The report keeps the spelling each entry gives.
	if got := providers["homekey"].APIKeyFile; got != "~/keys/home.txt" {
		t.Errorf("APIKeyFile = %q, want the spelling the entry gives", got)
	}
}

// TestExpandHomeConfigDir: a configuration directory a flag or the environment
// names expands the same way, and is the only layer read.
func TestExpandHomeConfigDir(t *testing.T) {
	home := stagedHome(t)
	want := filepath.Join(home, "cfg")
	for _, route := range []struct{ name, flag, env, wantSource string }{
		{"the flag", "~/cfg", "", LayerSourceExplicit},
		{"the environment", "", "~/cfg", LayerSourceEnvironment},
	} {
		t.Run(route.name, func(t *testing.T) {
			t.Setenv(EnvConfigDir, route.env)
			got, err := Layers(route.flag, t.TempDir())
			if err != nil {
				t.Fatalf("Layers: %v", err)
			}
			wantLayers := []Layer{{Path: want, Source: route.wantSource}}
			if len(got) != 1 || got[0] != wantLayers[0] {
				t.Errorf("Layers(%q) with %s=%q = %+v, want %+v", route.flag, EnvConfigDir, route.env, got, wantLayers)
			}
		})
	}
}
