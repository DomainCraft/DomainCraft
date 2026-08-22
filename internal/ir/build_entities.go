package ir

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// oldDatabaseColumnName returns the snake_case DB column name of a field's
// old_name hint (previous field name), or "" when the field has no rename hint.
func oldDatabaseColumnName(field *parser.ParsedField) string {
	if field == nil || field.OldName == "" {
		return ""
	}
	return textutil.ToDatabaseColumnName(field.OldName)
}

// Build converts ParsedSchema into IRProject.
func Build(schema *parser.ParsedSchema) (*IRProject, error) {
	if schema == nil {
		return nil, fmt.Errorf("parsed schema is nil")
	}

	irProject := &IRProject{
		Name:        schema.Project.Name,
		Description: schema.Project.Description,
		Database:    schema.Database,
		Auth:        convertAuth(schema.Auth, schema),
		APIStyle:    schema.APIStyle,
		Platform:    schema.Project.Platform,
		Enums:       copyEnumsSorted(schema.Enums),
		EnumOrder:   sortedEnumKeys(schema.Enums),
		Entities:    make([]IREntity, 0, len(schema.Entities)),
	}

	if schema.Project.Cache != nil {
		irProject.Cache = &IRCacheConfig{
			Enabled:          schema.Project.Cache.Enabled,
			Provider:         schema.Project.Cache.Provider,
			ConnectionString: schema.Project.Cache.ConnectionString,
			TTLSeconds:       schema.Project.Cache.TTLSeconds,
		}
	}
	if schema.Project.CORS != nil {
		irProject.CORS = &IRCORSConfig{
			Enabled: schema.Project.CORS.Enabled,
			Origins: append([]string(nil), schema.Project.CORS.Origins...),
		}
	}
	if schema.Project.Deploy != nil {
		irProject.Deploy = &IRDeployConfig{
			Domain: schema.Project.Deploy.Domain,
			Port:   schema.Project.Deploy.Port,
		}
	}
	if schema.Project.Versioning != nil {
		irProject.Versioning = &IRVersioningConfig{
			Enabled:        schema.Project.Versioning.Enabled,
			DefaultVersion: schema.Project.Versioning.DefaultVersion,
		}
	}
	if schema.Project.RateLimit != nil {
		irProject.RateLimit = &IRRateLimitConfig{
			Enabled:       schema.Project.RateLimit.Enabled,
			Policy:        schema.Project.RateLimit.Policy,
			PermitLimit:   schema.Project.RateLimit.PermitLimit,
			WindowSeconds: schema.Project.RateLimit.WindowSeconds,
		}
	}
	if schema.Project.Pagination != nil {
		irProject.Pagination = &IRPaginationConfig{
			DefaultPageSize: schema.Project.Pagination.DefaultPageSize,
			MaxPageSize:     schema.Project.Pagination.MaxPageSize,
		}
	}
	if schema.Project.Infrastructure != nil {
		irProject.Infrastructure = &IRInfrastructure{
			Queue:   schema.Project.Infrastructure.Queue,
			Cache:   schema.Project.Infrastructure.Cache,
			Secrets: schema.Project.Infrastructure.Secrets,
			Storage: schema.Project.Infrastructure.Storage,
		}
	}

	// Pass 1: build entities
	entityIndex := make(map[string]*IREntity, len(schema.Entities))
	for _, entityName := range schema.EntityOrder {
		sourceEntity := schema.Entities[entityName]
		if sourceEntity == nil {
			continue
		}

		irEntity := IREntity{
			Name:         sourceEntity.Name,
			OldName:      sourceEntity.OldName,
			NamePlural:   sourceEntity.NamePlural,
			Features:     copyFeatureMap(sourceEntity.Features),
			Fields:       make([]IRField, 0, len(sourceEntity.FieldOrder)),
			RelationsOut: make([]IRRelation, 0),
			RelationsIn:  make([]IRRelation, 0),
			Indexes:      make([]IRIndex, 0, len(sourceEntity.Indexes)),
			Seed:         sourceEntity.Seed,
			Permissions:  convertPermissions(sourceEntity.Permissions),
		}

		for _, fieldName := range sourceEntity.FieldOrder {
			field := sourceEntity.Fields[fieldName]
			if field == nil {
				continue
			}

			databaseType, err := resolveDatabaseType(field, schema)
			if err != nil {
				return nil, fmt.Errorf("field '%s.%s': %w", sourceEntity.Name, field.Name, err)
			}

			irEntity.Fields = append(irEntity.Fields, IRField{
				Name:                  field.Name,
				OldName:               field.OldName,
				DatabaseType:          databaseType,
				DatabaseColumnName:    field.DatabaseColumnName,
				OldDatabaseColumnName: oldDatabaseColumnName(field),
				NavigationName:        field.NavigationName(),
				IsPrimary:             field.IsPrimary,
				IsNullable:            field.IsOptional,
				IsUnique:              field.IsUnique,
				IsHidden:              field.IsHidden,
				IsReadonly:            field.IsReadonly,
				IsRelation:            field.IsRelation(),
				IsMany:                field.IsMany,
				RelationTarget:        field.TargetEntity,
				DefaultValue:          field.DefaultValue,
				DefaultIsFunc:         field.DefaultIsFunc,
				Validations:           convertValidations(field.Validations),
			})
		}

		for _, idx := range sourceEntity.Indexes {
			irEntity.Indexes = append(irEntity.Indexes, IRIndex{
				Name:   idx.Name,
				Fields: append([]string(nil), idx.Fields...),
				Type:   idx.Type,
				Sort:   append([]string(nil), idx.Sort...),
				Unique: idx.Unique,
			})
		}

		irProject.Entities = append(irProject.Entities, irEntity)
		entityIndex[irEntity.Name] = &irProject.Entities[len(irProject.Entities)-1]
	}

	// Pass 2a: forward relations
	if err := buildForwardRelations(irProject.Entities, schema, entityIndex); err != nil {
		return nil, err
	}

	// Pass 2b: reconcile one-to-many
	reconcileOneToMany(irProject.Entities)

	// Pass 2c: resolve inverse navigation names
	resolveInverseNavs(irProject.Entities)

	// Pass 3d: incoming relations
	buildIncomingRelations(irProject.Entities)

	// Topologically sort entities by FK dependencies (parents before children).
	irProject.Entities = sortEntities(irProject.Entities)

	return irProject, nil
}

