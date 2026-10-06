package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// The environment a subcommand that spends inference reads: kb_tools'
// variables under kbase's prefix, with kb_tools' meanings. KBASE_MODEL names
// both tiers.
const (
	EnvAPIBaseURL = "KBASE_API_BASE_URL"
	EnvModel      = "KBASE_MODEL"
	EnvAPIKeyFile = "KBASE_API_KEY_FILE"
)

// environmentEndpoint names an endpoint no providers.toml entry supplied.
const environmentEndpoint = "environment"

// Overrides is one layer of inference settings above the configuration files;
// a field left "" leaves it to the layer below.
type Overrides struct {
	BaseURL, Model, APIKeyFile string
}

// EnvOverrides is the environment's layer, read through getenv.
func EnvOverrides(getenv func(string) string) Overrides {
	return Overrides{
		BaseURL:    strings.TrimSpace(getenv(EnvAPIBaseURL)),
		Model:      strings.TrimSpace(getenv(EnvModel)),
		APIKeyFile: strings.TrimSpace(getenv(EnvAPIKeyFile)),
	}
}

// Inference is where a subcommand's model calls go and which model serves
// each tier.
type Inference struct {
	Name, BaseURL, APIKey string
	Heavy, Light          string
}

// LackError is a setting the inference configuration lacks, named by the
// variable that would supply it and the file that otherwise does.
type LackError struct {
	Key, Path, Detail string
}

func (e LackError) Error() string { return e.Detail }

// Named is the variable and the file the lacking setting is read from.
func (e LackError) Named() (key, path string) { return e.Key, e.Path }

// ResolveInference resolves each setting from the first of layers that gives
// it — highest precedence first — and otherwise from the configuration files:
// entry, the providers.toml entry they select (nil where they select none,
// unselected saying why), and cfg's tiers. A layer's field overrides that
// field and nothing else.
func ResolveInference(cfg Config, entry *Provider, unselected error, layers ...Overrides) (Inference, error) {
	pick := func(field func(Overrides) string) string {
		for _, l := range layers {
			if v := field(l); v != "" {
				return v
			}
		}
		return ""
	}
	var inf Inference
	if entry != nil {
		inf.Name, inf.BaseURL, inf.APIKey = entry.Name, entry.BaseURL, entry.APIKey
	}
	if v := pick(func(o Overrides) string { return o.BaseURL }); v != "" {
		inf.BaseURL = v
		if entry == nil {
			inf.Name = environmentEndpoint
		}
	}
	if inf.BaseURL == "" {
		why := "it selects no provider"
		if unselected != nil {
			why = unselected.Error()
		}
		return inf, LackError{Key: EnvAPIBaseURL, Path: ProvidersFileName,
			Detail: fmt.Sprintf("%s is unset and %s supplies no base URL: %s", EnvAPIBaseURL, ProvidersFileName, why)}
	}
	if path := pick(func(o Overrides) string { return o.APIKeyFile }); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return inf, LackError{Key: EnvAPIKeyFile, Path: path, Detail: fmt.Sprintf("%s names %s, which cannot be read: %v", EnvAPIKeyFile, path, err)}
		}
		inf.APIKey = strings.TrimSpace(string(b))
	}
	inf.Heavy, inf.Light = cfg.Models.Heavy, cfg.Models.Light
	if m := pick(func(o Overrides) string { return o.Model }); m != "" {
		inf.Heavy, inf.Light = m, m
	}
	var unset []string
	for _, t := range []struct{ tier, id string }{{TierLight, inf.Light}, {TierHeavy, inf.Heavy}} {
		if t.id == "" {
			unset = append(unset, "models."+t.tier)
		}
	}
	if len(unset) > 0 {
		return inf, LackError{Key: EnvModel, Path: ConfigFileName,
			Detail: fmt.Sprintf("%s is unset and %s sets no %s", EnvModel, ConfigFileName, strings.Join(unset, " or "))}
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
