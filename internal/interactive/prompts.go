package interactive

import (
	"fmt"
	"os"

	"github.com/DomainCraft/DomainCraft/internal/bridge"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

// IsTerminal returns true if stdin is a terminal (interactive mode).
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// SelectBridge shows an interactive bridge selection menu.
// Returns the selected registry entry.
func SelectBridge(registry *bridge.Registry) (*bridge.RegistryEntry, error) {
	entries := registry.All()
	if len(entries) == 0 {
		return nil, fmt.Errorf("no bridges available")
	}

	options := make([]huh.Option[*bridge.RegistryEntry], len(entries))
	for i := range entries {
		e := &entries[i]
		options[i] = huh.NewOption(
			fmt.Sprintf("%s (%s) — %s", e.Name, e.Language, e.Description),
			e,
		)
	}

	var selected *bridge.RegistryEntry
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[*bridge.RegistryEntry]().
				Title("Select a bridge").
				Description("Choose the target language/framework for code generation").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

// PromptProjectName asks for the project name interactively.
func PromptProjectName() (string, error) {
	var name string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("The name of your project (e.g. My App)").
				Placeholder("My App").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("project name is required")
					}
					return nil
				}),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return name, nil
}

// PromptDatabase asks for the database type interactively.
func PromptDatabase() (string, error) {
	var db string
	options := []huh.Option[string]{
		huh.NewOption("PostgreSQL (recommended)", "postgresql"),
		huh.NewOption("MySQL", "mysql"),
		huh.NewOption("SQLite", "sqlite"),
		huh.NewOption("MS SQL Server", "mssql"),
		huh.NewOption("MongoDB", "mongodb"),
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Database").
				Description("Choose the database engine").
				Options(options...).
				Value(&db),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return db, nil
}

// PromptAuth asks for the authentication type interactively.
func PromptAuth() (string, error) {
	var auth string
	options := []huh.Option[string]{
		huh.NewOption("JWT (recommended)", "jwt"),
		huh.NewOption("None", "none"),
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Authentication").
				Description("Choose the authentication method").
				Options(options...).
				Value(&auth),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return auth, nil
}

// PromptAPIStyle asks for the API style interactively.
func PromptAPIStyle() (string, error) {
	var style string
	options := []huh.Option[string]{
		huh.NewOption("REST (recommended)", "rest"),
		huh.NewOption("GraphQL", "graphql"),
		huh.NewOption("gRPC", "grpc"),
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("API style").
				Description("Choose the API paradigm").
				Options(options...).
				Value(&style),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return style, nil
}
