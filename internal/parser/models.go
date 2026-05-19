package parser

import "gopkg.in/yaml.v3"

// RawSchema represents the root structure of domain.yaml
type RawSchema struct {
	Project  ProjectConfig        `yaml:"project"`
	Database string               `yaml:"database"`
	Auth     string               `yaml:"auth"`
	APIStyle string               `yaml:"api_style"`
	Entities map[string]RawEntity `yaml:"entities"`
	Enums    map[string][]string  `yaml:"enums"`
}

// ProjectConfig contains project-level information
type ProjectConfig struct {
	Name         string              `yaml:"name"`
	Description  string              `yaml:"description"`
	Version      string              `yaml:"version"`
	MultiTenancy *MultiTenancyConfig `yaml:"multi_tenancy"`
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

// RawPermissions defines access control for an entity
type RawPermissions struct {
	Read       []string `yaml:"read"`
	Create     []string `yaml:"create"`
	Update     []string `yaml:"update"`
	Delete     []string `yaml:"delete"`
	ReadPublic string   `yaml:"read_public"` // condition(field == value)
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
	if schema.Auth == "" {
		schema.Auth = "none"
	}
	if schema.APIStyle == "" {
		schema.APIStyle = "rest"
	}

	return schema, nil
}
