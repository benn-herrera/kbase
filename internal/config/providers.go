package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// Provider kind and wire protocol. kbase is an inference-only appliance —
// one endpoint kind, one protocol — so the pair is a compatibility surface
// with the wider providers.toml format rather than a choice. Both fields
// are OPTIONAL and default to the values below; an entry that states them
// explicitly is accepted, and an entry that states something else is
// dropped with a named reason (see Provider.Validate) rather than being
// silently pointed at an endpoint that cannot serve it.
const (
	// ProviderTypeInference is an LLM endpoint: chat and model listing.
	// The default, and the only kind kbase can use.
	ProviderTypeInference = "inference"

	// ProviderAPIOpenAI is the OpenAI-compatible HTTP protocol. The
	// default, and the only protocol kbase speaks.
	ProviderAPIOpenAI = "openai"

	// keyFileExposedBits are the mode bits that make an apiKeyFile
	// readable by an account other than its owner. A credential every
	// local user can read has effectively already left the machine, which
	// is worth saying out loud even though kbase still loads it.
	keyFileExposedBits = 0o044
)

// A Provider declares WHERE and HOW to reach an endpoint — never WHICH
// MODEL to use. Providers rotate their catalogues constantly, so a model
// id pinned to a pool entry is a field that begs to go stale. The model
// choice lives in config.toml [models], which is the one place a user
// looks for it.
type Provider struct {
	Name    string `toml:"-"` // table header from TOML; populated post-decode
	BaseURL string `toml:"baseUrl"`

	// Type is the provider kind. Optional; empty means
	// ProviderTypeInference.
	Type string `toml:"type"`

	// API is the wire protocol. Optional; empty means ProviderAPIOpenAI.
	API string `toml:"api"`

	// APIKeyUnsafe is an inline API key. Discouraged — it places a secret
	// directly in providers.toml, making the file unsafe to scan. Prefer
	// APIKeyFile.
	APIKeyUnsafe string `toml:"apiKeyUnsafe"`

	// APIKeyFile is a path to a file holding the API key. Relative paths
	// resolve against the providers.toml directory. This is the
	// recommended form: the secret stays out of providers.toml, so the
	// config file itself is safe to scan and edit.
	APIKeyFile string `toml:"apiKeyFile"`

	// APIKey is the resolved key — populated by LoadProviders from
	// APIKeyFile (preferred) or APIKeyUnsafe. Not a TOML field.
	APIKey string `toml:"-"`
}

// Providers is a name-keyed set of providers loaded from providers.toml.
type Providers map[string]Provider

// ProviderFault names a provider that parsed correctly but did not load
// cleanly — an unusable declaration, an unreadable apiKeyFile, or a key
// file anyone on the machine can read. The fault lets the caller surface
// the problem without discarding the rest of the pool.
type ProviderFault struct {
	Name   string
	Reason string // never carries key content — path/IO detail only

	// Warning distinguishes a fault the provider SURVIVED from one that
	// dropped it. A warning entry is still in the Providers map and still
	// usable; it exists to tell the user about something they should fix,
	// not to explain a missing endpoint. The zero value is the dropping
	// kind, so a fault built without considering this stays the strict
	// one.
	Warning bool
}

// Kind returns the normalized (trimmed, lower-cased) provider type,
// defaulting to ProviderTypeInference when unstated. It exists so
// selection sites compare against the constants without each one
// re-deciding how to normalize a hand-edited TOML value.
func (p Provider) Kind() string {
	if k := strings.ToLower(strings.TrimSpace(p.Type)); k != "" {
		return k
	}
	return ProviderTypeInference
}

// Protocol returns the normalized wire protocol, defaulting to
// ProviderAPIOpenAI when unstated.
func (p Provider) Protocol() string {
	if a := strings.ToLower(strings.TrimSpace(p.API)); a != "" {
		return a
	}
	return ProviderAPIOpenAI
}

// Validate reports whether a pool entry is usable, checking the fields
// that decide WHERE traffic goes rather than the ones that merely
// decorate it.
//
// It is enforced at POOL LOAD, where an invalid entry is dropped and
// reported as a ProviderFault: the rest of the pool still loads, and the
// user gets a named reason instead of an endpoint that quietly went
// missing from the selection list.
//
// Reasons quote the offending type/api value — never a credential field —
// so the message identifies the typo it is complaining about.
func (p Provider) Validate() error {
	if strings.TrimSpace(p.BaseURL) == "" {
		return errors.New("no baseUrl declared")
	}
	if kind := p.Kind(); kind != ProviderTypeInference {
		return fmt.Errorf("unknown type %q (known types: %s)", p.Type, ProviderTypeInference)
	}
	if api := p.Protocol(); api != ProviderAPIOpenAI {
		return fmt.Errorf("api %q is not valid for a %s provider (which speaks: %s)",
			p.API, ProviderTypeInference, ProviderAPIOpenAI)
	}
	return nil
}

