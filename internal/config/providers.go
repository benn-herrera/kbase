package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"kbase/internal/secret"
)

// Provider kind and wire protocol. kbase is an inference-only appliance — one
// endpoint kind, two chat protocols — so the pair is a compatibility surface
// with the wider providers.toml format rather than a free choice. Both fields
// are OPTIONAL and default to the values below; an entry that states them
// explicitly is accepted, and an entry that states something else is
// dropped with a named reason (see Provider.Validate) rather than being
// silently pointed at an endpoint that cannot serve it.
const (
	// ProviderTypeInference is an LLM endpoint: chat and model listing.
	// The default, and the only kind kbase can use.
	ProviderTypeInference = "inference"

	// ProviderAPIOpenAI is the OpenAI-compatible HTTP protocol: chat on
	// `chat/completions`, plus the decision surfaces.
	ProviderAPIOpenAI = "openai"

	// ProviderAPIAnthropic is the Anthropic Messages protocol: chat on
	// `POST <baseUrl>/messages` and models on `GET <baseUrl>/models`. The
	// default. It has no decision surface; the decision role asks the
	// OpenAI-shaped one on the same base URL, whatever an entry's api.
	ProviderAPIAnthropic = "anthropic"

	// FlavorSystemOne is the decision surface kbase's multichoice asks have
	// always used: `POST <baseUrl>/systemone`, one question keyed by its
	// own key, answers keyed the same way. The default for an entry that
	// states no flavour.
	FlavorSystemOne = "systemone"

	// FlavorDecisions is the OpenAI Decisions surface: `POST
	// <baseUrl>/decisions`, questions an array of named questions and
	// answers an array carrying each answer's name. It is a different wire
	// shape for the same closed-set question, chosen per server.
	FlavorDecisions = "decisions"

	// keyFileExposedBits are the mode bits that make an apiKeyFile
	// readable by an account other than its owner. A credential every
	// local user can read has effectively already left the machine, which
	// is worth saying out loud even though kbase still loads it.
	keyFileExposedBits = 0o044
)

// ProviderNamePattern is what every providers.toml entry's name matches, and
// ProviderNameRule says it in words. internal/auth names key files by the same
// rule and, importing no config, states it again; a test pins the two equal.
const (
	ProviderNamePattern = `^[A-Za-z0-9_][A-Za-z0-9_-]*$`
	ProviderNameRule    = `a provider name is one or more ASCII letters, digits, "_" or "-", not beginning with "-"`
)

var providerName = regexp.MustCompile(ProviderNamePattern)

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

	// API is the wire protocol chat uses. Optional; empty means
	// ProviderAPIAnthropic. `openai` speaks the OpenAI-compatible
	// chat-completions surface; `anthropic` speaks the Messages protocol.
	// The decision role uses the OpenAI-shaped decision surface whatever API
	// says.
	API string `toml:"api"`

	// DecisionsFlavor is which decision surface this endpoint's server
	// speaks. Optional; empty means FlavorSystemOne. It names a wire
	// shape, never a model: the decision category's id stays in
	// config.toml.
	DecisionsFlavor string `toml:"decisionsFlavor"`

	// APIKeyFile is a path to a file holding the API key. Relative paths
	// resolve against the providers.toml directory. It is the only key field
	// an entry has: the secret stays out of providers.toml, so the config
	// file itself is safe to scan and edit. A key stored under the entry's
	// name outranks it (see keyKind).
	APIKeyFile string `toml:"apiKeyFile"`

	// APIKey is the resolved key — populated by LoadProviders from the
	// credentials store or the file APIKeyFile names. Not a TOML field.
	APIKey secret.Key `toml:"-"`

	// KeyStored is whether the credentials store holds a key under the
	// entry's name. Populated by LoadProviders, never by the file.
	KeyStored bool `toml:"-"`

	// Layer is the configuration layer this entry came from — repository or
	// global, or the source of the one directory a flag or the environment
	// named. Populated by Load's merge, never by the loader, which knows only
	// the file it read.
	Layer string `toml:"-"`
}

// Providers is a name-keyed set of providers loaded from providers.toml.
type Providers map[string]Provider

