package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeKey is the placeholder credential used throughout these tests. It is
// deliberately obvious: assertions may safely name it, which a real-shaped
// secret must never be.
const fakeKey = "test-key-123"

// writeFile writes a fixture file under dir and returns its path.
func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// writeProviders writes a providers.toml fixture and returns its path.
func writeProviders(t *testing.T, dir, body string) string {
	t.Helper()
	return writeFile(t, dir, ProvidersFileName, body)
}

func keysOf(p Providers) []string {
	out := make([]string, 0, len(p))
	for k := range p {
		out = append(out, k)
	}
	return out
}

// TestLoadProvidersResolvesKeys covers both credential forms in one pool:
// the inline apiKeyUnsafe and the recommended apiKeyFile, whose relative
// path resolves against the providers.toml directory.
func TestLoadProvidersResolvesKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "reaper.key", "  "+fakeKey+"\n")
	path := writeProviders(t, dir, `
[local]
baseUrl = "http://127.0.0.1:8080/v1"
apiKeyUnsafe = "`+fakeKey+`"

[reaper]
baseUrl = "https://api.example.com/v1"
apiKeyFile = "reaper.key"
type = "inference"
api = "openai"
`)

	got, faults, err := LoadProviders(path)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(faults) != 0 {
		t.Fatalf("healthy pool produced faults: %+v", faults)
	}
	for _, name := range []string{"local", "reaper"} {
		p, ok := got[name]
		if !ok {
			t.Fatalf("missing provider %q (got %v)", name, keysOf(got))
		}
		if p.Name != name {
			t.Errorf("%s: Name = %q, want the table header", name, p.Name)
		}
		if p.APIKey != fakeKey {
			t.Errorf("%s: APIKey not resolved (len %d)", name, len(p.APIKey))
		}
		if p.BaseURL == "" {
			t.Errorf("%s: BaseURL empty", name)
		}
	}
}

// TestLoadProvidersAbsoluteKeyFile: an absolute apiKeyFile is used as
// given, not joined onto the providers.toml directory.
func TestLoadProvidersAbsoluteKeyFile(t *testing.T) {
	keyDir := t.TempDir()
	keyPath := writeFile(t, keyDir, "abs.key", fakeKey)

	dir := t.TempDir()
	path := writeProviders(t, dir, `
[reaper]
baseUrl = "https://api.example.com/v1"
apiKeyFile = "`+keyPath+`"
`)
	got, faults, err := LoadProviders(path)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(faults) != 0 {
		t.Fatalf("unexpected faults: %+v", faults)
	}
	if got["reaper"].APIKey != fakeKey {
		t.Errorf("absolute apiKeyFile did not resolve (len %d)", len(got["reaper"].APIKey))
	}
}

// TestLoadProvidersUnreadableKeyFile: a broken entry is dropped as exactly
// one named fault, the healthy entry beside it still loads, and the fault
// reason carries the path but no key material.
func TestLoadProvidersUnreadableKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeProviders(t, dir, `
[healthy]
baseUrl = "http://ok/v1"
apiKeyUnsafe = "`+fakeKey+`"

[broken]
baseUrl = "http://x/v1"
apiKeyFile = "not-here.key"
`)
	got, faults, err := LoadProviders(path)
	if err != nil {
		t.Fatalf("an unreadable key file must not fail the whole load: %v", err)
	}
	if _, ok := got["broken"]; ok {
		t.Error("a provider with no resolvable key was admitted to the pool")
	}
	if _, ok := got["healthy"]; !ok {
		t.Error("one bad entry took the rest of the pool with it")
	}
	if len(faults) != 1 || faults[0].Name != "broken" {
		t.Fatalf("want exactly one fault naming broken, got %+v", faults)
	}
	if !strings.Contains(faults[0].Reason, "apiKeyFile") {
		t.Errorf("fault reason %q does not name the failing field", faults[0].Reason)
	}
	if strings.Contains(faults[0].Reason, fakeKey) {
		t.Errorf("ProviderFault.Reason carries key material: %q", faults[0].Reason)
	}
	if faults[0].Warning {
		t.Error("a dropped provider was reported as a warning")
	}
}

