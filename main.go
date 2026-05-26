package main

import (
	"context"
	"ecs-tool/cli"
	"ecs-tool/installer"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/briandowns/spinner"
)

type stepState struct {
	Profile             string
	Region              string
	ClusterArn          string
	Service             string
	TaskArn             string
	Container           string
	AutoSelectedCluster bool
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

	// Outer loop: after each exec session ends, drop the user back into the
	// picker (rewinding to the cluster step) so they can run another command
	// without restarting the binary. The interactive picker itself handles
	// ctrl+b back-navigation and ctrl+c clean exit.
	awsCfg := aws.Config{}
	awsCfgLoaded := false
	for {
		var err error
		awsCfg, awsCfgLoaded, err = runInteractiveSelection(ctx, c, &state, awsCfg, awsCfgLoaded)
		if err != nil {
			c.LogUserFriendlyError("Selection failed", err, "See error details above.", "", 0)
		}

		exitCode, execErr := cli.ExecECS(ctx, c, awsCfg, cli.ExecOptions{
			Region:     state.Region,
			ClusterArn: state.ClusterArn,
			TaskArn:    state.TaskArn,
			Container:  state.Container,
			Command:    c.Command,
		})
		if execErr != nil {
			fmt.Fprintln(os.Stderr, "exec-ecs:", execErr)
		}
		if exitCode != 0 {
			fmt.Fprintf(os.Stderr, "session exited with code %d\n", exitCode)
		}

		// Re-enter at the container step for the same task so running a
		// second command does not force the user back through cluster/service/task.
		resetFrom(&state, stepContainer)
	}
}

