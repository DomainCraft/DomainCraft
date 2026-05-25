// Package testutil provides shared test helpers for DomainCraft packages.
package testutil

import (
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/lexer"
	"github.com/DomainCraft/DomainCraft/internal/parser"
)

// MustParsedField parses a field definition string and returns a *ParsedField.
// Fails the test if parsing fails.
func MustParsedField(t *testing.T, name, input string) *parser.ParsedField {
	t.Helper()
	fieldDef, err := lexer.ParseFieldString(input)
	if err != nil {
		t.Fatalf("ParseFieldString(%q) error = %v", input, err)
	}
	fieldDef.Name = name
	return &parser.ParsedField{FieldDefinition: fieldDef}
}
