package validator

import (
	"strings"
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/internal/testutil"
)

func TestValidateDetectsMissingPrimaryKey(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"email"},
				Fields: map[string]*parser.ParsedField{
					"email": mustParsedField(t, "email", "string"),
				},
			},
		},
	}

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	if errs[0].Message != "entity must have at least one primary key" {
		t.Fatalf("unexpected error: %s", errs[0].Error())
	}
}

func TestValidateDetectsDuplicatePrimaryKey(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "code"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"code": mustParsedField(t, "code", "string [primary]"),
				},
			},
		},
	}

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "entity has 2 primary keys") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about duplicate primary keys, got %v", errs)
	}
}

func TestValidateDetectsBrokenRelation(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "categoryId"},
				Fields: map[string]*parser.ParsedField{
					"id":         mustParsedField(t, "id", "uuid"),
					"categoryId": mustParsedField(t, "categoryId", "relation(Category)"),
				},
			},
		},
	}
	product := schema.Entities["Product"]
	product.Fields["id"].IsPrimary = true
	product.Fields["categoryId"].IsRelation = true
	product.Fields["categoryId"].RelationTarget = "Category"

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
}

func TestValidateDetectsSetNullOnRequiredField(t *testing.T) {
	// Note: on_delete:set_null on a required relation is caught by the lexer, not the validator.
	// This test verifies the schema passes validation when the lexer constraint is respected.
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product", "Category"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "categoryId"},
				Fields: map[string]*parser.ParsedField{
					"id":         mustParsedField(t, "id", "uuid [primary]"),
					"categoryId": mustParsedField(t, "categoryId", "relation(Category) [optional]"),
				},
			},
			"Category": {
				Name:       "Category",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	product := schema.Entities["Product"]
	product.Fields["id"].IsPrimary = true
	product.Fields["categoryId"].IsRelation = true
	product.Fields["categoryId"].RelationTarget = "Category"
	product.Fields["categoryId"].OnDelete = "set_null"
	product.Fields["categoryId"].IsOptional = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid set_null on optional field, got %v", errs)
	}
}

func TestValidateDetectsUndefinedEnum(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product"},
		Enums:       map[string][]string{"ProductStatus": {"DRAFT", "PUBLISHED"}},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "status"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"status": mustParsedField(t, "status", "enum(NoSuchEnum)"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if e.Field == "status" && strings.Contains(e.Message, "NoSuchEnum") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about undefined enum, got %v", errs)
	}
}

func TestValidatePassesValidEnums(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product"},
		Enums:       map[string][]string{"ProductStatus": {"DRAFT", "PUBLISHED"}},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "status", "tags"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"status": mustParsedField(t, "status", "enum(ProductStatus)"),
					"tags":   mustParsedField(t, "tags", "array(ProductStatus)"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid enums, got %v", errs)
	}
}

func TestValidateDetectsEmptyProjectName(t *testing.T) {
	schema := &parser.ParsedSchema{
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "project name must not be empty") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about empty project name, got %v", errs)
	}
}

func TestValidateDetectsManyOnNonRelation(t *testing.T) {
	// Note: "many" on non-relation is caught by the lexer, not the validator.
	// This test verifies the lexer catches it before validation.
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	// Valid schema should pass — the lexer rejects "many" on non-relation at parse time.
	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateDetectsOnDeleteOnNonRelation(t *testing.T) {
	// Note: on_delete on non-relation is caught by the lexer, not the validator.
	// This test verifies the lexer catches it before validation.
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	// Valid schema should pass — the lexer rejects on_delete on non-relation at parse time.
	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateDetectsEmptyEnum(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Enums:       map[string][]string{"Status": {}},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "enum must have at least one value") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about empty enum, got %v", errs)
	}
}

func TestValidateDetectsEmptyIndexFields(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
				Indexes: []*parser.ParsedIndex{
					{Fields: []string{}, Type: "btree"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "index 0 has no fields") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about empty index, got %v", errs)
	}
}

func TestValidateDetectsSortLengthMismatch(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "email"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"email": mustParsedField(t, "email", "string"),
				},
				Indexes: []*parser.ParsedIndex{
					{Fields: []string{"id", "email"}, Sort: []string{"asc"}, Type: "btree"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "sort array length") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about sort length mismatch, got %v", errs)
	}
}

func mustParsedField(t *testing.T, name, input string) *parser.ParsedField {
	return testutil.MustParsedField(t, name, input)
}

func nonWarnings(errs []ValidationError) []ValidationError {
	var result []ValidationError
	for _, e := range errs {
		if !e.Warning {
			result = append(result, e)
		}
	}
	return result
}

