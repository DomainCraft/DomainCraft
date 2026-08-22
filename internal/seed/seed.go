// Package seed is the core's single source of seed data. Because the core knows
// every field's database type, its validation constraints (min/max, gte/lte,
// email, url, ...) and the relationships between entities, it produces fully
// typed, self-consistent seed records in the IR's SeedValue/SeedRecord shape.
// A bridge only serializes those records into its own literal syntax.
//
// Two producers share the exact same output shape:
//   - Generate  — deterministic MOCK seed, derived from types + constraints +
//     relations (no developer input needed; handy for demos and tests).
//   - Normalize — the developer's explicit `seed:` rows from domain.yaml,
//     coerced to the same typed shape (single serialization path for both).
package seed

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
)

// Options controls mock-seed generation.
type Options struct {
	// CountPerEntity is how many records to generate per entity (default 2).
	CountPerEntity int
	// Seed offsets the generated counters so different runs (e.g. a "user"
	// fixture vs a "test" fixture) don't collide on unique columns.
	Seed int
}

// Generate builds deterministic mock seed for every entity in the project.
func Generate(project *ir.IRProject, opts Options) *ir.SeedDataset {
	if opts.CountPerEntity <= 0 {
		opts.CountPerEntity = 2
	}

	ds := &ir.SeedDataset{}
	// Primary keys generated so far, keyed by entity name, so FK references can
	// point at real, already-generated parent rows (project.Entities is already
	// topologically sorted by the IR builder).
	generated := map[string][]interface{}{}
	counter := opts.Seed

	for i := range project.Entities {
		entity := &project.Entities[i]
		em := ir.SeedEntity{Name: entity.Name}

		for n := range opts.CountPerEntity {
			rec := ir.SeedRecord{}
			for _, field := range entity.Fields {
				// Sensitive fields (e.g. password) are never auto-mocked — the auth
				// flow owns them and tests create users with explicit credentials.
				// Feature fields (timestamps, version) are server-managed and get
				// their defaults on write, so they are omitted too.
				if field.IsSensitive() || field.IsFeatureField() {
					continue
				}
				v := valueFor(field, entity, project, generated, &counter, n)
				rec.Fields = append(rec.Fields, ir.SeedValue{
					Field:      field.Name,
					DBType:     field.DatabaseType,
					IsRelation: field.IsRelation,
					IsMany:     field.IsMany,
					Value:      v,
				})
			}
			em.Records = append(em.Records, rec)

			// Remember this row's primary key so children can reference it.
			if pk := primaryKey(entity); pk != nil {
				for _, sv := range rec.Fields {
					if sv.Field == pk.Name {
						generated[entity.Name] = append(generated[entity.Name], sv.Value)
					}
				}
			}
		}
		ds.Entities = append(ds.Entities, em)
	}
	return ds
}

// Normalize converts the developer's explicit `seed:` rows (raw maps) into the
// same typed SeedRecord shape Generate produces. Values are coerced to native
// Go types based on the field's IR database type, so a bridge serializes both
// explicit and generated seed through one code path.
func Normalize(project *ir.IRProject) *ir.SeedDataset {
	if project == nil {
		return &ir.SeedDataset{}
	}
	ds := &ir.SeedDataset{}
	for i := range project.Entities {
		entity := &project.Entities[i]
		if len(entity.Seed) == 0 {
			continue
		}
		em := ir.SeedEntity{Name: entity.Name}
		for _, raw := range entity.Seed {
			rec := ir.SeedRecord{}
			// Iterate fields in declaration order for deterministic output; only
			// include fields the developer actually provided a value for.
			for _, field := range entity.Fields {
				val, ok := raw[field.Name]
				if !ok {
					continue
				}
				rec.Fields = append(rec.Fields, ir.SeedValue{
					Field:      field.Name,
					DBType:     field.DatabaseType,
					IsRelation: field.IsRelation,
					IsMany:     field.IsMany,
					Value:      coerceValue(val, field.DatabaseType),
				})
			}
			em.Records = append(em.Records, rec)
		}
		ds.Entities = append(ds.Entities, em)
	}
	return ds
}

// primaryKey returns the entity's primary key field, or nil.
func primaryKey(entity *ir.IREntity) *ir.IRField {
	for i := range entity.Fields {
		if entity.Fields[i].IsPrimary {
			return &entity.Fields[i]
		}
	}
	return nil
}

// valueFor produces one field value for generated mock seed, honoring type,
// validations, defaults and relations.
func valueFor(field ir.IRField, entity *ir.IREntity, project *ir.IRProject, generated map[string][]interface{}, counter *int, record int) interface{} {
	// Default value wins when the model declares one (and it's not a function).
	if field.DefaultValue != "" && !field.DefaultIsFunc {
		return coerceValue(field.DefaultValue, field.DatabaseType)
	}

	if field.IsRelation {
		return relationValue(field, generated)
	}

	return scalarValue(field, project, counter, record)
}

// relationValue returns a reference to an existing parent row. For a [many]
// collection the value is nil (the child side owns the FK); for a single FK it
// returns the parent's primary-key value. When no parent was generated yet
// (self-reference or a dependency cycle) it falls back to nil.
func relationValue(field ir.IRField, generated map[string][]interface{}) interface{} {
	if field.IsMany {
		return nil
	}
	pks, ok := generated[field.RelationTarget]
	if !ok || len(pks) == 0 {
		return nil
	}
	return pks[0]
}

