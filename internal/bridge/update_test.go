package bridge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DomainCraft/DomainCraft/internal/appdir"
)

// gitAvailable reports whether git is installed (bridge update handling needs it).
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// testGit runs git in dir and returns combined output, failing the test on error.
func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(gitScrubbedEnv(), "GIT_AUTHOR_NAME=domaincraft-test", "GIT_AUTHOR_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// makeBridgeRepo creates a non-bare git repo at dir with a bridge.yaml committed
// on the given branch. Returns the repo path.
func makeBridgeRepo(t *testing.T, dir, branch string) {
	t.Helper()
	testGit(t, dir, "init", "-q", "-b", branch)
	testGit(t, dir, "config", "user.email", "test@domaincraft.dev")
	testGit(t, dir, "config", "user.name", "DomainCraft Test")
	writeBridgeYAML(t, dir, "1.0.0")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-q", "-m", "init")
}

// writeBridgeYAML writes a minimal bridge.yaml with the given version.
func writeBridgeYAML(t *testing.T, dir, version string) {
	t.Helper()
	body := "name: test-bridge\nversion: \"" + version + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "bridge.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write bridge.yaml: %v", err)
	}
}

// cacheEntry returns a RegistryEntry whose cache path is under the test cache root.
func cacheEntry(id, source string) RegistryEntry {
	return RegistryEntry{ID: id, GitHub: source}
}

func TestMetaRoundTrip(t *testing.T) {
	root := t.TempDir()
	appdir.SetBaseForTesting(root)
	defer appdir.SetBaseForTesting("")

	entry := cacheEntry("test-bridge", "owner/bridge")

	// Nothing cached yet -> no meta, not cached.
	if _, err := ReadMeta(entry); err != nil {
		t.Fatalf("ReadMeta on missing meta: %v", err)
	}

	now := time.Now()
	meta := &CacheMeta{
		ID:            entry.ID,
		Source:        entry.GitHub,
		Version:       "1.0.0",
		Commit:        "abc123",
		Branch:        "main",
		ClonedAt:      now,
		LastCheckedAt: now,
	}
	if err := WriteMeta(entry, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}

	got, err := ReadMeta(entry)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	if got.Version != "1.0.0" || got.Commit != "abc123" || got.Source != "owner/bridge" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if !got.ClonedAt.Equal(now) {
		t.Fatalf("ClonedAt not preserved: got %v want %v", got.ClonedAt, now)
	}
}

