package main

import (
	"context"
	"ecs-tool/cli"
	"ecs-tool/installer"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/briandowns/spinner"
	"github.com/creack/pty"
	"golang.org/x/term"
)

func main() {
	ctx := context.Background()

	installer.CheckAndInstallDependencies()

	cli := initializeCLI(ctx)
	awsCfg := loadAWSConfig(ctx, cli)

	validateSSOSession(ctx, cli, awsCfg)

	clusterArn := selectCluster(ctx, cli, awsCfg)
	serviceName := selectService(ctx, cli, awsCfg, clusterArn)
	taskArn := selectTask(ctx, cli, awsCfg, clusterArn, serviceName)

	executeECSCommand(cli, clusterArn, taskArn)
}

func initializeCLI(ctx context.Context) *cli.Cli {
	cli := cli.ParseArgs()
	cli.Profile = cli.SelectProfile()
	cli.Region = cli.PromptWithDefault("Choose AWS region", cli.Region, []string{"eu-north-1", "eu-central-1", "eu-west-2"})
	return &cli
}

func loadAWSConfig(ctx context.Context, cli *cli.Cli) aws.Config {
	sp := createSpinner("Loading AWS configuration...")
	defer sp.Stop()

	cli.LogAWSCommand("configure", "get", "region", "--profile", cli.Profile)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cli.Region),
		config.WithSharedConfigProfile(cli.Profile),
	)

	if err != nil {
		cli.LogUserFriendlyError("Unable to load AWS configuration", err, "Make sure your AWS credentials and configuration files are correctly set up.", "~/.aws/config", 37)
	}

	return cfg
}

func validateSSOSession(ctx context.Context, cli *cli.Cli, awsCfg aws.Config) {
	sp := createSpinner("Checking AWS SSO session...")
	defer sp.Stop()

	stsClient := sts.NewFromConfig(awsCfg)
	cli.LogAWSCommand("sts", "get-caller-identity", "--profile", cli.Profile)
	if err := cli.CheckSSOSession(ctx, stsClient, cli.Profile); err != nil {
		fmt.Println("No active SSO session found. Initiating login...")
		cli.LogAWSCommand("sso", "login", "--profile", cli.Profile)
		cmd := exec.Command("aws", "sso", "login", "--profile", cli.Profile)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			cli.LogUserFriendlyError("AWS SSO login failed", err, "Ensure you are authorized for SSO and that your credentials are valid.", "~/.aws/credentials", 45)
		}
	}
}

func selectCluster(ctx context.Context, cli *cli.Cli, awsCfg aws.Config) string {
	sp := createSpinner("Connecting to ECS...")
	defer sp.Stop()

	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "list-clusters", "--profile", cli.Profile, "--region", cli.Region)

	clusterArn, err := cli.SelectCluster(ctx, ecsClient)
	if err != nil {
		cli.LogUserFriendlyError("Error selecting cluster", err, "Verify that you have access to ECS clusters in the selected region.", "ECS Cluster configuration", 50)
	}
	return clusterArn
}

func selectService(ctx context.Context, cli *cli.Cli, awsCfg aws.Config, clusterArn string) string {
	sp := createSpinner("Fetching ECS services...")
	defer sp.Stop()

	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "list-services", "--cluster", clusterArn, "--profile", cli.Profile, "--region", cli.Region)

	services, err := cli.ListServices(ctx, ecsClient, clusterArn)
	if err != nil {
		cli.LogUserFriendlyError("Error listing services", err, "Check if the selected cluster has any services and you have proper permissions.", "ECS Service configuration", 55)
	}

	return cli.PromptWithDefault("Choose ECS service", cli.Service, services)
}

func selectTask(ctx context.Context, cli *cli.Cli, awsCfg aws.Config, clusterArn, serviceName string) string {
	sp := createSpinner("Fetching ECS tasks...")
	defer sp.Stop()

	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "list-tasks", "--cluster", clusterArn, "--service-name", serviceName, "--profile", cli.Profile, "--region", cli.Region)

	taskArn, err := cli.SelectTask(ctx, ecsClient, clusterArn, serviceName)
	if err != nil {
		cli.LogUserFriendlyError("Error selecting task", err, "Ensure there are running tasks in the selected service.", "ECS Task configuration", 60)
	}

	return taskArn
}

func executeECSCommand(cli *cli.Cli, clusterArn, taskArn string) {
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
	cmd := exec.Command("aws", executeCmd...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		cli.LogUserFriendlyError("Failed to start PTY session", err, "Ensure your system supports pseudo-terminals.", "PTY Setup", 67)
	}
	defer func() { _ = ptmx.Close() }()

	setupTerminalForPTY(ptmx)
}

func setupTerminalForPTY(ptmx *os.File) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("[ERROR] Failed to set terminal to raw mode: %v", err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, ptmx)
}

func createSpinner(suffix string) *spinner.Spinner {
	sp := spinner.New(spinner.CharSets[38], 100*time.Millisecond)
	sp.Start()
	sp.Suffix = " " + suffix
	return sp
}
