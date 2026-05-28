package validator

import (
	"fmt"
	"regexp"
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
	Warning bool // non-fatal hint, not a hard error
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

var validIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var validIndexTypes = map[string]bool{
	"btree": true, "hash": true, "gist": true, "gin": true, "brin": true,
}

var validMultiTenancyModes = map[string]bool{
	"column": true, "schema": true, "database": true,
}

var validCacheProviders = map[string]bool{
	"redis": true, "memcached": true, "in-memory": true,
}

var validPermissionKeys = map[string]bool{
	"read": true, "create": true, "update": true, "delete": true,
}

// stringValidationModifiers are only meaningful on string-type fields.
var stringValidationModifiers = map[string]bool{
	"min": true, "max": true, "email": true, "url": true, "ipv4": true, "regex": true,
}

// numericValidationModifiers are only meaningful on numeric-type fields.
var numericValidationModifiers = map[string]bool{
	"gte": true, "gt": true, "lte": true, "lt": true,
}

// numericDefaults are default values that make sense for numeric types.
var boolValues = map[string]bool{"true": true, "false": true}

func (v *Validator) Validate() []ValidationError {
	if v == nil || v.schema == nil {
		return []ValidationError{{Entity: "<schema>", Message: "schema is nil"}}
	}

	var errs []ValidationError

	errs = append(errs, v.validateProject()...)
	errs = append(errs, v.validateEnums()...)

	for _, entityName := range v.schema.EntityOrder {
		entity := v.schema.Entities[entityName]
		if entity == nil {
			continue
		}
		errs = append(errs, v.validateEntity(entityName, entity)...)
	}

	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Warning != errs[j].Warning {
			return !errs[i].Warning // errors first
		}
		if errs[i].Entity == errs[j].Entity {
			return errs[i].Field < errs[j].Field
		}
		return errs[i].Entity < errs[j].Entity
	})

	return errs
}

// --- project-level ---

func (v *Validator) validateProject() []ValidationError {
	var errs []ValidationError
	p := v.schema.Project

	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: "project name must not be empty"})
	}

	if len(v.schema.Entities) == 0 {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: "no entities defined", Warning: true})
	}

	if v.schema.Database != "" && !slices.Contains(specmeta.Databases, v.schema.Database) {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown database %q; allowed: %s", v.schema.Database, strings.Join(specmeta.Databases, ", "))})
	}
	if v.schema.Auth != "" && v.schema.Auth != "jwt" && v.schema.Auth != "none" {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown auth %q; allowed: jwt, none", v.schema.Auth)})
	}
	if v.schema.APIStyle != "" && !slices.Contains(specmeta.APIStyles, v.schema.APIStyle) {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown api_style %q; allowed: %s", v.schema.APIStyle, strings.Join(specmeta.APIStyles, ", "))})
	}

	// Multi-tenancy
	if p.MultiTenancy != nil && p.MultiTenancy.Enabled {
		if p.MultiTenancy.Mode != "" && !validMultiTenancyModes[p.MultiTenancy.Mode] {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown multi_tenancy.mode %q; allowed: column, schema, database", p.MultiTenancy.Mode)})
		}
	}

	// Cache
	if p.Cache != nil && p.Cache.Enabled {
		if p.Cache.Provider != "" && !validCacheProviders[p.Cache.Provider] {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown cache.provider %q; allowed: redis, memcached, in-memory", p.Cache.Provider)})
		}
		if p.Cache.TTLSeconds < 0 {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: "cache.ttl_seconds must be non-negative", Warning: true})
		}
	}

	// CORS
	if p.CORS != nil && p.CORS.Enabled && len(p.CORS.Origins) == 0 {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: "cors.enabled is true but no origins specified", Warning: true})
	}

	return errs
}

// --- enum-level ---

func (v *Validator) validateEnums() []ValidationError {
	var errs []ValidationError
	for name, values := range v.schema.Enums {
		if !validIdentifier.MatchString(name) {
			errs = append(errs, ValidationError{Entity: "<enum>", Field: name, Message: "enum name is not a valid identifier"})
		}
		if len(values) == 0 {
			errs = append(errs, ValidationError{Entity: "<enum>", Field: name, Message: "enum must have at least one value"})
			continue
		}
		seen := make(map[string]bool, len(values))
		for _, val := range values {
			if val == "" {
				errs = append(errs, ValidationError{Entity: "<enum>", Field: name, Message: "enum value must not be empty"})
			}
			lower := strings.ToLower(val)
			if seen[lower] {
				errs = append(errs, ValidationError{Entity: "<enum>", Field: name, Message: fmt.Sprintf("duplicate enum value %q (case-insensitive collision)", val), Warning: true})
			}
			seen[lower] = true
		}
	}
	return errs
}

// --- entity-level ---

