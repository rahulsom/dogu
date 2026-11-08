package selfupdate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dogu-dev/dogu/pkg/env"
)

const updateCheckInterval = 7 * 24 * time.Hour

type updateCheckCache struct {
	LastChecked time.Time `json:"last_checked"`
	Source      string    `json:"source"`
	Release     *Release  `json:"release,omitempty"`
}

// CheckForUpdateCached checks for automatic update notices at most once per week.
// Failed attempts are cached too. Explicit self-updates should use CheckForUpdate.
func CheckForUpdateCached(ctx context.Context, currentVersion, owner, repo string) (*Release, bool, error) {
	if owner == "" {
		owner = DefaultGitHubOwner
	}
	if repo == "" {
		repo = DefaultGitHubRepo
	}
	source := GitHubBaseURL + "/repos/" + owner + "/" + repo
	home := env.GetDoguHome()
	if err := os.MkdirAll(home, 0755); err != nil {
		return nil, false, err
	}
	// Serialize checks across CLI processes. Recover locks left by interrupted checks.
	lock := filepath.Join(home, "update-check.lock")
	if info, err := os.Stat(lock); err == nil && time.Since(info.ModTime()) > time.Minute {
		_ = os.Remove(lock)
	}
	if err := os.Mkdir(lock, 0700); err != nil {
		return nil, false, nil
	}
	defer os.Remove(lock)
	path := filepath.Join(home, "update-check.json")
	var cached updateCheckCache
	data, err := os.ReadFile(path)
	if err == nil && json.Unmarshal(data, &cached) == nil && cached.Source == source && !cached.LastChecked.IsZero() && time.Since(cached.LastChecked) < updateCheckInterval {
		return cached.Release, cachedUpdateAvailable(cached.Release, currentVersion), nil
	}
	// Persist the attempt before the request, so failures and interruptions also
	// respect the interval. If persistence fails, skip this automatic check.
	cached = updateCheckCache{LastChecked: time.Now().UTC(), Source: source}
	if err := writeUpdateCache(path, cached); err != nil {
		return nil, false, err
	}
	rel, available, err := CheckForUpdate(ctx, currentVersion, owner, repo)
	if err != nil {
		return nil, false, err
	}
	cached.Release = rel
	if err := writeUpdateCache(path, cached); err != nil {
		return nil, false, err
	}
	return rel, available, nil
}

func cachedUpdateAvailable(rel *Release, currentVersion string) bool {
	if rel == nil {
		return false
	}
	latest := rel.TagName
	if latest == "" {
		latest = rel.Name
	}
	return isUpdateAvailable(currentVersion, latest) && hasCompatibleAsset(rel)
}

func writeUpdateCache(path string, cached updateCheckCache) error {
	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "update-check-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
