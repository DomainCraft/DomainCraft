package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// BridgeCacheDir returns ~/.domaincraft/bridges/.
// Falls back to .domaincraft/bridges in the current directory if the home directory is unavailable.
func BridgeCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".domaincraft", "bridges")
	}
	return filepath.Join(home, ".domaincraft", "bridges")
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

// EnsureBridge clones the bridge from GitHub if not already cached.
// Returns the local path to the bridge directory.
func EnsureBridge(entry RegistryEntry) (string, error) {
	if entry.GitHub == "" {
		return "", fmt.Errorf("bridge %q has no GitHub repository configured", entry.ID)
	}

	cacheDir := CachePath(entry)
	if IsCached(entry) {
		return cacheDir, nil
	}

	if err := CloneBridge(entry); err != nil {
		return "", err
	}
	return cacheDir, nil
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
