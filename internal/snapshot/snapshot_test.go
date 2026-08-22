package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/internal/renderer"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := SnapshotPath(dir)

	snap := New("csharp-restful", testProject("Product"), []renderer.RenderedFile{
		{Path: "src/Domain/Entities/Product.cs", Entity: "Product", Written: true},
		{Path: "src/Services/ProductService.cs", Entity: "Product", Custom: true, Written: true},
	})

	if err := Save(path, snap); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot file not created: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.FormatVersion != FormatVersion {
		t.Errorf("FormatVersion = %d, want %d", loaded.FormatVersion, FormatVersion)
	}
	if len(loaded.Files) != 2 {
		t.Errorf("got %d files, want 2", len(loaded.Files))
	}
	state, ok := loaded.Entities["Product"]
	if !ok {
		t.Fatal("expected Product entity in snapshot")
	}
	if state.Fields["id"] != "uuid" {
		t.Errorf("Product.id = %q, want %q", state.Fields["id"], "uuid")
	}
}

func TestLoadMissingSnapshotReturnsNil(t *testing.T) {
	loaded, err := Load(filepath.Join(t.TempDir(), "nope", "snapshot.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil snapshot for missing file")
	}
}

func TestComputeDiffDeletedEntity(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Domain/Entities/Product.cs")
	writeTestFile(t, dir, "src/Services/ProductService.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{"Product": {Fields: map[string]string{"id": "uuid"}}},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Product.cs", Entity: "Product"},
			{Path: "src/Services/ProductService.cs", Entity: "Product", Custom: true},
			{Path: "src/WebApi/Program.cs"}, // project-level — ignored
		},
	}
	newProject := testProject("User")

	diff := ComputeDiff(old, newProject, dir)
	if len(diff.Deleted) != 1 {
		t.Fatalf("got %d deleted entities, want 1", len(diff.Deleted))
	}
	del := diff.Deleted[0]
	if del.Name != "Product" {
		t.Errorf("deleted name = %q, want Product", del.Name)
	}
	if len(del.Files) != 2 {
		t.Fatalf("got %d orphaned files, want 2", len(del.Files))
	}
	if !del.Files[0].Custom && !del.Files[1].Custom {
		t.Error("expected one custom file in orphaned set")
	}
}

func TestComputeDiffRenameAndTypeChange(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Domain/Entities/Product.cs")
	writeTestFile(t, dir, "src/Services/ProductService.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product": {Fields: map[string]string{"id": "uuid", "name": "string"}},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Product.cs", Entity: "Product"},
			{Path: "src/Services/ProductService.cs", Entity: "Product", Custom: true},
		},
	}

	// Product -> Item (via old_name) and id type changed uuid -> int.
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Item",
				OldName:    "Product",
				NamePlural: "Items",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "int", IsPrimary: true},
					{Name: "name", DatabaseType: "string"},
				},
			},
		},
	}

	diff := ComputeDiff(old, project, dir)
	if len(diff.Renamed) != 1 {
		t.Fatalf("got %d renames, want 1", len(diff.Renamed))
	}
	ren := diff.Renamed[0]
	if ren.OldName != "Product" || ren.NewName != "Item" {
		t.Errorf("rename = %q -> %q, want Product -> Item", ren.OldName, ren.NewName)
	}
	if len(diff.Deleted) != 0 {
		t.Errorf("got %d deleted entities, want 0", len(diff.Deleted))
	}
	if len(diff.TypeChanges) != 1 {
		t.Fatalf("got %d type changes, want 1", len(diff.TypeChanges))
	}
	tc := diff.TypeChanges[0]
	if tc.Entity != "Item" || tc.Field != "id" || tc.OldType != "uuid" || tc.NewType != "int" {
		t.Errorf("type change = %+v, want Item.id uuid->int", tc)
	}
	if len(tc.CustomFiles) != 1 || tc.CustomFiles[0] != "src/Services/ProductService.cs" {
		t.Errorf("custom files = %v, want [src/Services/ProductService.cs]", tc.CustomFiles)
	}
}

