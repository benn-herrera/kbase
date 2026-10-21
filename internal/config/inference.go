package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// The environment a subcommand that spends inference reads: the provider's
// base URL, the model each role asks, and the file the API key is read from.
// Each variable names one setting and nothing else — no variable spans two
// roles, and there is no KBASE_*_MODEL.
const (
	EnvAPIBaseURL = "KBASE_API_BASE_URL"
	EnvDecision   = "KBASE_DECISION"
	EnvText       = "KBASE_TEXT"
	EnvScoring    = "KBASE_SCORING"
	EnvAPIKeyFile = "KBASE_API_KEY_FILE"
)

// Overrides is the environment's inference settings, each already trimmed; a
// field left "" is a variable that is unset or empty. Each field names one
// setting.
type Overrides struct {
	BaseURL, Decision, Text, Scoring, APIKeyFile string
}

// EnvOverrides is the environment, read through getenv.
func EnvOverrides(getenv func(string) string) Overrides {
	return Overrides{
		BaseURL:    strings.TrimSpace(getenv(EnvAPIBaseURL)),
		Decision:   strings.TrimSpace(getenv(EnvDecision)),
		Text:       strings.TrimSpace(getenv(EnvText)),
		Scoring:    strings.TrimSpace(getenv(EnvScoring)),
		APIKeyFile: strings.TrimSpace(getenv(EnvAPIKeyFile)),
	}
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