// ProviderFault names a provider that parsed correctly but did not load
// cleanly — an unusable declaration, an unreadable apiKeyFile, a key
// file anyone on the machine can read, or a stored key file that did not
// open — or, with no Name, the credentials store's directory, which did not
// list. The fault lets the caller surface the problem without discarding the
// rest of the pool.
type ProviderFault struct {
	Name   string
	Reason string // never carries key content — path/IO detail only

	// Source is what the fault is against: a providers.toml entry, a stored
	// key file, or the store's directory.
	Source FaultSource

	// Path is the providers.toml this fault came from: the layer's own file,
	// never the single path a one-directory report used to assume — or, for a
	// stored key file that did not open, that file, and for the store's
	// directory, that directory.
	Path string

	// Warning distinguishes a fault the provider SURVIVED from one that
	// dropped it. A warning entry is still in the Providers map and still
	// usable; it exists to tell the user about something they should fix,
	// not to explain a missing endpoint. The zero value is the dropping
	// kind, so a fault built without considering this stays the strict
	// one.
	Warning bool

	// KeyFileUnread is the dropping fault of an entry whose apiKeyFile could
	// not be read. A stored key outranks that file, so storing one clears it.
	KeyFileUnread bool

	// DoesNotOpen is a stored key file that stands and is truncated or does
	// not decrypt, so adding the key again, which replaces the file, clears it.
	DoesNotOpen bool
}

// FaultSource is what a ProviderFault is against.
type FaultSource int

const (
	// SourceEntry is a providers.toml entry, the file being Path.
	SourceEntry FaultSource = iota
	// SourceStoreFile is the stored key file at Path, read for the entry Name.
	SourceStoreFile
	// SourceStoreDir is the store's directory at Path, which did not list;
	// Name is empty.
	SourceStoreDir
)

// LocatedReason is the fault's reason as a line that stands on its own states
// it: a store fault's led by its path, which its reason does not name; an
// entry's as it is.
func (f ProviderFault) LocatedReason() string {
	if f.Source == SourceEntry {
		return f.Reason
	}
	return f.Path + ": " + f.Reason
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
// ProviderAPIAnthropic when unstated.
func (p Provider) Protocol() string {
	if a := strings.ToLower(strings.TrimSpace(p.API)); a != "" {
		return a
	}
	return ProviderAPIAnthropic
}

// Flavor returns the normalized decision surface this entry's server speaks,
// defaulting to FlavorSystemOne when unstated — an entry that says nothing is
// a server kbase's multichoice asks already worked against.
func (p Provider) Flavor() string {
	if f := strings.ToLower(strings.TrimSpace(p.DecisionsFlavor)); f != "" {
		return f
	}
	return FlavorSystemOne
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
// Reasons quote the offending type/api/decisionsFlavor value — never a
// credential field — so the message identifies the typo it is complaining
// about.
func (p Provider) Validate() error {
	if strings.TrimSpace(p.BaseURL) == "" {
		return errors.New("no baseUrl declared")
	}
	if kind := p.Kind(); kind != ProviderTypeInference {
		return fmt.Errorf("unknown type %q (known types: %s)", p.Type, ProviderTypeInference)
	}
	if api := p.Protocol(); api != ProviderAPIOpenAI && api != ProviderAPIAnthropic {
		return fmt.Errorf("api %q is not valid for a %s provider (known protocols: %s, %s)",
			p.API, ProviderTypeInference, ProviderAPIOpenAI, ProviderAPIAnthropic)
	}
	switch p.Flavor() {
	case FlavorSystemOne, FlavorDecisions:
	default:
		return fmt.Errorf("unknown decisionsFlavor %q (known surfaces: %s, %s)",
			p.DecisionsFlavor, FlavorSystemOne, FlavorDecisions)
	}
	return nil
}

// LoadProviders reads providers.toml at path and returns a name-keyed
// map. Each provider's API key is the one storedKeys holds under the
// entry's name, lowercased as the store names its files; else the one in
// the file its APIKeyFile names, read relative to the providers.toml
// directory; else none (keyKind). An entry with a stored key never reads its
// APIKeyFile.
//
// A nonexistent or empty file yields an empty Providers, no faults, and a
// nil error — a fresh install has no populated providers.toml yet.
//
// The error return is reserved for file-level failures only: the
// providers.toml file unreadable (non-not-exist), malformed TOML, a
// field no Provider models — the table headers are provider names, an open
// set, but the fields inside them are a fixed set and a misspelled one
// refuses the load (see rejectUnknownKeys) — or a table header that breaks
// the provider-name rule or differs from another only by case (see
// rejectProviderNames). A
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
func LoadProviders(path string, storedKeys map[string]secret.Key) (Providers, []ProviderFault, error) {
	data, _, err := readProvidersFile(path)
	if err != nil {
		return nil, nil, err
	}
	return parseProviders(path, data, storedKeys)
}

// readProvidersFile returns the providers.toml at path and whether it stands:
// a file that does not is no content and no error.
func readProvidersFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("providers: read %s: %w", path, err)
	}
	return data, true, nil
}