func TestValidateAuthEntityMissingEmailPassword(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Auth:        &parser.AuthConfig{Type: "jwt", Entity: "Product"},
		EntityOrder: []string{"Product"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "name"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"name": mustParsedField(t, "name", "string"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "must have both 'email' and 'password'") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about missing email+password, got %v", errs)
	}
}

func TestValidateAuthEntityValidEmailPassword(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Auth:        &parser.AuthConfig{Type: "jwt", Entity: "User"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "email", "password"},
				Fields: map[string]*parser.ParsedField{
					"id":       mustParsedField(t, "id", "uuid [primary]"),
					"email":    mustParsedField(t, "email", "string"),
					"password": mustParsedField(t, "password", "string"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "email") || strings.Contains(e.Message, "password") {
			t.Fatalf("unexpected error: %v", e.Message)
		}
	}
}

func TestValidateDefaultNowOnNonDatetime(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "count"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"count": mustParsedField(t, "count", "int [default:now()]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "default 'now()' is not valid for type int") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about now() on int, got %v", errs)
	}
}

func TestValidateDefaultNowOnDatetime(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "created"},
				Fields: map[string]*parser.ParsedField{
					"id":      mustParsedField(t, "id", "uuid [primary]"),
					"created": mustParsedField(t, "created", "datetime [default:now()]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "now()") {
			t.Fatalf("unexpected error about now() on datetime: %v", e.Message)
		}
	}
}

func TestValidateDefaultUuidOnNonUuid(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "name"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"name": mustParsedField(t, "name", "string [default:uuid()]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "default 'uuid()' is not valid for type string") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about uuid() on string, got %v", errs)
	}
}

func TestValidateDeployPortRange(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"valid", 8080, false},
		{"zero (default)", 0, false},
		{"min valid", 1, false},
		{"max valid", 65535, false},
		{"below min", -1, true},
		{"above max", 99999, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &parser.ParsedSchema{
				Project: parser.ProjectConfig{
					Name:   "Test",
					Deploy: &parser.DeployConfig{Port: tt.port},
				},
				EntityOrder: []string{"User"},
				Entities: map[string]*parser.ParsedEntity{
					"User": {
						Name:       "User",
						FieldOrder: []string{"id"},
						Fields: map[string]*parser.ParsedField{
							"id": mustParsedField(t, "id", "uuid [primary]"),
						},
					},
				},
			}
			schema.Entities["User"].Fields["id"].IsPrimary = true

			errs := New(schema).Validate()
			errs = nonWarnings(errs)
			found := false
			for _, e := range errs {
				if strings.Contains(e.Message, "deploy.port") {
					found = true
				}
			}
			if tt.wantErr && !found {
				t.Fatalf("expected port range error, got %v", errs)
			}
			if !tt.wantErr && found {
				t.Fatalf("unexpected port range error, got %v", errs)
			}
		})
	}
}

func TestValidateCacheConfigWarnings(t *testing.T) {
	t.Run("provider without connection_string", func(t *testing.T) {
		schema := &parser.ParsedSchema{
			Project: parser.ProjectConfig{
				Name: "Test",
				Cache: &parser.CacheConfig{
					Enabled:  true,
					Provider: "redis",
				},
			},
			EntityOrder: []string{"User"},
			Entities: map[string]*parser.ParsedEntity{
				"User": {
					Name:       "User",
					FieldOrder: []string{"id"},
					Fields: map[string]*parser.ParsedField{
						"id": mustParsedField(t, "id", "uuid [primary]"),
					},
				},
			},
		}
		schema.Entities["User"].Fields["id"].IsPrimary = true

		errs := New(schema).Validate()
		found := false
		for _, e := range errs {
			if e.Warning && strings.Contains(e.Message, "connection_string is empty") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected warning about empty connection_string, got %v", errs)
		}
	})

	t.Run("provider specified but disabled", func(t *testing.T) {
		schema := &parser.ParsedSchema{
			Project: parser.ProjectConfig{
				Name: "Test",
				Cache: &parser.CacheConfig{
					Enabled:  false,
					Provider: "redis",
				},
			},
			EntityOrder: []string{"User"},
			Entities: map[string]*parser.ParsedEntity{
				"User": {
					Name:       "User",
					FieldOrder: []string{"id"},
					Fields: map[string]*parser.ParsedField{
						"id": mustParsedField(t, "id", "uuid [primary]"),
					},
				},
			},
		}
		schema.Entities["User"].Fields["id"].IsPrimary = true

		errs := New(schema).Validate()
		found := false
		for _, e := range errs {
			if e.Warning && strings.Contains(e.Message, "cache.enabled is false") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected warning about cache disabled, got %v", errs)
		}
	})
}

