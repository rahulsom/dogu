package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsUpdateAvailable(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"v1.0.0", "v1.1.0", true},
		{"v1.1.0", "v1.1.0", false},
		{"1.0.0", "1.2.0", true},
		{"1.2.0", "1.1.0", false},
		{"dev", "v1.0.0", false},
		{"(devel)", "v1.0.0", false},
		{"", "v1.0.0", false},
	}

	for _, tt := range tests {
		got := isUpdateAvailable(tt.current, tt.latest)
		if got != tt.expected {
			t.Errorf("isUpdateAvailable(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.expected)
		}
	}
}

func TestFindMatchingAsset(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "dogu_0.3.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux_amd64.tar.gz"},
		{Name: "dogu_0.3.0_linux_arm64.tar.gz", DownloadURL: "https://example.com/linux_arm64.tar.gz"},
		{Name: "dogu_0.3.0_linux_armv7.tar.gz", DownloadURL: "https://example.com/linux_armv7.tar.gz"},
		{Name: "dogu_0.3.0_darwin_amd64.tar.gz", DownloadURL: "https://example.com/darwin_amd64.tar.gz"},
		{Name: "dogu_0.3.0_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin_arm64.tar.gz"},
		{Name: "dogu_0.3.0_windows_amd64.zip", DownloadURL: "https://example.com/windows_amd64.zip"},
		{Name: "dogu-linux-amd64", DownloadURL: "https://example.com/dogu-linux-amd64"},
	}

	a, err := FindMatchingAsset(assets, "linux", "amd64", "")
	if err != nil || a.Name != "dogu_0.3.0_linux_amd64.tar.gz" {
		t.Fatalf("unexpected asset for linux/amd64: %v, err: %v", a, err)
	}

	aArm, err := FindMatchingAsset(assets, "linux", "arm", "7")
	if err != nil || aArm.Name != "dogu_0.3.0_linux_armv7.tar.gz" {
		t.Fatalf("unexpected asset for linux/arm/7: %v, err: %v", aArm, err)
	}

	aMac, err := FindMatchingAsset(assets, "darwin", "arm64", "")
	if err != nil || aMac.Name != "dogu_0.3.0_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected asset for darwin/arm64: %v, err: %v", aMac, err)
	}

	aWin, err := FindMatchingAsset(assets, "windows", "amd64", "")
	if err != nil || aWin.Name != "dogu_0.3.0_windows_amd64.zip" {
		t.Fatalf("unexpected asset for windows/amd64: %v, err: %v", aWin, err)
	}
}

func TestCheckForUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			rel := Release{
				TagName: "v1.5.0",
				Name:    "v1.5.0",
				Assets: []ReleaseAsset{
					{Name: "dogu_1.5.0_linux_amd64.tar.gz", DownloadURL: "http://example.com/dl"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rel)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	oldBase := GitHubBaseURL
	GitHubBaseURL = server.URL
	defer func() { GitHubBaseURL = oldBase }()

	ctx := context.Background()
	rel, available, err := CheckForUpdate(ctx, "v1.0.0", "owner", "repo")
	if err != nil {
		t.Fatalf("CheckForUpdate failed: %v", err)
	}
	if !available {
		t.Errorf("expected update to be available for v1.0.0 vs %s", rel.TagName)
	}
	if rel.TagName != "v1.5.0" {
		t.Errorf("expected tag v1.5.0, got %s", rel.TagName)
	}

	_, available, err = CheckForUpdate(ctx, "v1.5.0", "owner", "repo")
	if err != nil {
		t.Fatalf("CheckForUpdate failed: %v", err)
	}
	if available {
		t.Errorf("expected update not available for v1.5.0 vs v1.5.0")
	}
}

func createMockTarGz(binaryName, binaryContent string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte(binaryContent)
	hdr := &tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = gw.Close()

	return buf.Bytes()
}

func createMockZip(binaryName, binaryContent string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, _ := zw.Create(binaryName)
	_, _ = w.Write([]byte(binaryContent))
	_ = zw.Close()

	return buf.Bytes()
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	data := createMockTarGz("dogu", "mock-binary-v2")
	var out bytes.Buffer
	if err := extractBinaryFromTarGz(bytes.NewReader(data), &out); err != nil {
		t.Fatalf("failed to extract binary from tar.gz: %v", err)
	}
	if out.String() != "mock-binary-v2" {
		t.Errorf("extracted content = %q; want %q", out.String(), "mock-binary-v2")
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	data := createMockZip("dogu.exe", "mock-binary-zip")
	var out bytes.Buffer
	reader := bytes.NewReader(data)
	if err := extractBinaryFromZip(reader, int64(len(data)), &out); err != nil {
		t.Fatalf("failed to extract binary from zip: %v", err)
	}
	if out.String() != "mock-binary-zip" {
		t.Errorf("extracted content = %q; want %q", out.String(), "mock-binary-zip")
	}
}

func TestSuggestUpdateNotification(t *testing.T) {
	var buf bytes.Buffer
	SuggestUpdateNotification(&buf, "v0.1.0", "v0.2.0")
	output := buf.String()
	if !strings.Contains(output, "dogu self-update") {
		t.Errorf("expected output to mention 'dogu self-update', got: %s", output)
	}
	if !strings.Contains(output, "v0.1.0") || !strings.Contains(output, "v0.2.0") {
		t.Errorf("expected output to mention versions, got: %s", output)
	}
}

func TestReplaceExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	origExe := filepath.Join(tmpDir, "dogu")
	if runtime.GOOS == "windows" {
		origExe += ".exe"
	}
	if err := os.WriteFile(origExe, []byte("v1"), 0755); err != nil {
		t.Fatalf("failed to write orig binary: %v", err)
	}

	tempExe := filepath.Join(tmpDir, "dogu-new")
	if err := os.WriteFile(tempExe, []byte("v2"), 0755); err != nil {
		t.Fatalf("failed to write new binary: %v", err)
	}

	if err := replaceExecutable(tempExe, origExe); err != nil {
		t.Fatalf("replaceExecutable failed: %v", err)
	}

	content, err := os.ReadFile(origExe)
	if err != nil {
		t.Fatalf("failed to read replaced binary: %v", err)
	}
	if string(content) != "v2" {
		t.Errorf("expected content 'v2', got %q", string(content))
	}
}
