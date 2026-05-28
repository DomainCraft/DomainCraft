package parser

import (
	"testing"
)

// TestFullSchemaWithAllFeatures tests a full schema using all features
func TestFullSchemaWithAllFeatures(t *testing.T) {
	yamlData := []byte(`
project:
  name: ECommerce Platform
  description: Full-featured e-commerce system
  version: 1.0.0
  multi_tenancy:
    enabled: true
    mode: column

database: postgresql
auth:
  type: jwt
api_style: rest

enums:
  OrderStatus:
    - PENDING
    - PROCESSING
    - SHIPPED
    - DELIVERED
    - CANCELLED
  
  ProductStatus:
    - DRAFT
    - PUBLISHED
    - ARCHIVED

entities:
  User:
    features: [audit, soft_delete]
    fields:
      id: uuid [primary]
      email: string [required, unique, email]
      firstName: string [required, min:2, max:50]
      lastName: string [required, min:2, max:50]
      avatar: string [optional, url]
      isActive: boolean [default:true]
    
    indexes:
      - fields: [email]
        unique: true
    
    permissions:
      read: [Admin, "*"]
      create: ["*"]
      update: ["@Owner", Admin]
      delete: [Admin]

  Product:
    features: [audit, soft_delete, optimistic_lock]
    fields:
      id: uuid [primary]
      title: string [required, min:5, max:200]
      description: text [optional]
      price: decimal [required, gte:0]
      stock: int [required, default:0]
      status: enum(ProductStatus) [default:DRAFT]
      categoryId: relation(Category) [optional, on_delete:set_null]
      tags: relation(Tag) [many]
    
    indexes:
      - fields: [status, categoryId]
        type: btree
      - fields: [title]
        type: btree

  Category:
    features: [audit]
    fields:
      id: uuid [primary]
      name: string [required, unique, min:3, max:100]
      description: string [optional, max:500]
      parentId: relation(Category) [optional, on_delete:set_null]
      slug: string [required, unique, regex:"^[a-z0-9-]+$"]

  Order:
    features: [audit_log, soft_delete]
    fields:
      id: uuid [primary]
      orderNumber: string [required, unique]
      userId: relation(User) [required, on_delete:restrict]
      status: enum(OrderStatus) [default:PENDING]
      totalAmount: decimal [required, gte:0]
      notes: string [optional, max:1000]
      items: relation(OrderItem) [many]
    
    permissions:
      read: [Admin, "@Owner"]
      create: [User]
      update: ["@Owner"]
      delete: [Admin]

  OrderItem:
    features: [audit]
    fields:
      id: uuid [primary]
      orderId: relation(Order) [required, on_delete:cascade]
      productId: relation(Product) [required, on_delete:restrict]
      quantity: int [required, gte:1]
      unitPrice: decimal [required, gte:0]

  Tag:
    features: [audit]
    fields:
      id: uuid [primary]
      name: string [required, unique, min:2, max:50]
      slug: string [required, unique]

  Review:
    features: [audit_log, soft_delete]
    fields:
      id: uuid [primary]
      productId: relation(Product) [required, on_delete:cascade]
      userId: relation(User) [required, on_delete:cascade]
      rating: int [required, gte:1, lt:6]
      title: string [required, min:5, max:100]
      content: text [optional]
      isVerifiedPurchase: boolean [default:false]
    
    permissions:
      read: ["*"]
      create: [User]
      update: ["@Owner", Admin]
      delete: [Admin]
`)

	schema, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	// Verify project
	if schema.Project.Name != "ECommerce Platform" {
		t.Errorf("project name = %v, want ECommerce Platform", schema.Project.Name)
	}
	if !schema.Project.MultiTenancy.Enabled {
		t.Errorf("multi_tenancy should be enabled")
	}

	// Verify configuration
	if schema.Database != "postgresql" {
		t.Errorf("database = %v, want postgresql", schema.Database)
	}
	if schema.Auth.Type != "jwt" {
		t.Errorf("auth.type = %v, want jwt", schema.Auth.Type)
	}

	// Verify enums
	if len(schema.Enums) != 2 {
		t.Errorf("enums count = %d, want 2", len(schema.Enums))
	}
	if len(schema.Enums["OrderStatus"]) != 5 {
		t.Errorf("OrderStatus values count = %d, want 5", len(schema.Enums["OrderStatus"]))
	}

	// Verify entity count
	if len(schema.Entities) != 7 {
		t.Errorf("entities count = %d, want 7", len(schema.Entities))
	}

	// Verify User
	user := schema.Entities["User"]
	if user.Name != "User" || user.NamePlural != "Users" {
		t.Errorf("User entity name or plural incorrect")
	}
	if !user.Features["audit"] || !user.Features["soft_delete"] {
		t.Errorf("User should have audit and soft_delete features")
	}
	// User should have automatic fields: createdAt, updatedAt, deletedAt
	expectedUserFields := []string{"id", "email", "firstName", "lastName", "avatar", "isActive", "createdAt", "updatedAt", "deletedAt"}
	if len(user.Fields) != len(expectedUserFields) {
		t.Errorf("User fields count = %d, want %d", len(user.Fields), len(expectedUserFields))
	}

	// Verify Product
	product := schema.Entities["Product"]
	if !product.Features["audit"] || !product.Features["soft_delete"] || !product.Features["optimistic_lock"] {
		t.Errorf("Product should have audit, soft_delete, optimistic_lock features")
	}
	// Verify that version field was added
	if _, ok := product.Fields["version"]; !ok {
		t.Errorf("Product should have version field from optimistic_lock feature")
	}

	// Verify Product relations
	categoryId := product.Fields["categoryId"]
	if !categoryId.IsOptional || categoryId.OnDelete != "set_null" {
		t.Errorf("categoryId should be optional with on_delete:set_null")
	}

	tagsField := product.Fields["tags"]
	if tagsField.Type != "relation" || tagsField.RelationType != "many-to-many" {
		t.Errorf("tags should be many-to-many relation")
	}

	// Verify Order
	order := schema.Entities["Order"]
	orderNumber := order.Fields["orderNumber"]
	if !orderNumber.IsUnique {
		t.Errorf("orderNumber should be unique")
	}

	// Verify cascade delete
	orderItem := schema.Entities["OrderItem"]
	orderIdField := orderItem.Fields["orderId"]
	if orderIdField.OnDelete != "cascade" {
		t.Errorf("orderId should have on_delete:cascade")
	}

	// Verify indexes
	if len(product.Indexes) != 2 {
		t.Errorf("Product should have 2 indexes")
	}

	// Verify permissions
	if order.Permissions == nil {
		t.Errorf("Order should have permissions")
	}
	if len(order.Permissions.Read) != 2 {
		t.Errorf("Order read permissions count = %d, want 2", len(order.Permissions.Read))
	}

	// Verify Review with cascade delete
	review := schema.Entities["Review"]
	productIdField := review.Fields["productId"]
	if productIdField.RelationType != "many-to-one" {
		t.Errorf("review.productId should be many-to-one relation")
	}
}

