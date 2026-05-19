package ir

import "strings"

// HasValidation reports whether the field has the named validation.
func (f IRField) HasValidation(name string) bool {
	for _, validation := range f.Validations {
		if strings.EqualFold(validation.Name, name) {
			return true
		}
	}
	return false
}

// ValidationValue returns the first validation value for the given name.
func (f IRField) ValidationValue(name string) string {
	for _, validation := range f.Validations {
		if strings.EqualFold(validation.Name, name) {
			return validation.Value
		}
	}
	return ""
}
