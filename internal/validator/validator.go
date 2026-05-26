package validator

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
)

// ValidationError describes a logical validation error.
type ValidationError struct {
	Entity  string
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("Error in entity '%s', field '%s': %s", e.Entity, e.Field, e.Message)
	}
	return fmt.Sprintf("Error in entity '%s': %s", e.Entity, e.Message)
}

// Validator checks ParsedSchema for logical consistency.
type Validator struct {
	schema *parser.ParsedSchema
}

func New(schema *parser.ParsedSchema) *Validator {
	return &Validator{schema: schema}
}

func (v *Validator) Validate() []ValidationError {
	if v == nil || v.schema == nil {
		return []ValidationError{{Entity: "<schema>", Message: "schema is nil"}}
	}

	var errs []ValidationError

	// Validate project-level configuration values.
	if v.schema.Database != "" && !slices.Contains(specmeta.Databases, v.schema.Database) {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown database %q; allowed: %s", v.schema.Database, strings.Join(specmeta.Databases, ", "))})
	}
	if v.schema.Auth != "" && v.schema.Auth != "jwt" && v.schema.Auth != "none" {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown auth %q; allowed: jwt, none", v.schema.Auth)})
	}
	if v.schema.APIStyle != "" && !slices.Contains(specmeta.APIStyles, v.schema.APIStyle) {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown api_style %q; allowed: %s", v.schema.APIStyle, strings.Join(specmeta.APIStyles, ", "))})
	}

	for _, entityName := range v.schema.EntityOrder {
		entity := v.schema.Entities[entityName]
		if entity == nil {
			continue
		}

		if !hasPrimaryKey(entity) {
			errs = append(errs, ValidationError{Entity: entityName, Message: "entity must have at least one primary key"})
		}

		for _, fieldName := range entity.FieldOrder {
			field := entity.Fields[fieldName]
			if field == nil {
				continue
			}

			if field.IsRelation {
				if _, ok := v.schema.Entities[field.RelationTarget]; !ok {
					errs = append(errs, ValidationError{
						Entity:  entityName,
						Field:   fieldName,
						Message: fmt.Sprintf("relation target '%s' does not exist", field.RelationTarget),
					})
					continue
				}

				if field.OnDelete == "set_null" && !field.IsOptional {
					errs = append(errs, ValidationError{
						Entity:  entityName,
						Field:   fieldName,
						Message: "on_delete:set_null requires optional field",
					})
				}
			}

			if field.Type == "enum" {
				if field.TargetType != "" {
					if _, ok := v.schema.Enums[field.TargetType]; !ok {
						errs = append(errs, ValidationError{
							Entity:  entityName,
							Field:   fieldName,
							Message: fmt.Sprintf("enum '%s' is not defined in enums section", field.TargetType),
						})
					}
				}
			}

			if field.Type == "array" && field.TargetType != "" {
				inner := strings.ToLower(field.TargetType)
				if !specmeta.IsPrimitive(inner) {
					if _, ok := v.schema.Enums[field.TargetType]; !ok {
						errs = append(errs, ValidationError{
							Entity:  entityName,
							Field:   fieldName,
							Message: fmt.Sprintf("array element type '%s' is not a primitive or defined enum", field.TargetType),
						})
					}
				}
			}
		}

		// Validate index field names exist in the entity.
		for i, idx := range entity.Indexes {
			for _, idxField := range idx.Fields {
				if _, ok := entity.Fields[idxField]; !ok {
					errs = append(errs, ValidationError{
						Entity:  entityName,
						Field:   idxField,
						Message: fmt.Sprintf("index %d references unknown field '%s'", i, idxField),
					})
				}
			}
		}

		// Validate seed data field names exist in the entity.
		for i, seedEntry := range entity.Seed {
			for seedField := range seedEntry {
				if _, ok := entity.Fields[seedField]; !ok {
					errs = append(errs, ValidationError{
						Entity:  entityName,
						Field:   seedField,
						Message: fmt.Sprintf("seed entry %d references unknown field '%s'", i, seedField),
					})
				}
			}
		}
	}

	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Entity == errs[j].Entity {
			return errs[i].Field < errs[j].Field
		}
		return errs[i].Entity < errs[j].Entity
	})

	return errs
}

func hasPrimaryKey(entity *parser.ParsedEntity) bool {
	for _, fieldName := range entity.FieldOrder {
		field := entity.Fields[fieldName]
		if field != nil && field.IsPrimary {
			return true
		}
	}
	return false
}