func (v *Validator) validateEntity(entityName string, entity *parser.ParsedEntity) []ValidationError {
	var errs []ValidationError

	// Entity name must be a valid identifier.
	if !validIdentifier.MatchString(entityName) {
		errs = append(errs, ValidationError{Entity: entityName, Message: "entity name is not a valid identifier"})
	}

	// Must have at least one field.
	if len(entity.Fields) == 0 {
		errs = append(errs, ValidationError{Entity: entityName, Message: "entity has no fields"})
	}

	// Must have a primary key.
	if !hasPrimaryKey(entity) {
		errs = append(errs, ValidationError{Entity: entityName, Message: "entity must have at least one primary key"})
	}

	// Duplicate feature names.
	featureSeen := make(map[string]bool)
	for f := range entity.Features {
		if featureSeen[f] {
			errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("duplicate feature %q", f), Warning: true})
		}
		featureSeen[f] = true
	}

	// Track DB column names for collision detection.
	columnNames := make(map[string]string) // column -> field name

	for _, fieldName := range entity.FieldOrder {
		field := entity.Fields[fieldName]
		if field == nil {
			continue
		}

		errs = append(errs, v.validateField(entityName, fieldName, field)...)

		// DB column collision.
		col := field.DatabaseColumnName
		if col == "" {
			col = strings.ToLower(fieldName)
		}
		if prev, exists := columnNames[col]; exists {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("database column '%s' collides with field '%s'", col, prev),
			})
		}
		columnNames[col] = fieldName
	}

	// Indexes.
	for i, idx := range entity.Indexes {
		errs = append(errs, v.validateIndex(entityName, i, idx, entity)...)
	}

	// Permissions.
	if entity.Permissions != nil {
		errs = append(errs, v.validatePermissions(entityName, entity.Permissions)...)
	}

	// Seed data.
	for i, seedEntry := range entity.Seed {
		errs = append(errs, v.validateSeedEntry(entityName, entity, i, seedEntry)...)
	}

	return errs
}

// --- field-level ---

func (v *Validator) validateField(entityName string, fieldName string, field *parser.ParsedField) []ValidationError {
	var errs []ValidationError

	// Field name must be a valid identifier.
	if !validIdentifier.MatchString(fieldName) {
		errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "field name is not a valid identifier"})
	}

	// Feature field collision — warn only if user-defined type differs from auto-generated type.
	if fdef, isFeatureField := specmeta.FeatureFieldDefs[fieldName]; isFeatureField {
		userType := strings.ToLower(field.Type)
		if userType != "" && userType != fdef.Type {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("user-defined type '%s' overrides feature field '%s' (auto-generated type: '%s')", field.Type, fieldName, fdef.Type),
				Warning: true,
			})
		}
	}

	// Relation-specific checks.
	if field.IsRelation {
		if field.TargetEntity == "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "relation field must specify a target entity"})
		}
		if _, ok := v.schema.Entities[field.TargetEntity]; !ok && field.TargetEntity != "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("relation target '%s' does not exist", field.TargetEntity)})
		}
		if field.OnDelete == "set_null" && !field.IsOptional {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "on_delete:set_null requires optional field"})
		}
		if field.IsMany && field.OnDelete != "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "on_delete is not applicable on many-to-many relations", Warning: true})
		}
		if field.IsMany && field.IsUnique {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "unique is contradictory on a many-to-many relation"})
		}
		// Self-referential required cascade is dangerous.
		if field.TargetEntity == entityName && !field.IsOptional && field.OnDelete == "cascade" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "self-referential required cascade delete may cause recursive deletion", Warning: true})
		}
		// Required relation without on_delete — warn.
		if !field.IsOptional && !field.IsMany && field.OnDelete == "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "required relation has no on_delete specified; default behavior may not match expectations", Warning: true})
		}
	} else {
		// Non-relation fields should not have relation-only modifiers.
		if field.OnDelete != "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "on_delete is only valid on relation fields"})
		}
		if field.IsMany {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "many modifier is only valid on relation fields"})
		}
	}

	// Primary key constraints.
	if field.IsPrimary {
		if field.Type == "array" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "primary key cannot be an array type"})
		}
		if field.IsOptional {
			// Already caught by lexer, but double-check.
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "primary key cannot be optional"})
		}
	}

	// Enum reference.
	if field.Type == "enum" && field.TargetType != "" {
		if _, ok := v.schema.Enums[field.TargetType]; !ok {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("enum '%s' is not defined in enums section", field.TargetType)})
		}
	}

	// Array element type.
	if field.Type == "array" && field.TargetType != "" {
		inner := strings.ToLower(field.TargetType)
		if !specmeta.IsPrimitive(inner) {
			if _, ok := v.schema.Enums[field.TargetType]; !ok {
				errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("array element type '%s' is not a primitive or defined enum", field.TargetType)})
			}
		}
	}

	// Validation modifiers must be type-appropriate.
	ftype := strings.ToLower(field.Type)
	isNumeric := specmeta.IsNumeric(ftype)
	isString := ftype == "string" || ftype == "text"

	for mod := range field.Validations {
		if stringValidationModifiers[mod] && !isString {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("validation '%s' is only applicable to string/text fields, not %s", mod, ftype), Warning: true})
		}
		if numericValidationModifiers[mod] && !isNumeric {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("validation '%s' is only applicable to numeric fields, not %s", mod, ftype), Warning: true})
		}
	}

	// Default value type check.
	if field.DefaultValue != "" && !field.DefaultIsFunc {
		errs = append(errs, validateDefaultValue(entityName, fieldName, ftype, field.DefaultValue)...)
	}

	return errs
}

