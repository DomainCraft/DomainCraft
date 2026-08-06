package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolver maps a bridge identifier to a local filesystem path.
// The identifier can be:
//   - a local path (directory or file) — used directly
//   - a registry ID (e.g. "csharp-restful") — resolved via cache/clone
//   - empty — caller should prompt the user
type Resolver struct {
	registry *Registry
	ensure   *EnsureOptions // update/caching policy; nil = no update checks
}

// NewResolver creates a resolver backed by the given registry.
func NewResolver(registry *Registry) *Resolver {
	return &Resolver{registry: registry}
}

// WithEnsureOptions returns a resolver that applies the given caching/update
// policy whenever it clones or refreshes a bridge from a registry ID.
func (r *Resolver) WithEnsureOptions(opts *EnsureOptions) *Resolver {
	r2 := *r
	r2.ensure = opts
	return &r2
}

// Resolve maps a bridge identifier to a local path containing bridge.yaml.
// Returns ("", nil) when input is empty — caller must handle interactive selection.
func (r *Resolver) Resolve(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	// 1. Local path — use directly.
	if info, err := os.Stat(input); err == nil {
		if info.IsDir() {
			bridgeFile := filepath.Join(input, "bridge.yaml")
			if _, err := os.Stat(bridgeFile); err == nil {
				return input, nil
			}
			return "", fmt.Errorf("directory %q does not contain bridge.yaml", input)
		}
		return filepath.Dir(input), nil
	}

	// 2. Registry ID — check cache, clone if needed.
	if entry := r.registry.ByID(input); entry != nil {
		return EnsureBridge(*entry, r.ensure)
	}

	// 3. GitHub shorthand "owner/repo" — clone directly.
	if parts := strings.SplitN(input, "/", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		entry := RegistryEntry{
			ID:     parts[1],
			GitHub: input,
		}
		return EnsureBridge(entry, r.ensure)
	}

	return "", fmt.Errorf("bridge %q not found: not a local path, registry ID, or owner/repo", input)
}
