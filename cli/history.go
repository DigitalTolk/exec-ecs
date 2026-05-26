package cli

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	clearHistoryOption    = "🗑️  Clear History"
	historyDisplayMaxRune = 100
	historyDisplayEllipsis = "…"
)

var historyMenuOpen bool

// historyExtraOpts is appended to the default tea.Program options. Tests
// override this to feed scripted input / capture rendered output.
var historyExtraOpts []tea.ProgramOption

// truncateForDisplay collapses internal whitespace and shortens a command to a
// single line that fits within historyDisplayMaxRune runes. We deliberately
// avoid embedding `\n` in the displayed string — the previous approach broke
// the equality check used to map the chosen display back to the original
// command, and made nested bubbletea pagination layouts unpredictable.
func truncateForDisplay(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= historyDisplayMaxRune {
		return s
	}
	runes := []rune(s)
	cutoff := historyDisplayMaxRune - utf8.RuneCountInString(historyDisplayEllipsis)
	if cutoff < 1 {
		cutoff = 1
	}
	return string(runes[:cutoff]) + historyDisplayEllipsis
}

// BubbleteaHistorySelect shows the last-N history entries and returns the
// command the user picked (in its original, untruncated form). The display
// strings are kept distinct from the original commands via displayToOriginal
// so we never have to round-trip through a possibly-mangled label.
func BubbleteaHistorySelect(label string, items []string) (string, error) {
	displayItems := make([]string, 0, len(items))
	displayToOriginal := make(map[string]string, len(items))
	for _, cmd := range items {
		d := truncateForDisplay(cmd)
		// Ensure each display string is unique so the map lookup is exact.
		// Duplicate raw commands are dropped upstream by GetLastUniqueHistory.
		for _, existing := range displayItems {
			if existing == d {
				d = d + " "
				break
			}
		}
		displayItems = append(displayItems, d)
		displayToOriginal[d] = cmd
	}
	allItems := append(displayItems, clearHistoryOption)
	m := initialModel(label, allItems, "", false)
	m.historyMode = true

	for {
		opts := append([]tea.ProgramOption{tea.WithMouseAllMotion(), tea.WithAltScreen()}, historyExtraOpts...)
		p := tea.NewProgram(ideModel{menu: m}, opts...)
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
			_ = os.Remove(historyFile)
			fmt.Println("History cleared.")
			displayItems = nil
			displayToOriginal = map[string]string{}
			allItems = []string{clearHistoryOption}
			m = initialModel(label, allItems, "", false)
			m.historyMode = true
			continue
		}
		historyMenuOpen = false
		// Map the selected display string back to the original command.
		if original, ok := displayToOriginal[mm.choice]; ok {
			if mm.quitting && mmWasMouseClick(mm) {
				fmt.Println("\nCopied to clipboard (select and copy):\n" + original)
				return "", nil
			}
			return original, nil
		}
		return mm.choice, nil
	}
}

// Helper to detect if the last quit was due to a mouse click (to be set in menuModel)
func mmWasMouseClick(m menuModel) bool {
	// This is a placeholder. You will need to set a flag in menuModel.Update when a mouse click event occurs.
	return m.mouseClicked
}
