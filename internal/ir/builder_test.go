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

// TestBuildCopiesFieldOldName verifies the field-level `old_name` rename hint is
// carried into the IR, including the derived old snake_case DB column name that
// bridges use to emit a safe RenameColumn.
func TestBuildCopiesFieldOldName(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"Product"},
		Entities: map[string]*parser.ParsedEntity{
			"Product": {
				Name:       "Product",
				NamePlural: "Products",
				FieldOrder: []string{"id", "displayName"},
				Fields: map[string]*parser.ParsedField{
					"id":          mustParsedField(t, "id", "uuid [primary]"),
					"displayName": mustParsedField(t, "displayName", "string [required, old_name:productTitle]"),
				},
			},
		},
	}

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for _, f := range projectIR.Entities[0].Fields {
		if f.Name == "displayName" {
			if f.OldName != "productTitle" {
				t.Errorf("OldName = %q, want %q", f.OldName, "productTitle")
			}
			if f.OldDatabaseColumnName != "product_title" {
				t.Errorf("OldDatabaseColumnName = %q, want %q", f.OldDatabaseColumnName, "product_title")
			}
			return
		}
	}
	t.Fatal("displayName field not found in IR")
}

// TestBuildResolvesTargetEntityAfterTopoSort is a regression test: after the
// topological sort reorders the entity slice, every relation's TargetEntity
// pointer must still point at the correct entity (the in-place reorder used to
// overwrite the backing array the pointers referenced, corrupting the targets).
func TestBuildResolvesTargetEntityAfterTopoSort(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"User", "Product", "Category", "Order", "OrderItem", "Tag", "Review", "Document"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				NamePlural: "Users",
				FieldOrder: []string{"id"},
				Fields:     map[string]*parser.ParsedField{"id": mustParsedField(t, "id", "uuid [primary]")},
			},
			"Product": {
				Name:       "Product",
				NamePlural: "Products",
				FieldOrder: []string{"id", "categoryId", "supplierId", "tags"},
				Fields: map[string]*parser.ParsedField{
					"id":         mustParsedField(t, "id", "uuid [primary]"),
					"categoryId": mustParsedField(t, "categoryId", "relation(Category)"),
					"supplierId": mustParsedField(t, "supplierId", "relation(User)"),
					"tags":       mustParsedField(t, "tags", "relation(Tag) [many]"),
				},
			},
			"Category": {
				Name:       "Category",
				NamePlural: "Categories",
				FieldOrder: []string{"id"},
				Fields:     map[string]*parser.ParsedField{"id": mustParsedField(t, "id", "uuid [primary]")},
			},
			"Order": {
				Name:       "Order",
				NamePlural: "Orders",
				FieldOrder: []string{"id", "userId"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"userId": mustParsedField(t, "userId", "relation(User)"),
				},
			},
			"OrderItem": {
				Name:       "OrderItem",
				NamePlural: "OrderItems",
				FieldOrder: []string{"id", "orderId", "productId"},
				Fields: map[string]*parser.ParsedField{
					"id":        mustParsedField(t, "id", "uuid [primary]"),
					"orderId":   mustParsedField(t, "orderId", "relation(Order)"),
					"productId": mustParsedField(t, "productId", "relation(Product)"),
				},
			},
			"Tag": {
				Name:       "Tag",
				NamePlural: "Tags",
				FieldOrder: []string{"id"},
				Fields:     map[string]*parser.ParsedField{"id": mustParsedField(t, "id", "uuid [primary]")},
			},
			"Review": {
				Name:       "Review",
				NamePlural: "Reviews",
				FieldOrder: []string{"id", "productId", "userId"},
				Fields: map[string]*parser.ParsedField{
					"id":        mustParsedField(t, "id", "uuid [primary]"),
					"productId": mustParsedField(t, "productId", "relation(Product)"),
					"userId":    mustParsedField(t, "userId", "relation(User)"),
				},
			},
			"Document": {
				Name:       "Document",
				NamePlural: "Documents",
				FieldOrder: []string{"id", "authorId"},
				Fields: map[string]*parser.ParsedField{
					"id":       mustParsedField(t, "id", "uuid [primary]"),
					"authorId": mustParsedField(t, "authorId", "relation(User)"),
				},
			},
		},
	}

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	byName := make(map[string]*IREntity, len(projectIR.Entities))
	for i := range projectIR.Entities {
		byName[projectIR.Entities[i].Name] = &projectIR.Entities[i]
	}

	product := byName["Product"]
	if product == nil {
		t.Fatal("Product entity not found")
	}

	wantOut := []struct {
		field  string
		target string
		many   bool
	}{
		{"categoryId", "Category", false},
		{"supplierId", "User", false},
		{"tags", "Tag", true},
	}
	if len(product.RelationsOut) != len(wantOut) {
		t.Fatalf("Product.RelationsOut = %d relations, want %d", len(product.RelationsOut), len(wantOut))
	}
	for i, want := range wantOut {
		rel := product.RelationsOut[i]
		if rel.FieldName != want.field {
			t.Errorf("RelationsOut[%d].FieldName = %q, want %q", i, rel.FieldName, want.field)
		}
		if rel.TargetEntity == nil || rel.TargetEntity.Name != want.target {
			t.Errorf("RelationsOut[%d] (%s) target = %v, want %q", i, rel.FieldName, targetName(rel.TargetEntity), want.target)
		}
		if rel.IsMany != want.many {
			t.Errorf("RelationsOut[%d] (%s) IsMany = %v, want %v", i, rel.FieldName, rel.IsMany, want.many)
		}
	}

	// Every pointer must live in the returned project slice (not a stale copy).
	for i := range projectIR.Entities {
		for _, rel := range projectIR.Entities[i].RelationsOut {
			if rel.TargetEntity != nil && rel.TargetEntity != byName[rel.TargetEntity.Name] {
				t.Errorf("%s.RelationsOut[%s] target pointer is stale", projectIR.Entities[i].Name, rel.FieldName)
			}
		}
		for _, rel := range projectIR.Entities[i].RelationsIn {
			if rel.TargetEntity != nil && rel.TargetEntity != byName[rel.TargetEntity.Name] {
				t.Errorf("%s.RelationsIn[%s] target pointer is stale", projectIR.Entities[i].Name, rel.FieldName)
			}
		}
	}
}