func TestRenameRelPath(t *testing.T) {
	tests := []struct {
		rel, old, new, want string
	}{
		{"src/Services/ProductService.cs", "Product", "Item", "src/Services/ItemService.cs"},
		{"src/WebApi/Controllers/ProductsController.cs", "Product", "Item", "src/WebApi/Controllers/ItemsController.cs"},
		{"src/Domain/Entities/Product.cs", "Product", "Item", "src/Domain/Entities/Item.cs"},
		// irregular plural (Category -> Categories)
		{"src/WebApi/Controllers/CategoriesController.cs", "Category", "Item", "src/WebApi/Controllers/ItemsController.cs"},
		// no entity name in path -> unchanged
		{"src/Shared/Constants.cs", "Product", "Item", "src/Shared/Constants.cs"},
	}
	for _, tt := range tests {
		if got := RenameRelPath(tt.rel, tt.old, tt.new); got != tt.want {
			t.Errorf("RenameRelPath(%q, %q, %q) = %q, want %q", tt.rel, tt.old, tt.new, got, tt.want)
		}
	}
}

func TestDeleteFileAndRenameEntityFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Services/ProductService.cs")

	if _, renamed, err := RenameEntityFile(dir, "src/Services/ProductService.cs", "Product", "Item"); err != nil {
		t.Fatalf("RenameEntityFile() error = %v", err)
	} else if !renamed {
		t.Fatal("expected file to be renamed")
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "Services", "ItemService.cs")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "Services", "ProductService.cs")); err == nil {
		t.Error("old file still exists after rename")
	}

	if err := DeleteFile(dir, "src/Services/ItemService.cs"); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "Services", "ItemService.cs")); err == nil {
		t.Error("file still exists after delete")
	}
}

func TestComputeDiffNamespaceRename(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Application/Services/UserService.cs")
	writeTestFile(t, dir, "src/WebApi/Program.cs")
	// Custom file carries the OLD root namespace; Program.cs is overwritten by the
	// generator so it gets the new namespace and must NOT be flagged.
	abs := filepath.Join(dir, "src", "Application", "Services", "UserService.cs")
	oldContent := "namespace EscrowPay.Application.Services;\n"
	if err := os.WriteFile(abs, []byte(oldContent), 0o644); err != nil {
		t.Fatalf("write custom service: %v", err)
	}

	old := &Snapshot{
		ProjectNamespace: "EscrowPay",
		Files: []renderer.RenderedFile{
			{Path: "src/Application/Services/UserService.cs", Entity: "User", Custom: true, Written: true},
			{Path: "src/WebApi/Program.cs", Written: true},
		},
	}

	project := testProject("User")
	project.Name = "EscrowApi"
	diff := ComputeDiff(old, project, dir)
	if diff.NamespaceRename == nil {
		t.Fatal("expected a namespace rename warning")
	}
	if diff.NamespaceRename.OldNamespace != "EscrowPay" || diff.NamespaceRename.NewNamespace != "EscrowApi" {
		t.Errorf("mismatch namespaces = %q -> %q", diff.NamespaceRename.OldNamespace, diff.NamespaceRename.NewNamespace)
	}
	if len(diff.NamespaceRename.Files) != 1 || diff.NamespaceRename.Files[0] != "src/Application/Services/UserService.cs" {
		t.Errorf("affected files = %v, want only the custom UserService.cs", diff.NamespaceRename.Files)
	}
	if diff.NamespaceRenameReport() == "" {
		t.Error("NamespaceRenameReport() should be non-empty")
	}
}

func TestComputeDiffFieldRename(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Application/Services/ProductService.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product": {
				Fields:        map[string]string{"id": "uuid", "title": "string"},
				FieldOldNames: map[string]string{},
			},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Application/Services/ProductService.cs", Entity: "Product", Custom: true},
		},
	}

	// name: string [required, old_name: title] — title -> name, same string type.
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Product",
				NamePlural: "Products",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{
						Name:                  "name",
						OldName:               "title",
						DatabaseType:          "string",
						DatabaseColumnName:    "name",
						OldDatabaseColumnName: "title",
					},
				},
			},
		},
	}

	diff := ComputeDiff(old, project, dir)
	if len(diff.FieldRenames) != 1 {
		t.Fatalf("got %d field renames, want 1", len(diff.FieldRenames))
	}
	fr := diff.FieldRenames[0]
	if fr.Entity != "Product" || fr.OldField != "title" || fr.NewField != "name" {
		t.Errorf("field rename = %+v, want Product title->name", fr)
	}
	if fr.OldColumn != "title" || fr.NewColumn != "name" {
		t.Errorf("columns = %s -> %s, want title -> name", fr.OldColumn, fr.NewColumn)
	}
	if len(fr.CustomFiles) != 1 || fr.CustomFiles[0] != "src/Application/Services/ProductService.cs" {
		t.Errorf("custom files = %v, want [src/Application/Services/ProductService.cs]", fr.CustomFiles)
	}
	report := diff.FieldRenameReport()
	if report == "" {
		t.Error("FieldRenameReport() should be non-empty")
	}
	if diff.IsEmpty() {
		t.Error("diff with a field rename must not be empty")
	}
}

