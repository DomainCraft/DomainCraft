package renderer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/internal/packages"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/pkg/logger"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

// LiteralSpec declares how a bridge writes a scalar literal of one IR type in
// its target language. Loaded from type_mappings.yaml `literals:` (keyed by IR
// database type; "enum" is a reserved key for user-declared enums).
type LiteralSpec struct {
	// Parse is a printf template (one %s) that turns a serialized value into an
	// expression, e.g. "Guid.Parse(%s)" or "JsonDocument.Parse(%s, default)".
	Parse string `yaml:"parse"`
	// Suffix is appended to a bare numeric literal, e.g. "L" (long) or "m" (decimal).
	Suffix string `yaml:"suffix"`
	// Default is the value-less default expression, e.g. "Guid.NewGuid()" or "0m".
	Default string `yaml:"default"`
	// Member (enums) is a printf template (type, value) for a compile-time member
	// reference, e.g. "%s.%s" -> "OrderStatus.Draft".
	Member string `yaml:"member"`
	// Zero (enums) is a printf template (type) for a zero-valued enum, e.g. "(%s)0".
	Zero string `yaml:"zero"`
}

// ArrayLiteralSpec declares how a bridge writes array literals. %s placeholders
// are filled with the bridge's array type (e.g. "List<int>").
type ArrayLiteralSpec struct {
	Open  string `yaml:"open"`  // e.g. "new %s { "
	Close string `yaml:"close"` // e.g. " }"
	Empty string `yaml:"empty"` // e.g. "new %s()"
}

// typeMappings is the raw shape of a bridge's type_mappings.yaml. A composed
// renderer merges these across its bridge chain (adapter wins on conflicts).
type typeMappings struct {
	Types          map[string]string      `yaml:"types"`
	Behaviors      map[string]string      `yaml:"behaviors"`
	ValueTypes     []string               `yaml:"value_types"`
	ArrayFormat    string                 `yaml:"array_format"`
	EnumNullable   bool                   `yaml:"enum_nullable"`
	NullableFormat string                 `yaml:"nullable_format"`
	InputTypes     map[string]string      `yaml:"input_types"`
	Literals       map[string]LiteralSpec `yaml:"literals"`
	Array          ArrayLiteralSpec       `yaml:"array"`
	ColumnSizes    map[string]string      `yaml:"column_sizes"`
}

// sourceConfig is one bridge directory plus its parsed bridge.yaml. A composed
// renderer holds several of these, ordered base-first, with the adapter last.
type sourceConfig struct {
	dir    string
	config BridgeConfig
}

// Renderer reads bridge.yaml and renders files from IR.
//
// A renderer may be composed from a chain of bridges: the optional base bridges
// (declared via `extends:`) render first, then the adapter on top. Base
// templates, helper templates and type_mappings are all merged; the adapter's
// values win on conflicts.
type Renderer struct {
	sources []sourceConfig // ordered base-first; last entry is the adapter
	log     *logger.Logger
	// migration is the abstract schema-migration plan (set via SetMigration).
	migration *ir.MigrationPlan
	// seedData is the normalized explicit seed (set via SetSeedData).
	seedData *ir.SeedDataset
	// mockData is the deterministic mock seed (set via SetMockData).
	mockData *ir.SeedDataset
}

// config returns the topmost (adapter) bridge config, which owns the effective
// output_dir, name, delimiters and migration commands.
func (r *Renderer) config() BridgeConfig {
	if len(r.sources) == 0 {
		return BridgeConfig{}
	}
	return r.sources[len(r.sources)-1].config
}

// adapterDir returns the directory of the topmost (adapter) bridge.
func (r *Renderer) adapterDir() string {
	if len(r.sources) == 0 {
		return ""
	}
	return r.sources[len(r.sources)-1].dir
}

