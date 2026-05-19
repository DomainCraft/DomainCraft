package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"domaincraft/internal/specmeta"
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
			"project": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"version":     map[string]any{"type": "string"},
					"multi_tenancy": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"enabled": map[string]any{"type": "boolean"},
							"mode":    map[string]any{"type": "string"},
						},
						"additionalProperties": false,
					},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
			"database": map[string]any{
				"type": "string",
				"enum": specmeta.Databases,
			},
			"auth": map[string]any{"type": "string"},
			"api_style": map[string]any{
				"type": "string",
				"enum": specmeta.APIStyles,
			},
			"enums": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"entities": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type": "object",
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
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"fields": map[string]any{
										"type":  "array",
										"items": map[string]any{"type": "string"},
									},
									"type":   map[string]any{"type": "string"},
									"sort":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
									"unique": map[string]any{"type": "boolean"},
								},
								"required":             []string{"fields"},
								"additionalProperties": false,
							},
						},
						"permissions": map[string]any{
							"type": "object",
							"additionalProperties": map[string]any{
								"type":  "array",
								"items": map[string]any{"type": "string"},
							},
						},
						"seed": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "object", "additionalProperties": true},
						},
					},
					"required":             []string{"fields"},
					"additionalProperties": false,
				},
			},
		},
		"required": []string{"project", "entities"},
	}
}
