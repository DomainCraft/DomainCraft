package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// httpClient is shared by remote version probes.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// httpGetBodyFn is the HTTP GET used by update probes. It is a variable so
// tests can substitute a fake and exercise the GitHub code paths offline.
var httpGetBodyFn = httpGetBody

// DefaultUpdateInterval is how often cached bridges are polled against their
// remote for newer versions. It mirrors the TTL used by common package
// managers: fresh enough that template fixes reach users, sparse enough that
// an invocation does not hit the network every time.
const DefaultUpdateInterval = 24 * time.Hour

// Update describes a detected newer version of a cached bridge. Callers hand it
// to the user (e.g. an interactive prompt) and then to ApplyUpdate.
type Update struct {
	Entry        RegistryEntry
	LocalVersion string // version of the cached checkout
	RemoteCommit string
	RemoteBranch string
}

// CheckForUpdate detects whether a cached bridge has a newer version upstream.
//
// The remote is only contacted when the cached copy was last checked more than
// interval ago (interval <= 0 always checks). The check is best-effort: an
// unreachable remote is treated as "up to date" rather than as an error, so
// offline generation keeps working with the cached copy. A nil result means the
// bridge is current (or the check was skipped).
func CheckForUpdate(entry RegistryEntry, interval time.Duration) (*Update, error) {
	meta, err := ensureMeta(entry)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, nil // not cached
	}

	now := time.Now()
	if interval > 0 && now.Sub(meta.LastCheckedAt) < interval {
		return nil, nil // checked recently enough — don't hit the network
	}

	// Keep the local commit in sync with the actual checkout so a manual pull
	// of the cache does not incorrectly report an update.
	if head := gitHead(entry); head != "" {
		meta.Commit = head
	}

	remoteBranch, remoteCommit, err := githubAPIHead(entry)

	// Record the attempt regardless of outcome so the freshness gate applies
	// even while offline (otherwise every run would wait on a timeout).
	meta.LastCheckedAt = now
	if werr := WriteMeta(entry, meta); werr != nil && err == nil {
		return nil, werr
	}

	if err != nil || remoteCommit == "" {
		return nil, nil // can't reach the remote — assume up to date
	}
	if remoteCommit == meta.Commit {
		return nil, nil // local checkout already at the remote tip
	}

	return &Update{
		Entry:        entry,
		LocalVersion: meta.Version,
		RemoteCommit: remoteCommit,
		RemoteBranch: remoteBranch,
	}, nil
}

// ApplyUpdate brings a cached bridge up to the upstream default branch,
// preserving the checkout via a shallow fetch + hard reset (a full re-clone is
// used as a fallback if the in-place update fails). It returns the new version.
func ApplyUpdate(entry RegistryEntry, u *Update) (string, error) {
	cacheDir := CachePath(entry)

	branch := "main"
	if u != nil && u.RemoteBranch != "" {
		branch = u.RemoteBranch
	} else if meta, err := ReadMeta(entry); err == nil && meta != nil && meta.Branch != "" {
		branch = meta.Branch
	}

	// Fast path: update the existing checkout in place.
	if _, err := runGit(cacheDir, "fetch", "--depth", "1", "origin", branch); err == nil {
		if _, err := runGit(cacheDir, "reset", "--hard", "FETCH_HEAD"); err == nil {
			return syncMetaAfterUpdate(entry, branch)
		}
		// reset failed — fall through to a clean re-clone
	}

	// Fallback: the existing checkout is unusable; replace it entirely.
	if err := os.RemoveAll(cacheDir); err != nil {
		return "", err
	}
	if err := CloneBridge(entry); err != nil {
		return "", err
	}
	return syncMetaAfterUpdate(entry, branch)
}

// syncMetaAfterUpdate refreshes the cached checkout's metadata after a
// successful download/update and returns the new bridge version.
func syncMetaAfterUpdate(entry RegistryEntry, branch string) (string, error) {
	meta := newMetaFromCache(entry)
	if branch != "" {
		meta.Branch = branch
	}
	if err := WriteMeta(entry, meta); err != nil {
		return meta.Version, err
	}
	return meta.Version, nil
}

// githubAPIHead resolves the upstream default branch and its head commit
// through the GitHub REST API. Two requests (repo metadata, then branch
// commit) that do not touch the local checkout at all.
func githubAPIHead(entry RegistryEntry) (branch, commit string, err error) {
	repoBody, err := httpGetBodyFn(fmt.Sprintf("https://api.github.com/repos/%s", entry.GitHub))
	if err != nil {
		return "", "", err
	}
	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(repoBody, &repo); err != nil {
		return "", "", err
	}
	if repo.DefaultBranch == "" {
		return "", "", fmt.Errorf("github API: no default branch for %s", entry.GitHub)
	}

	commitBody, err := httpGetBodyFn(fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", entry.GitHub, repo.DefaultBranch))
	if err != nil {
		return "", "", err
	}
	var c struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(commitBody, &c); err != nil {
		return "", "", err
	}
	if c.SHA == "" {
		return "", "", fmt.Errorf("github API: no head commit for %s/%s", entry.GitHub, repo.DefaultBranch)
	}
	return repo.DefaultBranch, c.SHA, nil
}

// httpGetBody performs a GET and returns the response body, limiting its size
// to 1 MiB to avoid unbounded reads from a misbehaving endpoint.
func httpGetBody(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// runGit runs a git command in dir and returns its trimmed stdout.
//
// The child inherits the environment, but git-location variables (GIT_DIR,
// GIT_WORK_TREE, ...) are scrubbed so a developer who exports them globally
// can never redirect a bridge-cache operation onto the wrong repository — the
// command always acts on the repo in dir.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitScrubbedEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitScrubbedEnv returns the process environment without git-location variables.
func gitScrubbedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY":
			continue
		}
		env = append(env, kv)
	}
	return env
}