func targetName(e *IREntity) string {
	if e == nil {
		return "<nil>"
	}
	return e.Name
}

func findEntity(project *IRProject, name string) *IREntity {
	if project == nil {
		return nil
	}
	for i := range project.Entities {
		if project.Entities[i].Name == name {
			return &project.Entities[i]
		}
	}
	return nil
}

// TestBuildDisambiguatesInverseNavigationNames is a regression test: when two
// relations from the SAME entity point to the SAME target (e.g. EscrowContract
// has both a Buyer and a Seller referencing User), the pluralized inverse name
// "EscrowContracts" would collide and produce duplicate C# navigation properties.
func TestBuildDisambiguatesInverseNavigationNames(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"User", "EscrowContract"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				NamePlural: "Users",
				FieldOrder: []string{"id"},
				Fields:     map[string]*parser.ParsedField{"id": mustParsedField(t, "id", "uuid [primary]")},
			},
			"EscrowContract": {
				Name:       "EscrowContract",
				NamePlural: "EscrowContracts",
				FieldOrder: []string{"id", "buyer", "seller"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"buyer":  mustParsedField(t, "buyer", "relation(User) [required, on_delete:restrict]"),
					"seller": mustParsedField(t, "seller", "relation(User) [required, on_delete:restrict]"),
				},
			},
		},
	}
	schema.Entities["User"].Fields["id"].IsPrimary = true
	schema.Entities["EscrowContract"].Fields["id"].IsPrimary = true

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	user := findEntity(projectIR, "User")
	if user == nil {
		t.Fatal("User entity not found")
	}
	if len(user.RelationsIn) != 2 {
		t.Fatalf("User.RelationsIn = %d relations, want 2", len(user.RelationsIn))
	}
	seen := make(map[string]bool)
	names := make([]string, 0, len(user.RelationsIn))
	for _, in := range user.RelationsIn {
		if seen[in.InverseNavName] {
			t.Errorf("duplicate inverse navigation name %q on User", in.InverseNavName)
		}
		seen[in.InverseNavName] = true
		names = append(names, in.InverseNavName)
	}
	// The first relation keeps the plural name; the colliding one is prefixed
	// with its field name so both collections exist with unique names.
	want := []string{"EscrowContracts", "SellerEscrowContracts"}
	for i, w := range want {
		if i >= len(names) || names[i] != w {
			t.Errorf("User.RelationsIn[%d].InverseNavName = %v, want %v", i, names, want)
			break
		}
	}

	// The owning relations must reference the SAME disambiguated names so the
	// EF mapping (.WithMany) matches the generated collection properties.
	ec := findEntity(projectIR, "EscrowContract")
	if ec == nil {
		t.Fatal("EscrowContract entity not found")
	}
	invByField := make(map[string]string)
	for _, out := range ec.RelationsOut {
		invByField[out.FieldName] = out.InverseNavName
	}
	if invByField["buyer"] != "EscrowContracts" {
		t.Errorf("buyer inverse = %q, want EscrowContracts", invByField["buyer"])
	}
	if invByField["seller"] != "SellerEscrowContracts" {
		t.Errorf("seller inverse = %q, want SellerEscrowContracts", invByField["seller"])
	}
}

