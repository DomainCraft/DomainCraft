package packages

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DomainCraft/DomainCraft/internal/appdir"
	"github.com/DomainCraft/DomainCraft/internal/fsutil"
)

// resetCacheRoot points the app base dir at a fresh temp dir for a test.
func resetCacheRoot(t *testing.T) {
	t.Helper()
	appdir.SetBaseForTesting(t.TempDir())
	t.Cleanup(func() { appdir.SetBaseForTesting("") })
}

// seedVersion writes an entry directly into a bridge's cache file with the
// given stored-at time, bypassing the normal time.Now() used on resolution.
func seedVersion(t *testing.T, bridgeID, key, version string, storedAt time.Time) {
	t.Helper()
	dir, err := appdir.CacheDir(bridgeID)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, CacheFileName)
	if err := fsutil.WriteJSON(path, struct {
		Entries map[string]struct {
			Value    string    `json:"value"`
			StoredAt time.Time `json:"stored_at"`
		} `json:"entries"`
	}{
		Entries: map[string]struct {
			Value    string    `json:"value"`
			StoredAt time.Time `json:"stored_at"`
		}{
			key: {Value: version, StoredAt: storedAt},
		},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestVersionStoreIsPerBridge(t *testing.T) {
	resetCacheRoot(t)

	dir, err := appdir.CacheDir("csharp-restful")
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	want := filepath.Join(dir, CacheFileName)

	old := resolveVersionFn
	resolveVersionFn = func(_, _ string) (string, error) { return "1.0.0", nil }
	defer func() { resolveVersionFn = old }()

	if _, err := ResolveVersionCached("csharp-restful", "https://reg.example/{id}", "demo.pkg"); err != nil {
		t.Fatalf("ResolveVersionCached: %v", err)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected per-bridge cache file at %s: %v", want, err)
	}
}

// TestResolveVersionCachedFreshHit verifies that a fresh cache entry short-
// circuits the registry lookup (no network, no resolveFn call).
func TestResolveVersionCachedFreshHit(t *testing.T) {
	resetCacheRoot(t)

	seedVersion(t, "demo", cacheKey("https://reg.example/{id}", "demo.pkg"), "1.0.0", time.Now())

	// Guard: if the cache path is not taken, the fake below would be called.
	called := false
	old := resolveVersionFn
	resolveVersionFn = func(_, _ string) (string, error) {
		called = true
		return "9.9.9", nil
	}
	defer func() { resolveVersionFn = old }()

	got, err := ResolveVersionCached("demo", "https://reg.example/{id}", "demo.pkg")
	if err != nil {
		t.Fatalf("ResolveVersionCached: %v", err)
	}
	if got != "1.0.0" {
		t.Fatalf("expected cached version 1.0.0, got %q", got)
	}
	if called {
		t.Fatal("registry must not be contacted for a fresh cache entry")
	}
}

// TestResolveVersionCachedStaleRefreshes verifies that an expired entry is
// re-resolved and the new version replaces it in the cache.
func TestResolveVersionCachedStaleRefreshes(t *testing.T) {
	resetCacheRoot(t)

	key := cacheKey("https://reg.example/{id}", "demo.pkg")
	seedVersion(t, "demo", key, "1.0.0", time.Now().Add(-48*time.Hour))

	oldFn := resolveVersionFn
	resolveVersionFn = func(_, _ string) (string, error) {
		return "2.0.0", nil
	}
	defer func() { resolveVersionFn = oldFn }()

	got, err := ResolveVersionCached("demo", "https://reg.example/{id}", "demo.pkg")
	if err != nil {
		t.Fatalf("ResolveVersionCached: %v", err)
	}
	if got != "2.0.0" {
		t.Fatalf("expected refreshed version 2.0.0, got %q", got)
	}

	// The refreshed value must be persisted and now serve as a fresh hit.
	resolveVersionFn = func(_, _ string) (string, error) {
		t.Fatal("must not contact registry after refresh")
		return "", nil
	}
	got2, err := ResolveVersionCached("demo", "https://reg.example/{id}", "demo.pkg")
	if err != nil || got2 != "2.0.0" {
		t.Fatalf("expected persisted refresh to serve 2.0.0, got %q (err %v)", got2, err)
	}
}

// TestResolveVersionCachedIsolation verifies caches are per bridge: a version
// cached for one bridge is not served to another.
func TestResolveVersionCachedIsolation(t *testing.T) {
	resetCacheRoot(t)

	seedVersion(t, "bridge-a", cacheKey("https://reg.example/{id}", "demo.pkg"), "1.0.0", time.Now())

	oldFn := resolveVersionFn
	resolveVersionFn = func(_, _ string) (string, error) {
		return "9.9.9", nil
	}
	defer func() { resolveVersionFn = oldFn }()

	// bridge-b has no cache -> must resolve (fresh) via the registry fake.
	got, err := ResolveVersionCached("bridge-b", "https://reg.example/{id}", "demo.pkg")
	if err != nil {
		t.Fatalf("ResolveVersionCached(bridge-b): %v", err)
	}
	if got != "9.9.9" {
		t.Fatalf("bridge-b must not read bridge-a's cache, got %q", got)
	}
}

// TestLoadCacheIgnoresCorruptFile ensures a corrupt cache file degrades
// gracefully (fresh empty cache) instead of failing generation.
func TestLoadCacheIgnoresCorruptFile(t *testing.T) {
	resetCacheRoot(t)

	dir, err := appdir.CacheDir("demo")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, CacheFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A corrupt file must not fail resolution; it should fall through to a
	// fresh registry lookup.
	oldFn := resolveVersionFn
	resolveVersionFn = func(_, _ string) (string, error) { return "1.0.0", nil }
	defer func() { resolveVersionFn = oldFn }()

	got, err := ResolveVersionCached("demo", "https://reg.example/{id}", "demo.pkg")
	if err != nil {
		t.Fatalf("ResolveVersionCached: %v", err)
	}
	if got != "1.0.0" {
		t.Fatalf("expected fresh resolve for corrupt cache, got %q", got)
	}
}
