package parser

import (
	"fmt"
	"sort"
	"strings"

	"domaincraft/internal/lexer"
	"domaincraft/internal/specmeta"
	"domaincraft/pkg/textutil"
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
		NamePlural: textutil.Pluralize(name),
		Features:   make(map[string]bool),
		Fields:     make(map[string]*ParsedField),
		FieldOrder: make([]string, 0),
		Indexes:    make([]*ParsedIndex, 0),
	}

	// Parse features (behavior macros)
	featureSet := specmeta.SliceToSet(specmeta.Features)
	for _, feature := range raw.Features {
		feature = strings.TrimSpace(feature)
		if !featureSet[feature] {
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
		perms := &ParsedPermissions{
			Read:       extractStringList(raw.Permissions, "read"),
			Create:     extractStringList(raw.Permissions, "create"),
			Update:     extractStringList(raw.Permissions, "update"),
			Delete:     extractStringList(raw.Permissions, "delete"),
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

// addFeatureFields adds automatic fields based on entity features.
// Uses specmeta.FeatureFieldDefs as the single source of truth.
func (p *Parser) addFeatureFields(entity *ParsedEntity) error {
	for _, fieldName := range []string{"createdAt", "updatedAt", "createdBy", "updatedBy", "deletedAt", "version"} {
		def, ok := specmeta.FeatureFieldDefs[fieldName]
		if !ok {
			continue
		}
		if !entity.Features[def.Feature] {
			continue
		}
		entity.Fields[fieldName] = newFeatureField(fieldName, def.Type, def.DBColumn, def.IsFuncDefault, def.DefaultValue)
		if def.IsOptional {
			entity.Fields[fieldName].IsOptional = true
		}
		entity.FieldOrder = append(entity.FieldOrder, fieldName)
	}
	return nil
}

// newFeatureField creates a ParsedField for an auto-generated feature field.
func newFeatureField(name, typ, dbCol string, isFuncDefault bool, defaultVal string) *ParsedField {
	fd := &lexer.FieldDefinition{
		Name:         name,
		Type:         typ,
		IsRequired:   true,
		Validations:  make(map[string]string),
		DefaultValue: defaultVal,
		DefaultIsFunc: isFuncDefault,
	}
	return &ParsedField{
		FieldDefinition:    fd,
		DatabaseColumnName: dbCol,
	}
}

// extractStringList extracts a []string from a raw YAML map at the given key.
func extractStringList(raw map[string]interface{}, key string) []string {
	val, ok := raw[key]
	if !ok {
		return nil
	}
	list, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		result = append(result, fmt.Sprint(item))
	}
	return result
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
