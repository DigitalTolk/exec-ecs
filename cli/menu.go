package cli

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultItemsPerPage = 10
)

const matrixRainWidth = 40
const matrixRainHeight = 16
const matrixChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@#$%&"

type menuModel struct {
	items           []string
	filteredItems   []string
	cursor          int
	choice          string
	label           string
	quitting        bool
	viewport        viewport.Model
	textInput       textinput.Model
	filterMode      bool
	page            int
	historyMode     bool
	goBackTriggered bool

	// Animation state for Matrix theme
	frame      int
	matrixRain []string

	// Theme change flag
	themeChanged bool

	// Mouse click flag
	mouseClicked bool

	// Terminal size and scaling
	width        int
	height       int
	itemsPerPage int
	scaleFactor  float64

	// Theme preview
	previewTheme     *Theme
	originalTheme    *Theme
	isThemeSelection bool
}

type tickMsg time.Time

type ideModel struct {
	menu   menuModel
	width  int
	height int
}

func initialModel(label string, items []string, defaultSelected string, showGoBack bool) menuModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 50
	ti.Width = 30

	allItems := items
	selectedIdx := 0
	if defaultSelected != "" {
		for i, item := range allItems {
			if item == defaultSelected {
				selectedIdx = i
				break
			}
		}
	}

	// Default values, will be updated when window size is received
	itemsPerPage := defaultItemsPerPage
	page := selectedIdx / itemsPerPage
	cursor := selectedIdx % itemsPerPage

	m := menuModel{
		items:         allItems,
		filteredItems: allItems,
		label:         label,
		viewport:      viewport.New(80, itemsPerPage+2),
		textInput:     ti,
		cursor:        cursor,
		page:          page,
		itemsPerPage:  itemsPerPage,
		scaleFactor:   1.0,
	}
	if CurrentTheme.Name == "Matrix" {
		m.matrixRain = make([]string, matrixRainHeight)
		for i := range m.matrixRain {
			m.matrixRain[i] = randomMatrixLine()
		}
	}
	return m
}

func randomMatrixLine() string {
	line := make([]byte, matrixRainWidth)
	for i := range line {
		if rand.Float32() < 0.2 {
			line[i] = matrixChars[rand.Intn(len(matrixChars))]
		} else {
			line[i] = ' '
		}
	}
	return string(line)
}

