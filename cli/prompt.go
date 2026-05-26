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

func (c *Cli) PromptWithDefaultBreadcrumb(label, defaultValue string, items []string, showGoBack bool, breadcrumb string) (string, bool) {
	return c.PromptSelectBreadcrumb(label, items, defaultValue, showGoBack, breadcrumb)
}

func (c *Cli) PromptSelectBreadcrumb(label string, items []string, defaultSelected string, showGoBack bool, breadcrumb string) (string, bool) {
	selectedItem, goBack, err := bubbleteaSelectBreadcrumb(label, items, defaultSelected, showGoBack, breadcrumb, promptExtraOpts...)
	if err != nil {
		return c.PromptSelect(label, items, defaultSelected, showGoBack)
	}
	if goBack {
		return "", true
	}
	if selectedItem == "" {
		exitFn(0)
	}
	return selectedItem, false
}

func (c *Cli) PromptSelectLoaded(loadingLabel, label string, defaultSelected string, showGoBack bool, load func() ([]string, error)) (string, bool, error) {
	return c.PromptSelectLoadedBreadcrumb(loadingLabel, label, defaultSelected, showGoBack, "", false, load)
}

func (c *Cli) PromptSelectLoadedBreadcrumb(loadingLabel, label string, defaultSelected string, showGoBack bool, breadcrumb string, autoSelectSingle bool, load func() ([]string, error)) (string, bool, error) {
	selectedItem, goBack, err := bubbleteaSelectLoaded(loadingLabel, label, defaultSelected, showGoBack, breadcrumb, autoSelectSingle, load, promptExtraOpts...)
	if err != nil || goBack {
		return selectedItem, goBack, err
	}
	if selectedItem == "" {
		exitFn(0)
	}
	return selectedItem, false, nil
}

// bubbleteaSelect runs the picker. Extra tea.ProgramOption values are appended
// to the default `tea.WithAltScreen` so tests can inject a scripted input
// stream / capture stdout via tea.WithInput / tea.WithOutput.
func bubbleteaSelect(label string, items []string, defaultSelected string, showGoBack bool, extraOpts ...tea.ProgramOption) (string, bool, error) {
	return bubbleteaSelectBreadcrumb(label, items, defaultSelected, showGoBack, "", extraOpts...)
}

func bubbleteaSelectBreadcrumb(label string, items []string, defaultSelected string, showGoBack bool, breadcrumb string, extraOpts ...tea.ProgramOption) (string, bool, error) {
	m := initialModelWithBreadcrumb(label, items, defaultSelected, showGoBack, breadcrumb)
	return runBubbleteaSelect(m, label, defaultSelected, extraOpts...)
}

func bubbleteaSelectLoaded(loadingLabel, label string, defaultSelected string, showGoBack bool, breadcrumb string, autoSelectSingle bool, load func() ([]string, error), extraOpts ...tea.ProgramOption) (string, bool, error) {
	m := initialModelWithBreadcrumb(label, nil, defaultSelected, showGoBack, breadcrumb)
	m.loading = true
	m.loadingMessage = loadingLabel
	m.autoSelectSingle = autoSelectSingle
	m.loadCmd = func() tea.Msg {
		items, err := load()
		return loadItemsMsg{items: items, err: err}
	}
	return runBubbleteaSelect(m, label, defaultSelected, extraOpts...)
}

func runBubbleteaSelect(m menuModel, label, defaultSelected string, extraOpts ...tea.ProgramOption) (string, bool, error) {
	if strings.Contains(label, "Theme") {
		m.originalTheme = CurrentTheme
		m.isThemeSelection = true
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

	// WithAltScreen routes all output to the terminal's alternate buffer so
	// the menu redraws don't accumulate in scroll-back. Without this the
	// previous menu's output stays in the user's terminal history and a
	// re-render (e.g. after a region-discovery spinner) looks like a
	// duplicated window.
	opts := append([]tea.ProgramOption{tea.WithAltScreen()}, extraOpts...)
	p := tea.NewProgram(ideModel{menu: m}, opts...)
	finalModel, err := p.Run()
	if err != nil {
		return "", false, err
	}
	im, ok := finalModel.(ideModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}
	mm := im.menu
	if mm.loadErr != nil {
		return "", mm.goBackTriggered, mm.loadErr
	}
	if mm.themeChanged {
		if mm.choice != "" {
			return mm.choice, mm.goBackTriggered, nil
		}
		if mm.previewTheme != nil {
			return mm.previewTheme.Name, mm.goBackTriggered, nil
		}
	}
	return mm.choice, mm.goBackTriggered, nil
}

func init() {
	// Move any pre-existing ~/.ecs_cli_* / ~/.exec-ecs-* files into the
	// config dir on first run after upgrade. Failures are non-fatal.
	migrateLegacyPaths()
	ApplySavedThemeSelection()
}
