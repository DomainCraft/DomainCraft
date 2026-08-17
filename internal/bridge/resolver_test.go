package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolver_LocalPath(t *testing.T) {
	root := t.TempDir()
	bridgeDir := filepath.Join(root, "my-bridge")
	if err := os.MkdirAll(bridgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bridgeDir, "bridge.yaml"), []byte("name: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(&Registry{})

	// Directory path (absolute, relative, and a bare CWD name).
	for _, input := range []string{
		bridgeDir,
		filepath.Join(bridgeDir, "bridge.yaml"), // file itself → its dir
	} {
		got, err := r.Resolve(input)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", input, err)
		}
		if got != bridgeDir {
			t.Fatalf("Resolve(%q) = %q, want %q", input, got, bridgeDir)
		}
	}

	// Bare directory name resolvable from the CWD (backwards compatible).
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck // test-only cleanup
	got, err := r.Resolve("my-bridge")
	if err != nil {
		t.Fatalf("Resolve(my-bridge): %v", err)
	}
	if got != "my-bridge" {
		t.Fatalf("Resolve(my-bridge) = %q, want %q", got, "my-bridge")
	}
}

func TestResolver_MissingPathLikeIsErrorNotClone(t *testing.T) {
	r := NewResolver(&Registry{})

	for _, input := range []string{
		"../domaincraftcsharp",   // the regression: must NOT clone github.com/../domaincraftcsharp
		"./no-such-bridge",       // explicit relative path
		"/no/such/bridge",        // absolute path
		"~/no-such-bridge",       // home-relative
		"no\\such\\bridge",       // Windows-style relative
		"C:/no/such/bridge",      // Windows drive
	} {
		_, err := r.Resolve(input)
		if err == nil {
			t.Fatalf("Resolve(%q): expected error for missing path-like input", input)
		}
		if !strings.Contains(err.Error(), "no such local path") {
			t.Fatalf("Resolve(%q) error = %q, want 'no such local path'", input, err)
		}
		if strings.Contains(err.Error(), "github.com") {
			t.Fatalf("Resolve(%q) must not fall through to a GitHub clone: %v", input, err)
		}
	}
}

func TestLooksLikePath(t *testing.T) {
	pathLike := []string{
		"../x", "./x", ".", "..",
		"/abs", `\abs`, "~/home", "C:/x", `C:\x`, `a\b`,
	}
	notPathLike := []string{
		"", "csharp-restful", "DomainCraft/domaincraft-bridge-csharp", "owner/repo", "my-bridge",
	}
	for _, in := range pathLike {
		if !looksLikePath(in) {
			t.Errorf("looksLikePath(%q) = false, want true", in)
		}
	}
	for _, in := range notPathLike {
		if looksLikePath(in) {
			t.Errorf("looksLikePath(%q) = true, want false", in)
		}
	}
}
