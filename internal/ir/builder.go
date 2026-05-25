package ir

import (
	"fmt"
	"strings"

	"domaincraft/internal/parser"
	"domaincraft/internal/specmeta"
	"domaincraft/pkg/textutil"
)

// Builder converts ParsedSchema into IRProject.
type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Build(schema *parser.ParsedSchema) (*IRProject, error) {
	if schema == nil {
		return nil, fmt.Errorf("parsed schema is nil")
	}

	irProject := &IRProject{
		Name:     schema.Project.Name,
		Database: schema.Database,
		Auth:     schema.Auth,
		APIStyle: schema.APIStyle,
		Platform: schema.Project.Platform,
		Enums:    schema.Enums,
		Entities: make([]IREntity, 0, len(schema.Entities)),
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

	entityIndex := make(map[string]*IREntity, len(schema.Entities))
	for _, entityName := range schema.EntityOrder {
		sourceEntity := schema.Entities[entityName]
		if sourceEntity == nil {
			continue
		}

		irEntity := IREntity{
			Name:              sourceEntity.Name,
			NamePlural:        sourceEntity.NamePlural,
			HasAudit:          sourceEntity.Features["audit"],
			HasAuditLog:       sourceEntity.Features["audit_log"],
			HasSoftDelete:     sourceEntity.Features["soft_delete"],
			HasOptimisticLock: sourceEntity.Features["optimistic_lock"],
			Fields:            make([]IRField, 0, len(sourceEntity.FieldOrder)),
			RelationsOut:      make([]IRRelation, 0),
			RelationsIn:       make([]IRRelation, 0),
			Indexes:           make([]IRIndex, 0, len(sourceEntity.Indexes)),
			Seed:              sourceEntity.Seed,
			Permissions:       convertPermissions(sourceEntity.Permissions),
		}

		for _, fieldName := range sourceEntity.FieldOrder {
			field := sourceEntity.Fields[fieldName]
			if field == nil {
				continue
			}

			irEntity.Fields = append(irEntity.Fields, IRField{
				Name:           field.Name,
				DatabaseType:   resolveDatabaseType(schema.Database, field, schema),
				NavigationName: navigationName(field),
				IsPrimary:      field.IsPrimary,
				IsNullable:     field.IsOptional,
				IsUnique:       field.IsUnique,
				IsHidden:       field.IsHidden,
				IsRelation:     field.IsRelation,
				IsMany:         field.IsMany,
				RelationTarget: field.RelationTarget,
				DefaultValue:   field.DefaultValue,
				DefaultIsFunc:  field.DefaultIsFunc,
				Validations:    convertValidations(field.Validations),
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

	for i := range irProject.Entities {
		irEntity := &irProject.Entities[i]
		sourceEntity := schema.Entities[irEntity.Name]
		if sourceEntity == nil {
			continue
		}

		for _, fieldName := range sourceEntity.FieldOrder {
			field := sourceEntity.Fields[fieldName]
			if field == nil || !field.IsRelation {
				continue
			}

			targetEntity, ok := entityIndex[field.RelationTarget]
			if !ok {
				return nil, fmt.Errorf("relation target '%s' referenced by '%s.%s' does not exist", field.RelationTarget, irEntity.Name, field.Name)
			}

			relationType := field.RelationType
			if relationType == "" {
				relationType = "many-to-one"
				if field.IsUnique {
					relationType = "one-to-one"
				}
				if field.IsMany {
					relationType = "many-to-many"
				}
			}

			relation := IRRelation{
				FieldName:        field.Name,
				TargetEntity:     targetEntity,
				NavigationName:   navigationName(field),
				InverseNavName:   textutil.Pluralize(irEntity.Name),
				OnDeleteBehavior: normalizeDeleteBehavior(field.OnDelete),
				IsNullable:       field.IsOptional,
				IsMany:           field.IsMany,
				RelationType:     relationType,
			}

			irEntity.RelationsOut = append(irEntity.RelationsOut, relation)

			// Skip adding inverse RelationsIn if the target already has a forward IsMany
			// relation to this entity (avoids duplicate inverse collection navigations)
			hasForwardMany := false
			for _, out := range targetEntity.RelationsOut {
				if out.IsMany && out.TargetEntity != nil && out.TargetEntity.Name == irEntity.Name {
					hasForwardMany = true
					break
				}
			}
			if !hasForwardMany {
				targetEntity.RelationsIn = append(targetEntity.RelationsIn, IRRelation{
					FieldName:        relation.FieldName,
					TargetEntity:     irEntity,
					NavigationName:   relation.NavigationName,
					InverseNavName:   relation.InverseNavName,
					OnDeleteBehavior: relation.OnDeleteBehavior,
					IsMany:           !relation.IsMany, // Inverse cardinality: one-to-many becomes many-to-one on the target
					RelationType:     relation.RelationType,
				})
			}
		}
	}

	// Resolve InverseNavName to actual forward navigation name on target entity.
	// For OrderItem -> Order: InverseNavName should be "Items" (Order's forward nav),
	// not the computed "OrderItems".
	for i := range irProject.Entities {
		entity := &irProject.Entities[i]
		for j := range entity.RelationsOut {
			rel := &entity.RelationsOut[j]
			if rel.TargetEntity == nil || rel.IsMany {
				continue
			}
			// Find matching forward IsMany relation on target entity
			for _, targetRel := range rel.TargetEntity.RelationsOut {
				if targetRel.IsMany && targetRel.TargetEntity != nil && targetRel.TargetEntity.Name == entity.Name {
					// Use the field name (already plural in YAML) for collection navigations
					rel.InverseNavName = textutil.PascalCase(targetRel.FieldName)
					break
				}
			}
		}
	}

	return irProject, nil
}

func convertValidations(source map[string]string) []IRValidation {
	if len(source) == 0 {
		return nil
	}
	result := make([]IRValidation, 0, len(source))
	for key, value := range source {
		result = append(result, IRValidation{Name: key, Value: value})
	}
	return result
}

func convertPermissions(source *parser.ParsedPermissions) *IRPermissions {
	if source == nil {
		return nil
	}
	return &IRPermissions{
		Read:       append([]string(nil), source.Read...),
		Create:     append([]string(nil), source.Create...),
		Update:     append([]string(nil), source.Update...),
		Delete:     append([]string(nil), source.Delete...),
		ReadPublic: source.ReadPublic,
	}
}

func resolveDatabaseType(_ string, field *parser.ParsedField, schema *parser.ParsedSchema) string {
	if field == nil || field.FieldDefinition == nil {
		return "string"
	}

	if field.IsRelation {
		if target, ok := schema.Entities[field.RelationTarget]; ok {
			for _, targetFieldName := range target.FieldOrder {
				targetField := target.Fields[targetFieldName]
				if targetField != nil && targetField.IsPrimary {
					return resolveDatabaseType("", targetField, schema)
				}
			}
		}
		return "string"
	}

	if field.Type == "array" {
		return resolveArrayType("", field.TargetType)
	}

	if field.Type == "enum" {
		// Store the raw enum name as defined in YAML — templates decide how to render it
		// (e.g. PascalCase for C#/Java, snake_case for Python, etc.)
		if field.TargetType != "" {
			return field.TargetType
		}
		return "string"
	}

	if specmeta.IsPrimitive(field.Type) {
		return field.Type
	}
	return "string"
}

func resolveArrayType(_ string, targetType string) string {
	inner := strings.ToLower(targetType)
	if specmeta.IsPrimitive(inner) {
		return "array(" + inner + ")"
	}
	// Enum type — store raw name
	if targetType != "" {
		return "array(" + targetType + ")"
	}
	return "array(string)"
}

func navigationName(field *parser.ParsedField) string {
	if field == nil {
		return ""
	}

	name := field.Name
	if field.IsMany {
		name = textutil.Singularize(name)
	}
	if strings.HasSuffix(strings.ToLower(name), "id") && len(name) > 2 {
		name = name[:len(name)-2]
	}
	if name == "" {
		name = field.RelationTarget
	}
	return textutil.PascalCase(name)
}

func normalizeDeleteBehavior(value string) string {
	switch strings.ToLower(value) {
	case "cascade":
		return "CASCADE"
	case "set_null":
		return "SET NULL"
	case "restrict":
		return "RESTRICT"
	case "no_action":
		return "NO ACTION"
	default:
		return strings.ToUpper(strings.NewReplacer("_", " ", "-", " ").Replace(value))
	}
}