func TestComputeDiffChainedFieldRename(t *testing.T) {
	// The developer renamed title -> name, generated, but NEVER applied the
	// migration. Then they renamed name -> description and generated again. The
	// previous snapshot shows that `name` was itself a rename (of `title`), so the
	// new rename must be flagged as chained so the report can warn that the DB
	// still holds the original column (`title`), not `name`.
	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product": {
				Fields:        map[string]string{"id": "uuid", "name": "string"},
				FieldOldNames: map[string]string{"name": "title"},
			},
		},
	}

	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Product",
				NamePlural: "Products",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{
						Name:                  "description",
						OldName:               "name",
						DatabaseType:          "string",
						DatabaseColumnName:    "description",
						OldDatabaseColumnName: "name",
					},
				},
			},
		},
	}

	diff := ComputeDiff(old, project, t.TempDir())
	if len(diff.FieldRenames) != 1 {
		t.Fatalf("got %d field renames, want 1", len(diff.FieldRenames))
	}
	fr := diff.FieldRenames[0]
	if !fr.Chained {
		t.Error("FieldRename.Chained = false, want true (source field was itself a rename)")
	}
	report := diff.FieldRenameReport()
	if !strings.Contains(report, "WARNING") || !strings.Contains(report, "may never have been applied") {
		t.Errorf("chained rename report should warn about the unapplied chain, got:\n%s", report)
	}
}

func TestComputeDiffFieldRenameSkipsTypeChangeOnly(t *testing.T) {
	dir := t.TempDir()

	// Field keeps its name but the type changes — that is a type change, not a rename.
	old := &Snapshot{
		Entities: map[string]EntityState{"Product": {Fields: map[string]string{"price": "int"}}},
	}
	project := testProject("Product")
	project.Entities[0].Fields = []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}, {Name: "price", DatabaseType: "decimal", DatabaseColumnName: "price"}}

	diff := ComputeDiff(old, project, dir)
	if len(diff.FieldRenames) != 0 {
		t.Fatalf("got %d field renames, want 0", len(diff.FieldRenames))
	}
	if len(diff.TypeChanges) != 1 {
		t.Fatalf("got %d type changes, want 1", len(diff.TypeChanges))
	}
}

func TestComputeDiffDeletedEntityWithoutFiles(t *testing.T) {
	dir := t.TempDir()
	// Files are NOT created on disk — should filter them out.

	old := &Snapshot{
		Entities: map[string]EntityState{"Product": {Fields: map[string]string{"id": "uuid"}}},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Product.cs", Entity: "Product"},
			{Path: "src/Services/ProductService.cs", Entity: "Product", Custom: true},
		},
	}
	project := testProject("User")

	diff := ComputeDiff(old, project, dir)
	if len(diff.Deleted) != 1 {
		t.Fatalf("got %d deleted entities, want 1", len(diff.Deleted))
	}
	if len(diff.Deleted[0].Files) != 0 {
		t.Errorf("got %d files (should be 0 — none exist on disk)", len(diff.Deleted[0].Files))
	}
}