// TestLoadProvidersExposedKeyFile: a key file the whole machine can read is
// worth saying out loud — but the key resolved, so the provider still
// loads. A refusal here would be kbase declining to use a credential it
// just read successfully, which is a puzzle rather than a safeguard.
func TestLoadProvidersExposedKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := writeFile(t, dir, "reaper.key", fakeKey+"\n")
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Skipf("cannot chmod a fixture in this environment: %v", err)
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key fixture: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		// Windows synthesizes modes, so there is no exposure to detect.
		t.Skipf("the filesystem did not take the mode (got %v); nothing to test", info.Mode().Perm())
	}
	path := writeProviders(t, dir, `
[reaper]
baseUrl = "https://api.example.com/v1"
apiKeyFile = "reaper.key"
`)

	got, faults, err := LoadProviders(path)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if p, ok := got["reaper"]; !ok || p.APIKey != fakeKey {
		t.Fatal("an exposed key file is a warning, not a refusal: the provider must still load with its key")
	}
	if len(faults) != 1 || faults[0].Name != "reaper" || !faults[0].Warning {
		t.Fatalf("want exactly one warning fault naming reaper, got %+v", faults)
	}
	if !strings.Contains(faults[0].Reason, keyPath) || !strings.Contains(faults[0].Reason, "0644") {
		t.Errorf("fault reason %q names neither the path nor the mode", faults[0].Reason)
	}
	if strings.Contains(faults[0].Reason, fakeKey) {
		t.Errorf("ProviderFault.Reason carries key material: %q", faults[0].Reason)
	}
}

// TestLoadProvidersOwnerOnlyKeyFile: the recommended setup must be silent.
// A permission warning that fires on a correctly-permissioned file is a
// warning users learn to scroll past.
func TestLoadProvidersOwnerOnlyKeyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "reaper.key", fakeKey+"\n") // written 0600
	path := writeProviders(t, dir, `
[reaper]
baseUrl = "https://api.example.com/v1"
apiKeyFile = "reaper.key"
`)
	if _, faults, err := LoadProviders(path); err != nil || len(faults) != 0 {
		t.Errorf("a 0600 key file produced %+v (err %v)", faults, err)
	}
}

// TestLoadProvidersUnusableEntries: kbase is inference-only. An absent
// type/api pair is the DEFAULT (and must load), while an explicitly wrong
// one is dropped as a named fault listing the legal value — never silently
// absent from the selection list, which is how a "why is my provider
// gone?" afternoon starts.
func TestLoadProvidersUnusableEntries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		entry    string
		wantHint string
	}{
		{"no baseUrl", "apiKeyUnsafe = \"" + fakeKey + "\"\n", "no baseUrl declared"},
		{"unknown type", "baseUrl = \"http://x/v1\"\ntype = \"telepathy\"\n", "unknown type"},
		{"unknown api", "baseUrl = \"http://x/v1\"\napi = \"exa\"\n", "not valid for a inference provider"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeProviders(t, dir,
				"[healthy]\nbaseUrl = \"http://ok/v1\"\napiKeyUnsafe = \""+fakeKey+"\"\n\n[suspect]\n"+tc.entry)

			got, faults, err := LoadProviders(path)
			if err != nil {
				t.Fatalf("an invalid ENTRY must not fail the whole load: %v", err)
			}
			if _, ok := got["suspect"]; ok {
				t.Error("an unroutable provider was admitted to the pool")
			}
			if _, ok := got["healthy"]; !ok {
				t.Error("one bad entry took the rest of the pool with it")
			}
			if len(faults) != 1 || faults[0].Name != "suspect" {
				t.Fatalf("want exactly one fault naming suspect, got %+v", faults)
			}
			if !strings.Contains(faults[0].Reason, tc.wantHint) {
				t.Errorf("fault reason %q does not contain %q", faults[0].Reason, tc.wantHint)
			}
		})
	}
}

// TestProviderValidateDefaults: type and api are optional compatibility
// fields; absent, mixed-case and space-padded spellings of the one legal
// pairing all validate, and Kind/Protocol report the normalized value so
// call sites can compare against the constants.
func TestProviderValidateDefaults(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider Provider
		wantOK   bool
	}{
		{"both absent", Provider{BaseURL: "http://x/v1"}, true},
		{"both explicit", Provider{BaseURL: "http://x/v1", Type: ProviderTypeInference, API: ProviderAPIOpenAI}, true},
		{"type only", Provider{BaseURL: "http://x/v1", Type: ProviderTypeInference}, true},
		{"api only", Provider{BaseURL: "http://x/v1", API: ProviderAPIOpenAI}, true},
		{"normalized spelling", Provider{BaseURL: "http://x/v1", Type: " Inference ", API: "OpenAI"}, true},
		{"wrong type", Provider{BaseURL: "http://x/v1", Type: "search"}, false},
		{"wrong api", Provider{BaseURL: "http://x/v1", API: "exa"}, false},
		{"no baseUrl", Provider{Type: ProviderTypeInference}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.provider.Validate()
			if gotOK := err == nil; gotOK != tc.wantOK {
				t.Fatalf("Validate() error = %v, want ok = %v", err, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if k := tc.provider.Kind(); k != ProviderTypeInference {
				t.Errorf("Kind() = %q, want %q", k, ProviderTypeInference)
			}
			if a := tc.provider.Protocol(); a != ProviderAPIOpenAI {
				t.Errorf("Protocol() = %q, want %q", a, ProviderAPIOpenAI)
			}
		})
	}
}

