package installer

import (
	"log"
	"os"
	"os/exec"
)

func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func executeCommands(commands [][]string) {
	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to execute command %s: %v", cmdArgs[0], err)
		}
	}
}