// TestCheckAndApplyUpdate exercises the full freshness flow: GitHub API
// detection (faked offline) plus a real git fetch+reset apply against a local
// bare remote, so it doesn't depend on the network.
func TestCheckAndApplyUpdate(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	appdir.SetBaseForTesting(root)
	defer appdir.SetBaseForTesting("")

	// Bare remote acting as the bridge's origin.
	remote := filepath.Join(root, "remote.git")
	testGit(t, root, "init", "--bare", "-b", "main", "remote.git")
	testGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")

	// Working repo that we push to the remote.
	src := filepath.Join(root, "src")
	os.MkdirAll(src, 0o755)
	makeBridgeRepo(t, src, "main")
	testGit(t, src, "remote", "add", "origin", remote)
	testGit(t, src, "push", "-q", "-u", "origin", "main")

	// "Download" the bridge into our cache just like EnsureBridge would.
	entry := RegistryEntry{ID: "test-bridge", GitHub: "owner/test-bridge"}
	cacheDir := CachePath(entry)
	testGit(t, root, "clone", "-q", remote, cacheDir)
	if err := WriteMeta(entry, newMetaFromCache(entry)); err != nil {
		t.Fatalf("write meta after clone: %v", err)
	}

	// Fake the GitHub API: report the current head of the local bare remote.
	headCommit := func() string { return testGit(t, remote, "rev-parse", "HEAD") }
	oldHTTP := httpGetBodyFn
	httpGetBodyFn = func(url string) ([]byte, error) {
		switch {
		case strings.Contains(url, "/commits/main"):
			return []byte(`{"sha":"` + headCommit() + `"}`), nil
		case strings.Contains(url, "/repos/owner/test-bridge"):
			return []byte(`{"default_branch":"main"}`), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { httpGetBodyFn = oldHTTP }()

	// No update yet: local checkout already at the remote tip.
	upd, err := CheckForUpdate(entry, 0)
	if err != nil {
		t.Fatalf("CheckForUpdate (fresh): %v", err)
	}
	if upd != nil {
		t.Fatalf("expected no update, got %+v", upd)
	}

	// Freeze the last-checked timestamp so the freshness gate is exercised.
	meta, err := ReadMeta(entry)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	meta.LastCheckedAt = time.Now()
	if err := WriteMeta(entry, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}

	// Within the TTL the remote must NOT be contacted -> nil update.
	if upd, err := CheckForUpdate(entry, time.Hour); err != nil || upd != nil {
		t.Fatalf("expected TTL gate to skip check (upd=%+v err=%v)", upd, err)
	}

	// Publish a newer version upstream.
	writeBridgeYAML(t, src, "2.0.0")
	testGit(t, src, "add", ".")
	testGit(t, src, "commit", "-q", "-m", "bump version")
	testGit(t, src, "push", "-q", "origin", "main")

	// Now an update is detected.
	upd, err = CheckForUpdate(entry, 0)
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if upd == nil {
		t.Fatalf("expected an update after bumping the remote version")
	}
	if upd.LocalVersion != "1.0.0" {
		t.Fatalf("wrong local version: %q", upd.LocalVersion)
	}

	// Applying the update brings the checkout to v2.0.0.
	newVer, err := ApplyUpdate(entry, upd)
	if err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}
	if newVer != "2.0.0" {
		t.Fatalf("expected update to 2.0.0, got %q", newVer)
	}

	// The local HEAD now matches the remote tip -> no more update.
	if upd2, err := CheckForUpdate(entry, 0); err != nil || upd2 != nil {
		t.Fatalf("expected no update after applying (upd=%+v err=%v)", upd2, err)
	}
}

func TestEnsureMetaSynthesizesForLegacyCache(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	appdir.SetBaseForTesting(root)
	defer appdir.SetBaseForTesting("")

	entry := RegistryEntry{ID: "legacy-bridge", GitHub: "owner/legacy-bridge"}
	cacheDir := CachePath(entry)
	os.MkdirAll(cacheDir, 0o755)
	makeBridgeRepo(t, cacheDir, "main")

	// No metadata file exists (legacy cache), but the checkout is usable.
	meta, err := ensureMeta(entry)
	if err != nil {
		t.Fatalf("ensureMeta: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata to be synthesized for a cached bridge")
	}
	if meta.Version != "1.0.0" {
		t.Fatalf("expected synthesized version 1.0.0, got %q", meta.Version)
	}
	if meta.Commit == "" || meta.Branch != "main" {
		t.Fatalf("expected git-derived metadata, got %+v", meta)
	}
}

// TestCheckForUpdateViaGitHubAPI verifies update detection through the GitHub
// REST API for a cached bridge that has no usable git checkout (e.g. a bridge
// directory copied without its .git).
func TestCheckForUpdateViaGitHubAPI(t *testing.T) {
	root := t.TempDir()
	appdir.SetBaseForTesting(root)
	defer appdir.SetBaseForTesting("")

	entry := RegistryEntry{ID: "github-bridge", GitHub: "owner/github-bridge"}
	cacheDir := CachePath(entry)
	os.MkdirAll(cacheDir, 0o755)
	writeBridgeYAML(t, cacheDir, "1.0.0")

	// No .git in the checkout, but metadata records the local commit.
	meta := &CacheMeta{
		ID:            entry.ID,
		Source:        entry.GitHub,
		Version:       "1.0.0",
		Commit:        "old-local-commit",
		Branch:        "main",
		ClonedAt:      time.Now().Add(-24 * time.Hour),
		LastCheckedAt: time.Now().Add(-24 * time.Hour),
	}
	if err := WriteMeta(entry, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}

	// Fake the GitHub API responses.
	oldHTTP := httpGetBodyFn
	httpGetBodyFn = func(url string) ([]byte, error) {
		switch {
		case strings.Contains(url, "/commits/main"):
			return []byte(`{"sha":"new-remote-commit"}`), nil
		case strings.Contains(url, "/repos/owner/github-bridge"):
			return []byte(`{"default_branch":"main"}`), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { httpGetBodyFn = oldHTTP }()

	upd, err := CheckForUpdate(entry, 0)
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if upd == nil {
		t.Fatal("expected an update to be detected via the GitHub API")
	}
	if upd.RemoteCommit != "new-remote-commit" || upd.RemoteBranch != "main" {
		t.Fatalf("unexpected update info: %+v", upd)
	}
	if upd.LocalVersion != "1.0.0" {
		t.Fatalf("wrong local version: %q", upd.LocalVersion)
	}
}
