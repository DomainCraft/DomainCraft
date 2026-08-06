package lexer

import (
	"strings"
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
		input      string
		wantType   string
		wantTarget string
		wantIsMany bool
	}{
		{"relation(User)", "relation", "User", false},
		{"relation(Category) [unique]", "relation", "Category", false},
		{"relation(Tag) [many]", "relation", "Tag", true},
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
			if fd.IsMany != tt.wantIsMany {
				t.Errorf("got IsMany %v, want %v", fd.IsMany, tt.wantIsMany)
			}
		})
	}
}

func TestParseModifiers(t *testing.T) {
	tests := []struct {
		input         string
		wantPrimary   bool
		wantOptional  bool
		wantUnique    bool
		wantHidden    bool
		wantReadonly  bool
	}{
		{"string [primary]", true, false, false, false, false},
		{"string [optional]", false, true, false, false, false},
		{"string [required, unique]", false, false, true, false, false},
		{"string [hidden, optional]", false, true, false, true, false},
		{"decimal [required, readonly]", false, false, false, false, true},
		{"string [hidden, readonly]", false, false, false, true, true},
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
			if fd.IsReadonly != tt.wantReadonly {
				t.Errorf("got IsReadonly %v, want %v", fd.IsReadonly, tt.wantReadonly)
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

func TestParseOldName(t *testing.T) {
	fd, err := ParseFieldString("string [required, old_name: title]")
	if err != nil {
		t.Fatalf("ParseFieldString() error = %v", err)
	}
	if fd.OldName != "title" {
		t.Errorf("got OldName %q, want title", fd.OldName)
	}
	if !fd.IsRequired {
		t.Errorf("got IsRequired %v, want true", fd.IsRequired)
	}

	// Quoted values are accepted too.
	fd, err = ParseFieldString("text [optional, old_name: \"summary\"]")
	if err != nil {
		t.Fatalf("ParseFieldString() quoted error = %v", err)
	}
	if fd.OldName != "summary" {
		t.Errorf("got OldName %q, want summary", fd.OldName)
	}
}

func TestParseOldNameInvalid(t *testing.T) {
	tests := []string{
		"string [old_name:]",
		"string [old_name: not valid name]",
		"string [old_name: myField-1]",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseFieldString(input); err == nil {
				t.Errorf("ParseFieldString(%q) expected an error", input)
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

func TestParseRelationEmptyTarget(t *testing.T) {
	_, err := ParseFieldString("relation()")
	if err == nil {
		t.Fatal("expected error for empty relation target")
	}
	if !strings.Contains(err.Error(), "requires a target entity name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseArrayEmptyType(t *testing.T) {
	_, err := ParseFieldString("array()")
	if err == nil {
		t.Fatal("expected error for empty array type")
	}
	if !strings.Contains(err.Error(), "requires an element type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseEnumEmptyName(t *testing.T) {
	_, err := ParseFieldString("enum()")
	if err == nil {
		t.Fatal("expected error for empty enum name")
	}
	if !strings.Contains(err.Error(), "requires a name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParsePrimaryKeyIsRequired(t *testing.T) {
	// The lexer no longer normalizes primary→required; that's the parser's job.
	// The lexer only parses syntax; semantic normalization happens in parser.parseField.
	fd, err := ParseFieldString("uuid [primary]")
	if err != nil {
		t.Fatalf("ParseFieldString() error = %v", err)
	}
	if !fd.IsPrimary {
		t.Error("expected IsPrimary=true")
	}
	// IsRequired is NOT set by the lexer — parser handles this.
	if fd.IsRequired {
		t.Error("lexer should not set IsRequired for primary (parser handles normalization)")
	}
}

func TestParsePrimaryKeyWithExplicitRequired(t *testing.T) {
	fd, err := ParseFieldString("uuid [primary, required]")
	if err != nil {
		t.Fatalf("ParseFieldString() error = %v", err)
	}
	if !fd.IsPrimary {
		t.Error("expected IsPrimary=true")
	}
	if !fd.IsRequired {
		t.Error("expected IsRequired=true for primary key with explicit required")
	}
}

func TestParseOnDeleteOnNonRelationField(t *testing.T) {
	_, err := ParseFieldString("string [on_delete:cascade]")
	if err == nil {
		t.Fatal("expected error for on_delete on non-relation field")
	}
	if !strings.Contains(err.Error(), "on_delete is only valid on relation fields") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseManyOnNonRelationField(t *testing.T) {
	_, err := ParseFieldString("string [many]")
	if err == nil {
		t.Fatal("expected error for many on non-relation field")
	}
	if !strings.Contains(err.Error(), "many modifier is only valid on relation fields") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseRequiredOptionalConflict(t *testing.T) {
	_, err := ParseFieldString("string [required, optional]")
	if err == nil {
		t.Fatal("expected error for required+optional conflict")
	}
	if !strings.Contains(err.Error(), "cannot be both required and optional") {
		t.Errorf("unexpected error: %v", err)
	}
}
