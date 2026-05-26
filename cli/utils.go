package cli

import "os"

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// homeDir returns the current user's home directory using the same lookup
// rules across all platforms, falling back to `$HOME` only if the call fails
// (e.g. in a stripped-down container). Using `os.Getenv("HOME")` directly is
// wrong on Windows and breaks under daemon contexts where HOME is empty.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return os.Getenv("HOME")
}
