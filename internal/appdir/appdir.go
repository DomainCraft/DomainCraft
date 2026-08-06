// Package appdir resolves the per-user DomainCraft directory (~/.domaincraft).
//
// It is the single source of truth for where DomainCraft keeps per-user state
// (cached bridges, resolved package versions, ...), so every component derives
// paths from the same base instead of duplicating the home-directory fallback.
package appdir

import (
	"os"
	"path/filepath"
)

// baseOverride, when set, redirects the app base directory. Used by tests so
// they never write into the user's real home directory.
var baseOverride string

// SetBaseForTesting redirects the app base directory for a test. Restore with
// SetBaseForTesting("") in a defer.
func SetBaseForTesting(dir string) {
	baseOverride = dir
}

// Base returns the per-user DomainCraft directory, typically ~/.domaincraft.
// Returns .domaincraft relative to the current directory when the home
// directory is unavailable. The directory is NOT created; callers that write
// under it are responsible for creating the subdirectories they need.
func Base() (string, error) {
	if baseOverride != "" {
		return baseOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".domaincraft"), nil
	}
	return filepath.Join(home, ".domaincraft"), nil
}

// UnionDir returns the path of a subdirectory under the app base directory,
// e.g. appdir.UnionDir("bridges") -> ~/.domaincraft/bridges.
func UnionDir(elem ...string) (string, error) {
	base, err := Base()
	if err != nil {
		return "", err
	}
	parts := append([]string{base}, elem...)
	return filepath.Join(parts...), nil
}

// CacheDir resolves a per-consumer cache directory under ~/.domaincraft/cache.
// Each consumer passes its own name so caches never mix, e.g.
// appdir.CacheDir("csharp-restful") -> ~/.domaincraft/cache/csharp-restful.
func CacheDir(names ...string) (string, error) {
	parts := append([]string{"cache"}, names...)
	return UnionDir(parts...)
}
