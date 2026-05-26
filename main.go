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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/briandowns/spinner"
	"github.com/creack/pty"
	"golang.org/x/term"
)

type stepState struct {
	Profile    string
	Region     string
	ClusterArn string
	Service    string
	TaskArn    string
	Container  string
}

const (
	stepProfile   = 0
	stepRegion    = 1
	stepCluster   = 2
	stepService   = 3
	stepTask      = 4
	stepContainer = 5
	finalStep     = 6
)

func main() {
	ctx := context.Background()

	installer.CheckAndInstallDependencies()

	c := initializeCLI(ctx)

	if c.History {
		showHistoryAndExecute(c)
		return
	}

	state := stepState{
		Profile:    c.Profile,
		Region:     c.Region,
		ClusterArn: c.ClusterArn,
		Service:    c.Service,
		TaskArn:    c.TaskArn,
		Container:  c.Container,
	}

	if err := runInteractiveSelection(ctx, c, &state); err != nil {
		c.LogUserFriendlyError("Selection failed", err, "See error details above.", "", 0)
	}

	executeECSCommand(c, state.ClusterArn, state.TaskArn, state.Container)
}

func runInteractiveSelection(ctx context.Context, c *cli.Cli, state *stepState) error {
	step := 0
	awsCfg := aws.Config{}
	awsCfgLoaded := false
	ssoEnsured := false

	for step < finalStep {
		switch step {
		case stepProfile:
			profiles := c.SelectProfileList()
			if len(profiles) == 0 {
				return fmt.Errorf("no AWS profiles found")
			}
			selected, goBack := c.PromptSelect("Choose AWS profile", profiles, state.Profile, step > stepProfile)
			if goBack && step > stepProfile {
				resetFrom(state, stepProfile)
				awsCfgLoaded = false
				ssoEnsured = false
				step--
				continue
			}
			if state.Profile != selected {
				awsCfgLoaded = false
				ssoEnsured = false
			}
			state.Profile = selected
			c.Profile = selected
			resetFrom(state, stepRegion)
			step++

		case stepRegion:
			if !ssoEnsured {
				if err := ensureSSOLogin(ctx, c); err != nil {
					return err
				}
				ssoEnsured = true
			}

			regions, err := discoverRegions(ctx, c)
			if err != nil {
				fmt.Println("Failed to discover regions with clusters:", err)
				regions = cli.DefaultRegions
			}
			if len(regions) == 0 {
				fmt.Println("No regions with ECS clusters found for profile", c.Profile)
				choice, goBack := c.PromptSelect("No regions with clusters. What now?",
					[]string{"Refresh", "Back"}, "Refresh", true)
				if goBack || choice == "Back" {
					resetFrom(state, stepProfile)
					awsCfgLoaded = false
					ssoEnsured = false
					step--
					continue
				}
				cli.ClearRegionCache(c.Profile)
				continue
			}

			selected, goBack := c.PromptWithDefault("Choose AWS region", state.Region, regions, true)
			if goBack {
				resetFrom(state, stepRegion)
				awsCfgLoaded = false
				step--
				continue
			}
			if state.Region != selected {
				awsCfgLoaded = false
			}
			state.Region = selected
			c.Region = selected
			resetFrom(state, stepCluster)
			step++

		case stepCluster, stepService, stepTask, stepContainer:
			if !awsCfgLoaded {
				cfg, err := loadAWSConfig(ctx, c)
				if err != nil {
					return err
				}
				awsCfg = cfg
				if !ssoEnsured {
					if err := validateSSOSession(ctx, c, awsCfg); err != nil {
						return err
					}
					ssoEnsured = true
				}
				awsCfgLoaded = true
			}

			switch step {
			case stepCluster:
				next, err := pickCluster(ctx, c, awsCfg, state)
				if err != nil {
					return err
				}
				step += next
			case stepService:
				next, err := pickService(ctx, c, awsCfg, state)
				if err != nil {
					return err
				}
				step += next
			case stepTask:
				next, err := pickTask(ctx, c, awsCfg, state)
				if err != nil {
					return err
				}
				step += next
			case stepContainer:
				next, err := pickContainer(ctx, c, awsCfg, state)
				if err != nil {
					return err
				}
				step += next
			}
		}
	}
	return nil
}

func resetFrom(state *stepState, from int) {
	if from <= stepRegion {
		state.Region = ""
	}
	if from <= stepCluster {
		state.ClusterArn = ""
	}
	if from <= stepService {
		state.Service = ""
	}
	if from <= stepTask {
		state.TaskArn = ""
	}
	if from <= stepContainer {
		state.Container = ""
	}
}

