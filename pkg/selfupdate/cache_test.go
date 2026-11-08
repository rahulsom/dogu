package selfupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWeeklyUpdateChecks(t *testing.T) {
	for _, scenario := range []string{"available", "no update", "no assets", "wrong platform", "missing URL", "failed request", "expired", "corrupt"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("DOGU_HOME", home)
			calls := 0
			rel := Release{TagName: "v1.5.0", Assets: []ReleaseAsset{{Name: "dogu-" + runtime.GOOS + "-" + runtime.GOARCH + armAssetSuffix(), DownloadURL: "https://example.com/dogu"}}}
			switch scenario {
			case "no update":
				rel.TagName = "v1.0.0"
			case "no assets":
				rel.Assets = nil
			case "wrong platform":
				rel.Assets[0].Name = "dogu-unknown-unknown"
			case "missing URL":
				rel.Assets[0].DownloadURL = ""
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if scenario == "failed request" {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(rel)
			}))
			defer server.Close()
			old := GitHubBaseURL
			GitHubBaseURL = server.URL
			defer func() { GitHubBaseURL = old }()
			path := filepath.Join(home, "update-check.json")
			if scenario == "expired" {
				if err := writeUpdateCache(path, updateCheckCache{LastChecked: time.Now().Add(-updateCheckInterval), Source: server.URL + "/repos/owner/repo"}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "corrupt" {
				if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			wantAvailable := scenario == "available" || scenario == "expired" || scenario == "corrupt"
			for i := 0; i < 2; i++ {
				_, available, err := CheckForUpdateCached(context.Background(), "v1.0.0", "owner", "repo")
				if (err != nil) != (scenario == "failed request" && i == 0) {
					t.Fatalf("check %d: unexpected error %v", i, err)
				}
				if available != wantAvailable {
					t.Fatalf("available = %v, want %v", available, wantAvailable)
				}
			}
			if calls != 1 {
				t.Fatalf("made %d requests, want 1", calls)
			}
			var cached updateCheckCache
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &cached); err != nil {
				t.Fatal(err)
			}
			if cached.LastChecked.IsZero() {
				t.Fatal("missing check timestamp")
			}
			// An upgrade must suppress the cached notice without making another request.
			_, available, err := CheckForUpdateCached(context.Background(), "v1.5.0", "owner", "repo")
			if err != nil || available || calls != 1 {
				t.Fatalf("after upgrade: available=%v calls=%d err=%v", available, calls, err)
			}
			// An explicit check bypasses the weekly cache, including while assets build.
			_, _, _ = CheckForUpdate(context.Background(), "v1.0.0", "owner", "repo")
			if calls != 2 {
				t.Fatalf("explicit check did not bypass cache: %d calls", calls)
			}
		})
	}
}
