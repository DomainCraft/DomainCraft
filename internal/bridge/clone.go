package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/DomainCraft/DomainCraft/internal/appdir"
)

// BridgeCacheDir returns ~/.domaincraft/bridges/.
// Falls back to .domaincraft/bridges in the current directory if the home directory is unavailable.
func BridgeCacheDir() string {
	dir, err := appdir.UnionDir("bridges")
	if err != nil {
		return filepath.Join(".domaincraft", "bridges")
	}
	return dir
}

// CachePath returns the local cache directory for a bridge entry.
func CachePath(entry RegistryEntry) string {
	return filepath.Join(BridgeCacheDir(), entry.ID)
}

// IsCached checks whether a bridge is already cloned and contains bridge.yaml.
func IsCached(entry RegistryEntry) bool {
	bridgeFile := filepath.Join(CachePath(entry), "bridge.yaml")
	_, err := os.Stat(bridgeFile)
	return err == nil
}

// EnsureOptions controls how a cached bridge is kept fresh. A nil value
// performs no update checks (the cached copy, if any, is used as-is).
type EnsureOptions struct {
	// Interval is how often the remote is polled for newer versions.
	//   - positive: contact the remote only when last checked longer ago
	//     than Interval (avoids a network round-trip on every invocation)
	//   - 0: use DefaultUpdateInterval
	//   - negative: disable update checks entirely
	Interval time.Duration

	// Force makes the cache check for and honour updates immediately,
	// bypassing the freshness gate in Interval. Non-interactive CI uses this
	// via --update-bridges to pull in the latest templates deterministically.
	Force bool

	// ConfirmUpdate is invoked when a newer version of a cached bridge is
	// detected. Returning true downloads the update; returning false keeps the
	// cached copy (the user made an informed choice, so no warning is shown).
	// When nil, the update is never applied and a warning is emitted instead.
	ConfirmUpdate func(entry RegistryEntry, update *Update) (bool, error)

	// Warn receives non-fatal notices (e.g. "update available, not applied").
	// May be nil to suppress them.
	Warn func(format string, args ...any)
}

// EnsureBridge clones the bridge from GitHub if not already cached, then keeps
// the cached copy fresh by checking the upstream repo for newer versions.
// Returns the local path to the bridge directory.
func EnsureBridge(entry RegistryEntry, opts *EnsureOptions) (string, error) {
	if entry.GitHub == "" {
		return "", fmt.Errorf("bridge %q has no GitHub repository configured", entry.ID)
	}

	cacheDir := CachePath(entry)
	if !IsCached(entry) {
		if err := CloneBridge(entry); err != nil {
			return "", err
		}

		// Verify the cloned repo contains bridge.yaml.
		if !IsCached(entry) {
			os.RemoveAll(cacheDir)
			return "", fmt.Errorf("cloned bridge %q does not contain bridge.yaml", entry.ID)
		}

		if err := WriteMeta(entry, newMetaFromCache(entry)); err != nil {
			// Metadata is advisory; the bridge is already usable.
			warnOpts(opts, "could not record cache metadata for %q: %v", entry.ID, err)
		}
		return cacheDir, nil
	}

	if opts != nil {
		// Cache is advisory: a failed freshness check must never break generation.
		if err := maybeUpdate(entry, opts); err != nil {
			warnOpts(opts, "bridge %q update check failed: %v", entry.ID, err)
		}
	}
	return cacheDir, nil
}

// maybeUpdate checks whether the cached bridge is outdated and, if so,
// consults the update policy to decide whether to download the new version.
func maybeUpdate(entry RegistryEntry, opts *EnsureOptions) error {
	interval := opts.Interval
	switch {
	case interval == 0:
		interval = DefaultUpdateInterval
	case interval < 0:
		// Update checks are disabled.
		return nil
	}
	if opts.Force {
		interval = 0 // check now, regardless of when we last looked
	}

	update, err := CheckForUpdate(entry, interval)
	if err != nil {
		return err
	}
	if update == nil {
		return nil // up to date (or the remote could not be reached)
	}

	// No policy set: keep the cached version and surface the availability.
	if opts.ConfirmUpdate == nil {
		if opts.Warn != nil {
			opts.Warn("bridge %q (v%s) has an update available; re-run with --update-bridges to download it",
				entry.ID, update.LocalVersion)
		}
		return nil
	}

	apply, err := opts.ConfirmUpdate(entry, update)
	if err != nil {
		return err
	}
	if !apply {
		// The user chose to keep the cached version — respect the choice.
		return nil
	}

	newVersion, err := ApplyUpdate(entry, update)
	if err != nil {
		return fmt.Errorf("update bridge %q: %w", entry.ID, err)
	}
	if opts.Warn != nil {
		opts.Warn("bridge %q updated to v%s", entry.ID, newVersion)
	}
	return nil
}

// warnOpts logs through opts.Warn when the options are present.
func warnOpts(opts *EnsureOptions, format string, args ...any) {
	if opts != nil && opts.Warn != nil {
		opts.Warn(format, args...)
	}
}

// CloneBridge performs a shallow git clone of the bridge repository.
func CloneBridge(entry RegistryEntry) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required to download bridges but was not found in PATH")
	}

	cacheDir := CachePath(entry)
	parent := filepath.Dir(cacheDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	url := fmt.Sprintf("https://github.com/%s.git", entry.GitHub)
	cmd := exec.Command("git", "clone", "--depth", "1", url, cacheDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(cacheDir)
		return fmt.Errorf("clone %s: %w: %s", url, err, out)
	}
	return nil
}
