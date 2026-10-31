package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/secret"
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

// exposedKeyFile writes a key file holding body with group- and other-read
// bits set, and returns its path. A test that needs the exposure to be real
// skips where the filesystem will not hold the mode.
func exposedKeyFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := writeFile(t, dir, name, body)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Skipf("cannot chmod a fixture in this environment: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat key fixture: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		// Windows synthesizes modes, so there is no exposure to detect.
		t.Skipf("the filesystem did not take the mode (got %v); nothing to test", info.Mode().Perm())
	}
	return path
}

func keysOf(p Providers) []string {
	out := make([]string, 0, len(p))
	for k := range p {
		out = append(out, k)
	}
	return out
}

// TestLoadProvidersResolvesKeys: each entry's key is the content of the file
// its relative apiKeyFile names, resolved against the providers.toml
// directory and trimmed of surrounding whitespace.
func TestLoadProvidersResolvesKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "local.key", fakeKey)
	writeFile(t, dir, "reaper.key", "  "+fakeKey+"\n")
	path := writeProviders(t, dir, `
[local]
baseUrl = "http://127.0.0.1:8080/v1"
apiKeyFile = "local.key"

[reaper]
baseUrl = "https://api.example.com/v1"
apiKeyFile = "reaper.key"
type = "inference"
api = "openai"
`)

	got, faults, err := LoadProviders(path, nil)
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
		if p.APIKey.Reveal() != fakeKey {
			t.Errorf("%s: APIKey not resolved (len %d)", name, len(p.APIKey.Reveal()))
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
	got, faults, err := LoadProviders(path, nil)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(faults) != 0 {
		t.Fatalf("unexpected faults: %+v", faults)
	}
	if got["reaper"].APIKey.Reveal() != fakeKey {
		t.Errorf("absolute apiKeyFile did not resolve (len %d)", len(got["reaper"].APIKey.Reveal()))
	}
}

// TestLoadProvidersUnreadableKeyFile: a broken entry is dropped as exactly
// one named fault, the healthy entry beside it still loads, and the fault
// reason carries the path but no key material.
func TestLoadProvidersUnreadableKeyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "healthy.key", fakeKey+"\n")
	path := writeProviders(t, dir, `
[healthy]
baseUrl = "http://ok/v1"
apiKeyFile = "healthy.key"

[broken]
baseUrl = "http://x/v1"
apiKeyFile = "not-here.key"
`)
	got, faults, err := LoadProviders(path, nil)
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
	if !strings.Contains(faults[0].Reason, "key file") || !strings.Contains(faults[0].Reason, "not-here.key") {
		t.Errorf("fault reason (len %d) does not name the key file by its path", len(faults[0].Reason))
	}
	if strings.Contains(faults[0].Reason, fakeKey) {
		t.Error("ProviderFault.Reason carries key material")
	}
	if faults[0].Warning {
		t.Error("a dropped provider was reported as a warning")
	}
	if !faults[0].KeyFileUnread {
		t.Error("the fault of a key file that could not be read is not marked KeyFileUnread")
	}
}

// TestLoadProvidersExposedKeyFile: a key file the whole machine can read is
// worth saying out loud — but the key resolved, so the provider still
// loads. A refusal here would be kbase declining to use a credential it
// just read successfully, which is a puzzle rather than a safeguard.
func TestLoadProvidersExposedKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := exposedKeyFile(t, dir, "reaper.key", fakeKey+"\n")
	path := writeProviders(t, dir, `
[reaper]
baseUrl = "https://api.example.com/v1"
apiKeyFile = "reaper.key"
`)

	got, faults, err := LoadProviders(path, nil)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if p, ok := got["reaper"]; !ok || p.APIKey.Reveal() != fakeKey {
		t.Fatal("an exposed key file is a warning, not a refusal: the provider must still load with its key")
	}
	if len(faults) != 1 || faults[0].Name != "reaper" || !faults[0].Warning {
		t.Fatalf("want exactly one warning fault naming reaper, got %+v", faults)
	}
	if !strings.Contains(faults[0].Reason, keyPath) || !strings.Contains(faults[0].Reason, "0644") {
		t.Errorf("fault reason %q names neither the path nor the mode", faults[0].Reason)
	}
	if strings.Contains(faults[0].Reason, fakeKey) {
		t.Error("ProviderFault.Reason carries key material")
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
	if _, faults, err := LoadProviders(path, nil); err != nil || len(faults) != 0 {
		t.Errorf("a 0600 key file produced %+v (err %v)", faults, err)
	}
}

