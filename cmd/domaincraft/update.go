package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/DomainCraft/DomainCraft/internal/packages"
	"github.com/DomainCraft/DomainCraft/pkg/logger"

	"github.com/spf13/cobra"
)

// coreRepo is the GitHub owner/repo that publishes domaincraft releases.
const coreRepo = "DomainCraft/DomainCraft"

const releasesLatestURL = "https://api.github.com/repos/DomainCraft/DomainCraft/releases/latest"

// httpClient is the HTTP client used by update checks and binary downloads.
var httpClient = &http.Client{Timeout: 120 * time.Second}

// httpGetBodyFn is the HTTP GET used by the core update flow. It is a variable
// so tests can substitute a fake and exercise the GitHub code paths offline.
var httpGetBodyFn = httpGetBody

// githubRelease is the subset of the GitHub releases/latest payload we need.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// newUpdateCmd checks for and installs a newer version of the domaincraft core.
func newUpdateCmd() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the domaincraft core to the latest release",
		Long:  "Check the GitHub releases for a newer domaincraft version and, if available, download and replace the current executable.\nUse --check to only report whether a newer version exists, without downloading.",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New()
			log.SetWriter(cmd.OutOrStdout())
			return runCoreUpdate(log, checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only report whether a newer version exists (no download)")
	return cmd
}

// runCoreUpdate implements `domaincraft update` (and `domaincraft update --check`).
func runCoreUpdate(log *logger.Logger, checkOnly bool) error {
	current := strings.TrimPrefix(version, "v")
	if current == "" || current == "dev" {
		log.Info("Running a development build (v%s).", version)
		if !checkOnly {
			log.Info("To update, run: go install github.com/DomainCraft/DomainCraft/cmd/domaincraft@latest")
		}
		return nil
	}

	log.Info("Checking for updates (current: v%s)...", current)
	release, err := latestRelease()
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}
	latest := strings.TrimPrefix(release.TagName, "v")

	if !packages.VersionGreater(latest, current) {
		log.Success("Already up to date (v%s).", current)
		return nil
	}

	if checkOnly {
		log.Info("A newer version is available: v%s (current: v%s). Run `domaincraft update` to install it.", latest, current)
		return nil
	}

	assetURL, err := releaseAssetFor(release, assetName())
	if err != nil {
		return err
	}

	tmp, err := downloadBinary(assetURL)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	if err := replaceExecutable(tmp, exe); err != nil {
		return err
	}

	log.Success("Updated to v%s (was v%s). Re-run the command to use the new version.", latest, current)
	return nil
}

// latestRelease fetches the newest published release metadata.
func latestRelease() (*githubRelease, error) {
	body, err := httpGetBodyFn(releasesLatestURL)
	if err != nil {
		return nil, err
	}
	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("no releases found for %s", coreRepo)
	}
	return &rel, nil
}

// assetName returns the upload name of the binary for the current platform,
// matching the naming used by the release pipeline: domaincraft-<os>-<arch>,
// with a .exe suffix on Windows.
func assetName() string {
	name := fmt.Sprintf("domaincraft-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// releaseAssetFor finds the download URL of a release asset matching name.
func releaseAssetFor(rel *githubRelease, name string) (string, error) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no prebuilt binary %s for this release (platform %s/%s); use `go install` instead", name, runtime.GOOS, runtime.GOARCH)
}

// httpGetBody performs a GET and returns the response body, limiting its size
// to 64 MiB to avoid unbounded reads from a misbehaving endpoint.
func httpGetBody(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// downloadBinary fetches a release asset into a temp file and returns its path.
func downloadBinary(url string) (string, error) {
	data, err := httpGetBodyFn(url)
	if err != nil {
		return "", fmt.Errorf("download update: %w", err)
	}

	tmp, err := os.CreateTemp("", "domaincraft-update-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := tmp.Write(data); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmp.Name(), 0o755)
	}
	return tmp.Name(), nil
}

// replaceExecutable puts the new binary in place of the running executable. It
// first tries a direct rename; on platforms where the running binary is locked
// (Windows), it moves the current one aside first.
func replaceExecutable(tmp, exe string) error {
	if err := os.Rename(tmp, exe); err == nil {
		return nil
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("update in place failed (%v); replace %s manually or run `go install`", err, exe)
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Rename(old, exe) // restore the original
		return fmt.Errorf("could not install update: %w", err)
	}
	_ = os.Remove(old)
	return nil
}
