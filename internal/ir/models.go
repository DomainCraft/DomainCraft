package ir

import "github.com/DomainCraft/DomainCraft/internal/specmeta"

// IRProject represents the intermediate project model.
type IRProject struct {
	Name        string
	Description string
	Database    string
	Auth        *IRAuthConfig
	APIStyle    string
	Platform    string // target platform version (e.g. "net9.0"), passed through to templates
	Enums       map[string][]string
	Entities    []IREntity
	Cache       *IRCacheConfig
	CORS        *IRCORSConfig
	Deploy      *IRDeployConfig
}

// IRAuthConfig represents authentication configuration in IR.
type IRAuthConfig struct {
	Type      string
	Entity    string // resolved entity name
	Roles     []string
	Endpoints IRAuthEndpoints
}

// IRAuthEndpoints controls which auth endpoints are generated.
type IRAuthEndpoints struct {
	HasLogin    bool
	HasRegister bool
	HasMe       bool
}

// HasAuth returns true if authentication is enabled.
func (p *IRProject) HasAuth() bool {
	return p.Auth != nil && p.Auth.Type != "" && p.Auth.Type != "none"
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

// IRDeployConfig represents deployment configuration in IR.
type IRDeployConfig struct {
	Domain string // API domain (e.g. "localhost", "api.example.com")
	Port   int    // exposed port (default: 8080)
}

// IREntity represents an entity in IR.
type IREntity struct {
	Name         string
	OldName      string // previous entity name (rename hint for the migration engine)
	NamePlural   string
	Features     map[string]bool
	Fields       []IRField
	RelationsOut []IRRelation
	RelationsIn  []IRRelation
	Indexes      []IRIndex
	Seed         []map[string]interface{}
	Permissions  *IRPermissions
}

// IRField represents a field in IR.
type IRField struct {
	Name               string
	DatabaseType       string
	DatabaseColumnName string // snake_case column name (computed once, used by templates)
	NavigationName     string // resolved navigation property name (for relation fields)
	IsPrimary          bool
	IsNullable         bool
	IsUnique           bool
	IsHidden           bool
	IsRelation         bool
	IsMany             bool
	RelationTarget     string
	DefaultValue       string
	DefaultIsFunc      bool
	Validations        []IRValidation
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
	if e.Features != nil {
		return e.Features[name]
	}
	return false
}

// HasAudit returns true if the entity has the audit feature enabled.
func (e IREntity) HasAudit() bool { return e.HasFeature("audit") }

// HasAuditLog returns true if the entity has the audit_log feature enabled.
func (e IREntity) HasAuditLog() bool { return e.HasFeature("audit_log") }

// HasSoftDelete returns true if the entity has the soft_delete feature enabled.
func (e IREntity) HasSoftDelete() bool { return e.HasFeature("soft_delete") }

// HasOptimisticLock returns true if the entity has the optimistic_lock feature enabled.
func (e IREntity) HasOptimisticLock() bool { return e.HasFeature("optimistic_lock") }

// IsArray returns true if the field's DatabaseType is an array type (e.g. "array(int)").
func (f IRField) IsArray() bool {
	return specmeta.IsArrayType(f.DatabaseType)
}

// ArrayElementType returns the inner type of an array field (e.g. "int" from "array(int)").
// Returns empty string if the field is not an array.
func (f IRField) ArrayElementType() string {
	if !f.IsArray() {
		return ""
	}
	return specmeta.ParseArrayInner(f.DatabaseType)
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
	Read   []string
	Create []string
	Update []string
	Delete []string
}
