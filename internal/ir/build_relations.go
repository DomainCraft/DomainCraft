package ir

import (
	"fmt"

	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// buildForwardRelations (Pass 2a) builds forward (outgoing) relations for every
// entity. It must run as a separate pass from the inverse pass below so the
// inverse logic always sees the complete set of RelationsOut regardless of
// entity declaration order.
func buildForwardRelations(entities []IREntity, schema *parser.ParsedSchema, entityIndex map[string]*IREntity) error {
	for i := range entities {
		irEntity := &entities[i]
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
				return fmt.Errorf("relation target '%s' referenced by '%s.%s' does not exist", field.TargetEntity, irEntity.Name, field.Name)
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
	return nil
}

// reconcileOneToMany (Pass 2b) detects one-to-many relations declared on BOTH
// sides. When entity A has a `[many]` relation to B AND B declares a single FK
// relation back to A, the two declarations describe the SAME one-to-many
// relationship — not a many-to-many. The FK lives on B; A's [many] is its
// collection navigation. Without this reconciliation a bridge would generate a
// spurious join table in addition to the FK.
func reconcileOneToMany(entities []IREntity) {
	for i := range entities {
		entity := &entities[i]
		for j := range entity.RelationsOut {
			rel := &entity.RelationsOut[j]
			if !rel.IsMany || rel.TargetEntity == nil {
				continue
			}
			pair := findSingleBack(rel.TargetEntity, entity.Name)
			if pair == nil {
				continue
			}
			rel.RelationType = "one-to-many"
			rel.PairFieldName = pair.FieldName
			rel.PairNavigationName = pair.NavigationName
			rel.OnDeleteBehavior = pair.OnDeleteBehavior
			rel.IsNullable = pair.IsNullable
		}
	}
}

// resolveInverseNavs (Pass 2c) resolves each forward relation's inverse-navigation
// name to the actual collection on the target entity. For OrderItem→Order the
// inverse should be "Items" (Order's forward nav), not the computed "OrderItems".
func resolveInverseNavs(entities []IREntity) {
	for i := range entities {
		entity := &entities[i]
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
}

// buildIncomingRelations (Pass 3d) builds inverse (incoming) relations. Each
// incoming relation is a collection navigation on the target that points back
// to the owning entity.
func buildIncomingRelations(entities []IREntity) {
	usedInverse := make(map[string]map[string]bool) // target entity -> defined inverse nav names
	for i := range entities {
		entity := &entities[i]
		for j := range entity.RelationsOut {
			rel := &entity.RelationsOut[j]
			target := rel.TargetEntity
			if target == nil {
				continue
			}
			// A double-declared one-to-many: the collection already exists as the
			// forward [many] field — no separate inverse collection on target.
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
				// Two relations from the same source to the same target — disambiguate
				// with the field name (e.g. EscrowContract.Buyer / EscrowContract.Seller).
				invName = textutil.PascalCase(rel.FieldName) + textutil.Pluralize(entity.Name)
				for usedInverse[target.Name] != nil && usedInverse[target.Name][invName] {
					invName += "Side"
				}
			}
			if usedInverse[target.Name] == nil {
				usedInverse[target.Name] = make(map[string]bool)
			}
			usedInverse[target.Name][invName] = true
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