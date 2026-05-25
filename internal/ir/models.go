package ir

import "strings"

// IRProject represents the intermediate project model.
type IRProject struct {
	Name     string
	Database string
	Auth     string
	APIStyle string
	Platform string // target platform version (e.g. "net9.0"), passed through to templates
	Enums    map[string][]string
	Entities []IREntity
	Cache    *IRCacheConfig
	CORS     *IRCORSConfig
}

// IRCacheConfig represents cache configuration in IR.
type IRCacheConfig struct {
	Enabled          bool
	Provider         string
	ConnectionString string
	TTLSeconds       int
}

// IRCORSConfig represents CORS configuration in IR.
type IRCORSConfig struct {
	Enabled bool
	Origins []string
}

// IREntity represents an entity in IR.
type IREntity struct {
	Name              string
	NamePlural        string
	HasAudit          bool
	HasAuditLog       bool
	HasSoftDelete     bool
	HasOptimisticLock bool
	Fields            []IRField
	RelationsOut      []IRRelation
	RelationsIn       []IRRelation
	Indexes           []IRIndex
	Seed              []map[string]interface{}
	Permissions       *IRPermissions
}

// IRField represents a field in IR.
type IRField struct {
	Name           string
	DatabaseType   string
	NavigationName string // resolved navigation property name (for relation fields)
	IsPrimary      bool
	IsNullable     bool
	IsUnique       bool
	IsHidden       bool
	IsRelation     bool
	IsMany         bool
	RelationTarget string
	DefaultValue   string
	DefaultIsFunc  bool
	Validations    []IRValidation
}

// NonRelationFields returns only scalar/non-relation fields (excludes relation FK fields).
func (e IREntity) NonRelationFields() []IRField {
	var result []IRField
	for _, f := range e.Fields {
		if !f.IsRelation {
			result = append(result, f)
		}
	}
	return result
}

// RelationFields returns only relation fields.
func (e IREntity) RelationFields() []IRField {
	var result []IRField
	for _, f := range e.Fields {
		if f.IsRelation {
			result = append(result, f)
		}
	}
	return result
}

// HasFeature returns true if the entity has the named feature enabled.
func (e IREntity) HasFeature(name string) bool {
	switch name {
	case "audit":
		return e.HasAudit
	case "audit_log":
		return e.HasAuditLog
	case "soft_delete":
		return e.HasSoftDelete
	case "optimistic_lock":
		return e.HasOptimisticLock
	default:
		return false
	}
}

// IsArray returns true if the field's DatabaseType is an array type (e.g. "array(int)").
func (f IRField) IsArray() bool {
	return strings.HasPrefix(f.DatabaseType, "array(")
}

// ArrayElementType returns the inner type of an array field (e.g. "int" from "array(int)").
// Returns empty string if the field is not an array.
func (f IRField) ArrayElementType() string {
	if !f.IsArray() {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(f.DatabaseType, "array("), ")")
}

// IRValidation represents one validation rule.
type IRValidation struct {
	Name  string
	Value string
}

// IRRelation represents a relation between entities.
type IRRelation struct {
	FieldName        string
	TargetEntity     *IREntity
	NavigationName   string
	InverseNavName   string
	OnDeleteBehavior string
	IsNullable       bool
	IsMany           bool
	RelationType     string
}

// IRIndex represents an index.
type IRIndex struct {
	Name   string
	Fields []string
	Type   string
	Sort   []string
	Unique bool
}

// IRPermissions represents entity permissions in IR.
type IRPermissions struct {
	Read       []string
	Create     []string
	Update     []string
	Delete     []string
	ReadPublic string
}