// loadSource loads a single bridge directory (its bridge.yaml).
func loadSource(bridgePath string) (sourceConfig, error) {
	bridgeDir, configPath := resolveBridgePath(bridgePath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return sourceConfig{}, fmt.Errorf("read bridge config: %w", err)
	}

	var config BridgeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return sourceConfig{}, fmt.Errorf("parse bridge config: %w", err)
	}
	if config.OutputDir == "" {
		config.OutputDir = "generated"
	}
	return sourceConfig{dir: bridgeDir, config: config}, nil
}

// New loads a single (leaf) bridge.
func New(bridgePath string, log *logger.Logger) (*Renderer, error) {
	src, err := loadSource(bridgePath)
	if err != nil {
		return nil, err
	}
	return &Renderer{sources: []sourceConfig{src}, log: log}, nil
}

// Extends reports the base bridge this bridge composes on top of (a path,
// registry ID or owner/repo shorthand), or "" when it is a leaf bridge.
func (r *Renderer) Extends() string {
	return r.config().Extends
}

// AttachBases prepends resolved base bridge directories (outermost base first)
// under this bridge. Bases render before the adapter, and the adapter's
// type_mappings/helpers override the bases' on conflicts.
func (r *Renderer) AttachBases(basePaths []string) error {
	bases := make([]sourceConfig, 0, len(basePaths)+len(r.sources))
	for _, p := range basePaths {
		src, err := loadSource(p)
		if err != nil {
			return err
		}
		bases = append(bases, src)
	}
	r.sources = append(bases, r.sources...)
	return nil
}

// MigrationConfig exposes the bridge's declared database-migration commands
// (from migration.yaml bridge manifest), used by `domaincraft generate --migrate`.
func (r *Renderer) MigrationConfig() *MigrationConfig {
	return r.config().Migrations
}

// SetMigration attaches the core-computed schema migration plan to this renderer.
func (r *Renderer) SetMigration(plan *ir.MigrationPlan) { r.migration = plan }

// SetSeedData attaches the core-normalized explicit seed to this renderer.
func (r *Renderer) SetSeedData(ds *ir.SeedDataset) { r.seedData = ds }

// SetMockData attaches the core-generated mock seed to this renderer.
func (r *Renderer) SetMockData(ds *ir.SeedDataset) { r.mockData = ds }

// delimiters returns the configured template delimiters, defaulting to ["{{", "}}"].
func (r *Renderer) delimiters() (string, string) {
	if len(r.config().Delimiters) >= 2 {
		return r.config().Delimiters[0], r.config().Delimiters[1]
	}
	return "{{", "}}"
}

// applyDelimiters sets custom delimiters on a template if configured.
func (r *Renderer) applyDelimiters(t *template.Template) *template.Template {
	left, right := r.delimiters()
	if left != "{{" || right != "}}" {
		return t.Delims(left, right)
	}
	return t
}

func (r *Renderer) buildFuncMap() (template.FuncMap, error) {
	funcMap := template.FuncMap{}
	for key, value := range sprig.FuncMap() {
		funcMap[key] = value
	}
	// Generic language-agnostic functions
	funcMap["pluralize"] = textutil.Pluralize
	// pascalcase/camelcase accept any value and coerce it to a string first, so
	// bridges can pipe the IR's named string types (e.g. FilterOp) without
	// tripping over Go's nominal typing — the underlying value is already the
	// wire name the bridge needs.
	funcMap["pascalcase"] = func(v any) string { return textutil.PascalCase(stringArg(v)) }
	funcMap["camelcase"] = func(v any) string { return textutil.CamelCase(stringArg(v)) }
	funcMap["lowercase"] = strings.ToLower
	funcMap["uppercase"] = strings.ToUpper
	funcMap["humanize"] = func(name string) string {
		return strings.Join(textutil.SplitIdentifier(name), " ")
	}
	funcMap["jsonValue"] = jsonValue
	funcMap["fkName"] = textutil.FKName
	// seedKind maps an IR database type to its language-agnostic value kind
	// (uuid/text/int/.../enum/array), used by bridges that serialize seed values.
	funcMap["seedKind"] = ir.KindOf

	// Bridge-specific functions
	bridgeFuncs, err := r.getBridgeSpecificFuncs()
	if err != nil {
		return nil, err
	}
	for key, value := range bridgeFuncs {
		funcMap[key] = value
	}
	return funcMap, nil
}