// TestLoadProvidersKeyPrecedence: an entry's key is the one stored under its
// name, lowercased as the store names its files; else its apiKeyFile's; else
// none. An entry with a stored key never reads its apiKeyFile, so a missing or
// exposed one raises no fault.
func TestLoadProvidersKeyPrecedence(t *testing.T) {
	const storedKey, fileKey = "stored-key-456", "file-key-789"
	dir := t.TempDir()
	writeFile(t, dir, "readable.key", fileKey+"\n")
	exposedKeyFile(t, dir, "exposed.key", fileKey+"\n")
	path := writeProviders(t, dir, `
[Missing]
baseUrl = "http://missing.example/v1"
apiKeyFile = "not-here.key"

[exposed]
baseUrl = "http://exposed.example/v1"
apiKeyFile = "exposed.key"

[storedonly]
baseUrl = "http://storedonly.example/v1"

[fileonly]
baseUrl = "http://fileonly.example/v1"
apiKeyFile = "readable.key"

[nokey]
baseUrl = "http://nokey.example/v1"
`)
	stored := map[string]secret.Key{"missing": secret.NewKey(storedKey), "exposed": secret.NewKey(storedKey), "storedonly": secret.NewKey(storedKey)}

	got, faults, err := LoadProviders(path, stored)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(faults) != 0 {
		t.Errorf("faults = %+v, want none: a stored key leaves the apiKeyFile unread", faults)
	}
	for _, tc := range []struct {
		name, key string
		kind      KeyKind
	}{
		{"Missing", storedKey, KeyKindStore},
		{"exposed", storedKey, KeyKindStore},
		{"storedonly", storedKey, KeyKindStore},
		{"fileonly", fileKey, KeyKindFile},
		{"nokey", "", KeyKindNone},
	} {
		p, ok := got[tc.name]
		if !ok {
			t.Errorf("%s is not in the pool", tc.name)
			continue
		}
		if p.APIKey.Reveal() != tc.key {
			t.Errorf("%s: the key (len %d) is not the %s key (len %d)", tc.name, len(p.APIKey.Reveal()), tc.kind, len(tc.key))
		}
		if k := keyKind(p); k != tc.kind {
			t.Errorf("%s: key kind %s, want %s", tc.name, k, tc.kind)
		}
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
		{"no baseUrl", "apiKeyFile = \"never-read.key\"\n", "no baseUrl declared"},
		{"unknown type", "baseUrl = \"http://x/v1\"\ntype = \"telepathy\"\n", "unknown type"},
		{"unknown api", "baseUrl = \"http://x/v1\"\napi = \"exa\"\n", "not valid for a inference provider"},
		{"unknown decisionsFlavor", "baseUrl = \"http://x/v1\"\ndecisionsFlavor = \"systemtwo\"\n", "known surfaces: systemone, decisions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeProviders(t, dir,
				"[healthy]\nbaseUrl = \"http://ok/v1\"\n\n[suspect]\n"+tc.entry)

			got, faults, err := LoadProviders(path, nil)
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

// TestProviderValidateDefaults: type, api and decisionsFlavor are optional
// compatibility fields; absent, mixed-case and space-padded spellings of the
// one legal pairing all validate, and Kind/Protocol/Flavor report the
// normalized value so call sites can compare against the constants.
func TestProviderValidateDefaults(t *testing.T) {
	for _, tc := range []struct {
		name       string
		provider   Provider
		wantOK     bool
		wantAPI    string
		wantFlavor string
	}{
		{"both absent", Provider{BaseURL: "http://x/v1"}, true, "", ""},
		{"both explicit", Provider{BaseURL: "http://x/v1", Type: ProviderTypeInference, API: ProviderAPIOpenAI}, true, ProviderAPIOpenAI, ""},
		{"type only", Provider{BaseURL: "http://x/v1", Type: ProviderTypeInference}, true, "", ""},
		{"api only", Provider{BaseURL: "http://x/v1", API: ProviderAPIOpenAI}, true, ProviderAPIOpenAI, ""},
		{"normalized spelling", Provider{BaseURL: "http://x/v1", Type: " Inference ", API: "OpenAI"}, true, ProviderAPIOpenAI, ""},
		{"decisions flavour", Provider{BaseURL: "http://x/v1", DecisionsFlavor: FlavorDecisions}, true, "", FlavorDecisions},
		{"flavour, normalized spelling", Provider{BaseURL: "http://x/v1", DecisionsFlavor: " Decisions "}, true, "", FlavorDecisions},
		{"systemone flavour", Provider{BaseURL: "http://x/v1", DecisionsFlavor: FlavorSystemOne}, true, "", FlavorSystemOne},
		{"wrong type", Provider{BaseURL: "http://x/v1", Type: "search"}, false, "", ""},
		{"wrong api", Provider{BaseURL: "http://x/v1", API: "exa"}, false, "", ""},
		{"unknown decisionsFlavor", Provider{BaseURL: "http://x/v1", DecisionsFlavor: "systemtwo"}, false, "", ""},
		{"no baseUrl", Provider{Type: ProviderTypeInference}, false, "", ""},
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
			// An entry silent on the protocol speaks the Messages protocol.
			wantAPI := tc.wantAPI
			if wantAPI == "" {
				wantAPI = ProviderAPIAnthropic
			}
			if a := tc.provider.Protocol(); a != wantAPI {
				t.Errorf("Protocol() = %q, want %q", a, wantAPI)
			}
			// An entry silent on the flavour speaks the SystemOne surface.
			wantFlavor := tc.wantFlavor
			if wantFlavor == "" {
				wantFlavor = FlavorSystemOne
			}
			if f := tc.provider.Flavor(); f != wantFlavor {
				t.Errorf("Flavor() = %q, want %q", f, wantFlavor)
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
			got, faults, err := LoadProviders(writeProviders(t, t.TempDir(), tc.body), nil)
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
		got, faults, err := LoadProviders(filepath.Join(t.TempDir(), ProvidersFileName), nil)
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
	_, _, err := LoadProviders(path, nil)
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
// load refuses and names it, listing every offender at once. A field kbase
// no longer models (apiKeyUnsafe) is refused the same way, by name alone:
// providers.toml is not to hold credentials, and a file that still spells
// one must not have its value echoed back.
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
	_, _, err := LoadProviders(path, nil)
	if err == nil {
		t.Fatal("a misspelled provider field left the entry unroutable and the load allowed it: got nil error")
	}
	msg := err.Error()
	for _, want := range []string{"providers:", path, `"local.apiKeyUnsafe"`, `"reaper.url"`, `"reaper.apiKeyPath"`, unknownKeyHint} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal (len %d) does not name %s", len(msg), want)
		}
	}
	if strings.Contains(msg, fakeKey) {
		t.Error("refusal carries key material")
	}
}

// TestLoadProvidersFieldCaseInsensitive: TOML field matching is
// case-insensitive, so "baseURL" is the field, not a typo. Pinning that
// here keeps a future strictness pass from turning a working config into a
// refusal.
func TestLoadProvidersFieldCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "reaper.key", fakeKey+"\n")
	path := writeProviders(t, dir, `
[reaper]
BaseURL = "https://api.example.com/v1"
APIKEYFILE = "reaper.key"
`)
	got, faults, err := LoadProviders(path, nil)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(faults) != 0 {
		t.Fatalf("unexpected faults: %+v", faults)
	}
	if p := got["reaper"]; p.BaseURL == "" || p.APIKey.Reveal() != fakeKey {
		t.Errorf("case-variant field spellings did not decode: %+v", Provider{BaseURL: p.BaseURL})
	}
}

