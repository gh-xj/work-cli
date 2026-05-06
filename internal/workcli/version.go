package workcli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	appio "github.com/gh-xj/work-cli/internal/io"
)

const (
	versionSourceDefault     = "default"
	versionSourceGoBuildInfo = "go_build_info"
	versionSourceLdflags     = "ldflags"
)

const installHint = "go install github.com/gh-xj/work-cli/cmd/work@latest"

var fetchLatestRelease = fetchLatestGitHubRelease

type VersionCmd struct {
	Check bool `help:"check the latest GitHub release"`
}

func (c *VersionCmd) Run(globals *CLI) error {
	out := globals.stdout()
	version, source := detectAppVersion()
	data := versionPayload{
		SchemaVersion: "v1",
		Name:          binaryName,
		Version:       version,
		VersionSource: source,
		Commit:        appCommit,
		Date:          appDate,
	}
	if c.Check {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		latest, err := fetchLatestRelease(ctx)
		if err != nil {
			return err
		}
		data.Latest = latest.Version
		data.LatestURL = latest.URL
		data.InstallHint = installHint
		if cmp, ok := compareReleaseVersions(version, latest.Version); ok {
			outdated := cmp < 0
			data.Outdated = &outdated
			if outdated {
				data.Status = "outdated"
			} else {
				data.Status = "current"
			}
		} else {
			data.Status = "unknown"
		}
	}
	if globals.JSON {
		return appio.WriteJSON(out, data)
	}
	if !c.Check {
		_, err := fmt.Fprintf(out, "%s %s\n", data.Name, data.Version)
		return err
	}
	_, err := fmt.Fprintf(out, "%s %s (%s)\nlatest %s\nstatus %s\ninstall: %s\n", data.Name, data.Version, data.VersionSource, data.Latest, data.Status, data.InstallHint)
	return err
}

type versionPayload struct {
	SchemaVersion string `json:"schema_version"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	VersionSource string `json:"version_source"`
	Commit        string `json:"commit"`
	Date          string `json:"date"`
	Latest        string `json:"latest,omitempty"`
	LatestURL     string `json:"latest_url,omitempty"`
	Status        string `json:"status,omitempty"`
	Outdated      *bool  `json:"outdated,omitempty"`
	InstallHint   string `json:"install_hint,omitempty"`
}

type releaseVersion struct {
	Version string
	URL     string
}

func effectiveAppVersion() string {
	version, _ := detectAppVersion()
	return version
}

func detectAppVersion() (string, string) {
	if appVersion != "" && appVersion != "dev" {
		return appVersion, versionSourceLdflags
	}
	if version := goBuildInfoVersion(); version != "" {
		return version, versionSourceGoBuildInfo
	}
	if appVersion != "" {
		return appVersion, versionSourceDefault
	}
	return "dev", versionSourceDefault
}

func goBuildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	version := strings.TrimSpace(info.Main.Version)
	if version == "" || version == "(devel)" {
		return ""
	}
	return version
}

func fetchLatestGitHubRelease(ctx context.Context) (releaseVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/gh-xj/work-cli/releases/latest", nil)
	if err != nil {
		return releaseVersion{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "work-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return releaseVersion{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return releaseVersion{}, fmt.Errorf("check latest release: GitHub returned %s", resp.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return releaseVersion{}, err
	}
	if payload.TagName == "" {
		return releaseVersion{}, fmt.Errorf("check latest release: missing tag_name")
	}
	return releaseVersion{Version: payload.TagName, URL: payload.HTMLURL}, nil
}

func compareReleaseVersions(current, latest string) (int, bool) {
	currentParts, ok := releaseVersionParts(current)
	if !ok {
		return 0, false
	}
	latestParts, ok := releaseVersionParts(latest)
	if !ok {
		return 0, false
	}
	for i := range currentParts {
		if currentParts[i] < latestParts[i] {
			return -1, true
		}
		if currentParts[i] > latestParts[i] {
			return 1, true
		}
	}
	return 0, true
}

func releaseVersionParts(version string) ([3]int, bool) {
	var parts [3]int
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.Split(version, "-")[0]
	if version == "" || version == "dev" {
		return parts, false
	}
	rawParts := strings.Split(version, ".")
	if len(rawParts) > len(parts) {
		return parts, false
	}
	for i, raw := range rawParts {
		part, err := strconv.Atoi(raw)
		if err != nil {
			return parts, false
		}
		parts[i] = part
	}
	return parts, true
}
