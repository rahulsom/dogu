package testsupport

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestCachewTransportDiscoveryAndRedirect(t *testing.T) {
	var paths []string
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		wantAuth := ""
		if r.URL.Path == "/api.github.com/repos/example/tool/releases" {
			wantAuth = "Bearer test-token"
		}
		if got := r.Header.Get("Authorization"); got != wantAuth {
			t.Errorf("authorization for %s: %q, want %q", r.URL.Path, got, wantAuth)
		}
		if r.URL.Path == "/api.github.com/repos/example/tool/releases" {
			http.Redirect(w, r, "https://release-assets.githubusercontent.com/jdk%2B17.tar.gz?token=abc", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "cached archive")
	}))
	defer mirror.Close()
	// Neither upstream host is contacted, even when a redirect crosses hosts.
	t.Setenv("CACHEW_GITHUB_TOKEN", "test-token")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	transport, err := CachewTransport(mirror.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: transport}
	resp, err := client.Get("https://api.github.com/repos/example/tool/releases?page=1")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "cached archive" {
		t.Fatalf("body = %q, paths = %v, status = %s", body, paths, resp.Status)
	}
	resp, err = client.Get(mirror.URL + "/nodejs.org/dist/index.tab")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	want := []string{"/api.github.com/repos/example/tool/releases?page=1", "/release-assets.githubusercontent.com/jdk%2B17.tar.gz?token=abc", "/nodejs.org/dist/index.tab"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("requests = %v, want %v", paths, want)
	}
}

func TestCachewTransportRequiresURL(t *testing.T) {
	for _, raw := range []string{"", "localhost:8080", "file:///tmp/cachew", "http://user:pass@localhost", "http://localhost?query=1"} {
		if _, err := CachewTransport(raw); err == nil {
			t.Errorf("accepted invalid CACHEW_URL %q", raw)
		}
	}
}

func TestCachewTokenTrust(t *testing.T) {
	for _, tc := range []struct {
		host    string
		allowed bool
	}{
		{"localhost:8080", true}, {"LOCALHOST", true}, {"127.0.0.1:8080", true},
		{"127.0.0.2", true}, {"[::1]:8080", true}, {"[::ffff:127.0.0.1]", true},
		{"mirror.example.test", false}, {"localhost.evil.test", false}, {"192.168.1.2", false},
	} {
		t.Run(tc.host, func(t *testing.T) {
			base, err := url.Parse("http://" + tc.host)
			if err != nil {
				t.Fatal(err)
			}
			transport := &cachewTransport{base: base}
			if got := transport.cachewTokenForwardAllowed(); got != tc.allowed {
				t.Errorf("allowed = %v, want %v", got, tc.allowed)
			}
		})
	}
}

func TestCachewRemoteMirrorDoesNotReceiveToken(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("remote mirror received authorization: %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mirror.Close()
	t.Setenv("CACHEW_GITHUB_TOKEN", "test-token")
	transport, err := CachewTransport("http://mirror.example.test")
	if err != nil {
		t.Fatal(err)
	}
	transport.(*cachewTransport).transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, mirror.Listener.Addr().String())
	}
	client := &http.Client{Transport: transport}
	resp, err := client.Get("https://api.github.com/repos/example/tool/releases")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
