package specmeta

import "strings"

// FeatureFieldDef describes a field auto-injected by a feature macro.
type FeatureFieldDef struct {
	Feature       string `json:"feature"`
	Type          string `json:"type"`
	DBColumn      string `json:"dbColumn"`
	IsOptional    bool   `json:"isOptional"`
	IsFuncDefault bool   `json:"isFuncDefault"`
	DefaultValue  string `json:"defaultValue"`
}

var Databases = []string{"postgresql", "mysql", "sqlite", "mssql", "mongodb"}

var APIStyles = []string{"rest", "graphql", "grpc"}

var AuthTypes = []string{"jwt", "none"}

var Features = []string{"audit", "audit_log", "soft_delete", "optimistic_lock"}

var MetaFieldTypes = []string{"relation", "array", "enum"}

var FieldTypes = append(append([]string{}, PrimitiveFieldTypes...), MetaFieldTypes...)

var FeatureFieldDefs = map[string]FeatureFieldDef{
	"createdAt": {Feature: "audit", Type: "datetime", DBColumn: "created_at", IsFuncDefault: true, DefaultValue: "now"},
	"updatedAt": {Feature: "audit", Type: "datetime", DBColumn: "updated_at", IsFuncDefault: true, DefaultValue: "now"},
	"createdBy": {Feature: "audit_log", Type: "uuid", DBColumn: "created_by"},
	"updatedBy": {Feature: "audit_log", Type: "uuid", DBColumn: "updated_by"},
	"deletedAt": {Feature: "soft_delete", Type: "datetime", DBColumn: "deleted_at", IsOptional: true},
	"version":   {Feature: "optimistic_lock", Type: "int", DBColumn: "version", DefaultValue: "0"},
}

var OnDeleteValues = []string{"cascade", "set_null", "restrict", "no_action"}

var PrimitiveFieldTypes = []string{
	"string", "text", "int", "bigint", "float", "decimal",
	"boolean", "date", "datetime", "uuid", "json", "jsonb",
}

var NumericFieldTypes = []string{"int", "bigint", "float", "decimal"}

var StringFieldTypes = []string{"string", "text"}

var NumericValidationModifiers = []string{"gte", "gt", "lte", "lt"}

var StringValidationModifiers = []string{"min", "max", "email", "url", "ipv4", "regex"}

var IndexTypes = []string{"btree", "hash", "gist", "gin", "brin"}

var CacheProviders = []string{"redis", "memcached", "in-memory"}

var MultiTenancyModes = []string{"column", "schema", "database"}

var PermissionKeys = []string{"read", "create", "update", "delete"}

var SortDirections = []string{"asc", "desc"}

var BooleanFieldTypes = []string{"boolean"}

var FuncDefaults = map[string][]string{
	"now":  {"date", "datetime"},
	"uuid": {"uuid"},
}

const DefaultFieldType = "string"

var (
	primitiveSet            map[string]bool
	numericSet              map[string]bool
	stringSet               map[string]bool
	booleanSet              map[string]bool
	fieldTypeSet            map[string]bool
	onDeleteSet             map[string]bool
	featureSet              map[string]bool
	indexTypeSet            map[string]bool
	cacheProviderSet        map[string]bool
	multiTenancyModeSet     map[string]bool
	permissionKeySet        map[string]bool
	stringValidationModSet  map[string]bool
	numericValidationModSet map[string]bool
	sortDirectionSet        map[string]bool
	databaseSet             map[string]bool
	apiStyleSet             map[string]bool
	authTypeSet             map[string]bool
)

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

func IsPrimitive(typeName string) bool            { return primitiveSet[typeName] }
func IsNumeric(typeName string) bool              { return numericSet[typeName] }
func IsStringType(typeName string) bool           { return stringSet[typeName] }
func IsBooleanType(typeName string) bool          { return booleanSet[typeName] }
func IsSortDirection(value string) bool           { return sortDirectionSet[value] }
func IsFieldType(typeName string) bool            { return fieldTypeSet[typeName] }
func IsOnDeleteValue(value string) bool           { return onDeleteSet[value] }
func IsFeature(name string) bool                  { return featureSet[name] }
func IsIndexType(value string) bool               { return indexTypeSet[value] }
func IsCacheProvider(value string) bool           { return cacheProviderSet[value] }
func IsMultiTenancyMode(value string) bool        { return multiTenancyModeSet[value] }
func IsPermissionKey(value string) bool           { return permissionKeySet[value] }
func IsDatabase(value string) bool                { return databaseSet[value] }
func IsAPIStyle(value string) bool                { return apiStyleSet[value] }
func IsAuthType(value string) bool                { return authTypeSet[value] }
func IsStringValidationModifier(mod string) bool  { return stringValidationModSet[mod] }
func IsNumericValidationModifier(mod string) bool { return numericValidationModSet[mod] }

func SliceToSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func IsArrayType(dbType string) bool {
	return strings.HasPrefix(dbType, "array(")
}

func ParseArrayInner(dbType string) string {
	if !IsArrayType(dbType) {
		return dbType
	}
	return strings.TrimSuffix(strings.TrimPrefix(dbType, "array("), ")")
}

func FindAuthEntity(entityOrder []string, entityFields map[string][]string) string {
	for _, name := range entityOrder {
		if HasEmailAndPassword(entityFields[name]) {
			return name
		}
	}
	return ""
}

func HasEmailAndPassword(fieldNames []string) bool {
	hasEmail, hasPassword := AuthFieldState(fieldNames)
	return hasEmail && hasPassword
}

// AuthFieldState reports whether the field list contains "email" and "password"
// (case-insensitive). Used to build helpful messages and to auto-detect auth entities.
func AuthFieldState(fieldNames []string) (hasEmail, hasPassword bool) {
	for _, f := range fieldNames {
		switch strings.ToLower(f) {
		case "email":
			hasEmail = true
		case "password":
			hasPassword = true
		}
	}
	return hasEmail, hasPassword
}

func BuildEntityFields(entityOrder []string, getFields func(name string) []string) map[string][]string {
	result := make(map[string][]string, len(entityOrder))
	for _, name := range entityOrder {
		result[name] = getFields(name)
	}
	return result
}

// IsPublicPermission reports whether the role is the "*" wildcard (public access).
func IsPublicPermission(role string) bool { return role == "*" }

// IsOwnershipToken reports whether the role is an "@..." ownership token (e.g. "@Owner").
func IsOwnershipToken(role string) bool { return strings.HasPrefix(role, "@") }

// IsReservedPermissionToken reports whether the role is a special token
// ("*" public wildcard or "@..." ownership token) that is not a declared role.
func IsReservedPermissionToken(role string) bool {
	return IsPublicPermission(role) || IsOwnershipToken(role)
}
