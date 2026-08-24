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
		ID:          "csharp-core",
		Name:        "C# Core",
		Description: "C# language core — enum wire, entity types, query schema, permission matrix (thin, certified once per language)",
		Language:    "C#",
		GitHub:      "DomainCraft/domaincraft-bridge-csharp-core",
	},
	{
		ID:          "csharp-domain",
		Name:        "C# Domain",
		Description: "Domain + Application layer for C# (services, Generation Gap, ports) — extends csharp-core",
		Language:    "C#",
		GitHub:      "DomainCraft/domaincraft-bridge-csharp-domain",
	},
	{
		ID:          "csharp-efcore",
		Name:        "C# EF Core",
		Description: "EF Core persistence layer for DomainCraft C# (extends csharp-domain)",
		Language:    "C#",
		GitHub:      "DomainCraft/domaincraft-bridge-csharp-efcore",
	},
	{
		ID:          "csharp-rest",
		Name:        "C# REST",
		Description: "ASP.NET Core REST transport for DomainCraft C# (extends csharp-efcore)",
		Language:    "C#",
		GitHub:      "DomainCraft/domaincraft-bridge-csharp-rest",
	},
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
		Name:        "TypeScript core",
		Description: "TypeScript language core — enum wire, entity types, query schema, permission matrix (thin, certified once per language)",
		Language:    "TypeScript",
		GitHub:      "DomainCraft/domaincraft-bridge-ts-core",
	},
	{
		ID:          "ts-client",
		Name:        "TypeScript client",
		Description: "TypeScript client — Zod validation, JWT client and CRUD API. Extends ts-core",
		Language:    "TypeScript",
		GitHub:      "DomainCraft/domaincraft-bridge-ts-client",
	},
	{
		ID:          "react-rest",
		Name:        "React + TypeScript client",
		Description: "React data layer (TanStack Query hooks, JWT auth) — extends ts-client (which extends ts-core)",
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
