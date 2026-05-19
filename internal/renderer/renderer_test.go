package renderer

import (
	"os"
	"path/filepath"
	"testing"

	"domaincraft/internal/ir"
)

func TestRenderEntityTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	bridgeDir := filepath.Join(tmpDir, "bridge")
	if err := os.MkdirAll(filepath.Join(bridgeDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir bridge: %v", err)
	}

	bridgeYAML := []byte(`name: demo
output_dir: generated
templates:
  - for: entity
    source: templates/entity.tmpl
    targets:
      - "{{ .Entity.Name }}.txt"
      - "nested/{{ .Entity.Name }}.txt"
`)
	templateBytes := []byte(`{{ .Entity.Name }} -> {{ .Project.Name }}`)
	if err := os.WriteFile(filepath.Join(bridgeDir, "bridge.yaml"), bridgeYAML, 0o644); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bridgeDir, "templates", "entity.tmpl"), templateBytes, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	r, err := New(bridgeDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	project := &ir.IRProject{
		Name:     "TestProject",
		Entities: []ir.IREntity{{Name: "User", NamePlural: "Users"}},
	}
	written, err := r.Render(project, filepath.Join(tmpDir, "out"))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("got %d files, want 2", len(written))
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "out", "User.txt")); err != nil {
		t.Fatalf("expected generated file User.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "out", "nested", "User.txt")); err != nil {
		t.Fatalf("expected generated file nested/User.txt: %v", err)
	}
}
