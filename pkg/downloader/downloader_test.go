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
