package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExpandArgsFile(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child", "nested")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(path, contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "dogu.args"), "# root\n--java 21\n--node=22\n")
	write(filepath.Join(root, "child", "dogu.args"), "  # nearest\n--java 17\n")
	write(filepath.Join(child, "other.args"), "--node 20\n--offline\n")
	t.Chdir(child)

	tests := []struct {
		name string
		args []string
		want []string
		err  string
	}{
		{"nearest ancestor", []string{"--verbose"}, []string{"--java", "17", "--verbose"}, ""},
		{"explicit relative path", []string{"-v", "@other.args", "--java=21"}, []string{"-v", "--node", "20", "--offline", "--java=21"}, ""},
		{"explicit absolute path", []string{"@" + filepath.Join(root, "dogu.args")}, []string{"--java", "21", "--node=22"}, ""},
		{"missing explicit file", []string{"@missing.args"}, nil, "missing.args"},
		{"empty path", []string{"@"}, nil, "path after @ is empty"},
		{"multiple files", []string{"@other.args", "@other.args"}, nil, "only one"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandArgsFile(tt.args)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("got error %v, want %q", err, tt.err)
				}
			} else if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, %v; want %v", got, err, tt.want)
			}
		})
	}

	write(filepath.Join(child, "dogu.args"), "--node 18")
	got, err := expandArgsFile(nil)
	if err != nil || !reflect.DeepEqual(got, []string{"--node", "18"}) {
		t.Fatalf("current directory: got %v, %v", got, err)
	}

	for _, contents := range []string{"--java", "node 22", "--java 21 -- echo hi", "@other.args", "--help"} {
		write(filepath.Join(child, "dogu.args"), contents)
		if _, err := expandArgsFile(nil); err == nil || !strings.Contains(err.Error(), "invalid args file") {
			t.Errorf("%q: expected invalid args file error, got %v", contents, err)
		}
	}
}

func TestArgsFileReadErrors(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if got, err := expandArgsFile([]string{"--offline"}); err != nil || !reflect.DeepEqual(got, []string{"--offline"}) {
		t.Fatalf("no file: got %v, %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(root, "dogu.args"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := expandArgsFile(nil); err == nil {
		t.Fatal("expected read error for directory named dogu.args")
	}
}
