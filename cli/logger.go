package cli

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var cmdLogger = log.New(os.Stdout, "[AWS CMD] ", log.Ltime)

func (c *Cli) LogAWSCommand(cmd string, args ...string) {
	cmdLogger.Printf("aws %s %s", cmd, strings.Join(args, " "))
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