// TestLoadProvidersNoLiveTables: a missing file, an empty file, and the
// comment-only template a user starts from all mean "no providers yet" —
// an empty pool, no faults, no error.
func TestLoadProvidersNoLiveTables(t *testing.T) {
	template := `# kbase inference provider configuration
#
# [reaper]
# baseUrl    = "https://api.example.com/v1"
# apiKeyFile = "reaper.key"
`
	for _, tc := range []struct{ name, body string }{
		{"empty file", ""},
		{"comment-only template", template},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, faults, err := LoadProviders(writeProviders(t, t.TempDir(), tc.body))
			if err != nil {
				t.Fatalf("LoadProviders: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("expected empty pool, got %v", keysOf(got))
			}
			if len(faults) != 0 {
				t.Errorf("expected no faults, got %+v", faults)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		got, faults, err := LoadProviders(filepath.Join(t.TempDir(), ProvidersFileName))
		if err != nil {
			t.Fatalf("a missing providers.toml is not an error: %v", err)
		}
		if len(got) != 0 || len(faults) != 0 {
			t.Errorf("expected empty pool and no faults, got %v / %+v", keysOf(got), faults)
		}
	})
}

func TestLoadProvidersMalformed(t *testing.T) {
	path := writeProviders(t, t.TempDir(), "[bad toml content\n")
	_, _, err := LoadProviders(path)
	if err == nil {
		t.Fatal("expected error from malformed TOML, got nil")
	}
	if !strings.Contains(err.Error(), "providers:") {
		t.Errorf("error not wrapped with providers prefix: %v", err)
	}
}

// TestLoadProvidersUnknownField: the table headers are provider NAMES and
// stay open, but the fields inside an entry are a fixed set. A misspelled
// one decodes into nothing, leaving an entry that points nowhere — so the
// load refuses and names it, listing every offender at once. The refusal
// must name keys only: providers.toml holds credentials.
func TestLoadProvidersUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := writeProviders(t, dir, `
[local]
baseUrl = "http://127.0.0.1:8080/v1"
apiKeyUnsafe = "`+fakeKey+`"

[reaper]
url = "https://api.example.com/v1"
apiKeyPath = "reaper.key"
`)
	_, _, err := LoadProviders(path)
	if err == nil {
		t.Fatal("a misspelled provider field left the entry unroutable and the load allowed it: got nil error")
	}
	msg := err.Error()
	for _, want := range []string{"providers:", path, `"reaper.url"`, `"reaper.apiKeyPath"`, unknownKeyHint} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not name %s", msg, want)
		}
	}
	if strings.Contains(msg, fakeKey) {
		t.Errorf("refusal carries key material: %q", msg)
	}
}

// TestLoadProvidersFieldCaseInsensitive: TOML field matching is
// case-insensitive, so "baseURL" is the field, not a typo. Pinning that
// here keeps a future strictness pass from turning a working config into a
// refusal.
func TestLoadProvidersFieldCaseInsensitive(t *testing.T) {
	path := writeProviders(t, t.TempDir(), `
[reaper]
BaseURL = "https://api.example.com/v1"
APIKEYUNSAFE = "`+fakeKey+`"
`)
	got, faults, err := LoadProviders(path)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(faults) != 0 {
		t.Fatalf("unexpected faults: %+v", faults)
	}
	if p := got["reaper"]; p.BaseURL == "" || p.APIKey != fakeKey {
		t.Errorf("case-variant field spellings did not decode: %+v", Provider{BaseURL: p.BaseURL})
	}
}

// TestLoadProvidersDynamicNames: provider names are the user's own, so no
// spelling of a table header is unknown — only the fields beneath it are
// checked. A pool of oddly-named entries must load without complaint.
func TestLoadProvidersDynamicNames(t *testing.T) {
	path := writeProviders(t, t.TempDir(), `
[some-box_2]
baseUrl = "http://127.0.0.1:8080/v1"

["quoted.name"]
baseUrl = "http://127.0.0.1:8081/v1"
`)
	got, faults, err := LoadProviders(path)
	if err != nil {
		t.Fatalf("a provider name is not a schema key: %v", err)
	}
	if len(faults) != 0 {
		t.Errorf("unexpected faults: %+v", faults)
	}
	if len(got) != 2 {
		t.Errorf("expected both entries, got %v", keysOf(got))
	}
}

// TestLoadProvidersKeyAbsentFromParseError: when one table is malformed,
// the wrapped parse error must not contain the apiKey value from a sibling
// table that did parse.
func TestLoadProvidersKeyAbsentFromParseError(t *testing.T) {
	const sentinel = "sk-SECRET-MUST-NOT-LEAK-9f3a0b"
	path := writeProviders(t, t.TempDir(), `[clean]
baseUrl = "https://api.example.com/v1"
apiKeyUnsafe = "`+sentinel+`"

[broken
this is not valid toml
`)
	_, _, err := LoadProviders(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("API key leaked into parse error: %v", err)
	}
}
