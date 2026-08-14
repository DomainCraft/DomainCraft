package ir

import "github.com/DomainCraft/DomainCraft/internal/specmeta"

// IRPermissionPlan is the core-computed authorization plan for one operation
// (read/create/update/delete). The bridge no longer parses `*` wildcards or
// `@Owner` tokens in its templates — it receives the already-decomposed plan:
//   - IsPublic  — `*` is present: the endpoint is anonymous (no [Authorize]).
//   - HasOwner  — an `@Owner` token is present: the operation is owner-scoped.
//   - Roles     — the explicit (non-token) roles that may bypass the owner check
//     or are otherwise authorized via a policy.
//
// A role is either the public wildcard, an ownership token, or a declared role
// (validated against auth.roles elsewhere).
type IRPermissionPlan struct {
	Operation string   // read | create | update | delete
	IsPublic  bool     // wildcard "*" → anonymous access
	HasOwner  bool     // ownership token ("@...") → owner-scoped check
	Roles     []string // explicit roles (non-wildcard, non-token)
}

// HasOwnerToken reports whether ANY operation declares an ownership token
// ("@..."). Bridges use this to decide whether an entity needs owner-scoped
// row permissions (e.g. Appwrite row-level security) without parsing the
// role lists themselves.
func (p *IRPermissions) HasOwnerToken() bool {
	if p == nil {
		return false
	}
	for _, roles := range [][]string{p.Read, p.Create, p.Update, p.Delete} {
		for _, r := range roles {
			if specmeta.IsOwnershipToken(r) {
				return true
			}
		}
	}
	return false
}

// IsPublic reports whether the named operation is declared public (its role
// list contains the "*" wildcard). It returns false when no permissions block
// exists or the operation has no wildcard.
func (p *IRPermissions) IsPublic(operation string) bool {
	plan := p.Plan(operation)
	return plan != nil && plan.IsPublic
}

// Plan computes the authorization plan for the named operation. It returns nil
// when the entity declares no permissions for that operation (meaning the
// bridge falls back to its "no permissions defined" default).
func (p *IRPermissions) Plan(operation string) *IRPermissionPlan {
	if p == nil {
		return nil
	}
	var roles []string
	switch operation {
	case "read":
		roles = p.Read
	case "create":
		roles = p.Create
	case "update":
		roles = p.Update
	case "delete":
		roles = p.Delete
	default:
		return nil
	}

	plan := &IRPermissionPlan{Operation: operation}
	for _, r := range roles {
		switch {
		case r == "*":
			plan.IsPublic = true
		case len(r) > 0 && r[0] == '@':
			plan.HasOwner = true
		default:
			plan.Roles = append(plan.Roles, r)
		}
	}
	return plan
}

// PermissionPlan returns the authorization plan for the named operation on the
// current entity. It returns nil when the entity has no permissions block.
func (e IREntity) PermissionPlan(operation string) *IRPermissionPlan {
	if e.Permissions == nil {
		return nil
	}
	return e.Permissions.Plan(operation)
}
