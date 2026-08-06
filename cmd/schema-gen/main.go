package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DomainCraft/DomainCraft/internal/specmeta"
)

func main() {
	outputPath := flag.String("o", filepath.Join("spec", "domain.schema.json"), "output path for the generated schema")
	flag.Parse()

	schema := buildSchema()
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal schema:", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outputPath, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write schema:", err)
		os.Exit(1)
	}
}

func buildSchema() map[string]any {
	return map[string]any{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"$id":                  "domain.schema.json",
		"title":                "DomainCraft domain.yaml schema",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"project":   map[string]any{"$ref": "#/$defs/Project"},
			"database":  map[string]any{"type": "string", "enum": specmeta.Databases},
			"auth":      map[string]any{"$ref": "#/$defs/AuthConfig"},
			"api_style": map[string]any{"type": "string", "enum": specmeta.APIStyles},
			"enums": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"entities": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"$ref": "#/$defs/EntityDefinition"},
			},
		},
		"required": []string{"project", "entities"},
		"$defs": map[string]any{
			"Project": map[string]any{
				"title":                "Project",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"version":     map[string]any{"type": "string"},
					"platform":    map[string]any{"type": "string", "description": "Target platform version (e.g. net10.0). Defaults to net10.0 for the csharp bridge."},
					"multi_tenancy": map[string]any{
						"$ref": "#/$defs/MultiTenancy",
					},
					"cache": map[string]any{
						"$ref": "#/$defs/CacheConfig",
					},
					"cors": map[string]any{
						"$ref": "#/$defs/CORSConfig",
					},
					"deploy": map[string]any{
						"$ref": "#/$defs/DeployConfig",
					},
					"versioning": map[string]any{
						"$ref": "#/$defs/VersioningConfig",
					},
					"rate_limit": map[string]any{
						"$ref": "#/$defs/RateLimitConfig",
					},
					"pagination": map[string]any{
						"$ref": "#/$defs/PaginationConfig",
					},
					"infrastructure": map[string]any{
						"$ref": "#/$defs/InfrastructureConfig",
					},
				},
				"required": []string{"name"},
			},
			"CacheConfig": map[string]any{
				"title":                "CacheConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"enabled":           map[string]any{"type": "boolean"},
					"provider":          map[string]any{"type": "string", "enum": specmeta.CacheProviders, "description": "Cache provider"},
					"connection_string": map[string]any{"type": "string"},
					"ttl_seconds":       map[string]any{"type": "integer", "minimum": 0},
				},
				"required": []string{"enabled"},
			},
			"CORSConfig": map[string]any{
				"title":                "CORSConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"enabled": map[string]any{"type": "boolean"},
					"origins": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []string{"enabled"},
			},
			"MultiTenancy": map[string]any{
				"title":                "MultiTenancy",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"enabled": map[string]any{"type": "boolean"},
					"mode":    map[string]any{"type": "string", "enum": specmeta.MultiTenancyModes},
				},
				"required": []string{"enabled"},
			},
			"DeployConfig": map[string]any{
				"title":                "DeployConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"domain": map[string]any{"type": "string", "description": "API domain (e.g. localhost, api.example.com)"},
					"port":   map[string]any{"type": "integer", "minimum": 1, "maximum": 65535, "description": "Exposed application port (default: 8080)"},
				},
			},
			"VersioningConfig": map[string]any{
				"title":                "VersioningConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"enabled":         map[string]any{"type": "boolean", "description": "Enable API versioning (default: true)"},
					"default_version": map[string]any{"type": "string", "description": "Default API version (default: 1.0)"},
				},
			},
			"RateLimitConfig": map[string]any{
				"title":                "RateLimitConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"enabled":        map[string]any{"type": "boolean", "description": "Enable request rate limiting (default: true)"},
					"policy":         map[string]any{"type": "string", "enum": specmeta.RateLimitPolicies, "description": "Limiter algorithm (default: fixed)"},
					"permit_limit":   map[string]any{"type": "integer", "minimum": 1, "description": "Max requests (default: 100)"},
					"window_seconds": map[string]any{"type": "integer", "minimum": 1, "description": "Window duration in seconds (default: 60)"},
				},
			},
			"PaginationConfig": map[string]any{
				"title":                "PaginationConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"default_page_size": map[string]any{"type": "integer", "minimum": 1, "description": "Default page size (default: 20)"},
					"max_page_size":     map[string]any{"type": "integer", "minimum": 1, "description": "Hard cap on page size (default: 200)"},
				},
			},
			"InfrastructureConfig": map[string]any{
				"title":                "InfrastructureConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"queue":   map[string]any{"type": "string", "enum": specmeta.InfraQueues, "description": "Message broker"},
					"cache":   map[string]any{"type": "string", "enum": specmeta.InfraCacheStores, "description": "Distributed cache store"},
					"secrets": map[string]any{"type": "string", "enum": specmeta.InfraSecretStores, "description": "Secrets store"},
					"storage": map[string]any{"type": "string", "enum": specmeta.InfraStores, "description": "Object/file storage"},
				},
			},
			"EntityDefinition": map[string]any{
				"title":                "EntityDefinition",
				"type":                 "object",
				"description":          "An entity. Valid keys: old_name, features, fields, indexes, permissions, seed. There is no `relations:` key — relations are declared as fields of type `relation(Target)` (see the fields description).",
				"additionalProperties": false,
				"properties": map[string]any{
					"old_name": map[string]any{"type": "string", "description": "Previous entity name — a hint for the migration engine to detect renames"},
					"features": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string", "enum": specmeta.Features},
					},
					"fields": map[string]any{
						"type":        "object",
						"description": "Field definitions. Each value is a definition string: `type [modifiers]` (e.g. `string [required, max:255]`). Relations are fields, not a separate key: `relation(Target) [many]` for many-to-many, `relation(Target) [required, on_delete:cascade]`, `relation(Target) [optional, on_delete:set_null]`. `on_delete` accepts cascade|restrict|set_null|no_action. Flag modifiers: `required`, `optional`, `unique`, `hidden`, `readonly`, `primary`, `many`, `email`, `url`, `ipv4`. Key:value modifiers: `min:N`, `max:N`, `gte:N`, `lte:N`, `gt:N`, `lt:N`, `regex:\"...\"`, `default:...`, `old_name:<previousName>`. `old_name` is a rename hint for the migration engine: the field was previously named `<previousName>`, so bridges can generate a safe `RenameColumn` instead of a destructive drop + add. `hidden` excludes a field from API responses; `readonly` keeps it in responses but excludes it from create/update/patch requests (server-owned). Do not put a space after `:` inside the definition string (`default:5`, not `default: 5`); quoted string defaults are allowed (`default:\"pending\"`).",
						"additionalProperties": map[string]any{"type": "string"},
					},
					"indexes": map[string]any{
						"type":  "array",
						"items": map[string]any{"$ref": "#/$defs/IndexDefinition"},
					},
					"permissions": map[string]any{
						"$ref": "#/$defs/EntityPermissions",
					},
					"seed": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "object", "additionalProperties": true},
					},
				},
				"required": []string{"fields"},
			},
			"IndexDefinition": map[string]any{
				"title":                "IndexDefinition",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"fields": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
					"type":   map[string]any{"type": "string", "enum": specmeta.IndexTypes},
					"sort":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"unique": map[string]any{"type": "boolean"},
				},
				"required": []string{"fields"},
			},
			"EntityPermissions": map[string]any{
				"title":                "EntityPermissions",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"read":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"create": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"update": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"delete": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
			"AuthConfig": map[string]any{
				"title":                "AuthConfig",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"type":      map[string]any{"type": "string", "enum": specmeta.AuthTypes},
					"entity":    map[string]any{"type": "string", "description": "Entity with email+password fields (auto-detected if omitted)"},
					"roles":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"endpoints": map[string]any{"$ref": "#/$defs/AuthEndpoints"},
				},
				"required": []string{"type"},
			},
			"AuthEndpoints": map[string]any{
				"title":                "AuthEndpoints",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"login":    map[string]any{"type": "boolean", "description": "Generate login endpoint (default: true)"},
					"register": map[string]any{"type": "boolean", "description": "Generate register endpoint (default: true)"},
					"me":       map[string]any{"type": "boolean", "description": "Generate /me endpoint (default: true)"},
				},
			},
		},
	}
}
