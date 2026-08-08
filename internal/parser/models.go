package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"gopkg.in/yaml.v3"
)

// RawSchema represents the root structure of domain.yaml
type RawSchema struct {
	Project  ProjectConfig        `yaml:"project" json:"project"`
	Database string               `yaml:"database" json:"database,omitempty"`
	Auth     AuthConfig           `yaml:"auth" json:"auth"`
	APIStyle string               `yaml:"api_style" json:"apiStyle"`
	Entities map[string]RawEntity `yaml:"entities" json:"entities"`
	Enums    map[string][]string  `yaml:"enums" json:"enums,omitempty"`
}

// ProjectConfig contains project-level information
type ProjectConfig struct {
	Name         string              `yaml:"name" json:"name"`
	Description  string              `yaml:"description" json:"description,omitempty"`
	Version      string              `yaml:"version" json:"version,omitempty"`
	Platform     string              `yaml:"platform" json:"platform,omitempty"`
	MultiTenancy *MultiTenancyConfig `yaml:"multi_tenancy" json:"multi_tenancy,omitempty"`
	Cache        *CacheConfig        `yaml:"cache" json:"cache,omitempty"`
	CORS         *CORSConfig         `yaml:"cors" json:"cors,omitempty"`
	Deploy       *DeployConfig       `yaml:"deploy" json:"deploy,omitempty"`
	// Versioning controls API versioning. Language-agnostic: the bridge decides
	// how to express it (Asp.Versioning, Django REST framework versioning, etc.).
	Versioning *VersioningConfig `yaml:"versioning" json:"versioning,omitempty"`
	// RateLimit controls HTTP rate limiting policy (permit limit / window / algorithm).
	RateLimit *RateLimitConfig `yaml:"rate_limit" json:"rate_limit,omitempty"`
	// Pagination controls list endpoint page sizing (default page size + hard cap).
	Pagination *PaginationConfig `yaml:"pagination" json:"pagination,omitempty"`
	// Infrastructure declares the backing services (message broker, distributed
	// cache, secrets store) that addons such as Dapr wire up at runtime. Kept
	// purely declarative and provider-agnostic — no specific library/cloud terms.
	Infrastructure *InfrastructureConfig `yaml:"infrastructure" json:"infrastructure,omitempty"`
}

// InfrastructureConfig declares the provider-agnostic infrastructure backing a
// project. Bridges that ship an addon (e.g. Dapr) turn these into concrete
// components (pubsub/statestore/secrets) while the domain model stays clean.
type InfrastructureConfig struct {
	Queue   string `yaml:"queue" json:"queue,omitempty"`     // message broker: pubsub, rabbitmq, kafka, redis, nats, in-memory
	Cache   string `yaml:"cache" json:"cache,omitempty"`     // distributed cache store: redis, memcached, in-memory
	Secrets string `yaml:"secrets" json:"secrets,omitempty"` // secrets store: local, kubernetes, azure-keyvault, aws-secrets
	Storage string `yaml:"storage" json:"storage,omitempty"` // object/file storage: local, s3, azure-blob, gcs
}

// AuthConfig describes authentication configuration.
type AuthConfig struct {
	Type      string        `yaml:"type" json:"type"`
	Entity    string        `yaml:"entity" json:"entity,omitempty"`
	Roles     []string      `yaml:"roles" json:"roles,omitempty"`
	Endpoints AuthEndpoints `yaml:"endpoints" json:"endpoints"`
}

// AuthEndpoints controls which auth endpoints are generated.
type AuthEndpoints struct {
	Login    *bool `yaml:"login" json:"login"`
	Register *bool `yaml:"register" json:"register"`
	Me       *bool `yaml:"me" json:"me"`
	Setup    *bool `yaml:"setup" json:"setup"`
}

// HasLogin returns true if login endpoint is enabled (default: true).
func (e AuthEndpoints) HasLogin() bool { return e.Login == nil || *e.Login }

// HasRegister returns true if register endpoint is enabled (default: true).
func (e AuthEndpoints) HasRegister() bool { return e.Register == nil || *e.Register }

// HasMe returns true if me endpoint is enabled (default: true).
func (e AuthEndpoints) HasMe() bool { return e.Me == nil || *e.Me }

