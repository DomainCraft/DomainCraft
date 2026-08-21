package ir

import (
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// IRProject represents the intermediate project model.
type IRProject struct {
	Name        string
	Description string
	Database    string
	Auth        *IRAuthConfig
	APIStyle    string
	Platform    string // target platform version (e.g. "net9.0"), passed through to templates
	// Enums maps enum name → ordered values. EnumOrder holds the sorted keys so
	// templates can iterate deterministically (Go map iteration order is random).
	Enums     map[string][]string
	EnumOrder []string
	Entities  []IREntity
	Cache     *IRCacheConfig
	CORS      *IRCORSConfig
	Deploy    *IRDeployConfig
	// Versioning carries API versioning settings (enabled flag + default version).
	Versioning *IRVersioningConfig
	// RateLimit carries the HTTP request rate limiting policy.
	RateLimit *IRRateLimitConfig
	// Pagination carries list sizing defaults (page size + cap).
	Pagination *IRPaginationConfig
	// Addons contains the infrastructure accelerators requested via the CLI
	// (e.g. {"dapr": true}). It is not derived from domain.yaml — declared
	// infrastructure lives in Infrastructure, addon templates go on/off here.
	Addons map[string]bool
	// Infrastructure holds the provider-agnostic backing services declared in
	// project.infrastructure (queue / cache / secrets). It is the input that
	// addon bridges like Dapr consume to emit concrete component manifests.
	Infrastructure *IRInfrastructure
}

// IRInfrastructure represents the project's backing services in IR.
type IRInfrastructure struct {
	Queue   string // message broker: pubsub, rabbitmq, kafka, redis, nats, in-memory
	Cache   string // distributed cache store: redis, memcached, in-memory
	Secrets string // secrets store: local, kubernetes, azure-keyvault, aws-secrets
	Storage string // object/file storage: local, s3, azure-blob, gcs
}

// HasAddon returns true if the named infrastructure addon is enabled.
func (p *IRProject) HasAddon(name string) bool {
	if p.Addons == nil {
		return false
	}
	return p.Addons[name]
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
	HasSetup    bool
}

// HasAuth returns true if authentication is enabled.
func (p *IRProject) HasAuth() bool {
	return p.Auth != nil && p.Auth.Type != "" && p.Auth.Type != "none"
}

// AuthEntity returns the resolved authentication entity (the entity that owns
// email/password and drives the login/register/me/setup flow), or nil when auth
// is disabled or the entity could not be found. Bridges use this instead of
// scanning Project.Entities for the auth entity.
func (p *IRProject) AuthEntity() *IREntity {
	if p == nil || !p.HasAuth() || p.Auth == nil {
		return nil
	}
	for i := range p.Entities {
		if p.Entities[i].Name == p.Auth.Entity {
			return &p.Entities[i]
		}
	}
	return nil
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

// Versioning enables/disables API versioning and the default version string.
type IRVersioningConfig struct {
	Enabled        bool
	DefaultVersion string
}

// RateLimitConfig represents the HTTP request rate limiting policy in IR.
type IRRateLimitConfig struct {
	Enabled       bool
	Policy        string // fixed | sliding
	PermitLimit   int
	WindowSeconds int
}

// IRPaginationConfig represents list pagination sizing in IR.
type IRPaginationConfig struct {
	DefaultPageSize int
	MaxPageSize     int
}

// HasVersioning returns true when API versioning is enabled.
func (p *IRProject) HasVersioning() bool {
	return p.Versioning != nil && p.Versioning.Enabled
}

// HasRateLimit returns true when request rate limiting is enabled.
func (p *IRProject) HasRateLimit() bool {
	return p.RateLimit != nil && p.RateLimit.Enabled
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
	OldName            string // previous field name (rename hint from `old_name:` modifier)
	DatabaseType       string
	DatabaseColumnName string // snake_case column name (computed once, used by templates)
	// OldDatabaseColumnName is the snake_case column name of OldName. Bridges use
	// it to emit a safe RenameColumn instead of DropColumn + AddColumn.
	OldDatabaseColumnName string
	NavigationName        string // resolved navigation property name (for relation fields)
	IsPrimary             bool
	IsNullable            bool
	IsUnique              bool
	IsHidden              bool
	IsReadonly            bool
	IsRelation            bool
	IsMany                bool
	RelationTarget        string
	DefaultValue          string
	DefaultIsFunc         bool
	Validations           []IRValidation
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

// EagerLoadRelations returns the relations a bridge should eager-load when
// fetching a full entity graph (list / by-id): every outgoing relation. Bridges
// iterate this instead of re-deriving the Include set, and call
// EagerLoadNavigation() on each relation for the property to Include.
func (e IREntity) EagerLoadRelations() []IRRelation {
	return e.RelationsOut
}

// EagerLoadNavigation returns the navigation property name used to Include a
// relation: the collection property for a [many] relation, the navigation
// property for a single foreign key. Bridges apply their own casing
// (pascalcase, camelcase, ...) to the returned name.
func (r IRRelation) EagerLoadNavigation() string {
	if r.IsMany {
		return r.FieldName
	}
	return r.NavigationName
}

// TableName returns the entity's snake_case database table name. It is the
// single source of truth for the table name — bridges must print this instead of
// re-deriving it with their own snake_case (the template `snakecase` diverges
// from textutil.ToDatabaseColumnName on acronyms, e.g. `IPv4Address`).
func (e IREntity) TableName() string {
	return textutil.ToDatabaseColumnName(e.NamePlural)
}

// ForeignKeyColumnName returns the snake_case DB column name of this relation's
// foreign key (fieldName + "Id", snake_cased). Bridges must print this instead
// of `fkName .FieldName | snakecase`, which diverges from the core's column-name
// algorithm on acronyms.
func (r IRRelation) ForeignKeyColumnName() string {
	return textutil.ToDatabaseColumnName(textutil.FKName(r.FieldName))
}

// ColumnName returns the field's canonical snake_case DB column name: the FK
// column name for a relation field, the parser-computed DatabaseColumnName for a
// scalar field (falling back to ToDatabaseColumnName(Name) when empty). This is
// the single source of truth a bridge or the migration engine uses so it never
// re-derives a column name with its own snake_case function.
func (f IRField) ColumnName() string {
	if f.IsRelation {
		return textutil.ToDatabaseColumnName(textutil.FKName(f.Name))
	}
	if f.DatabaseColumnName != "" {
		return f.DatabaseColumnName
	}
	return textutil.ToDatabaseColumnName(f.Name)
}

// HasFeature returns true if the entity has the named feature enabled.
func (e IREntity) HasFeature(name string) bool {
	if e.Features != nil {
		return e.Features[name]
	}
	return false
}

// PrimaryKey returns the entity's primary key field, or nil when none is marked.
func (e IREntity) PrimaryKey() *IRField {
	for i := range e.Fields {
		if e.Fields[i].IsPrimary {
			return &e.Fields[i]
		}
	}
	return nil
}

// FieldByName returns the field with the given name (case-insensitive), or nil.
// Bridges use this to resolve a seed/column name back to its IRField without
// re-implementing the lookup.
func (e IREntity) FieldByName(name string) *IRField {
	for i := range e.Fields {
		if strings.EqualFold(e.Fields[i].Name, name) {
			return &e.Fields[i]
		}
	}
	return nil
}

// HasAudit returns true if the entity has the audit feature enabled.
func (e IREntity) HasAudit() bool { return e.HasFeature("audit") }

// HasAuditLog returns true if the entity has the audit_log feature enabled.
func (e IREntity) HasAuditLog() bool { return e.HasFeature("audit_log") }

// HasSoftDelete returns true if the entity has the soft_delete feature enabled.
func (e IREntity) HasSoftDelete() bool { return e.HasFeature("soft_delete") }

// HasOptimisticLock returns true if the entity has the optimistic_lock feature enabled.
func (e IREntity) HasOptimisticLock() bool { return e.HasFeature("optimistic_lock") }

// HasEventSourced returns true if the entity emits domain events (event_sourced feature).
func (e IREntity) HasEventSourced() bool { return e.HasFeature("event_sourced") }

// HasCacheable returns true if the entity should use the distributed cache (cacheable feature).
func (e IREntity) HasCacheable() bool { return e.HasFeature("cacheable") }

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

	// PairFieldName is set when a `[many]` relation is reconciled with a single
	// (FK) relation declared on the target entity: the two declarations describe
	// one one-to-many relationship. It holds the FK field name on the target
	// (e.g. "wallet" for Wallet.Transactions <-> WalletTransaction.Wallet), so
	// bridges can render WithOne(...).HasForeignKey(...) instead of a join table.
	PairFieldName string

	// PairNavigationName is the resolved navigation property name for the paired
	// FK relation on the target entity. For Order.Items <-> OrderItem.OrderId the
	// target's field is "orderId" so the navigation is "Order" (not "OrderId").
	// Bridges use this to render WithOne(e => e.Order) correctly.
	PairNavigationName string
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
