package renderer

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/internal/templatefuncs"
	"github.com/DomainCraft/DomainCraft/pkg/logger"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"

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

func (r *Renderer) buildFuncMap() (template.FuncMap, error) {
	funcMap := templatefuncs.FuncMap()
	// Renderer/IR-specific functions that need renderer context or IR types.
	funcMap["jsonValue"] = jsonValue
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
		ArrayFormat:    cmp.Or(overlay.ArrayFormat, base.ArrayFormat),
		NullableFormat: cmp.Or(overlay.NullableFormat, base.NullableFormat),
		EnumNullable:   overlay.EnumNullable || base.EnumNullable,
		ValueTypes:     overlay.ValueTypes,
		Literals:       mergeLiterals(base.Literals, overlay.Literals),
		Array: ArrayLiteralSpec{
			Open:  cmp.Or(overlay.Array.Open, base.Array.Open),
			Close: cmp.Or(overlay.Array.Close, base.Array.Close),
			Empty: cmp.Or(overlay.Array.Empty, base.Array.Empty),
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

func jsonValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}