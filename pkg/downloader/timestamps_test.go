package downloader_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dogu-dev/dogu/pkg/downloader"
)

func TestExtractTarGzPreservesSourceTimestamps(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "source.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	generated := time.Unix(1700000100, 0)
	input := generated.Add(-time.Minute)
	// Archive order must not make generated files older than their inputs.
	entries := []struct {
		name    string
		modTime time.Time
	}{
		{"source/configure", generated},
		{"source/configure.ac", input},
	}
	for _, entry := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0644, Size: 1, ModTime: entry.modTime}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := downloader.ExtractTarGz(archive, dest); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		info, err := os.Stat(filepath.Join(dest, entry.name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(entry.modTime) {
			t.Errorf("%s modification time = %s, want %s", entry.name, info.ModTime(), entry.modTime)
		}
	}
}
