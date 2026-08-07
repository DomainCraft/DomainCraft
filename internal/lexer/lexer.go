package lexer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/specmeta"
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
	IsReadonly bool
	IsRequired bool

	// Relations
	OnDelete string // cascade, set_null, restrict, no_action
	IsMany   bool   // for many-to-many (parser infers RelationType from this)

	// OldName is a rename hint for the migration engine: the field was previously
	// named OldName (e.g. `name: string [required, old_name: title]`). Bridges use
	// it to generate a safe RenameColumn instead of DropColumn + AddColumn, which
	// would destroy the column's data on a real database.
	OldName string

	// Validation
	Validations   map[string]string // min, max, email, url, regex, gte, lt, lte, gt
	DefaultValue  string
	DefaultIsFunc bool // true if default:now()
}

// Lexer parses field definition strings into FieldDefinition
type Lexer struct {
	input string
}

// NewLexer creates a new Lexer
func NewLexer(input string) *Lexer {
	return &Lexer{input: strings.TrimSpace(input)}
}

// Parse parses a full field definition: "string [required, max:255]"
func (l *Lexer) Parse() (*FieldDefinition, error) {
	fd := &FieldDefinition{
		Validations: make(map[string]string),
	}

	// Split into type and modifiers: "string [required, max:255]". Split at the
	// first "[" only; the rest (modifiers, which may themselves contain brackets,
	// e.g. default values like "[a, b, c]") is handled by parseModifiers.
	parts := strings.SplitN(l.input, "[", 2)

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
		modifierStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), "]")
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
		if fd.TargetEntity == "" {
			return fmt.Errorf("relation type requires a target entity name")
		}
		return nil
	}

	// Check for array(Type)
	if strings.HasPrefix(typeStr, "array(") {
		fd.Type = "array"
		innerType := strings.TrimPrefix(typeStr, "array(")
		innerType = strings.TrimSuffix(innerType, ")")
		fd.TargetType = strings.TrimSpace(innerType)
		if fd.TargetType == "" {
			return fmt.Errorf("array type requires an element type")
		}
		return nil
	}

	// Check for enum(Name)
	if strings.HasPrefix(typeStr, "enum(") {
		fd.Type = "enum"
		enumName := strings.TrimPrefix(typeStr, "enum(")
		enumName = strings.TrimSuffix(enumName, ")")
		fd.TargetType = strings.TrimSpace(enumName)
		if fd.TargetType == "" {
			return fmt.Errorf("enum type requires a name")
		}
		return nil
	}

	// Built-in types
	if !specmeta.IsFieldType(typeStr) {
		return fmt.Errorf("unknown type: %s. valid types: string, int, uuid, relation, array, enum, json, text, etc.", typeStr)
	}

	fd.Type = typeStr
	return nil
}

// parseModifiers parses modifiers inside square brackets
func (l *Lexer) parseModifiers(modStr string, fd *FieldDefinition) error {
	modifiers := splitModifiers(modStr)

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
		case "min", "max", "gte", "gt", "lte", "lt", "regex":
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
			if !specmeta.IsOnDeleteValue(value) {
				return fmt.Errorf("unknown on_delete value: %s. valid: cascade, set_null, restrict, no_action", value)
			}
			fd.OnDelete = value
		case "old_name":
			// Rename hint: `name: string [required, old_name: title]`. The value is
			// the field's previous name; the migration engine + bridges turn it into
			// a safe column rename instead of a destructive drop + add.
			if value == "" {
				return fmt.Errorf("old_name requires a previous field name")
			}
			fd.OldName = value
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
		case "readonly":
			fd.IsReadonly = true
		case "many":
			fd.IsMany = true
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

// splitModifiers splits a modifier clause on commas while respecting square
// brackets (array default values like "[a, b, c]") and single/double quotes.
// Without this, "default:[a, b, c]" would be chopped at every comma and the
// default value would never round-trip through parse → serialize → parse.
func splitModifiers(s string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			cur.WriteByte(c)
		case '[':
			depth++
			cur.WriteByte(c)
		case ']':
			if depth > 0 {
				depth--
			}
			cur.WriteByte(c)
		case ',':
			if depth == 0 {
				out = append(out, cur.String())
				cur.Reset()
				continue
			}
			cur.WriteByte(c)
		default:
			cur.WriteByte(c)
		}
	}
	out = append(out, cur.String())
	return out
}

// Validate checks the logical consistency of a FieldDefinition
func (fd *FieldDefinition) Validate() error {
	// Cannot be both primary and optional
	if fd.IsPrimary && fd.IsOptional {
		return fmt.Errorf("field cannot be both primary and optional")
	}

	// Cannot be both required and optional
	if fd.IsRequired && fd.IsOptional {
		return fmt.Errorf("field cannot be both required and optional")
	}

	// on_delete is only valid on relation fields
	if fd.OnDelete != "" && fd.Type != "relation" {
		return fmt.Errorf("on_delete is only valid on relation fields")
	}

	// many modifier is only valid on relation fields
	if fd.IsMany && fd.Type != "relation" {
		return fmt.Errorf("many modifier is only valid on relation fields")
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

	// old_name must be a valid identifier (like the field name itself).
	if fd.OldName != "" && !isValidIdentifier(fd.OldName) {
		return fmt.Errorf("old_name %q is not a valid field name", fd.OldName)
	}

	return nil
}

// isValidIdentifier reports whether s is a valid domain.yaml field name.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') {
				return false
			}
			continue
		}
		if r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// ParseFieldString is a convenience function for parsing a field string
func ParseFieldString(fieldString string) (*FieldDefinition, error) {
	lexer := NewLexer(fieldString)
	return lexer.Parse()
}