func TestValidateIndexOnRelationField(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product", "Category"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "categoryId"},
				Fields: map[string]*parser.ParsedField{
					"id":         mustParsedField(t, "id", "uuid [primary]"),
					"categoryId": mustParsedField(t, "categoryId", "relation(Category)"),
				},
				Indexes: []*parser.ParsedIndex{
					{Fields: []string{"categoryId"}, Type: "btree"},
				},
			},
			"Category": {
				Name:       "Category",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true
	schema.Entities["Product"].Fields["categoryId"].IsRelation = true
	schema.Entities["Product"].Fields["categoryId"].RelationTarget = "Category"

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if e.Warning && strings.Contains(e.Message, "references relation field") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning about index on relation field, got %v", errs)
	}
}

func TestValidateIndexOnPrimitiveField(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "email"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"email": mustParsedField(t, "email", "string"),
				},
				Indexes: []*parser.ParsedIndex{
					{Fields: []string{"email"}, Type: "btree"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "references relation field") {
			t.Fatalf("unexpected warning about index on primitive field: %v", e.Message)
		}
	}
}

func TestValidateEnumDefaultMustMatchDeclaredValues(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product"},
		Enums:       map[string][]string{"ProductStatus": {"DRAFT", "PUBLISHED"}},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "status"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"status": mustParsedField(t, "status", "enum(ProductStatus) [default:DRAFTT]"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "not a declared value of enum") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about invalid enum default, got %v", errs)
	}
}

func TestValidateEnumDefaultValidValue(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product"},
		Enums:       map[string][]string{"ProductStatus": {"DRAFT", "PUBLISHED"}},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "status"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"status": mustParsedField(t, "status", "enum(ProductStatus) [default:DRAFT]"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "not a declared value of enum") {
			t.Fatalf("unexpected error about valid enum default: %v", e.Message)
		}
	}
}

func TestValidateNumericValidationValueNotNumeric(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "age"},
				Fields: map[string]*parser.ParsedField{
					"id":  mustParsedField(t, "id", "uuid [primary]"),
					"age": mustParsedField(t, "age", "int [gte:notanumber]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "not a valid number") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about non-numeric validation value, got %v", errs)
	}
}

func TestValidateNumericValidationValueValid(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "age"},
				Fields: map[string]*parser.ParsedField{
					"id":  mustParsedField(t, "id", "uuid [primary]"),
					"age": mustParsedField(t, "age", "int [gte:0, lt:150]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "not a valid number") {
			t.Fatalf("unexpected error about valid numeric validation value: %v", e.Message)
		}
	}
}

func TestValidateEntityEnumNameCollision(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Status"},
		Enums:       map[string][]string{"Status": {"ACTIVE", "INACTIVE"}},
		Entities: map[string]*parser.ParsedEntity{
			"Status": {
				Name:       "Status",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["Status"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if e.Warning && strings.Contains(e.Message, "collides with an enum") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning about entity-enum name collision, got %v", errs)
	}
}

func TestValidatePermissionRolesEmptyAuthRoles(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Auth:        &parser.AuthConfig{Type: "jwt"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "email", "password"},
				Fields: map[string]*parser.ParsedField{
					"id":       mustParsedField(t, "id", "uuid [primary]"),
					"email":    mustParsedField(t, "email", "string"),
					"password": mustParsedField(t, "password", "string"),
				},
				Permissions: &parser.ParsedPermissions{
					Read: []string{"Admin"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if e.Warning && strings.Contains(e.Message, "auth.roles is empty") && strings.Contains(e.Message, "Admin") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning about permission role with empty auth.roles, got %v", errs)
	}
}

func TestValidateContradictoryStringMinMax(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "name"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"name": mustParsedField(t, "name", "string [min:10, max:5]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "min") && strings.Contains(e.Message, "max") && strings.Contains(e.Message, "greater") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about contradictory min>max, got %v", errs)
	}
}

func TestValidateValidStringMinMax(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "name"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"name": mustParsedField(t, "name", "string [min:2, max:50]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "min") && strings.Contains(e.Message, "max") {
			t.Fatalf("unexpected contradictory range error: %v", e.Message)
		}
	}
}

func TestValidateContradictoryNumericGteLt(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "age"},
				Fields: map[string]*parser.ParsedField{
					"id":  mustParsedField(t, "id", "uuid [primary]"),
					"age": mustParsedField(t, "age", "int [gte:100, lt:10]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "gte") && strings.Contains(e.Message, "lt") && strings.Contains(e.Message, "must be less") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about contradictory gte>=lt, got %v", errs)
	}
}