func TestComputeDiffMultipleDeletedEntities(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Domain/Entities/Product.cs")
	writeTestFile(t, dir, "src/Domain/Entities/Category.cs")
	writeTestFile(t, dir, "src/Domain/Entities/Tag.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product":  {Fields: map[string]string{"id": "uuid"}},
			"Category": {Fields: map[string]string{"id": "uuid"}},
			"Tag":      {Fields: map[string]string{"id": "uuid"}},
			"User":     {Fields: map[string]string{"id": "uuid"}},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Product.cs", Entity: "Product"},
			{Path: "src/Domain/Entities/Category.cs", Entity: "Category"},
			{Path: "src/Domain/Entities/Tag.cs", Entity: "Tag"},
			{Path: "src/Domain/Entities/User.cs", Entity: "User"},
		},
	}
	// Only User survives.
	project := testProject("User")

	diff := ComputeDiff(old, project, dir)
	if len(diff.Deleted) != 3 {
		t.Fatalf("got %d deleted entities, want 3 (Product, Category, Tag)", len(diff.Deleted))
	}
	// Product, Category, Tag in alphabetical order.
	if diff.Deleted[0].Name != "Category" {
		t.Errorf("first deleted = %q, want Category", diff.Deleted[0].Name)
	}
	if diff.Deleted[1].Name != "Product" {
		t.Errorf("second deleted = %q, want Product", diff.Deleted[1].Name)
	}
	if diff.Deleted[2].Name != "Tag" {
		t.Errorf("third deleted = %q, want Tag", diff.Deleted[2].Name)
	}
	// User is not deleted.
	if !diff.HasSchemaChanges() {
		t.Error("HasSchemaChanges() should be true when entities are deleted")
	}
	if diff.IsEmpty() {
		t.Error("IsEmpty() should be false when entities are deleted")
	}
}

func TestComputeDiffEntityRenameOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Domain/Entities/Product.cs")
	writeTestFile(t, dir, "src/Services/ProductService.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product": {Fields: map[string]string{"id": "uuid", "name": "string"}},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Product.cs", Entity: "Product"},
			{Path: "src/Services/ProductService.cs", Entity: "Product", Custom: true},
		},
	}

	// Product -> Item, fields unchanged.
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Item",
				OldName:    "Product",
				NamePlural: "Items",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "name", DatabaseType: "string"},
				},
			},
		},
	}

	diff := ComputeDiff(old, project, dir)
	if len(diff.Renamed) != 1 {
		t.Fatalf("got %d renames, want 1", len(diff.Renamed))
	}
	ren := diff.Renamed[0]
	if ren.OldName != "Product" || ren.NewName != "Item" {
		t.Errorf("rename = %q -> %q, want Product -> Item", ren.OldName, ren.NewName)
	}
	if len(ren.Files) != 2 {
		t.Fatalf("got %d files for rename, want 2", len(ren.Files))
	}
	if len(diff.Deleted) != 0 {
		t.Errorf("got %d deleted entities, want 0", len(diff.Deleted))
	}
	if len(diff.TypeChanges) != 0 {
		t.Errorf("got %d type changes, want 0", len(diff.TypeChanges))
	}
	if len(diff.FieldRenames) != 0 {
		t.Errorf("got %d field renames, want 0", len(diff.FieldRenames))
	}
}

func TestComputeDiffEntityRenameWithoutFiles(t *testing.T) {
	dir := t.TempDir()
	// No files created on disk.

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product": {Fields: map[string]string{"id": "uuid"}},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Product.cs", Entity: "Product"},
		},
	}

	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Item",
				OldName:    "Product",
				NamePlural: "Items",
				Fields:     []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}},
			},
		},
	}

	diff := ComputeDiff(old, project, dir)
	if len(diff.Renamed) != 1 {
		t.Fatalf("got %d renames, want 1", len(diff.Renamed))
	}
	if len(diff.Renamed[0].Files) != 0 {
		t.Errorf("got %d files for rename, want 0 (files do not exist on disk)", len(diff.Renamed[0].Files))
	}
}

func TestComputeDiffFieldRenameWithTypeChange(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Services/ProductService.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product": {
				Fields:        map[string]string{"id": "uuid", "title": "string"},
				FieldOldNames: map[string]string{},
			},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Services/ProductService.cs", Entity: "Product", Custom: true},
		},
	}

	// title: string -> name: text [old_name: title] — rename AND type change.
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Product",
				NamePlural: "Products",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{
						Name:                  "name",
						OldName:               "title",
						DatabaseType:          "text",
						DatabaseColumnName:    "name",
						OldDatabaseColumnName: "title",
					},
				},
			},
		},
	}

	diff := ComputeDiff(old, project, dir)
	if len(diff.FieldRenames) != 1 {
		t.Fatalf("got %d field renames, want 1", len(diff.FieldRenames))
	}
	fr := diff.FieldRenames[0]
	if fr.Entity != "Product" || fr.OldField != "title" || fr.NewField != "name" {
		t.Errorf("field rename = %+v, want Product title->name", fr)
	}
	if fr.OldType != "string" || fr.NewType != "text" {
		t.Errorf("types = %s -> %s, want string -> text", fr.OldType, fr.NewType)
	}
	if fr.Chained {
		t.Error("Chained should be false (first rename)")
	}
	if len(fr.CustomFiles) != 1 || fr.CustomFiles[0] != "src/Services/ProductService.cs" {
		t.Errorf("custom files = %v, want [src/Services/ProductService.cs]", fr.CustomFiles)
	}

	// A field rename should NOT also produce a type change entry.
	if len(diff.TypeChanges) != 0 {
		t.Errorf("got %d type changes, want 0 (should be covered by the field rename)", len(diff.TypeChanges))
	}
}

