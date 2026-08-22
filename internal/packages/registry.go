// Package packages provides package registry resolution for bridge dependencies.
package packages

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var registryClient = &http.Client{Timeout: 5 * time.Second}

// ResolveVersion queries a package registry (e.g. NuGet) for the latest stable version
// of a package. The registryURL template must contain {id} which is replaced with the package ID.
func ResolveVersion(registryURL string, packageID string) (string, error) {
	url := strings.ReplaceAll(registryURL, "{id}", strings.ToLower(packageID))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := registryClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned %d for %s", resp.StatusCode, packageID)
	}

	var result struct {
		Versions []string `json:"versions"`
	}
	// Limit response body to 1MB to prevent OOM from malicious/misconfigured registries.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", err
	}

	// Filter for stable versions (no pre-release suffix) and find the maximum.
	var best string
	for _, v := range result.Versions {
		if strings.ContainsAny(v, "-") {
			continue
		}
		if best == "" || VersionGreater(v, best) {
			best = v
		}
	}
	if best == "" {
		return "", fmt.Errorf("no stable versions found for %s", packageID)
	}

	return best, nil
}

// VersionGreater compares two semver strings and returns true if a > b.
func VersionGreater(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := range maxLen {
		var ai, bi int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &ai)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bi)
		}
		if ai > bi {
			return true
		}
		if ai < bi {
			return false
		}
	}
	return false
}
