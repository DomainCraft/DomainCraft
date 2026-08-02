package validator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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

// boolValues are valid boolean default values.
var boolValues = map[string]bool{"true": true, "false": true}

func (v *Validator) Validate() []ValidationError {
	if v == nil || v.schema == nil {
		return []ValidationError{{Entity: "<schema>", Message: "schema is nil"}}
	}

	var errs []ValidationError

	errs = append(errs, v.validateProject()...)
	errs = append(errs, v.validateEnums()...)

	// Entity vs enum name collision.
	if v.schema.Enums != nil {
		for _, entityName := range v.schema.EntityOrder {
			if _, isEnum := v.schema.Enums[entityName]; isEnum {
				errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("entity name %q collides with an enum of the same name; templates may produce ambiguous output", entityName), Warning: true})
			}
		}
	}

	for _, entityName := range v.schema.EntityOrder {
		entity := v.schema.Entities[entityName]
		if entity == nil {
			continue
		}
		errs = append(errs, v.validateEntity(entityName, entity)...)
	}

	errs = append(errs, v.validateCircularRelations()...)

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

	if v.schema.Database != "" && !specmeta.IsDatabase(v.schema.Database) {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown database %q; allowed: %s", v.schema.Database, strings.Join(specmeta.Databases, ", "))})
	}
	// Auth validation
	if v.schema.Auth != nil {
		errs = append(errs, v.validateAuth()...)
	}
	if v.schema.APIStyle != "" && !specmeta.IsAPIStyle(v.schema.APIStyle) {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown api_style %q; allowed: %s", v.schema.APIStyle, strings.Join(specmeta.APIStyles, ", "))})
	}

	// Multi-tenancy
	if p.MultiTenancy != nil && p.MultiTenancy.Enabled {
		if p.MultiTenancy.Mode != "" && !specmeta.IsMultiTenancyMode(p.MultiTenancy.Mode) {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown multi_tenancy.mode %q; allowed: %s", p.MultiTenancy.Mode, strings.Join(specmeta.MultiTenancyModes, ", "))})
		}
	}

	// Cache
	if p.Cache != nil && p.Cache.Enabled {
		if p.Cache.Provider != "" && !specmeta.IsCacheProvider(p.Cache.Provider) {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown cache.provider %q; allowed: %s", p.Cache.Provider, strings.Join(specmeta.CacheProviders, ", "))})
		}
		if p.Cache.TTLSeconds < 0 {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: "cache.ttl_seconds must be non-negative", Warning: true})
		}
	}
	if p.Cache != nil && !p.Cache.Enabled && (p.Cache.Provider != "" || p.Cache.ConnectionString != "") {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: "cache.provider or cache.connection_string specified but cache.enabled is false", Warning: true})
	}
	if p.Cache != nil && p.Cache.Enabled && p.Cache.Provider != "" && p.Cache.ConnectionString == "" {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("cache.provider %q specified but cache.connection_string is empty", p.Cache.Provider), Warning: true})
	}

	// CORS
	if p.CORS != nil && p.CORS.Enabled && len(p.CORS.Origins) == 0 {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: "cors.enabled is true but no origins specified", Warning: true})
	}

	// Deploy
	if p.Deploy != nil {
		if p.Deploy.Port != 0 && (p.Deploy.Port < 1 || p.Deploy.Port > 65535) {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("deploy.port must be between 1 and 65535, got %d", p.Deploy.Port)})
		}
	}

	return errs
}

