package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var versionNumRegex = regexp.MustCompile(`[0-9]+`)

// CleanVersion removes common prefixes and distribution suffixes from a version string.
func CleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "bun-v")
	v = strings.TrimPrefix(v, "apache-maven-")
	v = strings.TrimPrefix(v, "apache-ant-")
	v = strings.TrimPrefix(v, "gradle-")
	v = strings.TrimPrefix(v, "release_")
	v = strings.TrimPrefix(v, "cpython-")
	v = strings.TrimPrefix(v, "yarn-v")
	v = strings.TrimPrefix(v, "v")

	// Strip distribution suffixes if present, e.g., "17-tem" or "17-zulu" or "17.0.2+7"
	if idx := strings.IndexAny(v, "+"); idx != -1 {
		v = v[:idx]
	}
	return v
}

// MatchVersion checks whether candidateVersion satisfies versionSpec.
func MatchVersion(candidateVersion string, versionSpec string) bool {
	if candidateVersion == "" || versionSpec == "" {
		return false
	}

	candClean := CleanVersion(candidateVersion)
	specClean := CleanVersion(versionSpec)

	// If versionSpec contains distribution parts (e.g. 17-tem), strip distribution for matching spec clean
	specVersionOnly := specClean
	if idx := strings.Index(specClean, "-"); idx != -1 {
		specVersionOnly = specClean[:idx]
	}

	if candClean == specClean || candClean == specVersionOnly {
		return true
	}

	// Check dotted or dashed prefix matching: e.g. "1.1" matches "1.1.0", "3" matches "3.9.6"
	if strings.HasPrefix(candClean, specVersionOnly+".") || strings.HasPrefix(candClean, specVersionOnly+"-") {
		return true
	}

	return false
}

// IsRC returns true if the version string represents a Release Candidate or Pre-release.
func IsRC(v string) bool {
	if v == "" {
		return false
	}
	clean := CleanVersion(v)
	lower := strings.ToLower(clean)

	rcKeywords := []string{
		"rc", "alpha", "beta", "dev", "milestone", "preview",
		"snapshot", "nightly", "next", "canary", "cr",
	}

	for _, kw := range rcKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	if strings.Contains(lower, "-ea") || strings.HasSuffix(lower, "ea") {
		return true
	}

	if regexp.MustCompile(`(?i)[-.]m[0-9]+`).MatchString(lower) {
		return true
	}

	if regexp.MustCompile(`(?i)[0-9]+[ab][0-9]+`).MatchString(lower) {
		return true
	}

	return false
}

func getMainNumericParts(v string) []int {
	c := CleanVersion(v)
	cut := len(c)
	if IsRC(c) {
		lower := strings.ToLower(c)
		for _, kw := range []string{"rc", "alpha", "beta", "dev", "milestone", "preview", "snapshot", "nightly", "next", "canary", "cr", "ea"} {
			if idx := strings.Index(lower, kw); idx != -1 && idx < cut {
				cut = idx
			}
		}
		if idx := regexp.MustCompile(`(?i)[-.]m[0-9]+`).FindStringIndex(lower); idx != nil && idx[0] < cut {
			cut = idx[0]
		}
		if idx := regexp.MustCompile(`(?i)[0-9]+[ab][0-9]+`).FindStringIndex(lower); idx != nil {
			match := lower[idx[0]:idx[1]]
			for i, ch := range match {
				if ch == 'a' || ch == 'b' || ch == 'A' || ch == 'B' {
					cutCandidate := idx[0] + i
					if cutCandidate < cut {
						cut = cutCandidate
					}
					break
				}
			}
		}
		if idx := strings.Index(c, "-"); idx != -1 && idx < cut {
			cut = idx
		}
	}

	prefix := c[:cut]
	rawParts := versionNumRegex.FindAllString(prefix, -1)
	if len(rawParts) == 0 {
		rawParts = versionNumRegex.FindAllString(c, -1)
	}

	var res []int
	for _, p := range rawParts {
		n, _ := strconv.Atoi(p)
		res = append(res, n)
	}
	return res
}

