package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/bridge"
	"github.com/DomainCraft/DomainCraft/pkg/logger"
)

// writeBridgeDir scaffolds a minimal bridge directory (just a bridge.yaml,
// optionally declaring `extends:`) and returns its absolute path.
func writeBridgeDir(t *testing.T, dir, extends string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "name: " + filepath.Base(dir) + "\n"
	if extends != "" {
		content += "extends: " + extends + "\n"
	}
	path := filepath.Join(dir, "bridge.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return dir
}

func TestResolveBaseChainOrdersOutermostBaseFirst(t *testing.T) {
	root := t.TempDir()

	c := writeBridgeDir(t, filepath.Join(root, "bridge-c"), "")
	b := writeBridgeDir(t, filepath.Join(root, "bridge-b"), c)
	a := writeBridgeDir(t, filepath.Join(root, "bridge-a"), b)

	chain, err := resolveBaseChain(a, b, logger.New())
	if err != nil {
		t.Fatalf("resolveBaseChain: %v", err)
	}

	if len(chain) != 2 {
		t.Fatalf("chain = %v, want the two bases (the adapter itself is not part of the chain)", chain)
	}
	if chain[0] != c || chain[1] != b {
		t.Errorf("chain = %v, want [%s %s] (deepest base first)", chain, c, b)
	}
}

func TestResolveBaseChainDetectsCycle(t *testing.T) {
	root := t.TempDir()

	b := writeBridgeDir(t, filepath.Join(root, "bridge-b"), "")
	a := writeBridgeDir(t, filepath.Join(root, "bridge-a"), b)
	// Close the cycle: B now extends A while A extends B.
	writeBridgeDir(t, filepath.Join(root, "bridge-b"), a)

	_, err := resolveBaseChain(a, b, logger.New())
	if err == nil {
		t.Fatal("expected a cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("err = %v, want it to mention the cycle", err)
	}
}

func TestResolveExtendsRefPrefersLocalCheckout(t *testing.T) {
	root := t.TempDir()
	resolver := bridge.NewResolver(bridge.Default())

	// Registry ID whose repo checkout sits next to the extending bridge
	// (monorepo sibling layout) — must resolve locally without cloning.
	sibling := writeBridgeDir(t, filepath.Join(root, "domaincraft-bridge-csharp"), "")

	got, err := resolveExtendsRef("csharp-restful", filepath.Join(root, "adapter"), resolver)
	if err != nil {
		t.Fatalf("resolveExtendsRef: %v", err)
	}
	if abs, _ := filepath.Abs(got); abs != sibling {
		t.Errorf("resolved = %q, want the sibling checkout %q", got, sibling)
	}

	// CI layout: the base bridge checked out inside the adapter's directory.
	innerRoot := t.TempDir()
	innerAdapter := filepath.Join(innerRoot, "adapter")
	inner := writeBridgeDir(t, filepath.Join(innerAdapter, "domaincraft-bridge-csharp"), "")

	got, err = resolveExtendsRef("csharp-restful", innerAdapter, resolver)
	if err != nil {
		t.Fatalf("resolveExtendsRef: %v", err)
	}
	if abs, _ := filepath.Abs(got); abs != inner {
		t.Errorf("resolved = %q, want the in-directory checkout %q", got, inner)
	}
}

func TestVersionDelta(t *testing.T) {
	cases := []struct {
		name string
		u    *bridge.Update
		want string
	}{
		{"nil update", nil, ""},
		{"unknown local version", &bridge.Update{}, ""},
		{"known local version", &bridge.Update{LocalVersion: "1.2.3"}, "(v1.2.3 available)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := versionDelta(c.u); got != c.want {
				t.Errorf("versionDelta = %q, want %q", got, c.want)
			}
		})
	}
}