// TestBuildReconcilesOneToManyDeclaredOnBothSides is a regression test: a
// `[many]` relation whose target declares a single FK back describes the SAME
// one-to-many relationship, not a many-to-many. Without reconciliation the C#
// bridge would emit a spurious EF join table AND a bogus inverse collection.
func TestBuildReconcilesOneToManyDeclaredOnBothSides(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"Wallet", "WalletTransaction"},
		Entities: map[string]*parser.ParsedEntity{
			"Wallet": {
				Name:       "Wallet",
				NamePlural: "Wallets",
				FieldOrder: []string{"id", "transactions"},
				Fields: map[string]*parser.ParsedField{
					"id":           mustParsedField(t, "id", "uuid [primary]"),
					"transactions": mustParsedField(t, "transactions", "relation(WalletTransaction) [many]"),
				},
			},
			"WalletTransaction": {
				Name:       "WalletTransaction",
				NamePlural: "WalletTransactions",
				FieldOrder: []string{"id", "wallet"},
				Fields: map[string]*parser.ParsedField{
					"id":     mustParsedField(t, "id", "uuid [primary]"),
					"wallet": mustParsedField(t, "wallet", "relation(Wallet) [required, on_delete:cascade]"),
				},
			},
		},
	}
	schema.Entities["Wallet"].Fields["id"].IsPrimary = true
	schema.Entities["WalletTransaction"].Fields["id"].IsPrimary = true

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	wallet := findEntity(projectIR, "Wallet")
	if wallet == nil {
		t.Fatal("Wallet entity not found")
	}

	// The [many] relation must be classified as the collection side of a
	// one-to-many (paired with the FK declared on WalletTransaction).
	var transactions *IRRelation
	for i := range wallet.RelationsOut {
		if wallet.RelationsOut[i].FieldName == "transactions" {
			transactions = &wallet.RelationsOut[i]
		}
	}
	if transactions == nil {
		t.Fatal("Wallet.transactions relation not found")
	}
	if transactions.RelationType != "one-to-many" {
		t.Errorf("transactions.RelationType = %q, want one-to-many", transactions.RelationType)
	}
	if transactions.PairFieldName != "wallet" {
		t.Errorf("transactions.PairFieldName = %q, want wallet", transactions.PairFieldName)
	}
	if transactions.PairNavigationName != "Wallet" {
		t.Errorf("transactions.PairNavigationName = %q, want Wallet", transactions.PairNavigationName)
	}
	if transactions.OnDeleteBehavior != "cascade" {
		t.Errorf("transactions.OnDeleteBehavior = %q, want cascade (borrowed from FK)", transactions.OnDeleteBehavior)
	}

	// The FK side's inverse collection resolves to the [many] field's name.
	wt := findEntity(projectIR, "WalletTransaction")
	if wt == nil {
		t.Fatal("WalletTransaction entity not found")
	}
	var walletRel *IRRelation
	for i := range wt.RelationsOut {
		if wt.RelationsOut[i].FieldName == "wallet" {
			walletRel = &wt.RelationsOut[i]
		}
	}
	if walletRel == nil {
		t.Fatal("WalletTransaction.wallet relation not found")
	}
	if walletRel.InverseNavName != "Transactions" {
		t.Errorf("wallet.InverseNavName = %q, want Transactions", walletRel.InverseNavName)
	}

	// No spurious inverse collection on WalletTransaction (would become the
	// many-to-many join collection) and no duplicate collection on Wallet.
	if len(wt.RelationsIn) != 0 {
		t.Errorf("WalletTransaction.RelationsIn = %d relations, want 0 (no join collection)", len(wt.RelationsIn))
	}
	if len(wallet.RelationsIn) != 0 {
		t.Errorf("Wallet.RelationsIn = %d relations, want 0", len(wallet.RelationsIn))
	}
}

