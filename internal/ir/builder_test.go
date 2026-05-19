package ir

import (
	"testing"

	"domaincraft/internal/lexer"
	"domaincraft/internal/parser"
)

func TestBuildCreatesRelations(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"Product", "Category"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				NamePlural: "Products",
				FieldOrder: []string{"id", "categoryId"},
				Fields: map[string]*parser.ParsedField{
					"id":         mustParsedField(t, "id", "uuid [primary]"),
					"categoryId": mustParsedField(t, "categoryId", "relation(Category)"),
				},
			},
			"Category": {
				Name:       "Category",
				NamePlural: "Categories",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["categoryId"].IsRelation = true
	schema.Entities["Product"].Fields["categoryId"].RelationTarget = "Category"

	projectIR, err := NewBuilder().Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(projectIR.Entities) != 2 {
		t.Fatalf("got %d entities, want 2", len(projectIR.Entities))
	}

	productIR := projectIR.Entities[0]
	if len(productIR.RelationsOut) != 1 {
		t.Fatalf("expected one outgoing relation")
	}
	if productIR.RelationsOut[0].NavigationName == "" {
		t.Fatalf("navigation name must not be empty")
	}
	if len(projectIR.Entities[1].RelationsIn) != 1 {
		t.Fatalf("expected one incoming relation on Category")
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
