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

var AuthTypes = []string{
	"jwt",
	"none",
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

// StringFieldTypes are the string-like scalar types.
var StringFieldTypes = []string{
	"string", "text",
}

// NumericValidationModifiers are only meaningful on numeric-type fields.
var NumericValidationModifiers = []string{
	"gte", "gt", "lte", "lt",
}

// StringValidationModifiers are only meaningful on string-type fields.
var StringValidationModifiers = []string{
	"min", "max", "email", "url", "ipv4", "regex",
}

// IndexTypes are the valid database index types.
var IndexTypes = []string{
	"btree", "hash", "gist", "gin", "brin",
}

// CacheProviders are the valid cache provider names.
var CacheProviders = []string{
	"redis", "memcached", "in-memory",
}

// MultiTenancyModes are the valid multi-tenancy modes.
var MultiTenancyModes = []string{
	"column", "schema", "database",
}

// PermissionKeys are the valid permission operation keys.
var PermissionKeys = []string{
	"read", "create", "update", "delete",
}

// SortDirections are the valid index sort directions.
var SortDirections = []string{"asc", "desc"}

// FuncDefaults maps function-style default names to the types they are valid for.
var FuncDefaults = map[string][]string{
	"now":  {"date", "datetime"},
	"uuid": {"uuid"},
}

// BooleanFieldTypes are the boolean scalar types.
var BooleanFieldTypes = []string{"boolean"}

// DefaultFieldType is the fallback IR database type when no specific type can be resolved.
const DefaultFieldType = "string"

var primitiveSet map[string]bool
var numericSet map[string]bool
var stringSet map[string]bool
var booleanSet map[string]bool
var fieldTypeSet map[string]bool
var onDeleteSet map[string]bool
var featureSet map[string]bool
var indexTypeSet map[string]bool
var cacheProviderSet map[string]bool
var multiTenancyModeSet map[string]bool
var permissionKeySet map[string]bool
var stringValidationModSet map[string]bool
var numericValidationModSet map[string]bool
var sortDirectionSet map[string]bool
var databaseSet map[string]bool
var apiStyleSet map[string]bool
var authTypeSet map[string]bool

func init() {
	primitiveSet = SliceToSet(PrimitiveFieldTypes)
	numericSet = SliceToSet(NumericFieldTypes)
	stringSet = SliceToSet(StringFieldTypes)
	booleanSet = SliceToSet(BooleanFieldTypes)
	fieldTypeSet = SliceToSet(FieldTypes)
	onDeleteSet = SliceToSet(OnDeleteValues)
	featureSet = SliceToSet(Features)
	indexTypeSet = SliceToSet(IndexTypes)
	cacheProviderSet = SliceToSet(CacheProviders)
	multiTenancyModeSet = SliceToSet(MultiTenancyModes)
	permissionKeySet = SliceToSet(PermissionKeys)
	stringValidationModSet = SliceToSet(StringValidationModifiers)
	numericValidationModSet = SliceToSet(NumericValidationModifiers)
	sortDirectionSet = SliceToSet(SortDirections)
	databaseSet = SliceToSet(Databases)
	apiStyleSet = SliceToSet(APIStyles)
	authTypeSet = SliceToSet(AuthTypes)
}

// IsPrimitive returns true if the type name is a built-in scalar (not an enum, array, or relation).
func IsPrimitive(typeName string) bool {
	return primitiveSet[typeName]
}

// IsNumeric returns true if the type name is a numeric scalar (int, bigint, float, decimal).
func IsNumeric(typeName string) bool {
	return numericSet[typeName]
}

// IsStringType returns true if the type name is a string-like scalar (string, text).
func IsStringType(typeName string) bool {
	return stringSet[typeName]
}

// IsBooleanType returns true if the type name is the boolean scalar type.
func IsBooleanType(typeName string) bool {
	return booleanSet[typeName]
}

// IsSortDirection returns true if the value is a valid sort direction.
func IsSortDirection(value string) bool {
	return sortDirectionSet[value]
}

// IsFieldType returns true if the type name is a valid field type keyword.
func IsFieldType(typeName string) bool {
	return fieldTypeSet[typeName]
}

// IsOnDeleteValue returns true if the value is a valid on_delete behavior.
func IsOnDeleteValue(value string) bool {
	return onDeleteSet[value]
}

// IsFeature returns true if the name is a valid feature.
func IsFeature(name string) bool {
	return featureSet[name]
}

// IsIndexType returns true if the value is a valid index type.
func IsIndexType(value string) bool {
	return indexTypeSet[value]
}

// IsCacheProvider returns true if the value is a valid cache provider.
func IsCacheProvider(value string) bool {
	return cacheProviderSet[value]
}

// IsMultiTenancyMode returns true if the value is a valid multi-tenancy mode.
func IsMultiTenancyMode(value string) bool {
	return multiTenancyModeSet[value]
}

// IsPermissionKey returns true if the value is a valid permission operation key.
func IsPermissionKey(value string) bool {
	return permissionKeySet[value]
}

// IsDatabase returns true if the value is a valid database type.
func IsDatabase(value string) bool {
	return databaseSet[value]
}

// IsAPIStyle returns true if the value is a valid API style.
func IsAPIStyle(value string) bool {
	return apiStyleSet[value]
}

// IsAuthType returns true if the value is a valid authentication type.
func IsAuthType(value string) bool {
	return authTypeSet[value]
}

// IsStringValidationModifier returns true if the modifier is only valid on string-type fields.
func IsStringValidationModifier(mod string) bool {
	return stringValidationModSet[mod]
}

// IsNumericValidationModifier returns true if the modifier is only valid on numeric-type fields.
func IsNumericValidationModifier(mod string) bool {
	return numericValidationModSet[mod]
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

// FindAuthEntity finds the first entity with both "email" and "password" fields.
// entityFields maps entity names to their field name lists.
// Returns empty string if not found.
func FindAuthEntity(entityOrder []string, entityFields map[string][]string) string {
	for _, name := range entityOrder {
		if HasEmailAndPassword(entityFields[name]) {
			return name
		}
	}
	return ""
}

// HasEmailAndPassword checks whether a field name list contains both "email" and "password".
func HasEmailAndPassword(fieldNames []string) bool {
	hasEmail := false
	hasPassword := false
	for _, f := range fieldNames {
		lower := strings.ToLower(f)
		if lower == "email" {
			hasEmail = true
		}
		if lower == "password" {
			hasPassword = true
		}
	}
	return hasEmail && hasPassword
}

// BuildEntityFields returns a map of entity name → field name order.
// Used by validator and IR builder to look up entity field lists for specmeta helpers.
func BuildEntityFields(entityOrder []string, getFields func(name string) []string) map[string][]string {
	result := make(map[string][]string, len(entityOrder))
	for _, name := range entityOrder {
		result[name] = getFields(name)
	}
	return result
}
