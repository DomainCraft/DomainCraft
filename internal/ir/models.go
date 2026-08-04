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

// HasInfraQueue returns true when a message broker is declared.
func (p *IRProject) HasInfraQueue() bool {
	return p.Infrastructure != nil && p.Infrastructure.Queue != "" && p.Infrastructure.Queue != "in-memory"
}

// HasInfraStorage returns true when an object/file storage is declared.
func (p *IRProject) HasInfraStorage() bool {
	return p.Infrastructure != nil && p.Infrastructure.Storage != "" && p.Infrastructure.Storage != "local"
}

// HasInfraCache returns true when a distributed cache store is declared.
func (p *IRProject) HasInfraCache() bool {
	return p.Infrastructure != nil && p.Infrastructure.Cache != "" && p.Infrastructure.Cache != "in-memory"
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