// validateCircularRelations detects cycles in the FK graph (entity A references
// B which, directly or transitively, references A). Such cycles can produce
// problematic delete orders and seed ordering in generated code.
//
// Only FK-bearing relations participate in the graph: self-referential links
// (e.g. Category.parentId -> Category) are excluded because they are a normal
// hierarchy pattern, and many-to-many links (which own no FK column) are excluded
// because the FK physically lives on the other side.
func (v *Validator) validateCircularRelations() []ValidationError {
	var errs []ValidationError

	// Build adjacency: entity -> list of target entities it references via an FK.
	adj := make(map[string][]string)
	for _, entityName := range v.schema.EntityOrder {
		entity := v.schema.Entities[entityName]
		if entity == nil {
			continue
		}
		for _, fieldName := range entity.FieldOrder {
			field := entity.Fields[fieldName]
			if field == nil || !field.IsRelation() || field.TargetEntity == "" {
				continue
			}
			// Skip many-to-many links (no FK on this side) and self-references.
			if field.IsMany || field.TargetEntity == entityName {
				continue
			}
			adj[entityName] = append(adj[entityName], field.TargetEntity)
		}
	}

	// Standard DFS cycle detection (white/gray/black). All nodes on a gray stack
	// when a back-edge is found are part of the cycle.
	const (
		white = iota
		gray
		black
	)
	color := make(map[string]int)
	inCycle := make(map[string]bool)
	stack := make([]string, 0, len(v.schema.EntityOrder))

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		stack = append(stack, node)
		for _, target := range adj[node] {
			switch color[target] {
			case white:
				dfs(target)
			case gray:
				// Back-edge found: mark everything from target up the stack.
				marking := false
				for _, n := range stack {
					if n == target {
						marking = true
					}
					if marking {
						inCycle[n] = true
					}
				}
			case black:
				// already fully explored
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
	}

	for _, entityName := range v.schema.EntityOrder {
		if color[entityName] == white {
			dfs(entityName)
		}
	}

	sorted := make([]string, 0, len(inCycle))
	for name := range inCycle {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	for _, entityName := range sorted {
		errs = append(errs, ValidationError{
			Entity:  entityName,
			Message: "entity is part of a circular relation chain (A → B → … → A); generated delete/seed ordering may be ambiguous",
			Warning: true,
		})
	}

	return errs
}

func (v *Validator) validateAuth() []ValidationError {
	var errs []ValidationError
	auth := v.schema.Auth

	if auth.Type != "" && !specmeta.IsAuthType(auth.Type) {
		errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("unknown auth.type %q; allowed: %s", auth.Type, strings.Join(specmeta.AuthTypes, ", "))})
	}

	if auth.Type == "none" {
		return errs
	}

	// Entity validation
	if auth.Entity != "" {
		entity, ok := v.schema.Entities[auth.Entity]
		if !ok {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("auth.entity %q does not exist", auth.Entity)})
		} else if !specmeta.HasEmailAndPassword(entity.FieldOrder) {
			hasEmail, hasPassword := specmeta.AuthFieldState(entity.FieldOrder)
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("auth.entity %q must have both 'email' and 'password' fields (missing: %s)", auth.Entity, missingFields(!hasEmail, !hasPassword))})
		}
	} else {
		// Auto-detect: find entity with email + password using specmeta helper.
		entityFields := v.buildEntityFields()
		if specmeta.FindAuthEntity(v.schema.EntityOrder, entityFields) == "" {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("auth.type is %q but no entity has both 'email' and 'password' fields; specify auth.entity explicitly", auth.Type), Warning: true})
		}
	}

	// Roles must be valid identifiers
	for _, role := range auth.Roles {
		if !validIdentifier.MatchString(role) {
			errs = append(errs, ValidationError{Entity: "<schema>", Message: fmt.Sprintf("auth.roles: %q is not a valid identifier", role)})
		}
	}

	// Validate that auth roles are used in at least one entity's permissions.
	// When auth.roles is non-empty, check that permission roles are declared.
	// When auth.roles is empty, warn if any entity references named roles.
	roleSet := make(map[string]bool, len(auth.Roles))
	for _, r := range auth.Roles {
		roleSet[r] = true
	}
	for _, entityName := range v.schema.EntityOrder {
		entity := v.schema.Entities[entityName]
		if entity == nil || entity.Permissions == nil {
			continue
		}
		for _, roles := range [][]string{
			entity.Permissions.Read,
			entity.Permissions.Create,
			entity.Permissions.Update,
			entity.Permissions.Delete,
		} {
			for _, r := range roles {
				if specmeta.IsReservedPermissionToken(r) || r == "" {
					continue
				}
				if len(auth.Roles) > 0 && !roleSet[r] {
					errs = append(errs, ValidationError{
						Entity:  entityName,
						Message: fmt.Sprintf("permission role %q is not defined in auth.roles; add it to auth.roles or remove from permissions", r),
						Warning: true,
					})
				} else if len(auth.Roles) == 0 {
					errs = append(errs, ValidationError{
						Entity:  entityName,
						Message: fmt.Sprintf("permission role %q is used but auth.roles is empty; define auth.roles to declare valid roles", r),
						Warning: true,
					})
				}
			}
		}
	}

	return errs
}

