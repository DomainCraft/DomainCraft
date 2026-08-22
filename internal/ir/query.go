package ir

import (
	"cmp"
	"slices"

	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// This file holds the whole list-query contract (spec/query.md): search, sort
// and filter are three peers of the same query, not a filter-first API. The core
// emits the *schema* (what a bridge may search/sort/filter), and a bridge parses
// and validates the query strings at runtime against that schema.

// FilterOp is a comparison operator in the filter grammar. The set is fixed by
// the core so every bridge serializes the same vocabulary.
type FilterOp string

const (
	OpEq         FilterOp = "eq"
	OpNe         FilterOp = "ne"
	OpGt         FilterOp = "gt"
	OpGte        FilterOp = "gte"
	OpLt         FilterOp = "lt"
	OpLte        FilterOp = "lte"
	OpIn         FilterOp = "in"
	OpContains   FilterOp = "contains"
	OpStartsWith FilterOp = "startsWith"
	OpEndsWith   FilterOp = "endsWith"
)

// AllFilterOperators returns the complete, canonical filter-operator vocabulary
// in a deterministic order. The set is fixed by the core (spec/query.md) so
// every bridge serializes the same operator names. A bridge iterates this to
// emit its runtime operator enum (member names via `pascalcase`) instead of
// hand-writing a parallel vocabulary that can drift out of sync with the core.
func (IRProject) AllFilterOperators() []FilterOp {
	return []FilterOp{OpEq, OpNe, OpGt, OpGte, OpLt, OpLte, OpIn, OpContains, OpStartsWith, OpEndsWith}
}

// jsonTextOperators is the text-semantics operator set allowed on any JSON
// leaf: equality/set/string predicates compare the extracted value as text.
func jsonTextOperators() []FilterOp {
	return []FilterOp{OpEq, OpNe, OpIn, OpContains, OpStartsWith, OpEndsWith}
}

// jsonFilterOperators is the operator set for a JSON path leaf. The value type
// of a JSON leaf is dynamic, so the core cannot coerce it statically. Ordered
// operators (gt/gte/lt/lte) carry numeric semantics (the bridge casts the
// extracted value) and are only valid on a `jsonb` column — `json` stores an
// opaque, untyped document, so only text-semantics operators are allowed there.
func jsonFilterOperators(dbType string) []FilterOp {
	if dbType == "jsonb" {
		return []FilterOp{OpEq, OpNe, OpGt, OpGte, OpLt, OpLte, OpIn, OpContains, OpStartsWith, OpEndsWith}
	}
	return jsonTextOperators()
}

// FilterPathSegmentKind classifies one step of a resolved filter path.
type FilterPathSegmentKind string

const (
	// SegmentScalar is the terminal scalar leaf (e.g. "price").
	SegmentScalar FilterPathSegmentKind = "scalar"
	// SegmentRelation is a to-one relation navigation (e.g. "category").
	SegmentRelation FilterPathSegmentKind = "relation"
	// SegmentJSON is a JSON field whose remaining keys follow (e.g. "meta" in
	// "meta.price").
	SegmentJSON FilterPathSegmentKind = "json"
)

// FilterOperators returns the comparison operators a field's type supports, in
// a canonical order. Bridges use this to emit a field's allowed-filter map
// without re-deriving which operator is valid for which type.
func (f IRField) FilterOperators() []FilterOp {
	switch {
	case f.IsText():
		return []FilterOp{OpEq, OpNe, OpContains, OpStartsWith, OpEndsWith, OpIn}
	case f.IsBoolean():
		return []FilterOp{OpEq, OpNe}
	case f.IsInteger() || f.IsFloat() || f.IsDate() || f.IsDateTime():
		return []FilterOp{OpEq, OpNe, OpGt, OpGte, OpLt, OpLte, OpIn}
	case f.IsUuid(), f.IsEnum():
		return []FilterOp{OpEq, OpNe, OpIn}
	default:
		return nil
	}
}

// SearchableFields returns the scalar text fields eligible for a free-text
// `search` (case-insensitive contains across each). Relations, enums, JSON
// blobs, arrays, hidden and feature fields are excluded.
func (e IREntity) SearchableFields() []IRField {
	return e.queryFields(func(f IRField) bool {
		return f.IsText() && !f.IsFeatureField() && !f.IsPrimary && !f.IsHidden
	})
}

// SortableFields returns the scalar fields a list endpoint may order by.
// Relations, enums, JSON blobs, arrays, hidden and feature fields and the
// primary key are excluded (the PK is the default order, not an explicit key).
func (e IREntity) SortableFields() []IRField {
	return e.queryFields(func(f IRField) bool {
		return !f.IsFeatureField() && !f.IsPrimary && !f.IsRelation &&
			!f.IsEnum() && !f.IsArray() && !f.IsJson() && !f.IsHidden
	})
}

// FilterableFields returns the scalar fields that may be filtered by value:
// every scalar type (including enums) except feature fields, the primary key,
// relations, arrays, JSON blobs and hidden fields.
func (e IREntity) FilterableFields() []IRField {
	return e.queryFields(func(f IRField) bool { return f.isFilterableScalar() })
}

func (e IREntity) queryFields(pred func(IRField) bool) []IRField {
	out := make([]IRField, 0, len(e.Fields))
	for _, f := range e.Fields {
		if pred(f) {
			out = append(out, f)
		}
	}
	return out
}

// isFilterableScalar reports whether the field is a scalar eligible for value
// filtering (not a feature field, primary key, relation, array, JSON blob or
// hidden field). It is the shared predicate behind FilterableFields.
func (f IRField) isFilterableScalar() bool {
	return !f.IsFeatureField() && !f.IsPrimary && !f.IsRelation &&
		!f.IsArray() && !f.IsJson() && !f.IsHidden
}

// FilterPathSpec describes one filterable path for bridge schema generation.
// It is the closed, deterministic enumeration a bridge emits as its runtime
// validation map: scalars, one-hop relation paths and JSON roots (whose keys
// are open-ended).
type FilterPathSpec struct {
	// Path is the dotted path (e.g. "sku", "category.name", "meta").
	Path string
	// Kind is "scalar", "relation" or "json".
	Kind FilterPathSegmentKind
	// Operators is the allowed operator set for this path.
	Operators []FilterOp
}

// FilterablePaths returns every filterable path for the entity: scalar fields,
// one-hop to-one relation paths (e.g. "category.name") and JSON roots (e.g.
// "meta", whose key path is open-ended). Output is sorted by path for
// deterministic rendering.
func (e IREntity) FilterablePaths() []FilterPathSpec {
	var out []FilterPathSpec

	for _, f := range e.FilterableFields() {
		out = append(out, FilterPathSpec{Path: f.Name, Kind: SegmentScalar, Operators: f.FilterOperators()})
	}

	// One-hop relation paths: to-one relations only, leaf = filterable scalar on
	// the target. The path uses the camelCase navigation name (e.g. "category").
	for _, r := range e.RelationsOut {
		if r.IsMany || r.TargetEntity == nil {
			continue
		}
		seg := textutil.CamelCase(r.NavigationName)
		for _, tf := range r.TargetEntity.FilterableFields() {
			out = append(out, FilterPathSpec{
				Path:      seg + "." + tf.Name,
				Kind:      SegmentRelation,
				Operators: tf.FilterOperators(),
			})
		}
	}

	// JSON roots: the field itself, with the operator set its column type allows
	// (jsonb → full incl. ordered/numeric; json → text-only). The bridge treats
	// any "root.key[.key...]" path as valid.
	for _, f := range e.Fields {
		if f.IsJson() && !f.IsHidden {
			out = append(out, FilterPathSpec{Path: f.Name, Kind: SegmentJSON, Operators: jsonFilterOperators(f.DatabaseType)})
		}
	}

	slices.SortFunc(out, func(a, b FilterPathSpec) int { return cmp.Compare(a.Path, b.Path) })
	return out
}

// QuerySchema bundles the entity's complete list-query surface — searchable,
// sortable and filterable columns — into one object. It is the single entry
// point a bridge consumes to render its runtime query validator; search, sort
// and filter are peers of the same list-query contract (spec/query.md), not a
// filter-first API.
type QuerySchema struct {
	Searchable []IRField
	Sortable   []IRField
	Filterable []FilterPathSpec
}

// CursorField returns the field used as the keyset-pagination cursor: the
// primary key when it is a monotonic integer (int/bigint), which a bridge can
// compare with `>` and order by. It returns nil for uuid/text/date keys, where
// keyset comparison is not reliably translatable across providers — those
// entities fall back to offset pagination.
func (e IREntity) CursorField() *IRField {
	pk := e.PrimaryKey()
	if pk == nil {
		return nil
	}
	if pk.IsInteger() || pk.IsBigInt() {
		return pk
	}
	return nil
}

// QuerySchema returns the entity's full query surface in one object. Bridges
// iterate this instead of calling SearchableFields/SortableFields/FilterablePaths
// separately.
func (e IREntity) QuerySchema() QuerySchema {
	return QuerySchema{
		Searchable: e.SearchableFields(),
		Sortable:   e.SortableFields(),
		Filterable: e.FilterablePaths(),
	}
}
