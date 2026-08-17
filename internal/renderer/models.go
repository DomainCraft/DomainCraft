package renderer

import (
	"github.com/DomainCraft/DomainCraft/internal/ir"
)

// BridgeConfig describes bridge.yaml.
type BridgeConfig struct {
	Name             string            `yaml:"name"`
	Description      string            `yaml:"description"`
	OutputDir        string            `yaml:"output_dir"`
	Extends          string            `yaml:"extends"`           // Optional base bridge (path / registry ID / owner-repo) this bridge composes on top of
	Helpers          string            `yaml:"helpers"`           // Optional shared template file with named templates
	RegistryURL      string            `yaml:"registry_url"`      // URL template for package registry ({id} = lowercase package ID)
	RegistryPackages map[string]string `yaml:"registry_packages"` // logical key -> registry package ID (used with registry_url)
	Templates        []TemplateSpec    `yaml:"templates"`
	Delimiters       []string          `yaml:"delimiters"` // Optional custom delimiters [left, right], default ["{{", "}}"]
	Migrations       *MigrationConfig  `yaml:"migrations"` // Optional database migration commands run by `domaincraft generate --migrate`
}

// MigrationConfig lets a bridge declare how to apply schema changes to a real
// database after code generation. The CLI runs the commands in order (from the
// generated output directory) only when `--migrate` is passed. Commands are
// shell lines executed via the platform shell.
type MigrationConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Commands []string `yaml:"commands"`
}

// TemplateSpec describes one template rendering rule.
type TemplateSpec struct {
	For       string   `yaml:"for"`
	Source    string   `yaml:"source"`
	Target    string   `yaml:"target"`
	Targets   []string `yaml:"targets"`
	When      string   `yaml:"when"`      // Optional condition: "hasSeed", etc.
	Overwrite *bool    `yaml:"overwrite"` // false = scaffold once (skip if file exists); default: true
}

// IsCustom reports whether the template produces a developer-owned (scaffold)
// file that must not be overwritten on subsequent runs.
func (s TemplateSpec) IsCustom() bool {
	return s.Overwrite != nil && !*s.Overwrite
}

// TargetPatterns returns the configured target patterns, falling back to target.
func (s TemplateSpec) TargetPatterns() []string {
	if len(s.Targets) > 0 {
		patterns := make([]string, 0, len(s.Targets))
		for _, target := range s.Targets {
			if target == "" {
				continue
			}
			patterns = append(patterns, target)
		}
		if len(patterns) > 0 {
			return patterns
		}
	}
	if s.Target != "" {
		return []string{s.Target}
	}
	return nil
}

// RenderContext is passed to templates.
type RenderContext struct {
	Project  *ir.IRProject
	Entity   *ir.IREntity
	Bridge   *BridgeConfig     // Bridge-level config available as .Bridge
	Packages map[string]string // Resolved package versions for the current platform
	// Migration is the abstract schema-migration plan computed by the core from
	// the previous snapshot (nil on the first run or when nothing changed).
	Migration *ir.MigrationPlan
	// SeedData is the developer's explicit `seed:` rows from domain.yaml,
	// normalized by the core into the same typed SeedRecord shape as mock data.
	SeedData *ir.SeedDataset
	// MockData is deterministic, generated mock seed (same shape as SeedData).
	MockData *ir.SeedDataset
}

// Name exposes the current entity name to templates.
func (c RenderContext) Name() string {
	if c.Entity == nil {
		return ""
	}
	return c.Entity.Name
}

// NamePlural exposes the current entity plural name to templates.
func (c RenderContext) NamePlural() string {
	if c.Entity == nil {
		return ""
	}
	return c.Entity.NamePlural
}

// HasAudit reports whether the current entity has audit fields enabled.
func (c RenderContext) HasAudit() bool {
	return c.Entity != nil && c.Entity.HasAudit()
}

// HasAuditLog reports whether the current entity has audit-log fields enabled.
func (c RenderContext) HasAuditLog() bool {
	return c.Entity != nil && c.Entity.HasAuditLog()
}

// HasSoftDelete reports whether the current entity has soft delete enabled.
func (c RenderContext) HasSoftDelete() bool {
	return c.Entity != nil && c.Entity.HasSoftDelete()
}

// HasOptimisticLock reports whether the current entity has optimistic locking enabled.
func (c RenderContext) HasOptimisticLock() bool {
	return c.Entity != nil && c.Entity.HasOptimisticLock()
}

// Permissions exposes the current entity permissions to templates.
func (c RenderContext) Permissions() *ir.IRPermissions {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.Permissions
}

// PermissionPlan exposes the core-computed authorization plan for one operation.
func (c RenderContext) PermissionPlan(operation string) *ir.IRPermissionPlan {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.PermissionPlan(operation)
}

// Endpoints exposes the standard HTTP endpoint contract for the current entity.
func (c RenderContext) Endpoints() []ir.IREndpoint {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.Endpoints()
}

// AllIndexes exposes the normalized index list (declared + implicit unique).
func (c RenderContext) AllIndexes() []ir.IRIndex {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.AllIndexes()
}

