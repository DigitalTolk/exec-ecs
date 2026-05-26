package cli

import (
	"os"
	"path/filepath"
)

// configDirOverride lets tests redirect the config root without messing with
// the user's HOME / XDG_CONFIG_HOME. Empty means "use os.UserConfigDir".
var configDirOverride string

// ConfigDir returns the directory where exec-ecs stores its history, theme
// choice, and region cache. Uses the platform-standard location:
//
//   - Linux:   $XDG_CONFIG_HOME/exec-ecs (defaulting to ~/.config/exec-ecs)
//   - macOS:   ~/Library/Application Support/exec-ecs
//   - Windows: %AppData%\exec-ecs
//
// The directory is created on first use with mode 0700 so secrets-adjacent
// files (region cache, history of run commands) are owner-only.
func ConfigDir() string {
	if configDirOverride != "" {
		return configDirOverride
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "exec-ecs")
	}
	return filepath.Join(homeDir(), ".config", "exec-ecs")
}

// EnsureConfigDir creates the config dir if missing. Cheap to call repeatedly.
func EnsureConfigDir() error {
	return os.MkdirAll(ConfigDir(), 0o700)
}

// historyPath / themePath / regionCacheFilePath centralise the on-disk layout
// so any future move is a single-file change.
func historyPath() string     { return filepath.Join(ConfigDir(), "history") }
func themePath() string       { return filepath.Join(ConfigDir(), "theme") }
func regionCacheFilePath() string {
	if v := os.Getenv("EXEC_ECS_REGION_CACHE_PATH"); v != "" {
		return v
	}
	return filepath.Join(ConfigDir(), "region-cache.json")
}

// legacyPaths returns the historical home-directory locations we used before
// adopting the config dir. We read these on first access so existing users
// don't lose their command history or theme choice on upgrade.
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