func (m menuModel) Init() tea.Cmd {
	if CurrentTheme.Name == "Matrix" {
		return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	return textinput.Blink
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// While the filter input is focused we must let `q`, `/`, etc. flow
		// into the textinput as ordinary characters. Only ctrl+c, esc and
		// enter retain their menu semantics inside the filter.
		if m.filterMode {
			switch key {
			case "ctrl+c":
				if m.isThemeSelection && m.originalTheme != nil {
					CurrentTheme = m.originalTheme
				}
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.filterMode = false
				m.textInput.Blur()
				return m, nil
			case "enter":
				m.filterMode = false
				m.textInput.Blur()
				return m, nil
			}
			m.textInput, cmd = m.textInput.Update(msg)
			m.filterItems(m.textInput.Value())
			return m, cmd
		}

		switch key {
		case "ctrl+c", "q":
			// Restore previewed-but-not-applied theme on hard quit so the
			// user doesn't end up stuck with a temp theme they were only
			// auditioning.
			if m.isThemeSelection && m.originalTheme != nil {
				CurrentTheme = m.originalTheme
			}
			m.quitting = true
			return m, tea.Quit

		case "/":
			m.filterMode = true
			m.textInput.Focus()
			return m, textinput.Blink

		case "esc":
			// If this is theme selection and user hits esc, restore original theme
			if m.isThemeSelection && m.originalTheme != nil {
				CurrentTheme = m.originalTheme
			}
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if m.filterMode {
				m.filterMode = false
				m.textInput.Blur()
				return m, nil
			}
			if len(m.filteredItems) > 0 {
				choice := m.filteredItems[m.cursor+m.page*m.itemsPerPage]
				m.choice = choice
				// If this is theme selection and user hits enter, apply the previewed theme
				if m.isThemeSelection && m.previewTheme != nil {
					CurrentTheme = m.previewTheme
					SaveThemeSelection(m.previewTheme.Name)
					m.themeChanged = true
				}
				m.quitting = true
				return m, tea.Quit
			}

		case "up":
			if !m.filterMode {
				if m.cursor > 0 {
					m.cursor--
					m.updateThemePreview()
				} else if m.page > 0 {
					m.page--
					m.cursor = m.itemsPerPage - 1
					m.updateThemePreview()
				}
			}

		case "down":
			if !m.filterMode {
				maxIndex := min(m.itemsPerPage, len(m.filteredItems)-m.page*m.itemsPerPage)
				if m.cursor < maxIndex-1 {
					m.cursor++
					m.updateThemePreview()
				} else if (m.page+1)*m.itemsPerPage < len(m.filteredItems) {
					m.page++
					m.cursor = 0
					m.updateThemePreview()
				}
			}

		case "pgup":
			if !m.filterMode && m.page > 0 {
				m.page--
				m.cursor = 0
			}

		case "pgdown":
			if !m.filterMode && (m.page+1)*m.itemsPerPage < len(m.filteredItems) {
				m.page++
				m.cursor = 0
			}

		case "ctrl+h":
			if m.historyMode || historyMenuOpen {
				return m, nil
			}
			historyMenuOpen = true
			history := GetLastUniqueHistory(5)
			if len(history) == 0 {
				historyMenuOpen = false
				return m, nil
			}
			selected, err := BubbleteaHistorySelect("Command History (last 5 unique)", history)
			historyMenuOpen = false
			if err == nil && selected != "" {
				fmt.Println("\nExecuting:", selected)
				cmd := exec.Command("sh", "-c", selected)
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				cmd.Stdin = os.Stdin
				_ = cmd.Run()
			}
			m.reset()
			return m, tea.ClearScreen
		case "ctrl+t":
			themeNames := GetThemeNames()
			selected, _, err := bubbleteaSelect("Select Theme", themeNames, CurrentTheme.Name, false)
			if err == nil && selected != "" {
				SetThemeByName(selected)
				SaveThemeSelection(selected)
				// Apply the theme and continue without quitting
				m.reset()
				return m, tea.Batch(tea.ClearScreen, tea.EnterAltScreen)
			}
			m.reset()
			return m, tea.Batch(tea.ClearScreen, tea.EnterAltScreen)
		case "ctrl+left":
			m.goBackTriggered = true
			m.quitting = true
			return m, tea.Quit
		case "ctrl+b":
			m.goBackTriggered = true
			m.quitting = true
			return m, tea.Quit
		}
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			// Calculate which item was clicked based on Y coordinate
			itemIndex := int(msg.Y) - 4 // 2 for title+newline, 1 for filter (if present), 1 for border (if present)
			if m.filterMode {
				itemIndex-- // extra line for filter input
			}
			start := m.page * m.itemsPerPage
			end := min(start+m.itemsPerPage, len(m.filteredItems))
			if itemIndex >= 0 && itemIndex < end-start {
				m.choice = m.filteredItems[start+itemIndex]
				m.mouseClicked = true
				m.quitting = true
				return m, tea.Quit
			}
		}
	case tickMsg:
		if CurrentTheme.Name == "Matrix" && len(m.matrixRain) > 0 {
			copy(m.matrixRain[1:], m.matrixRain[:len(m.matrixRain)-1])
			m.matrixRain[0] = randomMatrixLine()
			m.frame++
			return m, tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg { return tickMsg(t) })
		}
		return m, nil
	}

	_ = cmd
	return m, nil
}

func (m menuModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	if CurrentTheme.Name == "Matrix" && len(m.matrixRain) > 0 {
		for _, line := range m.matrixRain {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Render(line) + "\n")
		}
	}

	s.WriteString(CurrentTheme.TitleStyle.Render(m.label))

	if m.filterMode {
		s.WriteString(CurrentTheme.FilterStyle.Render("Filter: " + m.textInput.View()))
	}

	start := m.page * m.itemsPerPage
	end := min(start+m.itemsPerPage, len(m.filteredItems))
	s.WriteString("\n")
	for i := start; i < end; i++ {
		item := m.filteredItems[i]
		if i-start == m.cursor {
			s.WriteString(CurrentTheme.SelectedItem.Render("▶ " + item))
		} else {
			s.WriteString(CurrentTheme.ItemStyle.Render(item))
		}
		s.WriteString("\n")
	}

	if len(m.filteredItems) > m.itemsPerPage {
		s.WriteString(fmt.Sprintf("\nPage %d/%d", m.page+1, (len(m.filteredItems)-1)/m.itemsPerPage+1))
	}

	if m.historyMode {
		help := "\nTo go back press esc key"
		s.WriteString(CurrentTheme.HelpStyle.Render(help))
		return s.String()
	}

	help := "\n↑↓ Move • Enter Select • / Filter • q Quit • ctrl+b Back • ctrl+h History"
	if m.filterMode {
		help = "\nEsc: Exit Filter • Enter Apply Filter"
	}
	s.WriteString(CurrentTheme.HelpStyle.Render(help))

	return s.String()
}