func TestValidateContradictoryNumericGtLte(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "score"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"score": mustParsedField(t, "score", "int [gt:50, lte:50]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "gt") && strings.Contains(e.Message, "lte") && strings.Contains(e.Message, "must be less") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about contradictory gt>=lte, got %v", errs)
	}
}

func TestValidateValidNumericRange(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "age"},
				Fields: map[string]*parser.ParsedField{
					"id":  mustParsedField(t, "id", "uuid [primary]"),
					"age": mustParsedField(t, "age", "int [gte:0, lt:150]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "gte") && strings.Contains(e.Message, "lt") {
			t.Fatalf("unexpected contradictory range error: %v", e.Message)
		}
	}
}

func TestValidateUnusedEnumWarning(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Enums:       map[string][]string{"Role": {"ADMIN", "USER"}, "Status": {"ACTIVE", "INACTIVE"}},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "role"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"role": mustParsedField(t, "role", "enum(Role)"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true
	schema.Entities["User"].Fields["role"].Type = "enum"
	schema.Entities["User"].Fields["role"].TargetType = "Role"

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "enum is declared but never used") && e.Field == "Status" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning about unused enum 'Status', got %v", errs)
	}
}

func TestValidateUsedEnumNoWarning(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Enums:       map[string][]string{"Role": {"ADMIN", "USER"}},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "role"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"role": mustParsedField(t, "role", "enum(Role)"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true
	schema.Entities["User"].Fields["role"].Type = "enum"
	schema.Entities["User"].Fields["role"].TargetType = "Role"

	errs := New(schema).Validate()
	for _, e := range errs {
		if strings.Contains(e.Message, "enum is declared but never used") {
			t.Fatalf("unexpected warning about used enum: %v", e.Message)
		}
	}
}

func TestValidateOnDeleteValueInvalid(t *testing.T) {
	// Note: invalid on_delete values are caught by the lexer, not the validator.
	// This test verifies the lexer catches it before validation.
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Order", "User"},
		Entities: map[string]*parser.ParsedEntity{
			"Order": {
				Name:       "Order",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["Order"].Fields["id"].IsPrimary = true
	schema.Entities["User"].Fields["id"].IsPrimary = true

	// Valid schema should pass — the lexer rejects invalid on_delete at parse time.
	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateSeedValueTypeMismatch(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "age"},
				Fields: map[string]*parser.ParsedField{
					"id":  mustParsedField(t, "id", "uuid [primary]"),
					"age": mustParsedField(t, "age", "int"),
				},
				Seed: []map[string]interface{}{
					{"id": "550e8400-e29b-41d4-a716-446655440000", "age": true},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "seed entry") && strings.Contains(e.Message, "not compatible with type int") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about seed type mismatch, got %v", errs)
	}
}

func TestValidateSeedBooleanTypeMismatch(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Config"},
		Entities: map[string]*parser.ParsedEntity{
			"Config": {
				Name:       "Config",
				FieldOrder: []string{"id", "enabled"},
				Fields: map[string]*parser.ParsedField{
					"id":      mustParsedField(t, "id", "uuid [primary]"),
					"enabled": mustParsedField(t, "enabled", "boolean"),
				},
				Seed: []map[string]interface{}{
					{"id": "550e8400-e29b-41d4-a716-446655440000", "enabled": "notabool"},
				},
			},
		},
	}
	schema.Entities["Config"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "seed entry") && strings.Contains(e.Message, "not valid for boolean") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about seed boolean type mismatch, got %v", errs)
	}
}

func TestValidateOnDeleteOnManyToManyWarning(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product", "Tag"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "tags"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"tags": mustParsedField(t, "tags", "relation(Tag) [many]"),
				},
			},
			"Tag": {
				Name:       "Tag",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true
	schema.Entities["Product"].Fields["tags"].IsRelation = true
	schema.Entities["Product"].Fields["tags"].RelationTarget = "Tag"
	schema.Entities["Product"].Fields["tags"].IsMany = true
	schema.Entities["Product"].Fields["tags"].OnDelete = "cascade"

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if e.Warning && strings.Contains(e.Message, "on_delete is meaningless on a many-to-many relation") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning about on_delete on many-to-many, got %v", errs)
	}
}

func TestValidateOnDeleteOnManyToManyNoWarningWithoutOnDelete(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product", "Tag"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "tags"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"tags": mustParsedField(t, "tags", "relation(Tag) [many]"),
				},
			},
			"Tag": {
				Name:       "Tag",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true
	schema.Entities["Product"].Fields["tags"].IsRelation = true
	schema.Entities["Product"].Fields["tags"].RelationTarget = "Tag"
	schema.Entities["Product"].Fields["tags"].IsMany = true

	errs := New(schema).Validate()
	for _, e := range errs {
		if strings.Contains(e.Message, "on_delete is meaningless on a many-to-many") {
			t.Fatalf("unexpected warning about on_delete on many-to-many when no on_delete set: %v", e.Message)
		}
	}
}

