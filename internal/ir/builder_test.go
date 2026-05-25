package ir

import (
	"testing"

	"domaincraft/internal/parser"
	"domaincraft/internal/testutil"
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

func TestBuildResolvesEnumTypes(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"Product"},
		Enums:       map[string][]string{"ProductStatus": {"DRAFT", "PUBLISHED"}, "Tag": {"A", "B"}},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				NamePlural: "Products",
				FieldOrder: []string{"id", "status", "tags"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"status": mustParsedField(t, "status", "enum(ProductStatus)"),
					"tags":   mustParsedField(t, "tags", "array(Tag)"),
				},
			},
		},
	}
	schema.Entities["Product"].Fields["id"].IsPrimary = true

	projectIR, err := NewBuilder().Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	product := projectIR.Entities[0]

	// id should be "uuid"
	if id := product.Fields[0]; id.DatabaseType != "uuid" {
		t.Errorf("id.DatabaseType = %q, want %q", id.DatabaseType, "uuid")
	}

	// enum should store raw name
	if status := product.Fields[1]; status.DatabaseType != "ProductStatus" {
		t.Errorf("status.DatabaseType = %q, want %q", status.DatabaseType, "ProductStatus")
	}

	// array(enum) should store raw enum name
	if tags := product.Fields[2]; tags.DatabaseType != "array(Tag)" {
		t.Errorf("tags.DatabaseType = %q, want %q", tags.DatabaseType, "array(Tag)")
	}
}

func TestBuildResolvesPrimitiveTypes(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				NamePlural: "Users",
				FieldOrder: []string{"id", "name", "count", "price", "active", "born", "created", "data", "items"},
				Fields: map[string]*parser.ParsedField{
					"id":      mustParsedField(t, "id", "uuid [primary]"),
					"name":    mustParsedField(t, "name", "string"),
					"count":   mustParsedField(t, "count", "bigint"),
					"price":   mustParsedField(t, "price", "decimal"),
					"active":  mustParsedField(t, "active", "boolean"),
					"born":    mustParsedField(t, "born", "date"),
					"created": mustParsedField(t, "created", "datetime"),
					"data":    mustParsedField(t, "data", "jsonb"),
					"items":   mustParsedField(t, "items", "array(int)"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	projectIR, err := NewBuilder().Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	user := projectIR.Entities[0]

	expected := map[string]string{
		"id":      "uuid",
		"name":    "string",
		"count":   "bigint",
		"price":   "decimal",
		"active":  "boolean",
		"born":    "date",
		"created": "datetime",
		"data":    "jsonb",
		"items":   "array(int)",
	}

	for _, field := range user.Fields {
		want, ok := expected[field.Name]
		if !ok {
			continue
		}
		if field.DatabaseType != want {
			t.Errorf("%s.DatabaseType = %q, want %q", field.Name, field.DatabaseType, want)
		}
	}
}

func mustParsedField(t *testing.T, name, input string) *parser.ParsedField {
	return testutil.MustParsedField(t, name, input)
}
