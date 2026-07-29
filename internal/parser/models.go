package parser

import (
	"fmt"

	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"gopkg.in/yaml.v3"
)

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
	Deploy       *DeployConfig       `yaml:"deploy"`
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

// DeployConfig represents deployment configuration.
type DeployConfig struct {
	Domain string `yaml:"domain"` // API domain (e.g. "localhost", "api.example.com")
	Port   int    `yaml:"port"`   // exposed port (default: 8080)
}

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
	Features    []string               `yaml:"features"`
	Fields      map[string]string      `yaml:"fields"`
	FieldOrder  []string               // preserved from YAML — not a yaml tag
	Indexes     []RawIndex             `yaml:"indexes"`
	Permissions *RawPermissions        `yaml:"permissions"`
	Seed        []map[string]interface{} `yaml:"seed"`
}

// RawPermissions represents entity permissions before parsing.
type RawPermissions struct {
	Read   []string `yaml:"read"`
	Create []string `yaml:"create"`
	Update []string `yaml:"update"`
	Delete []string `yaml:"delete"`
}

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
		return fmt.Errorf("unknown entity key %q; valid keys: features, fields, indexes, permissions, seed", key)
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

	return schema, nil
}
