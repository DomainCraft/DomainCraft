package ir

import (
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/lexer"
	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/internal/testutil"
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
	schema.Entities["Product"].Fields["id"].IsPrimary = true

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(projectIR.Entities) != 2 {
		t.Fatalf("got %d entities, want 2", len(projectIR.Entities))
	}

	// After topological sort, Category (no deps) comes before Product (depends on Category).
	categoryIR := projectIR.Entities[0]
	if categoryIR.Name != "Category" {
		t.Fatalf("expected Category first, got %s", categoryIR.Name)
	}
	if len(categoryIR.RelationsIn) != 1 {
		t.Fatalf("expected one incoming relation on Category")
	}

	productIR := projectIR.Entities[1]
	if productIR.Name != "Product" {
		t.Fatalf("expected Product second, got %s", productIR.Name)
	}
	if len(productIR.RelationsOut) != 1 {
		t.Fatalf("expected one outgoing relation")
	}
	if productIR.RelationsOut[0].NavigationName == "" {
		t.Fatalf("navigation name must not be empty")
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

	projectIR, err := Build(schema)
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

	projectIR, err := Build(schema)
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

func TestNavigationNameStripsIdSuffix(t *testing.T) {
	tests := []struct {
		name    string
		field   *parser.ParsedField
		wantNav string
	}{
		{
			name:    "categoryId",
			field:   &parser.ParsedField{FieldDefinition: &lexer.FieldDefinition{Name: "categoryId", Type: "relation", TargetEntity: "Category"}},
			wantNav: "Category",
		},
		{
			name:    "categoryID",
			field:   &parser.ParsedField{FieldDefinition: &lexer.FieldDefinition{Name: "categoryID", Type: "relation", TargetEntity: "Category"}},
			wantNav: "Category",
		},
		{
			name:    "fluid does not strip id",
			field:   &parser.ParsedField{FieldDefinition: &lexer.FieldDefinition{Name: "fluid", Type: "string"}},
			wantNav: "Fluid",
		},
		{
			name:    "squid does not strip id",
			field:   &parser.ParsedField{FieldDefinition: &lexer.FieldDefinition{Name: "squid", Type: "string"}},
			wantNav: "Squid",
		},
		{
			name:    "avoid does not strip id",
			field:   &parser.ParsedField{FieldDefinition: &lexer.FieldDefinition{Name: "avoid", Type: "string"}},
			wantNav: "Avoid",
		},
		{
			name:    "userId",
			field:   &parser.ParsedField{FieldDefinition: &lexer.FieldDefinition{Name: "userId", Type: "relation", TargetEntity: "User"}},
			wantNav: "User",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.field.NavigationName()
			if got != tt.wantNav {
				t.Errorf("NavigationName() = %q, want %q", got, tt.wantNav)
			}
		})
	}
}

func TestBuildCopiesDescription(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test", Description: "A test project"},
		Database:    "postgresql",
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				NamePlural: "Users",
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if projectIR.Description != "A test project" {
		t.Errorf("Description = %q, want %q", projectIR.Description, "A test project")
	}
}

func TestBuildCopiesFeaturesMap(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				NamePlural: "Users",
				Features:   map[string]bool{"audit": true, "soft_delete": true},
				FieldOrder: []string{"id"},
				Fields: map[string]*parser.ParsedField{
					"id": mustParsedField(t, "id", "uuid [primary]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	user := projectIR.Entities[0]
	if user.Features == nil {
		t.Fatal("Features map is nil")
	}
	if !user.Features["audit"] {
		t.Error("expected Features[audit] = true")
	}
	if !user.Features["soft_delete"] {
		t.Error("expected Features[soft_delete] = true")
	}
	if user.Features["optimistic_lock"] {
		t.Error("expected Features[optimistic_lock] = false")
	}
	if !user.HasFeature("audit") {
		t.Error("HasFeature('audit') should be true")
	}
	if !user.HasFeature("soft_delete") {
		t.Error("HasFeature('soft_delete') should be true")
	}
	if user.HasFeature("nonexistent") {
		t.Error("HasFeature('nonexistent') should be false")
	}
}

func TestBuildCircularDependency(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"A", "B"},
		Entities: map[string]*parser.ParsedEntity{
			"A": {
				Name:       "A",
				NamePlural: "As",
				FieldOrder: []string{"id", "bRef"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"bRef": mustParsedField(t, "bRef", "relation(B)"),
				},
			},
			"B": {
				Name:       "B",
				NamePlural: "Bs",
				FieldOrder: []string{"id", "aRef"},
				Fields: map[string]*parser.ParsedField{
					"id":   mustParsedField(t, "id", "uuid [primary]"),
					"aRef": mustParsedField(t, "aRef", "relation(A)"),
				},
			},
		},
	}
	schema.Entities["A"].Fields["id"].IsPrimary = true
	schema.Entities["B"].Fields["id"].IsPrimary = true

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(projectIR.Entities) != 2 {
		t.Fatalf("got %d entities, want 2", len(projectIR.Entities))
	}
}

func TestBuildCopiesDatabaseColumnName(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"User"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				NamePlural: "Users",
				FieldOrder: []string{"id", "firstName"},
				Fields: map[string]*parser.ParsedField{
					"id":        mustParsedField(t, "id", "uuid [primary]"),
					"firstName": mustParsedField(t, "firstName", "string"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true
	schema.Entities["User"].Fields["firstName"].DatabaseColumnName = "first_name"

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	user := projectIR.Entities[0]
	for _, f := range user.Fields {
		if f.Name == "firstName" {
			if f.DatabaseColumnName != "first_name" {
				t.Errorf("DatabaseColumnName = %q, want %q", f.DatabaseColumnName, "first_name")
			}
			return
		}
	}
	t.Fatal("firstName field not found in IR")
}