func validateDefaultValue(entityName, fieldName, ftype, defaultVal string) []ValidationError {
	var errs []ValidationError
	switch {
	case specmeta.IsNumeric(ftype):
		if boolValues[defaultVal] {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("default value '%s' is not valid for numeric type %s", defaultVal, ftype), Warning: true})
		}
	case ftype == "boolean":
		if !boolValues[defaultVal] {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("default value '%s' is not valid for boolean (expected true/false)", defaultVal), Warning: true})
		}
	}
	return errs
}

// --- index-level ---

func (v *Validator) validateIndex(entityName string, idxNum int, idx *parser.ParsedIndex, entity *parser.ParsedEntity) []ValidationError {
	var errs []ValidationError

	if len(idx.Fields) == 0 {
		errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d has no fields", idxNum)})
	}

	// Duplicate fields in index.
	fieldSeen := make(map[string]bool)
	for _, f := range idx.Fields {
		if fieldSeen[f] {
			errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d has duplicate field '%s'", idxNum, f), Warning: true})
		}
		fieldSeen[f] = true

		if _, ok := entity.Fields[f]; !ok {
			errs = append(errs, ValidationError{Entity: entityName, Field: f, Message: fmt.Sprintf("index %d references unknown field '%s'", idxNum, f)})
		}
	}

	// Index type.
	if idx.Type != "" && !validIndexTypes[idx.Type] {
		errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d has unknown type %q; allowed: btree, hash, gist, gin, brin", idxNum, idx.Type)})
	}

	// Sort array length must match fields length.
	if len(idx.Sort) > 0 && len(idx.Sort) != len(idx.Fields) {
		errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d: sort array length (%d) does not match fields length (%d)", idxNum, len(idx.Sort), len(idx.Fields))})
	}

	// Sort values must be asc or desc.
	for _, s := range idx.Sort {
		if s != "asc" && s != "desc" {
			errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d: invalid sort value %q; allowed: asc, desc", idxNum, s)})
		}
	}

	return errs
}

// --- permission-level ---

func (v *Validator) validatePermissions(entityName string, perms *parser.ParsedPermissions) []ValidationError {
	var errs []ValidationError

	// Check for unknown permission keys by looking at the raw data.
	// We can't directly check, but we can validate role values.
	validateRoles := func(operation string, roles []string) {
		for _, role := range roles {
			if role == "" {
				errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("permission '%s' has an empty role", operation)})
			}
		}
	}

	validateRoles("read", perms.Read)
	validateRoles("create", perms.Create)
	validateRoles("update", perms.Update)
	validateRoles("delete", perms.Delete)

	return errs
}

// --- seed-level ---

func (v *Validator) validateSeedEntry(entityName string, entity *parser.ParsedEntity, entryIdx int, entry map[string]interface{}) []ValidationError {
	var errs []ValidationError

	// Check that seed fields exist.
	for seedField := range entry {
		if _, ok := entity.Fields[seedField]; !ok {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   seedField,
				Message: fmt.Sprintf("seed entry %d references unknown field '%s'", entryIdx, seedField),
			})
		}
	}

	// Check required relation FK dependencies.
	for _, fieldName := range entity.FieldOrder {
		field := entity.Fields[fieldName]
		if field == nil || !field.IsRelation || field.IsOptional || field.IsMany {
			continue
		}
		if _, inSeed := entry[fieldName]; inSeed {
			continue
		}
		if field.DefaultValue != "" {
			continue
		}
		target := v.schema.Entities[field.RelationTarget]
		if target == nil || len(target.Seed) == 0 {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("seed entry %d: required relation '%s' -> '%s' is not seeded (add FK value to seed or add seed data to %s)", entryIdx, fieldName, field.RelationTarget, field.RelationTarget),
			})
		}
	}

	return errs
}

// --- helpers ---

func hasPrimaryKey(entity *parser.ParsedEntity) bool {
	for _, fieldName := range entity.FieldOrder {
		field := entity.Fields[fieldName]
		if field != nil && field.IsPrimary {
			return true
		}
	}
	return false
}