func pickCluster(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Connecting to ECS...")
	ecsClient := ecs.NewFromConfig(awsCfg)
	c.LogAWSCommand("ecs", "list-clusters", "--profile", c.Profile, "--region", c.Region)
	clusters, clusterArns, err := c.ListClusterNamesArns(ctx, ecsClient)
	sp.Stop()
	if err != nil {
		fmt.Println("Failed to list ECS clusters:", err)
		choice, goBack := c.PromptSelect("Cluster lookup failed. What now?",
			[]string{"Retry", "Back"}, "Retry", true)
		if goBack || choice == "Back" {
			resetFrom(state, stepCluster)
			return -1, nil
		}
		return 0, nil
	}
	if len(clusters) == 0 {
		fmt.Println("No ECS clusters found in region:", c.Region)
		choice, goBack := c.PromptSelect("No clusters found. What now?",
			[]string{"Retry", "Back"}, "Retry", true)
		if goBack || choice == "Back" {
			resetFrom(state, stepCluster)
			cli.ClearRegionCache(c.Profile)
			return -1, nil
		}
		return 0, nil
	}
	selected, goBack := c.PromptSelect("Choose ECS cluster", clusters, getKeyByValue(clusterArns, state.ClusterArn), true)
	if goBack {
		resetFrom(state, stepCluster)
		return -1, nil
	}
	state.ClusterArn = clusterArns[selected]
	c.ClusterArn = state.ClusterArn
	resetFrom(state, stepService)
	return 1, nil
}

func pickService(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Fetching ECS services...")
	ecsClient := ecs.NewFromConfig(awsCfg)
	c.LogAWSCommand("ecs", "list-services", "--cluster", state.ClusterArn, "--profile", c.Profile, "--region", c.Region)
	services, serviceArns, err := c.ListServiceNamesArns(ctx, ecsClient, state.ClusterArn)
	sp.Stop()
	if err != nil {
		fmt.Println("Failed to list ECS services:", err)
		resetFrom(state, stepService)
		return -1, nil
	}
	if len(services) == 0 {
		fmt.Println("No ECS services found. Going back.")
		resetFrom(state, stepService)
		return -1, nil
	}
	selected, goBack := c.PromptSelect("Choose ECS service", services, getKeyByValue(serviceArns, state.Service), true)
	if goBack {
		resetFrom(state, stepService)
		return -1, nil
	}
	state.Service = serviceArns[selected]
	c.Service = state.Service
	resetFrom(state, stepTask)
	return 1, nil
}

func pickTask(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Fetching ECS tasks...")
	ecsClient := ecs.NewFromConfig(awsCfg)
	c.LogAWSCommand("ecs", "list-tasks", "--cluster", state.ClusterArn, "--service-name", state.Service, "--profile", c.Profile, "--region", c.Region)
	tasks, taskArns, err := c.ListTaskNamesArns(ctx, ecsClient, state.ClusterArn, state.Service)
	sp.Stop()
	if err != nil {
		fmt.Println("Failed to list ECS tasks:", err)
		resetFrom(state, stepTask)
		return -1, nil
	}
	if len(tasks) == 0 {
		fmt.Println("No ECS tasks found. Going back.")
		resetFrom(state, stepTask)
		return -1, nil
	}
	selected, goBack := c.PromptSelect("Choose ECS task", tasks, getKeyByValue(taskArns, state.TaskArn), true)
	if goBack {
		resetFrom(state, stepTask)
		return -1, nil
	}
	state.TaskArn = taskArns[selected]
	c.TaskArn = state.TaskArn
	resetFrom(state, stepContainer)
	return 1, nil
}

func pickContainer(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Fetching ECS containers...")
	ecsClient := ecs.NewFromConfig(awsCfg)
	c.LogAWSCommand("ecs", "describe-tasks", "--cluster", state.ClusterArn, "--tasks", state.TaskArn, "--profile", c.Profile, "--region", c.Region)
	containers, err := c.ListContainerNames(ctx, ecsClient, state.ClusterArn, state.TaskArn)
	sp.Stop()
	if err != nil {
		fmt.Println("Failed to describe ECS task:", err)
		resetFrom(state, stepContainer)
		return -1, nil
	}
	if len(containers) == 0 {
		fmt.Println("No containers found. Going back.")
		resetFrom(state, stepContainer)
		return -1, nil
	}
	selected, goBack := c.PromptSelect("Choose a container", containers, state.Container, true)
	if goBack {
		resetFrom(state, stepContainer)
		return -1, nil
	}
	state.Container = selected
	c.Container = state.Container
	return 1, nil
}

func discoverRegions(ctx context.Context, c *cli.Cli) ([]string, error) {
	sp := createSpinner("Discovering regions with ECS clusters...")
	defer sp.Stop()
	return cli.DiscoverRegionsWithClusters(ctx, c.Profile, cli.DefaultRegions)
}

