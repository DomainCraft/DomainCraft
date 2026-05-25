package specmeta

import "testing"

func TestIsPrimitive(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"string", true},
		{"text", true},
		{"int", true},
		{"bigint", true},
		{"float", true},
		{"decimal", true},
		{"boolean", true},
		{"date", true},
		{"datetime", true},
		{"uuid", true},
		{"json", true},
		{"jsonb", true},
		{"STRING", false},    // case-sensitive, callers must lowercase
		{"ProductStatus", false}, // enum
		{"enum", false},      // meta-type
		{"array", false},     // meta-type
		{"relation", false},  // meta-type
		{"", false},
	}

	for _, tt := range tests {
		got := IsPrimitive(tt.input)
		if got != tt.want {
			t.Errorf("IsPrimitive(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFieldTypesDerivedCorrectly(t *testing.T) {
	// FieldTypes is derived from PrimitiveFieldTypes + MetaFieldTypes.
	// Verify the derivation produces the expected total count.
	expected := len(PrimitiveFieldTypes) + len(MetaFieldTypes)
	if len(FieldTypes) != expected {
		t.Errorf("len(FieldTypes) = %d, want %d (= PrimitiveFieldTypes + MetaFieldTypes)", len(FieldTypes), expected)
	}

	// Verify all primitives are in FieldTypes
	primitiveSet := SliceToSet(PrimitiveFieldTypes)
	for _, ft := range FieldTypes {
		if !primitiveSet[ft] && ft != "relation" && ft != "array" && ft != "enum" {
			t.Errorf("FieldTypes contains unexpected type %q", ft)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"int", true},
		{"bigint", true},
		{"float", true},
		{"decimal", true},
		{"string", false},
		{"boolean", false},
		{"datetime", false},
		{"uuid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsNumeric(tt.input); got != tt.want {
			t.Errorf("IsNumeric(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
