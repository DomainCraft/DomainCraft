package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/internal/packages"
)

func (r *Renderer) Render(project *ir.IRProject, outputDir string) ([]string, []RenderedFile, error) {
	if project == nil {
		return nil, nil, fmt.Errorf("IR project is nil")
	}
	if outputDir == "" {
		outputDir = r.config().OutputDir
	}

	funcMap, err := r.buildFuncMap()
	if err != nil {
		return nil, nil, fmt.Errorf("build template functions: %w", err)
	}

	// Resolve package versions once — avoids repeated HTTP requests per template.
	pkgs := r.resolvePackages()

	// Parse shared helper templates from every bridge in the chain (base first,
	// adapter last). They define named templates ({{ define "name" }}) that all
	// other templates can call; a composed adapter inherits its bases' helpers.
	var helpersTemplate *template.Template
	for _, src := range r.sources {
		if src.config.Helpers == "" {
			continue
		}
		helperPath := filepath.Join(src.dir, src.config.Helpers)
		helperBytes, err := os.ReadFile(helperPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read helpers %s: %w", src.config.Helpers, err)
		}
		if helpersTemplate == nil {
			helpersTemplate, err = r.applyDelimiters(template.New("helpers").Funcs(funcMap)).Parse(string(helperBytes))
		} else {
			helpersTemplate, err = helpersTemplate.Parse(string(helperBytes))
		}
		if err != nil {
			return nil, nil, fmt.Errorf("parse helpers %s: %w", src.config.Helpers, err)
		}
	}

	writtenFiles := make([]string, 0)
	manifest := make([]RenderedFile, 0)
	for _, src := range r.sources {
		for _, spec := range src.config.Templates {
			// Check if template should be rendered based on "When" condition
			if !r.shouldRender(spec, project, pkgs) {
				continue
			}

			sourcePath := filepath.Join(src.dir, spec.Source)
			tplBytes, err := os.ReadFile(sourcePath)
			if err != nil {
				return nil, nil, fmt.Errorf("read template %s: %w", spec.Source, err)
			}

			tplName := filepath.Base(spec.Source)
			var parsedTemplate *template.Template
			if helpersTemplate != nil {
				// Clone helpers so all named templates are available
				parsedTemplate, err = helpersTemplate.Clone()
				if err != nil {
					return nil, nil, fmt.Errorf("clone helpers for %s: %w", spec.Source, err)
				}
				parsedTemplate, err = r.applyDelimiters(parsedTemplate.New(tplName)).Parse(string(tplBytes))
			} else {
				parsedTemplate, err = r.applyDelimiters(template.New(tplName).Funcs(funcMap)).Parse(string(tplBytes))
			}
			if err != nil {
				return nil, nil, fmt.Errorf("parse template %s: %w", spec.Source, err)
			}

			contexts, err := r.renderContexts(spec.For, project, pkgs)
			if err != nil {
				return nil, nil, err
			}

			for _, context := range contexts {
				// Check if this specific context should be rendered based on "When" condition
				if !r.shouldRenderContext(spec, context) {
					continue
				}

				entityName := ""
				if context.Entity != nil {
					entityName = context.Entity.Name
				}

				for _, targetPattern := range spec.TargetPatterns() {
					renderedTarget, err := renderTemplateString(targetPattern, context, funcMap)
					if err != nil {
						return nil, nil, fmt.Errorf("render target path: %w", err)
					}

					absoluteTarget := filepath.Join(outputDir, filepath.FromSlash(renderedTarget))
					absOutput, err := filepath.Abs(outputDir)
					if err != nil {
						return nil, nil, fmt.Errorf("resolve output directory: %w", err)
					}
					absTarget, err := filepath.Abs(absoluteTarget)
					if err != nil {
						return nil, nil, fmt.Errorf("resolve target path: %w", err)
					}
					if !strings.HasPrefix(absTarget, absOutput+string(filepath.Separator)) {
						return nil, nil, fmt.Errorf("template target path escapes output directory: %s", renderedTarget)
					}

					relPath := filepath.ToSlash(renderedTarget)
					record := RenderedFile{
						Path:    relPath,
						Entity:  entityName,
						Custom:  spec.IsCustom(),
						Written: true,
					}

					// Scaffold semantics: overwrite: false files are created only once.
					// The developer owns them afterwards, so they survive regeneration.
					if spec.IsCustom() {
						if _, statErr := os.Stat(absoluteTarget); statErr == nil {
							record.Written = false
							manifest = append(manifest, record)
							continue
						}
					}

					if err := os.MkdirAll(filepath.Dir(absoluteTarget), 0o755); err != nil {
						return nil, nil, fmt.Errorf("create output dir: %w", err)
					}

					file, err := os.Create(absoluteTarget)
					if err != nil {
						return nil, nil, fmt.Errorf("create output file: %w", err)
					}

					if err := parsedTemplate.Execute(file, context); err != nil {
						_ = file.Close()
						_ = os.Remove(absoluteTarget)
						return nil, nil, fmt.Errorf("execute template: %w", err)
					}
					if err := file.Close(); err != nil {
						_ = os.Remove(absoluteTarget)
						return nil, nil, err
					}
					writtenFiles = append(writtenFiles, absoluteTarget)
					manifest = append(manifest, record)
				}
			}
		}
	}

	return writtenFiles, manifest, nil
}