// --- enum-level ---

// buildEntityFields returns entity name -> field order mapping for specmeta helpers.
func (v *Validator) buildEntityFields() map[string][]string {
	return specmeta.BuildEntityFields(v.schema.EntityOrder, func(name string) []string {
		if entity := v.schema.Entities[name]; entity != nil {
			return entity.FieldOrder
		}
		return nil
	})
}

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

	// Check for unused enums.
	if v.schema.Enums != nil {
		usedEnums := make(map[string]bool)
		for _, entity := range v.schema.Entities {
			if entity == nil {
				continue
			}
			for _, field := range entity.Fields {
				if field == nil {
					continue
				}
				if field.Type == "enum" && field.TargetType != "" {
					usedEnums[field.TargetType] = true
				}
				if field.Type == "array" && field.TargetType != "" {
					usedEnums[field.TargetType] = true
				}
			}
		}
		enumNames := make([]string, 0, len(v.schema.Enums))
		for name := range v.schema.Enums {
			enumNames = append(enumNames, name)
		}
		sort.Strings(enumNames)
		for _, enumName := range enumNames {
			if !usedEnums[enumName] {
				errs = append(errs, ValidationError{Entity: "<enum>", Field: enumName, Message: "enum is declared but never used by any entity field", Warning: true})
			}
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
	pkCount := countPrimaryKeys(entity)
	if pkCount == 0 {
		errs = append(errs, ValidationError{Entity: entityName, Message: "entity must have at least one primary key"})
	} else if pkCount > 1 {
		// Must not have duplicate primary keys.
		errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("entity has %d primary keys; expected exactly one", pkCount)})
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

	// Track navigation names for collision detection.
	navigationNames := make(map[string]string) // navigation name -> field name

	for _, fieldName := range entity.FieldOrder {
		field := entity.Fields[fieldName]
		if field == nil {
			continue
		}

		errs = append(errs, v.validateField(entityName, fieldName, field)...)

		// DB column collision.
		col := field.DatabaseColumnName
		if prev, exists := columnNames[col]; exists {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("database column '%s' collides with field '%s'", col, prev),
			})
		}
		columnNames[col] = fieldName

		// Navigation name collision: two relation fields resolving to the same
		// navigation property would produce duplicate properties in generated code.
		if field.IsRelation() {
			nav := field.NavigationName()
			if prev, exists := navigationNames[nav]; exists {
				errs = append(errs, ValidationError{
					Entity:  entityName,
					Field:   fieldName,
					Message: fmt.Sprintf("navigation name '%s' collides with field '%s'", nav, prev),
				})
			}
			navigationNames[nav] = fieldName
		}
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

	// Seed unique constraint: check that seed entries don't violate unique fields.
	if len(entity.Seed) > 1 {
		uniqueFields := make(map[string]map[string]bool) // field -> set of values
		for _, field := range entity.Fields {
			if field != nil && field.IsUnique {
				uniqueFields[field.Name] = make(map[string]bool)
			}
		}
		if len(uniqueFields) > 0 {
			for i, seedEntry := range entity.Seed {
				for fieldName, values := range uniqueFields {
					seedVal, ok := seedEntry[fieldName]
					if !ok {
						continue
					}
					valStr := fmt.Sprintf("%v", seedVal)
					if values[valStr] {
						errs = append(errs, ValidationError{
							Entity:  entityName,
							Field:   fieldName,
							Message: fmt.Sprintf("seed entry %d: value %q for unique field '%s' is duplicated in seed data", i, valStr, fieldName),
						})
					}
					values[valStr] = true
				}
			}
		}
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
	// Note: on_delete value, set_null+optional, many+on_delete, and many+relation
	// constraints are already enforced by the lexer. Only cross-entity and semantic
	// checks that the lexer cannot perform belong here.
	if field.IsRelation() {
		if field.TargetEntity == "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "relation field must specify a target entity"})
		}
		if _, ok := v.schema.Entities[field.TargetEntity]; !ok && field.TargetEntity != "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("relation target '%s' does not exist", field.TargetEntity)})
		}
		if field.IsMany && field.IsUnique {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "unique is contradictory on a many-to-many relation"})
		}
		// on_delete is meaningless on many-to-many relations (join table has no FK to delete).
		if field.IsMany && field.OnDelete != "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "on_delete is meaningless on a many-to-many relation; remove it or use a many-to-one relation instead", Warning: true})
		}
		// Self-referential required cascade is dangerous.
		if field.TargetEntity == entityName && !field.IsOptional && field.OnDelete == "cascade" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "self-referential required cascade delete may cause recursive deletion", Warning: true})
		}
		// Required relation without on_delete — warn.
		if !field.IsOptional && !field.IsMany && field.OnDelete == "" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "required relation has no on_delete specified; default behavior may not match expectations", Warning: true})
		}
	}

	// Primary key constraints.
	if field.IsPrimary {
		if field.Type == "array" {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: "primary key cannot be an array type"})
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
	isString := specmeta.IsStringType(ftype)

	for mod, val := range field.Validations {
		if specmeta.IsStringValidationModifier(mod) && !isString {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("validation '%s' is only applicable to string/text fields, not %s", mod, ftype), Warning: true})
		}
		if specmeta.IsNumericValidationModifier(mod) && !isNumeric {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("validation '%s' is only applicable to numeric fields, not %s", mod, ftype), Warning: true})
		}
		// Validate that numeric validation values are actually numeric
		// (numeric modifiers AND string min/max, which are also numeric).
		if (specmeta.IsNumericValidationModifier(mod) || mod == "min" || mod == "max") && val != "" {
			if _, err := strconv.ParseFloat(val, 64); err != nil {
				errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("validation '%s' value %q is not a valid number", mod, val)})
			}
		}
	}

	// Check for contradictory range validations.
	errs = append(errs, validateContradictoryRanges(entityName, fieldName, field, isNumeric, isString)...)

	// Default value type check.
	if field.DefaultValue != "" && field.DefaultIsFunc {
		errs = append(errs, validateFuncDefault(entityName, fieldName, ftype, field.DefaultValue)...)
	} else if field.DefaultValue != "" {
		errs = append(errs, validateDefaultValue(entityName, fieldName, ftype, field.DefaultValue)...)
	}

	// Enum default must be one of the declared enum values.
	if field.Type == "enum" && field.TargetType != "" && field.DefaultValue != "" && !field.DefaultIsFunc {
		if enumValues, ok := v.schema.Enums[field.TargetType]; ok {
			found := false
			for _, v := range enumValues {
				if v == field.DefaultValue {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("default value '%s' is not a declared value of enum '%s'; valid values: %s", field.DefaultValue, field.TargetType, strings.Join(enumValues, ", "))})
			}
		}
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
	case ftype == "date":
		if !isValidDate(defaultVal) {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("default value '%s' is not a valid date (expected YYYY-MM-DD)", defaultVal), Warning: true})
		}
	case ftype == "datetime":
		if !isValidDatetime(defaultVal) {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("default value '%s' is not a valid datetime (expected ISO 8601 format, e.g. 2024-01-15T10:30:00Z)", defaultVal), Warning: true})
		}
	case ftype == "uuid":
		if !isValidUUID(defaultVal) {
			errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("default value '%s' is not a valid UUID", defaultVal), Warning: true})
		}
	}
	return errs
}