// parseProviders is LoadProviders over data, the content of the providers.toml
// at path.
func parseProviders(path string, data []byte, storedKeys map[string]secret.Key) (Providers, []ProviderFault, error) {
	raw := map[string]Provider{}
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		// Wrapped as-is on purpose. BurntSushi's ParseError.Error() reports
		// line, column and key; its ErrorWithPosition() additionally prints
		// the OFFENDING SOURCE LINE, whose content this package does not
		// control, in a message headed for the console and any log behind
		// it. Do not "improve" this error with it.
		return nil, nil, fmt.Errorf("providers: parse %s: %w", path, err)
	}
	// A file-level refusal, like a parse error: the provider NAMES are the
	// table headers, an open set, but a misspelled field inside an entry
	// ("baseURL") would otherwise leave the entry pointing nowhere with
	// nothing said. Runs before the per-entry loop so no key file is read
	// for a file that is going to be rejected.
	if err := rejectUnknownKeys(path, md); err != nil {
		return nil, nil, fmt.Errorf("providers: %w", err)
	}
	if err := rejectProviderNames(path, slices.Sorted(maps.Keys(raw))); err != nil {
		return nil, nil, fmt.Errorf("providers: %w", err)
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
			faults = append(faults, ProviderFault{Name: name, Path: path, Reason: err.Error()})
			continue
		}
		p.APIKey, p.KeyStored = storedKeys[strings.ToLower(name)]
		if keyKind(p) == KeyKindFile {
			key, err := resolveAPIKey(p, dir)
			if err != nil {
				faults = append(faults, ProviderFault{Name: name, Path: path, Reason: err.Error(), KeyFileUnread: true})
				continue
			}
			if reason := keyFileExposure(p, dir); reason != "" {
				faults = append(faults, ProviderFault{Name: name, Path: path, Reason: reason, Warning: true})
			}
			p.APIKey = key
		}
		out[name] = p
	}
	return out, faults, nil
}

// rejectProviderNames refuses the providers.toml at path where any of its
// entry names, in sorted order, breaks the provider-name rule — naming every
// such entry — or differs from another only by case. It returns nil when
// every name is usable and distinct.
func rejectProviderNames(path string, names []string) error {
	var unusable []string
	for _, name := range names {
		if !providerName.MatchString(name) {
			unusable = append(unusable, fmt.Sprintf("%q", name))
		}
	}
	switch len(unusable) {
	case 0:
	case 1:
		return fmt.Errorf("%s: entry %s is not a usable provider name — %s", path, unusable[0], ProviderNameRule)
	default:
		return fmt.Errorf("%s: entries %s are not usable provider names — %s", path, strings.Join(unusable, ", "), ProviderNameRule)
	}
	var pairs []string
	for _, c := range caseCollisions(map[string]statedName{}, path, names) {
		pairs = append(pairs, fmt.Sprintf("%q and %q", c.first.name, c.second.name))
	}
	if len(pairs) > 0 {
		return fmt.Errorf("%s: entries %s differ only by case, which no two entry names may; rename one",
			path, strings.Join(pairs, "; "))
	}
	return nil
}

// statedName is a providers.toml entry's name and the file stating it.
type statedName struct{ name, path string }

// caseCollision is two entries whose names differ only by case, first the one
// seen first.
type caseCollision struct{ first, second statedName }

// caseCollisions records each of names, stated in path, in seen under its
// lowercase, and returns each name whose lowercase a differently spelled name
// already holds there. A name spelled exactly as the one seen is no collision.
func caseCollisions(seen map[string]statedName, path string, names []string) []caseCollision {
	var found []caseCollision
	for _, name := range names {
		folded := strings.ToLower(name)
		first, ok := seen[folded]
		switch {
		case !ok:
			seen[folded] = statedName{name: name, path: path}
		case first.name != name:
			found = append(found, caseCollision{first: first, second: statedName{name: name, path: path}})
		}
	}
	return found
}

// resolveAPIKey returns the provider's API key, read from the file
// APIKeyFile names; an entry that names no file resolves to no key. The
// key file's content is trimmed of surrounding whitespace. The returned
// error never carries key content — only the file path, on an I/O failure.
func resolveAPIKey(p Provider, dir string) (secret.Key, error) {
	if p.APIKeyFile == "" {
		return secret.Key{}, nil
	}
	b, err := os.ReadFile(keyFilePath(p, dir))
	if err != nil {
		return secret.Key{}, fmt.Errorf("read the key file providers.toml names: %w", err)
	}
	return secret.NewKey(strings.TrimSpace(string(b))), nil
}

// keyFilePath resolves the provider's apiKeyFile: a leading ~ expands from the
// home directory first, so an absolute path after that expansion is never
// joined to the providers.toml directory; anything else resolves against it.
func keyFilePath(p Provider, dir string) string {
	path := expandHome(p.APIKeyFile)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
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
	return fmt.Sprintf("key file %s, named in providers.toml, is readable beyond its owner (mode %#o) — chmod 600 it", path, mode)
}