func runInteractiveSelection(ctx context.Context, c *cli.Cli, state *stepState, awsCfg aws.Config, awsCfgLoaded bool) (aws.Config, bool, error) {
	step := initialSelectionStep(*state)
	ssoEnsured := awsCfgLoaded

	for step < finalStep {
		switch step {
		case stepProfile:
			profiles := c.SelectProfileList()
			if len(profiles) == 0 {
				return awsCfg, awsCfgLoaded, fmt.Errorf("no AWS profiles found")
			}
			selected, goBack := c.PromptSelectBreadcrumb("Choose AWS profile", profiles, state.Profile, step > stepProfile, breadcrumbFor(*state, stepProfile))
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
					return awsCfg, awsCfgLoaded, err
				}
				ssoEnsured = true
			}

			var discoveryErr error
			regions, goBack, err := c.PromptSelectLoadedBreadcrumb("Discovering regions with ECS clusters...", "Choose AWS region", state.Region, true, breadcrumbFor(*state, stepRegion), false, func() ([]string, error) {
				regions, err := discoverRegions(ctx, c)
				if err != nil {
					discoveryErr = err
					return cli.DefaultRegions, nil
				}
				if len(regions) == 0 {
					return nil, errNoRegions
				}
				return regions, nil
			})
			if goBack {
				resetFrom(state, stepRegion)
				awsCfgLoaded = false
				step--
				continue
			}
			if err != nil {
				if errors.Is(err, errNoRegions) {
					fmt.Println("No regions with ECS clusters found for profile", c.Profile)
					choice, goBack := c.PromptSelectBreadcrumb("No regions with clusters. What now?",
						[]string{"Refresh", "Back"}, "Refresh", true, breadcrumbFor(*state, stepRegion))
					if goBack || choice == "Back" {
						resetFrom(state, stepRegion)
						awsCfgLoaded = false
						step--
						continue
					}
					cli.ClearRegionCache(c.Profile)
					continue
				}
				return awsCfg, awsCfgLoaded, err
			}
			if discoveryErr != nil {
				fmt.Println("Failed to discover regions with clusters:", discoveryErr)
			}
			if state.Region != regions {
				awsCfgLoaded = false
			}
			state.Region = regions
			c.Region = regions
			resetFrom(state, stepCluster)
			step++

		case stepCluster, stepService, stepTask, stepContainer:
			if !awsCfgLoaded {
				cfg, err := loadAWSConfig(ctx, c)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				awsCfg = cfg
				if !ssoEnsured {
					if err := validateSSOSession(ctx, c, awsCfg); err != nil {
						return awsCfg, awsCfgLoaded, err
					}
					ssoEnsured = true
				}
				awsCfgLoaded = true
			}

			switch step {
			case stepCluster:
				next, err := pickCluster(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			case stepService:
				next, err := pickService(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			case stepTask:
				next, err := pickTask(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			case stepContainer:
				next, err := pickContainer(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			}
		}
	}
	return awsCfg, awsCfgLoaded, nil
}

func initialSelectionStep(state stepState) int {
	if state.Profile == "" {
		return stepProfile
	}
	if state.Region == "" {
		return stepRegion
	}
	if state.ClusterArn == "" {
		return stepCluster
	}
	if state.Service == "" {
		return stepService
	}
	if state.TaskArn == "" {
		return stepTask
	}
	if state.Container == "" {
		return stepContainer
	}
	return finalStep
}

func resetFrom(state *stepState, from int) {
	if from <= stepRegion {
		state.Region = ""
	}
	if from <= stepCluster {
		state.ClusterArn = ""
		state.AutoSelectedCluster = false
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

// pickCluster / pickService / pickTask / pickContainer keep the AWS loading
// state inside the same Bubble Tea dialog that later shows the selection list.
func pickCluster(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	client := cli.NewECSClient(awsCfg, c.Region)
	var clusterArns map[string]string
	autoSelectedCluster := false

	c.LogAWSCommand("ecs", "list-clusters", "--profile", c.Profile, "--region", c.Region)
	selected, goBack, err := c.PromptSelectLoadedBreadcrumb("Connecting to ECS...", "Choose ECS cluster", getKeyByValue(clusterArns, state.ClusterArn), true, breadcrumbFor(*state, stepCluster), true, func() ([]string, error) {
		clusters, arns, err := c.ListClusterNamesArns(ctx, client)
		if err != nil {
			return nil, err
		}
		if len(clusters) == 0 {
			return nil, errNoClusters
		}
		autoSelectedCluster = len(clusters) == 1
		clusterArns = arns
		return clusters, nil
	})
	if goBack {
		resetFrom(state, stepCluster)
		return int(cli.ActionBack), nil
	}
	if errors.Is(err, errNoClusters) {
		fmt.Println("No ECS clusters found in region:", c.Region)
		choice, goBack := c.PromptSelectBreadcrumb("No clusters found. What now?", []string{"Retry", "Back"}, "Retry", true, breadcrumbFor(*state, stepCluster))
		if goBack || choice == "Back" {
			resetFrom(state, stepCluster)
			cli.ClearRegionCache(c.Profile)
			return int(cli.ActionBack), nil
		}
		return int(cli.ActionRetry), nil
	}
	if err != nil {
		fmt.Println("Failed to list ECS clusters:", err)
		choice, goBack := c.PromptSelectBreadcrumb("Cluster lookup failed. What now?", []string{"Retry", "Back"}, "Retry", true, breadcrumbFor(*state, stepCluster))
		if goBack || choice == "Back" {
			resetFrom(state, stepCluster)
			return int(cli.ActionBack), nil
		}
		return int(cli.ActionRetry), nil
	}
	state.ClusterArn = clusterArns[selected]
	state.AutoSelectedCluster = autoSelectedCluster
	c.ClusterArn = state.ClusterArn
	resetFrom(state, stepService)
	return int(cli.ActionAdvance), nil
}

func pickService(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	client := cli.NewECSClient(awsCfg, c.Region)
	var serviceArns map[string]string

	c.LogAWSCommand("ecs", "list-services", "--cluster", state.ClusterArn, "--profile", c.Profile, "--region", c.Region)
	selected, goBack, err := c.PromptSelectLoadedBreadcrumb("Fetching ECS services...", "Choose ECS service", getKeyByValue(serviceArns, state.Service), true, breadcrumbFor(*state, stepService), false, func() ([]string, error) {
		services, arns, err := c.ListServiceNamesArns(ctx, client, state.ClusterArn)
		if err != nil {
			return nil, err
		}
		if len(services) == 0 {
			return nil, errNoServices
		}
		serviceArns = arns
		return services, nil
	})
	if goBack {
		return serviceBackDelta(state), nil
	}
	if errors.Is(err, errNoServices) {
		fmt.Println("No ECS services found in cluster:", displayName(state.ClusterArn))
		choice, goBack := c.PromptSelectBreadcrumb("No ECS services found. What now?",
			[]string{"Choose region", "Choose cluster"}, "Choose region", true, breadcrumbFor(*state, stepService))
		if goBack || choice == "Choose region" || state.AutoSelectedCluster {
			resetFrom(state, stepCluster)
			return stepRegion - stepService, nil
		}
		resetFrom(state, stepService)
		return int(cli.ActionBack), nil
	}
	if err != nil {
		fmt.Println("Failed to list ECS services:", err)
		resetFrom(state, stepService)
		return int(cli.ActionBack), nil
	}
	state.Service = serviceArns[selected]
	c.Service = state.Service
	resetFrom(state, stepTask)
	return int(cli.ActionAdvance), nil
}

func serviceBackDelta(state *stepState) int {
	if state.AutoSelectedCluster {
		resetFrom(state, stepCluster)
		return stepRegion - stepService
	}
	resetFrom(state, stepService)
	return int(cli.ActionBack)
}

func pickTask(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	client := cli.NewECSClient(awsCfg, c.Region)
	var taskArns map[string]string

	c.LogAWSCommand("ecs", "list-tasks", "--cluster", state.ClusterArn, "--service-name", state.Service, "--profile", c.Profile, "--region", c.Region)
	selected, goBack, err := c.PromptSelectLoadedBreadcrumb("Fetching ECS tasks...", "Choose ECS task", getKeyByValue(taskArns, state.TaskArn), true, breadcrumbFor(*state, stepTask), false, func() ([]string, error) {
		tasks, arns, err := c.ListTaskNamesArns(ctx, client, state.ClusterArn, state.Service)
		if err != nil {
			return nil, err
		}
		if len(tasks) == 0 {
			return nil, errNoTasks
		}
		taskArns = arns
		return tasks, nil
	})
	if goBack {
		resetFrom(state, stepTask)
		return int(cli.ActionBack), nil
	}
	if errors.Is(err, errNoTasks) {
		fmt.Println("No ECS tasks found. Going back.")
		resetFrom(state, stepTask)
		return int(cli.ActionBack), nil
	}
	if err != nil {
		fmt.Println("Failed to list ECS tasks:", err)
		resetFrom(state, stepTask)
		return int(cli.ActionBack), nil
	}
	state.TaskArn = taskArns[selected]
	c.TaskArn = state.TaskArn
	resetFrom(state, stepContainer)
	return int(cli.ActionAdvance), nil
}

func pickContainer(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	client := cli.NewECSClient(awsCfg, c.Region)

	c.LogAWSCommand("ecs", "describe-tasks", "--cluster", state.ClusterArn, "--tasks", state.TaskArn, "--profile", c.Profile, "--region", c.Region)
	selected, goBack, err := c.PromptSelectLoadedBreadcrumb("Fetching ECS containers...", "Choose a container", state.Container, true, breadcrumbFor(*state, stepContainer), false, func() ([]string, error) {
		containers, err := c.ListContainerNames(ctx, client, state.ClusterArn, state.TaskArn)
		if err != nil {
			return nil, err
		}
		if len(containers) == 0 {
			return nil, errNoContainers
		}
		return containers, nil
	})
	if goBack {
		resetFrom(state, stepContainer)
		return int(cli.ActionBack), nil
	}
	if errors.Is(err, errNoContainers) {
		fmt.Println("No containers found. Going back.")
		resetFrom(state, stepContainer)
		return int(cli.ActionBack), nil
	}
	if err != nil {
		fmt.Println("Failed to describe ECS task:", err)
		resetFrom(state, stepContainer)
		return int(cli.ActionBack), nil
	}
	state.Container = selected
	c.Container = state.Container
	return int(cli.ActionAdvance), nil
}

var (
	errNoClusters   = errors.New("no ECS clusters")
	errNoRegions    = errors.New("no ECS regions")
	errNoServices   = errors.New("no ECS services")
	errNoTasks      = errors.New("no ECS tasks")
	errNoContainers = errors.New("no containers")
)

func toCliState(s stepState) cli.State {
	return cli.State{
		Profile: s.Profile, Region: s.Region, ClusterArn: s.ClusterArn,
		Service: s.Service, TaskArn: s.TaskArn, Container: s.Container,
	}
}

func fromCliState(s cli.State) stepState {
	return stepState{
		Profile: s.Profile, Region: s.Region, ClusterArn: s.ClusterArn,
		Service: s.Service, TaskArn: s.TaskArn, Container: s.Container,
	}
}

func breadcrumbFor(state stepState, step int) string {
	parts := make([]string, 0, 5)
	if state.Profile != "" && step > stepProfile {
		parts = append(parts, "Profile: "+state.Profile)
	}
	if state.Region != "" && step > stepRegion {
		parts = append(parts, "Region: "+state.Region)
	}
	if state.ClusterArn != "" && step > stepCluster {
		parts = append(parts, "Cluster: "+displayName(state.ClusterArn))
	}
	if state.Service != "" && step > stepService {
		parts = append(parts, "Service: "+displayName(state.Service))
	}
	if state.TaskArn != "" && step > stepTask {
		parts = append(parts, "Task: "+displayName(state.TaskArn))
	}
	return strings.Join(parts, " > ")
}

func displayName(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	return parts[len(parts)-1]
}

func discoverRegions(ctx context.Context, c *cli.Cli) ([]string, error) {
	return cli.DiscoverRegionsWithClusters(ctx, c.Profile, cli.DefaultRegions)
}

func initializeCLI(ctx context.Context) *cli.Cli {
	cli.ApplySavedThemeSelection()
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

// ensureSSOLogin authenticates the user up-front so that all profiles bound
// to the same sso-session piggy-back on a single login. SSO profiles use the
// OAuth2 device-code flow natively; no `aws sso login` subprocess is used.
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

	ssoCfg, err := c.LookupSSOSessionConfig(c.Profile)
	if err != nil {
		return err
	}
	if ssoCfg == nil {
		// Not an SSO profile and we don't have valid creds — nothing we
		// can do natively. Tell the user instead of silently failing.
		return fmt.Errorf("profile %q is not an SSO profile and has no valid credentials cached; configure sso_start_url / sso_region (or run `aws configure sso`)", c.Profile)
	}
	if ssoCfg.Legacy {
		fmt.Printf("No active SSO session found. Logging in to %s (legacy SSO profile)...\n", ssoCfg.StartURL)
		c.LogAWSCommand("[native]", "sso", "login", "--profile", c.Profile)
	} else {
		fmt.Printf("No active SSO session found. Logging in to sso-session %q (covers every profile bound to it)...\n", ssoCfg.Name)
		c.LogAWSCommand("[native]", "sso", "login", "--sso-session", ssoCfg.Name)
	}
	if err := c.PerformNativeSSOLogin(ctx, ssoCfg); err != nil {
		return err
	}

	// Sanity-check credentials immediately after login so we fail fast (and
	// loudly) here rather than after the user has waited for region
	// discovery. A ForbiddenException at this point almost always means the
	// role on the profile isn't actually granted in the account.
	freshCfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(c.Profile),
		config.WithDefaultRegion(defaultProbeRegion(c)),
	)
	if err != nil {
		return fmt.Errorf("reload AWS config after login: %w", err)
	}
	if _, err := sts.NewFromConfig(freshCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); err != nil {
		return fmt.Errorf("SSO login succeeded but credentials don't work for profile %q (check sso_account_id / sso_role_name): %w", c.Profile, err)
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
