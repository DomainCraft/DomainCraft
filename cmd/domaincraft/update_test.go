package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DomainCraft/DomainCraft/pkg/logger"
)

// fakeReleases returns an httpGetBodyFn that serves a fixed release payload.
func fakeReleases(tag string, assets map[string]string) func(string) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString(`{"tag_name":"` + tag + `","assets":[`)
	first := true
	for name, url := range assets {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(`{"name":"` + name + `","browser_download_url":"` + url + `"}`)
	}
	sb.WriteString("]}")
	return func(url string) ([]byte, error) {
		return []byte(sb.String()), nil
	}
}

func runUpdateCheck(t *testing.T, setVersion string) (string, error) {
	t.Helper()
	old := version
	version = setVersion
	t.Cleanup(func() { version = old })

	buf := &bytes.Buffer{}
	log := logger.New()
	log.SetWriter(buf)
	err := runCoreUpdate(log, true)
	return buf.String(), err
}

func TestUpdateDevBuildDoesNotCheck(t *testing.T) {
	oldHTTP := httpGetBodyFn
	httpGetBodyFn = func(url string) ([]byte, error) {
		t.Fatalf("network must not be contacted for a dev build (url=%s)", url)
		return nil, nil
	}
	defer func() { httpGetBodyFn = oldHTTP }()

	out, err := runUpdateCheck(t, "dev")
	if err != nil {
		t.Fatalf("runCoreUpdate: %v", err)
	}
	if !strings.Contains(out, "development build") {
		t.Fatalf("expected dev-build notice, got: %s", out)
	}
}

func TestUpdateCheckUpToDate(t *testing.T) {
	oldHTTP := httpGetBodyFn
	httpGetBodyFn = fakeReleases("v1.0.0", map[string]string{
		"domaincraft-linux-amd64": "https://example/dc-linux-amd64",
	})
	defer func() { httpGetBodyFn = oldHTTP }()

	out, err := runUpdateCheck(t, "v1.0.0")
	if err != nil {
		t.Fatalf("runCoreUpdate: %v", err)
	}
	if !strings.Contains(out, "Already up to date") {
		t.Fatalf("expected up-to-date message, got: %s", out)
	}
}

func TestUpdateCheckReportsNewer(t *testing.T) {
	oldHTTP := httpGetBodyFn
	httpGetBodyFn = fakeReleases("v1.2.0", map[string]string{
		"domaincraft-linux-amd64": "https://example/dc-linux-amd64",
	})
	defer func() { httpGetBodyFn = oldHTTP }()

	out, err := runUpdateCheck(t, "v1.0.0")
	if err != nil {
		t.Fatalf("runCoreUpdate: %v", err)
	}
	if !strings.Contains(out, "v1.2.0") || !strings.Contains(out, "v1.0.0") {
		t.Fatalf("expected both versions in output, got: %s", out)
	}
	if !strings.Contains(out, "`domaincraft update`") {
		t.Fatalf("expected install hint, got: %s", out)
	}
}

func TestUpdateCheckSamePatch(t *testing.T) {
	oldHTTP := httpGetBodyFn
	httpGetBodyFn = fakeReleases("v1.0.0", map[string]string{})
	defer func() { httpGetBodyFn = oldHTTP }()

	out, err := runUpdateCheck(t, "v1.0.0")
	if err != nil {
		t.Fatalf("runCoreUpdate: %v", err)
	}
	if !strings.Contains(out, "Already up to date") {
		t.Fatalf("expected up-to-date message, got: %s", out)
	}
}

func TestReleaseAssetFor(t *testing.T) {
	rel := &githubRelease{
		TagName: "v1.2.0",
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{Name: "domaincraft-linux-amd64", BrowserDownloadURL: "https://example/dc-linux-amd64"},
			{Name: "domaincraft-darwin-arm64", BrowserDownloadURL: "https://example/dc-darwin-arm64"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example/checksums.txt"},
		},
	}

	url, err := releaseAssetFor(rel, "domaincraft-linux-amd64")
	if err != nil {
		t.Fatalf("releaseAssetFor: %v", err)
	}
	if url != "https://example/dc-linux-amd64" {
		t.Fatalf("wrong url: %q", url)
	}

	if _, err := releaseAssetFor(rel, "domaincraft-riscv64"); err == nil {
		t.Fatal("expected error for missing asset")
	}
}

func TestDownloadAndReplace(t *testing.T) {
	oldHTTP := httpGetBodyFn
	httpGetBodyFn = func(url string) ([]byte, error) {
		return []byte("new-binary-content"), nil
	}
	defer func() { httpGetBodyFn = oldHTTP }()

	tmp, err := downloadBinary("https://example/dc")
	if err != nil {
		t.Fatalf("downloadBinary: %v", err)
	}
	defer os.Remove(tmp)

	// Replace a non-running target so the rename path is exercised directly.
	target := filepath.Join(t.TempDir(), "domaincraft.exe")
	if err := os.WriteFile(target, []byte("old-binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(tmp, target); err != nil {
		t.Fatalf("replaceExecutable: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(got) != "new-binary-content" {
		t.Fatalf("expected new content, got %q", got)
	}
}
