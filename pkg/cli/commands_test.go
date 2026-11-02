package cli

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestAutocomplete(t *testing.T) {
	for _, shell := range []string{"bash", "/bin/zsh", "fish", "pwsh", "powershell.exe"} {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeAutocomplete(&out, shell); err != nil {
				t.Fatal(err)
			}
			for _, word := range []string{"list-versions", "autocomplete", "--node", "java"} {
				if !strings.Contains(out.String(), word) {
					t.Errorf("missing %s", word)
				}
			}
			if shell == "bash" || shell == "/bin/zsh" {
				cmd := exec.Command(shell, "-n")
				cmd.Stdin = strings.NewReader(out.String())
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("invalid shell script: %v: %s", err, output)
				}
			}
		})
	}
	if err := writeAutocomplete(&bytes.Buffer{}, "unknown"); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}

func TestInfoCommandValidation(t *testing.T) {
	for _, args := range [][]string{{"version", "extra"}, {"list-versions"}, {"list-versions", "unknown"}, {"autocomplete", "bash", "extra"}} {
		handled, err := runInfoCommand(args, nil)
		if !handled || err == nil {
			t.Errorf("%v: handled=%v err=%v", args, handled, err)
		}
	}
	t.Setenv("SHELL", "/bin/bash")
	if handled, err := runInfoCommand([]string{"--help"}, nil); handled || err != nil {
		t.Fatalf("unexpected handling: %v %v", handled, err)
	}
}

func TestVersionListOptions(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		limit  int
		prefix string
	}{
		{[]string{"java"}, 20, ""},
		{[]string{"java", "21", "--all"}, 0, "21"},
		{[]string{"--limit", "3", "node", "22"}, 3, "22"},
	} {
		opts, err := parseVersionListArgs(tc.args)
		if err != nil {
			t.Fatal(err)
		}
		if opts.limit != tc.limit || opts.prefix != tc.prefix {
			t.Fatalf("%v: %+v", tc.args, opts)
		}
	}
	for _, args := range [][]string{{}, {"java", "--limit"}, {"java", "--limit", "0"}, {"java", "--limit", "x"}, {"java", "--all", "--limit", "2"}, {"java", "--unknown"}, {"java", "21", "extra"}} {
		if _, err := parseVersionListArgs(args); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

func TestVersionListOutput(t *testing.T) {
	versions := []string{"v22.1.0", "v21.2.0", "v21.1.0", "v20.0.0"}
	for _, tc := range []struct {
		opts   versionListOptions
		want   string
		notice bool
	}{
		{versionListOptions{tool: "node", limit: 2}, "v22.1.0\nv21.2.0\n", true},
		{versionListOptions{tool: "node", prefix: "21", limit: 20}, "v21.2.0\nv21.1.0\n", false},
		{versionListOptions{tool: "node"}, strings.Join(versions, "\n") + "\n", false},
	} {
		var out, notice bytes.Buffer
		if err := writeVersionList(&out, &notice, versions, tc.opts); err != nil {
			t.Fatal(err)
		}
		if out.String() != tc.want {
			t.Errorf("output %q want %q", out.String(), tc.want)
		}
		if (notice.Len() > 0) != tc.notice {
			t.Errorf("unexpected notice %q", notice.String())
		}
	}
	if err := writeVersionList(&bytes.Buffer{}, &bytes.Buffer{}, versions, versionListOptions{tool: "node", prefix: "99"}); err == nil {
		t.Fatal("expected no match error")
	}
}