// TestLoadProvidersDynamicNames: provider names are the user's own, so a name
// the schema has never seen is no unknown key — only the fields beneath it are
// checked. Any name the provider-name rule admits loads without complaint.
func TestLoadProvidersDynamicNames(t *testing.T) {
	path := writeProviders(t, t.TempDir(), `
[some-box_2]
baseUrl = "http://127.0.0.1:8080/v1"

[_Lab-9]
baseUrl = "http://127.0.0.1:8081/v1"

[42]
baseUrl = "http://127.0.0.1:8082/v1"
`)
	got, faults, err := LoadProviders(path, nil)
	if err != nil {
		t.Fatalf("a provider name is not a schema key: %v", err)
	}
	if len(faults) != 0 {
		t.Errorf("unexpected faults: %+v", faults)
	}
	if len(got) != 3 {
		t.Errorf("expected every entry, got %v", keysOf(got))
	}
}

// TestLoadProvidersRefusesUnusableNames: an entry whose name breaks the
// provider-name rule — a quoted header holding a dot, a space or a non-ASCII
// letter, an empty one, or a leading "-" — fails the file's load, naming the
// file, the entry and the rule, before any key file is read.
func TestLoadProvidersRefusesUnusableNames(t *testing.T) {
	for _, name := range []string{"quoted.name", "with space", "naïve", "", "-lead", "colon:name", "slash/name"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeProviders(t, dir, fmt.Sprintf("[%q]\nbaseUrl = \"http://x.example/v1\"\napiKeyFile = \"not-here.key\"\n\n[fine]\nbaseUrl = \"http://y.example/v1\"\n", name))
			got, faults, err := LoadProviders(path, nil)
			if err == nil {
				t.Fatalf("the load allowed it: pool %v, faults %+v", keysOf(got), faults)
			}
			msg := err.Error()
			for _, want := range []string{"providers:", path, fmt.Sprintf("%q", name), ProviderNameRule} {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal %q does not name %s", msg, want)
				}
			}
		})
	}
}