// Seed exposes normalized seed rows to templates: the current entity's rows at
// entity scope, or every entity's rows flattened at project scope.
func (c RenderContext) Seed() []ir.SeedRecord {
	if c.SeedData == nil {
		return nil
	}
	if c.Entity != nil {
		for i := range c.SeedData.Entities {
			if c.SeedData.Entities[i].Name == c.Entity.Name {
				return c.SeedData.Entities[i].Records
			}
		}
		return nil
	}
	var out []ir.SeedRecord
	for i := range c.SeedData.Entities {
		out = append(out, c.SeedData.Entities[i].Records...)
	}
	return out
}

// HasSeedData reports whether the context carries normalized explicit seed for
// this entity (or, project-level, whether any entity has seed). When no
// normalized dataset was attached it falls back to the entity's raw seed rows
// so bridges relying on `when: hasSeed` still work.
func (c RenderContext) HasSeedData() bool {
	if c.SeedData != nil {
		if c.Entity == nil {
			for i := range c.SeedData.Entities {
				if len(c.SeedData.Entities[i].Records) > 0 {
					return true
				}
			}
			return false
		}
		for i := range c.SeedData.Entities {
			if c.SeedData.Entities[i].Name == c.Entity.Name && len(c.SeedData.Entities[i].Records) > 0 {
				return true
			}
		}
		return false
	}
	if c.Entity != nil {
		return len(c.Entity.Seed) > 0
	}
	if c.Project != nil {
		for _, e := range c.Project.Entities {
			if len(e.Seed) > 0 {
				return true
			}
		}
	}
	return false
}

// HasFeature reports whether the current entity has the named feature enabled.
func (c RenderContext) HasFeature(name string) bool {
	return c.Entity != nil && c.Entity.HasFeature(name)
}

// HasAddon reports whether the named infrastructure addon is enabled at the
// project level (e.g. "dapr"). Templates use this to toggle addon-specific code.
func (c RenderContext) HasAddon(name string) bool {
	return c.Project != nil && c.Project.HasAddon(name)
}

// HasEventSourced reports whether the current entity emits domain events.
func (c RenderContext) HasEventSourced() bool {
	return c.Entity != nil && c.Entity.HasEventSourced()
}

// HasCacheable reports whether the current entity should use the distributed cache.
func (c RenderContext) HasCacheable() bool {
	return c.Entity != nil && c.Entity.HasCacheable()
}

// ReadFields exposes the current entity's read-DTO projection to templates.
func (c RenderContext) ReadFields() []ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.ReadFields()
}

// CreateFields exposes the current entity's create-DTO projection to templates.
func (c RenderContext) CreateFields() []ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.CreateFields()
}

// UpdateFields exposes the current entity's update-DTO projection to templates.
func (c RenderContext) UpdateFields() []ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.UpdateFields()
}

// PatchFields exposes the current entity's patch-DTO projection to templates:
// the core's patchable surface (scalars + single foreign keys) plus the
// optimistic-lock concurrency token.
func (c RenderContext) PatchFields() []ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.PatchFields()
}

// SearchableFields exposes the scalar text fields eligible for free-text search.
func (c RenderContext) SearchableFields() []ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.SearchableFields()
}

// SortableFields exposes the scalar fields a list endpoint may order by.
func (c RenderContext) SortableFields() []ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.SortableFields()
}

// FilterableFields exposes the scalar fields that may be filtered by value,
// each carrying its supported operators via IRField.FilterOperators().
func (c RenderContext) FilterableFields() []ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.FilterableFields()
}

// FilterablePaths exposes the complete filter path schema: scalar fields,
// one-hop relation paths and JSON roots, each with its allowed operators.
func (c RenderContext) FilterablePaths() []ir.FilterPathSpec {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.FilterablePaths()
}

// QuerySchema exposes the entity's whole list-query surface (search + sort +
// filter) in one object — the single entry point for a bridge's runtime query
// validator.
func (c RenderContext) QuerySchema() ir.QuerySchema {
	if c.Entity == nil {
		return ir.QuerySchema{}
	}
	return c.Entity.QuerySchema()
}

// HasMigration reports whether the context carries a non-empty migration plan.
func (c RenderContext) HasMigration() bool {
	return c.Migration != nil && !c.Migration.IsEmpty()
}

// HasMockData reports whether the context carries mock data for this entity.
func (c RenderContext) HasMockData() bool {
	if c.MockData == nil {
		return false
	}
	if c.Entity == nil {
		return len(c.MockData.Entities) > 0
	}
	for _, em := range c.MockData.Entities {
		if em.Name == c.Entity.Name && len(em.Records) > 0 {
			return true
		}
	}
	return false
}

// CursorField returns the keyset-pagination cursor field (the PK when it is a
// monotonic integer), or nil when keyset is unsupported for this entity.
func (c RenderContext) CursorField() *ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.CursorField()
}

// PrimaryKey returns the primary key field of the current entity, or nil if not found.
func (c RenderContext) PrimaryKey() *ir.IRField {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.PrimaryKey()
}

// RenderedFile describes a single file produced by the renderer.
// It is the unit of the file manifest recorded in the schema snapshot.
type RenderedFile struct {
	Path    string `json:"path"`             // path relative to the output dir, forward slashes
	Entity  string `json:"entity,omitempty"` // entity name the file was generated for ("" = project-level)
	Custom  bool   `json:"custom"`           // true when the template declared overwrite: false (developer-owned scaffold)
	Written bool   `json:"written"`          // true when the file was created this run; false when skipped because it already existed
}
