package lexer

import (
	"testing"
)

func TestParsePrimitiveTypes(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantErr  bool
	}{
		{"string", "string", false},
		{"int", "int", false},
		{"uuid", "uuid", false},
		{"boolean", "boolean", false},
		{"datetime", "datetime", false},
		{"decimal", "decimal", false},
		{"text", "text", false},
		{"json", "json", false},
		{"unknown_type", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			fd, err := ParseFieldString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFieldString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if fd != nil && fd.Type != tt.wantType {
				t.Errorf("ParseFieldString() got type %v, want %v", fd.Type, tt.wantType)
			}
		})
	}
}

func TestParseRelationType(t *testing.T) {
	tests := []struct {
		input       string
		wantType    string
		wantTarget  string
		wantRelType string
	}{
		{"relation(User)", "relation", "User", "many-to-one"},
		{"relation(Category) [unique]", "relation", "Category", "one-to-one"},
		{"relation(Tag) [many]", "relation", "Tag", "many-to-many"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			fd, err := ParseFieldString(tt.input)
			if err != nil {
				t.Errorf("ParseFieldString() error = %v", err)
				return
			}
			if fd.Type != tt.wantType {
				t.Errorf("got type %v, want %v", fd.Type, tt.wantType)
			}
			if fd.TargetEntity != tt.wantTarget {
				t.Errorf("got target %v, want %v", fd.TargetEntity, tt.wantTarget)
			}
			if fd.RelationType != tt.wantRelType {
				t.Errorf("got relType %v, want %v", fd.RelationType, tt.wantRelType)
			}
		})
	}
}

func TestParseModifiers(t *testing.T) {
	tests := []struct {
		input        string
		wantPrimary  bool
		wantOptional bool
		wantUnique   bool
		wantHidden   bool
	}{
		{"string [primary]", true, false, false, false},
		{"string [optional]", false, true, false, false},
		{"string [required, unique]", false, false, true, false},
		{"string [hidden, optional]", false, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			fd, err := ParseFieldString(tt.input)
			if err != nil {
				t.Errorf("ParseFieldString() error = %v", err)
				return
			}
			if fd.IsPrimary != tt.wantPrimary {
				t.Errorf("got IsPrimary %v, want %v", fd.IsPrimary, tt.wantPrimary)
			}
			if fd.IsOptional != tt.wantOptional {
				t.Errorf("got IsOptional %v, want %v", fd.IsOptional, tt.wantOptional)
			}
			if fd.IsUnique != tt.wantUnique {
				t.Errorf("got IsUnique %v, want %v", fd.IsUnique, tt.wantUnique)
			}
			if fd.IsHidden != tt.wantHidden {
				t.Errorf("got IsHidden %v, want %v", fd.IsHidden, tt.wantHidden)
			}
		})
	}
}

func TestParseValidations(t *testing.T) {
	fd, err := ParseFieldString("string [min:5, max:100, email]")
	if err != nil {
		t.Fatalf("ParseFieldString() error = %v", err)
	}

	if fd.Validations["min"] != "5" {
		t.Errorf("got min %v, want 5", fd.Validations["min"])
	}
	if fd.Validations["max"] != "100" {
		t.Errorf("got max %v, want 100", fd.Validations["max"])
	}
	if fd.Validations["email"] != "true" {
		t.Errorf("got email %v, want true", fd.Validations["email"])
	}
}

func TestParseDefaults(t *testing.T) {
	tests := []struct {
		input           string
		wantDefault     string
		wantDefaultFunc bool
	}{
		{"boolean [default:false]", "false", false},
		{"datetime [default:now()]", "now", true},
		{"string [default:\"Unknown\"]", "Unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			fd, err := ParseFieldString(tt.input)
			if err != nil {
				t.Errorf("ParseFieldString() error = %v", err)
				return
			}
			if fd.DefaultValue != tt.wantDefault {
				t.Errorf("got default %v, want %v", fd.DefaultValue, tt.wantDefault)
			}
			if fd.DefaultIsFunc != tt.wantDefaultFunc {
				t.Errorf("got defaultIsFunc %v, want %v", fd.DefaultIsFunc, tt.wantDefaultFunc)
			}
		})
	}
}

func TestParseOnDelete(t *testing.T) {
	tests := []struct {
		input        string
		wantBehavior string
		wantErr      bool
	}{
		{"relation(Category) [on_delete:cascade]", "cascade", false},
		{"relation(Category) [on_delete:restrict]", "restrict", false},
		{"relation(Category) [optional, on_delete:set_null]", "set_null", false},
		{"relation(Category) [on_delete:invalid]", "", true},
		{"relation(Category) [on_delete:set_null]", "", true}, // should error -- field is not optional
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			fd, err := ParseFieldString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFieldString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if fd != nil && fd.OnDelete != tt.wantBehavior {
				t.Errorf("got onDelete %v, want %v", fd.OnDelete, tt.wantBehavior)
			}
		})
	}
}

func TestParseArrayType(t *testing.T) {
	fd, err := ParseFieldString("array(int)")
	if err != nil {
		t.Fatalf("ParseFieldString() error = %v", err)
	}

	if fd.Type != "array" {
		t.Errorf("got type %v, want array", fd.Type)
	}
	if fd.TargetType != "int" {
		t.Errorf("got targetType %v, want int", fd.TargetType)
	}
}

func TestParseEnumType(t *testing.T) {
	fd, err := ParseFieldString("enum(Status)")
	if err != nil {
		t.Fatalf("ParseFieldString() error = %v", err)
	}

	if fd.Type != "enum" {
		t.Errorf("got type %v, want enum", fd.Type)
	}
	if fd.TargetType != "Status" {
		t.Errorf("got targetType %v, want Status", fd.TargetType)
	}
}

func TestParseComplexField(t *testing.T) {
	// Complex case: title: string [required, min:5, max:120]
	fd, err := ParseFieldString("string [required, min:5, max:120]")
	if err != nil {
		t.Fatalf("ParseFieldString() error = %v", err)
	}

	if fd.Type != "string" {
		t.Errorf("got type %v, want string", fd.Type)
	}
	if !fd.IsRequired {
		t.Errorf("got IsRequired %v, want true", fd.IsRequired)
	}
	if fd.Validations["min"] != "5" {
		t.Errorf("got min %v, want 5", fd.Validations["min"])
	}
	if fd.Validations["max"] != "120" {
		t.Errorf("got max %v, want 120", fd.Validations["max"])
	}
}

func TestParseFieldsMap(t *testing.T) {
	fieldsMap := map[string]string{
		"id":    "uuid [primary]",
		"email": "string [required, unique, email]",
		"name":  "string [required, min:2, max:50]",
	}

	fields, err := ParseFieldsMap(fieldsMap)
	if err != nil {
		t.Fatalf("ParseFieldsMap() error = %v", err)
	}

	if len(fields) != 3 {
		t.Errorf("got %d fields, want 3", len(fields))
	}

	if fields["id"].Type != "uuid" || !fields["id"].IsPrimary {
		t.Errorf("invalid id field")
	}

	if fields["email"].Type != "string" || !fields["email"].IsRequired {
		t.Errorf("invalid email field")
	}
}