// HasSetup returns true if setup endpoint is enabled (default: true).
func (e AuthEndpoints) HasSetup() bool { return e.Setup == nil || *e.Setup }

// DeployConfig represents deployment configuration.
type DeployConfig struct {
	Domain string `yaml:"domain" json:"domain,omitempty"`
	Port   int    `yaml:"port" json:"port,omitempty"`
}

// CacheConfig represents cache configuration (agnostic — no language/platform specifics).
type CacheConfig struct {
	Enabled          bool   `yaml:"enabled" json:"enabled"`
	Provider         string `yaml:"provider" json:"provider,omitempty"`
	ConnectionString string `yaml:"connection_string" json:"connection_string,omitempty"`
	TTLSeconds       int    `yaml:"ttl_seconds" json:"ttl_seconds,omitempty"`
}

// CORSConfig represents CORS configuration.
type CORSConfig struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Origins []string `yaml:"origins" json:"origins,omitempty"`
}

// VersioningConfig represents API versioning configuration.
type VersioningConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	DefaultVersion string `yaml:"default_version" json:"default_version,omitempty"`
}

// RateLimitConfig represents HTTP request rate limiting configuration.
type RateLimitConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Policy        string `yaml:"policy" json:"policy,omitempty"` // fixed | sliding
	PermitLimit   int    `yaml:"permit_limit" json:"permit_limit,omitempty"`
	WindowSeconds int    `yaml:"window_seconds" json:"window_seconds,omitempty"`
}

// PaginationConfig represents list pagination sizing.
type PaginationConfig struct {
	DefaultPageSize int `yaml:"default_page_size" json:"default_page_size,omitempty"`
	MaxPageSize     int `yaml:"max_page_size" json:"max_page_size,omitempty"`
}

// MultiTenancyConfig holds multi-tenancy settings
type MultiTenancyConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Mode    string `yaml:"mode" json:"mode,omitempty"`
}

// RawEntity represents an unprocessed entity from YAML
type RawEntity struct {
	OldName     string                   `yaml:"old_name" json:"old_name,omitempty"` // hint for the migration engine: previous entity name (rename detection)
	Features    []string                 `yaml:"features" json:"features,omitempty"`
	Fields      map[string]string        `yaml:"fields" json:"fields"`
	FieldOrder  []string                 `yaml:"-" json:"fieldOrder"` // preserved from YAML — not a yaml tag
	Indexes     []RawIndex               `yaml:"indexes" json:"indexes,omitempty"`
	Permissions *RawPermissions          `yaml:"permissions" json:"permissions,omitempty"`
	Seed        []map[string]interface{} `yaml:"seed" json:"seed,omitempty"`
}

// RawPermissions represents entity permissions before parsing.
type RawPermissions struct {
	Read   []string `yaml:"read" json:"read,omitempty"`
	Create []string `yaml:"create" json:"create,omitempty"`
	Update []string `yaml:"update" json:"update,omitempty"`
	Delete []string `yaml:"delete" json:"delete,omitempty"`
}

// UnmarshalYAML rejects unknown permission keys instead of silently ignoring them.
func (p *RawPermissions) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return value.Decode((*rawPermissionsAlias)(p))
	}
	// Iterate keys explicitly so unknown permission keys become errors.
	for i := 0; i < len(value.Content)-1; i += 2 {
		key := value.Content[i].Value
		if !specmeta.IsPermissionKey(key) {
			return fmt.Errorf("unknown permission key %q; valid keys: %s", key, strings.Join(specmeta.PermissionKeys, ", "))
		}
	}
	return value.Decode((*rawPermissionsAlias)(p))
}

type rawPermissionsAlias RawPermissions

