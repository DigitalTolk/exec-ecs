package cli

import (
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestMenuModelEnterClampsStalePage(t *testing.T) {
	t.Parallel()
	items := make([]string, 18)
	for i := range items {
		items[i] = "profile"
	}
	items[17] = "last"

	m := initialModel("Choose AWS profile", items, "", false)
	m.itemsPerPage = 10
	m.page = 2
	m.cursor = 7

	updated, _ := m.Update(keyMsg("enter"))
	mm := updated.(menuModel)
	if mm.choice != "last" {
		t.Fatalf("stale page/cursor should clamp to last item, got %q", mm.choice)
	}
}

func TestMenuViewClampsStalePageSoProfilesRender(t *testing.T) {
	t.Parallel()
	items := []string{"alpha", "beta"}
	m := initialModel("Choose AWS profile", items, "", false)
	m.itemsPerPage = 10
	m.page = 2
	m.cursor = 7

	out := m.menuViewOnly()
	if !strings.Contains(out, "alpha") && !strings.Contains(out, "beta") {
		t.Fatalf("stale page should still render profile items, got %q", out)
	}
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

func TestMenuModelLoadingRendersInsideDialog(t *testing.T) {
	t.Parallel()
	m := initialModel("Choose ECS cluster", nil, "", true)
	m.loading = true
	m.loadingMessage = "Connecting to ECS..."
	m.itemsPerPage = 10

	out := m.menuViewOnly()
	if !strings.Contains(out, "Choose ECS cluster") || !strings.Contains(out, "Connecting to ECS...") {
		t.Fatalf("loading view missing label/message: %q", out)
	}
	if help := m.menuHelpOnly(); !strings.Contains(help, "Loading") || !strings.Contains(help, "Back") {
		t.Fatalf("loading help = %q", help)
	}
}

func TestMenuModelLoadItemsSwitchesToSelection(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", nil, "beta", true)
	m.loading = true
	m.itemsPerPage = 10

	updated, _ := m.Update(loadItemsMsg{items: []string{"alpha", "beta"}})
	mm := updated.(menuModel)
	if mm.loading {
		t.Fatal("load completion should exit loading mode")
	}
	if mm.cursor != 1 {
		t.Fatalf("cursor = %d, want default item index 1", mm.cursor)
	}
}

func TestMenuModelLoadItemsAutoSelectsSingleItem(t *testing.T) {
	t.Parallel()
	m := initialModel("Choose ECS cluster", nil, "", true)
	m.loading = true
	m.autoSelectSingle = true
	m.itemsPerPage = 10

	updated, _ := m.Update(loadItemsMsg{items: []string{"only-cluster"}})
	mm := updated.(menuModel)
	if mm.choice != "only-cluster" || !mm.quitting {
		t.Fatalf("single item should auto-select and quit: %+v", mm)
	}
}

func TestMenuModelBreadcrumbRenders(t *testing.T) {
	t.Parallel()
	m := initialModelWithBreadcrumb("Choose ECS service", []string{"api"}, "", true, "Profile: dt > Region: eu-north-1 > Cluster: prod")
	m.itemsPerPage = 10

	out := m.menuViewOnly()
	if !strings.Contains(out, "Profile: dt") || !strings.Contains(out, "Cluster: prod") {
		t.Fatalf("breadcrumb missing from view: %q", out)
	}
}

func TestMenuModelHighlightUsesPlainMarkerAcrossThemes(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })

	for _, theme := range []*Theme{GhostyTheme, PacManTheme, MatrixTheme, ZeldaTheme} {
		CurrentTheme = theme
		m := initialModel("pick", []string{"alpha"}, "", true)
		m.itemsPerPage = 10
		out := m.menuViewOnly()
		for _, disallowed := range []string{"👻", "👾", "🟡", "💚", "▣"} {
			if strings.Contains(out, disallowed) {
				t.Fatalf("theme %s highlight should not render theme marker %q in %q", theme.Name, disallowed, out)
			}
		}
		if !strings.Contains(out, selectedMarker+" alpha") && !strings.Contains(out, selectedMarker+"········ alpha") {
			t.Fatalf("theme %s missing plain selected marker in %q", theme.Name, out)
		}
	}
}

func TestMenuModelEscGoesBackWhenAvailable(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a"}, "", true)
	m.itemsPerPage = 10

	updated, _ := m.Update(keyMsg("esc"))
	mm := updated.(menuModel)
	if !mm.goBackTriggered || !mm.quitting {
		t.Fatalf("esc should trigger goBack when available: %+v", mm)
	}
}

func TestMenuModelEscQuitsWhenNoBackStep(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a"}, "", false)
	m.itemsPerPage = 10

	updated, _ := m.Update(keyMsg("esc"))
	mm := updated.(menuModel)
	if mm.goBackTriggered || !mm.quitting {
		t.Fatalf("esc should only quit without a back step: %+v", mm)
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

func TestMenuModelInitReturnsACmd(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a"}, "", false)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init returned nil")
	}
}

func TestMenuModelInitMatrixTheme(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })
	CurrentTheme = MatrixTheme
	m := initialModel("pick", []string{"a"}, "", false)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("matrix init should also return a non-nil cmd")
	}
}

func TestMenuModelViewQuitting(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a"}, "", false)
	m.quitting = true
	if out := m.View(); out != "" {
		t.Fatalf("expected empty view when quitting, got %q", out)
	}
}

