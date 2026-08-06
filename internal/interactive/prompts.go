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

// PromptGenerateAdmin asks whether to generate an admin panel alongside the backend.
func PromptGenerateAdmin() (bool, error) {
	var generate bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Generate admin panel?").
				Description("Create an Alpine.js admin panel (single-page HTML) for managing your data").
				Value(&generate),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return generate, nil
}

// PromptBridgeUpdate asks whether to update a cached bridge to the newer
// upstream version. Returns true to download the update, false to keep the
// cached copy.
func PromptBridgeUpdate(entry bridge.RegistryEntry, update *bridge.Update) (bool, error) {
	title := fmt.Sprintf("Update bridge %q?", entry.ID)

	description := "A newer version is available for the cached bridge."
	if update.LocalVersion != "" {
		description = fmt.Sprintf("A newer version is available for the cached bridge (v%s). Update the cached copy?", update.LocalVersion)
	}

	var apply bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&apply),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return apply, nil
}

// FileChoice is a selectable file with a display label.
type FileChoice struct {
	Path  string // value returned when selected
	Label string // display text (may annotate e.g. "[custom]")
}

// PromptDeleteFiles asks which orphaned files of a deleted entity should be
// removed. Returns the selected relative paths (empty = keep everything).
func PromptDeleteFiles(entityName string, files []FileChoice) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	options := make([]huh.Option[string], len(files))
	preselected := make([]string, 0, len(files))
	for i, f := range files {
		options[i] = huh.NewOption(f.Label, f.Path)
		preselected = append(preselected, f.Path)
	}
	selected := preselected
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Entity %q was removed from domain.yaml", entityName)).
				Description("The following files are no longer generated. Select files to delete:").
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

// PromptRenameFiles asks which files of a renamed entity should be renamed.
// Returns the selected relative paths (empty = keep everything).
func PromptRenameFiles(oldName, newName string, files []FileChoice) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	options := make([]huh.Option[string], len(files))
	preselected := make([]string, 0, len(files))
	for i, f := range files {
		options[i] = huh.NewOption(f.Label, f.Path)
		preselected = append(preselected, f.Path)
	}
	selected := preselected
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Entity %q was renamed to %q", oldName, newName)).
				Description("Select files to rename (old name -> new name in the path):").
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}
