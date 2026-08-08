package ir

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// oldDatabaseColumnName returns the snake_case DB column name of a field's
// old_name hint (previous field name), or "" when the field has no rename hint.
// Bridges use it to emit a safe RenameColumn(old, new) migration.
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
				IsPrimary:          field.IsPrimary,
				IsNullable:         field.IsOptional,
				IsUnique:           field.IsUnique,
				IsHidden:           field.IsHidden,
				IsReadonly:         field.IsReadonly,
				IsRelation:         field.IsRelation(),
				IsMany:             field.IsMany,
				RelationTarget:     field.TargetEntity,
				DefaultValue:       field.DefaultValue,
				DefaultIsFunc:      field.DefaultIsFunc,
				Validations:        convertValidations(field.Validations),
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

	// Pass 2a: build forward (outgoing) relations for every entity. This is a
	// separate pass from the inverse pass below so the inverse logic always sees
	// the complete set of RelationsOut regardless of entity declaration order.
	for i := range irProject.Entities {
		irEntity := &irProject.Entities[i]
		sourceEntity := schema.Entities[irEntity.Name]
		if sourceEntity == nil {
			continue
		}

		for _, fieldName := range sourceEntity.FieldOrder {
			field := sourceEntity.Fields[fieldName]
			if field == nil || !field.IsRelation() {
				continue
			}

			targetEntity, ok := entityIndex[field.TargetEntity]
			if !ok {
				return nil, fmt.Errorf("relation target '%s' referenced by '%s.%s' does not exist", field.TargetEntity, irEntity.Name, field.Name)
			}

			irEntity.RelationsOut = append(irEntity.RelationsOut, IRRelation{
				FieldName:        field.Name,
				TargetEntity:     targetEntity,
				NavigationName:   field.NavigationName(),
				InverseNavName:   textutil.Pluralize(irEntity.Name),
				OnDeleteBehavior: field.OnDelete,
				IsNullable:       field.IsOptional,
				IsMany:           field.IsMany,
				RelationType:     field.RelationType,
			})
		}
	}

	// Pass 2b: reconcile one-to-many relations that are declared on BOTH sides.
	// When entity A has a `[many]` relation to B AND B declares a single (non-many)
	// FK relation back to A, the two declarations describe the SAME one-to-many
	// relationship, not a many-to-many. Without this reconciliation the C# bridge
	// would generate a spurious EF Core join table in addition to the FK.
	for i := range irProject.Entities {
		entity := &irProject.Entities[i]
		for j := range entity.RelationsOut {
			rel := &entity.RelationsOut[j]
			if !rel.IsMany || rel.TargetEntity == nil {
				continue
			}
			pair := findSingleBack(rel.TargetEntity, entity.Name)
			if pair == nil {
				continue
			}
			// This [many] field is really the collection side of a one-to-many;
			// the FK lives on the target (`pair`). Remember which field that is,
			// and borrow the FK's required/delete-behavior so the owning side can
			// render WithOne(...).HasForeignKey(...) consistently.
			rel.RelationType = "one-to-many"
			rel.PairFieldName = pair.FieldName
			rel.PairNavigationName = pair.NavigationName
			rel.OnDeleteBehavior = pair.OnDeleteBehavior
			rel.IsNullable = pair.IsNullable
		}
	}

	// Pass 2c: resolve each forward relation's inverse-navigation name to the
	// actual collection on the target entity. For OrderItem -> Order the inverse
	// should be "Items" (Order's forward nav), not the computed "OrderItems".
	for i := range irProject.Entities {
		entity := &irProject.Entities[i]
		for j := range entity.RelationsOut {
			rel := &entity.RelationsOut[j]
			if rel.TargetEntity == nil || rel.IsMany {
				continue
			}
			for _, targetRel := range rel.TargetEntity.RelationsOut {
				if targetRel.IsMany && targetRel.TargetEntity != nil && targetRel.TargetEntity.Name == entity.Name {
					rel.InverseNavName = textutil.PascalCase(targetRel.FieldName)
					break
				}
			}
		}
	}

	// Pass 3d: build inverse (incoming) relations. Each incoming relation is a
	// collection navigation on the target that points back to the owning entity.
	usedInverse := make(map[string]map[string]bool) // target entity -> defined inverse nav names
	for i := range irProject.Entities {
		entity := &irProject.Entities[i]
		for j := range entity.RelationsOut {
			rel := &entity.RelationsOut[j]
			target := rel.TargetEntity
			if target == nil {
				continue
			}
			// Reconcile a double-declared one-to-many: the collection already
			// exists as the forward `[many]` field on this entity, driven by the
			// FK declared on the target — no separate inverse collection on target.
			if rel.IsMany && rel.RelationType == "one-to-many" {
				continue
			}
			// If the target already declares its own forward IsMany relation back
			// to this entity, the inverse collection lives there instead.
			if hasForwardManyTo(target, entity.Name) {
				continue
			}
			invName := rel.InverseNavName
			if usedInverse[target.Name] != nil && usedInverse[target.Name][invName] {
				// Two relations from the same source to the same target (e.g.
				// EscrowContract.Buyer / EscrowContract.Seller) would collide on
				// the pluralized inverse name. Disambiguate with the field name.
				invName = textutil.PascalCase(rel.FieldName) + textutil.Pluralize(entity.Name)
				for usedInverse[target.Name] != nil && usedInverse[target.Name][invName] {
					invName += "Side"
				}
			}
			if usedInverse[target.Name] == nil {
				usedInverse[target.Name] = make(map[string]bool)
			}
			usedInverse[target.Name][invName] = true
			// Keep the owning relation in sync so EF mapping (.WithMany) and the
			// inverse collection property use the same name.
			rel.InverseNavName = invName
			target.RelationsIn = append(target.RelationsIn, IRRelation{
				FieldName:        rel.FieldName,
				TargetEntity:     entity,
				InverseNavName:   invName,
				OnDeleteBehavior: rel.OnDeleteBehavior,
				IsNullable:       rel.IsNullable,
				IsMany:           !rel.IsMany,
				RelationType:     rel.RelationType,
			})
		}
	}

	// Topologically sort entities by FK dependencies (parents before children),
	// so the seeder inserts rows in the correct order.
	irProject.Entities = sortEntities(irProject.Entities)

	return irProject, nil
}