// validateFuncDefault checks that function-style defaults (e.g. now(), uuid()) are used on appropriate types.
func validateFuncDefault(entityName, fieldName, ftype, funcName string) []ValidationError {
	var errs []ValidationError
	allowedTypes, ok := specmeta.FuncDefaults[funcName]
	if !ok {
		errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("unknown default function '%s()'", funcName)})
		return errs
	}
	allowed := false
	for _, t := range allowedTypes {
		if ftype == t {
			allowed = true
			break
		}
	}
	if !allowed {
		errs = append(errs, ValidationError{Entity: entityName, Field: fieldName, Message: fmt.Sprintf("default '%s()' is not valid for type %s; allowed types: %s", funcName, ftype, strings.Join(allowedTypes, ", "))})
	}
	return errs
}

// validateContradictoryRanges checks for impossible validation ranges (e.g. min > max, gte > lt).
func validateContradictoryRanges(entityName, fieldName string, field *parser.ParsedField, isNumeric, isString bool) []ValidationError {
	var errs []ValidationError

	getFloat := func(key string) (float64, bool) {
		val, ok := field.Validations[key]
		if !ok || val == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}

	if isString {
		minVal, hasMin := getFloat("min")
		maxVal, hasMax := getFloat("max")
		if hasMin && hasMax && minVal > maxVal {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("validation min (%.0f) is greater than max (%.0f)", minVal, maxVal),
			})
		}
	}

	if isNumeric {
		// Check gte vs lt (strict upper bound)
		gteVal, hasGte := getFloat("gte")
		ltVal, hasLt := getFloat("lt")
		if hasGte && hasLt && gteVal >= ltVal {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("validation gte (%.0f) must be less than lt (%.0f)", gteVal, ltVal),
			})
		}
		// Check gt vs lte (non-strict bounds)
		gtVal, hasGt := getFloat("gt")
		lteVal, hasLte := getFloat("lte")
		if hasGt && hasLte && gtVal >= lteVal {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("validation gt (%.0f) must be less than lte (%.0f)", gtVal, lteVal),
			})
		}
	}

	return errs
}

