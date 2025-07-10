package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const Version = "v1.1.7"

func CheckAndInstallDependencies() {
	dependencies := map[string]string{
		"aws":                    "AWS CLI is required. Please install it from https://aws.amazon.com/cli/",
		"session-manager-plugin": "AWS Session Manager Plugin is required. Please install it from https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html",
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

func InstallCommand(command string) {
	osType := runtime.GOOS

	switch command {
	case "aws":
		installAWSCLI(osType)
	case "session-manager-plugin":
		installSessionManagerPlugin(osType)
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

func installSessionManagerPlugin(osType string) {
	switch osType {
	case "linux":
		executeCommands([][]string{
			{"curl", "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/linux_64bit/session-manager-plugin.rpm", "-o", "session-manager-plugin.rpm"},
			{"sudo", "yum", "install", "-y", "session-manager-plugin.rpm"},
		})
	case "darwin":
		executeCommands([][]string{
			{"curl", "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/mac/sessionmanager-bundle.zip", "-o", "sessionmanager-bundle.zip"},
			{"unzip", "sessionmanager-bundle.zip"},
			{"sudo", "./sessionmanager-bundle/install", "-i", "/usr/local/sessionmanagerplugin", "-b", "/usr/local/bin/session-manager-plugin"},
		})
	case "windows":
		fmt.Println("Please download and install the AWS Session Manager Plugin manually from https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	default:
		log.Fatalf("Unsupported operating system: %s. Please install AWS Session Manager Plugin manually.", osType)
	}
}

func UpgradeExecECS() {
	repo := "DigitalTolk/exec-ecs"
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	fmt.Println("Checking for the latest version...")
	resp, err := http.Get(apiURL)
	if err != nil {
		log.Fatalf("Failed to fetch release info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Unexpected response status: %v", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Fatalf("Failed to parse release info: %v", err)
	}

	if release.TagName == Version {
		fmt.Println("You already have the latest version:", Version)
		return
	}

	fmt.Printf("New version available: %s. Upgrading...\n", release.TagName)
	binaryName := getBinaryName()

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			downloadURL = asset.URL
			break
		}
	}

	if downloadURL == "" {
		log.Fatalf("No suitable binary found for your platform.")
	}

	downloadAndInstall(downloadURL)
	fmt.Printf("Successfully upgraded to version %s\n", release.TagName)
}

func getBinaryName() string {
	platform := runtime.GOOS
	arch := runtime.GOARCH

	switch platform {
	case "darwin":
		platform = "Darwin"
	case "linux":
		platform = "Linux"
	case "windows":
		platform = "Windows"
	}

	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	case "arm":
		arch = "armv6"
	case "386":
		arch = "i386"
	}

	ext := "tar.gz"
	if platform == "Windows" {
		ext = "zip"
	}

	return fmt.Sprintf("exec-ecs_%s_%s.%s", platform, arch, ext)
}

func downloadAndInstall(url string) {

	if os.Geteuid() != 0 {
		fmt.Println("This operation requires administrator privileges.")
		fmt.Println("Please enter your password when prompted.")

		exe, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to get executable path: %v", err)
		}

		args := []string{exe}
		args = append(args, os.Args[1:]...)

		// Run the same command with sudo
		sudoCmd := exec.Command("sudo", args...)
		sudoCmd.Stdin = os.Stdin
		sudoCmd.Stdout = os.Stdout
		sudoCmd.Stderr = os.Stderr

		if err := sudoCmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				os.Exit(exitError.ExitCode())
			}
			log.Fatalf("Failed to run with sudo: %v", err)
		}

		os.Exit(0)
	}

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to download binary: %v", err)
	}
	defer resp.Body.Close()

	urlPath := strings.Split(url, "/")
	filename := urlPath[len(urlPath)-1]

	file, err := os.CreateTemp("", "exec-ecs-*-"+filename)
	if err != nil {
		log.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(file.Name())

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Fatalf("Failed to write binary to file: %v", err)
	}

	if err := file.Close(); err != nil {
		log.Fatalf("Failed to close file: %v", err)
	}

	fmt.Println("Extracting and installing...")
	extractAndInstall(file.Name(), filename)
}

func extractAndInstall(filePath, originalFilename string) {
	var ext string
	if strings.HasSuffix(originalFilename, ".tar.gz") {
		ext = ".tar.gz"
	} else if strings.HasSuffix(originalFilename, ".zip") {
		ext = ".zip"
	} else {
		log.Fatalf("Unsupported file format: %s", originalFilename)
	}

	tempDir, err := os.MkdirTemp("", "exec-ecs-extract-*")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	log.Printf("Extracting binary with extension: %v", ext)

	if ext == ".zip" {
		cmd := exec.Command("unzip", "-o", filePath, "-d", tempDir)
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to unzip binary: %v", err)
		}
	} else if ext == ".tar.gz" {
		cmd := exec.Command("tar", "-xzf", filePath, "-C", tempDir)
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to extract binary: %v", err)
		}
	}

	binaryName := "exec-ecs"
	if runtime.GOOS == "windows" {
		binaryName = "exec-ecs.exe"
	}

	sourcePath := filepath.Join(tempDir, binaryName)
	destPath := "/usr/local/bin/exec-ecs"

	input, err := os.ReadFile(sourcePath)
	if err != nil {
		log.Fatalf("Failed to read binary: %v", err)
	}

	if err := os.WriteFile(destPath, input, 0755); err != nil {
		log.Fatalf("Failed to install binary to %s: %v", destPath, err)
	}

	fmt.Println("exec-ecs successfully upgraded!")
}
