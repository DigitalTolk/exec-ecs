package cli

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var cmdLogger = log.New(os.Stdout, "\n [AWS CMD] ", log.Ltime)
var historyFile = os.Getenv("HOME") + "/.ecs_cli_history"

func (c *Cli) LogAWSCommand(cmd string, args ...string) {
	if c.Debug {
		cmdLogger.Printf("aws %s %s", cmd, strings.Join(args, " "))
	}
}

func (c *Cli) LogUserFriendlyError(message string, err error, potentialFix, filePath string, lineNumber int) {
	fmt.Printf("\n============================\n")
	fmt.Printf("\033[1;31mERROR: %s\033[0m\n", message)
	fmt.Printf("Details: %v\n", err)
	fmt.Printf("Potential Fix: \033[1;34m%s\033[0m\n", potentialFix)
	if filePath != "" {
		fmt.Printf("File Path: \033[1;32m%s\033[0m\n", filePath)
	}
	if lineNumber > 0 {
		fmt.Printf("Line Number: \033[1;36m%d\033[0m\n", lineNumber)
	}
	fmt.Printf("============================\n\n")
	os.Exit(1)
}

// Append a command to the history file
func AppendToHistory(cmd string) {
	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return // fail silently
	}
	defer f.Close()
	f.WriteString(cmd + "\n")
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