// missingFields returns a human-readable list of missing field names.
func missingFields(missingEmail, missingPassword bool) string {
	var parts []string
	if missingEmail {
		parts = append(parts, "email")
	}
	if missingPassword {
		parts = append(parts, "password")
	}
	return strings.Join(parts, " and ")
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
		} else if entity.Fields[f].IsRelation() && !isFKFieldName(f) {
			errs = append(errs, ValidationError{Entity: entityName, Field: f, Message: fmt.Sprintf("index %d references relation field '%s'; index the FK field instead (e.g. '%sId')", idxNum, f, f), Warning: true})
		}
	}

	// Index type.
	if idx.Type != "" && !specmeta.IsIndexType(idx.Type) {
		errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d has unknown type %q; allowed: %s", idxNum, idx.Type, strings.Join(specmeta.IndexTypes, ", "))})
	}

	// Sort array length must match fields length.
	if len(idx.Sort) > 0 && len(idx.Sort) != len(idx.Fields) {
		errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d: sort array length (%d) does not match fields length (%d)", idxNum, len(idx.Sort), len(idx.Fields))})
	}

	// Sort values must be asc or desc.
	for _, s := range idx.Sort {
		if !specmeta.IsSortDirection(s) {
			errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("index %d: invalid sort value %q; allowed: %s", idxNum, s, strings.Join(specmeta.SortDirections, ", "))})
		}
	}

	return errs
}