// findSingleBack returns the target entity's forward single (non-many) FK
// relation that points back to sourceName, or nil if none exists.
func findSingleBack(target *IREntity, sourceName string) *IRRelation {
	if target == nil {
		return nil
	}
	for i := range target.RelationsOut {
		r := &target.RelationsOut[i]
		if !r.IsMany && r.TargetEntity != nil && r.TargetEntity.Name == sourceName {
			return r
		}
	}
	return nil
}

// hasForwardManyTo reports whether target declares a forward IsMany relation
// that points back to entityName. When true, the inverse collection for that
// relationship already lives on target as its own field.
func hasForwardManyTo(target *IREntity, entityName string) bool {
	if target == nil {
		return false
	}
	for i := range target.RelationsOut {
		r := &target.RelationsOut[i]
		if r.IsMany && r.TargetEntity != nil && r.TargetEntity.Name == entityName {
			return true
		}
	}
	return false
}

// sortEntities reorders the slice by FK dependencies using Kahn's algorithm.
// The order is deterministic: ties are broken by entity name.
// It returns a new slice because relations hold *IREntity pointers into the
// original backing array; the original must stay intact until every pointer has
// been rebound to its new location.
func sortEntities(entities []IREntity) []IREntity {
	byName := make(map[string]int, len(entities))
	for i, e := range entities {
		byName[e.Name] = i
	}

	// deps[i] = set of entity names that entities[i] depends on (must come first).
	deps := make([]map[string]bool, len(entities))
	inDegree := make([]int, len(entities))
	for i, entity := range entities {
		depSet := make(map[string]bool)
		for _, rel := range entity.RelationsOut {
			if !rel.IsMany && rel.TargetEntity != nil && rel.TargetEntity.Name != entity.Name {
				depSet[rel.TargetEntity.Name] = true
			}
		}
		deps[i] = depSet
		inDegree[i] = len(depSet)
	}

	// Initialize the queue with all entities that have no dependencies.
	ready := make([]string, 0)
	for i, e := range entities {
		if inDegree[i] == 0 {
			ready = append(ready, e.Name)
		}
	}
	// Deterministic ordering: process ready entities in name order.
	sort.Strings(ready)

	order := make([]string, 0, len(entities))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		order = append(order, name)

		// For every entity depending on this one, decrement in-degree.
		for i := range entities {
			if deps[i][name] {
				inDegree[i]--
				if inDegree[i] == 0 {
					// Insert in sorted order to keep processing deterministic.
					ready = append(ready, entities[i].Name)
					sort.Strings(ready)
				}
			}
		}
	}

	// Entities that form a cycle are appended in name order (after all others).
	if len(order) < len(entities) {
		inOrder := make(map[string]bool, len(order))
		for _, name := range order {
			inOrder[name] = true
		}
		remaining := make([]string, 0, len(entities)-len(order))
		for _, e := range entities {
			if !inOrder[e.Name] {
				remaining = append(remaining, e.Name)
			}
		}
		sort.Strings(remaining)
		order = append(order, remaining...)
	}

	// Build the reordered slice. The source `entities` backing array is left
	// untouched, so every TargetEntity pointer still points to valid memory we
	// can read the name from while rebinding it to the new slice.
	sorted := make([]IREntity, 0, len(entities))
	for _, name := range order {
		sorted = append(sorted, entities[byName[name]])
	}

	relocated := make(map[string]*IREntity, len(sorted))
	for i := range sorted {
		relocated[sorted[i].Name] = &sorted[i]
	}
	for i := range sorted {
		for j := range sorted[i].RelationsOut {
			if sorted[i].RelationsOut[j].TargetEntity != nil {
				sorted[i].RelationsOut[j].TargetEntity = relocated[sorted[i].RelationsOut[j].TargetEntity.Name]
			}
		}
		for j := range sorted[i].RelationsIn {
			if sorted[i].RelationsIn[j].TargetEntity != nil {
				sorted[i].RelationsIn[j].TargetEntity = relocated[sorted[i].RelationsIn[j].TargetEntity.Name]
			}
		}
	}

	return sorted
}

func convertValidations(source map[string]string) []IRValidation {
	if len(source) == 0 {
		return nil
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]IRValidation, 0, len(source))
	for _, key := range keys {
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
		// Store the raw enum name as defined in YAML — templates decide how to render it
		// (e.g. PascalCase for C#/Java, snake_case for Python, etc.)
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
	// Enum type — store raw name
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
	if source == nil {
		return nil
	}
	result := make(map[string]bool, len(source))
	for k, v := range source {
		result[k] = v
	}
	return result
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
	keys := make([]string, 0, len(source))
	for k := range source {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
