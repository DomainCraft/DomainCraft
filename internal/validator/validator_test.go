package validator

import (
	"testing"

	"domaincraft/internal/lexer"
	"domaincraft/internal/parser"
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

func mustParsedField(t *testing.T, name, input string) *parser.ParsedField {
	t.Helper()
	fieldDef, err := lexer.ParseFieldString(input)
	if err != nil {
		t.Fatalf("ParseFieldString(%q) error = %v", input, err)
	}
	fieldDef.Name = name
	return &parser.ParsedField{FieldDefinition: fieldDef}
}
