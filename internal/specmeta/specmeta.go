package specmeta

import "strings"

// FeatureFieldDef describes a field auto-injected by a feature macro.
type FeatureFieldDef struct {
	Feature      string // feature name (e.g. "audit")
	Type         string // IR type (e.g. "datetime", "uuid", "int")
	DBColumn     string // snake_case column name
	IsOptional   bool
	IsFuncDefault bool   // true if DefaultValue is a function name (e.g. "now")
	DefaultValue string
}

var Databases = []string{
	"postgresql",
	"mysql",
	"sqlite",
	"mssql",
	"mongodb",
}

var APIStyles = []string{
	"rest",
	"graphql",
	"grpc",
}

var Features = []string{
	"audit",
	"audit_log",
	"soft_delete",
	"optimistic_lock",
}

// MetaFieldTypes are the non-scalar type keywords (relation, array, enum).
// FieldTypes is derived from PrimitiveFieldTypes + MetaFieldTypes.
var MetaFieldTypes = []string{
	"relation",
	"array",
	"enum",
}

// FieldTypes is the complete list of all valid type keywords.
// Derived from PrimitiveFieldTypes + MetaFieldTypes so there is a single source of truth.
var FieldTypes = append(append([]string{}, PrimitiveFieldTypes...), MetaFieldTypes...)

// FeatureFieldNames maps auto-generated field names to the feature that creates them.
// Used by renderer's isFeatureField and parser's addFeatureFields.
var FeatureFieldNames = map[string]string{
	"createdAt": "audit",
	"updatedAt": "audit",
	"createdBy": "audit_log",
	"updatedBy": "audit_log",
	"deletedAt": "soft_delete",
	"version":   "optimistic_lock",
}

// FeatureFieldDefs is the single source of truth for auto-injected feature fields.
// Parser and renderer consume this map instead of hardcoding field definitions.
var FeatureFieldDefs = map[string]FeatureFieldDef{
	"createdAt": {Feature: "audit", Type: "datetime", DBColumn: "created_at", IsFuncDefault: true, DefaultValue: "now"},
	"updatedAt": {Feature: "audit", Type: "datetime", DBColumn: "updated_at", IsFuncDefault: true, DefaultValue: "now"},
	"createdBy": {Feature: "audit_log", Type: "uuid", DBColumn: "created_by"},
	"updatedBy": {Feature: "audit_log", Type: "uuid", DBColumn: "updated_by"},
	"deletedAt": {Feature: "soft_delete", Type: "datetime", DBColumn: "deleted_at", IsOptional: true},
	"version":   {Feature: "optimistic_lock", Type: "int", DBColumn: "version", DefaultValue: "0"},
}

var OnDeleteValues = []string{
	"cascade",
	"set_null",
	"restrict",
	"no_action",
}

// PrimitiveFieldTypes are the built-in scalar types (not enum, array, or relation).
// Used by IR builder and renderer to distinguish primitives from user-defined enums.
var PrimitiveFieldTypes = []string{
	"string", "text", "int", "bigint", "float", "decimal",
	"boolean", "date", "datetime", "uuid", "json", "jsonb",
}

// NumericFieldTypes are the numeric scalar types.
// Used by rawSeedValue and templates to distinguish numeric literals.
var NumericFieldTypes = []string{
	"int", "bigint", "float", "decimal",
}

var primitiveSet map[string]bool
var numericSet map[string]bool

func init() {
	primitiveSet = SliceToSet(PrimitiveFieldTypes)
	numericSet = SliceToSet(NumericFieldTypes)
}

// IsPrimitive returns true if the type name is a built-in scalar (not an enum, array, or relation).
func IsPrimitive(typeName string) bool {
	return primitiveSet[typeName]
}

// IsNumeric returns true if the type name is a numeric scalar (int, bigint, float, decimal).
func IsNumeric(typeName string) bool {
	return numericSet[typeName]
}

// SliceToSet converts a string slice to a set (map[string]bool) for O(1) lookups.
func SliceToSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// IsArrayType returns true if the IR type is an array(...) type.
func IsArrayType(dbType string) bool {
	return strings.HasPrefix(dbType, "array(")
}

// ParseArrayInner extracts the inner type from "array(X)" or returns the type as-is.
func ParseArrayInner(dbType string) string {
	if !IsArrayType(dbType) {
		return dbType
	}
	return strings.TrimSuffix(strings.TrimPrefix(dbType, "array("), ")")
}
