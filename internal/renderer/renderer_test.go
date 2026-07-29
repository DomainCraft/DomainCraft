package renderer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/ir"
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

	r, err := New(bridgeDir, nil)
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

func TestRawSeedValue(t *testing.T) {
	tests := []struct {
		name   string
		value  interface{}
		dbType string
		want   string
	}{
		{"integer", 42, "int", "42"},
		{"float", 3.14, "float", "3.14"},
		{"boolean true", true, "boolean", "true"},
		{"boolean false", false, "boolean", "false"},
		{"boolean string true", "true", "boolean", "true"},
		{"boolean string false", "false", "boolean", "false"},
		{"boolean numeric 1", "1", "boolean", "true"},
		{"string value", "hello", "string", `"hello"`},
		{"uuid value", "550e8400-e29b-41d4-a716-446655440000", "uuid", `"550e8400-e29b-41d4-a716-446655440000"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rawSeedValue(tt.value, tt.dbType)
			if got != tt.want {
				t.Errorf("rawSeedValue(%v, %q) = %q, want %q", tt.value, tt.dbType, got, tt.want)
			}
		})
	}
}

func TestRenderContext_HasFeature(t *testing.T) {
	tests := []struct {
		name     string
		features map[string]bool
		feature  string
		want     bool
	}{
		{"audit enabled", map[string]bool{"audit": true}, "audit", true},
		{"audit disabled", map[string]bool{"audit": false}, "audit", false},
		{"nil features", nil, "audit", false},
		{"nil entity", nil, "audit", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := RenderContext{
				Entity: &ir.IREntity{Features: tt.features},
			}
			if tt.name == "nil entity" {
				ctx.Entity = nil
			}
			if got := ctx.HasFeature(tt.feature); got != tt.want {
				t.Errorf("HasFeature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestRenderContext_HasAudit(t *testing.T) {
	ctx := RenderContext{
		Entity: &ir.IREntity{Features: map[string]bool{"audit": true}},
	}
	if !ctx.HasAudit() {
		t.Error("HasAudit() = false, want true")
	}

	ctx2 := RenderContext{Entity: nil}
	if ctx2.HasAudit() {
		t.Error("HasAudit() = true on nil entity, want false")
	}
}

func TestRenderContext_HasSoftDelete(t *testing.T) {
	ctx := RenderContext{
		Entity: &ir.IREntity{Features: map[string]bool{"soft_delete": true}},
	}
	if !ctx.HasSoftDelete() {
		t.Error("HasSoftDelete() = false, want true")
	}
}

func TestRenderContext_HasOptimisticLock(t *testing.T) {
	ctx := RenderContext{
		Entity: &ir.IREntity{Features: map[string]bool{"optimistic_lock": true}},
	}
	if !ctx.HasOptimisticLock() {
		t.Error("HasOptimisticLock() = false, want true")
	}
}

func TestRenderContext_HasAuditLog(t *testing.T) {
	ctx := RenderContext{
		Entity: &ir.IREntity{Features: map[string]bool{"audit_log": true}},
	}
	if !ctx.HasAuditLog() {
		t.Error("HasAuditLog() = false, want true")
	}
}

func TestRenderContext_Permissions(t *testing.T) {
	ctx := RenderContext{Entity: nil}
	if ctx.Permissions() != nil {
		t.Error("Permissions() should return nil for nil entity")
	}

	perms := &ir.IRPermissions{Read: []string{"Admin"}}
	ctx2 := RenderContext{
		Entity: &ir.IREntity{Permissions: perms},
	}
	if ctx2.Permissions() != perms {
		t.Error("Permissions() should return entity permissions")
	}
}

func TestRenderContext_PrimaryKey(t *testing.T) {
	ctx := RenderContext{Entity: nil}
	if ctx.PrimaryKey() != nil {
		t.Error("PrimaryKey() should return nil for nil entity")
	}

	ctx2 := RenderContext{
		Entity: &ir.IREntity{
			Fields: []ir.IRField{
				{Name: "id", IsPrimary: true},
				{Name: "name"},
			},
		},
	}
	pk := ctx2.PrimaryKey()
	if pk == nil || pk.Name != "id" {
		t.Errorf("PrimaryKey() = %v, want field 'id'", pk)
	}
}
