package lexer

import (
	"fmt"
	"regexp"
	"strings"

	"domaincraft/internal/specmeta"
)

// FieldDefinition represents a parsed field definition
type FieldDefinition struct {
	Name         string // field name (set by the entity parser)
	Type         string // string, int, uuid, relation, enum, array, json, text, etc.
	TargetEntity string // for relation(EntityName)
	TargetType   string // for array(Type) or enum(Name)

	// Modifiers
	IsPrimary  bool
	IsOptional bool
	IsUnique   bool
	IsHidden   bool
	IsRequired bool

	// Relations
	RelationType string // one-to-one, one-to-many, many-to-many
	OnDelete     string // cascade, set_null, restrict, no_action
	IsMany       bool   // for many-to-many

	// Validation
	Validations   map[string]string // min, max, email, url, regex, gte, lt, lte, gt
	DefaultValue  string
	DefaultIsFunc bool // true if default:now()

	// Original string for debugging
	RawString string
}

// Lexer parses field definition strings into FieldDefinition
type Lexer struct {
	input string
	pos   int
}

// NewLexer creates a new Lexer
func NewLexer(input string) *Lexer {
	return &Lexer{input: strings.TrimSpace(input), pos: 0}
}

// Parse parses a full field definition: "string [required, max:255]"
func (l *Lexer) Parse() (*FieldDefinition, error) {
	fd := &FieldDefinition{
		RawString:   l.input,
		Validations: make(map[string]string),
	}

	// Split into type and modifiers: "string [required, max:255]"
	parts := strings.Split(l.input, "[")

	typePart := strings.TrimSpace(parts[0])
	if typePart == "" {
		return nil, fmt.Errorf("empty field type: %s", l.input)
	}

	// Parse the type
	if err := l.parseType(typePart, fd); err != nil {
		return nil, err
	}

	// Parse modifiers if present
	if len(parts) > 1 {
		modifierStr := strings.TrimRight(parts[1], "]")
		if err := l.parseModifiers(modifierStr, fd); err != nil {
			return nil, err
		}
	}

	// Validate logic
	if err := fd.Validate(); err != nil {
		return nil, err
	}

	return fd, nil
}

// parseType parses the type portion (e.g. "string", "relation(User)", "array(int)", "enum(Status)")
func (l *Lexer) parseType(typeStr string, fd *FieldDefinition) error {
	typeStr = strings.TrimSpace(typeStr)

	// Check for relation(EntityName)
	if strings.HasPrefix(typeStr, "relation(") {
		fd.Type = "relation"
		targetEntity := strings.TrimPrefix(typeStr, "relation(")
		targetEntity = strings.TrimSuffix(targetEntity, ")")
		fd.TargetEntity = strings.TrimSpace(targetEntity)
		return nil
	}

	// Check for array(Type)
	if strings.HasPrefix(typeStr, "array(") {
		fd.Type = "array"
		innerType := strings.TrimPrefix(typeStr, "array(")
		innerType = strings.TrimSuffix(innerType, ")")
		fd.TargetType = strings.TrimSpace(innerType)
		return nil
	}

	// Check for enum(Name)
	if strings.HasPrefix(typeStr, "enum(") {
		fd.Type = "enum"
		enumName := strings.TrimPrefix(typeStr, "enum(")
		enumName = strings.TrimSuffix(enumName, ")")
		fd.TargetType = strings.TrimSpace(enumName)
		return nil
	}

	// Built-in types
	validTypes := make(map[string]bool, len(specmeta.FieldTypes))
	for _, typeName := range specmeta.FieldTypes {
		validTypes[typeName] = true
	}

	if !validTypes[typeStr] {
		return fmt.Errorf("unknown type: %s. valid types: string, int, uuid, relation, array, enum, json, text, etc.", typeStr)
	}

	fd.Type = typeStr
	return nil
}

