// NOTE: Most menu, AWS, and history logic has been moved to menu.go, aws_select.go, and history.go for clarity.
package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (c *Cli) PromptWithDefault(label, defaultValue string, items []string, showGoBack bool) (string, bool) {
	allItems := items
	return c.PromptSelect(label, allItems, defaultValue, showGoBack)
}

// Update bubbleteaSelect to use ideModel
func bubbleteaSelect(label string, items []string, defaultSelected string, showGoBack bool) (string, bool, error) {
	for {
		m := initialModel(label, items, defaultSelected, showGoBack)

		// Set up theme preview if this is a theme selection
		if strings.Contains(label, "Theme") {
			m.originalTheme = CurrentTheme
			m.isThemeSelection = true
			// Initialize preview with the default selected theme
			if defaultSelected != "" {
				for _, theme := range allThemes {
					if theme.Name == defaultSelected {
						m.previewTheme = theme
						CurrentTheme = theme
						break
					}
				}
			}
		}

		p := tea.NewProgram(ideModel{menu: m})
		finalModel, err := p.Run()
		if err != nil {
			return "", false, err
		}
		im, ok := finalModel.(ideModel)
		if !ok {
			return "", false, fmt.Errorf("unexpected model type")
		}
		mm := im.menu
		if mm.themeChanged {
			// Theme was changed and applied, return the selected theme name
			// The choice should contain the theme name that was selected
			if mm.choice != "" {
				return mm.choice, mm.goBackTriggered, nil
			}
			// If choice is empty but themeChanged is true, return the preview theme name
			if mm.previewTheme != nil {
				return mm.previewTheme.Name, mm.goBackTriggered, nil
			}
		}
		return mm.choice, mm.goBackTriggered, nil
	}
}

func init() {
	if name := LoadThemeSelection(); name != "" {
		SetThemeByName(name)
	}
}
