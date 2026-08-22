package ir

import (
	"cmp"
	"slices"

	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// AllIndexes returns the entity's complete, normalized index list: the
// declared indexes plus one implicit single-column unique index per field
// marked `unique: true` that is not already covered by a declared index.
//
// This is the single source of truth for a bridge's index rendering — a bridge
// no longer needs to remember that per-field uniqueness also produces an index.
// The result is sorted by name for deterministic output.
func (e IREntity) AllIndexes() []IRIndex {
	result := make([]IRIndex, 0, len(e.Indexes)+1)

	// Copy declared indexes (normalizing empty Sort to asc-per-field).
	for _, idx := range e.Indexes {
		cp := idx
		cp.Fields = append([]string(nil), idx.Fields...)
		cp.Sort = normalizeSort(idx.Sort, len(idx.Fields))
		result = append(result, cp)
	}

	// Covered single-field unique targets, so an implicit index isn't duplicated.
	covered := map[string]bool{}
	for _, idx := range result {
		if idx.Unique && len(idx.Fields) == 1 {
			covered[idx.Fields[0]] = true
		}
	}

	// Implicit unique indexes for fields declared `unique: true`.
	for _, f := range e.Fields {
		if !f.IsUnique || f.IsRelation {
			continue
		}
		if covered[f.Name] {
			continue
		}
		result = append(result, IRIndex{
			Name:   "idx_" + textutil.ToDatabaseColumnName(e.NamePlural) + "_" + textutil.ToDatabaseColumnName(f.Name),
			Fields: []string{f.Name},
			Sort:   []string{"asc"},
			Unique: true,
		})
	}

	slices.SortFunc(result, func(a, b IRIndex) int { return cmp.Compare(a.Name, b.Name) })
	return result
}

// DatabaseName returns the canonical database index name: the "IX_" prefix plus
// the snake_case index name. Bridges print this instead of re-deriving
// `IX_{{ .Name | snakecase }}`, which double-prefixes and re-snake_cases a name
// the core already computed (the template `snakecase` diverges from the core's
// column naming on acronyms).
func (i IRIndex) DatabaseName() string {
	return "IX_" + i.Name
}

// normalizeSort pads the sort directions to len(fields), defaulting to "asc",
// so bridges can index-safely pair each field with its direction.
func normalizeSort(sort []string, n int) []string {
	out := make([]string, n)
	for i := range n {
		if i < len(sort) && sort[i] != "" {
			out[i] = sort[i]
		} else {
			out[i] = "asc"
		}
	}
	return out
}
