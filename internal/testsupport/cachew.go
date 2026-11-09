// Package testsupport contains helpers for integration tests, not application configuration.
package testsupport

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// CachewTransport routes every HTTP request, including discovery and redirects,
// through Cachew's host strategy. There is no direct-upstream fallback.
func CachewTransport(rawURL string) (http.RoundTripper, error) {
	base, err := url.Parse(rawURL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("invalid CACHEW_URL %q", rawURL)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &cachewTransport{base: base, transport: transport, githubToken: os.Getenv("CACHEW_GITHUB_TOKEN")}, nil
}

type cachewTransport struct {
	base        *url.URL
	transport   *http.Transport
	githubToken string
}

func (t *cachewTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "http" && r.URL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported upstream scheme: %s", r.URL.Scheme)
	}
	local := r.Clone(r.Context())
	// CI's scoped token raises the public API quota for the complete example
	// suite. Never forward it to asset hosts or arbitrary mirrors.
	if r.URL.Host == "api.github.com" && t.githubToken != "" && t.cachewTokenForwardAllowed() {
		local.Header.Set("Authorization", "Bearer "+t.githubToken)
	}
	if r.URL.Host != t.base.Host || r.URL.Scheme != t.base.Scheme {
		// Preserve escaped asset paths (for example Java versions containing '+').
		local.URL.Path = strings.TrimRight(t.base.Path, "/") + "/" + r.URL.Host + r.URL.Path
		local.URL.RawPath = strings.TrimRight(t.base.EscapedPath(), "/") + "/" + r.URL.Host + r.URL.EscapedPath()
		local.URL.Scheme = t.base.Scheme
		local.URL.Host = t.base.Host
		local.URL.User = nil
		local.Host = ""
	}
	return t.transport.RoundTrip(local)
}

// Only the local test mirror is trusted to receive the GitHub API token.
func (t *cachewTransport) cachewTokenForwardAllowed() bool {
	host := t.base.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
