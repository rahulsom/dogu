package downloader

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarHardLinks(t *testing.T) {
	for _, tc := range []struct {
		name, link string
		valid      bool
	}{
		{"source archive", "source/redhat/spec.in", true},
		{"traversal", "../../outside", false},
		{"missing source", "source/missing", false},
		{"symlinked source", "source/escape/outside", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := t.TempDir()
			outside := t.TempDir()
			if err := os.WriteFile(filepath.Join(outside, "outside"), []byte("private"), 0600); err != nil {
				t.Fatal(err)
			}
			var archive bytes.Buffer
			tw := tar.NewWriter(&archive)
			for _, h := range []*tar.Header{
				{Name: "source/redhat/spec.in", Typeflag: tar.TypeReg, Mode: 0644, Size: 4},
				{Name: "source/escape", Typeflag: tar.TypeSymlink, Linkname: outside},
				{Name: "source/redhat/alias.in", Typeflag: tar.TypeLink, Linkname: tc.link},
			} {
				if err := tw.WriteHeader(h); err != nil {
					t.Fatal(err)
				}
				if h.Typeflag == tar.TypeReg {
					if _, err := tw.Write([]byte("spec")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			err := extractTar(&archive, dest)
			if !tc.valid {
				if err == nil {
					t.Fatal("accepted invalid hard link")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			source, err := os.Stat(filepath.Join(dest, "source/redhat/spec.in"))
			if err != nil {
				t.Fatal(err)
			}
			alias, err := os.Stat(filepath.Join(dest, "source/redhat/alias.in"))
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(source, alias) {
				t.Fatal("archive hard link was not preserved")
			}
			data, err := os.ReadFile(filepath.Join(dest, "source/redhat/alias.in"))
			if err != nil || string(data) != "spec" {
				t.Fatalf("hard link content %q: %v", data, err)
			}
		})
	}
}
