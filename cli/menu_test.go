package cli

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilterItemsContainsCaseInsensitive(t *testing.T) {
	t.Parallel()
	m := &menuModel{items: []string{"alpha", "Beta", "gamma", "delta"}, filteredItems: nil}
	m.filterItems("")
	if !reflect.DeepEqual(m.filteredItems, m.items) {
		t.Fatalf("empty filter should keep all items")
	}

	m.filterItems("BE")
	if !reflect.DeepEqual(m.filteredItems, []string{"Beta"}) {
		t.Fatalf("filtered = %v", m.filteredItems)
	}

	m.filterItems("a")
	want := []string{"alpha", "Beta", "gamma", "delta"}
	if !reflect.DeepEqual(m.filteredItems, want) {
		t.Fatalf("filtered = %v want %v", m.filteredItems, want)
	}

	m.filterItems("nothing-matches")
	if len(m.filteredItems) != 0 {
		t.Fatalf("expected empty, got %v", m.filteredItems)
	}
}

func TestMenuReset(t *testing.T) {
	t.Parallel()
	m := &menuModel{
		items:            []string{"a", "b"},
		filteredItems:    []string{"a"},
		cursor:           3,
		page:             2,
		filterMode:       true,
		previewTheme:     MatrixTheme,
		originalTheme:    DraculaTheme,
		isThemeSelection: true,
	}
	m.reset()
	if m.cursor != 0 || m.page != 0 || m.filterMode {
		t.Fatalf("reset did not clear state: %+v", *m)
	}
	if !reflect.DeepEqual(m.filteredItems, m.items) {
		t.Fatal("filtered should equal items after reset")
	}
	if m.isThemeSelection || m.previewTheme != nil || m.originalTheme != nil {
		t.Fatal("theme selection state must be cleared on reset")
	}
}

func TestRandomMatrixLineHasFixedWidth(t *testing.T) {
	t.Parallel()
	line := randomMatrixLine()
	if len(line) != matrixRainWidth {
		t.Fatalf("randomMatrixLine length = %d want %d", len(line), matrixRainWidth)
	}
}

func TestInitialModelHonoursDefault(t *testing.T) {
	t.Parallel()
	items := []string{"x", "y", "z"}
	m := initialModel("pick", items, "y", false)
	// Initial cursor should land on the matching default.
	if m.cursor+m.page*m.itemsPerPage != 1 {
		t.Fatalf("expected default selection at index 1, got %d (cursor=%d, page=%d)", m.cursor+m.page*m.itemsPerPage, m.cursor, m.page)
	}
}

func TestInitialModelDefaultsZeroWhenAbsent(t *testing.T) {
	t.Parallel()
	items := []string{"x", "y", "z"}
	m := initialModel("pick", items, "missing", false)
	if m.cursor != 0 || m.page != 0 {
		t.Fatalf("missing default should land on first item, got cursor=%d page=%d", m.cursor, m.page)
	}
}

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+b":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case "ctrl+left":
		return tea.KeyMsg{Type: tea.KeyCtrlLeft}
	case "/":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func TestMenuModelUpdateNavigation(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a", "b", "c"}, "", false)
	m.itemsPerPage = 10

	updated, _ := m.Update(keyMsg("down"))
	mm := updated.(menuModel)
	if mm.cursor != 1 {
		t.Fatalf("down: cursor=%d", mm.cursor)
	}
	updated, _ = mm.Update(keyMsg("up"))
	mm = updated.(menuModel)
	if mm.cursor != 0 {
		t.Fatalf("up: cursor=%d", mm.cursor)
	}
}

func TestMenuModelUpdateEnterSelects(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"alpha", "beta"}, "", false)
	m.itemsPerPage = 10
	updated, cmd := m.Update(keyMsg("down"))
	mm := updated.(menuModel)
	updated, _ = mm.Update(keyMsg("enter"))
	mm = updated.(menuModel)
	if mm.choice != "beta" {
		t.Fatalf("enter selected = %q", mm.choice)
	}
	if !mm.quitting {
		t.Fatal("enter should set quitting")
	}
	_ = cmd
}

func TestMenuModelUpdateGoBack(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a"}, "", true)
	m.itemsPerPage = 10
	updated, _ := m.Update(keyMsg("ctrl+b"))
	mm := updated.(menuModel)
	if !mm.goBackTriggered || !mm.quitting {
		t.Fatalf("expected goBack+quitting: %+v", mm)
	}
}

func TestMenuModelEnterFilterMode(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a", "b"}, "", false)
	m.itemsPerPage = 10
	updated, _ := m.Update(keyMsg("/"))
	mm := updated.(menuModel)
	if !mm.filterMode {
		t.Fatalf("/ should enter filter mode")
	}
	updated, _ = mm.Update(keyMsg("esc"))
	mm = updated.(menuModel)
	if mm.filterMode {
		t.Fatalf("esc should exit filter mode")
	}
}

func TestMenuModelViewNonEmpty(t *testing.T) {
	t.Parallel()
	m := initialModel("Choose AWS region", []string{"us-east-1", "us-west-2"}, "us-east-1", true)
	m.itemsPerPage = 10
	out := m.View()
	if out == "" {
		t.Fatal("View should render something")
	}
}

func TestIdeModelView(t *testing.T) {
	t.Parallel()
	im := ideModel{menu: initialModel("Pick", []string{"a", "b"}, "", false), width: 100, height: 30}
	im.menu.width = im.width
	im.menu.height = im.height
	im.menu.itemsPerPage = 10
	if out := im.View(); out == "" {
		t.Fatal("ide View should render something")
	}
}

func TestIdeModelWindowResize(t *testing.T) {
	t.Parallel()
	im := ideModel{menu: initialModel("Pick", []string{"a"}, "", false)}
	updated, _ := im.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	im2 := updated.(ideModel)
	if im2.width != 200 || im2.height != 60 {
		t.Fatalf("width/height not updated: %+v", im2)
	}
	if im2.menu.width > maxLayoutWidth || im2.menu.height > maxLayoutHeight {
		t.Fatalf("menu dims should be capped: %+v", im2.menu)
	}
}
