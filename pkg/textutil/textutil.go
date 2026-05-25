// Package textutil provides shared string transformation functions
// used by parser, IR builder, and renderer.
package textutil

import (
	"strings"
	"unicode"
)

// Pluralize converts a singular noun to its plural form.
// Handles: -y->-ies, -zz->-zzes, -s/-ss/-sh/-x/-z/-ch->-es, -o->-es (after consonant), default -s.
func Pluralize(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "y") && len(name) > 1 {
		if !isVowel(rune(name[len(name)-2])) {
			return name[:len(name)-1] + "ies"
		}
	}
	if strings.HasSuffix(lower, "zz") {
		return name + "es"
	}
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "ss") ||
		strings.HasSuffix(lower, "sh") || strings.HasSuffix(lower, "ch") ||
		strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") {
		if strings.HasSuffix(lower, "z") && !strings.HasSuffix(lower, "zz") {
			return name + "zes"
		}
		return name + "es"
	}
	if strings.HasSuffix(lower, "o") && len(name) > 1 {
		if !isVowel(rune(name[len(name)-2])) {
			return name + "es"
		}
	}
	return name + "s"
}

// Singularize converts a plural noun to its singular form.
func Singularize(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "ies") && len(name) > 3 {
		return name[:len(name)-3] + "y"
	}
	// "Quizzes" -> "Quiz" (zzes -> z)
	if strings.HasSuffix(lower, "zzes") && len(name) > 4 {
		return name[:len(name)-3]
	}
	if strings.HasSuffix(lower, "ses") || strings.HasSuffix(lower, "xes") || strings.HasSuffix(lower, "zes") {
		return name[:len(name)-2]
	}
	if strings.HasSuffix(lower, "s") && len(name) > 1 {
		return name[:len(name)-1]
	}
	return name
}

// PascalCase converts a string to PascalCase.
// Handles snake_case, kebab-case, space-separated, and camelCase boundaries.
func PascalCase(value string) string {
	if value == "" {
		return ""
	}
	parts := SplitIdentifier(value)
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, "")
}

// CamelCase converts a string to camelCase.
func CamelCase(value string) string {
	value = PascalCase(value)
	if value == "" {
		return ""
	}
	return strings.ToLower(value[:1]) + value[1:]
}

// SplitIdentifier splits a string into parts at underscores, hyphens, spaces,
// and camelCase boundaries (e.g. "firstName" -> ["first", "Name"],
// "HTMLParser" -> ["HTML", "Parser"]).
func SplitIdentifier(value string) []string {
	// Normalize separators
	normalized := strings.ReplaceAll(value, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	// Insert separator at camelCase boundaries
	runes := []rune(normalized)
	var withSeps []rune
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			// camelCase boundary: lowercase -> Uppercase
			if unicode.IsUpper(r) && unicode.IsLower(prev) {
				withSeps = append(withSeps, '_')
			}
			// ACRONYM boundary: UPPERCASE -> Uppercase+lowercase (e.g., HTMLParser -> HTML_Parser)
			if i+1 < len(runes) && unicode.IsUpper(r) && unicode.IsUpper(prev) && unicode.IsLower(runes[i+1]) {
				withSeps = append(withSeps, '_')
			}
		}
		withSeps = append(withSeps, r)
	}

	parts := strings.Split(string(withSeps), "_")
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{value}
	}
	return result
}

func isVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}