func TestComputeDiffSimultaneousChanges(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Domain/Entities/Item.cs")
	writeTestFile(t, dir, "src/Services/ItemService.cs")
	writeTestFile(t, dir, "src/Domain/Entities/Category.cs")
	writeTestFile(t, dir, "src/Domain/Entities/User.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product":  {Fields: map[string]string{"id": "uuid", "title": "string", "price": "int"}},
			"Category": {Fields: map[string]string{"id": "uuid", "name": "string"}},
			"User":     {Fields: map[string]string{"id": "uuid", "email": "string"}},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Item.cs", Entity: "Product"},
			{Path: "src/Services/ItemService.cs", Entity: "Product", Custom: true},
			{Path: "src/Domain/Entities/Category.cs", Entity: "Category"},
			{Path: "src/Domain/Entities/User.cs", Entity: "User"},
		},
	}

	// Product -> Item (rename), title -> name (field rename), price int -> decimal (type change).
	// Category: deleted.
	// User: unchanged.
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Item",
				OldName:    "Product",
				NamePlural: "Items",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{
						Name:                  "name",
						OldName:               "title",
						DatabaseType:          "string",
						DatabaseColumnName:    "name",
						OldDatabaseColumnName: "title",
					},
					{Name: "price", DatabaseType: "decimal", DatabaseColumnName: "price"},
				},
			},
			{
				Name:       "User",
				NamePlural: "Users",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "email", DatabaseType: "string"},
				},
			},
		},
	}

	diff := ComputeDiff(old, project, dir)

	// 1 rename (Product -> Item).
	if len(diff.Renamed) != 1 {
		t.Errorf("got %d renames, want 1", len(diff.Renamed))
	} else {
		if diff.Renamed[0].OldName != "Product" || diff.Renamed[0].NewName != "Item" {
			t.Errorf("rename = %q -> %q, want Product -> Item", diff.Renamed[0].OldName, diff.Renamed[0].NewName)
		}
	}

	// 1 deleted (Category).
	if len(diff.Deleted) != 1 {
		t.Errorf("got %d deleted entities, want 1", len(diff.Deleted))
	} else {
		if diff.Deleted[0].Name != "Category" {
			t.Errorf("deleted = %q, want Category", diff.Deleted[0].Name)
		}
	}

	// 1 type change (price: int -> decimal).
	if len(diff.TypeChanges) != 1 {
		t.Errorf("got %d type changes, want 1", len(diff.TypeChanges))
	} else {
		tc := diff.TypeChanges[0]
		if tc.Entity != "Item" || tc.Field != "price" || tc.OldType != "int" || tc.NewType != "decimal" {
			t.Errorf("type change = %+v, want Item.price int->decimal", tc)
		}
	}

	// 1 field rename (title -> name).
	if len(diff.FieldRenames) != 1 {
		t.Errorf("got %d field renames, want 1", len(diff.FieldRenames))
	} else {
		fr := diff.FieldRenames[0]
		if fr.Entity != "Item" || fr.OldField != "title" || fr.NewField != "name" {
			t.Errorf("field rename = %+v, want Item title->name", fr)
		}
	}

	if !diff.HasSchemaChanges() {
		t.Error("HasSchemaChanges() should be true with multiple changes")
	}
	if diff.IsEmpty() {
		t.Error("IsEmpty() should be false with changes")
	}
}

