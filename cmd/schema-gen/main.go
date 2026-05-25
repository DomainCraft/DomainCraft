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
			"auth":      map[string]any{"type": "string"},
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
					"name":         map[string]any{"type": "string"},
					"description":  map[string]any{"type": "string"},
					"version":      map[string]any{"type": "string"},
					"platform":     map[string]any{"type": "string", "description": "Target platform version (e.g. net9.0, net8.0)"},
					"multi_tenancy": map[string]any{
						"$ref": "#/$defs/MultiTenancy",
					},
					"cache": map[string]any{
						"$ref": "#/$defs/CacheConfig",
					},
					"cors": map[string]any{
						"$ref": "#/$defs/CORSConfig",
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
					"provider":          map[string]any{"type": "string", "description": "Cache provider (redis, memcached, etc.)"},
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
					"mode":    map[string]any{"type": "string"},
				},
				"required": []string{"enabled"},
			},
			"EntityDefinition": map[string]any{
				"title":                "EntityDefinition",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"features": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string", "enum": specmeta.Features},
					},
					"fields": map[string]any{
						"type":                 "object",
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
					"type":   map[string]any{"type": "string"},
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
					"read":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"create":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"update":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"delete":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"read_public": map[string]any{"type": "string"},
				},
			},
		},
	}
}