func initializeCLI(ctx context.Context) *cli.Cli {
	c := cli.ParseArgs()
	_ = ctx
	switch {
	case c.Version:
		fmt.Println("exec-ecs version", installer.Version)
		os.Exit(0)
	case c.Upgrade:
		installer.UpgradeExecECS()
		os.Exit(0)
	}
	return &c
}

func loadAWSConfig(ctx context.Context, c *cli.Cli) (aws.Config, error) {
	sp := createSpinner("Loading AWS configuration...")
	defer sp.Stop()

	c.LogAWSCommand("configure", "get", "region", "--profile", c.Profile)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(c.Region),
		config.WithSharedConfigProfile(c.Profile),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	return cfg, nil
}

// ensureSSOLogin logs into the SSO session up-front (before any per-region
// operation) so that profiles bound to the same sso_session do not have to
// re-authenticate for every region or account.
func ensureSSOLogin(ctx context.Context, c *cli.Cli) error {
	probeCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(defaultProbeRegion(c)),
		config.WithSharedConfigProfile(c.Profile),
	)
	if err != nil {
		return fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	stsClient := sts.NewFromConfig(probeCfg)
	c.LogAWSCommand("sts", "get-caller-identity", "--profile", c.Profile)
	if err := c.CheckSSOSession(ctx, stsClient, c.Profile); err == nil {
		return nil
	}

	sso := c.LookupSSOSessionForProfile(c.Profile)
	var args []string
	if sso != "" {
		fmt.Printf("No active SSO session found. Logging in to sso-session '%s' (covers all profiles bound to it)...\n", sso)
		c.LogAWSCommand("sso", "login", "--sso-session", sso)
		args = []string{"sso", "login", "--sso-session", sso}
	} else {
		fmt.Println("No active SSO session found. Initiating login...")
		c.LogAWSCommand("sso", "login", "--profile", c.Profile)
		args = []string{"sso", "login", "--profile", c.Profile}
	}

	cmd := exec.Command("aws", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws sso login failed: %w", err)
	}
	return nil
}

func defaultProbeRegion(c *cli.Cli) string {
	if c.Region != "" {
		return c.Region
	}
	return "us-east-1"
}

func validateSSOSession(ctx context.Context, c *cli.Cli, awsCfg aws.Config) error {
	sp := createSpinner("Checking AWS SSO session...")
	defer sp.Stop()

	stsClient := sts.NewFromConfig(awsCfg)
	c.LogAWSCommand("sts", "get-caller-identity", "--profile", c.Profile)
	if err := c.CheckSSOSession(ctx, stsClient, c.Profile); err != nil {
		sp.Stop()
		return ensureSSOLogin(ctx, c)
	}
	return nil
}

func executeECSCommand(c *cli.Cli, clusterArn, taskArn string, container string) {
	executeCmd := []string{
		"ecs", "execute-command",
		"--cluster", clusterArn,
		"--task", taskArn,
		"--container", container,
		"--interactive",
		"--command", c.Command,
		"--profile", c.Profile,
		"--region", c.Region,
	}

	c.LogAWSCommand(executeCmd[0], executeCmd[1:]...)
	cmd := exec.Command("aws", executeCmd...)

	c.AppendToHistory("aws " + strings.Join(executeCmd, " "))

	ptmx, err := pty.Start(cmd)
	if err != nil {
		c.LogUserFriendlyError("Failed to start PTY session", err, "Ensure your system supports pseudo-terminals.", "PTY Setup", 67)
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

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		buf := make([]byte, 1)
		_, _ = io.CopyBuffer(ptmx, os.Stdin, buf)
		wg.Done()
	}()

	go func() {
		buf := make([]byte, 1024)
		_, _ = io.CopyBuffer(os.Stdout, ptmx, buf)
		wg.Done()
	}()

	wg.Wait()
}

func createSpinner(suffix string) *spinner.Spinner {
	sp := spinner.New(spinner.CharSets[38], 100*time.Millisecond)
	sp.Suffix = " " + suffix
	sp.Start()
	return sp
}

func showHistoryAndExecute(c *cli.Cli) {
	history := c.GetLastUniqueHistory(5)
	if len(history) == 0 {
		fmt.Println("No command history found.")
		return
	}
	selected, err := c.BubbleteaHistorySelect("Command History (last 5 unique)", history)
	if err != nil || selected == "" {
		return
	}
	fmt.Println("Executing:", selected)
	cmd := exec.Command("sh", "-c", selected)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func getKeyByValue(m map[string]string, value string) string {
	for k, v := range m {
		if v == value {
			return k
		}
	}
	return ""
}