// LoadProviders reads providers.toml at path and returns a name-keyed
// map. Each provider's API key is resolved: from APIKeyFile when set
// (read relative to the providers.toml directory), otherwise from
// APIKeyUnsafe.
//
// A nonexistent or empty file yields an empty Providers, no faults, and a
// nil error — a fresh install has no populated providers.toml yet.
//
// The error return is reserved for file-level failures only: the
// providers.toml file unreadable (non-not-exist) or malformed TOML. A
// provider that is individually unusable — a mismatched `type`/`api`
// pair, a missing baseUrl, or an apiKeyFile that cannot be read — does
// NOT abort the load: it is omitted from the returned map and appended to
// the returned []ProviderFault so the rest of the pool still loads.
//
// A loaded provider may also carry a WARNING fault (ProviderFault.Warning)
// — today, a key file readable beyond its owner. It stays in the map and
// stays usable; the fault is there to be reported, not to gate anything.
//
// Errors and faults are constructed without TOML value content or key
// material — a parse error references line/column, and a key-file read
// failure references the path, so no secret leaks.
func LoadProviders(path string) (Providers, []ProviderFault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Providers{}, nil, nil
		}
		return nil, nil, fmt.Errorf("providers: read %s: %w", path, err)
	}

	raw := map[string]Provider{}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		// Wrapped as-is on purpose. BurntSushi's ParseError.Error() reports
		// line, column and key; its ErrorWithPosition() additionally prints
		// the OFFENDING SOURCE LINE — which for a malformed apiKeyUnsafe
		// entry is the secret itself, in a message headed for the console
		// and any log behind it. Do not "improve" this error with it.
		return nil, nil, fmt.Errorf("providers: parse %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	out := make(Providers, len(raw))
	var faults []ProviderFault
	for name, p := range raw {
		p.Name = name
		// Shape before secrets: an entry nothing can route to is dropped
		// with a named reason rather than silently vanishing from the
		// selection lists it would never have matched. Checked first
		// because there is no point reading a key file for it.
		if err := p.Validate(); err != nil {
			faults = append(faults, ProviderFault{Name: name, Reason: err.Error()})
			continue
		}
		key, err := resolveAPIKey(p, dir)
		if err != nil {
			faults = append(faults, ProviderFault{Name: name, Reason: err.Error()})
			continue
		}
		if reason := keyFileExposure(p, dir); reason != "" {
			faults = append(faults, ProviderFault{Name: name, Reason: reason, Warning: true})
		}
		p.APIKey = key
		out[name] = p
	}
	return out, faults, nil
}

// resolveAPIKey returns the provider's API key from APIKeyFile
// (preferred) or APIKeyUnsafe. The key file's content is trimmed of
// surrounding whitespace. The returned error never carries key content —
// only the file path, on an I/O failure.
func resolveAPIKey(p Provider, dir string) (string, error) {
	if p.APIKeyFile == "" {
		return p.APIKeyUnsafe, nil
	}
	b, err := os.ReadFile(keyFilePath(p, dir))
	if err != nil {
		return "", fmt.Errorf("read apiKeyFile: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// keyFilePath resolves the provider's apiKeyFile, relative paths against
// the providers.toml directory.
func keyFilePath(p Provider, dir string) string {
	if filepath.IsAbs(p.APIKeyFile) {
		return p.APIKeyFile
	}
	return filepath.Join(dir, p.APIKeyFile)
}

// keyFileExposure returns a warning reason when the provider's key file is
// readable by group or other, and "" when it is not.
//
// A warning, not a refusal: ssh's hard refusal is right for a daemon's
// identity, too strict for an appliance the user runs by hand — and kbase
// refusing to talk to a provider whose key it just read successfully would
// be a puzzle, not a safeguard. The reason names the path and the mode,
// never the file's content.
func keyFileExposure(p Provider, dir string) string {
	// Windows file modes are synthesized (0666 / 0444) rather than real
	// permission bits, so this check would fire on every install there and
	// teach the user to ignore it. The ACL is the thing that would have to
	// be read instead.
	if p.APIKeyFile == "" || runtime.GOOS == "windows" {
		return ""
	}
	path := keyFilePath(p, dir)
	info, err := os.Stat(path)
	if err != nil {
		// The read above already succeeded, so a stat failure here is a
		// race with something else on the filesystem, not a fact about the
		// user's setup. One complaint per problem.
		return ""
	}
	mode := info.Mode().Perm()
	if mode&keyFileExposedBits == 0 {
		return ""
	}
	return fmt.Sprintf("apiKeyFile %s is readable beyond its owner (mode %#o) — chmod 600 it", path, mode)
}
