package ir

import "github.com/DomainCraft/DomainCraft/internal/specmeta"

// SeedValue is one generated or declared seed field value. It carries the
// field's IR database type and relation flags so a bridge can serialize the
// value into its own literal syntax WITHOUT re-looking-up the entity/field from
// the project (dumb bridge, smart core). Explicit `seed:` rows and
// auto-generated mock seed both use this exact shape.
type SeedValue struct {
	Field      string      // logical field name
	DBType     string      // IR database type (e.g. "uuid", "decimal", "Status", "array(int)")
	IsRelation bool        // true when the field is a relation (FK or collection)
	IsMany     bool        // true when the relation is a collection ([many])
	Value      interface{} // native Go value (string, int64, float64, bool, []interface{}, map, nil)
}

// Kind returns the language-agnostic value kind for serialization, derived from
// DBType. A bridge switches on this instead of re-deriving the kind from its own
// mapped type name (e.g. "Guid"/"DateTime"/"List<…>"). Kinds: array, uuid, text,
// int, bigint, float, decimal, bool, date, datetime, json, enum.
func (v SeedValue) Kind() string { return KindOf(v.DBType) }

// ElementType returns the element type for an array value ("array(int)" -> "int"),
// or the value's own DBType for non-array values. Bridges use it to map array
// elements without string-slicing their own array syntax.
func (v SeedValue) ElementType() string { return specmeta.ParseArrayInner(v.DBType) }

// KindOf maps an IR database type to its language-agnostic value kind.
func KindOf(dbType string) string {
	inner := specmeta.ParseArrayInner(dbType)
	switch {
	case specmeta.IsArrayType(dbType):
		return "array"
	case inner == "uuid":
		return "uuid"
	case inner == "string" || inner == "text":
		return "text"
	case inner == "int":
		return "int"
	case inner == "bigint":
		return "bigint"
	case inner == "float":
		return "float"
	case inner == "decimal":
		return "decimal"
	case inner == "boolean":
		return "bool"
	case inner == "date":
		return "date"
	case inner == "datetime":
		return "datetime"
	case inner == "json" || inner == "jsonb":
		return "json"
	case !specmeta.IsPrimitive(inner):
		return "enum"
	default:
		return "text"
	}
}

// SeedRecord is one seed row for an entity: its fields in declaration order.
type SeedRecord struct {
	Fields []SeedValue
}

// SeedEntity is the seed data for one entity.
type SeedEntity struct {
	Name    string
	Records []SeedRecord
}

// SeedDataset is a collection of seed data, one entry per entity in the
// project's topological (FK-parents-first) order.
type SeedDataset struct {
	Entities []SeedEntity
}

// Entity returns the seed entry for the named entity, or nil.
func (d *SeedDataset) Entity(name string) *SeedEntity {
	if d == nil {
		return nil
	}
	for i := range d.Entities {
		if d.Entities[i].Name == name {
			return &d.Entities[i]
		}
	}
	return nil
}