func TestComputeDiffMultipleRenames(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/Domain/Entities/Item.cs")
	writeTestFile(t, dir, "src/Domain/Entities/Group.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{
			"Product":  {Fields: map[string]string{"id": "uuid"}},
			"Category": {Fields: map[string]string{"id": "uuid"}},
			"User":     {Fields: map[string]string{"id": "uuid"}},
		},
		Files: []renderer.RenderedFile{
			{Path: "src/Domain/Entities/Item.cs", Entity: "Product"},
			{Path: "src/Domain/Entities/Group.cs", Entity: "Category"},
			{Path: "src/Domain/Entities/User.cs", Entity: "User"},
		},
	}

	// Product -> Item, Category -> Group, User unchanged.
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Item",
				OldName:    "Product",
				NamePlural: "Items",
				Fields:     []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}},
			},
			{
				Name:       "Group",
				OldName:    "Category",
				NamePlural: "Groups",
				Fields:     []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}},
			},
			{
				Name:       "User",
				NamePlural: "Users",
				Fields:     []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}},
			},
		},
	}

	diff := ComputeDiff(old, project, dir)
	if len(diff.Renamed) != 2 {
		t.Fatalf("got %d renames, want 2", len(diff.Renamed))
	}
	// Sorted by OldName: Category -> Group, then Product -> Item.
	if diff.Renamed[0].OldName != "Category" || diff.Renamed[0].NewName != "Group" {
		t.Errorf("first rename = %q -> %q, want Category -> Group", diff.Renamed[0].OldName, diff.Renamed[0].NewName)
	}
	if diff.Renamed[1].OldName != "Product" || diff.Renamed[1].NewName != "Item" {
		t.Errorf("second rename = %q -> %q, want Product -> Item", diff.Renamed[1].OldName, diff.Renamed[1].NewName)
	}
	if len(diff.Deleted) != 0 {
		t.Errorf("got %d deleted entities, want 0", len(diff.Deleted))
	}
}

func TestComputeDiffExistingFileFiltersDeleted(t *testing.T) {
	dir := t.TempDir()
	// Only create A.cs — B.cs does not exist on disk.
	writeTestFile(t, dir, "src/Services/A.cs")

	old := &Snapshot{
		Entities: map[string]EntityState{"Product": {Fields: map[string]string{"id": "uuid"}}},
		Files: []renderer.RenderedFile{
			{Path: "src/Services/A.cs", Entity: "Product"},
			{Path: "src/Services/B.cs", Entity: "Product", Custom: true},
		},
	}
	project := testProject("User")

	diff := ComputeDiff(old, project, dir)
	if len(diff.Deleted) != 1 {
		t.Fatalf("got %d deleted entities, want 1", len(diff.Deleted))
	}
	if len(diff.Deleted[0].Files) != 1 {
		t.Fatalf("got %d files, want 1 (only A.cs exists on disk)", len(diff.Deleted[0].Files))
	}
	if diff.Deleted[0].Files[0].Path != "src/Services/A.cs" {
		t.Errorf("file path = %q, want src/Services/A.cs", diff.Deleted[0].Files[0].Path)
	}
	if diff.Deleted[0].Files[0].Custom {
		t.Error("A.cs should not be marked custom")
	}
}

func testProject(entityName string) *ir.IRProject {
	return &ir.IRProject{
		Entities: []ir.IREntity{
			{Name: entityName, NamePlural: entityName + "s", Fields: []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}}},
		},
	}
}

func writeTestFile(t *testing.T, root, rel string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte("// generated\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestTypeChangeReport(t *testing.T) {
	t.Run("empty diff yields empty report", func(t *testing.T) {
		d := &Diff{}
		if got := d.TypeChangeReport(); got != "" {
			t.Errorf("report = %q, want empty", got)
		}
	})

	t.Run("lists changes and custom files", func(t *testing.T) {
		d := &Diff{TypeChanges: []TypeChange{
			{Entity: "Product", Field: "price", OldType: "decimal", NewType: "int", CustomFiles: []string{"src/Services/ProductService.cs"}},
			{Entity: "Order", Field: "note", OldType: "text", NewType: "string"},
		}}

		report := d.TypeChangeReport()

		for _, want := range []string{
			"Product (Field: price)", "decimal -> int",
			"src/Services/ProductService.cs",
			"Order (Field: note)",
			"no custom files recorded",
		} {
			if !strings.Contains(report, want) {
				t.Errorf("report missing %q:\n%s", want, report)
			}
		}
	})
}
