package packages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestResolveVersionPicksHighestStable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newtonsoft.json/index.json" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":["1.0.0","9.0.1","9.0.2-beta1","10.0.0-rc.2","9.0.2","2.0.0"]}`))
	}))
	defer srv.Close()

	got, err := ResolveVersion(srv.URL+"/{id}/index.json", "Newtonsoft.Json")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if got != "9.0.2" {
		t.Errorf("version = %q, want 9.0.2 (highest stable; pre-releases skipped)", got)
	}
}

func TestResolveVersionErrors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"registry error", http.StatusInternalServerError, `{}`, "registry returned 500"},
		{"malformed json", http.StatusOK, `{not json`, ""},
		{"no stable versions", http.StatusOK, `{"versions":["1.0.0-preview","2.0.0-rc.1"]}`, "no stable versions"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()

			got, err := ResolveVersion(srv.URL+"/{id}.json", "Some.Package")
			if err == nil {
				t.Fatalf("expected an error, got version %q", got)
			}
			if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %q, want it to contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

func TestResolveVersionUnreachableRegistry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening anymore

	if _, err := ResolveVersion(url+"/{id}", "Any"); err == nil {
		t.Fatal("expected a connection error for a dead registry")
	}
}
