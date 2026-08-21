package ir

import "github.com/DomainCraft/DomainCraft/pkg/textutil"

// IREnumValue is one enum value plus its stable, language-agnostic wire form.
// The wire value (snake_case) is the contract representation of the value: it is
// what the API serializes, what filters match, and what a bridge stores. Bridges
// print .WireValue instead of re-deriving snake_case themselves — the template
// `snakecase` diverges from textutil.ToDatabaseColumnName on acronyms (e.g.
// `IPv4`), so the wire form must be computed once, in the core.
type IREnumValue struct {
	// Name is the raw value as declared in domain.yaml (e.g. "IN_PROGRESS").
	Name string
	// WireValue is the canonical snake_case contract value (e.g. "in_progress").
	WireValue string
	// Ordinal is the zero-based declaration order, i.e. the enum's numeric value.
	Ordinal int
}

// EnumValues returns the enum's values in declaration order, each carrying its
// canonical wire value. Bridges iterate this instead of reading Project.Enums[name]
// and re-deriving snake_case for the wire representation.
func (p *IRProject) EnumValues(name string) []IREnumValue {
	if p == nil || p.Enums == nil {
		return nil
	}
	values := p.Enums[name]
	if values == nil {
		return nil
	}
	out := make([]IREnumValue, 0, len(values))
	for i, v := range values {
		out = append(out, IREnumValue{
			Name:      v,
			WireValue: textutil.ToDatabaseColumnName(v),
			Ordinal:   i,
		})
	}
	return out
}
