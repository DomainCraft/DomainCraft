package snapshot

// ComputeMigrationPlan compares the previous snapshot (old model) against the
// current IR (new model) and returns an ordered, deterministic migration tree:
// CreateTable / DropTable / RenameTable / AddColumn / DropColumn / RenameColumn /
// AlterColumn. The plan is language-agnostic — bridges translate each op into
// their own migration syntax (EF Core, Alembic, a NoSQL transform, ...).
//
// Unlike ComputeDiff (which is about orphaned files and developer-owned custom
// code), this is about the database schema. Renames use the `old_name` hints;
// a RenameTable/RenameColumn preserves data, whereas a Drop+Add would not.

import (
	"maps"
	"slices"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// tableName returns the snake_case plural table name for an entity name, the
// same convention the IR entity's NamePlural follows.
func tableName(entityName string) string {
	return textutil.ToDatabaseColumnName(textutil.Pluralize(entityName))
}

// ComputeMigrationPlan builds the abstract migration tree.
func ComputeMigrationPlan(old *Snapshot, project *ir.IRProject) *ir.MigrationPlan {
	plan := &ir.MigrationPlan{}
	if old == nil || project == nil {
		return plan
	}

	oldEntities := old.Entities
	newByOldName := make(map[string]*ir.IREntity, len(project.Entities)) // old_name -> new entity
	newByName := make(map[string]*ir.IREntity, len(project.Entities))    // name -> new entity
	for i := range project.Entities {
		e := &project.Entities[i]
		newByName[e.Name] = e
		if e.OldName != "" {
			newByOldName[e.OldName] = e
		}
	}

	// --- table-level operations -------------------------------------------

	// CreateTable: entity is new (not in the old model and not a rename).
	newEntityNames := make([]string, 0, len(project.Entities))
	for _, e := range project.Entities {
		newEntityNames = append(newEntityNames, e.Name)
	}
	slices.Sort(newEntityNames)
	for _, name := range newEntityNames {
		e := newByName[name]
		if _, existed := oldEntities[name]; existed {
			continue
		}
		if e.OldName != "" {
			if _, wasRenamed := oldEntities[e.OldName]; wasRenamed {
				continue // handled as a rename below
			}
		}
		op := ir.MigrationOp{Kind: ir.OpCreateTable, Entity: e.Name, Table: tableName(e.Name)}
		for _, f := range e.Fields {
			column := f.ColumnName()
			op.Columns = append(op.Columns, ir.MigrationColumn{
				Name:           f.Name,
				Column:         column,
				DBType:         f.DatabaseType,
				Nullable:       f.IsNullable,
				IsPrimary:      f.IsPrimary,
				IsRelation:     f.IsRelation,
				RelationTarget: f.RelationTarget,
			})
		}
		plan.Operations = append(plan.Operations, op)
	}

	// RenameTable: entity declares old_name pointing at a previously existing entity.
	renamedOldNames := make([]string, 0)
	for oldName := range newByOldName {
		if _, existed := oldEntities[oldName]; existed {
			renamedOldNames = append(renamedOldNames, oldName)
		}
	}
	slices.Sort(renamedOldNames)
	for _, oldName := range renamedOldNames {
		e := newByOldName[oldName]
		plan.Operations = append(plan.Operations, ir.MigrationOp{
			Kind:     ir.OpRenameTable,
			Entity:   e.Name,
			OldName:  oldName,
			Table:    tableName(e.Name),
			OldTable: tableName(oldName),
		})
	}

	// DropTable: entity existed before but is gone now (and not renamed).
	oldEntityNames := slices.Sorted(maps.Keys(oldEntities))
	for _, name := range oldEntityNames {
		if _, stillExists := newByName[name]; stillExists {
			continue
		}
		if _, renamed := newByOldName[name]; renamed {
			continue
		}
		plan.Operations = append(plan.Operations, ir.MigrationOp{
			Kind:   ir.OpDropTable,
			Entity: name,
			Table:  tableName(name),
		})
	}

	// --- column-level operations -----------------------------------------
	// For entities that survive (same name, or renamed), diff their fields.
	for _, name := range newEntityNames {
		e := newByName[name]
		// The old state this entity corresponds to (rename resolution).
		oldName := name
		if e.OldName != "" {
			if _, ok := oldEntities[e.OldName]; ok {
				oldName = e.OldName
			}
		}
		oldState, ok := oldEntities[oldName]
		if !ok {
			continue // brand-new entity already handled by CreateTable
		}
		diffColumns(plan, oldState, e)
	}

	return plan
}

// diffColumns appends AddColumn / DropColumn / RenameColumn / AlterColumn ops
// for one surviving entity.
func diffColumns(plan *ir.MigrationPlan, oldState EntityState, e *ir.IREntity) {
	oldFields := oldState.Fields // field name -> IR database type
	newFieldByName := make(map[string]*ir.IRField, len(e.Fields))
	for i := range e.Fields {
		f := &e.Fields[i]
		newFieldByName[f.Name] = f
	}

	table := tableName(e.Name)

	// RenameColumn: new field declares old_name pointing at a field that existed.
	renamedFields := make([]string, 0)
	for _, f := range e.Fields {
		if f.OldName == "" {
			continue
		}
		if _, existed := oldFields[f.OldName]; existed {
			// Ambiguous: the new name also existed before — not a clean rename.
			if _, shadowed := oldFields[f.Name]; shadowed {
				continue
			}
			renamedFields = append(renamedFields, f.Name)
		}
	}
	slices.Sort(renamedFields)
	for _, fieldName := range renamedFields {
		f := newFieldByName[fieldName]
		oldColumn := f.OldDatabaseColumnName
		if oldColumn == "" {
			oldColumn = textutil.ToDatabaseColumnName(f.OldName)
		}
		newColumn := f.ColumnName()
		plan.Operations = append(plan.Operations, ir.MigrationOp{
			Kind:      ir.OpRenameColumn,
			Entity:    e.Name,
			Table:     table,
			Column:    newColumn,
			OldColumn: oldColumn,
			DBType:    f.DatabaseType,
			OldDBType: oldFields[f.OldName],
		})
		// A rename that also changed type is two logical operations for most
		// migration systems (RenameColumn then AlterColumn). Emit both so a bridge
		// never has to re-derive the type change.
		if oldType := oldFields[f.OldName]; oldType != f.DatabaseType {
			plan.Operations = append(plan.Operations, ir.MigrationOp{
				Kind:      ir.OpAlterColumn,
				Entity:    e.Name,
				Table:     table,
				Column:    newColumn,
				DBType:    f.DatabaseType,
				OldDBType: oldType,
			})
		}
	}

	// AddColumn: field present now, absent before, and not a rename.
	for _, f := range e.Fields {
		if _, existed := oldFields[f.Name]; existed {
			continue
		}
		if f.OldName != "" {
			if _, wasRenamed := oldFields[f.OldName]; wasRenamed {
				continue // handled by RenameColumn above
			}
		}
		column := f.ColumnName()
		plan.Operations = append(plan.Operations, ir.MigrationOp{
			Kind:   ir.OpAddColumn,
			Entity: e.Name,
			Table:  table,
			Column: column,
			DBType: f.DatabaseType,
			Columns: []ir.MigrationColumn{{
				Name:           f.Name,
				Column:         column,
				DBType:         f.DatabaseType,
				Nullable:       f.IsNullable,
				IsPrimary:      f.IsPrimary,
				IsRelation:     f.IsRelation,
				RelationTarget: f.RelationTarget,
			}},
		})
	}

	// DropColumn: field existed before but is gone now (and not renamed).
	oldFieldNames := slices.Sorted(maps.Keys(oldFields))
	for _, fn := range oldFieldNames {
		if _, stillExists := newFieldByName[fn]; stillExists {
			continue
		}
		if renamedTo := renameTarget(e, fn); renamedTo != "" {
			continue
		}
		plan.Operations = append(plan.Operations, ir.MigrationOp{
			Kind:   ir.OpDropColumn,
			Entity: e.Name,
			Table:  table,
			Column: textutil.ToDatabaseColumnName(fn),
		})
	}

	// AlterColumn: same field, different database type.
	for _, fn := range oldFieldNames {
		f, ok := newFieldByName[fn]
		if !ok {
			continue
		}
		if oldType := oldFields[fn]; oldType != f.DatabaseType {
			column := f.ColumnName()
			plan.Operations = append(plan.Operations, ir.MigrationOp{
				Kind:      ir.OpAlterColumn,
				Entity:    e.Name,
				Table:     table,
				Column:    column,
				DBType:    f.DatabaseType,
				OldDBType: oldType,
			})
		}
	}
}

// renameTarget returns the new field name that renames from oldField, or "".
func renameTarget(e *ir.IREntity, oldField string) string {
	for i := range e.Fields {
		if e.Fields[i].OldName == oldField {
			return e.Fields[i].Name
		}
	}
	return ""
}