// UnmarshalYAML preserves field order from the YAML mapping node.
func (e *RawEntity) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		// Fallback for non-mapping nodes
		type rawEntityAlias RawEntity
		return value.Decode((*rawEntityAlias)(e))
	}

	// Extract ordered keys and decode fields in order
	for i := 0; i < len(value.Content)-1; i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		key := keyNode.Value

		switch key {
		case "old_name":
			if err := valNode.Decode(&e.OldName); err != nil {
				return err
			}
		case "features":
			if err := valNode.Decode(&e.Features); err != nil {
				return err
			}
		case "fields":
			e.Fields = make(map[string]string)
			e.FieldOrder = make([]string, 0, len(valNode.Content)/2)
			for j := 0; j < len(valNode.Content)-1; j += 2 {
				fKey := valNode.Content[j].Value
				var fVal string
				if err := valNode.Content[j+1].Decode(&fVal); err != nil {
					return err
				}
				e.Fields[fKey] = fVal
				e.FieldOrder = append(e.FieldOrder, fKey)
			}
		case "indexes":
			if err := valNode.Decode(&e.Indexes); err != nil {
				return err
			}
		case "permissions":
			if err := valNode.Decode(&e.Permissions); err != nil {
				return err
			}
		case "seed":
			if err := valNode.Decode(&e.Seed); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown entity key %q; valid keys: old_name, features, fields, indexes, permissions, seed", key)
		}
	}

	// Ensure non-nil slices
	if e.FieldOrder == nil {
		e.FieldOrder = []string{}
	}

	return nil
}

// RawIndex represents an index definition
type RawIndex struct {
	Fields []string `yaml:"fields" json:"fields"`
	Type   string   `yaml:"type" json:"type,omitempty"`
	Sort   []string `yaml:"sort" json:"sort,omitempty"`
	Unique bool     `yaml:"unique" json:"unique,omitempty"`
}

// UnmarshalYAML rejects unknown index keys instead of silently ignoring them.
func (r *RawIndex) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return value.Decode((*rawIndexAlias)(r))
	}
	valid := specmeta.SliceToSet([]string{"fields", "type", "sort", "unique"})
	for i := 0; i < len(value.Content)-1; i += 2 {
		key := value.Content[i].Value
		if !valid[key] {
			return fmt.Errorf("unknown index key %q; valid keys: fields, type, sort, unique", key)
		}
	}
	return value.Decode((*rawIndexAlias)(r))
}

type rawIndexAlias RawIndex

// ParseRawSchema reads YAML and converts it to RawSchema
func ParseRawSchema(data []byte) (*RawSchema, error) {
	schema := &RawSchema{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(schema); err != nil {
		return nil, err
	}

	// Set defaults
	if schema.Database == "" {
		schema.Database = specmeta.Databases[0]
	}
	if schema.Auth.Type == "" {
		schema.Auth.Type = specmeta.AuthTypes[len(specmeta.AuthTypes)-1] // "none"
	}
	if schema.APIStyle == "" {
		schema.APIStyle = specmeta.APIStyles[0]
	}
	// Deploy defaults — consistent with parser setting other defaults.
	if schema.Project.Deploy == nil {
		schema.Project.Deploy = &DeployConfig{}
	}
	if schema.Project.Deploy.Port == 0 {
		schema.Project.Deploy.Port = 8080
	}
	if schema.Project.Deploy.Domain == "" {
		schema.Project.Deploy.Domain = "localhost"
	}

	// Versioning defaults — enabled with default version "1.0".
	if schema.Project.Versioning == nil {
		schema.Project.Versioning = &VersioningConfig{Enabled: true}
	}
	if schema.Project.Versioning.DefaultVersion == "" {
		schema.Project.Versioning.DefaultVersion = "1.0"
	}

	// Rate limit defaults — enabled, fixed window, 100 req / 60s.
	if schema.Project.RateLimit == nil {
		schema.Project.RateLimit = &RateLimitConfig{Enabled: true}
	}
	if schema.Project.RateLimit.Policy == "" {
		schema.Project.RateLimit.Policy = "fixed"
	}
	if schema.Project.RateLimit.PermitLimit == 0 {
		schema.Project.RateLimit.PermitLimit = 100
	}
	if schema.Project.RateLimit.WindowSeconds == 0 {
		schema.Project.RateLimit.WindowSeconds = 60
	}

	// Pagination defaults — 20 per page, capped at 200.
	if schema.Project.Pagination == nil {
		schema.Project.Pagination = &PaginationConfig{}
	}
	if schema.Project.Pagination.DefaultPageSize == 0 {
		schema.Project.Pagination.DefaultPageSize = 20
	}
	if schema.Project.Pagination.MaxPageSize == 0 {
		schema.Project.Pagination.MaxPageSize = 200
	}

	return schema, nil
}
