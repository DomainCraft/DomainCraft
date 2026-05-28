package parser

import "gopkg.in/yaml.v3"

// RawSchema represents the root structure of domain.yaml
type RawSchema struct {
	Project  ProjectConfig        `yaml:"project"`
	Database string               `yaml:"database"`
	Auth     AuthConfig           `yaml:"auth"`
	APIStyle string               `yaml:"api_style"`
	Entities map[string]RawEntity `yaml:"entities"`
	Enums    map[string][]string  `yaml:"enums"`
}

// ProjectConfig contains project-level information
type ProjectConfig struct {
	Name         string              `yaml:"name"`
	Description  string              `yaml:"description"`
	Version      string              `yaml:"version"`
	Platform     string              `yaml:"platform"`
	MultiTenancy *MultiTenancyConfig `yaml:"multi_tenancy"`
	Cache        *CacheConfig        `yaml:"cache"`
	CORS         *CORSConfig         `yaml:"cors"`
}

// AuthConfig describes authentication configuration.
type AuthConfig struct {
	Type      string        `yaml:"type"`      // jwt, none
	Entity    string        `yaml:"entity"`    // optional, auto-detect if empty
	Roles     []string      `yaml:"roles"`     // optional, for enum generation
	Endpoints AuthEndpoints `yaml:"endpoints"` // optional, defaults to all true
}

// AuthEndpoints controls which auth endpoints are generated.
type AuthEndpoints struct {
	Login    *bool `yaml:"login"`    // default: true
	Register *bool `yaml:"register"` // default: true
	Me       *bool `yaml:"me"`       // default: true
}

// HasLogin returns true if login endpoint is enabled (default: true).
func (e AuthEndpoints) HasLogin() bool { return e.Login == nil || *e.Login }

// HasRegister returns true if register endpoint is enabled (default: true).
func (e AuthEndpoints) HasRegister() bool { return e.Register == nil || *e.Register }

// HasMe returns true if me endpoint is enabled (default: true).
func (e AuthEndpoints) HasMe() bool { return e.Me == nil || *e.Me }

// CacheConfig represents cache configuration (agnostic — no language/platform specifics).
type CacheConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Provider         string `yaml:"provider"`
	ConnectionString string `yaml:"connection_string"`
	TTLSeconds       int    `yaml:"ttl_seconds"`
}

// CORSConfig represents CORS configuration.
type CORSConfig struct {
	Enabled bool     `yaml:"enabled"`
	Origins []string `yaml:"origins"`
}

// MultiTenancyConfig holds multi-tenancy settings
type MultiTenancyConfig struct {
	Enabled bool   `yaml:"enabled"`
	Mode    string `yaml:"mode"` // column, schema, database
}

// RawEntity represents an unprocessed entity from YAML
type RawEntity struct {
	Features    []string                 `yaml:"features"`
	Fields      map[string]string        `yaml:"fields"`
	Indexes     []RawIndex               `yaml:"indexes"`
	Permissions map[string]interface{}   `yaml:"permissions"`
	Seed        []map[string]interface{} `yaml:"seed"`
}

// RawIndex represents an index definition
type RawIndex struct {
	Fields []string `yaml:"fields"`
	Type   string   `yaml:"type"` // btree, hash, gist, gin, brin
	Sort   []string `yaml:"sort"` // asc, desc
	Unique bool     `yaml:"unique"`
}

// ParseRawSchema reads YAML and converts it to RawSchema
func ParseRawSchema(data []byte) (*RawSchema, error) {
	schema := &RawSchema{}
	if err := yaml.Unmarshal(data, schema); err != nil {
		return nil, err
	}

	// Set defaults
	if schema.Database == "" {
		schema.Database = "postgresql"
	}
	if schema.Auth.Type == "" {
		schema.Auth.Type = "none"
	}
	if schema.APIStyle == "" {
		schema.APIStyle = "rest"
	}

	return schema, nil
}
