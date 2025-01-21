package installer

import (
	"fmt"
	"log"
	"runtime"
)

func CheckAndInstallDependencies() {
	dependencies := map[string]string{
		"aws": "AWS CLI is required. Please install it from https://aws.amazon.com/cli/",
	}

	for command, message := range dependencies {
		if !isCommandAvailable(command) {
			fmt.Printf("%s is not installed.\n%s\n", command, message)
			fmt.Print("Would you like to install it now? (y/n): ")

			var response string
			fmt.Scanln(&response)
			if response == "y" || response == "Y" {
				InstallCommand(command)
			} else {
				log.Fatalf("%s is required to run this application. Exiting.", command)
			}
		}
	}
}

// InstallCommand installs a given command based on the OS.
func InstallCommand(command string) {
	osType := runtime.GOOS

	switch command {
	case "aws":
		installAWSCLI(osType)
	default:
		log.Fatalf("Installation script for %s is not implemented.", command)
	}
}

func installAWSCLI(osType string) {
	switch osType {
	case "linux":
		installAWSCLIOnLinux()
	case "darwin":
		installAWSCLIOnMac()
	case "windows":
		installAWSCLIOnWindows()
	default:
		log.Fatalf("Unsupported operating system: %s. Please install AWS CLI manually.", osType)
	}
}
