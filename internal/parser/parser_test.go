package parser

import (
	"testing"
)

func TestParseRawSchema(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test Project
database: postgresql
auth: jwt
api_style: rest
entities:
  User:
    fields:
      id: uuid [primary]
      name: string
`)

	schema, err := ParseRawSchema(yamlData)
	if err != nil {
		t.Fatalf("ParseRawSchema() error = %v", err)
	}

	if schema.Project.Name != "Test Project" {
		t.Errorf("got project name %v, want Test Project", schema.Project.Name)
	}
	if schema.Database != "postgresql" {
		t.Errorf("got database %v, want postgresql", schema.Database)
	}
	if schema.Auth != "jwt" {
		t.Errorf("got auth %v, want jwt", schema.Auth)
	}
}

func TestParseDefaults(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities: {}
`)

	schema, err := ParseRawSchema(yamlData)
	if err != nil {
		t.Fatalf("ParseRawSchema() error = %v", err)
	}

	if schema.Database != "postgresql" {
		t.Errorf("got default database %v, want postgresql", schema.Database)
	}
	if schema.Auth != "none" {
		t.Errorf("got default auth %v, want none", schema.Auth)
	}
	if schema.APIStyle != "rest" {
		t.Errorf("got default api_style %v, want rest", schema.APIStyle)
	}
}

func TestParseEntity(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
database: postgresql
entities:
  User:
    features: [audit]
    fields:
      id: uuid [primary]
      email: string [required, unique, email]
      name: string
    permissions:
      read: [Admin, "*"]
      create: ["*"]
`)

	parsed, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	user, ok := parsed.Entities["User"]
	if !ok {
		t.Fatalf("User entity not found")
	}

	if user.Name != "User" {
		t.Errorf("got name %v, want User", user.Name)
	}
	if user.NamePlural != "Users" {
		t.Errorf("got plural %v, want Users", user.NamePlural)
	}

	if len(user.Fields) != 5 {
		t.Errorf("got %d fields, want 5", len(user.Fields))
	}

	if _, ok := user.Fields["createdAt"]; !ok {
		t.Errorf("createdAt field not added by audit feature")
	}
	if _, ok := user.Fields["updatedAt"]; !ok {
		t.Errorf("updatedAt field not added by audit feature")
	}

	if user.Features["audit"] != true {
		t.Errorf("audit feature not set")
	}
}

func TestParseFeatures(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  Document:
    features: [audit_log, soft_delete, optimistic_lock]
    fields:
      id: uuid [primary]
`)

	parsed, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	doc := parsed.Entities["Document"]

	// Verify that required fields were added
	expectedFields := []string{
		"id", "createdBy", "updatedBy", "deletedAt", "version",
	}

	for _, fieldName := range expectedFields {
		if _, ok := doc.Fields[fieldName]; !ok && fieldName != "id" {
			t.Errorf("expected field %v not found", fieldName)
		}
	}

	// Verify that deletedAt is optional
	if deletedAt, ok := doc.Fields["deletedAt"]; ok && !deletedAt.IsOptional {
		t.Errorf("deletedAt should be optional")
	}
}

func TestParseRelations(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  Product:
    fields:
      id: uuid [primary]
      categoryId: relation(Category) [optional, on_delete:set_null]
      supplierId: relation(User)
`)

	parsed, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	product := parsed.Entities["Product"]

	// Check categoryId
	categoryId := product.Fields["categoryId"]
	if categoryId.Type != "relation" {
		t.Errorf("categoryId type should be relation")
	}
	if categoryId.RelationTarget != "Category" {
		t.Errorf("categoryId target should be Category")
	}
	if !categoryId.IsOptional {
		t.Errorf("categoryId should be optional")
	}
	if categoryId.OnDelete != "set_null" {
		t.Errorf("categoryId on_delete should be set_null")
	}

	// Check supplierId
	supplierId := product.Fields["supplierId"]
	if supplierId.RelationTarget != "User" {
		t.Errorf("supplierId target should be User")
	}
	if supplierId.IsOptional {
		t.Errorf("supplierId should not be optional")
	}
}

func TestParseIndexes(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  Product:
    fields:
      id: uuid [primary]
      status: string
      createdAt: datetime
    indexes:
      - fields: [status, createdAt]
        type: btree
      - fields: [id]
        unique: true
`)

	parsed, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	product := parsed.Entities["Product"]
	if len(product.Indexes) != 2 {
		t.Errorf("got %d indexes, want 2", len(product.Indexes))
	}

	if product.Indexes[0].Type != "btree" {
		t.Errorf("first index type should be btree")
	}
	if len(product.Indexes[0].Fields) != 2 {
		t.Errorf("first index should have 2 fields")
	}
	if !product.Indexes[1].Unique {
		t.Errorf("second index should be unique")
	}
}

func TestPluralizeNames(t *testing.T) {
	tests := []struct {
		singular string
		plural   string
	}{
		{"User", "Users"},
		{"Category", "Categories"},
		{"Status", "Statuses"},
		{"Box", "Boxes"},
		{"Lady", "Ladies"},
	}

	for _, tt := range tests {
		// Test via ParseYAML since pluralize is an internal function
		yamlData := []byte("project:\n  name: Test\nentities:\n  " + tt.singular + ":\n    fields:\n      id: uuid [primary]")
		parsed, _ := ParseYAML(yamlData)
		entity := parsed.Entities[tt.singular]
		if entity.NamePlural != tt.plural {
			t.Errorf("pluralize(%s) = %s, want %s", tt.singular, entity.NamePlural, tt.plural)
		}
	}
}

func TestDatabaseColumnNames(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  User:
    fields:
      id: uuid [primary]
      firstName: string
      lastName: string
      createdAt: datetime
`)

	parsed, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	user := parsed.Entities["User"]

	tests := []struct {
		fieldName  string
		wantDbName string
	}{
		{"id", "id"},
		{"firstName", "first_name"},
		{"lastName", "last_name"},
		{"createdAt", "created_at"},
	}

	for _, tt := range tests {
		if field, ok := user.Fields[tt.fieldName]; ok {
			if field.DatabaseColumnName != tt.wantDbName {
				t.Errorf("field %s: got db name %v, want %v", tt.fieldName, field.DatabaseColumnName, tt.wantDbName)
			}
		}
	}
}

func TestParsePermissions(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  Document:
    fields:
      id: uuid [primary]
    permissions:
      read: [Admin, "@Owner"]
      create: [User, Admin]
      update: ["@Owner"]
      delete: [Admin]
      read_public: "condition(isPublished == true)"
`)

	parsed, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	doc := parsed.Entities["Document"]
	if doc.Permissions == nil {
		t.Fatalf("Permissions should not be nil")
	}

	if len(doc.Permissions.Read) != 2 {
		t.Errorf("got %d read permissions, want 2", len(doc.Permissions.Read))
	}
	if doc.Permissions.ReadPublic != "condition(isPublished == true)" {
		t.Errorf("got read_public %v", doc.Permissions.ReadPublic)
	}
}
