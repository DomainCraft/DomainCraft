package packages

import (
	"os"
	"path/filepath"
	"time"

	"github.com/DomainCraft/DomainCraft/internal/appdir"
	"github.com/DomainCraft/DomainCraft/internal/fsutil"
)

// DefaultCacheTTL is how long a resolved package version is trusted before the
// registry is consulted again. Versions change rarely, so 24h keeps generation
// offline-friendly while still picking up real version bumps.
const DefaultCacheTTL = 24 * time.Hour

// CacheFileName is the file storing resolved package versions inside a bridge's
// cache directory.
const CacheFileName = "packages.json"

// versionEntry is one cached package version.
type versionEntry struct {
	Value    string    `json:"value"`
	StoredAt time.Time `json:"stored_at"`
}

// cacheKey derives a stable key for a (registry, package) pair.
func cacheKey(registryURL, packageID string) string {
	return registryURL + "\x00" + packageID
}

// resolveVersionFn is the actual registry lookup performed on a cache miss. It
// is a variable so tests can substitute a fake (avoiding a real network call).
var resolveVersionFn = ResolveVersion

// versionStore loads (or initializes) the package-version cache file for a
// single bridge. Every bridge keeps its own file
// (~/.domaincraft/cache/<bridge>/packages.json), so with many bridges the
// caches never mix and a corrupt one only affects its own bridge.
func versionStore(bridgeID string) (map[string]versionEntry, error) {
	dir, err := appdir.CacheDir(bridgeID)
	if err != nil {
		return nil, err
	}
	var data struct {
		Entries map[string]versionEntry `json:"entries"`
	}
	// A missing or corrupt cache file yields an empty cache rather than an
	// error, so a broken cache never breaks the caller.
	if err := fsutil.ReadJSON(filepath.Join(dir, CacheFileName), &data); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if data.Entries == nil {
		data.Entries = make(map[string]versionEntry)
	}
	return data.Entries, nil
}

// persistVersionCache writes the cache file atomically. Best-effort: a failed
// write must not fail generation (only cache warmth is lost).
func persistVersionCache(bridgeID string, entries map[string]versionEntry) {
	if dir, err := appdir.CacheDir(bridgeID); err == nil {
		_ = fsutil.WriteJSON(filepath.Join(dir, CacheFileName), struct {
			Entries map[string]versionEntry `json:"entries"`
		}{Entries: entries})
	}
}

// ResolveVersionCached resolves the latest stable version of a package from its
// registry, caching the result per bridge for DefaultCacheTTL. A fresh cached
// value avoids a network round-trip on every generation; a stale one triggers a
// refresh. A corrupt or missing cache is ignored and simply re-resolved.
func ResolveVersionCached(bridgeID, registryURL, packageID string) (string, error) {
	key := cacheKey(registryURL, packageID)

	entries, err := versionStore(bridgeID)
	if err != nil {
		// Cache directory unusable — resolve uncached rather than fail.
		return resolveVersionFn(registryURL, packageID)
	}

	if e, ok := entries[key]; ok && e.Value != "" && time.Since(e.StoredAt) <= DefaultCacheTTL {
		return e.Value, nil
	}

	version, err := resolveVersionFn(registryURL, packageID)
	if err != nil {
		return "", err
	}
	if version == "" {
		return "", nil
	}

	entries[key] = versionEntry{Value: version, StoredAt: time.Now()}
	persistVersionCache(bridgeID, entries)
	return version, nil
}