// --- permission-level ---

func (v *Validator) validatePermissions(entityName string, perms *parser.ParsedPermissions) []ValidationError {
	var errs []ValidationError

	validateRoles := func(operation string, roles []string) {
		for _, role := range roles {
			if role == "" {
				errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("permission '%s' has an empty role", operation)})
				continue
			}
			// Skip special tokens: * (public) and @... (ownership tokens)
			if specmeta.IsReservedPermissionToken(role) {
				continue
			}
			if !validIdentifier.MatchString(role) {
				errs = append(errs, ValidationError{Entity: entityName, Message: fmt.Sprintf("permission '%s' role %q is not a valid identifier", operation, role), Warning: true})
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

	// Check seed value types against field types.
	for seedField, seedValue := range entry {
		field := entity.Fields[seedField]
		if field == nil {
			continue
		}
		errs = append(errs, v.validateSeedValueType(entityName, seedField, entryIdx, field, seedValue)...)
	}

	// Check required relation FK dependencies.
	for _, fieldName := range entity.FieldOrder {
		field := entity.Fields[fieldName]
		if field == nil || !field.IsRelation() || field.IsOptional || field.IsMany {
			continue
		}
		if _, inSeed := entry[fieldName]; inSeed {
			continue
		}
		if field.DefaultValue != "" {
			continue
		}
		target := v.schema.Entities[field.TargetEntity]
		if target == nil || len(target.Seed) == 0 {
			errs = append(errs, ValidationError{
				Entity:  entityName,
				Field:   fieldName,
				Message: fmt.Sprintf("seed entry %d: required relation '%s' -> '%s' is not seeded (add FK value to seed or add seed data to %s)", entryIdx, fieldName, field.TargetEntity, field.TargetEntity),
			})
		}
	}

	return errs
}

// validateSeedValueType checks that a seed value is type-compatible with its field.
func (v *Validator) validateSeedValueType(entityName, fieldName string, entryIdx int, field *parser.ParsedField, seedValue interface{}) []ValidationError {
	var errs []ValidationError
	ftype := strings.ToLower(field.Type)

	switch ftype {
	case "string", "text":
		// String-like fields accept strings or JSON string values; numbers/bools are
		// accepted too (YAML coercion) but must not be a map/slice.
		if _, isMap := seedValue.(map[string]interface{}); isMap {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with type %s", entryIdx, seedValue, ftype),
			})
		} else if _, isSlice := seedValue.([]interface{}); isSlice {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with type %s", entryIdx, seedValue, ftype),
			})
		}
	case "int", "bigint":
		switch seedValue.(type) {
		case int, int32, int64, float32, float64:
			// numeric OK
		case string:
			// string representation — OK if it parses
		default:
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with type %s", entryIdx, seedValue, ftype),
			})
		}
	case "float", "decimal":
		switch seedValue.(type) {
		case int, int32, int64, float32, float64:
			// numeric OK
		case string:
			// string representation — OK
		default:
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with type %s", entryIdx, seedValue, ftype),
			})
		}
	case "boolean":
		switch seedValue.(type) {
		case bool:
			// OK
		case string:
			s := strings.ToLower(seedValue.(string))
			if s != "true" && s != "false" {
				errs = append(errs, ValidationError{
					Entity: entityName, Field: fieldName,
					Message: fmt.Sprintf("seed entry %d: value %q is not valid for boolean type", entryIdx, seedValue),
				})
			}
		default:
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with boolean type", entryIdx, seedValue),
			})
		}
	case "date":
		s, ok := seedValue.(string)
		if !ok {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with date type (expected string YYYY-MM-DD)", entryIdx, seedValue),
			})
		} else if !isValidDate(s) {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %q is not a valid date (expected YYYY-MM-DD)", entryIdx, s),
			})
		}
	case "uuid":
		s, ok := seedValue.(string)
		if !ok {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with uuid type (expected string)", entryIdx, seedValue),
			})
		} else if !isValidUUID(s) {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %q is not a valid UUID (expectedxxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)", entryIdx, s),
			})
		}
	case "datetime":
		s, ok := seedValue.(string)
		if !ok {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with datetime type (expected string)", entryIdx, seedValue),
			})
		} else if !isValidDatetime(s) {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %q is not a valid datetime (expected ISO 8601 format, e.g. 2024-01-15T10:30:00Z)", entryIdx, s),
			})
		}
	case "json", "jsonb":
		// JSON values can be any valid JSON — accept any type.
		// String values are validated as parseable JSON.
		if s, ok := seedValue.(string); ok {
			if !isValidJSON(s) {
				errs = append(errs, ValidationError{
					Entity: entityName, Field: fieldName,
					Message: fmt.Sprintf("seed entry %d: value %q is not valid JSON", entryIdx, s),
				})
			}
		}
	case "enum":
		// The value must be one of the declared enum values.
		sv, ok := seedValue.(string)
		if !ok {
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with enum type (expected string)", entryIdx, seedValue),
			})
			break
		}
		if values, defined := v.schema.Enums[field.TargetType]; defined {
			found := false
			for _, val := range values {
				if val == sv {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, ValidationError{
					Entity: entityName, Field: fieldName,
					Message: fmt.Sprintf("seed entry %d: value %q is not a declared value of enum '%s'; valid values: %s", entryIdx, sv, field.TargetType, strings.Join(values, ", ")),
				})
			}
		}
	case "array":
		// Accept a YAML list. String values are accepted as JSON arrays.
		switch v := seedValue.(type) {
		case []interface{}:
			// OK — list of elements
		case []string:
			// OK
		case string:
			if !isValidJSONArray(v) {
				errs = append(errs, ValidationError{
					Entity: entityName, Field: fieldName,
					Message: fmt.Sprintf("seed entry %d: value %q is not a valid JSON array for array type", entryIdx, v),
				})
			}
		default:
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with array type (expected a list)", entryIdx, seedValue),
			})
		}
	case "relation":
		// FK values reference the target entity's PK. Accept scalar values
		// (string/number/bool matching the target PK type); more precise
		// cross-entity type checking is left to the seed FK dependency check.
		switch seedValue.(type) {
		case string, int, int32, int64, float32, float64, bool:
			// OK
		default:
			errs = append(errs, ValidationError{
				Entity: entityName, Field: fieldName,
				Message: fmt.Sprintf("seed entry %d: value %v is not compatible with relation FK type", entryIdx, seedValue),
			})
		}
	}

	return errs
}

