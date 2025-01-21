package installer

import "fmt"

func installAWSCLIOnLinux() {
	fmt.Println("Detected OS: Linux. Installing AWS CLI...")
	commands := [][]string{
		{"curl", "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip", "-o", "awscliv2.zip"},
		{"unzip", "awscliv2.zip"},
		{"sudo", "./aws/install"},
	}
	executeCommands(commands)
	fmt.Println("AWS CLI installed successfully on Linux.")
}

func installAWSCLIOnMac() {
	fmt.Println("Detected OS: macOS. Installing AWS CLI...")
	commands := [][]string{
		{"curl", "https://awscli.amazonaws.com/AWSCLIV2.pkg", "-o", "AWSCLIV2.pkg"},
		{"sudo", "installer", "-pkg", "AWSCLIV2.pkg", "-target", "/"},
	}
	executeCommands(commands)
	fmt.Println("AWS CLI installed successfully on macOS.")
}

func installAWSCLIOnWindows() {
	fmt.Println("Detected OS: Windows. Please download and install the AWS CLI manually from:")
	fmt.Println("https://aws.amazon.com/cli/")
}
