package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// configDirOverride lets tests redirect the config root without messing with
// the user's HOME / XDG_CONFIG_HOME. Empty means "use the default lookup".
var configDirOverride string

// ConfigDir returns the directory where exec-ecs stores its history, theme
// choice, and region cache. We deliberately diverge from os.UserConfigDir on
// macOS and Windows: this is a CLI tool, and the convention modern CLIs
// (gh, kubectl, helm, terraform, …) follow is XDG-style `~/.config/<name>/`
// on every platform, so the same dotfiles + shell tooling work the same way
// across machines.
//
// Lookup order:
//
//  1. $XDG_CONFIG_HOME/exec-ecs (if XDG_CONFIG_HOME is set and non-empty)
//  2. ~/.config/exec-ecs (on every platform — Linux, macOS, Windows)
//
// The directory is created on first use with mode 0700 so secrets-adjacent
// files (region cache, history of run commands) are owner-only.
func ConfigDir() string {
	if configDirOverride != "" {
		return configDirOverride
	}
	if v := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); v != "" {
		return filepath.Join(v, "exec-ecs")
	}
	return filepath.Join(homeDir(), ".config", "exec-ecs")
}

// EnsureConfigDir creates the config dir if missing. Cheap to call repeatedly.
func EnsureConfigDir() error {
	return os.MkdirAll(ConfigDir(), 0o700)
}

// historyPath / themePath / regionCacheFilePath centralise the on-disk layout
// so any future move is a single-file change.
func historyPath() string { return filepath.Join(ConfigDir(), "history") }
func themePath() string   { return filepath.Join(ConfigDir(), "theme") }
func regionCacheFilePath() string {
	if v := os.Getenv("EXEC_ECS_REGION_CACHE_PATH"); v != "" {
		return v
	}
	return filepath.Join(ConfigDir(), "region-cache.json")
}

// legacyPaths returns the historical home-directory dotfiles we used before
// adopting `~/.config/exec-ecs/`. We read these on first access so existing
// users don't lose their command history or theme choice on upgrade.
//
// Only bare dotfiles directly in $HOME are migrated:
// `.ecs_cli_history`, `.ecs_cli_theme`, `.exec-ecs-region-cache.json`.
type legacyPath struct{ old, current string }

func legacyPaths() []legacyPath {
	h := homeDir()
	return []legacyPath{
		{filepath.Join(h, ".ecs_cli_history"), historyPath()},
		{filepath.Join(h, ".ecs_cli_theme"), themePath()},
		{filepath.Join(h, ".exec-ecs-region-cache.json"), regionCacheFilePath()},
	}
}

// migrateLegacyPaths moves any legacy file into the new config dir if the
// new path doesn't already exist. Failures are non-fatal — we'd rather use
// the default than abort startup.
func migrateLegacyPaths() {
	if err := EnsureConfigDir(); err != nil {
		return
	}
	for _, lp := range legacyPaths() {
		if _, err := os.Stat(lp.current); err == nil {
			continue
		}
		if _, err := os.Stat(lp.old); err != nil {
			continue
		}
		_ = os.Rename(lp.old, lp.current)
	}
}