// TestBuildPairNavigationNameStripsIdSuffix is a regression test for the C# bridge:
// when the paired FK field carries an "Id" suffix (e.g. OrderItem.orderId), the
// [many] side's WithOne(...) must reference the NAVIGATION property ("Order"), not
// the pascalcased field name ("OrderId"). Without PairNavigationName the generated
// OrderConfiguration.cs failed to compile with
// "Cannot implicitly convert type 'System.Guid' to 'Order'".
func TestBuildPairNavigationNameStripsIdSuffix(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Test"},
		Database:    "postgresql",
		EntityOrder: []string{"Order", "OrderItem"},
		Entities: map[string]*parser.ParsedEntity{
			"Order": {
				Name:       "Order",
				NamePlural: "Orders",
				FieldOrder: []string{"id", "items"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"items": mustParsedField(t, "items", "relation(OrderItem) [many]"),
				},
			},
			"OrderItem": {
				Name:       "OrderItem",
				NamePlural: "OrderItems",
				FieldOrder: []string{"id", "orderId"},
				Fields: map[string]*parser.ParsedField{
					"id":      mustParsedField(t, "id", "uuid [primary]"),
					"orderId": mustParsedField(t, "orderId", "relation(Order) [required, on_delete:cascade]"),
				},
			},
		},
	}
	schema.Entities["Order"].Fields["id"].IsPrimary = true
	schema.Entities["OrderItem"].Fields["id"].IsPrimary = true

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	order := findEntity(projectIR, "Order")
	if order == nil {
		t.Fatal("Order entity not found")
	}

	var items *IRRelation
	for i := range order.RelationsOut {
		if order.RelationsOut[i].FieldName == "items" {
			items = &order.RelationsOut[i]
		}
	}
	if items == nil {
		t.Fatal("Order.items relation not found")
	}
	if items.PairFieldName != "orderId" {
		t.Errorf("items.PairFieldName = %q, want orderId", items.PairFieldName)
	}
	// The WithOne navigation must be the stripped navigation name, not "OrderId".
	if items.PairNavigationName != "Order" {
		t.Errorf("items.PairNavigationName = %q, want Order", items.PairNavigationName)
	}
}

func TestBuildConvertsAuthAndValidations(t *testing.T) {
	disabled := false
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Shop"},
		Database:    "postgresql",
		EntityOrder: []string{"User", "Order"},
		Entities: map[string]*parser.ParsedEntity{
			"User": {
				Name:       "User",
				NamePlural: "Users",
				FieldOrder: []string{"id", "email", "password"},
				Fields: map[string]*parser.ParsedField{
					"id":       mustParsedField(t, "id", "uuid [primary]"),
					"email":    mustParsedField(t, "email", "string [required, email]"),
					"password": mustParsedField(t, "password", "string [required]"),
				},
			},
			"Order": {
				Name:       "Order",
				NamePlural: "Orders",
				FieldOrder: []string{"id", "total", "sku"},
				Fields: map[string]*parser.ParsedField{
					"id":    mustParsedField(t, "id", "uuid [primary]"),
					"total": mustParsedField(t, "total", "decimal [min:0, max:999999]"),
					"sku":   mustParsedField(t, "sku", "string [max:32]"),
				},
			},
		},
	}
	schema.Auth = &parser.AuthConfig{
		Type:  "jwt",
		Roles: []string{"Admin", "User"},
		Endpoints: parser.AuthEndpoints{
			Register: &disabled,
		},
	}

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	auth := projectIR.Auth
	if auth == nil {
		t.Fatal("auth config missing from IR")
	}
	if auth.Entity != "User" {
		t.Errorf("auth entity = %q, want auto-detected User (email+password)", auth.Entity)
	}
	if len(auth.Roles) != 2 || auth.Roles[0] != "Admin" {
		t.Errorf("roles = %v, want a copy of the declared roles", auth.Roles)
	}
	if !auth.Endpoints.HasLogin || !auth.Endpoints.HasMe || !auth.Endpoints.HasSetup {
		t.Errorf("endpoints default to true, got %+v", auth.Endpoints)
	}
	if auth.Endpoints.HasRegister {
		t.Error("register explicitly disabled but got true")
	}

	var order *IREntity
	for i := range projectIR.Entities {
		if projectIR.Entities[i].Name == "Order" {
			order = &projectIR.Entities[i]
		}
	}
	if order == nil {
		t.Fatal("Order missing from IR entities")
	}
	total := order.Fields[1]
	if total.Name != "total" {
		t.Fatalf("expected total at index 1, got %s", total.Name)
	}
	names := map[string]string{}
	for _, v := range total.Validations {
		names[v.Name] = v.Value
	}
	if names["min"] != "0" || names["max"] != "999999" {
		t.Errorf("validations = %v, want min=0 and max=999999 converted into the IR", names)
	}
}

func TestBuildWithoutAuthYieldsNilAuth(t *testing.T) {
	schema := &parser.ParsedSchema{
		Project:     parser.ProjectConfig{Name: "Plain"},
		Database:    "postgresql",
		EntityOrder: []string{"Item"},
		Entities: map[string]*parser.ParsedEntity{
			"Item": {
				Name:       "Item",
				NamePlural: "Items",
				FieldOrder: []string{"id"},
				Fields:     map[string]*parser.ParsedField{"id": mustParsedField(t, "id", "uuid [primary]")},
			},
		},
	}

	projectIR, err := Build(schema)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if projectIR.Auth != nil {
		t.Errorf("auth = %+v, want nil when no auth block is declared", projectIR.Auth)
	}
}