func TestValidateSeedUniqueConstraintViolation(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "email"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"email": mustParsedField(t, "email", "string [unique]"),
				},
				Seed: []map[string]interface{}{
					{"id": "11111111-1111-1111-1111-111111111111", "email": "a@test.com"},
					{"id": "22222222-2222-2222-2222-222222222222", "email": "a@test.com"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "duplicated in seed data") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about seed unique constraint violation, got %v", errs)
	}
}

func TestValidateSeedUniqueConstraintPass(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "email"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"email": mustParsedField(t, "email", "string [unique]"),
				},
				Seed: []map[string]interface{}{
					{"id": "11111111-1111-1111-1111-111111111111", "email": "a@test.com"},
					{"id": "22222222-2222-2222-2222-222222222222", "email": "b@test.com"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "duplicated in seed data") {
			t.Fatalf("unexpected seed unique constraint error: %v", e.Message)
		}
	}
}

func TestValidateSeedInvalidUUID(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
				Seed: []map[string]interface{}{
					{"id": "not-a-uuid"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "not a valid UUID") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about invalid UUID in seed, got %v", errs)
	}
}

func TestValidateSeedValidUUID(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
				Seed: []map[string]interface{}{
					{"id": "550e8400-e29b-41d4-a716-446655440000"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "not a valid UUID") {
			t.Fatalf("unexpected UUID error: %v", e.Message)
		}
	}
}

func TestValidateSeedInvalidDatetime(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "createdAt"},
				Fields: map[string]*parser.ParsedField{
					"id":        mustParsedField(t, "id", "uuid [primary]"),
					"createdAt": mustParsedField(t, "createdAt", "datetime"),
				},
				Seed: []map[string]interface{}{
					{"id": "550e8400-e29b-41d4-a716-446655440000", "createdAt": "not-a-date"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "not a valid datetime") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about invalid datetime in seed, got %v", errs)
	}
}

func TestValidateSeedValidDatetime(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "createdAt"},
				Fields: map[string]*parser.ParsedField{
					"id":        mustParsedField(t, "id", "uuid [primary]"),
					"createdAt": mustParsedField(t, "createdAt", "datetime"),
				},
				Seed: []map[string]interface{}{
					{"id": "550e8400-e29b-41d4-a716-446655440000", "createdAt": "2024-01-15T10:30:00Z"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "not a valid datetime") {
			t.Fatalf("unexpected datetime error: %v", e.Message)
		}
	}
}

func TestValidateSeedInvalidJSON(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "metadata"},
				Fields: map[string]*parser.ParsedField{
					"id":       mustParsedField(t, "id", "uuid [primary]"),
					"metadata": mustParsedField(t, "metadata", "json"),
				},
				Seed: []map[string]interface{}{
					{"id": "550e8400-e29b-41d4-a716-446655440000", "metadata": "{invalid json}"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "not valid JSON") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about invalid JSON in seed, got %v", errs)
	}
}

func TestValidateSeedValidJSON(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "metadata"},
				Fields: map[string]*parser.ParsedField{
					"id":       mustParsedField(t, "id", "uuid [primary]"),
					"metadata": mustParsedField(t, "metadata", "json"),
				},
				Seed: []map[string]interface{}{
					{"id": "550e8400-e29b-41d4-a716-446655440000", "metadata": `{"key":"value"}`},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	for _, e := range errs {
		if strings.Contains(e.Message, "not valid JSON") {
			t.Fatalf("unexpected JSON error: %v", e.Message)
		}
	}
}

func TestValidatePermissionRoleIdentifierInvalid(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
				Permissions: &parser.ParsedPermissions{
					Read: []string{"valid-role", "123invalid"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if e.Warning && strings.Contains(e.Message, "not a valid identifier") && strings.Contains(e.Message, "123invalid") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning about invalid permission role identifier, got %v", errs)
	}
}

func TestValidatePermissionRoleSpecialTokensAllowed(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
				Permissions: &parser.ParsedPermissions{
					Read:   []string{"*", "@Owner"},
					Create: []string{"Admin"},
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	errs := New(schema).Validate()
	for _, e := range errs {
		if e.Warning && strings.Contains(e.Message, "not a valid identifier") {
			t.Fatalf("unexpected warning about special token in permissions: %v", e.Message)
		}
	}
}