// maxLayoutHeight caps the chrome height so a 100-row terminal does not
// produce a 90-row mostly-empty box. Width is left unbounded — rows fill
// the screen, the alternating-background pattern is gone, so wide screens
// look fine.
const maxLayoutHeight = 28

func (m ideModel) View() string {
	title := "EXEC ECS"
	if m.width < 60 {
		title = "ECS"
	}

	boxWidth := m.width - 2
	if boxWidth < 20 {
		boxWidth = 20
	}
	boxHeight := min(m.height-4, maxLayoutHeight)
	if boxHeight < 8 {
		boxHeight = 8
	}

	mainBox := lipgloss.NewStyle().
		Border(CurrentTheme.BorderStyle, true).
		BorderForeground(CurrentTheme.MainBorder).
		Background(CurrentTheme.MainBg).
		Padding(1, 2).
		Width(boxWidth).
		Height(boxHeight).
		Render(m.menu.menuViewOnly())

	help := m.menu.menuHelpOnly()
	status := lipgloss.NewStyle().
		Background(CurrentTheme.StatusBg).
		Foreground(CurrentTheme.StatusFg).
		Padding(0, 2).
		Width(m.width).
		Render(help)
	titleRendered := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	return titleRendered + "\n" + mainBox + "\n" + status
}

func (m ideModel) Init() tea.Cmd {
	return m.menu.Init()
}

func (m ideModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Width runs full-screen now; only height is capped so a tall
		// terminal doesn't produce a 60-row empty box.
		effectiveHeight := min(msg.Height, maxLayoutHeight)

		scaleFactor := 1.0
		if msg.Width < 80 {
			scaleFactor = 0.8
		} else if msg.Width > 120 {
			scaleFactor = 1.2
		}

		availableHeight := effectiveHeight - 8
		itemsPerPage := max(5, min(availableHeight, 20))

		m.menu.width = msg.Width
		m.menu.height = effectiveHeight
		m.menu.scaleFactor = scaleFactor
		m.menu.itemsPerPage = itemsPerPage
		m.menu.viewport.Width = msg.Width - 4
		m.menu.viewport.Height = itemsPerPage + 2

		m.menu.textInput.Width = min(50, msg.Width-20)

		return m, nil
	}
	updated, cmd := m.menu.Update(msg)
	m.menu = updated.(menuModel)
	return m, cmd
}