func (r *Renderer) shouldRender(spec TemplateSpec, project *ir.IRProject, pkgs map[string]string) bool {
	// Entity-level When conditions are checked per-context in shouldRenderContext.
	// Project-level When conditions are checked here.
	if spec.When == "" || spec.For == "entity" {
		return true
	}
	cfg := r.config()
	return r.shouldRenderContext(spec, RenderContext{Project: project, Bridge: &cfg, Packages: pkgs, Migration: r.migration, SeedData: r.seedData, MockData: r.mockData})
}

func (r *Renderer) shouldRenderContext(spec TemplateSpec, context RenderContext) bool {
	if spec.When == "" {
		return true
	}
	// Addon conditions accept a suffix after a colon, e.g. "hasAddon:dapr" /
	// "notHasAddon:dapr". Bare "hasAddon" treats the presence of any addon as
	// true.
	if cond, name, ok := strings.Cut(spec.When, ":"); ok {
		switch strings.TrimSpace(cond) {
		case "hasAddon":
			if context.Project != nil {
				if name == "" {
					return len(context.Project.Addons) > 0
				}
				return context.Project.HasAddon(strings.TrimSpace(name))
			}
			return false
		case "notHasAddon":
			if context.Project != nil {
				if name == "" {
					return len(context.Project.Addons) == 0
				}
				return !context.Project.HasAddon(strings.TrimSpace(name))
			}
			return true
		}
	}
	switch spec.When {
	case "hasSeed":
		// Only render seed templates if there's actual seed data.
		return context.HasSeedData()
	case "hasEnums":
		// Only render enum templates if there are enums defined
		if context.Project != nil && len(context.Project.Enums) > 0 {
			return true
		}
		return false
	case "hasOwnerTokens":
		// Only render owner resolver if any entity uses @Owner tokens
		if context.Project != nil {
			for _, e := range context.Project.Entities {
				if e.Permissions != nil && e.Permissions.HasOwnerToken() {
					return true
				}
			}
		}
		return false
	case "hasAuth":
		return context.Project != nil && context.Project.HasAuth()
	case "hasMigration":
		// True when the core computed a non-empty schema migration plan.
		return context.HasMigration()
	case "hasMockData":
		// True when the context carries generated mock data.
		return context.HasMockData()
	default:
		return true
	}
}

// resolvePackages resolves all package versions from the package registry,
// cached per bridge so repeated runs don't hit the registry every time.
func (r *Renderer) resolvePackages() map[string]string {
	cfg := r.config()
	if len(cfg.RegistryPackages) == 0 {
		return nil
	}

	result := make(map[string]string, len(cfg.RegistryPackages))
	for key, packageID := range cfg.RegistryPackages {
		version, err := packages.ResolveVersionCached(r.cacheNamespace(), cfg.RegistryURL, packageID)
		if err != nil {
			r.log.Warn("failed to resolve package %s: %v", packageID, err)
			continue
		}
		if version != "" {
			result[key] = version
		}
	}
	return result
}

// cacheNamespace returns a stable, filesystem-safe identifier for this bridge,
// used to keep its package-version cache separate from other bridges'. The
// bridge.yaml name is preferred; a path-based fallback covers nameless bridges.
func (r *Renderer) cacheNamespace() string {
	name := strings.ToLower(strings.TrimSpace(r.config().Name))
	if name == "" {
		name = filepath.Base(r.adapterDir())
	}

	var b strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			b.WriteRune(ch)
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

func (r *Renderer) renderContexts(scope string, project *ir.IRProject, pkgs map[string]string) ([]RenderContext, error) {
	cfg := r.config()
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", "entity":
		contexts := make([]RenderContext, 0, len(project.Entities))
		for i := range project.Entities {
			contexts = append(contexts, RenderContext{Project: project, Entity: &project.Entities[i], Bridge: &cfg, Packages: pkgs, Migration: r.migration, SeedData: r.seedData, MockData: r.mockData})
		}
		return contexts, nil
	case "project":
		return []RenderContext{{Project: project, Bridge: &cfg, Packages: pkgs, Migration: r.migration, SeedData: r.seedData, MockData: r.mockData}}, nil
	default:
		return nil, fmt.Errorf("unsupported template scope '%s'", scope)
	}
}