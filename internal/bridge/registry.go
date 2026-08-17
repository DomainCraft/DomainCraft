package bridge

// RegistryEntry describes a known bridge in the registry.
type RegistryEntry struct {
	ID          string // unique identifier, e.g. "csharp-restful"
	Name        string // display name, e.g. "C# REST API"
	Description string // one-line description
	Language    string // target language, e.g. "C#", "Java", "TypeScript"
	GitHub      string // GitHub "owner/repo", e.g. "DomainCraft/domaincraft-bridge-csharp"
}

// DefaultRegistry is the hardcoded list of known bridges.
// Extend this slice to add new bridges without changing any other code.
var DefaultRegistry = []RegistryEntry{
	{
		ID:          "csharp-restful",
		Name:        "C# REST API",
		Description: "ASP.NET Core + EF Core + PostgreSQL + JWT + Clean Architecture",
		Language:    "C#",
		GitHub:      "DomainCraft/domaincraft-bridge-csharp",
	},
	{
		ID:          "admin-alpine",
		Name:        "Admin Panel (Alpine.js)",
		Description: "Admin panel with Alpine.js + 0build UIkit",
		Language:    "HTML",
		GitHub:      "DomainCraft/domaincraft-bridge-admin",
	},
	{
		ID:          "appwrite",
		Name:        "Appwrite (TablesDB)",
		Description: "Appwrite schema: TablesDB + role teams + settings in one appwrite.config.json, deployed via the Appwrite CLI",
		Language:    "JSON",
		GitHub:      "DomainCraft/domaincraft-bridge-appwrite",
	},
	{
		ID:          "ts-core",
		Name:        "TypeScript core client",
		Description: "Framework-agnostic TypeScript data layer generated from the IR (typed query DSL, Zod validation, JWT client, permissions) — composed under framework adapters via `extends`",
		Language:    "TypeScript",
		GitHub:      "DomainCraft/domaincraft-bridge-ts",
	},
	{
		ID:          "react-rest",
		Name:        "React + TypeScript client",
		Description: "React data layer generated from the IR (typed query DSL, Zod validation, TanStack Query hooks, JWT auth)",
		Language:    "TypeScript",
		GitHub:      "DomainCraft/domaincraft-bridge-react",
	},
}

// Registry provides lookup methods over a set of bridge entries.
type Registry struct {
	entries []RegistryEntry
}

// NewRegistry creates a registry from the given entries.
func NewRegistry(entries []RegistryEntry) *Registry {
	return &Registry{entries: entries}
}

// Default returns a registry backed by the built-in bridge list.
func Default() *Registry {
	return NewRegistry(DefaultRegistry)
}

// All returns every registered bridge.
func (r *Registry) All() []RegistryEntry {
	return r.entries
}

// ByID finds a bridge by its unique ID. Returns nil if not found.
func (r *Registry) ByID(id string) *RegistryEntry {
	for i := range r.entries {
		if r.entries[i].ID == id {
			return &r.entries[i]
		}
	}
	return nil
}