// TestLoadProvidersRefusesCaseOnlyNames: two entries of one file whose names
// differ only by case fail its load, naming both spellings and the file.
func TestLoadProvidersRefusesCaseOnlyNames(t *testing.T) {
	path := writeProviders(t, t.TempDir(), `
[Reaper]
baseUrl = "http://a.example/v1"

[reaper]
baseUrl = "http://b.example/v1"
`)
	_, _, err := LoadProviders(path, nil)
	if err == nil {
		t.Fatal("the load allowed two names differing only by case")
	}
	for _, want := range []string{"providers:", path, `"Reaper"`, `"reaper"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %s", err, want)
		}
	}
}

// TestLoadProvidersKeyAbsentFromFaults: a fault names a key file by its path
// and mode, never by its content. The sentinel key really is read — the
// usable entry carries it — and appears in no fault reason, whichever fault
// a provider collects.
func TestLoadProvidersKeyAbsentFromFaults(t *testing.T) {
	const sentinel = "sk-SECRET-MUST-NOT-LEAK-9f3a0b"
	dir := t.TempDir()
	exposedKeyFile(t, dir, "reaper.key", sentinel+"\n")
	path := writeProviders(t, dir, `[reaper]
baseUrl = "https://api.example.com/v1"
apiKeyFile = "reaper.key"

[broken]
baseUrl = "http://x/v1"
apiKeyFile = "not-here.key"
`)
	got, faults, err := LoadProviders(path, nil)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if got["reaper"].APIKey.Reveal() != sentinel {
		t.Fatal("the fixture's key file did not resolve, so the fault text proves nothing")
	}
	if len(faults) != 2 {
		t.Fatalf("want the dropped entry and the exposure warning, got %+v", faults)
	}
	for _, f := range faults {
		if strings.Contains(f.Reason, sentinel) {
			t.Errorf("ProviderFault.Reason of %s carries key material", f.Name)
		}
	}
}

// TestProviderValidateProtocols: `api` is a two-value field — anthropic, the
// Messages protocol and the default, and openai — and an entry naming anything
// else is refused listing both.
func TestProviderValidateProtocols(t *testing.T) {
	for _, tc := range []struct {
		name   string
		api    string
		wantOK bool
	}{
		{"openai", ProviderAPIOpenAI, true},
		{"anthropic", ProviderAPIAnthropic, true},
		{"normalized spelling", " Anthropic ", true},
		{"neither", "grpc", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Provider{BaseURL: "http://x/v1", API: tc.api}
			err := p.Validate()
			if gotOK := err == nil; gotOK != tc.wantOK {
				t.Fatalf("Validate() = %v, want ok = %t", err, tc.wantOK)
			}
			if !tc.wantOK {
				for _, want := range []string{"grpc", ProviderAPIOpenAI, ProviderAPIAnthropic} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q does not name %q", err, want)
					}
				}
				return
			}
			if got, want := p.Protocol(), strings.ToLower(strings.TrimSpace(tc.api)); got != want {
				t.Errorf("Protocol() = %q, want %q", got, want)
			}
		})
	}
}

// TestLoadProvidersAdmitsAMessagesEntry: an `api = "anthropic"` entry is a
// usable pool entry, not a value the loader drops.
func TestLoadProvidersAdmitsAMessagesEntry(t *testing.T) {
	dir := t.TempDir()
	path := writeProviders(t, dir, `
[claude]
baseUrl = "https://api.anthropic.example/v1"
api = "anthropic"
`)
	got, faults, err := LoadProviders(path, nil)
	if err != nil || len(faults) != 0 {
		t.Fatalf("LoadProviders = %v, %v; want the entry loaded", faults, err)
	}
	if entry, ok := got["claude"]; !ok || entry.Protocol() != ProviderAPIAnthropic {
		t.Errorf("pool = %v, claude speaks %q; want the anthropic entry", keysOf(got), entry.Protocol())
	}
}

// TestLoadProvidersProtocolDefault: an entry silent on `api` loads speaking the
// Messages protocol, and `api = "openai"` is what selects the OpenAI-compatible
// one.
func TestLoadProvidersProtocolDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeProviders(t, dir, `
[silent]
baseUrl = "http://silent.example/v1"

[compat]
baseUrl = "http://compat.example/v1"
api = "openai"
`)
	got, faults, err := LoadProviders(path, nil)
	if err != nil || len(faults) != 0 {
		t.Fatalf("LoadProviders = %v, %v; want both entries loaded", faults, err)
	}
	for name, want := range map[string]string{"silent": ProviderAPIAnthropic, "compat": ProviderAPIOpenAI} {
		if p := got[name].Protocol(); p != want {
			t.Errorf("%s speaks %q, want %q", name, p, want)
		}
	}
}
