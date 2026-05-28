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
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"Product", "Category"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				FieldOrder: []string{"id", "categoryId"},
				Fields: map[string]*parser.ParsedField{
					"id":         mustParsedField(t, "id", "uuid"),
					"categoryId": mustParsedField(t, "categoryId", "relation(Category)"),
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
	product.Fields["categoryId"].IsOptional = false

	errs := New(schema).Validate()
	errs = nonWarnings(errs)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
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
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "tags"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"tags": mustParsedField(t, "tags", "string"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true
	schema.Entities["User"].Fields["tags"].IsMany = true

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "many modifier is only valid on relation fields") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about many on non-relation, got %v", errs)
	}
}

func TestValidateDetectsOnDeleteOnNonRelation(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				FieldOrder: []string{"id", "name"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"name": mustParsedField(t, "name", "string"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true
	schema.Entities["User"].Fields["name"].OnDelete = "cascade"

	errs := New(schema).Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "on_delete is only valid on relation fields") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error about on_delete on non-relation, got %v", errs)
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
