package ir

import (
	"reflect"
	"testing"
)

func testEntity() IREntity {
	category := IREntity{
		Name: "Category",
		Fields: []IRField{
			{Name: "id", DatabaseType: "uuid", IsPrimary: true},
			{Name: "name", DatabaseType: "string"},
			{Name: "slug", DatabaseType: "string"},
			{Name: "sortOrder", DatabaseType: "int"},
		},
	}
	return IREntity{
		Name: "Product",
		Fields: []IRField{
			{Name: "id", DatabaseType: "uuid", IsPrimary: true},
			{Name: "name", DatabaseType: "string"},
			{Name: "description", DatabaseType: "text"},
			{Name: "price", DatabaseType: "decimal"},
			{Name: "stock", DatabaseType: "int"},
			{Name: "active", DatabaseType: "boolean"},
			{Name: "status", DatabaseType: "Status"},
			{Name: "sku", DatabaseType: "string"},
			{Name: "tags", DatabaseType: "array(string)"},
			{Name: "meta", DatabaseType: "jsonb"},
			{Name: "config", DatabaseType: "json"},
			{Name: "internalNotes", DatabaseType: "string", IsHidden: true},
			{Name: "categoryId", DatabaseType: "uuid", IsRelation: true, RelationTarget: "Category"},
			{Name: "createdAt", DatabaseType: "datetime"}, // feature field
		},
		RelationsOut: []IRRelation{
			{FieldName: "categoryId", TargetEntity: &category, NavigationName: "Category", IsMany: false},
		},
	}
}

func TestSearchableSortableFilterableFields(t *testing.T) {
	e := testEntity()

	names := func(fields []IRField) []string {
		var out []string
		for _, f := range fields {
			out = append(out, f.Name)
		}
		return out
	}

	if got := names(e.SearchableFields()); !reflect.DeepEqual(got, []string{"name", "description", "sku"}) {
		t.Errorf("SearchableFields = %v, want [name description sku]", got)
	}
	if got := names(e.SortableFields()); !reflect.DeepEqual(got, []string{"name", "description", "price", "stock", "active", "sku"}) {
		t.Errorf("SortableFields = %v", got)
	}
	// Filterable adds the enum; excludes arrays/json/relations/feature/primary.
	if got := names(e.FilterableFields()); !reflect.DeepEqual(got, []string{"name", "description", "price", "stock", "active", "status", "sku"}) {
		t.Errorf("FilterableFields = %v", got)
	}
}

func TestFilterOperators(t *testing.T) {
	cases := []struct {
		dbType string
		want   []FilterOp
	}{
		{"string", []FilterOp{OpEq, OpNe, OpContains, OpStartsWith, OpEndsWith, OpIn}},
		{"boolean", []FilterOp{OpEq, OpNe}},
		{"decimal", []FilterOp{OpEq, OpNe, OpGt, OpGte, OpLt, OpLte, OpIn}},
		{"uuid", []FilterOp{OpEq, OpNe, OpIn}},
		{"Status", []FilterOp{OpEq, OpNe, OpIn}},
		{"array(string)", nil},
		{"jsonb", nil},
	}
	for _, c := range cases {
		f := IRField{DatabaseType: c.dbType}
		got := f.FilterOperators()
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("FilterOperators(%q) = %v, want %v", c.dbType, got, c.want)
		}
	}
}

func TestAllFilterOperators(t *testing.T) {
	got := (IRProject{}).AllFilterOperators()
	want := []FilterOp{OpEq, OpNe, OpGt, OpGte, OpLt, OpLte, OpIn, OpContains, OpStartsWith, OpEndsWith}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllFilterOperators() = %v, want %v", got, want)
	}
}

func TestFilterablePaths(t *testing.T) {
	e := testEntity()
	paths := e.FilterablePaths()

	byPath := map[string]FilterPathSpec{}
	for _, p := range paths {
		byPath[p.Path] = p
	}

	if p, ok := byPath["sku"]; !ok || p.Kind != SegmentScalar {
		t.Errorf("sku path missing or wrong kind: %+v", p)
	}
	if p, ok := byPath["category.name"]; !ok || p.Kind != SegmentRelation {
		t.Errorf("category.name path missing or wrong kind: %+v", p)
	}
	if p, ok := byPath["meta"]; !ok || p.Kind != SegmentJSON || len(p.Operators) != 10 {
		t.Errorf("meta (jsonb) path missing or wrong: %+v", p)
	}
	if p, ok := byPath["config"]; !ok || p.Kind != SegmentJSON || len(p.Operators) != 6 {
		t.Errorf("config (json) path missing or wrong (want 6 text ops): %+v", p)
	}

	// Deterministic sorted order.
	for i := 1; i < len(paths); i++ {
		if paths[i-1].Path >= paths[i].Path {
			t.Errorf("paths not sorted: %q >= %q", paths[i-1].Path, paths[i].Path)
		}
	}
}

func TestQuerySchema(t *testing.T) {
	e := testEntity()
	q := e.QuerySchema()

	// The single entry point must match the individual lists exactly.
	if !reflect.DeepEqual(q.Searchable, e.SearchableFields()) {
		t.Errorf("QuerySchema.Searchable != SearchableFields()")
	}
	if !reflect.DeepEqual(q.Sortable, e.SortableFields()) {
		t.Errorf("QuerySchema.Sortable != SortableFields()")
	}
	if !reflect.DeepEqual(q.Filterable, e.FilterablePaths()) {
		t.Errorf("QuerySchema.Filterable != FilterablePaths()")
	}

	if len(q.Searchable) == 0 || len(q.Sortable) == 0 || len(q.Filterable) == 0 {
		t.Errorf("QuerySchema should be non-empty for the test entity: %+v", q)
	}
}

func TestCursorField(t *testing.T) {
	cases := []struct {
		pkType string
		want   bool
	}{
		{"int", true},
		{"bigint", true},
		{"uuid", false},
		{"string", false},
		{"date", false},
	}
	for _, c := range cases {
		e := IREntity{Fields: []IRField{{Name: "id", DatabaseType: c.pkType, IsPrimary: true}}}
		got := e.CursorField()
		if (got != nil) != c.want {
			t.Errorf("CursorField(pk=%s) = %v, want non-nil=%v", c.pkType, got != nil, c.want)
		}
	}

	// No PK -> nil.
	if got := (IREntity{Fields: []IRField{{Name: "name"}}}).CursorField(); got != nil {
		t.Errorf("CursorField() with no PK = %v, want nil", got)
	}
}
