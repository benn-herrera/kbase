package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"kbase/internal/secret"
)

// CleartextWarning is the one line saying a configured key travels in
// cleartext — the base URL is http:// to a host off the local network — and ""
// otherwise. A local-network host is localhost, a *.local name, or a loopback,
// private or link-local IP literal. It names the host, never the key.
func CleartextWarning(baseURL string, apiKey secret.Key) string {
	if apiKey.IsZero() {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil || !strings.EqualFold(u.Scheme, "http") {
		return ""
	}
	host := u.Hostname()
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if h == "localhost" || strings.HasSuffix(h, ".local") {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return ""
	}
	return fmt.Sprintf("warning: the API key travels in cleartext to %s (http://, not a local-network host)", host)
}
