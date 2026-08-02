package parser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/lexer"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// Parser is the main parser for converting RawSchema to ParsedSchema
type Parser struct {
	raw *RawSchema
}

// ParsedSchema is the structure after full parsing
type ParsedSchema struct {
	Project     ProjectConfig
	Database    string
	Auth        *AuthConfig
	APIStyle    string
	Entities    map[string]*ParsedEntity
	Enums       map[string][]string
	EntityOrder []string // entity order for deterministic generation
}

// ParsedEntity represents a fully parsed entity
type ParsedEntity struct {
	Name        string
	OldName     string // previous entity name (rename hint for the migration engine)
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
	RelationType       string // inferred by parser: many-to-one, one-to-one, many-to-many
}

// IsRelation reports whether the field is a relation field.
func (f *ParsedField) IsRelation() bool { return f.Type == "relation" }

// NavigationName returns the resolved navigation property name for the field.
// For "categoryId" -> "Category", for a many relation "tags" -> "Tag".
// Falls back to the relation target entity name when the field name has no
// meaningful stem after stripping the Id suffix.
func (f *ParsedField) NavigationName() string {
	if f == nil || f.FieldDefinition == nil {
		return ""
	}
	name := f.Name
	if f.IsMany {
		name = textutil.Singularize(name)
	}
	// Strip "Id" or "ID" suffix (FK convention), not arbitrary "id" substrings.
	if strings.HasSuffix(name, "Id") && len(name) > 2 {
		name = name[:len(name)-2]
	} else if strings.HasSuffix(name, "ID") && len(name) > 2 {
		name = name[:len(name)-2]
	}
	if name == "" {
		name = f.TargetEntity
	}
	return textutil.PascalCase(name)
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
	Read   []string
	Create []string
	Update []string
	Delete []string
}

// NewParser creates a new Parser
func NewParser(raw *RawSchema) *Parser {
	return &Parser{raw: raw}
}

// Parse performs full parsing with validation
func (p *Parser) Parse() (*ParsedSchema, error) {
	auth := p.raw.Auth
	schema := &ParsedSchema{
		Project:     p.raw.Project,
		Database:    p.raw.Database,
		Auth:        &auth,
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
		OldName:    raw.OldName,
		NamePlural: textutil.Pluralize(name),
		Features:   make(map[string]bool),
		Fields:     make(map[string]*ParsedField),
		FieldOrder: make([]string, 0),
		Indexes:    make([]*ParsedIndex, 0),
	}

	// Parse features (behavior macros)
	for _, feature := range raw.Features {
		feature = strings.TrimSpace(feature)
		if !specmeta.IsFeature(feature) {
			return nil, fmt.Errorf("unknown feature: %s", feature)
		}
		entity.Features[feature] = true
	}

	// Add automatic fields based on features
	if err := p.addFeatureFields(entity); err != nil {
		return nil, err
	}

	// Parse fields in YAML order (preserved by RawEntity.UnmarshalYAML).
	for _, fieldName := range raw.FieldOrder {
		fieldDef := raw.Fields[fieldName]
		field, err := p.parseField(fieldName, fieldDef)
		if err != nil {
			return nil, err
		}
		entity.Fields[fieldName] = field
		// Skip if already added by addFeatureFields (user overriding a feature field).
		if _, alreadyPresent := specmeta.FeatureFieldDefs[fieldName]; !alreadyPresent {
			entity.FieldOrder = append(entity.FieldOrder, fieldName)
		}
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
			Read:   raw.Permissions.Read,
			Create: raw.Permissions.Create,
			Update: raw.Permissions.Update,
			Delete: raw.Permissions.Delete,
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

	// Semantic normalization: primary implies required, optional implies not required.
	if fd.IsPrimary {
		fd.IsRequired = true
	}
	if fd.IsOptional {
		fd.IsRequired = false
	}

	// Sync required flag to validations map so templates can use HasValidation "required"
	if fd.IsRequired {
		fd.Validations["required"] = "true"
	}

	// Infer relation type for relation fields.
	var relType string
	if fd.Type == "relation" {
		if fd.IsMany {
			relType = "many-to-many"
		} else if fd.IsUnique {
			relType = "one-to-one"
		} else {
			relType = "many-to-one"
		}
	}

	pf := &ParsedField{
		FieldDefinition:    fd,
		DatabaseColumnName: textutil.ToDatabaseColumnName(name),
		RelationType:       relType,
	}

	return pf, nil
}

// addFeatureFields adds automatic fields based on entity features.
// Uses specmeta.FeatureFieldDefs as the single source of truth.
func (p *Parser) addFeatureFields(entity *ParsedEntity) error {
	// Iterate in sorted order for deterministic field generation.
	names := make([]string, 0, len(specmeta.FeatureFieldDefs))
	for name := range specmeta.FeatureFieldDefs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, fieldName := range names {
		def := specmeta.FeatureFieldDefs[fieldName]
		if !entity.Features[def.Feature] {
			continue
		}
		entity.Fields[fieldName] = newFeatureField(fieldName, def)
		entity.FieldOrder = append(entity.FieldOrder, fieldName)
	}
	return nil
}

// newFeatureField creates a ParsedField for an auto-generated feature field.
func newFeatureField(name string, def specmeta.FeatureFieldDef) *ParsedField {
	fd := &lexer.FieldDefinition{
		Name:          name,
		Type:          def.Type,
		IsRequired:    !def.IsOptional,
		IsOptional:    def.IsOptional,
		Validations:   make(map[string]string),
		DefaultValue:  def.DefaultValue,
		DefaultIsFunc: def.IsFuncDefault,
	}
	return &ParsedField{
		FieldDefinition:    fd,
		DatabaseColumnName: def.DBColumn,
	}
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
