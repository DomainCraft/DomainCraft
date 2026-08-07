package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteEntityContent(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"pascal standalone", "using AcmeShop.Domain.Entities; var p = new Product();", "using AcmeShop.Domain.Entities; var p = new Item();"},
		{"compound service", "class ProductService : IProductService", "class ItemService : IItemService"},
		{"camel", "var product = repo;", "var item = repo;"},
		{"plural pascal", "List<Product> Products = null;", "List<Item> Items = null;"},
		{"plural camel", "var products = new list;", "var items = new list;"},
		{"does not touch longer word", "Productivity", "Productivity"},
		{"snake token", "product_name", "item_name"},
		{"upper", "PRODUCTS", "ITEMS"},
		{"namespace untouched by entity rewrite", "namespace AcmeShop.App;", "namespace AcmeShop.App;"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(RewriteEntityContent([]byte(tc.in), "Product", "Item"))
			if got != tc.want {
				t.Errorf("RewriteEntityContent mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestRewriteFieldContent(t *testing.T) {
	in := "entity.Title == prev.Title; var title = Title; TITLE;"
	want := "entity.Name == prev.Name; var name = Name; NAME;"
	if got := string(RewriteFieldContent([]byte(in), "title", "name")); got != want {
		t.Errorf("field rewrite\ngot: %q\nwant: %q", got, want)
	}
}

func TestRewriteNamespaceContent(t *testing.T) {
	in := "namespace AcmeShop.Application.Services;\nusing AcmeShop.Domain.Entities;"
	want := "namespace AcmeStore.Application.Services;\nusing AcmeStore.Domain.Entities;"
	got := string(RewriteNamespaceContent([]byte(in), "AcmeShop", "AcmeStore"))
	if got != want {
		t.Errorf("namespace rewrite\ngot: %q\nwant: %q", got, want)
	}
}

func TestRewriteFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rel := "src/Services/ItemService.cs"
	original := "public partial class ProductService { Task<Product> GetAsync(); }"
	if err := os.MkdirAll(filepath.Join(dir, "src", "Services"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	diff := &Diff{NamespaceRename: &NamespaceMismatch{OldNamespace: "EscrowPay", NewNamespace: "EscrowNow"}}
	rewrote, err := RewriteFile(dir, rel, RenameTransforms("Product", "Item", diff)...)
	if err != nil {
		t.Fatal(err)
	}
	if !rewrote {
		t.Fatal("expected file to be rewritten")
	}
	got, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	want := "public partial class ItemService { Task<Item> GetAsync(); }"
	if string(got) != want {
		t.Errorf("round trip\ngot: %q\nwant: %q", string(got), want)
	}
}

func TestRewriteFileNoop(t *testing.T) {
	dir := t.TempDir()
	rel := "a.cs"
	content := "var unrelated = 1;"
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rewrote, err := RewriteFile(dir, rel, EntityTransform("Product", "Item"))
	if err != nil {
		t.Fatal(err)
	}
	if rewrote {
		t.Fatal("file should not be rewritten when nothing matched")
	}
	got, _ := os.ReadFile(filepath.Join(dir, rel))
	if string(got) != content {
		t.Errorf("file changed: %q", string(got))
	}
}

func TestRewriteFileMissing(t *testing.T) {
	dir := t.TempDir()
	rewrote, err := RewriteFile(dir, "does/not/exist.cs", EntityTransform("Product", "Item"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if rewrote {
		t.Fatal("missing file must not be reported as rewritten")
	}
}
