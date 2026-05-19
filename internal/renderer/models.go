package renderer

import "domaincraft/internal/ir"

// BridgeConfig describes bridge.yaml.
type BridgeConfig struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	OutputDir   string         `yaml:"output_dir"`
	Templates   []TemplateSpec `yaml:"templates"`
}

// TemplateSpec describes one template rendering rule.
type TemplateSpec struct {
	For     string   `yaml:"for"`
	Source  string   `yaml:"source"`
	Target  string   `yaml:"target"`
	Targets []string `yaml:"targets"`
	When    string   `yaml:"when"` // Optional condition: "hasSeed", etc.
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
	Project *ir.IRProject
	Entity  *ir.IREntity
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
	return c.Entity != nil && c.Entity.HasAudit
}

// HasSoftDelete reports whether the current entity has soft delete enabled.
func (c RenderContext) HasSoftDelete() bool {
	return c.Entity != nil && c.Entity.HasSoftDelete
}

// HasOptimisticLock reports whether the current entity has optimistic locking enabled.
func (c RenderContext) HasOptimisticLock() bool {
	return c.Entity != nil && c.Entity.HasOptimisticLock
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
