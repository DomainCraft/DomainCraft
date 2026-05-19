package parser

import (
	"fmt"
	"sort"
	"strings"

	"domaincraft/internal/lexer"
)

// Parser is the main parser for converting RawSchema to ParsedSchema
type Parser struct {
	raw *RawSchema
}

// ParsedSchema is the structure after full parsing
type ParsedSchema struct {
	Project     ProjectConfig
	Database    string
	Auth        string
	APIStyle    string
	Entities    map[string]*ParsedEntity
	Enums       map[string][]string
	EntityOrder []string // entity order for deterministic generation
}

// ParsedEntity represents a fully parsed entity
type ParsedEntity struct {
	Name        string
	NamePlural  string // auto-generated plural form
	Features    map[string]bool
	Fields      map[string]*ParsedField
	FieldOrder  []string // field order for generation
	Indexes     []*ParsedIndex
	Permissions *ParsedPermissions
	Seed        []map[string]interface{}
}

// ParsedField represents a fully parsed field
type ParsedField struct {
	*lexer.FieldDefinition
	// Additional info after full analysis
	DatabaseColumnName string // generated from Name
	IsRelation         bool
	RelationTarget     string
}

// ParsedIndex represents a parsed index
type ParsedIndex struct {
	Fields []string
	Type   string
	Sort   []string
	Unique bool
	Name   string // auto-generated name
}

// ParsedPermissions represents parsed permissions
type ParsedPermissions struct {
	Read       []string
	Create     []string
	Update     []string
	Delete     []string
	ReadPublic string // condition expression
}

// NewParser creates a new Parser
func NewParser(raw *RawSchema) *Parser {
	return &Parser{raw: raw}
}

// Parse performs full parsing with validation
func (p *Parser) Parse() (*ParsedSchema, error) {
	schema := &ParsedSchema{
		Project:     p.raw.Project,
		Database:    p.raw.Database,
		Auth:        p.raw.Auth,
		APIStyle:    p.raw.APIStyle,
		Entities:    make(map[string]*ParsedEntity),
		Enums:       p.raw.Enums,
		EntityOrder: make([]string, 0),
	}

	// Parse all entities
	for entityName, rawEntity := range p.raw.Entities {
		entity, err := p.parseEntity(entityName, rawEntity)
		if err != nil {
			return nil, fmt.Errorf("error parsing entity '%s': %w", entityName, err)
		}
		schema.Entities[entityName] = entity
		schema.EntityOrder = append(schema.EntityOrder, entityName)
	}

	// Sort for deterministic order
	sort.Strings(schema.EntityOrder)

	return schema, nil
}

// parseEntity parses a single entity
func (p *Parser) parseEntity(name string, raw RawEntity) (*ParsedEntity, error) {
	entity := &ParsedEntity{
		Name:       name,
		NamePlural: pluralize(name),
		Features:   make(map[string]bool),
		Fields:     make(map[string]*ParsedField),
		FieldOrder: make([]string, 0),
		Indexes:    make([]*ParsedIndex, 0),
	}

	// Parse features (behavior macros)
	for _, feature := range raw.Features {
		validFeatures := map[string]bool{
			"audit": true, "audit_log": true, "soft_delete": true, "optimistic_lock": true,
		}
		feature = strings.TrimSpace(feature)
		if !validFeatures[feature] {
			return nil, fmt.Errorf("unknown feature: %s", feature)
		}
		entity.Features[feature] = true
	}

	// Add automatic fields based on features
	if err := p.addFeatureFields(entity); err != nil {
		return nil, err
	}

	// Parse fields
	for fieldName, fieldDef := range raw.Fields {
		field, err := p.parseField(fieldName, fieldDef)
		if err != nil {
			return nil, err
		}
		entity.Fields[fieldName] = field
		entity.FieldOrder = append(entity.FieldOrder, fieldName)
	}

	// Parse indexes
	for i, rawIdx := range raw.Indexes {
		idx := &ParsedIndex{
			Fields: rawIdx.Fields,
			Type:   rawIdx.Type,
			Sort:   rawIdx.Sort,
			Unique: rawIdx.Unique,
			Name:   generateIndexName(name, rawIdx.Fields, i),
		}
		entity.Indexes = append(entity.Indexes, idx)
	}

	// Parse permissions
	if raw.Permissions != nil {
		perms := &ParsedPermissions{}

		if read, ok := raw.Permissions["read"]; ok {
			if readList, ok := read.([]interface{}); ok {
				for _, r := range readList {
					perms.Read = append(perms.Read, fmt.Sprint(r))
				}
			}
		}
		if create, ok := raw.Permissions["create"]; ok {
			if createList, ok := create.([]interface{}); ok {
				for _, c := range createList {
					perms.Create = append(perms.Create, fmt.Sprint(c))
				}
			}
		}
		if update, ok := raw.Permissions["update"]; ok {
			if updateList, ok := update.([]interface{}); ok {
				for _, u := range updateList {
					perms.Update = append(perms.Update, fmt.Sprint(u))
				}
			}
		}
		if delete, ok := raw.Permissions["delete"]; ok {
			if deleteList, ok := delete.([]interface{}); ok {
				for _, d := range deleteList {
					perms.Delete = append(perms.Delete, fmt.Sprint(d))
				}
			}
		}
		if readPublic, ok := raw.Permissions["read_public"]; ok {
			perms.ReadPublic = fmt.Sprint(readPublic)
		}

		entity.Permissions = perms
	}

	// Seed data
	entity.Seed = raw.Seed

	return entity, nil
}

