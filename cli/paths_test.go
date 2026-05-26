package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirOverride(t *testing.T) {
	t.Parallel()
	prev := configDirOverride
	configDirOverride = "/tmp/override"
	defer func() { configDirOverride = prev }()
	if got := ConfigDir(); got != "/tmp/override" {
		t.Fatalf("override ignored: %s", got)
	}
}

func TestConfigDirHonoursXDG(t *testing.T) {
	prev := configDirOverride
	configDirOverride = ""
	t.Cleanup(func() { configDirOverride = prev })

	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	if got := ConfigDir(); got != "/tmp/xdg-test/exec-ecs" {
		t.Fatalf("XDG override ignored: got %s", got)
	}
}

func TestConfigDirFallsBackToDotConfig(t *testing.T) {
	prev := configDirOverride
	configDirOverride = ""
	t.Cleanup(func() { configDirOverride = prev })

	// Unset XDG; default should be ~/.config/exec-ecs on every platform
	// (incl. macOS / Windows — we deliberately deviate from os.UserConfigDir).
	t.Setenv("XDG_CONFIG_HOME", "")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	want := tmp + "/.config/exec-ecs"
	if got := ConfigDir(); got != want {
		t.Fatalf("default config dir = %q want %q", got, want)
	}
}

func TestEnsureConfigDirCreates(t *testing.T) {
	tmp := t.TempDir()
	prev := configDirOverride
	configDirOverride = filepath.Join(tmp, "nested")
	t.Cleanup(func() { configDirOverride = prev })
	if err := EnsureConfigDir(); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	stat, err := os.Stat(configDirOverride)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !stat.IsDir() {
		t.Fatal("not a dir")
	}
	if stat.Mode().Perm() != 0o700 {
		t.Fatalf("perm = %v", stat.Mode().Perm())
	}
}

func TestMigrateLegacyPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := t.TempDir()
	prev := configDirOverride
	configDirOverride = cfg
	t.Cleanup(func() { configDirOverride = prev })

	// Drop a legacy history file in home.
	legacy := filepath.Join(home, ".ecs_cli_history")
	if err := os.WriteFile(legacy, []byte("aws ecs ...\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	migrateLegacyPaths()

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy file should have been moved, stat err=%v", err)
	}
	moved := filepath.Join(cfg, "history")
	data, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("read moved: %v", err)
	}
	if string(data) != "aws ecs ...\n" {
		t.Fatalf("contents = %q", string(data))
	}
}

func TestMigrateLegacyPathsKeepsExistingTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := t.TempDir()
	prev := configDirOverride
	configDirOverride = cfg
	t.Cleanup(func() { configDirOverride = prev })

	// Legacy + new both exist; the new should win.
	if err := os.WriteFile(filepath.Join(home, ".ecs_cli_theme"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "theme"), []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}

	migrateLegacyPaths()

	data, err := os.ReadFile(filepath.Join(cfg, "theme"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "current" {
		t.Fatalf("expected current preserved, got %q", string(data))
	}
}

func TestMigrateLegacyPathsIgnoresNativeConfigTheme(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := t.TempDir()
	prev := configDirOverride
	configDirOverride = cfg
	t.Cleanup(func() { configDirOverride = prev })

	nativeTheme := filepath.Join(home, "Library", "Application Support", "exec-ecs", "theme")
	if err := os.MkdirAll(filepath.Dir(nativeTheme), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nativeTheme, []byte("native"), 0o600); err != nil {
		t.Fatal(err)
	}

	migrateLegacyPaths()

	if _, err := os.Stat(filepath.Join(cfg, "theme")); !os.IsNotExist(err) {
		t.Fatalf("native config theme should not be migrated, stat err=%v", err)
	}
	data, err := os.ReadFile(nativeTheme)
	if err != nil {
		t.Fatalf("native theme should remain untouched: %v", err)
	}
	if string(data) != "native" {
		t.Fatalf("native theme contents = %q", string(data))
	}
}
