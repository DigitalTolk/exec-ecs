package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

var cmdLogger = log.New(os.Stdout, "\n [AWS CMD] ", log.Ltime)
// historyFile is a `var` so tests can rebind it. In normal runs it always
// resolves to the standard config dir; we only fall back to legacy paths via
// migrateLegacyPaths() at startup.
var historyFile = historyPath()

// errorWriter and exitFn are package-level so tests can capture output without
// actually exiting the process.
var (
	errorWriter io.Writer = os.Stdout
	exitFn                = os.Exit
)

func (c *Cli) LogAWSCommand(cmd string, args ...string) {
	if c.Debug {
		cmdLogger.Printf("aws %s %s", cmd, strings.Join(args, " "))
	}
}

func (c *Cli) LogUserFriendlyError(message string, err error, potentialFix, filePath string, lineNumber int) {
	fmt.Fprintf(errorWriter, "\n============================\n")
	fmt.Fprintf(errorWriter, "\033[1;31mERROR: %s\033[0m\n", message)
	fmt.Fprintf(errorWriter, "Details: %v\n", err)
	fmt.Fprintf(errorWriter, "Potential Fix: \033[1;34m%s\033[0m\n", potentialFix)
	if filePath != "" {
		fmt.Fprintf(errorWriter, "File Path: \033[1;32m%s\033[0m\n", filePath)
	}
	if lineNumber > 0 {
		fmt.Fprintf(errorWriter, "Line Number: \033[1;36m%d\033[0m\n", lineNumber)
	}
	fmt.Fprintf(errorWriter, "============================\n\n")
	exitFn(1)
}

// Append a command to the history file
func AppendToHistory(cmd string) {
	_ = EnsureConfigDir()
	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return // fail silently
	}
	defer f.Close()
	_, _ = f.WriteString(cmd + "\n")
}

// Get the last 5 unique commands from history, most recent first
func GetLastUniqueHistory(n int) []string {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	seen := make(map[string]struct{})
	var unique []string
	for i := len(lines) - 1; i >= 0 && len(unique) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if _, ok := seen[line]; !ok {
			unique = append(unique, line)
			seen[line] = struct{}{}
		}
	}
	return unique
}
