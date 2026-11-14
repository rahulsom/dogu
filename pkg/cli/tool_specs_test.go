package cli

import (
	"testing"

	"github.com/dogu-dev/dogu/pkg/tools"
)

func TestToolVersionFormats(t *testing.T) {
	for _, name := range tools.List() {
		for _, tc := range []struct {
			args    []string
			version string
		}{
			{[]string{"--" + name, "25"}, "25"},
			{[]string{"--" + name + "=25"}, "25"},
			{[]string{"--" + name + "="}, ""},
		} {
			specs, _, _, _, _, err := parseToolSpecs(tc.args)
			if err != nil || len(specs) != 1 || specs[0].Name != name || specs[0].Version != tc.version {
				t.Fatalf("%v: specs=%v, err=%v", tc.args, specs, err)
			}
		}
		if _, _, _, _, _, err := parseToolSpecs([]string{"--" + name}); err == nil {
			t.Fatalf("bare --%s must still require a value", name)
		}
	}
	specs, force, _, _, _, err := parseToolSpecs([]string{"--java=", "--node=22", "--force"})
	if err != nil || len(specs) != 2 || specs[0].Version != "" || specs[1].Version != "22" || !force {
		t.Fatalf("empty value consumed the next flag: %v, %v", specs, err)
	}
	specs, _, offline, _, _, err := parseToolSpecs([]string{"--offline", "--node=22"})
	if err != nil || len(specs) != 1 || specs[0].Name != "node" || !offline {
		t.Fatalf("offline flag was not parsed: specs=%v, offline=%v, err=%v", specs, offline, err)
	}
}
