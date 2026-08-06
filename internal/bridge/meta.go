package bridge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DomainCraft/DomainCraft/internal/fsutil"

	"gopkg.in/yaml.v3"
)

// MetaFileName is the cache-metadata filename stored inside each cached bridge.
// It records where a bridge was downloaded from and which version/commit it is
// on, so the CLI can detect when a newer upstream version is available without
// re-cloning or re-reading the whole checkout.
const MetaFileName = ".domaincraft-meta.json"

// CacheMeta records the origin and version of a cached bridge checkout.
type CacheMeta struct {
	ID      string `json:"id"`      // bridge registry ID, e.g. "csharp-restful"
	Source  string `json:"source"`  // GitHub "owner/repo"
	Version string `json:"version"` // bridge.yaml version at download/update time
	Commit  string `json:"commit"`  // git HEAD of the local checkout
	Branch  string `json:"branch"`  // upstream default branch
	// ClonedAt marks when the bridge was first downloaded.
	ClonedAt time.Time `json:"cloned_at"`
	// LastCheckedAt is updated every time the remote is actually contacted, so
	// the freshness gate (EnsureOptions.Interval) can avoid a network call on
	// every invocation.
	LastCheckedAt time.Time `json:"last_checked_at"`
}

// MetaPath returns the cache-metadata file path for a bridge entry.
func MetaPath(entry RegistryEntry) string {
	return filepath.Join(CachePath(entry), MetaFileName)
}

// ReadMeta loads cache metadata for a bridge. It returns (nil, nil) when no
// metadata file exists (e.g. a bridge cached before metadata was introduced).
func ReadMeta(entry RegistryEntry) (*CacheMeta, error) {
	var meta CacheMeta
	if err := fsutil.ReadJSON(MetaPath(entry), &meta); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &meta, nil
}

// WriteMeta persists cache metadata atomically (write to a temp file, then
// rename) so a crash mid-write never leaves a corrupt metadata file behind.
func WriteMeta(entry RegistryEntry, meta *CacheMeta) error {
	if meta == nil {
		return fmt.Errorf("cannot write nil cache metadata")
	}
	return fsutil.WriteJSON(MetaPath(entry), meta)
}

// ensureMeta returns the cache metadata for a cached bridge, synthesizing it
// from the on-disk checkout when the metadata file is missing.
func ensureMeta(entry RegistryEntry) (*CacheMeta, error) {
	meta, err := ReadMeta(entry)
	if err != nil {
		return nil, err
	}
	if meta != nil {
		return meta, nil
	}
	// Bridges cached before metadata existed: build it from the checkout.
	if !IsCached(entry) {
		return nil, nil
	}
	return newMetaFromCache(entry), nil
}

// newMetaFromCache builds metadata from the current on-disk checkout. It is
// best-effort: missing git or an unreadable bridge.yaml degrade gracefully.
func newMetaFromCache(entry RegistryEntry) *CacheMeta {
	now := time.Now()
	meta := &CacheMeta{
		ID:            entry.ID,
		Source:        entry.GitHub,
		Version:       readBridgeVersion(entry),
		Commit:        gitHead(entry),
		Branch:        gitBranch(entry),
		ClonedAt:      now,
		LastCheckedAt: now,
	}
	return meta
}

// readBridgeVersion parses the `version:` field out of a cached bridge.yaml.
// Returns "" when the field is absent or the file cannot be read.
func readBridgeVersion(entry RegistryEntry) string {
	data, err := os.ReadFile(filepath.Join(CachePath(entry), "bridge.yaml"))
	if err != nil {
		return ""
	}
	var cfg struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.Version
}

// gitHead returns the current git HEAD commit of the cached checkout, or "".
func gitHead(entry RegistryEntry) string {
	out, err := runGit(CachePath(entry), "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

// gitBranch returns the current branch of the cached checkout, or "".
func gitBranch(entry RegistryEntry) string {
	out, err := runGit(CachePath(entry), "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return out
}
