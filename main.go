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
	"strings"
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

	// Check for history flag
	if cli.History {
		showHistoryAndExecute(cli)
		return
	}

	awsCfg := loadAWSConfig(ctx, cli)

	validateSSOSession(ctx, cli, awsCfg)

	type StepState struct {
		Profile    string
		Region     string
		ClusterArn string
		Service    string
		TaskArn    string
		Container  string
	}

	state := StepState{
		Profile:    cli.Profile,
		Region:     cli.Region,
		ClusterArn: cli.ClusterArn,
		Service:    cli.Service,
		TaskArn:    cli.TaskArn,
		Container:  cli.Container,
	}

	step := 0
	const (
		stepProfile   = 0
		stepRegion    = 1
		stepCluster   = 2
		stepService   = 3
		stepTask      = 4
		stepContainer = 5
		finalStep     = 6
	)

	for step < finalStep {
		switch step {
		case stepProfile:
			profiles := cli.SelectProfileList()
			if len(profiles) == 0 {
				fmt.Println("No AWS profiles found. Exiting.")
				os.Exit(1)
			}
			selected, goBack := cli.PromptSelect("Choose AWS profile", profiles, state.Profile, step > stepProfile)
			if goBack && step > stepProfile {
				// Clear current and subsequent state
				state.Profile = ""
				state.Region = ""
				state.ClusterArn = ""
				state.Service = ""
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			state.Profile = selected
			cli.Profile = selected
			// Clear downstream state
			state.Region = ""
			state.ClusterArn = ""
			state.Service = ""
			state.TaskArn = ""
			state.Container = ""
			step++
		case stepRegion:
			regions := []string{"eu-north-1", "eu-central-1", "eu-west-2"}
			selected, goBack := cli.PromptWithDefault("Choose AWS region", state.Region, regions, true)
			if goBack {
				// Clear current and subsequent state
				state.Region = ""
				state.ClusterArn = ""
				state.Service = ""
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			state.Region = selected
			cli.Region = selected
			// Clear downstream state
			state.ClusterArn = ""
			state.Service = ""
			state.TaskArn = ""
			state.Container = ""
			step++
		case stepCluster:
			sp := createSpinner("Connecting to ECS...")
			ecsClient := ecs.NewFromConfig(loadAWSConfig(ctx, cli))
			cli.LogAWSCommand("ecs", "list-clusters", "--profile", cli.Profile, "--region", cli.Region)
			sp.Stop()
			clusters, clusterArns := cli.ListClusterNamesArns(ctx, ecsClient)
			if len(clusters) == 0 {
				fmt.Println("No ECS clusters found. Going back.")
				state.ClusterArn = ""
				state.Service = ""
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			selected, goBack := cli.PromptSelect("Choose ECS cluster", clusters, getKeyByValue(clusterArns, state.ClusterArn), true)
			if goBack {
				state.ClusterArn = ""
				state.Service = ""
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			state.ClusterArn = clusterArns[selected]
			cli.ClusterArn = state.ClusterArn
			// Clear downstream state
			state.Service = ""
			state.TaskArn = ""
			state.Container = ""
			step++
		case stepService:
			sp := createSpinner("Fetching ECS services...")
			ecsClient := ecs.NewFromConfig(loadAWSConfig(ctx, cli))
			cli.LogAWSCommand("ecs", "list-services", "--cluster", state.ClusterArn, "--profile", cli.Profile, "--region", cli.Region)
			sp.Stop()
			services, serviceArns := cli.ListServiceNamesArns(ctx, ecsClient, state.ClusterArn)
			if len(services) == 0 {
				fmt.Println("No ECS services found. Going back.")
				state.Service = ""
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			selected, goBack := cli.PromptSelect("Choose ECS service", services, getKeyByValue(serviceArns, state.Service), true)
			if goBack {
				state.Service = ""
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			state.Service = serviceArns[selected]
			cli.Service = state.Service
			// Clear downstream state
			state.TaskArn = ""
			state.Container = ""
			step++
		case stepTask:
			sp := createSpinner("Fetching ECS tasks...")
			ecsClient := ecs.NewFromConfig(loadAWSConfig(ctx, cli))
			cli.LogAWSCommand("ecs", "list-tasks", "--cluster", state.ClusterArn, "--service-name", state.Service, "--profile", cli.Profile, "--region", cli.Region)
			sp.Stop()
			tasks, taskArns := cli.ListTaskNamesArns(ctx, ecsClient, state.ClusterArn, state.Service)
			if len(tasks) == 0 {
				fmt.Println("No ECS tasks found. Going back.")
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			selected, goBack := cli.PromptSelect("Choose ECS task", tasks, getKeyByValue(taskArns, state.TaskArn), true)
			if goBack {
				state.TaskArn = ""
				state.Container = ""
				step--
				continue
			}
			state.TaskArn = taskArns[selected]
			cli.TaskArn = state.TaskArn
			// Clear downstream state
			state.Container = ""
			step++
		case stepContainer:
			sp := createSpinner("Fetching ECS containers...")
			ecsClient := ecs.NewFromConfig(loadAWSConfig(ctx, cli))
			cli.LogAWSCommand("ecs", "describe-tasks", "--cluster", state.ClusterArn, "--tasks", state.TaskArn, "--profile", cli.Profile, "--region", cli.Region)
			sp.Stop()
			containers := cli.ListContainerNames(ctx, ecsClient, state.ClusterArn, state.TaskArn)
			if len(containers) == 0 {
				fmt.Println("No containers found. Going back.")
				state.Container = ""
				step--
				continue
			}
			selected, goBack := cli.PromptSelect("Choose a container", containers, state.Container, true)
			if goBack {
				state.Container = ""
				step--
				continue
			}
			state.Container = selected
			cli.Container = state.Container
			step++
		}
	}

	executeECSCommand(cli, state.ClusterArn, state.TaskArn, state.Container)
}

func initializeCLI(ctx context.Context) *cli.Cli {
	cli := cli.ParseArgs()
	switch {
	case cli.Version:
		fmt.Println("exec-ecs version", installer.Version)
		os.Exit(0)
	case cli.Upgrade:
		installer.UpgradeExecECS()
		os.Exit(0)
	}
	// Remove profile/region selection from here
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

	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "list-clusters", "--profile", cli.Profile, "--region", cli.Region)
	sp.Stop()

	clusterArn, err := cli.SelectCluster(ctx, ecsClient)
	if err != nil {
		cli.LogUserFriendlyError("Error selecting cluster", err, "Verify that you have access to ECS clusters in the selected region.", "ECS Cluster configuration", 50)
	}
	return clusterArn
}

func selectService(ctx context.Context, cli *cli.Cli, awsCfg aws.Config, clusterArn string) string {
	sp := createSpinner("Fetching ECS services...")

	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "list-services", "--cluster", clusterArn, "--profile", cli.Profile, "--region", cli.Region)
	sp.Stop()
	serviceName, err := cli.SelectService(ctx, ecsClient, clusterArn)
	if err != nil {
		cli.LogUserFriendlyError("Error selecting service", err, "Ensure there are services running in the selected cluster.", "ECS Service configuration", 55)
	}
	return serviceName

}

func selectTask(ctx context.Context, cli *cli.Cli, awsCfg aws.Config, clusterArn, serviceName string) string {
	sp := createSpinner("Fetching ECS tasks...")

	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "list-tasks", "--cluster", clusterArn, "--service-name", serviceName, "--profile", cli.Profile, "--region", cli.Region)
	sp.Stop()

	taskArn, err := cli.SelectTask(ctx, ecsClient, clusterArn, serviceName)
	if err != nil {
		cli.LogUserFriendlyError("Error selecting task", err, "Ensure there are running tasks in the selected service.", "ECS Task configuration", 60)
	}
	return taskArn
}

func selectContainer(ctx context.Context, cli *cli.Cli, awsCfg aws.Config, clusterArn, taskArn string) string {
	sp := createSpinner("Fetching ECS containers...")

	ecsClient := ecs.NewFromConfig(awsCfg)
	cli.LogAWSCommand("ecs", "describe-tasks", "--cluster", clusterArn, "--tasks", taskArn, "--profile", cli.Profile, "--region", cli.Region)
	sp.Stop()

	container, err := cli.SelectContainer(ctx, ecsClient, clusterArn, taskArn)
	if err != nil {
		cli.LogUserFriendlyError("Error selecting container", err, "Ensure the selected task has containers running.", "ECS Container configuration", 65)
	}
	return container
}

func executeECSCommand(cli *cli.Cli, clusterArn, taskArn string, container string) {
	executeCmd := []string{
		"ecs", "execute-command",
		"--cluster", clusterArn,
		"--task", taskArn,
		"--container", container,
		"--interactive",
		"--command", cli.Command,
		"--profile", cli.Profile,
		"--region", cli.Region,
	}

	cli.LogAWSCommand(executeCmd[0], executeCmd[1:]...)
	cmd := exec.Command("aws", executeCmd...)

	// Append to history
	cli.AppendToHistory("aws " + strings.Join(executeCmd, " "))

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

// Show history menu and execute selected command
func showHistoryAndExecute(cli *cli.Cli) {
	history := cli.GetLastUniqueHistory(5)
	if len(history) == 0 {
		fmt.Println("No command history found.")
		return
	}
	selected, err := cli.BubbleteaHistorySelect("Command History (last 5 unique)", history)
	if err != nil || selected == "" {
		return
	}
	fmt.Println("Executing:", selected)
	cmd := exec.Command("sh", "-c", selected)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

// Helper to get key by value from map[string]string
func getKeyByValue(m map[string]string, value string) string {
	for k, v := range m {
		if v == value {
			return k
		}
	}
	return ""
}