func TestMenuModelViewHelp(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"alpha"}, "", false)
	m.itemsPerPage = 5
	m.historyMode = true
	out := m.View()
	if out == "" {
		t.Fatal("history-mode view should render help")
	}
}

func TestIdeModelInit(t *testing.T) {
	t.Parallel()
	im := ideModel{menu: initialModel("pick", []string{"a"}, "", false)}
	if cmd := im.Init(); cmd == nil {
		t.Fatal("nil cmd")
	}
}

func TestMenuModelUpdatePageNavigation(t *testing.T) {
	t.Parallel()
	items := make([]string, 25)
	for i := range items {
		items[i] = "i" + string(rune('a'+i%26))
	}
	m := initialModel("pick", items, "", false)
	m.itemsPerPage = 10

	// pgdown advances pages.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	mm := updated.(menuModel)
	if mm.page != 1 {
		t.Fatalf("pgdown page=%d", mm.page)
	}
	// pgup rewinds.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	mm = updated.(menuModel)
	if mm.page != 0 {
		t.Fatalf("pgup page=%d", mm.page)
	}
}

func TestMenuModelUpdateFilterTyping(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"alpha", "alabama", "beta"}, "", false)
	m.itemsPerPage = 10
	// Enter filter mode.
	updated, _ := m.Update(keyMsg("/"))
	mm := updated.(menuModel)
	// Type "al".
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	mm = updated.(menuModel)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	mm = updated.(menuModel)
	// alpha and alabama both contain "al".
	if len(mm.filteredItems) != 2 {
		t.Fatalf("expected 2 matches, got %v", mm.filteredItems)
	}
	// Exit filter via enter — filter mode should drop.
	updated, _ = mm.Update(keyMsg("enter"))
	mm = updated.(menuModel)
	if mm.filterMode {
		t.Fatal("enter should exit filter mode")
	}
}

func TestMenuModelUpdateMouseClick(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"alpha", "beta", "gamma"}, "", false)
	m.itemsPerPage = 10
	// MouseLeft at row 4 (item index 0 after the title/border offsets).
	updated, _ := m.Update(tea.MouseMsg{Type: tea.MouseLeft, Y: 4})
	mm := updated.(menuModel)
	if !mm.mouseClicked {
		t.Fatal("expected mouseClicked")
	}
	if mm.choice == "" {
		t.Fatal("expected a choice")
	}
}

func TestMenuModelUpdateTick(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })
	CurrentTheme = MatrixTheme
	m := initialModel("pick", []string{"a"}, "", false)
	updated, cmd := m.Update(tickMsg(time.Now()))
	mm := updated.(menuModel)
	if mm.frame == 0 {
		t.Fatal("frame should advance on tick")
	}
	if cmd == nil {
		t.Fatal("matrix tick should schedule next tick")
	}
}

func TestMenuModelUpdateTickNonMatrix(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })
	CurrentTheme = DraculaTheme
	m := initialModel("pick", []string{"a"}, "", false)
	updated, cmd := m.Update(tickMsg(time.Now()))
	mm := updated.(menuModel)
	if mm.frame != 0 {
		t.Fatal("non-matrix theme should ignore tick")
	}
	_ = cmd
}

func TestMenuModelUpdateCtrlLeft(t *testing.T) {
	t.Parallel()
	m := initialModel("pick", []string{"a"}, "", true)
	m.itemsPerPage = 10
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlLeft})
	mm := updated.(menuModel)
	if !mm.goBackTriggered || !mm.quitting {
		t.Fatalf("ctrl+left should trigger goBack: %+v", mm)
	}
}

func TestMenuViewOnlyAcrossThemes(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })

	for _, theme := range allThemes {
		CurrentTheme = theme
		m := initialModel("pick", []string{"a", "b"}, "", false)
		m.itemsPerPage = 5
		if out := m.menuViewOnly(); out == "" {
			t.Fatalf("menuViewOnly empty under theme %s", theme.Name)
		}
	}
}

func TestThemePreviewSwitches(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })

	m := initialModel("Select Theme", GetThemeNames(), "", false)
	m.itemsPerPage = 20
	m.isThemeSelection = true
	m.originalTheme = CurrentTheme
	m.updateThemePreview()
	if m.previewTheme == nil {
		t.Fatal("expected a previewTheme after first update")
	}
	// Move cursor; preview should swap.
	first := m.previewTheme
	m.cursor = 1
	m.updateThemePreview()
	if m.previewTheme == first {
		t.Fatal("preview did not change with cursor")
	}
}

func TestIdeModelViewQuitting(t *testing.T) {
	t.Parallel()
	im := ideModel{menu: initialModel("pick", []string{"a"}, "", false), width: 80, height: 20}
	im.menu.quitting = true
	// View only short-circuits in menuViewOnly; ideModel.View still renders.
	if out := im.View(); out == "" {
		t.Fatal("ide view should render frame even when inner menu is quitting")
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
	// Width now scales freely (full terminal); only height is capped.
	if im2.menu.width != 200 {
		t.Fatalf("menu width should match terminal width, got %d", im2.menu.width)
	}
	if im2.menu.height > maxLayoutHeight {
		t.Fatalf("menu height should be capped at %d, got %d", maxLayoutHeight, im2.menu.height)
	}
}
