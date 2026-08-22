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

func TestGenerateHonorsNumericBounds(t *testing.T) {
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name: "Bounds",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "qtyInclusiveMin", DatabaseType: "int", Validations: []ir.IRValidation{{Name: "gte", Value: "10"}}},
					{Name: "qtyExclusiveMin", DatabaseType: "int", Validations: []ir.IRValidation{{Name: "gt", Value: "5"}}},
					{Name: "qtyExclusiveMax", DatabaseType: "bigint", Validations: []ir.IRValidation{{Name: "lt", Value: "3"}}},
					{Name: "qtyClampedByExclusiveMax", DatabaseType: "int", Validations: []ir.IRValidation{{Name: "lt", Value: "1"}}},
					{Name: "priceInclusiveMax", DatabaseType: "decimal", Validations: []ir.IRValidation{{Name: "lte", Value: "0.25"}}},
				},
			},
		},
	}

	ds := Generate(project, Options{CountPerEntity: 2})
	vals := func(record int) map[string]interface{} {
		m := map[string]interface{}{}
		for _, sv := range ds.Entities[0].Records[record].Fields {
			m[sv.Field] = sv.Value
		}
		return m
	}

	r0, r1 := vals(0), vals(1)
	// The base mock value is record+1; bounds clamp it when violated, so both
	// records may legitimately land on the same clamped number.
	if r0["qtyInclusiveMin"] != int64(10) || r1["qtyInclusiveMin"] != int64(10) {
		t.Errorf("inclusive min: %v / %v, want 10 / 10 (clamped up to the bound)", r0["qtyInclusiveMin"], r1["qtyInclusiveMin"])
	}
	if r0["qtyExclusiveMin"] != int64(6) {
		t.Errorf("exclusive min = %v, want 6 (just above the bound)", r0["qtyExclusiveMin"])
	}
	if r0["qtyExclusiveMax"] != int64(1) || r1["qtyExclusiveMax"] != int64(2) {
		t.Errorf("exclusive max: %v / %v, want 1 / 2 (base values already within the bound)", r0["qtyExclusiveMax"], r1["qtyExclusiveMax"])
	}
	if r0["qtyClampedByExclusiveMax"] != int64(0) {
		t.Errorf("clamped exclusive max = %v, want 0 (just below the violated bound 1)", r0["qtyClampedByExclusiveMax"])
	}
	if r0["priceInclusiveMax"] != float64(0.25) {
		t.Errorf("inclusive max = %v, want 0.25 (clamped down)", r0["priceInclusiveMax"])
	}
}

func TestGenerateHonorsStringValidations(t *testing.T) {
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name: "Strings",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "email", DatabaseType: "string", Validations: []ir.IRValidation{{Name: "email"}}},
					{Name: "homepage", DatabaseType: "string", Validations: []ir.IRValidation{{Name: "url"}}},
					{Name: "padded", DatabaseType: "string", Validations: []ir.IRValidation{{Name: "min", Value: "30"}}},
					{Name: "short", DatabaseType: "string", Validations: []ir.IRValidation{{Name: "max", Value: "4"}}},
				},
			},
		},
	}

	ds := Generate(project, Options{CountPerEntity: 1})
	vals := map[string]interface{}{}
	for _, sv := range ds.Entities[0].Records[0].Fields {
		vals[sv.Field] = sv.Value
	}
	if vals["email"] != "mock1@example.com" {
		t.Errorf("email = %v", vals["email"])
	}
	if vals["homepage"] != "https://example.com/resource1" {
		t.Errorf("url = %v", vals["homepage"])
	}
	if s, _ := vals["padded"].(string); len(s) < 30 {
		t.Errorf("min_length padding: got %q (%d chars), want at least 30", s, len(s))
	}
	if s, _ := vals["short"].(string); len(s) > 4 {
		t.Errorf("max_length clamp: got %q (%d chars), want at most 4", s, len(s))
	}
}

func TestNormalizeCoerceEdgeCases(t *testing.T) {
	project := &ir.IRProject{
		Enums: map[string][]string{"Status": {"draft", "live"}},
		Entities: []ir.IREntity{
			{
				Name: "Edges",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "count", DatabaseType: "int"},
					{Name: "big", DatabaseType: "bigint"},
					{Name: "ratio", DatabaseType: "float"},
					{Name: "enabled", DatabaseType: "boolean"},
					{Name: "tags", DatabaseType: "array(string)"},
					{Name: "statuses", DatabaseType: "array(Status)"},
					{Name: "meta", DatabaseType: "jsonb"},
					{Name: "status", DatabaseType: "Status"},
				},
				Seed: []map[string]interface{}{
					{
						"id":       "550e8400-e29b-41d4-a716-446655440000",
						"count":    "not-a-number",
						"big":      float64(7.9),
						"ratio":    "also-not-a-number",
						"enabled":  "not-a-bool",
						"tags":     "oops-not-a-list",
						"statuses": []interface{}{"draft", "live"},
						"meta":     map[string]interface{}{"k": "v"},
						"status":   "draft",
					},
				},
			},
		},
	}

	ds := Normalize(project)
	vals := map[string]interface{}{}
	for _, sv := range ds.Entities[0].Records[0].Fields {
		vals[sv.Field] = sv.Value
	}
	if vals["count"] != int64(0) {
		t.Errorf("unparseable int = %#v, want the zero value int64(0)", vals["count"])
	}
	if vals["big"] != int64(7) {
		t.Errorf("float for bigint = %#v, want truncated int64(7)", vals["big"])
	}
	if vals["ratio"] != float64(0) {
		t.Errorf("unparseable float = %#v, want the zero value float64(0)", vals["ratio"])
	}
	if vals["enabled"] != false {
		t.Errorf("unparseable bool = %#v, want false", vals["enabled"])
	}
	if arr, ok := vals["tags"].([]interface{}); !ok || len(arr) != 0 {
		t.Errorf("non-list for an array field = %#v, want an empty slice", vals["tags"])
	}
	arr, ok := vals["statuses"].([]interface{})
	if !ok || len(arr) != 2 || arr[0] != "draft" {
		t.Errorf("enum array = %#v, want [draft live]", vals["statuses"])
	}
	if m, ok := vals["meta"].(map[string]interface{}); !ok || m["k"] != "v" {
		t.Errorf("jsonb should keep its native shape, got %#v", vals["meta"])
	}
	if vals["status"] != "draft" {
		t.Errorf("enum scalar = %#v, want the declared wire value draft", vals["status"])
	}
}

func TestGenerateUsesDeclaredDefaultOverMock(t *testing.T) {
	project := &ir.IRProject{
		Entities: []ir.IREntity{
			{
				Name: "Defaults",
				Fields: []ir.IRField{
					{Name: "id", DatabaseType: "uuid", IsPrimary: true},
					{Name: "country", DatabaseType: "string", DefaultValue: "DE"},
					{Name: "attempts", DatabaseType: "int", DefaultValue: "3"},
				},
			},
		},
	}

	ds := Generate(project, Options{CountPerEntity: 1})
	vals := map[string]interface{}{}
	for _, sv := range ds.Entities[0].Records[0].Fields {
		vals[sv.Field] = sv.Value
	}
	if vals["country"] != "DE" {
		t.Errorf("country = %v, want the declared default DE", vals["country"])
	}
	if vals["attempts"] != int64(3) {
		t.Errorf("attempts = %#v, want int64(3)", vals["attempts"])
	}
}