// TestCircularRelationships tests self-referential (circular) relationships
func TestCircularRelationships(t *testing.T) {
	// A category can have a parent category -- this is a self-referential relationship
	yamlData := []byte(`
project:
  name: Test
entities:
  Category:
    fields:
      id: uuid [primary]
      name: string
      parentId: relation(Category) [optional, on_delete:set_null]
`)

	schema, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	category := schema.Entities["Category"]
	parentId := category.Fields["parentId"]

	if parentId.Type != "relation" || parentId.RelationTarget != "Category" {
		t.Errorf("parentId should be self-referential relation to Category")
	}
	if !parentId.IsOptional {
		t.Errorf("parentId should be optional")
	}
}

// TestDatabaseColumnNameGeneration tests database column name generation
func TestDatabaseColumnNameGeneration(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  User:
    fields:
      id: uuid [primary]
      firstName: string
      lastName: string
      emailAddress: string
      createdAt: datetime
      updatedAt: datetime
      isActive: boolean
`)

	schema, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	user := schema.Entities["User"]

	tests := []struct {
		fieldName  string
		wantDbName string
	}{
		{"id", "id"},
		{"firstName", "first_name"},
		{"lastName", "last_name"},
		{"emailAddress", "email_address"},
		{"createdAt", "created_at"},
		{"updatedAt", "updated_at"},
		{"isActive", "is_active"},
	}

	for _, tt := range tests {
		if field, ok := user.Fields[tt.fieldName]; ok {
			if field.DatabaseColumnName != tt.wantDbName {
				t.Errorf("field %s: got db name %v, want %v",
					tt.fieldName, field.DatabaseColumnName, tt.wantDbName)
			}
		}
	}
}

// TestSeedData tests seed data parsing
func TestSeedData(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  Role:
    fields:
      id: int [primary]
      name: string
    seed:
      - { id: 1, name: "Admin" }
      - { id: 2, name: "User" }
      - { id: 3, name: "Guest" }
`)

	schema, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	role := schema.Entities["Role"]
	if len(role.Seed) != 3 {
		t.Errorf("Role seed count = %d, want 3", len(role.Seed))
	}

	firstSeed := role.Seed[0]
	if id, ok := firstSeed["id"]; !ok || id != 1 {
		t.Errorf("first seed id should be 1")
	}
	if name, ok := firstSeed["name"]; !ok || name != "Admin" {
		t.Errorf("first seed name should be Admin")
	}
}

// TestEntityOrdering tests entity ordering
func TestEntityOrdering(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  Zebra:
    fields:
      id: uuid [primary]
  Apple:
    fields:
      id: uuid [primary]
  Banana:
    fields:
      id: uuid [primary]
`)

	schema, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	// Order should be sorted alphabetically
	expectedOrder := []string{"Apple", "Banana", "Zebra"}
	for i, expected := range expectedOrder {
		if schema.EntityOrder[i] != expected {
			t.Errorf("entity order[%d] = %v, want %v", i, schema.EntityOrder[i], expected)
		}
	}
}

// TestComplexValidations tests complex validation rules
func TestComplexValidations(t *testing.T) {
	yamlData := []byte(`
project:
  name: Test
entities:
  Product:
    fields:
      id: uuid [primary]
      title: string [required, min:5, max:200]
      sku: string [unique, regex:"^[A-Z0-9-]{5,20}$"]
      price: decimal [required, gte:0.01, lt:1000000]
      discount: decimal [optional, gte:0, lte:100, default:0]
`)

	schema, err := ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	product := schema.Entities["Product"]

	// Verify title validations
	title := product.Fields["title"]
	if title.Validations["min"] != "5" || title.Validations["max"] != "200" {
		t.Errorf("title validations incorrect")
	}

	// Verify sku with regex
	sku := product.Fields["sku"]
	if sku.IsUnique != true {
		t.Errorf("sku should be unique")
	}
	if _, ok := sku.Validations["regex"]; !ok {
		t.Errorf("sku should have regex validation")
	}

	// Verify price
	price := product.Fields["price"]
	if price.Validations["gte"] != "0.01" {
		t.Errorf("price gte should be 0.01")
	}

	// Verify discount default
	discount := product.Fields["discount"]
	if discount.DefaultValue != "0" {
		t.Errorf("discount default should be 0")
	}
}
