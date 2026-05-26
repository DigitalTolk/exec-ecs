package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetThemeNamesIncludesDefaults(t *testing.T) {
	t.Parallel()
	names := GetThemeNames()
	if len(names) != len(allThemes) {
		t.Fatalf("expected %d themes, got %d", len(allThemes), len(names))
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["Matrix"] || !found["Dracula"] {
		t.Fatalf("expected canonical themes, got %v", names)
	}
}

func TestSetThemeByName(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })

	SetThemeByName("Matrix")
	if CurrentTheme.Name != "Matrix" {
		t.Fatalf("expected Matrix, got %s", CurrentTheme.Name)
	}

	SetThemeByName("does-not-exist")
	if CurrentTheme.Name != "Matrix" {
		t.Fatalf("expected unchanged theme, got %s", CurrentTheme.Name)
	}
}

func TestSaveAndLoadThemeSelection(t *testing.T) {
	tmp := t.TempDir()
	prev := configDirOverride
	configDirOverride = tmp
	t.Cleanup(func() { configDirOverride = prev })

	if got := LoadThemeSelection(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	SaveThemeSelection("Matrix")
	if got := LoadThemeSelection(); got != "Matrix" {
		t.Fatalf("expected Matrix, got %q", got)
	}

	stat, err := os.Stat(filepath.Join(tmp, "theme"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %v want 0600", stat.Mode().Perm())
	}
}

func TestApplySavedThemeSelectionRestoresThemeAfterRestart(t *testing.T) {
	tmp := t.TempDir()
	prevDir := configDirOverride
	prevTheme := CurrentTheme
	configDirOverride = tmp
	t.Cleanup(func() {
		configDirOverride = prevDir
		CurrentTheme = prevTheme
	})

	SaveThemeSelection("Matrix")
	CurrentTheme = DraculaTheme

	ApplySavedThemeSelection()

	if CurrentTheme.Name != "Matrix" {
		t.Fatalf("theme after simulated restart = %q, want Matrix", CurrentTheme.Name)
	}
}

func TestSavedThemeAffectsInitialMenuRender(t *testing.T) {
	tmp := t.TempDir()
	prevDir := configDirOverride
	prevTheme := CurrentTheme
	configDirOverride = tmp
	t.Cleanup(func() {
		configDirOverride = prevDir
		CurrentTheme = prevTheme
	})

	SaveThemeSelection("Matrix")
	CurrentTheme = DraculaTheme
	ApplySavedThemeSelection()

	m := initialModel("Choose AWS profile", []string{"alpha"}, "", false)
	if CurrentTheme.Name != "Matrix" {
		t.Fatalf("theme before render = %q, want Matrix", CurrentTheme.Name)
	}
	if out := m.menuViewOnly(); !strings.Contains(out, "alpha") {
		t.Fatalf("saved theme render lost menu content: %q", out)
	}
}