func isValidDate(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	return err == nil && t.Year() >= 1970 && t.Year() <= 2100
}

func isValidJSONArray(s string) bool {
	var arr []interface{}
	return json.Unmarshal([]byte(s), &arr) == nil
}

// isFKFieldName reports whether a field name is already an FK-style name
// (ends with "Id"/"id"), in which case indexing it maps to a real FK column.
func isFKFieldName(name string) bool {
	return strings.HasSuffix(name, "Id") || strings.HasSuffix(name, "id")
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isValidUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

var datetimePatterns = []string{
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05.000-07:00",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func isValidDatetime(s string) bool {
	for _, pattern := range datetimePatterns {
		if t, err := time.Parse(pattern, s); err == nil && t.Year() >= 1970 && t.Year() <= 2100 {
			return true
		}
	}
	return false
}

func isValidJSON(s string) bool {
	var v interface{}
	return json.Unmarshal([]byte(s), &v) == nil
}

// --- helpers ---

// countPrimaryKeys returns the number of primary key fields in the entity.
func countPrimaryKeys(entity *parser.ParsedEntity) int {
	count := 0
	for _, fieldName := range entity.FieldOrder {
		field := entity.Fields[fieldName]
		if field != nil && field.IsPrimary {
			count++
		}
	}
	return count
}