func (m menuModel) menuViewOnly() string {
	if m.quitting {
		return ""
	}
	var s strings.Builder
	// Title (inside main area)
	s.WriteString(CurrentTheme.TitleStyle.Render(m.label))
	s.WriteString("\n\n") // Add a newline after the label/title
	// Filter input
	if m.filterMode {
		s.WriteString(CurrentTheme.FilterStyle.Render("Filter: " + m.textInput.View()))
		s.WriteString("\n") // Add a newline after filter input so border/items always align
	}
	// Items with per-theme effects
	start := m.page * m.itemsPerPage
	end := min(start+m.itemsPerPage, len(m.filteredItems))

	// Scale theme-specific elements based on terminal size
	if CurrentTheme.Name == "Pac-Man" {
		ghosts := []string{"👻", "👾", "👻", "👾"}
		mazeBorder := lipgloss.NewStyle().Foreground(lipgloss.Color("#fff200")).Render("╔══════════════════════════════════╗")
		mazeBottom := lipgloss.NewStyle().Foreground(lipgloss.Color("#fff200")).Render("╚══════════════════════════════════╝")
		s.WriteString(mazeBorder + "\n")
		for i := start; i < end; i++ {
			item := m.filteredItems[i]
			icon := ghosts[i%len(ghosts)]
			dots := ""
			for d := 0; d < 8; d++ {
				dots += "·"
			}
			if i-start == m.cursor {
				s.WriteString(CurrentTheme.SelectedItem.Render("🟡" + dots + " " + item))
			} else {
				s.WriteString(CurrentTheme.ItemStyle.Render(icon + dots + " " + item))
			}
			s.WriteString("\n")
		}
		s.WriteString(mazeBottom + "\n")
		return s.String()
	}
	if CurrentTheme.Name == "Matrix" {
		codeRainTop := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Render("┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓")
		codeRainBottom := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Render("┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛")
		s.WriteString(codeRainTop + "\n")
		for i := start; i < end; i++ {
			item := m.filteredItems[i]
			if i-start == m.cursor {
				s.WriteString(CurrentTheme.SelectedItem.Render("▣ " + item + " "))
			} else {
				s.WriteString(CurrentTheme.ItemStyle.Render("░ " + item + " "))
			}
			s.WriteString("\n")
		}
		s.WriteString(codeRainBottom + "\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Render("  ᚠᚮᛚᛚᚮᚡ Þᛂ ᚡᛡᛁᛐᛂ ᚱᛆᛒᛒᛁᛐ  "))
		return s.String()
	}
	for i := start; i < end; i++ {
		item := m.filteredItems[i]
		// Use a single row style on every row — alternating backgrounds
		// looked odd on wide terminals because the highlighted strip only
		// covered the rendered text, not the full row.
		style := CurrentTheme.ItemStyle
		icon := CurrentTheme.UnselectedIcon
		if i-start == m.cursor {
			icon = CurrentTheme.SelectionIcon
			s.WriteString(CurrentTheme.SelectedItem.Render(icon + " " + item + strings.Repeat(" ", CurrentTheme.SelectedPaddingRight)))
		} else {
			s.WriteString(style.Render(icon + " " + item))
		}
		s.WriteString("\n")
	}
	// Pagination info
	if len(m.filteredItems) > m.itemsPerPage {
		s.WriteString(fmt.Sprintf("\nPage %d/%d", m.page+1, (len(m.filteredItems)-1)/m.itemsPerPage+1))
	}
	if m.historyMode {
		s.WriteString(CurrentTheme.HelpStyle.Render("\nTo go back press esc key"))
	}
	return s.String()
}

func (m menuModel) menuHelpOnly() string {
	// Scale help text based on terminal width
	defaultShortcuts := "↑↓ Move • Enter Select • / Filter • q Quit • ctrl+b Back • ctrl+h History • ctrl+t Theme"
	if m.width < 80 {
		defaultShortcuts = "↑↓ Move • Enter Select • / Filter • q Quit • ctrl+b Back • ctrl+h History • ctrl+t Theme"
	} else if m.width > 120 {
		defaultShortcuts = "↑↓ Move • Enter Select • / Filter • q Quit • ctrl+b Back • ctrl+h History • ctrl+t Theme"
	}

	if m.filterMode {
		return "Esc: Exit Filter • Enter Apply Filter"
	}
	if m.historyMode {
		return "To go back press esc key"
	}
	custom := CurrentTheme.HelpHint
	if custom != "" {
		return custom + "  " + defaultShortcuts
	}
	return defaultShortcuts
}

func (m *menuModel) filterItems(filter string) {
	if filter == "" {
		m.filteredItems = m.items
		return
	}
	filtered := make([]string, 0)
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item), strings.ToLower(filter)) {
			filtered = append(filtered, item)
		}
	}
	m.filteredItems = filtered
	m.cursor = 0
	m.page = 0
}

// updateThemePreview updates the preview theme based on current cursor position
func (m *menuModel) updateThemePreview() {
	if !m.isThemeSelection {
		return
	}

	start := m.page * m.itemsPerPage
	end := min(start+m.itemsPerPage, len(m.filteredItems))
	if m.cursor >= 0 && m.cursor < end-start {
		selectedIndex := start + m.cursor
		if selectedIndex < len(m.filteredItems) {
			themeName := m.filteredItems[selectedIndex]
			// Find the theme by name and preview it
			for _, theme := range allThemes {
				if theme.Name == themeName {
					m.previewTheme = theme
					CurrentTheme = theme
					return
				}
			}
		}
	}
}

func (m *menuModel) reset() {
	m.filteredItems = m.items
	m.cursor = 0
	m.page = 0
	m.filterMode = false
	m.textInput.SetValue("")
	if CurrentTheme.Name == "Matrix" {
		m.matrixRain = make([]string, matrixRainHeight)
		for i := range m.matrixRain {
			m.matrixRain[i] = randomMatrixLine()
		}
	}
	// Reset theme preview
	m.previewTheme = nil
	m.originalTheme = nil
	m.isThemeSelection = false
}
