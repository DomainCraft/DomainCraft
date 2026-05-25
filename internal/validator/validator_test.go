package validator

import (
	"strings"
	"testing"

	"domaincraft/internal/parser"
	"domaincraft/internal/testutil"
)

func TestValidateDetectsMissingPrimaryKey(t *testing.T) {
	schema := &parser.ParsedSchema{
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
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
	if errs[0].Message != "entity must have at least one primary key" {
		t.Fatalf("unexpected error: %s", errs[0].Error())
	}
}

func TestValidateDetectsBrokenRelation(t *testing.T) {
	schema := &parser.ParsedSchema{
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
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
}

func TestValidateDetectsSetNullOnRequiredField(t *testing.T) {
	schema := &parser.ParsedSchema{
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
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
}

func TestValidateDetectsUndefinedEnum(t *testing.T) {
	schema := &parser.ParsedSchema{
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
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid enums, got %v", errs)
	}
}

func mustParsedField(t *testing.T, name, input string) *parser.ParsedField {
	return testutil.MustParsedField(t, name, input)
}