// CompareVersions compares two version strings numerically.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func CompareVersions(v1Str, v2Str string) int {
	c1 := CleanVersion(v1Str)
	c2 := CleanVersion(v2Str)

	if c1 == c2 {
		return 0
	}

	main1 := getMainNumericParts(c1)
	main2 := getMainNumericParts(c2)

	maxLen := len(main1)
	if len(main2) > maxLen {
		maxLen = len(main2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(main1) {
			n1 = main1[i]
		}
		if i < len(main2) {
			n2 = main2[i]
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	if len(main1) < len(main2) {
		return -1
	}
	if len(main1) > len(main2) {
		return 1
	}

	rc1 := IsRC(c1)
	rc2 := IsRC(c2)

	if !rc1 && rc2 {
		return 1
	}
	if rc1 && !rc2 {
		return -1
	}

	parts1 := versionNumRegex.FindAllString(c1, -1)
	parts2 := versionNumRegex.FindAllString(c2, -1)

	maxLenAll := len(parts1)
	if len(parts2) > maxLenAll {
		maxLenAll = len(parts2)
	}

	for i := 0; i < maxLenAll; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return strings.Compare(c1, c2)
}

// SelectBestVersion returns the highest matching version from available list for versionSpec.
func SelectBestVersion(versions []string, spec string, includePrerelease bool) (string, error) {
	specIsRC := IsRC(spec)

	var matches []string
	for _, v := range versions {
		if MatchVersion(v, spec) {
			if !includePrerelease && !specIsRC && IsRC(v) {
				continue
			}
			matches = append(matches, v)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no matching version found for spec '%s'", spec)
	}

	sort.Slice(matches, func(i, j int) bool {
		return CompareVersions(matches[i], matches[j]) < 0
	})

	return matches[len(matches)-1], nil
}

// FetchNPMVersions retrieves all published version strings for an npm package.
func FetchNPMVersions(ctx context.Context, pkgName string, mirrorBaseURL string) ([]string, error) {
	reqURL := fmt.Sprintf("https://registry.npmjs.org/%s", pkgName)
	if mirrorBaseURL != "" && !strings.Contains(mirrorBaseURL, "github.com") {
		reqURL = strings.TrimSuffix(mirrorBaseURL, "/")
		reqURL = strings.TrimSuffix(reqURL, "/-")
		reqURL = strings.TrimSuffix(reqURL, "/"+pkgName)
		if !strings.HasSuffix(reqURL, pkgName) {
			reqURL = fmt.Sprintf("%s/%s", reqURL, pkgName)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm registry returned status %s", resp.Status)
	}

	var data struct {
		Versions map[string]interface{} `json:"versions"`
		Time     map[string]interface{} `json:"time"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var versions []string
	for v := range data.Versions {
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		for v := range data.Time {
			if v != "created" && v != "modified" {
				versions = append(versions, v)
			}
		}
	}

	return versions, nil
}

// FetchGradleVersions retrieves version strings from services.gradle.org/versions/all.
func FetchGradleVersions(ctx context.Context, baseURL string) ([]string, error) {
	reqURL := "https://services.gradle.org/versions/all"
	if baseURL != "" {
		reqURL = strings.TrimSuffix(baseURL, "/")
		reqURL = strings.TrimSuffix(reqURL, "/distributions")
		if !strings.HasSuffix(reqURL, "/versions/all") {
			reqURL = fmt.Sprintf("%s/versions/all", reqURL)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gradle versions service returned status %s", resp.Status)
	}

	var gradleReleases []struct {
		Version  string `json:"version"`
		Snapshot bool   `json:"snapshot"`
		Nightly  bool   `json:"nightly"`
		Broken   bool   `json:"broken"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gradleReleases); err != nil {
		return nil, err
	}

	var versions []string
	for _, r := range gradleReleases {
		if !r.Snapshot && !r.Nightly && !r.Broken && r.Version != "" {
			versions = append(versions, r.Version)
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no valid gradle versions found at %s", reqURL)
	}

	return versions, nil
}

// FetchApacheAntVersions retrieves version strings from Apache Ant archive binaries index.
func FetchApacheAntVersions(ctx context.Context, baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = "https://archive.apache.org/dist/ant/binaries"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/", nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apache ant archive returned status %s", resp.Status)
	}

	dirRegex := regexp.MustCompile(`apache-ant-([0-9]+\.[0-9]+\.[0-9]+)-bin\.tar\.gz`)
	scanner := bufio.NewScanner(resp.Body)
	versionMap := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		matches := dirRegex.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) > 1 {
				versionMap[m[1]] = true
			}
		}
	}

	var versions []string
	for v := range versionMap {
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found at apache ant archive %s", baseURL)
	}

	return versions, nil
}

