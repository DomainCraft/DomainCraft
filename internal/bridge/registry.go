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
		ID:          "admin-refine",
		Name:        "Admin Panel (Refine)",
		Description: "React admin panel with Refine.dev + Ant Design + Vite",
		Language:    "TypeScript",
		GitHub:      "DomainCraft/domaincraft-bridge-admin",
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

// ByLanguage returns all bridges for a given language.
func (r *Registry) ByLanguage(lang string) []RegistryEntry {
	var result []RegistryEntry
	for _, e := range r.entries {
		if e.Language == lang {
			result = append(result, e)
		}
	}
	return result
}
