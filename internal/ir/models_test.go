package ir

import "testing"

func TestIRField_IsArray(t *testing.T) {
	tests := []struct {
		name         string
		databaseType string
		want         bool
	}{
		{"array of int", "array(int)", true},
		{"array of enum", "array(ProductStatus)", true},
		{"plain string", "string", false},
		{"plain int", "int", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := IRField{DatabaseType: tt.databaseType}
			if got := f.IsArray(); got != tt.want {
				t.Errorf("IsArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIRField_ArrayElementType(t *testing.T) {
	tests := []struct {
		name         string
		databaseType string
		want         string
	}{
		{"array of int", "array(int)", "int"},
		{"array of enum", "array(ProductStatus)", "ProductStatus"},
		{"not array", "string", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := IRField{DatabaseType: tt.databaseType}
			if got := f.ArrayElementType(); got != tt.want {
				t.Errorf("ArrayElementType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIREntity_HasFeature(t *testing.T) {
	tests := []struct {
		name     string
		features map[string]bool
		feature  string
		want     bool
	}{
		{"audit enabled", map[string]bool{"audit": true}, "audit", true},
		{"audit disabled", map[string]bool{"audit": false}, "audit", false},
		{"feature not in map", map[string]bool{}, "audit", false},
		{"nil features", nil, "audit", false},
		{"unknown feature", map[string]bool{"audit": true}, "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := IREntity{Features: tt.features}
			if got := e.HasFeature(tt.feature); got != tt.want {
				t.Errorf("HasFeature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestIREntity_HasAudit(t *testing.T) {
	tests := []struct {
		name     string
		features map[string]bool
		want     bool
	}{
		{"audit enabled", map[string]bool{"audit": true}, true},
		{"audit disabled", map[string]bool{"audit": false}, false},
		{"nil features", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := IREntity{Features: tt.features}
			if got := e.HasAudit(); got != tt.want {
				t.Errorf("HasAudit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIREntity_HasAuditLog(t *testing.T) {
	e := IREntity{Features: map[string]bool{"audit_log": true}}
	if !e.HasAuditLog() {
		t.Error("HasAuditLog() = false, want true")
	}
	e2 := IREntity{Features: map[string]bool{}}
	if e2.HasAuditLog() {
		t.Error("HasAuditLog() = true, want false")
	}
}

func TestIREntity_HasSoftDelete(t *testing.T) {
	e := IREntity{Features: map[string]bool{"soft_delete": true}}
	if !e.HasSoftDelete() {
		t.Error("HasSoftDelete() = false, want true")
	}
}

func TestIREntity_HasOptimisticLock(t *testing.T) {
	e := IREntity{Features: map[string]bool{"optimistic_lock": true}}
	if !e.HasOptimisticLock() {
		t.Error("HasOptimisticLock() = false, want true")
	}
}

func TestIREntity_NonRelationFields(t *testing.T) {
	e := IREntity{
		Fields: []IRField{
			{Name: "id", IsRelation: false},
			{Name: "categoryId", IsRelation: true},
			{Name: "name", IsRelation: false},
		},
	}
	result := e.NonRelationFields()
	if len(result) != 2 {
		t.Fatalf("got %d non-relation fields, want 2", len(result))
	}
	if result[0].Name != "id" || result[1].Name != "name" {
		t.Errorf("unexpected fields: %v", result)
	}
}

func TestIREntity_RelationFields(t *testing.T) {
	e := IREntity{
		Fields: []IRField{
			{Name: "id", IsRelation: false},
			{Name: "categoryId", IsRelation: true},
			{Name: "name", IsRelation: false},
		},
	}
	result := e.RelationFields()
	if len(result) != 1 {
		t.Fatalf("got %d relation fields, want 1", len(result))
	}
	if result[0].Name != "categoryId" {
		t.Errorf("unexpected field: %v", result[0].Name)
	}
}
