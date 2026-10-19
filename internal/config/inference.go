package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// The environment a subcommand that spends inference reads: the provider's
// base URL, the model each category asks, and the file the API key is read
// from. Each variable names one setting and nothing else — no variable spans
// both categories.
const (
	EnvAPIBaseURL = "KBASE_API_BASE_URL"
	EnvDecision   = "KBASE_DECISION"
	EnvText       = "KBASE_TEXT"
	EnvAPIKeyFile = "KBASE_API_KEY_FILE"
)

// environmentEndpoint names an endpoint no providers.toml entry supplied.
const environmentEndpoint = "environment"

// Overrides is one layer of inference settings above the configuration files;
// a field left "" leaves it to the layer below. Each field names one setting.
type Overrides struct {
	BaseURL, Decision, Text, APIKeyFile string
}

// EnvOverrides is the environment's layer, read through getenv.
func EnvOverrides(getenv func(string) string) Overrides {
	return Overrides{
		BaseURL:    strings.TrimSpace(getenv(EnvAPIBaseURL)),
		Decision:   strings.TrimSpace(getenv(EnvDecision)),
		Text:       strings.TrimSpace(getenv(EnvText)),
		APIKeyFile: strings.TrimSpace(getenv(EnvAPIKeyFile)),
	}
}

// Inference is where a subcommand's model calls go, which model serves each
// category, and the layer that supplied each field it resolved.
// DecisionsFlavor is the decision surface the selected entry speaks, and
// FlavorSystemOne where none was selected: the environment names a base URL,
// not a surface, so there is nothing else it could be.
type Inference struct {
	Name, BaseURL, APIKey string
	Decision, Text        string
	DecisionsFlavor       string
	Sources               Sources
}

// Sources names the layer that supplied each resolved field: the environment,
// or the configuration layer — explicit, environment, repository or global —
// the pool entry or the config.toml value came from. A field no layer supplied
// — a key an entry does not state, a category an unresolved inference never
// reached — carries "". The environment is the only layer a subcommand merges
// above the files, so a field it supplies reports its own name; the files'
// layers carry theirs from the load.
type Sources struct {
	BaseURL, Decision, Text, Key string
}

// LackError is a setting the inference configuration lacks, named by the
// variable that would supply it — or, for a setting the configuration files
// carry, the key they spell it with — and the file it is read from or belongs
// in.
type LackError struct {
	Key, Path, Detail string
}

func (e LackError) Error() string { return e.Detail }

// Named is the variable and the file the lacking setting is read from.
func (e LackError) Named() (key, path string) { return e.Key, e.Path }

// ResolveInference resolves each setting from the first of layers that gives
// it — highest precedence first — and otherwise from the configuration files:
// entry, the providers.toml entry they select (nil where they select none,
// unselected saying why), and cfg's categories. A layer's field overrides that
// field and nothing else. Each resolved field carries the layer it came from.
//
// writeDir is the configuration directory a lack is reported against: the
// providers.toml or config.toml the setting belongs in is named by its own
// path, since a repository layer and a global one can each hold either file and
// a bare name would not say which. An empty writeDir leaves the bare name.
func ResolveInference(cfg Config, entry *Provider, unselected error, writeDir string, layers ...Overrides) (Inference, error) {
	pick := func(field func(Overrides) string) string {
		for _, l := range layers {
			if v := field(l); v != "" {
				return v
			}
		}
		return ""
	}
	// An endpoint no pool entry supplied has no flavour to state — the
	// environment names a base URL, not a decision surface — so the SystemOne
	// shape is what resolves; a selected entry's own flavour overrides it.
	inf := Inference{DecisionsFlavor: FlavorSystemOne}
	if entry != nil {
		inf.Name, inf.BaseURL, inf.APIKey = entry.Name, entry.BaseURL, entry.APIKey
		inf.DecisionsFlavor = entry.Flavor()
		if entry.BaseURL != "" {
			inf.Sources.BaseURL = entry.Layer
		}
		if entry.APIKey != "" {
			inf.Sources.Key = entry.Layer
		}
	}
	if v := pick(func(o Overrides) string { return o.BaseURL }); v != "" {
		inf.BaseURL, inf.Sources.BaseURL = v, LayerSourceEnvironment
		if entry == nil {
			inf.Name = environmentEndpoint
		}
	}
	if inf.BaseURL == "" {
		why := "it selects no provider"
		if unselected != nil {
			why = unselected.Error()
		}
		path := ProvidersPath(writeDir)
		return inf, LackError{Key: "baseUrl", Path: path,
			Detail: fmt.Sprintf("%s is unset and %s supplies no base URL: %s", EnvAPIBaseURL, path, why)}
	}
	if path := expandHome(pick(func(o Overrides) string { return o.APIKeyFile })); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return inf, LackError{Key: EnvAPIKeyFile, Path: path, Detail: fmt.Sprintf("%s names %s, which cannot be read: %v", EnvAPIKeyFile, path, err)}
		}
		inf.APIKey, inf.Sources.Key = strings.TrimSpace(string(b)), LayerSourceEnvironment
	}
	// A category's id comes from the first of: its specific variable
	// (KBASE_DECISION / KBASE_TEXT) and the configuration, whose merge
	// recorded which layer supplied the id it holds.
	categoryID := func(specific func(Overrides) string, id, source string) (string, string) {
		if v := pick(specific); v != "" {
			return v, LayerSourceEnvironment
		}
		return id, source
	}
	inf.Decision, inf.Sources.Decision = categoryID(func(o Overrides) string { return o.Decision }, cfg.Models.Decision, cfg.Sources.Decision)
	inf.Text, inf.Sources.Text = categoryID(func(o Overrides) string { return o.Text }, cfg.Models.Text, cfg.Sources.Text)
	var unset []string
	for _, c := range []struct{ category, id string }{{CategoryDecision, inf.Decision}, {CategoryText, inf.Text}} {
		if c.id == "" {
			unset = append(unset, "models."+c.category)
		}
	}
	if len(unset) > 0 {
		path := ConfigPath(writeDir)
		return inf, LackError{Key: unset[0], Path: path,
			Detail: fmt.Sprintf("%s sets no %s", path, strings.Join(unset, " or "))}
	}
	return inf, nil
}

// CleartextWarning is the one line saying a configured key travels in
// cleartext — the base URL is http:// to a host other than loopback — and ""
// otherwise. It names the host, never the key.
func CleartextWarning(baseURL, apiKey string) string {
	if apiKey == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil || !strings.EqualFold(u.Scheme, "http") {
		return ""
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return ""
	}
	return fmt.Sprintf("warning: the API key travels in cleartext to %s (http://, not a loopback host)", host)
}
