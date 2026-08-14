package ir

import "strings"

// IRValidationRule is a normalized, language-agnostic validation rule. The core
// derives these from the field's type, nullability and the declared validation
// modifiers, so a bridge never re-parses modifier names or re-derives which
// modifier is a length bound vs a numeric bound. It just maps each Kind (and its
// stable Code) to its own syntax (DataAnnotations, FluentValidation,
// class-validator, etc.).
type IRValidationRule struct {
	// Kind is the semantic rule kind: required, email, url, ipv4, regex,
	// min_length, max_length, min_value, max_value.
	Kind string
	// Code is the stable, machine-readable identifier for the rule, shared
	// across bridges (REQUIRED, EMAIL, URL, IPV4, REGEX, MIN_LENGTH, MAX_LENGTH,
	// MIN_VALUE, MAX_VALUE). Bridges that carry per-rule codes natively
	// (FluentValidation .WithErrorCode, class-validator) use this directly.
	Code string
	// Value is the rule parameter ("" for flags like required/email/url/ipv4):
	//   regex      -> the pattern
	//   min_length -> the minimum string length
	//   max_length -> the maximum string length
	//   min_value  -> the lower numeric bound (inclusive unless Exclusive)
	//   max_value  -> the upper numeric bound (inclusive unless Exclusive)
	Value string
	// Exclusive marks a non-inclusive numeric bound (the `gt`/`lt` modifiers).
	// min_value/max_value are inclusive by default; Exclusive flips the bound so a
	// consumer (seed generator, FluentValidation, class-validator, ...) can express
	// a strict greater-than / less-than without re-deriving it from the modifier name.
	Exclusive bool
	// Message is a stable, human-readable default message with the rule's value
	// already interpolated. Bridges use it verbatim (e.g. DataAnnotations
	// ErrorMessage) so the same rule renders the same text on every backend.
	Message string
}

// validationRuleMeta returns the stable wire code and default message for a
// normalized rule kind. The value is interpolated into length/numeric messages.
func validationRuleMeta(kind, value string, exclusive bool) (code, message string) {
	switch kind {
	case "required":
		return "REQUIRED", "is required"
	case "email":
		return "EMAIL", "must be a valid email address"
	case "url":
		return "URL", "must be a valid URL"
	case "ipv4":
		return "IPV4", "must be a valid IPv4 address"
	case "regex":
		return "REGEX", "must match the required format"
	case "min_length":
		return "MIN_LENGTH", "must be at least " + value + " characters"
	case "max_length":
		return "MAX_LENGTH", "must be at most " + value + " characters"
	case "min_value":
		if exclusive {
			return "MIN_VALUE", "must be greater than " + value
		}
		return "MIN_VALUE", "must be greater than or equal to " + value
	case "max_value":
		if exclusive {
			return "MAX_VALUE", "must be less than " + value
		}
		return "MAX_VALUE", "must be less than or equal to " + value
	}
	return strings.ToUpper(kind), ""
}

// newRule builds a normalized rule with its stable code and default message.
func newRule(kind, value string, exclusive bool) IRValidationRule {
	code, message := validationRuleMeta(kind, value, exclusive)
	return IRValidationRule{Kind: kind, Code: code, Value: value, Exclusive: exclusive, Message: message}
}

// ValidationRules returns the field's validations as a normalized, ordered list
// of rules. The order is stable and semantic: required first, then format
// validators, then length bounds, then numeric bounds.
//
// Mapping from the raw modifiers:
//
//	required (from the field not being nullable / the "required" modifier)
//	email / url / ipv4 / regex   (string validators, unchanged)
//	min / max                    -> min_length / max_length on string types,
//	                               min_value / max_value on numeric types
//	gte / lte                   -> inclusive min_value / max_value
//	gt / lt                     -> min_value / max_value with Exclusive=true
func (f IRField) ValidationRules() []IRValidationRule {
	rules := make([]IRValidationRule, 0, len(f.Validations)+1)

	if !f.IsNullable && !f.IsPrimary {
		// Non-nullable fields are required by construction. (A nullable field
		// may still declare an explicit `required` modifier, handled below.)
		rules = append(rules, newRule("required", "", false))
	}

	for _, v := range f.Validations {
		name := strings.ToLower(v.Name)
		switch name {
		case "required":
			// Deduplicate: a non-nullable field already carries required.
			if !f.IsNullable && !f.IsPrimary {
				continue
			}
			rules = append(rules, newRule("required", "", false))
		case "email", "url", "ipv4":
			rules = append(rules, newRule(name, "", false))
		case "regex":
			rules = append(rules, newRule("regex", v.Value, false))
		case "min", "max":
			if f.IsNumeric() {
				rules = append(rules, newRule(name+"_value", v.Value, false))
			} else {
				rules = append(rules, newRule(name+"_length", v.Value, false))
			}
		case "gte":
			rules = append(rules, newRule("min_value", v.Value, false))
		case "gt":
			rules = append(rules, newRule("min_value", v.Value, true))
		case "lte":
			rules = append(rules, newRule("max_value", v.Value, false))
		case "lt":
			rules = append(rules, newRule("max_value", v.Value, true))
		}
	}

	return rules
}