// loadTypeMappings reads a bridge directory's type_mappings.yaml. It returns nil
// (no error) when the file is absent — a bridge with no mappings contributes
// nothing, and the merged result drives the language functions.
func (r *Renderer) loadTypeMappings(dir string) (*typeMappings, error) {
	data, err := os.ReadFile(filepath.Join(dir, "type_mappings.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m typeMappings
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse type_mappings.yaml: %w", err)
	}
	return &m, nil
}

// mergeTypeMappings overlays the adapter's mappings on the base's. Map fields
// merge key-by-key (adapter wins); scalar fields fall back to the base when the
// adapter leaves them zero-valued.
func mergeTypeMappings(base, overlay *typeMappings) *typeMappings {
	if base == nil {
		base = &typeMappings{}
	}
	if overlay == nil {
		overlay = &typeMappings{}
	}
	mergeStrings := func(dst, src map[string]string) map[string]string {
		if len(dst) == 0 && len(src) == 0 {
			return nil
		}
		m := make(map[string]string, len(dst)+len(src))
		for k, v := range dst {
			m[k] = v
		}
		for k, v := range src {
			m[k] = v
		}
		return m
	}
	out := &typeMappings{
		Types:          mergeStrings(base.Types, overlay.Types),
		Behaviors:      mergeStrings(base.Behaviors, overlay.Behaviors),
		InputTypes:     mergeStrings(base.InputTypes, overlay.InputTypes),
		ColumnSizes:    mergeStrings(base.ColumnSizes, overlay.ColumnSizes),
		ArrayFormat:    firstNonEmpty(overlay.ArrayFormat, base.ArrayFormat),
		NullableFormat: firstNonEmpty(overlay.NullableFormat, base.NullableFormat),
		EnumNullable:   overlay.EnumNullable || base.EnumNullable,
		ValueTypes:     overlay.ValueTypes,
		Literals:       mergeLiterals(base.Literals, overlay.Literals),
		Array: ArrayLiteralSpec{
			Open:  firstNonEmpty(overlay.Array.Open, base.Array.Open),
			Close: firstNonEmpty(overlay.Array.Close, base.Array.Close),
			Empty: firstNonEmpty(overlay.Array.Empty, base.Array.Empty),
		},
	}
	if len(out.ValueTypes) == 0 {
		out.ValueTypes = base.ValueTypes
	}
	return out
}

func mergeLiterals(base, overlay map[string]LiteralSpec) map[string]LiteralSpec {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	m := make(map[string]LiteralSpec, len(base)+len(overlay))
	for k, v := range base {
		m[k] = v
	}
	for k, v := range overlay {
		m[k] = v
	}
	return m
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// getBridgeSpecificFuncs loads type_mappings from every bridge in the chain
// (base-first, adapter last) and merges them before building the language
// functions, so an adapter inherits its base's type mapping and can override it.
func (r *Renderer) getBridgeSpecificFuncs() (map[string]interface{}, error) {
	var merged *typeMappings
	for _, src := range r.sources {
		m, err := r.loadTypeMappings(src.dir)
		if err != nil {
			return nil, err
		}
		merged = mergeTypeMappings(merged, m)
	}
	return buildBridgeFuncs(merged, r.log), nil
}

// buildBridgeFuncs turns a merged type_mappings into the template func map that
// bridges use to map IR types into language-specific code. A nil mapping (no
// type_mappings.yaml anywhere in the chain) yields no bridge-specific funcs.
func buildBridgeFuncs(m *typeMappings, log *logger.Logger) map[string]interface{} {
	bridgeFuncs := make(map[string]interface{})
	if m == nil {
		return bridgeFuncs
	}
	mapping := m

	arrayFormat := mapping.ArrayFormat
	if arrayFormat == "" {
		arrayFormat = "%s[]" // default: generic array syntax (override in type_mappings.yaml for target language)
	}

	nullableSuffix := mapping.NullableFormat
	if nullableSuffix == "" {
		nullableSuffix = "" // no default — bridges must set nullable_format explicitly
	}

	// wrapNullable applies the bridge-specific nullable format to a type name.
	wrapNullable := func(typeName string) string {
		return typeName + nullableSuffix
	}

	// Build set of value types for quick lookup
	valueTypeSet := make(map[string]bool, len(mapping.ValueTypes))
	for _, vt := range mapping.ValueTypes {
		valueTypeSet[vt] = true
	}

	// wrapArray wraps a type name in the bridge-specific array format.
	wrapArray := func(elementType string) string {
		return fmt.Sprintf(arrayFormat, elementType)
	}

	// languageType maps IR database types to language-specific types via type_mappings.yaml
	languageType := func(dbType string, nullable bool) string {
		// Check explicit type mapping first (covers primitives and arrays)
		if mapped, ok := mapping.Types[dbType]; ok {
			if nullable && !specmeta.IsArrayType(dbType) {
				return wrapNullable(mapped)
			}
			return mapped
		}

		inner := specmeta.ParseArrayInner(dbType)

		// Enum type (not a primitive, not in mapping) — format for target language
		if !specmeta.IsPrimitive(strings.ToLower(inner)) {
			formatted := textutil.PascalCase(inner)
			if specmeta.IsArrayType(dbType) {
				return wrapArray(formatted)
			}
			if nullable && mapping.EnumNullable {
				return wrapNullable(formatted)
			}
			return formatted
		}

		// Primitive array not in explicit mapping — wrap inner type
		if specmeta.IsArrayType(dbType) {
			if mapped, ok := mapping.Types[inner]; ok {
				return wrapArray(mapped)
			}
			log.Warn("type_mappings.yaml has no type for %q; emitting raw type name %q", inner, inner)
			return wrapArray(inner)
		}

		log.Warn("type_mappings.yaml has no type for %q; emitting raw type name %q", dbType, dbType)
		return dbType
	}
	bridgeFuncs["languageType"] = languageType

	// literalValue renders a scalar value of an IR database type as a bridge
	// literal/parse expression. value is a native Go value (string, int64,
	// float64, bool) or the raw string from a domain.yaml default.
	bridgeFuncs["literalValue"] = func(dbType string, value any) string {
		kind := ir.KindOf(dbType)
		spec := mapping.Literals[dbType]
		switch kind {
		case "text":
			return jsonValue(value)
		case "int", "bigint", "float", "decimal", "bool":
			return fmt.Sprintf("%v", value) + spec.Suffix
		case "uuid", "date", "datetime", "json":
			if spec.Parse == "" {
				return jsonValue(value)
			}
			return fmt.Sprintf(spec.Parse, jsonValue(value))
		case "enum":
			e := mapping.Literals["enum"]
			if e.Parse == "" {
				return jsonValue(value)
			}
			return fmt.Sprintf(e.Parse, languageType(dbType, false), jsonValue(textutil.PascalCase(fmt.Sprintf("%v", value))))
		}
		return jsonValue(value)
	}

	// literalDefault renders the value-less default expression for a type:
	// a scalar default, a zero-valued enum, or an empty array.
	bridgeFuncs["literalDefault"] = func(dbType string) string {
		if ir.KindOf(dbType) == "array" {
			return fmt.Sprintf(mapping.Array.Empty, languageType(dbType, false))
		}
		spec := mapping.Literals[dbType]
		if ir.KindOf(dbType) == "enum" {
			if spec.Zero != "" {
				return fmt.Sprintf(spec.Zero, languageType(dbType, false))
			}
			return fmt.Sprintf("(%s)0", languageType(dbType, false))
		}
		if spec.Default == "" {
			return "default(" + languageType(dbType, false) + ")"
		}
		return spec.Default
	}

	// literalMember renders a compile-time enum member reference (Type.Value),
	// used for enum defaults in domain.yaml.
	bridgeFuncs["literalMember"] = func(dbType string, value string) string {
		if ir.KindOf(dbType) == "enum" {
			if spec := mapping.Literals["enum"]; spec.Member != "" {
				return fmt.Sprintf(spec.Member, languageType(dbType, false), textutil.PascalCase(value))
			}
			return languageType(dbType, false) + "." + textutil.PascalCase(value)
		}
		return jsonValue(value)
	}

	// arrayLiteralOpen/Close wrap rendered elements in the bridge's array
	// syntax (e.g. "new List<int> { " ... " }").
	bridgeFuncs["arrayLiteralOpen"] = func(dbType string) string {
		return fmt.Sprintf(mapping.Array.Open, languageType(dbType, false))
	}
	bridgeFuncs["arrayLiteralClose"] = func() string {
		return mapping.Array.Close
	}

	bridgeFuncs["isValueType"] = func(typeName string) bool {
		if mapped, ok := mapping.Types[typeName]; ok {
			return valueTypeSet[mapped]
		}
		return false
	}

	bridgeFuncs["deleteBehaviorName"] = func(value string) string {
		key := strings.ToLower(strings.TrimSpace(value))
		if v, ok := mapping.Behaviors[key]; ok {
			return v
		}
		return textutil.PascalCase(value)
	}

	// columnSize returns the bridge's declared default column size for an IR
	// type (e.g. appwrite varchar string -> 255, uuid -> 36), or "" when the
	// bridge declared no size for that type. Sizes are bridge-specific, so
	// they live in type_mappings.yaml `column_sizes:`, keyed by IR type name.
	// The input is the IR type name (element type for arrays, the field type
	// otherwise).
	if len(mapping.ColumnSizes) > 0 {
		bridgeFuncs["columnSize"] = func(dbType string) string {
			return mapping.ColumnSizes[dbType]
		}
	}

	// inputType maps IR database types to UI input components (e.g. "string" -> "Input", "datetime" -> "DatePicker").
	// Bridges that generate UI code define input_types in their type_mappings.yaml.
	if len(mapping.InputTypes) > 0 {
		bridgeFuncs["inputType"] = func(dbType string) string {
			inner := specmeta.ParseArrayInner(dbType)
			if mapped, ok := mapping.InputTypes[inner]; ok {
				return mapped
			}
			if mapped, ok := mapping.InputTypes[dbType]; ok {
				return mapped
			}
			return "Input"
		}
	}

	return bridgeFuncs
}

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
	packages := r.resolvePackages()

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
			if !r.shouldRender(spec, project, packages) {
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

			contexts, err := r.renderContexts(spec.For, project, packages)
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

func (r *Renderer) shouldRender(spec TemplateSpec, project *ir.IRProject, packages map[string]string) bool {
	// Entity-level When conditions are checked per-context in shouldRenderContext.
	// Project-level When conditions are checked here.
	if spec.When == "" || spec.For == "entity" {
		return true
	}
	cfg := r.config()
	return r.shouldRenderContext(spec, RenderContext{Project: project, Bridge: &cfg, Packages: packages, Migration: r.migration, SeedData: r.seedData, MockData: r.mockData})
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

func resolveBridgePath(bridgePath string) (string, string) {
	info, err := os.Stat(bridgePath)
	if err != nil {
		// Path does not exist or is not accessible — treat as file path.
		return filepath.Dir(bridgePath), bridgePath
	}
	if info.IsDir() {
		return bridgePath, filepath.Join(bridgePath, "bridge.yaml")
	}
	return filepath.Dir(bridgePath), bridgePath
}

func renderTemplateString(pattern string, data any, funcMap template.FuncMap) (string, error) {
	tpl, err := template.New("path").Funcs(funcMap).Parse(pattern)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	if err := tpl.Execute(&builder, data); err != nil {
		return "", err
	}
	return builder.String(), nil
}

// stringArg coerces a template argument to its string form. It backs the
// lenient case-transform functions (pascalcase/camelcase): plain strings pass
// through, fmt.Stringer values use String(), and anything else (including the
// IR's named string types such as FilterOp) is formatted with fmt.Sprint.
func stringArg(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

func jsonValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}
