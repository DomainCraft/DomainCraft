package ir

import (
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/specmeta"
)

// IsSensitive reports whether the field holds credentials that must never be
// serialized into a response DTO (currently: a field literally named "password").
// Sensitive fields are also excluded from the generic create/update projections
// — the auth flow handles them explicitly (hashing on write, never returning).
func (f IRField) IsSensitive() bool {
	return strings.EqualFold(f.Name, "password")
}

// IsFeatureField reports whether the field was auto-injected by an entity feature
// (audit, audit_log, soft_delete, optimistic_lock, ...). Feature fields are
// server-managed: they are returned in read projections but never accepted from
// a client in a create request.
func (f IRField) IsFeatureField() bool {
	_, ok := specmeta.FeatureFieldDefs[f.Name]
	return ok
}

// IsConcurrencyToken reports whether the field is the optimistic-lock version
// token. It is readable and accepted on update (so the client can send the
// version it last read) but is never copied from the request body into the
// entity — the server owns the increment.
func (f IRField) IsConcurrencyToken() bool {
	return f.IsFeatureField() && f.Name == "version"
}

// ReadFields returns the fields exposed in a read (response) DTO: every field
// except hidden and sensitive ones. Primary, readonly and feature fields are all
// readable. Feature (server-managed) fields are ordered last, after the
// developer-declared fields, so a response DTO reads naturally (identity and
// data first, timestamps and the version token at the end).
func (e IREntity) ReadFields() []IRField {
	out := make([]IRField, 0, len(e.Fields))
	var feature []IRField
	for _, f := range e.Fields {
		if f.IsHidden || f.IsSensitive() {
			continue
		}
		if f.IsFeatureField() {
			feature = append(feature, f)
			continue
		}
		out = append(out, f)
	}
	return append(out, feature...)
}

// CreateFields returns the fields accepted in a create (request) DTO: primary,
// readonly, feature and sensitive fields are excluded. Hidden fields are kept —
// they are client-settable, just never returned in responses.
func (e IREntity) CreateFields() []IRField {
	out := make([]IRField, 0, len(e.Fields))
	for _, f := range e.Fields {
		if f.IsPrimary || f.IsReadonly || f.IsFeatureField() || f.IsSensitive() {
			continue
		}
		out = append(out, f)
	}
	return out
}

// UpdateFields returns the fields accepted in a full update (request) DTO. It
// matches CreateFields but additionally exposes the optimistic-lock concurrency
// token (version), which the client echoes on update for safe concurrency.
func (e IREntity) UpdateFields() []IRField {
	out := make([]IRField, 0, len(e.Fields))
	for _, f := range e.Fields {
		if f.IsPrimary || f.IsReadonly || f.IsSensitive() {
			continue
		}
		if f.IsFeatureField() && !f.IsConcurrencyToken() {
			continue
		}
		out = append(out, f)
	}
	return out
}
