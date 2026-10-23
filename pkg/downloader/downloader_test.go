package downloader_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dogu-dev/dogu/pkg/downloader"
)

func TestDownloadAndExtract_ProgressNotification(t *testing.T) {
	// Create a dummy HTTP server serving a small payload with Content-Length
	payload := []byte("hello world progress test payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "cache", "test.txt")
	targetDir := filepath.Join(tempDir, "target")

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	err = downloader.SaveSingleBinary(context.Background(), server.URL, cacheFile, targetDir, "test.txt")

	w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, r)

	if err != nil {
		t.Fatalf("SaveSingleBinary failed: %v", err)
	}

	stderrStr := stderrBuf.String()
	if !strings.Contains(stderrStr, "Downloading") && !strings.Contains(stderrStr, "Downloaded") {
		t.Errorf("expected progress notification in stderr, got: %q", stderrStr)
	}
}

func TestDownloadAndExtract_RedirectProgressNotification(t *testing.T) {
	payload := []byte("hello world redirect progress test payload")

	var targetServer *httptest.Server
	targetServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer targetServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetServer.URL+"/redirected-file.txt", http.StatusFound)
	}))
	defer redirectServer.Close()

	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "cache", "test_redirect.txt")
	targetDir := filepath.Join(tempDir, "target")

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	err = downloader.SaveSingleBinary(context.Background(), redirectServer.URL, cacheFile, targetDir, "test_redirect.txt")

	w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, r)

	if err != nil {
		t.Fatalf("SaveSingleBinary with redirect failed: %v", err)
	}

	stderrStr := stderrBuf.String()
	if !strings.Contains(stderrStr, redirectServer.URL) {
		t.Errorf("expected stderr to contain original URL %q, got: %q", redirectServer.URL, stderrStr)
	}
	if !strings.Contains(stderrStr, targetServer.URL) {
		t.Errorf("expected stderr to contain redirected target URL %q, got: %q", targetServer.URL, stderrStr)
	}
}

type testRewriter struct {
	from string
	to   string
}

func (tr testRewriter) RewriteURL(u string) string {
	if strings.HasPrefix(u, tr.from) {
		return tr.to + strings.TrimPrefix(u, tr.from)
	}
	return u
}

func TestDownload_URLRewriteOnRedirect(t *testing.T) {
	payload := []byte("redirect rewrite test payload")

	// Internal/artifactory server
	internalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/download/jdk.tar.gz" {
			http.Error(w, "not found on internal server", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer internalServer.Close()

	// Initial server that redirects to upstream public URL
	upstreamPublicPrefix := "https://github.com/adoptium/temurin25-binaries/releases/download/"
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, upstreamPublicPrefix+"jdk.tar.gz", http.StatusFound)
	}))
	defer redirectServer.Close()

	// Configure rewrite rule: upstreamPublicPrefix -> internalServer.URL + "/internal/download/"
	rewriter := testRewriter{
		from: upstreamPublicPrefix,
		to:   internalServer.URL + "/internal/download/",
	}

	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "cache", "jdk.tar.gz")
	targetDir := filepath.Join(tempDir, "target")

	err := downloader.SaveSingleBinary(context.Background(), redirectServer.URL, cacheFile, targetDir, "jdk.tar.gz", downloader.WithRewriteRule(rewriter))
	if err != nil {
		t.Fatalf("SaveSingleBinary with rewrite rule failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(targetDir, "jdk.tar.gz"))
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != string(payload) {
		t.Errorf("expected content %q, got %q", string(payload), string(content))
	}
}