// parseField parses a single field using the lexer
func (p *Parser) parseField(name string, fieldDef string) (*ParsedField, error) {
	fd, err := lexer.ParseFieldString(fieldDef)
	if err != nil {
		return nil, err
	}
	fd.Name = name

	pf := &ParsedField{
		FieldDefinition:    fd,
		DatabaseColumnName: toDatabaseColumnName(name),
		IsRelation:         fd.Type == "relation",
		RelationTarget:     fd.TargetEntity,
	}

	return pf, nil
}

// addFeatureFields adds automatic fields based on entity features
func (p *Parser) addFeatureFields(entity *ParsedEntity) error {
	// audit: adds createdAt and updatedAt
	if entity.Features["audit"] {
		createdAtField := &ParsedField{
			FieldDefinition: &lexer.FieldDefinition{
				Name:          "createdAt",
				Type:          "datetime",
				IsRequired:    true,
				IsPrimary:     false,
				IsOptional:    false,
				Validations:   make(map[string]string),
				DefaultValue:  "now",
				DefaultIsFunc: true,
			},
			DatabaseColumnName: "created_at",
		}
		updatedAtField := &ParsedField{
			FieldDefinition: &lexer.FieldDefinition{
				Name:          "updatedAt",
				Type:          "datetime",
				IsRequired:    true,
				IsPrimary:     false,
				IsOptional:    false,
				Validations:   make(map[string]string),
				DefaultValue:  "now",
				DefaultIsFunc: true,
			},
			DatabaseColumnName: "updated_at",
		}
		entity.Fields["createdAt"] = createdAtField
		entity.Fields["updatedAt"] = updatedAtField
		entity.FieldOrder = append(entity.FieldOrder, "createdAt", "updatedAt")
	}

	// audit_log: adds createdBy and updatedBy (uuid references to User)
	if entity.Features["audit_log"] {
		createdByField := &ParsedField{
			FieldDefinition: &lexer.FieldDefinition{
				Name:        "createdBy",
				Type:        "uuid",
				IsRequired:  true,
				Validations: make(map[string]string),
			},
			DatabaseColumnName: "created_by",
		}
		updatedByField := &ParsedField{
			FieldDefinition: &lexer.FieldDefinition{
				Name:        "updatedBy",
				Type:        "uuid",
				IsRequired:  true,
				Validations: make(map[string]string),
			},
			DatabaseColumnName: "updated_by",
		}
		entity.Fields["createdBy"] = createdByField
		entity.Fields["updatedBy"] = updatedByField
		entity.FieldOrder = append(entity.FieldOrder, "createdBy", "updatedBy")
	}

	// soft_delete: adds deletedAt (nullable datetime)
	if entity.Features["soft_delete"] {
		deletedAtField := &ParsedField{
			FieldDefinition: &lexer.FieldDefinition{
				Name:        "deletedAt",
				Type:        "datetime",
				IsOptional:  true,
				Validations: make(map[string]string),
			},
			DatabaseColumnName: "deleted_at",
		}
		entity.Fields["deletedAt"] = deletedAtField
		entity.FieldOrder = append(entity.FieldOrder, "deletedAt")
	}

	// optimistic_lock: adds version (int)
	if entity.Features["optimistic_lock"] {
		versionField := &ParsedField{
			FieldDefinition: &lexer.FieldDefinition{
				Name:         "version",
				Type:         "int",
				IsRequired:   true,
				DefaultValue: "0",
				Validations:  make(map[string]string),
			},
			DatabaseColumnName: "version",
		}
		entity.Fields["version"] = versionField
		entity.FieldOrder = append(entity.FieldOrder, "version")
	}

	return nil
}

// Helper functions for name transformations

// pluralize is a simple pluralizer for English nouns
func pluralize(name string) string {
	if strings.HasSuffix(name, "y") && len(name) > 1 {
		// lady -> ladies
		if !isVowel(name[len(name)-2]) {
			return name[:len(name)-1] + "ies"
		}
	}
	if strings.HasSuffix(name, "s") || strings.HasSuffix(name, "ss") ||
		strings.HasSuffix(name, "x") || strings.HasSuffix(name, "z") {
		return name + "es"
	}
	if strings.HasSuffix(name, "o") {
		if !isVowel(name[len(name)-2]) {
			return name + "es"
		}
	}
	return name + "s"
}

func isVowel(b byte) bool {
	return b == 'a' || b == 'e' || b == 'i' || b == 'o' || b == 'u'
}

// toDatabaseColumnName converts camelCase to snake_case
func toDatabaseColumnName(fieldName string) string {
	var result strings.Builder
	for i, r := range fieldName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// generateIndexName generates an index name from entity name and fields
func generateIndexName(entityName string, fields []string, idx int) string {
	fieldsPart := strings.Join(fields, "_")
	return fmt.Sprintf("idx_%s_%s_%d",
		strings.ToLower(entityName),
		strings.ToLower(fieldsPart),
		idx)
}

// ParseYAML is a convenience function for full parsing from YAML bytes
func ParseYAML(data []byte) (*ParsedSchema, error) {
	raw, err := ParseRawSchema(data)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %w", err)
	}

	parser := NewParser(raw)
	return parser.Parse()
}
