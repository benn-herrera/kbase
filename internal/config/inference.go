package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"kbase/internal/secret"
)

// CleartextWarning is the one line saying a configured key travels in
// cleartext — the base URL is http:// to a host other than loopback — and ""
// otherwise. It names the host, never the key.
func CleartextWarning(baseURL string, apiKey secret.Key) string {
	if apiKey.IsZero() {
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
