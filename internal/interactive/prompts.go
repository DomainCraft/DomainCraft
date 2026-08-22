package interactive

import (
	"fmt"

	"github.com/DomainCraft/DomainCraft/internal/bridge"
)

// SelectBridge shows a bridge selection menu. ↑/↓ or 1-9 moves, Enter picks,
// Esc aborts.
func SelectBridge(registry *bridge.Registry) (*bridge.RegistryEntry, error) {
	entries := registry.All()
	if len(entries) == 0 {
		return nil, fmt.Errorf("no bridges available")
	}

	cursor := 0
	build := func() []string {
		lines := []string{lineField(ansiBold + "Select a bridge (target language/framework for code generation)")}
		for i, e := range entries {
			s := fmt.Sprintf("  %2d) %s (%s) — %s", i+1, e.Name, e.Language, e.Description)
			if i == cursor {
				lines = append(lines, lineFocus(s))
			} else {
				lines = append(lines, lineField(s))
			}
		}
		lines = append(lines, lineField(ansiDim+"  ↑/↓ or 1-9 to move, Enter to confirm"))
		return lines
	}

	f := &field{}
	f.draw(build())
	var selected *bridge.RegistryEntry
	err := rawMode(func() error {
		for {
			k, err := readKey()
			if err != nil {
				return err
			}
			switch k.kind {
			case keyUp:
				if cursor > 0 {
					cursor--
					f.draw(build())
				}
			case keyDown:
				if cursor < len(entries)-1 {
					cursor++
					f.draw(build())
				}
			case keyEnter:
				selected = &entries[cursor]
				return nil
			case keyEsc:
				return errAborted
			case keyRune:
				if k.r >= '1' && k.r <= '9' {
					if n := int(k.r - '1'); n < len(entries) && n != cursor {
						cursor = n
						f.draw(build())
					}
				}
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return selected, nil
}

// PromptGenerateAdmin asks whether to generate an admin panel alongside the backend.
func PromptGenerateAdmin() (bool, error) {
	return confirm(
		"Generate admin panel?",
		"Create an Alpine.js admin panel (single-page HTML) for managing your data",
	)
}

// PromptBridgeUpdate asks whether to update a cached bridge to the newer
// upstream version. Returns true to download the update, false to keep the
// cached copy.
func PromptBridgeUpdate(entry bridge.RegistryEntry, update *bridge.Update) (bool, error) {
	description := "A newer version is available for the cached bridge."
	if update.LocalVersion != "" {
		description = fmt.Sprintf("A newer version is available for the cached bridge (v%s). Update the cached copy?", update.LocalVersion)
	}
	return confirm(fmt.Sprintf("Update bridge %q?", entry.ID), description)
}

// confirm asks a yes/no question with two buttons. ←/→ switches the green
// focus, Enter confirms, Esc aborts. Letter shortcuts (y/n) are intentionally
// absent — they break on non-Latin keyboard layouts.
func confirm(title, description string) (bool, error) {
	cursor := 1 // default: No
	build := func() []string {
		lines := []string{lineField(ansiBold + title)}
		if description != "" {
			lines = append(lines, lineField("  "+description))
		}
		buttons := button("Yes", cursor == 0) + " " + button("No", cursor == 1)
		lines = append(lines, lineField("  "+buttons+"   "+ansiDim+"←/→ to switch, Enter to confirm"))
		return lines
	}

	f := &field{}
	f.draw(build())
	var result bool
	err := rawMode(func() error {
		for {
			k, err := readKey()
			if err != nil {
				return err
			}
			switch {
			case k.kind == keyEnter:
				result = cursor == 0
				return nil
			case k.kind == keyEsc:
				return errAborted
			case k.kind == keyLeft:
				if cursor != 0 {
					cursor = 0
					f.draw(build())
				}
			case k.kind == keyRight:
				if cursor != 1 {
					cursor = 1
					f.draw(build())
				}
			}
		}
	})
	return result, err
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
	return multiSelect(
		fmt.Sprintf("Entity %q was removed from domain.yaml", entityName),
		"The following files are no longer generated. Select files to delete:",
		files,
	)
}

// PromptRenameFiles asks which files of a renamed entity should be renamed.
// Returns the selected relative paths (empty = keep everything).
func PromptRenameFiles(oldName, newName string, files []FileChoice) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	return multiSelect(
		fmt.Sprintf("Entity %q was renamed to %q", oldName, newName),
		"Select files to rename (old name -> new name in the path):",
		files,
	)
}

// multiSelect shows a numbered list with every item preselected. ↑/↓ moves
// the green focus, Space toggles the focused item, 1-9 toggle directly,
// Enter confirms, Esc aborts. Returns selected paths in original order.
func multiSelect(title, description string, files []FileChoice) ([]string, error) {
	selected := make([]bool, len(files))
	for i := range files {
		selected[i] = true
	}
	cursor := 0

	build := func() []string {
		lines := []string{lineField(ansiBold + title)}
		if description != "" {
			lines = append(lines, lineField("  "+description))
		}
		for i, f := range files {
			marker := " "
			if selected[i] {
				if i == cursor {
					marker = "\u2713"
				} else {
					marker = fgGreen("\u2713")
				}
			}
			s := fmt.Sprintf("  [%s] %2d) %s", marker, i+1, f.Label)
			if i == cursor {
				lines = append(lines, lineFocus(s))
			} else {
				lines = append(lines, lineField(s))
			}
		}
		lines = append(lines, lineField(ansiDim+"  ↑/↓ to move, Space or 1-9 to toggle, Enter to confirm"))
		return lines
	}

	f := &field{}
	f.draw(build())
	err := rawMode(func() error {
		for {
			k, err := readKey()
			if err != nil {
				return err
			}
			switch k.kind {
			case keyUp:
				if cursor > 0 {
					cursor--
					f.draw(build())
				}
			case keyDown:
				if cursor < len(files)-1 {
					cursor++
					f.draw(build())
				}
			case keySpace:
				selected[cursor] = !selected[cursor]
				f.draw(build())
			case keyEnter:
				return nil
			case keyEsc:
				return errAborted
			case keyRune:
				if k.r >= '1' && k.r <= '9' {
					if n := int(k.r - '1'); n < len(files) {
						selected[n] = !selected[n]
						f.draw(build())
					}
				}
			}
		}
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(files))
	for i, f := range files {
		if selected[i] {
			result = append(result, f.Path)
		}
	}
	return result, nil
}