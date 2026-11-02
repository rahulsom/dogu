package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ListVersions discovers available releases without installing a tool. Results
// are deduplicated and sorted newest first, including prereleases.
func ListVersions(ctx context.Context, name, mirror string) ([]string, error) {
	if _, err := FindTool(name); err != nil {
		return nil, err
	}
	versions, err := listVersions(ctx, name, mirror)
	if err != nil {
		return nil, fmt.Errorf("list versions for %s: %w", name, err)
	}
	seen := map[string]bool{}
	result := []string{}
	for _, v := range versions {
		if v != "" && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		c := CompareVersions(result[i], result[j])
		if c == 0 {
			return result[i] > result[j]
		}
		return c > 0
	})
	if len(result) == 0 {
		return nil, fmt.Errorf("no available versions found for %s", name)
	}
	return result, nil
}

func listVersions(ctx context.Context, name, mirror string) ([]string, error) {
	for _, d := range []toolDefinition{antDefinition, mavenDefinition, gradleDefinition, sbtDefinition, yarnDefinition, bazelDefinition, buck2Definition, millDefinition, groovyDefinition, kotlinDefinition, scalaDefinition, elmDefinition} {
		if d.name == name {
			return d.source.fetch(ctx, resolveBaseURL(d.baseURL, mirror), mirror)
		}
	}
	switch name {
	case "npm", "pnpm":
		return FetchNPMVersions(ctx, name, mirror)
	case "bun":
		return FetchGitHubReleases(ctx, "oven-sh", "bun")
	case "uv":
		return FetchPyPIVersions(ctx, "uv")
	case "pants":
		return FetchPyPIVersions(ctx, "pantsbuild.pants")
	case "python":
		// Release tags identify build dates; asset names identify Python versions.
		data, err := fetchReleaseData(ctx, "https://api.github.com/repos/indygreg/python-build-standalone/releases?per_page=100")
		if err != nil {
			return nil, err
		}
		matches := regexp.MustCompile(`cpython-([0-9]+\.[0-9]+\.[0-9]+(?:[abrc]+[0-9]+)?)-`).FindAllSubmatch(data, -1)
		var versions []string
		for _, m := range matches {
			versions = append(versions, string(m[1]))
		}
		return versions, nil
	case "node":
		data, err := fetchReleaseData(ctx, resolveBaseURL("https://nodejs.org/dist", mirror)+"/index.tab")
		if err != nil {
			return nil, err
		}
		var versions []string
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 && strings.HasPrefix(fields[0], "v") && fields[0] != "version" {
				versions = append(versions, fields[0])
			}
		}
		return versions, nil
	case "java":
		data, err := fetchReleaseData(ctx, resolveBaseURL("https://api.foojay.io/disco/v3.0/packages", mirror)+"?package_type=jdk&directly_downloadable=true")
		if err != nil {
			return nil, err
		}
		var response discoResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, err
		}
		var versions []string
		for _, p := range response.Result {
			dist := p.Distribution
			if dist == "temurin" {
				dist = "tem"
			}
			v := p.JavaVersion
			if dist != "" {
				v += "-" + dist
			}
			versions = append(versions, v)
		}
		return versions, nil
	case "go":
		data, err := fetchReleaseData(ctx, resolveBaseURL("https://go.dev/dl", mirror)+"/?mode=json&include=all")
		if err != nil {
			return nil, err
		}
		var index []struct{ Version string }
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, err
		}
		var versions []string
		for _, r := range index {
			versions = append(versions, strings.TrimPrefix(r.Version, "go"))
		}
		return versions, nil
	case "julia":
		data, err := fetchReleaseData(ctx, resolveBaseURL("https://julialang-s3.julialang.org/bin", mirror)+"/versions.json")
		if err != nil {
			return nil, err
		}
		var index map[string]json.RawMessage
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, err
		}
		var versions []string
		for v := range index {
			versions = append(versions, v)
		}
		return versions, nil
	case "ghc":
		return fetchIndexVersions(ctx, resolveBaseURL("https://downloads.haskell.org/~ghc", mirror))
	case "rust":
		return fetchGitHubReleases(ctx, "rust-lang", "rust", false)
	case "r":
		data, err := fetchReleaseData(ctx, resolveBaseURL("https://cran.r-project.org/src/base/R-4", mirror)+"/")
		if err != nil {
			return nil, err
		}
		matches := regexp.MustCompile(`R-([0-9]+\.[0-9]+\.[0-9]+)\.tar\.gz`).FindAllSubmatch(data, -1)
		var versions []string
		for _, m := range matches {
			versions = append(versions, string(m[1]))
		}
		return versions, nil
	}
	return nil, fmt.Errorf("version discovery unavailable for %s", name)
}
