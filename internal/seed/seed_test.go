package seed

import (
	"strings"
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/ir"
)

func TestGenerateBasicTypes(t *testing.T) {
	project := &ir.IRProject{
		Enums: map[string][]string{"Status": {"pending", "active"}},
		Entities: []ir.IREntity{
			{
				Name:       "Product",
				NamePlural: "Products",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "name", DatabaseType: "string"},
					{Name: "price", DatabaseType: "decimal"},
					{Name: "active", DatabaseType: "boolean"},
					{Name: "status", DatabaseType: "Status"},
					{Name: "tags", DatabaseType: "array(string)"},
					{Name: "payload", DatabaseType: "jsonb"},
				},
			},
		},
	}

	ds := Generate(project, Options{CountPerEntity: 3})
	if len(ds.Entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(ds.Entities))
	}
	em := ds.Entities[0]
	if len(em.Records) != 3 {
		t.Fatalf("got %d records, want 3", len(em.Records))
	}

	first := em.Records[0].Fields
	vals := map[string]interface{}{}
	for _, sv := range first {
		vals[sv.Field] = sv.Value
	}
	if _, ok := vals["id"].(string); !ok {
		t.Errorf("id should be a string uuid, got %T", vals["id"])
	}
	if _, ok := vals["name"].(string); !ok {
		t.Errorf("name should be a string, got %T", vals["name"])
	}
	if _, ok := vals["price"].(float64); !ok {
		t.Errorf("price should be a float64, got %T", vals["price"])
	}
	if vals["active"] != true {
		t.Errorf("active = %v, want true", vals["active"])
	}
	if vals["status"] != "pending" {
		t.Errorf("status = %v, want pending (first enum value)", vals["status"])
	}
	if arr, ok := vals["tags"].([]interface{}); !ok || len(arr) != 1 {
		t.Errorf("tags should be a 1-element slice, got %#v", vals["tags"])
	}
	if _, ok := vals["payload"].(string); !ok {
		t.Errorf("payload should be a JSON string, got %T", vals["payload"])
	}
}

func TestGenerateReferencesParentPK(t *testing.T) {
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name:       "Category",
				NamePlural: "Categories",
				Fields:     []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}},
			},
			{
				Name:       "Product",
				NamePlural: "Products",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "categoryId", DatabaseType: "uuid", IsRelation: true, RelationTarget: "Category"},
					{Name: "images", DatabaseType: "uuid", IsRelation: true, IsMany: true, RelationTarget: "Image"},
				},
			},
		},
	}

	ds := Generate(project, Options{CountPerEntity: 1})
	var categoryPK interface{}
	for _, sv := range ds.Entities[0].Records[0].Fields {
		if sv.Field == "id" {
			categoryPK = sv.Value
		}
	}
	for _, sv := range ds.Entities[1].Records[0].Fields {
		switch sv.Field {
		case "categoryId":
			if sv.Value != categoryPK {
				t.Errorf("categoryId = %v, want parent PK %v", sv.Value, categoryPK)
			}
		case "images":
			if sv.Value != nil {
				t.Errorf("[many] relation should be nil, got %v", sv.Value)
			}
		}
	}
}

func TestNormalizeCoercesTypes(t *testing.T) {
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name: "Doc",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "title", DatabaseType: "string"},
					{Name: "count", DatabaseType: "int"},
					{Name: "ratio", DatabaseType: "decimal"},
					{Name: "enabled", DatabaseType: "boolean"},
					{Name: "tags", DatabaseType: "array(string)"},
				},
				Seed: []map[string]interface{}{
					{
						"id":      "550e8400-e29b-41d4-a716-446655440000",
						"title":   "hello",
						"count":   7,
						"ratio":   2.5,
						"enabled": true,
						"tags":    []interface{}{"a", "b"},
					},
				},
			},
		},
	}

	ds := Normalize(project)
	if len(ds.Entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(ds.Entities))
	}
	vals := map[string]interface{}{}
	for _, sv := range ds.Entities[0].Records[0].Fields {
		vals[sv.Field] = sv.Value
	}
	if vals["count"] != int64(7) {
		t.Errorf("count = %#v, want int64(7)", vals["count"])
	}
	if vals["ratio"] != float64(2.5) {
		t.Errorf("ratio = %#v, want float64(2.5)", vals["ratio"])
	}
	if vals["enabled"] != true {
		t.Errorf("enabled = %#v, want true", vals["enabled"])
	}
	if arr, ok := vals["tags"].([]interface{}); !ok || len(arr) != 2 || arr[0] != "a" {
		t.Errorf("tags = %#v, want [a b]", vals["tags"])
	}
}

func TestNormalizeSkipsEntitiesWithoutSeed(t *testing.T) {
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{Name: "Empty", Fields: []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}}},
			{Name: "Seeded", Fields: []ir.IRField{{Name: "id", DatabaseType: "uuid", IsPrimary: true}}, Seed: []map[string]interface{}{{"id": "x"}}},
		},
	}
	ds := Normalize(project)
	if len(ds.Entities) != 1 || ds.Entities[0].Name != "Seeded" {
		t.Fatalf("got %+v, want only Seeded", ds.Entities)
	}
}

func TestNormalizeIncludesRelationFlags(t *testing.T) {
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name: "Order",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "customerId", DatabaseType: "uuid", IsRelation: true, RelationTarget: "Customer"},
				},
				Seed: []map[string]interface{}{{"customerId": "550e8400-e29b-41d4-a716-446655440000"}},
			},
		},
	}
	ds := Normalize(project)
	sv := ds.Entities[0].Records[0].Fields[0]
	if !sv.IsRelation || sv.IsMany {
		t.Errorf("customerId should be a single FK relation, got IsRelation=%v IsMany=%v", sv.IsRelation, sv.IsMany)
	}
	if !strings.HasPrefix(sv.Value.(string), "550e8400") {
		t.Errorf("uuid value = %v, want the declared uuid string", sv.Value)
	}
}
