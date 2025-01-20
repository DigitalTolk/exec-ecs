package main

import (
	"context"
	"ecs-tool/cli"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/briandowns/spinner"
	"github.com/creack/pty"
	"golang.org/x/term"
)

func main() {
	cli := cli.ParseArgs()
	ctx := context.Background()

	fmt.Println("Interactive mode is enabled.")
	cli.Profile = cli.SelectProfile()
	cli.Region = cli.PromptWithDefault("Choose AWS region", cli.Region, []string{"eu-north-1", "eu-central-1", "eu-west-2"})

	spinner := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spinner.Start()
	spinner.Suffix = " Loading AWS configuration..."

	cli.LogAWSCommand("configure", "get", "region", "--profile", cli.Profile)
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cli.Region),
		config.WithSharedConfigProfile(cli.Profile),
	)
	spinner.Stop()

	if err != nil {
		cli.LogUserFriendlyError("Unable to load AWS configuration", err, "Make sure your AWS credentials and configuration files are correctly set up.", "~/.aws/config", 37)
	}

	stsClient := sts.NewFromConfig(awsCfg)
	spinner.Start()
	spinner.Suffix = " Checking AWS SSO session..."

	cli.LogAWSCommand("sts", "get-caller-identity", "--profile", cli.Profile)
	if err := cli.CheckSSOSession(ctx, stsClient, cli.Profile); err != nil {
		spinner.Stop()
		fmt.Println("No active SSO session found. Initiating login...")
		cli.LogAWSCommand("sso", "login", "--profile", cli.Profile)
		cmd := exec.Command("aws", "sso", "login", "--profile", cli.Profile)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			cli.LogUserFriendlyError("AWS SSO login failed", err, "Ensure you are authorized for SSO and that your credentials are valid.", "~/.aws/credentials", 45)
		}
	}
	spinner.Stop()

	spinner.Start()
	spinner.Suffix = " Connecting to ECS..."
	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "list-clusters", "--profile", cli.Profile, "--region", cli.Region)
	clusterArn, err := cli.SelectCluster(ctx, ecsClient)
	spinner.Stop()

	if err != nil {
		cli.LogUserFriendlyError("Error selecting cluster", err, "Verify that you have access to ECS clusters in the selected region.", "ECS Cluster configuration", 50)
	}

	spinner.Start()
	spinner.Suffix = " Fetching ECS services..."
	cli.LogAWSCommand("ecs", "list-services", "--cluster", clusterArn, "--profile", cli.Profile, "--region", cli.Region)
	services, err := cli.ListServices(ctx, ecsClient, clusterArn)
	spinner.Stop()

	if err != nil {
		cli.LogUserFriendlyError("Error listing services", err, "Check if the selected cluster has any services and you have proper permissions.", "ECS Service configuration", 55)
	}
	cli.Service = cli.PromptWithDefault("Choose ECS service", cli.Service, services)

	spinner.Start()
	spinner.Suffix = " Fetching ECS tasks..."
	cli.LogAWSCommand("ecs", "list-tasks", "--cluster", clusterArn, "--service-name", cli.Service, "--profile", cli.Profile, "--region", cli.Region)
	taskArn, err := cli.SelectTask(ctx, ecsClient, clusterArn, cli.Service)
	spinner.Stop()

	if err != nil {
		cli.LogUserFriendlyError("Error selecting task", err, "Ensure there are running tasks in the selected service.", "ECS Task configuration", 60)
	}

	executeCmd := []string{
		"ecs", "execute-command",
		"--cluster", clusterArn,
		"--task", taskArn,
		"--container", cli.Container,
		"--interactive",
		"--command", cli.Command,
		"--profile", cli.Profile,
		"--region", cli.Region,
	}

	cli.LogAWSCommand(executeCmd[0], executeCmd[1:]...)

	// Use PTY for interactive session
	cmd := exec.Command("aws", executeCmd...)

	// Start PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		cli.LogUserFriendlyError("Failed to start PTY session", err, "Ensure your system supports pseudo-terminals.", "PTY Setup", 67)
	}
	defer func() { _ = ptmx.Close() }() // Close the PTY

	// Set the terminal in raw mode for interactive session
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("[ERROR] Failed to set terminal to raw mode: %v", err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Restore the terminal state

	// Copy stdin to PTY and PTY to stdout/stderr
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, ptmx)
}
