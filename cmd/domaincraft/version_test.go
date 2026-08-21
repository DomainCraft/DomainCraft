package main

import "testing"

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		stamped, buildInfo, want string
	}{
		{"v0.5.0", "v0.5.0", "v0.5.0"},             // stamped wins
		{"dev", "v0.5.0", "v0.5.0"},                // go install pkg@vX.Y.Z: clean tag
		{"dev", "(devel)", "dev"},                  // local build, no stamp
		{"dev", "v0.5.1-0.20260821125410-876219469a43", "dev"},                // pseudo-version = dev
		{"dev", "v0.5.1-0.20260821125410-876219469a43+dirty", "dev"},          // pseudo-version with build metadata
		{"dev", "", "dev"},                         // no build info at all
		{"", "v0.5.0", "v0.5.0"},                   // empty stamp, clean tag
		{"", "(devel)", "dev"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.stamped, c.buildInfo); got != c.want {
			t.Errorf("resolveVersion(%q, %q) = %q, want %q", c.stamped, c.buildInfo, got, c.want)
		}
	}
}