// scalarValue produces a scalar value from the field's type and constraints.
func scalarValue(field ir.IRField, project *ir.IRProject, counter *int, record int) interface{} {
	dbType := field.DatabaseType

	if field.IsArray() {
		inner := specmeta.ParseArrayInner(dbType)
		elem := innerValue(inner, field, project, counter, record)
		return []interface{}{elem}
	}

	switch dbType {
	case "uuid":
		*counter = *counter + 1
		return fmt.Sprintf("00000000-0000-0000-0000-%012d", *counter)
	case "string", "text":
		return stringValue(field, record)
	case "int", "bigint":
		return intValue(field, record)
	case "float", "decimal":
		return floatValue(field, record)
	case "boolean":
		return true
	case "date":
		return "2024-01-01"
	case "datetime":
		return "2024-01-01T00:00:00Z"
	case "json", "jsonb":
		return "{}"
	default:
		// Enum (or unknown) — use the first declared value when available.
		if values, ok := project.Enums[dbType]; ok && len(values) > 0 {
			return values[0]
		}
		return ""
	}
}

// innerValue produces a scalar for an array element type.
func innerValue(dbType string, field ir.IRField, project *ir.IRProject, counter *int, record int) interface{} {
	inner := ir.IRField{Name: field.Name, DatabaseType: dbType}
	if specmeta.IsPrimitive(strings.ToLower(dbType)) {
		return scalarValue(inner, project, counter, record)
	}
	if values, ok := project.Enums[dbType]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

// stringValue returns a valid mock string, honoring email/url validations and
// min/max length constraints (via the normalized ValidationRules contract).
func stringValue(field ir.IRField, record int) string {
	isEmail, isURL := false, false
	minLen, maxLen := "", ""
	for _, r := range field.ValidationRules() {
		switch r.Kind {
		case "email":
			isEmail = true
		case "url":
			isURL = true
		case "min_length":
			minLen = r.Value
		case "max_length":
			maxLen = r.Value
		}
	}

	var v string
	switch {
	case isEmail:
		v = fmt.Sprintf("mock%d@example.com", record+1)
	case isURL:
		v = fmt.Sprintf("https://example.com/resource%d", record+1)
	default:
		v = fmt.Sprintf("mock_%s_%d", strings.ToLower(field.Name), record+1)
	}

	if n, err := strconv.Atoi(minLen); err == nil && len(v) < n {
		v += strings.Repeat("x", n-len(v))
	}
	if n, err := strconv.Atoi(maxLen); err == nil && len(v) > n {
		v = v[:n]
	}
	return v
}

// intValue returns an int honoring the numeric min/max bounds (inclusive or
// exclusive) from the normalized ValidationRules contract.
func intValue(field ir.IRField, record int) int64 {
	v := int64(record + 1)
	for _, r := range field.ValidationRules() {
		n, err := strconv.ParseInt(r.Value, 10, 64)
		if err != nil {
			continue
		}
		switch {
		case r.Kind == "min_value" && r.Exclusive && v <= n:
			v = n + 1
		case r.Kind == "min_value" && v < n:
			v = n
		case r.Kind == "max_value" && r.Exclusive && v >= n:
			v = n - 1
		case r.Kind == "max_value" && v > n:
			v = n
		}
	}
	return v
}

// floatValue returns a float honoring the numeric min/max bounds (inclusive or
// exclusive) from the normalized ValidationRules contract.
func floatValue(field ir.IRField, record int) float64 {
	v := float64(record+1) + 0.5
	for _, r := range field.ValidationRules() {
		n, err := strconv.ParseFloat(r.Value, 64)
		if err != nil {
			continue
		}
		switch {
		case r.Kind == "min_value" && r.Exclusive && v <= n:
			v = n + 1
		case r.Kind == "min_value" && v < n:
			v = n
		case r.Kind == "max_value" && r.Exclusive && v >= n:
			v = n - 1
		case r.Kind == "max_value" && v > n:
			v = n
		}
	}
	return v
}

// coerceValue converts a declared seed/default value (from YAML: string, number,
// bool, list or map) to the field's native Go type.
func coerceValue(value interface{}, dbType string) interface{} {
	inner := specmeta.ParseArrayInner(dbType)

	// Arrays produce a slice of coerced elements.
	if specmeta.IsArrayType(dbType) {
		if list, ok := value.([]interface{}); ok {
			out := make([]interface{}, 0, len(list))
			for _, item := range list {
				out = append(out, coerceValue(item, inner))
			}
			return out
		}
		return []interface{}{}
	}

	switch {
	case specmeta.IsNumeric(inner):
		if inner == "int" || inner == "bigint" {
			switch v := value.(type) {
			case int:
				return int64(v)
			case int64:
				return v
			case float64:
				return int64(v)
			case string:
				if n, err := strconv.ParseInt(v, 10, 64); err == nil {
					return n
				}
			}
			return int64(0)
		}
		switch v := value.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
		return float64(0)
	case specmeta.IsBooleanType(inner):
		switch v := value.(type) {
		case bool:
			return v
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		}
		return false
	case inner == "uuid", inner == "string", inner == "text", inner == "date", inner == "datetime":
		return fmt.Sprintf("%v", value)
	case inner == "json", inner == "jsonb":
		return value // keep maps/slices/strings as-is for the bridge's JSON serializer
	default:
		// Enum value name (or unknown) — keep as a string.
		return fmt.Sprintf("%v", value)
	}
}