// FetchPyPIVersions retrieves all published version strings for a PyPI package.
func FetchPyPIVersions(ctx context.Context, pkgName string) ([]string, error) {
	reqURL := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkgName)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi returned status %s", resp.Status)
	}

	var data struct {
		Releases map[string]interface{} `json:"releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var versions []string
	for v := range data.Releases {
		versions = append(versions, v)
	}

	return versions, nil
}

// FetchGitHubReleases retrieves version strings from GitHub repo releases or Atom feed.
func FetchGitHubReleases(ctx context.Context, owner, repo string) ([]string, error) {
	return fetchGitHubReleases(ctx, owner, repo, true)
}

func fetchGitHubReleases(ctx context.Context, owner, repo string, requireAssets bool) ([]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	var versions []string
	for page := 1; page <= 3; page++ {
		reqURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100&page=%d", owner, repo, page)
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			break
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			break
		}

		var releases []struct {
			TagName string        `json:"tag_name"`
			Name    string        `json:"name"`
			Assets  []interface{} `json:"assets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil || len(releases) == 0 {
			resp.Body.Close()
			break
		}
		resp.Body.Close()

		for _, r := range releases {
			if requireAssets && len(r.Assets) == 0 {
				continue
			}
			v := r.TagName
			if v == "" {
				v = r.Name
			}
			if v != "" {
				versions = append(versions, v)
			}
		}
	}

	if len(versions) > 0 {
		return versions, nil
	}

	// Fallback to Atom feed
	atomURL := fmt.Sprintf("https://github.com/%s/%s/releases.atom", owner, repo)
	reqAtom, err := http.NewRequestWithContext(ctx, "GET", atomURL, nil)
	if err != nil {
		return nil, err
	}

	respAtom, err := client.Do(reqAtom)
	if err != nil {
		return nil, err
	}
	defer respAtom.Body.Close()

	if respAtom.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github atom feed returned status %s", respAtom.Status)
	}

	tagRegex := regexp.MustCompile(`tag/([^"]+)`)
	scanner := bufio.NewScanner(respAtom.Body)
	versionMap := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		matches := tagRegex.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) > 1 {
				versionMap[m[1]] = true
			}
		}
	}

	var atomVersions []string
	for v := range versionMap {
		atomVersions = append(atomVersions, v)
	}

	if len(atomVersions) == 0 {
		return nil, fmt.Errorf("no versions found in github atom feed for %s/%s", owner, repo)
	}

	return atomVersions, nil
}

// ResolveGitHubVersion fetches tags/releases for owner/repo and finds the best match for versionSpec.
func ResolveGitHubVersion(ctx context.Context, owner, repo, versionSpec string, includePrerelease bool) string {
	if versions, err := FetchGitHubReleases(ctx, owner, repo); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}

// FetchApacheMavenVersions retrieves version strings from Apache Maven archive index.
func FetchApacheMavenVersions(ctx context.Context, baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = "https://archive.apache.org/dist/maven/maven-3"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/", nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apache maven archive returned status %s", resp.Status)
	}

	dirRegex := regexp.MustCompile(`href="([0-9]+\.[0-9]+\.[0-9]+)/?"`)
	scanner := bufio.NewScanner(resp.Body)
	versionMap := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		matches := dirRegex.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) > 1 {
				versionMap[m[1]] = true
			}
		}
	}

	var versions []string
	for v := range versionMap {
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found at apache maven archive %s", baseURL)
	}

	return versions, nil
}
