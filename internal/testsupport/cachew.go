// Package testsupport contains helpers for integration tests, not application configuration.
package testsupport

import (
	"fmt"
	"net/http"
	"net/url"
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
	return &cachewTransport{base: base, transport: transport}, nil
}

type cachewTransport struct {
	base      *url.URL
	transport *http.Transport
}

func (t *cachewTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "http" && r.URL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported upstream scheme: %s", r.URL.Scheme)
	}
	local := r.Clone(r.Context())
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
