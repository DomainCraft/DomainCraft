package ir

// MigrationOpKind enumerates the provider-agnostic schema operations the core
// emits when comparing the previous domain model (snapshot) against the current
// IR. Bridges render each op into their own migration syntax (EF Core,
// Alembic, a NoSQL collection transform, an exotic SQL dialect, ...).
type MigrationOpKind string

const (
	// OpCreateTable — a brand-new entity; Columns holds the full column list.
	OpCreateTable MigrationOpKind = "create_table"
	// OpDropTable — an entity removed from the model.
	OpDropTable MigrationOpKind = "drop_table"
	// OpRenameTable — an entity renamed via old_name (table name changed).
	OpRenameTable MigrationOpKind = "rename_table"
	// OpAddColumn — a field added to an existing entity.
	OpAddColumn MigrationOpKind = "add_column"
	// OpDropColumn — a field removed from an existing entity.
	OpDropColumn MigrationOpKind = "drop_column"
	// OpRenameColumn — a field renamed via the old_name modifier.
	OpRenameColumn MigrationOpKind = "rename_column"
	// OpAlterColumn — a field whose database type changed.
	OpAlterColumn MigrationOpKind = "alter_column"
)

// MigrationColumn describes one column for a create_table operation. Column
// names are bridge-agnostic snake_case; DBType is the IR database type.
type MigrationColumn struct {
	Name           string // logical field name
	Column         string // snake_case database column
	DBType         string // IR database type
	Nullable       bool
	IsPrimary      bool
	IsRelation     bool
	RelationTarget string
}

// MigrationOp is one abstract schema operation.
type MigrationOp struct {
	Kind   MigrationOpKind
	Entity string // current entity name
	// OldName / OldTable / OldColumn reference the previous model for rename
	// and drop operations.
	OldName   string
	Table     string // current snake_case table name
	OldTable  string
	Column    string // current snake_case column name
	OldColumn string
	DBType    string // current IR type (add/alter/rename)
	OldDBType string // previous IR type (alter/rename)
	Columns   []MigrationColumn
}

// MigrationPlan is the ordered, deterministic migration tree computed by the
// core from the old snapshot and the current IR.
type MigrationPlan struct {
	Operations []MigrationOp
}

// IsEmpty reports whether the plan contains no schema changes.
func (p *MigrationPlan) IsEmpty() bool {
	return p == nil || len(p.Operations) == 0
}
