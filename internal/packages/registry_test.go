package packages

import "testing"

func TestVersionGreater(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"major greater", "2.0.0", "1.0.0", true},
		{"major lesser", "1.0.0", "2.0.0", false},
		{"minor greater", "1.2.0", "1.1.0", true},
		{"minor lesser", "1.1.0", "1.2.0", false},
		{"patch greater", "1.0.2", "1.0.1", true},
		{"patch lesser", "1.0.1", "1.0.2", false},
		{"equal", "1.0.0", "1.0.0", false},
		{"different lengths", "1.0", "1.0.0", false},
		{"different lengths reverse", "1.0.0", "1.0", false},
		{"three-part major", "10.0.0", "9.0.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VersionGreater(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("VersionGreater(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
