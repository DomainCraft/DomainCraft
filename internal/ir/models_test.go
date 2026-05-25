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
