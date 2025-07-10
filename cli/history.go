package cli

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const clearHistoryOption = "🗑️  Clear History"

var historyMenuOpen bool

func BubbleteaHistorySelect(label string, items []string) (string, error) {
	displayItems := make([]string, len(items))
	for i, cmd := range items {
		var formatted strings.Builder
		count := 0
		for _, r := range cmd {
			formatted.WriteRune(r)
			count++
			if count >= 80 && r == ' ' {
				formatted.WriteRune('\n')
				count = 0
			}
		}
		displayItems[i] = formatted.String()
	}
	allItems := append(displayItems, clearHistoryOption)
	m := initialModel(label, allItems, "", false)
	m.historyMode = true
	for {
		p := tea.NewProgram(ideModel{menu: m}, tea.WithMouseAllMotion())
		finalModel, err := p.Run()
		if err != nil {
			historyMenuOpen = false
			return "", err
		}
		im, ok := finalModel.(ideModel)
		if !ok {
			historyMenuOpen = false
			return "", fmt.Errorf("unexpected model type")
		}
		mm := im.menu
		if mm.choice == clearHistoryOption {
			historyFile := os.Getenv("HOME") + "/.ecs_cli_history"
			_ = os.Remove(historyFile)
			fmt.Println("History cleared.")
			allItems = []string{clearHistoryOption}
			m = initialModel(label, allItems, "", false)
			m.historyMode = true
			continue
		}
		// Mouse click support: if a mouse click occurred, print the command for copying
		if mm.choice != "" && mm.choice != clearHistoryOption && mm.quitting && mmWasMouseClick(mm) {
			// Map the selected display string back to the original command
			for i, disp := range displayItems {
				if mm.choice == disp {
					fmt.Println("\nCopied to clipboard (select and copy):\n" + items[i])
					historyMenuOpen = false
					return "", nil
				}
			}
		}
		for i, disp := range displayItems {
			if mm.choice == disp {
				historyMenuOpen = false
				return items[i], nil
			}
		}
		historyMenuOpen = false
		return mm.choice, nil
	}
}

// Helper to detect if the last quit was due to a mouse click (to be set in menuModel)
func mmWasMouseClick(m menuModel) bool {
	// This is a placeholder. You will need to set a flag in menuModel.Update when a mouse click event occurs.
	return m.mouseClicked
}