func convertValidations(source map[string]string) []IRValidation {
	if len(source) == 0 {
		return nil
	}
	result := make([]IRValidation, 0, len(source))
	for _, key := range slices.Sorted(maps.Keys(source)) {
		result = append(result, IRValidation{Name: key, Value: source[key]})
	}
	return result
}

func convertPermissions(source *parser.ParsedPermissions) *IRPermissions {
	if source == nil {
		return nil
	}
	return &IRPermissions{
		Read:   append([]string(nil), source.Read...),
		Create: append([]string(nil), source.Create...),
		Update: append([]string(nil), source.Update...),
		Delete: append([]string(nil), source.Delete...),
	}
}

func resolveDatabaseType(field *parser.ParsedField, schema *parser.ParsedSchema) (string, error) {
	if field == nil || field.FieldDefinition == nil {
		return "", fmt.Errorf("field definition is missing")
	}

	if field.IsRelation() {
		target, ok := schema.Entities[field.TargetEntity]
		if !ok {
			return "", fmt.Errorf("relation target '%s' does not exist", field.TargetEntity)
		}
		for _, targetFieldName := range target.FieldOrder {
			targetField := target.Fields[targetFieldName]
			if targetField != nil && targetField.IsPrimary {
				return resolveDatabaseType(targetField, schema)
			}
		}
		return "", fmt.Errorf("relation target '%s' has no primary key", field.TargetEntity)
	}

	if field.Type == "array" {
		return resolveArrayType(field.TargetType), nil
	}

	if field.Type == "enum" {
		if field.TargetType != "" {
			return field.TargetType, nil
		}
		return specmeta.DefaultFieldType, nil
	}

	if specmeta.IsPrimitive(field.Type) {
		return field.Type, nil
	}
	return specmeta.DefaultFieldType, nil
}

func resolveArrayType(targetType string) string {
	inner := strings.ToLower(targetType)
	if specmeta.IsPrimitive(inner) {
		return "array(" + inner + ")"
	}
	if targetType != "" {
		return "array(" + targetType + ")"
	}
	return "array(string)"
}

func convertAuth(source *parser.AuthConfig, schema *parser.ParsedSchema) *IRAuthConfig {
	if source == nil || source.Type == "" || source.Type == "none" {
		return nil
	}

	entity := source.Entity
	if entity == "" {
		entity = specmeta.FindAuthEntity(schema.EntityOrder, specmeta.BuildEntityFields(schema.EntityOrder, func(name string) []string {
			if e := schema.Entities[name]; e != nil {
				return e.FieldOrder
			}
			return nil
		}))
	}

	login := source.Endpoints.HasLogin()
	register := source.Endpoints.HasRegister()
	me := source.Endpoints.HasMe()
	setup := source.Endpoints.HasSetup()

	return &IRAuthConfig{
		Type:   source.Type,
		Entity: entity,
		Roles:  append([]string(nil), source.Roles...),
		Endpoints: IRAuthEndpoints{
			HasLogin:    login,
			HasRegister: register,
			HasMe:       me,
			HasSetup:    setup,
		},
	}
}

func copyFeatureMap(source map[string]bool) map[string]bool {
	return maps.Clone(source)
}

// copyEnumsSorted returns a deep copy of the enums map with sorted keys for deterministic iteration.
func copyEnumsSorted(source map[string][]string) map[string][]string {
	if source == nil {
		return nil
	}
	result := make(map[string][]string, len(source))
	for k, v := range source {
		cp := make([]string, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}

// sortedEnumKeys returns a sorted slice of enum names for deterministic iteration.
func sortedEnumKeys(source map[string][]string) []string {
	return slices.Sorted(maps.Keys(source))
}