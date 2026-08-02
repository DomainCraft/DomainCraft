package renderer

import "github.com/DomainCraft/DomainCraft/internal/ir"

// BridgeConfig describes bridge.yaml.
type BridgeConfig struct {
	Name             string            `yaml:"name"`
	Description      string            `yaml:"description"`
	OutputDir        string            `yaml:"output_dir"`
	Helpers          string            `yaml:"helpers"`           // Optional shared template file with named templates
	RegistryURL      string            `yaml:"registry_url"`      // URL template for package registry ({id} = lowercase package ID)
	RegistryPackages map[string]string `yaml:"registry_packages"` // logical key -> registry package ID (used with registry_url)
	Templates        []TemplateSpec    `yaml:"templates"`
	Delimiters       []string          `yaml:"delimiters"` // Optional custom delimiters [left, right], default ["{{", "}}"]
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

// Seed exposes the current entity seed data to templates.
func (c RenderContext) Seed() []map[string]interface{} {
	if c.Entity == nil {
		return nil
	}
	return c.Entity.Seed
}

// HasFeature reports whether the current entity has the named feature enabled.
func (c RenderContext) HasFeature(name string) bool {
	return c.Entity != nil && c.Entity.HasFeature(name)
}

// PrimaryKey returns the primary key field of the current entity, or nil if not found.
func (c RenderContext) PrimaryKey() *ir.IRField {
	if c.Entity == nil {
		return nil
	}
	for i := range c.Entity.Fields {
		if c.Entity.Fields[i].IsPrimary {
			return &c.Entity.Fields[i]
		}
	}
	return nil
}

// RenderedFile describes a single file produced by the renderer.
// It is the unit of the file manifest recorded in the schema snapshot.
type RenderedFile struct {
	Path    string `json:"path"`             // path relative to the output dir, forward slashes
	Entity  string `json:"entity,omitempty"` // entity name the file was generated for ("" = project-level)
	Custom  bool   `json:"custom"`           // true when the template declared overwrite: false (developer-owned scaffold)
	Written bool   `json:"written"`          // true when the file was created this run; false when skipped because it already existed
}