// parseModifiers parses modifiers inside square brackets
func (l *Lexer) parseModifiers(modStr string, fd *FieldDefinition) error {
	modifiers := strings.Split(modStr, ",")

	for _, mod := range modifiers {
		mod = strings.TrimSpace(mod)
		if mod == "" {
			continue
		}

		// Key:value modifiers
		if strings.Contains(mod, ":") {
			parts := strings.SplitN(mod, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Strip quotes if present
			value = strings.Trim(value, `"'`)

			switch key {
			case "min", "max", "gte", "gt", "lte", "lt":
				fd.Validations[key] = value
			case "default":
				// Check for function-style defaults like now(), uuid()
				if strings.HasSuffix(value, "()") {
					fd.DefaultIsFunc = true
					fd.DefaultValue = value[:len(value)-2] // strip ()
				} else {
					fd.DefaultValue = value
				}
			case "on_delete":
				validOnDelete := make(map[string]bool, len(specmeta.OnDeleteValues))
				for _, valueName := range specmeta.OnDeleteValues {
					validOnDelete[valueName] = true
				}
				if !validOnDelete[value] {
					return fmt.Errorf("unknown on_delete value: %s. valid: cascade, set_null, restrict, no_action", value)
				}
				fd.OnDelete = value
			case "regex":
				fd.Validations["regex"] = value
			default:
				return fmt.Errorf("unknown modifier: %s", key)
			}
			continue
		}

		// Flag modifiers (no value)
		switch mod {
		case "primary":
			fd.IsPrimary = true
		case "optional":
			fd.IsOptional = true
		case "required":
			fd.IsRequired = true
		case "unique":
			fd.IsUnique = true
		case "hidden":
			fd.IsHidden = true
		case "many":
			fd.IsMany = true
			fd.RelationType = "many-to-many"
		case "email":
			fd.Validations["email"] = "true"
		case "url":
			fd.Validations["url"] = "true"
		case "ipv4":
			fd.Validations["ipv4"] = "true"
		default:
			return fmt.Errorf("unknown modifier flag: %s", mod)
		}
	}

	return nil
}

// Validate checks the logical consistency of a FieldDefinition
func (fd *FieldDefinition) Validate() error {
	// Cannot be both primary and optional
	if fd.IsPrimary && fd.IsOptional {
		return fmt.Errorf("field cannot be both primary and optional")
	}

	// Primary implies required
	if fd.IsPrimary {
		fd.IsRequired = true
	}

	// If both primary and required, ignore redundant required
	if fd.IsPrimary && fd.IsRequired {
		fd.IsRequired = false // Primary already implies required
	}

	// Optional implies not required
	if fd.IsOptional {
		fd.IsRequired = false
	}

	// on_delete:set_null is only valid for optional relation fields
	if fd.Type == "relation" && fd.OnDelete == "set_null" && !fd.IsOptional {
		return fmt.Errorf("on_delete:set_null is only allowed for optional relation fields")
	}

	// Validate regex if present
	if regex, ok := fd.Validations["regex"]; ok {
		if _, err := regexp.Compile(regex); err != nil {
			return fmt.Errorf("invalid regex '%s': %v", regex, err)
		}
	}

	// Set default relation type for relation fields
	if fd.Type == "relation" && fd.RelationType == "" {
		fd.RelationType = "many-to-one"
		if fd.IsUnique {
			fd.RelationType = "one-to-one"
		}
	}

	return nil
}

// ParseFieldString is a convenience function for parsing a field string
func ParseFieldString(fieldString string) (*FieldDefinition, error) {
	lexer := NewLexer(fieldString)
	return lexer.Parse()
}

// ParseFieldsMap parses a map of fields (as in RawEntity.Fields)
func ParseFieldsMap(fieldsMap map[string]string) (map[string]*FieldDefinition, error) {
	result := make(map[string]*FieldDefinition)

	for fieldName, fieldDef := range fieldsMap {
		fd, err := ParseFieldString(fieldDef)
		if err != nil {
			return nil, fmt.Errorf("error parsing field '%s': %w", fieldName, err)
		}
		fd.Name = fieldName
		result[fieldName] = fd
	}

	return result, nil
}
